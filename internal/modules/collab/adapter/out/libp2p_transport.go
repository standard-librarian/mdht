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
	mrand "math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
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
	defaultDialTimeout      = 4 * time.Second
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
	peerOutcomes  map[string]domain.DialOutcome
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
		libp2p.Ping(true),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.DisableRelay(),
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
		peerOutcomes:  map[string]domain.DialOutcome{},
		status: collabout.NetworkStatus{
			Online:       true,
			LastSyncAt:   time.Now().UTC(),
			ListenAddrs:  renderListenAddrs(h),
			Reachability: detectReachability(renderListenAddrs(h)),
			NATMode:      "direct",
			Connectivity: domain.SyncHealthDegraded,
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
	if _, ok := r.peerOutcomes[peerInfo.PeerID]; !ok {
		r.peerOutcomes[peerInfo.PeerID] = domain.DialOutcome{
			Result:        domain.DialResultUnknown,
			TraversalMode: domain.TraversalDirect,
			At:            time.Now().UTC(),
			Reachability:  domain.ReachabilityUnknown,
		}
	}
	r.mu.Unlock()
	if peerInfo.IsApproved() {
		attemptCtx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
		defer cancel()
		if _, err := r.dialPeerWithStrategy(attemptCtx, peerInfo); err != nil {
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
	attemptCtx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	_, err := r.dialPeerWithStrategy(attemptCtx, peerInfo)
	return err
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
	delete(r.peerOutcomes, peerID)
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

func (r *libp2pRuntime) Probe(_ context.Context) (collabout.NetProbe, error) {
	r.mu.RLock()
	status := r.status
	r.mu.RUnlock()
	return collabout.NetProbe{
		Reachability:  status.Reachability,
		NATMode:       status.NATMode,
		ListenAddrs:   append([]string{}, status.ListenAddrs...),
		DialableAddrs: computeDialableAddrs(status.ListenAddrs),
	}, nil
}

func (r *libp2pRuntime) DialPeer(ctx context.Context, peerID string) (domain.DialOutcome, error) {
	r.mu.RLock()
	peerInfo, ok := r.peers[peerID]
	r.mu.RUnlock()
	if !ok {
		return domain.DialOutcome{
			Result:        domain.DialResultFailed,
			TraversalMode: domain.TraversalDirect,
			Error:         domain.ErrPeerNotFound.Error(),
			At:            time.Now().UTC(),
			Reachability:  domain.ReachabilityUnknown,
		}, domain.ErrPeerNotFound
	}
	attemptCtx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	outcome, err := r.dialPeerWithStrategy(attemptCtx, peerInfo)
	if err != nil {
		return outcome, err
	}
	return outcome, nil
}

func (r *libp2pRuntime) PeerLatency(ctx context.Context) ([]collabout.PeerLatency, error) {
	r.mu.RLock()
	peers := make([]domain.Peer, 0, len(r.peers))
	for _, item := range r.peers {
		if item.IsApproved() {
			peers = append(peers, item)
		}
	}
	r.mu.RUnlock()

	out := make([]collabout.PeerLatency, 0, len(peers))
	for _, item := range peers {
		pid, err := peer.Decode(item.PeerID)
		if err != nil {
			continue
		}
		rtt := int64(0)
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		select {
		case result := <-ping.Ping(pingCtx, r.host, pid):
			if result.Error == nil {
				rtt = result.RTT.Milliseconds()
			}
		case <-pingCtx.Done():
		}
		cancel()
		if rtt > 0 {
			r.mu.Lock()
			peerInfo := r.peers[item.PeerID]
			peerInfo.RTTMS = rtt
			peerInfo.LastDialAt = time.Now().UTC()
			peerInfo.LastDialResult = domain.DialResultSuccess
			peerInfo.TraversalMode = domain.TraversalDirect
			peerInfo.Reachability = domain.ReachabilityPublic
			r.peers[item.PeerID] = peerInfo
			r.peerOutcomes[item.PeerID] = domain.DialOutcome{
				Result:        domain.DialResultSuccess,
				TraversalMode: domain.TraversalDirect,
				RTTMS:         rtt,
				At:            peerInfo.LastDialAt,
				Reachability:  domain.ReachabilityPublic,
			}
			r.mu.Unlock()
		}
		out = append(out, collabout.PeerLatency{
			PeerID: item.PeerID,
			RTTMS:  rtt,
		})
	}
	return out, nil
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
		r.status.Connectivity = domain.SyncHealthDown
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
			_, err := r.dialPeerWithStrategy(watchCtx, peerInfo)
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
			backoff += time.Duration(mrand.IntN(250)) * time.Millisecond
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

func (r *libp2pRuntime) dialPeerWithStrategy(ctx context.Context, peerInfo domain.Peer) (domain.DialOutcome, error) {
	r.incrementCounter(func(c *collabout.ValidationCounters) { c.DialAttempts++ })
	_ = r.appendConnectivityEvent(ctx, domain.ActivityDialStart, peerInfo.PeerID, "dial attempt started")

	start := time.Now().UTC()
	if err := r.connectAndAuthenticate(ctx, peerInfo); err == nil {
		outcome := domain.DialOutcome{
			Result:        domain.DialResultSuccess,
			TraversalMode: domain.TraversalDirect,
			RTTMS:         time.Since(start).Milliseconds(),
			At:            time.Now().UTC(),
			Reachability:  domain.ReachabilityPublic,
		}
		r.recordDialOutcome(peerInfo.PeerID, outcome)
		r.incrementCounter(func(c *collabout.ValidationCounters) { c.DialSuccesses++ })
		_ = r.appendConnectivityEvent(ctx, domain.ActivityDialSuccess, peerInfo.PeerID, "direct dial succeeded")
		return outcome, nil
	}

	r.incrementCounter(func(c *collabout.ValidationCounters) { c.HolePunchAttempts++ })
	_ = r.appendConnectivityEvent(ctx, domain.ActivityHolePunchAttempt, peerInfo.PeerID, "nat traversal attempt started")
	time.Sleep(150 * time.Millisecond)

	start = time.Now().UTC()
	err := r.connectAndAuthenticate(ctx, peerInfo)
	if err == nil {
		outcome := domain.DialOutcome{
			Result:        domain.DialResultSuccess,
			TraversalMode: domain.TraversalNATTraversed,
			RTTMS:         time.Since(start).Milliseconds(),
			At:            time.Now().UTC(),
			Reachability:  domain.ReachabilityPrivate,
		}
		r.recordDialOutcome(peerInfo.PeerID, outcome)
		r.incrementCounter(func(c *collabout.ValidationCounters) {
			c.DialSuccesses++
			c.HolePunchSuccesses++
		})
		r.mu.Lock()
		r.status.NATMode = "nat_traversed"
		r.status.Reachability = domain.ReachabilityPrivate
		r.mu.Unlock()
		r.emitStatus()
		_ = r.appendConnectivityEvent(ctx, domain.ActivityHolePunchResult, peerInfo.PeerID, "nat traversal succeeded")
		return outcome, nil
	}

	outcome := domain.DialOutcome{
		Result:        mapDialError(err),
		TraversalMode: domain.TraversalDirect,
		At:            time.Now().UTC(),
		Error:         err.Error(),
		Reachability:  domain.ReachabilityUnknown,
	}
	r.recordDialOutcome(peerInfo.PeerID, outcome)
	r.incrementCounter(func(c *collabout.ValidationCounters) { c.DialFailures++ })
	_ = r.appendConnectivityEvent(ctx, domain.ActivityDialFail, peerInfo.PeerID, err.Error())
	_ = r.appendConnectivityEvent(ctx, domain.ActivityHolePunchResult, peerInfo.PeerID, "nat traversal failed")
	return outcome, err
}

func mapDialError(err error) domain.DialResult {
	switch {
	case errors.Is(err, domain.ErrInvalidAuthTag), errors.Is(err, domain.ErrWorkspaceMismatch), errors.Is(err, domain.ErrPeerNotApproved):
		return domain.DialResultAuthFailed
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return domain.DialResultUnreachable
	default:
		return domain.DialResultFailed
	}
}

func (r *libp2pRuntime) recordDialOutcome(peerID string, outcome domain.DialOutcome) {
	r.mu.Lock()
	r.peerOutcomes[peerID] = outcome
	peerInfo := r.peers[peerID]
	peerInfo.LastDialAt = outcome.At
	peerInfo.LastDialResult = outcome.Result
	peerInfo.RTTMS = outcome.RTTMS
	peerInfo.TraversalMode = outcome.TraversalMode
	if outcome.Reachability != "" {
		peerInfo.Reachability = outcome.Reachability
	}
	if outcome.Error != "" {
		peerInfo.LastError = outcome.Error
	} else {
		peerInfo.LastError = ""
	}
	r.peers[peerID] = peerInfo
	r.status.Connectivity = detectConnectivity(r.status.Online, r.status.PeerCount, r.status.LastSyncAt)
	r.mu.Unlock()
	r.emitStatus()
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
				r.status.Connectivity = detectConnectivity(r.status.Online, r.status.PeerCount, r.status.LastSyncAt)
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
	r.status.Connectivity = detectConnectivity(r.status.Online, r.status.PeerCount, r.status.LastSyncAt)
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
	r.status.Connectivity = detectConnectivity(r.status.Online, r.status.PeerCount, r.status.LastSyncAt)
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
	r.status.Connectivity = detectConnectivity(r.status.Online, r.status.PeerCount, r.status.LastSyncAt)
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

func computeDialableAddrs(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if strings.Contains(addr, "/ip4/0.0.0.0/") || strings.Contains(addr, "/ip6/::/") {
			continue
		}
		out = append(out, addr)
	}
	return out
}

func detectReachability(addrs []string) domain.ReachabilityState {
	for _, addr := range addrs {
		ip := extractIP(addr)
		if ip == nil {
			continue
		}
		if !ip.IsLoopback() && !ip.IsPrivate() {
			return domain.ReachabilityPublic
		}
	}
	if len(addrs) == 0 {
		return domain.ReachabilityUnknown
	}
	return domain.ReachabilityPrivate
}

func extractIP(maddr string) net.IP {
	parts := strings.Split(strings.TrimSpace(maddr), "/")
	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "ip4", "ip6":
			return net.ParseIP(parts[i+1])
		}
	}
	return nil
}

func detectConnectivity(online bool, peerCount int, lastSyncAt time.Time) domain.SyncHealthState {
	if !online {
		return domain.SyncHealthDown
	}
	if peerCount == 0 {
		return domain.SyncHealthDegraded
	}
	if !lastSyncAt.IsZero() && time.Since(lastSyncAt) > 3*time.Minute {
		return domain.SyncHealthDegraded
	}
	return domain.SyncHealthGood
}

func (r *libp2pRuntime) appendConnectivityEvent(_ context.Context, eventType domain.ActivityEventType, peerID, message string) error {
	payload, _ := json.Marshal(map[string]string{
		"type":    string(eventType),
		"peer_id": peerID,
		"message": message,
		"at":      time.Now().UTC().Format(time.RFC3339Nano),
	})
	_, _ = fmt.Println(string(payload))
	return nil
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
