package out

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	initialReconnectBackoff = 500 * time.Millisecond
	maxReconnectBackoff     = 15 * time.Second
	steadyPeerCheckInterval = 5 * time.Second
)

type Libp2pTransport struct{}

type libp2pRuntime struct {
	host        host.Host
	workspaceID string
	keyRing     domain.KeyRing
	handlers    collabout.TransportHandlers

	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.RWMutex
	peers         map[string]domain.Peer
	authenticated map[peer.ID]struct{}
	watchers      map[string]context.CancelFunc
	status        collabout.NetworkStatus
	stopOnce      sync.Once
}

type authRequest struct {
	WorkspaceID string `json:"workspace_id"`
	KeyID       string `json:"key_id"`
	Nonce       string `json:"nonce"`
	Tag         string `json:"tag"`
}

type authResponse struct {
	WorkspaceID string `json:"workspace_id"`
	KeyID       string `json:"key_id"`
	Nonce       string `json:"nonce"`
	Tag         string `json:"tag"`
	NodeID      string `json:"node_id"`
	ErrorCode   string `json:"error_code,omitempty"`
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

	runCtx, cancel := context.WithCancel(ctx)
	r := &libp2pRuntime{
		host:          h,
		workspaceID:   input.WorkspaceID,
		keyRing:       input.KeyRing,
		handlers:      handlers,
		ctx:           runCtx,
		cancel:        cancel,
		peers:         map[string]domain.Peer{},
		authenticated: map[peer.ID]struct{}{},
		watchers:      map[string]context.CancelFunc{},
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
		if configuredPeer.IsApproved() {
			r.startPeerWatcher(configuredPeer)
		} else {
			r.mu.Lock()
			r.peers[configuredPeer.PeerID] = configuredPeer
			r.mu.Unlock()
		}
	}
	r.emitStatus()

	go func() {
		<-runCtx.Done()
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
			r.incrementCounter(func(c *collabout.ValidationCounters) { c.BroadcastSendErrors++ })
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
			r.incrementCounter(func(c *collabout.ValidationCounters) { c.ReconcileSendErrors++ })
			continue
		}
	}
	r.markSynced()
	return nil
}

func (r *libp2pRuntime) AddPeer(ctx context.Context, peerInfo domain.Peer) error {
	addr, err := multiaddr.NewMultiaddr(peerInfo.Address)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidPeerAddress, err)
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidPeerAddress, err)
	}
	peerInfo.PeerID = info.ID.String()
	r.mu.Lock()
	r.peers[peerInfo.PeerID] = peerInfo
	r.mu.Unlock()
	if peerInfo.IsApproved() {
		attemptCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		if err := r.connectAndAuthenticate(attemptCtx, peerInfo); err != nil {
			return err
		}
		r.startPeerWatcher(peerInfo)
	}
	return nil
}

func (r *libp2pRuntime) ApprovePeer(ctx context.Context, peerInfo domain.Peer) error {
	r.mu.Lock()
	r.peers[peerInfo.PeerID] = peerInfo
	r.mu.Unlock()
	r.startPeerWatcher(peerInfo)
	attemptCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return r.connectAndAuthenticate(attemptCtx, peerInfo)
}

func (r *libp2pRuntime) RevokePeer(_ context.Context, peerID string) error {
	r.mu.Lock()
	if peerInfo, ok := r.peers[peerID]; ok {
		peerInfo.State = domain.PeerStateRevoked
		r.peers[peerID] = peerInfo
	}
	r.mu.Unlock()
	return r.RemovePeer(context.Background(), peerID)
}

func (r *libp2pRuntime) RemovePeer(_ context.Context, peerID string) error {
	r.mu.Lock()
	cancel, ok := r.watchers[peerID]
	if ok {
		cancel()
		delete(r.watchers, peerID)
	}
	delete(r.peers, peerID)
	pid, decodeErr := peer.Decode(peerID)
	if decodeErr == nil {
		delete(r.authenticated, pid)
	}
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()

	if decodeErr == nil {
		_ = r.host.Network().ClosePeer(pid)
	}
	r.emitStatus()
	return nil
}

