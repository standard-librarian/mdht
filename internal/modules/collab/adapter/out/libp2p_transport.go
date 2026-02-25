package out

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

const (
	authProtocol protocol.ID = "/mdht/collab/auth/1.0.0"
	opProtocol   protocol.ID = "/mdht/collab/op/1.0.0"
	syncProtocol protocol.ID = "/mdht/collab/sync/1.0.0"
)

type Libp2pTransport struct{}

type libp2pRuntime struct {
	host         host.Host
	workspaceID  string
	workspaceKey []byte
	handlers     collabout.TransportHandlers

	mu            sync.RWMutex
	peers         map[string]domain.Peer
	authenticated map[peer.ID]struct{}
	status        collabout.NetworkStatus
}

type authRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Nonce       string `json:"nonce"`
	Tag         string `json:"tag"`
}

type authResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Nonce       string `json:"nonce"`
	Tag         string `json:"tag"`
	NodeID      string `json:"node_id"`
}

func NewLibp2pTransport() collabout.Transport {
	return &Libp2pTransport{}
}

func (t *Libp2pTransport) Start(ctx context.Context, input collabout.TransportStartInput, handlers collabout.TransportHandlers) (collabout.RuntimeTransport, error) {
	decodedPriv, err := base64.StdEncoding.DecodeString(input.NodeIdentity.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode node private key: %w", err)
	}
	privKey, err := crypto.UnmarshalEd25519PrivateKey(decodedPriv)
	if err != nil {
		return nil, fmt.Errorf("unmarshal node private key: %w", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0", "/ip6/::/tcp/0"),
	)
	if err != nil {
		return nil, fmt.Errorf("start libp2p host: %w", err)
	}

	r := &libp2pRuntime{
		host:          h,
		workspaceID:   input.WorkspaceID,
		workspaceKey:  input.WorkspaceKey,
		handlers:      handlers,
		peers:         map[string]domain.Peer{},
		authenticated: map[peer.ID]struct{}{},
		status: collabout.NetworkStatus{
			Online:      true,
			LastSyncAt:  time.Now().UTC(),
			ListenAddrs: renderListenAddrs(h),
		},
	}

	h.SetStreamHandler(authProtocol, r.handleAuth)
	h.SetStreamHandler(opProtocol, r.handleOp)
	h.SetStreamHandler(syncProtocol, r.handleSync)

	for _, configuredPeer := range input.Peers {
		_ = r.AddPeer(ctx, configuredPeer)
	}
	r.emitStatus()

	go func() {
		<-ctx.Done()
		_ = r.Stop()
	}()

	return r, nil
}

func (r *libp2pRuntime) Broadcast(ctx context.Context, op domain.OpEnvelope) error {
	payload, err := json.Marshal(op)
	if err != nil {
		return err
	}
	for _, id := range r.authenticatedPeers() {
		if err := r.writeMessage(ctx, id, opProtocol, payload); err != nil {
			continue
		}
	}
	r.markSynced()
	return nil
}

func (r *libp2pRuntime) Reconcile(ctx context.Context, ops []domain.OpEnvelope) error {
	payload, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	for _, id := range r.authenticatedPeers() {
		if err := r.writeMessage(ctx, id, syncProtocol, payload); err != nil {
			continue
		}
	}
	r.markSynced()
	return nil
}

func (r *libp2pRuntime) AddPeer(ctx context.Context, peerInfo domain.Peer) error {
	addr, err := multiaddr.NewMultiaddr(peerInfo.Address)
	if err != nil {
		return err
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}
	r.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	dialCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if err := r.host.Connect(dialCtx, *info); err != nil {
		return err
	}
	if err := r.authenticatePeer(dialCtx, info.ID); err != nil {
		return err
	}

	r.mu.Lock()
	r.peers[peerInfo.PeerID] = peerInfo
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()
	r.emitStatus()
	return nil
}

func (r *libp2pRuntime) RemovePeer(_ context.Context, peerID string) error {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.peers, peerID)
	delete(r.authenticated, pid)
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()
	_ = r.host.Network().ClosePeer(pid)
	r.emitStatus()
	return nil
}

func (r *libp2pRuntime) Status() collabout.NetworkStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

func (r *libp2pRuntime) Stop() error {
	r.mu.Lock()
	r.status.Online = false
	r.status.PeerCount = 0
	r.mu.Unlock()
	r.emitStatus()
	return r.host.Close()
}