func (r *libp2pRuntime) UpdateKeyRing(_ context.Context, ring domain.KeyRing) error {
	r.mu.Lock()
	r.keyRing = ring
	r.mu.Unlock()
	return nil
}

func (r *libp2pRuntime) Status() collabout.NetworkStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

func (r *libp2pRuntime) Stop() error {
	var stopErr error
	r.stopOnce.Do(func() {
		r.cancel()
		r.mu.Lock()
		for _, cancel := range r.watchers {
			cancel()
		}
		r.watchers = map[string]context.CancelFunc{}
		r.status.Online = false
		r.status.PeerCount = 0
		r.mu.Unlock()
		r.emitStatus()
		stopErr = r.host.Close()
	})
	return stopErr
}

func (r *libp2pRuntime) startPeerWatcher(peerInfo domain.Peer) {
	r.mu.Lock()
	if existingCancel, exists := r.watchers[peerInfo.PeerID]; exists {
		existingCancel()
	}
	watchCtx, cancel := context.WithCancel(r.ctx)
	r.watchers[peerInfo.PeerID] = cancel
	r.peers[peerInfo.PeerID] = peerInfo
	r.mu.Unlock()

	go func() {
		backoff := initialReconnectBackoff
		for {
			select {
			case <-watchCtx.Done():
				return
			default:
			}

			r.incrementCounter(func(c *collabout.ValidationCounters) { c.ReconnectAttempts++ })
			err := r.connectAndAuthenticate(watchCtx, peerInfo)
			if err == nil {
				r.incrementCounter(func(c *collabout.ValidationCounters) { c.ReconnectSuccesses++ })
				backoff = initialReconnectBackoff
				if !r.waitUntilDisconnected(watchCtx, peerInfo.PeerID) {
					return
				}
				continue
			}

			select {
			case <-time.After(backoff):
			case <-watchCtx.Done():
				return
			}
			backoff *= 2
			if backoff > maxReconnectBackoff {
				backoff = maxReconnectBackoff
			}
		}
	}()
}

func (r *libp2pRuntime) connectAndAuthenticate(ctx context.Context, peerInfo domain.Peer) error {
	addr, err := multiaddr.NewMultiaddr(peerInfo.Address)
	if err != nil {
		return err
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}
	r.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	if err := r.host.Connect(ctx, *info); err != nil {
		return err
	}
	if err := r.authenticatePeer(ctx, info.ID); err != nil {
		_ = r.host.Network().ClosePeer(info.ID)
		return err
	}
	return nil
}

func (r *libp2pRuntime) waitUntilDisconnected(ctx context.Context, peerID string) bool {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return true
	}
	ticker := time.NewTicker(steadyPeerCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if r.host.Network().Connectedness(pid) != network.Connected {
				r.mu.Lock()
				delete(r.authenticated, pid)
				r.status.PeerCount = len(r.authenticated)
				r.mu.Unlock()
				r.emitStatus()
				return true
			}
		}
	}
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
	keyID, key, err := r.activeKey()
	if err != nil {
		return err
	}
	req := authRequest{WorkspaceID: r.workspaceID, KeyID: keyID, Nonce: nonce, Tag: signAuth(key, r.workspaceID, nonce)}
	if err := encoder.Encode(req); err != nil {
		return err
	}
	resp := authResponse{}
	if err := decoder.Decode(&resp); err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
		if errors.Is(err, io.EOF) {
			return domain.ErrInvalidAuthTag
		}
		return err
	}
	if resp.ErrorCode != "" {
		return errorFromCode(resp.ErrorCode)
	}
	if resp.WorkspaceID != r.workspaceID || resp.Nonce != nonce {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.WorkspaceMismatch++ })
		return domain.ErrWorkspaceMismatch
	}
	key, err = r.resolveKey(resp.KeyID)
	if err != nil || !verifyAuth(key, r.workspaceID, resp.Nonce, resp.Tag) {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.InvalidAuthTag++ })
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
	writeErr := func(code domain.ErrorCode) {
		_ = encoder.Encode(authResponse{
			WorkspaceID: r.workspaceID,
			ErrorCode:   string(code),
		})
	}
	req := authRequest{}
	if err := decoder.Decode(&req); err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
		writeErr(domain.ErrorAuthInvalid)
		return
	}
	if req.WorkspaceID != r.workspaceID {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.WorkspaceMismatch++ })
		writeErr(domain.ErrorWorkspaceMismatch)
		return
	}
	if !r.isApprovedPeer(stream.Conn().RemotePeer().String()) {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.UnauthenticatedPeer++ })
		writeErr(domain.ErrorPeerNotApproved)
		return
	}
	key, err := r.resolveKey(req.KeyID)
	if err != nil || !verifyAuth(key, req.WorkspaceID, req.Nonce, req.Tag) {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.InvalidAuthTag++ })
		writeErr(domain.ErrorAuthInvalid)
		return
	}
	peerID := stream.Conn().RemotePeer()
	r.mu.Lock()
	r.authenticated[peerID] = struct{}{}
	r.status.PeerCount = len(r.authenticated)
	r.mu.Unlock()
	r.emitStatus()

	keyID, activeKey, err := r.activeKey()
	if err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
		return
	}
	if err := encoder.Encode(authResponse{
		WorkspaceID: r.workspaceID,
		KeyID:       keyID,
		Nonce:       req.Nonce,
		Tag:         signAuth(activeKey, r.workspaceID, req.Nonce),
		NodeID:      r.host.ID().String(),
	}); err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
		return
	}
}

func (r *libp2pRuntime) isApprovedPeer(peerID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.peers[peerID]
	if !ok {
		return false
	}
	return item.IsApproved()
}

func (r *libp2pRuntime) resolveKey(keyID string) ([]byte, error) {
	r.mu.RLock()
	ring := r.keyRing
	r.mu.RUnlock()
	return ring.ResolveKey(keyID, time.Now().UTC())
}

func (r *libp2pRuntime) activeKey() (string, []byte, error) {
	r.mu.RLock()
	ring := r.keyRing
	r.mu.RUnlock()
	key, err := ring.ResolveKey(ring.ActiveKeyID, time.Now().UTC())
	if err != nil {
		return "", nil, err
	}
	return ring.ActiveKeyID, key, nil
}

func (r *libp2pRuntime) handleOp(stream network.Stream) {
	defer stream.Close()
	if !r.isAuthenticated(stream.Conn().RemotePeer()) {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.UnauthenticatedPeer++ })
		return
	}
	op := domain.OpEnvelope{}
	if err := json.NewDecoder(stream).Decode(&op); err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
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
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.UnauthenticatedPeer++ })
		return
	}
	ops := []domain.OpEnvelope{}
	if err := json.NewDecoder(stream).Decode(&ops); err != nil {
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DecodeErrors++ })
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
	if _, err := stream.Write(payload); err != nil {
		return err
	}
	return nil
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

func (r *libp2pRuntime) incrementCounter(update func(c *collabout.ValidationCounters)) {
	r.mu.Lock()
	update(&r.status.Counters)
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

func errorFromCode(code string) error {
	switch domain.ErrorCode(code) {
	case domain.ErrorPeerNotApproved:
		return domain.ErrPeerNotApproved
	case domain.ErrorAuthInvalid:
		return domain.ErrInvalidAuthTag
	case domain.ErrorWorkspaceMismatch:
		return domain.ErrWorkspaceMismatch
	default:
		return domain.ErrInvalidAuthTag
	}
}