func (r *libp2pRuntime) authenticatePeer(ctx context.Context, peerID peer.ID) error {
	stream, err := r.host.NewStream(ctx, peerID, authProtocol)
	if err != nil {
		return err
	}
	defer stream.Close()
	encoder := json.NewEncoder(stream)
	decoder := json.NewDecoder(stream)

	nonceRaw := make([]byte, 16)
	if _, err := rand.Read(nonceRaw); err != nil {
		return err
	}
	nonce := hex.EncodeToString(nonceRaw)
	req := authRequest{WorkspaceID: r.workspaceID, Nonce: nonce, Tag: signAuth(r.workspaceKey, r.workspaceID, nonce)}
	if err := encoder.Encode(req); err != nil {
		return err
	}
	resp := authResponse{}
	if err := decoder.Decode(&resp); err != nil {
		return err
	}
	if resp.WorkspaceID != r.workspaceID || resp.Nonce != nonce {
		return domain.ErrWorkspaceMismatch
	}
	if !verifyAuth(r.workspaceKey, r.workspaceID, resp.Nonce, resp.Tag) {
		return domain.ErrInvalidAuthTag
	}

	r.mu.Lock()
	r.authenticated[peerID] = struct{}{}
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()
	r.emitStatus()
	return nil
}

func (r *libp2pRuntime) handleAuth(stream network.Stream) {
	defer stream.Close()
	decoder := json.NewDecoder(stream)
	encoder := json.NewEncoder(stream)
	req := authRequest{}
	if err := decoder.Decode(&req); err != nil {
		return
	}
	if req.WorkspaceID != r.workspaceID {
		return
	}
	if !verifyAuth(r.workspaceKey, req.WorkspaceID, req.Nonce, req.Tag) {
		return
	}
	peerID := stream.Conn().RemotePeer()
	r.mu.Lock()
	r.authenticated[peerID] = struct{}{}
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()
	r.emitStatus()

	_ = encoder.Encode(authResponse{
		WorkspaceID: r.workspaceID,
		Nonce:       req.Nonce,
		Tag:         signAuth(r.workspaceKey, r.workspaceID, req.Nonce),
		NodeID:      r.host.ID().String(),
	})
}

func (r *libp2pRuntime) handleOp(stream network.Stream) {
	defer stream.Close()
	if !r.isAuthenticated(stream.Conn().RemotePeer()) {
		return
	}
	op := domain.OpEnvelope{}
	if err := json.NewDecoder(stream).Decode(&op); err != nil {
		return
	}
	if r.handlers.OnOp != nil {
		r.handlers.OnOp(op)
	}
	r.markSynced()
}

func (r *libp2pRuntime) handleSync(stream network.Stream) {
	defer stream.Close()
	if !r.isAuthenticated(stream.Conn().RemotePeer()) {
		return
	}
	ops := []domain.OpEnvelope{}
	if err := json.NewDecoder(stream).Decode(&ops); err != nil {
		return
	}
	if r.handlers.OnOp != nil {
		for _, op := range ops {
			r.handlers.OnOp(op)
		}
	}
	r.markSynced()
}

func (r *libp2pRuntime) writeMessage(ctx context.Context, id peer.ID, p protocol.ID, payload []byte) error {
	stream, err := r.host.NewStream(ctx, id, p)
	if err != nil {
		return err
	}
	defer stream.Close()
	_, err = stream.Write(payload)
	return err
}

func (r *libp2pRuntime) authenticatedPeers() []peer.ID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]peer.ID, 0, len(r.authenticated))
	for id := range r.authenticated {
		out = append(out, id)
	}
	return out
}

func (r *libp2pRuntime) isAuthenticated(id peer.ID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.authenticated[id]
	return ok
}

func (r *libp2pRuntime) markSynced() {
	r.mu.Lock()
	r.status.LastSyncAt = time.Now().UTC()
	r.mu.Unlock()
	r.emitStatus()
}

func (r *libp2pRuntime) emitStatus() {
	if r.handlers.OnStatus == nil {
		return
	}
	r.handlers.OnStatus(r.Status())
}

func renderListenAddrs(h host.Host) []string {
	out := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		full := addr.Encapsulate(multiaddr.StringCast("/p2p/" + h.ID().String()))
		out = append(out, full.String())
	}
	return out
}

func signAuth(key []byte, workspaceID, nonce string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(workspaceID + "|" + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyAuth(key []byte, workspaceID, nonce, tag string) bool {
	given, err := hex.DecodeString(tag)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(signAuth(key, workspaceID, nonce))
	if err != nil {
		return false
	}
	return hmac.Equal(given, want)
}
