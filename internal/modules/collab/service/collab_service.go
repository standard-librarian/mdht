package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

const (
	reconcileInterval   = 15 * time.Second
	daemonStartTimeout  = 5 * time.Second
	defaultLogTailLines = 200
	syncLagDegraded     = 2 * time.Minute
	syncLagDown         = 10 * time.Minute
	pendingOpsDegraded  = 10
	pendingOpsDown      = 100
)

type runtimeState struct {
	workspace    domain.Workspace
	keyRing      domain.KeyRing
	node         domain.NodeIdentity
	state        domain.CRDTState
	transport    collabout.RuntimeTransport
	status       collabout.NetworkStatus
	cancel       context.CancelFunc
	metricsSrv   *http.Server
	metricsLn    net.Listener
	metricsAddr  string
	conflictOpen int
}

type CollabService struct {
	vaultPath string
	workspace collabout.WorkspaceStore
	peers     collabout.PeerStore
	oplog     collabout.OpLogStore
	snapshot  collabout.SnapshotStore
	extractor collabout.ProjectionExtractor
	applier   collabout.ProjectionApplier
	transport collabout.Transport
	daemon    collabout.DaemonStore
	ipcServer collabout.IPCServer
	ipcClient collabout.IPCClient
	activity  collabout.ActivityStore
	conflicts collabout.ConflictStore

	mu      sync.RWMutex
	runtime *runtimeState
}

func NewCollabService(
	vaultPath string,
	workspace collabout.WorkspaceStore,
	peers collabout.PeerStore,
	oplog collabout.OpLogStore,
	snapshot collabout.SnapshotStore,
	extractor collabout.ProjectionExtractor,
	applier collabout.ProjectionApplier,
	transport collabout.Transport,
	daemon collabout.DaemonStore,
	ipcServer collabout.IPCServer,
	ipcClient collabout.IPCClient,
	activity collabout.ActivityStore,
	conflicts collabout.ConflictStore,
) *CollabService {
	return &CollabService{
		vaultPath: vaultPath,
		workspace: workspace,
		peers:     peers,
		oplog:     oplog,
		snapshot:  snapshot,
		extractor: extractor,
		applier:   applier,
		transport: transport,
		daemon:    daemon,
		ipcServer: ipcServer,
		ipcClient: ipcClient,
		activity:  activity,
		conflicts: conflicts,
	}
}

func (s *CollabService) RunDaemon(ctx context.Context) error {
	if err := s.cleanupStaleArtifacts(ctx); err != nil {
		return err
	}
	if err := s.migrateIfNeeded(ctx); err != nil {
		return err
	}

	workspace, ring, node, err := s.workspace.Load(ctx)
	if err != nil {
		return err
	}
	peers, err := s.peers.List(ctx)
	if err != nil {
		return err
	}
	state, err := s.materializeState(ctx)
	if err != nil {
		return err
	}
	conflicts, err := s.conflicts.List(ctx, "")
	if err != nil {
		return err
	}
	openConflicts := 0
	for _, item := range conflicts {
		if item.Status == domain.ConflictStatusOpen {
			openConflicts++
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	rt, err := s.transport.Start(runCtx, collabout.TransportStartInput{
		WorkspaceID:  workspace.ID,
		KeyRing:      ring,
		NodeIdentity: node,
		Peers:        peers,
	}, collabout.TransportHandlers{
		OnOp: func(op domain.OpEnvelope) {
			_ = s.ingestRemoteOp(context.Background(), op)
		},
		OnStatus: func(status collabout.NetworkStatus) {
			s.mu.Lock()
			if s.runtime != nil {
				s.runtime.status = status
			}
			s.mu.Unlock()
		},
	})
	if err != nil {
		cancel()
		return err
	}

	s.mu.Lock()
	s.runtime = &runtimeState{
		workspace:    workspace,
		keyRing:      ring,
		node:         node,
		state:        state,
		transport:    rt,
		status:       rt.Status(),
		cancel:       cancel,
		conflictOpen: openConflicts,
	}
	s.mu.Unlock()

	if err := s.startMetricsEndpoint(); err != nil {
		cancel()
		_ = rt.Stop()
		return err
	}

	if err := s.daemon.WritePID(ctx, os.Getpid()); err != nil {
		cancel()
		_ = rt.Stop()
		return err
	}

	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityMigration,
		Message: "collab daemon started",
	})

	ipcErr := make(chan error, 1)
	go func() {
		if s.ipcServer == nil {
			ipcErr <- fmt.Errorf("ipc server is not configured")
			return
		}
		ipcErr <- s.ipcServer.Serve(runCtx, s.daemon.SocketPath(), s)
	}()

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-runCtx.Done():
			s.cleanupRuntime(context.Background())
			return nil
		case err := <-ipcErr:
			if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
				s.cleanupRuntime(context.Background())
				return err
			}
			s.cleanupRuntime(context.Background())
			return nil
		case <-ticker.C:
			s.syncTick(context.Background())
		}
	}
}

func (s *CollabService) StartDaemon(ctx context.Context) error {
	if err := s.cleanupStaleArtifacts(ctx); err != nil {
		return err
	}
	status, err := s.DaemonStatus(ctx)
	if err == nil && status.Running {
		if socketReachable(s.daemon.SocketPath()) {
			return nil
		}
		return fmt.Errorf("%w: daemon process is alive but socket is unavailable", domain.ErrDaemonStartFailed)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.daemon.LogPath()), 0o755); err != nil {
		return fmt.Errorf("create daemon log dir: %w", err)
	}
	if err := os.Remove(s.daemon.SocketPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale daemon socket: %w", err)
	}

	logFile, err := os.OpenFile(s.daemon.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(execPath, "collab", "daemon", "__run", "--vault", s.vaultPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	if err := s.daemon.WritePID(ctx, cmd.Process.Pid); err != nil {
		return err
	}
	_ = cmd.Process.Release()

	if err := waitForSocket(s.daemon.SocketPath(), daemonStartTimeout); err != nil {
		_ = s.daemon.ClearPID(ctx)
		return fmt.Errorf("%w: %v", domain.ErrDaemonStartFailed, err)
	}
	return nil
}

func (s *CollabService) StopDaemon(ctx context.Context) error {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.cancel != nil {
		rt.cancel()
		return nil
	}

	if s.ipcClient != nil {
		_ = s.ipcClient.Stop(ctx, s.daemon.SocketPath())
	}

	pid, err := s.daemon.ReadPID(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(s.daemon.SocketPath())
			return nil
		}
		return err
	}
	if pid <= 0 {
		_ = s.daemon.ClearPID(ctx)
		_ = os.Remove(s.daemon.SocketPath())
		return nil
	}
	if !processAlive(pid) {
		_ = s.daemon.ClearPID(ctx)
		_ = os.Remove(s.daemon.SocketPath())
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop daemon pid=%d: %w", pid, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if processAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if err := s.daemon.ClearPID(ctx); err != nil {
		return err
	}
	_ = os.Remove(s.daemon.SocketPath())
	return nil
}

func (s *CollabService) DaemonStatus(ctx context.Context) (collabout.DaemonRuntimeStatus, error) {
	out := collabout.DaemonRuntimeStatus{SocketPath: s.daemon.SocketPath()}

	pid, err := s.daemon.ReadPID(ctx)
	if err == nil {
		out.PID = pid
		out.Running = processAlive(pid)
	}

	if out.Running && s.ipcClient != nil {
		status, statusErr := s.ipcClient.Status(ctx, s.daemon.SocketPath())
		if statusErr == nil {
			out.Status = status
		}
	}
	return out, nil
}

func (s *CollabService) WorkspaceInit(ctx context.Context, name string) (domain.Workspace, error) {
	workspace, _, _, err := s.workspace.Init(ctx, name)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (s *CollabService) WorkspaceShow(ctx context.Context) (domain.Workspace, string, []domain.Peer, error) {
	workspace, _, node, err := s.workspace.Load(ctx)
	if err != nil {
		return domain.Workspace{}, "", nil, err
	}
	peers, err := s.peers.List(ctx)
	if err != nil {
		return domain.Workspace{}, "", nil, err
	}
	return workspace, node.NodeID, peers, nil
}

func (s *CollabService) WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (domain.Workspace, error) {
	ring, err := s.workspace.RotateKey(ctx, gracePeriod)
	if err != nil {
		return domain.Workspace{}, err
	}
	s.mu.Lock()
	if s.runtime != nil {
		s.runtime.keyRing = ring
		if s.runtime.transport != nil {
			_ = s.runtime.transport.UpdateKeyRing(ctx, ring)
		}
	}
	s.mu.Unlock()
	workspace, _, _, err := s.workspace.Load(ctx)
	if err != nil {
		return domain.Workspace{}, err
	}
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityKeyRotated,
		Message: "workspace key rotated",
		Fields: map[string]string{
			"active_key_id": ring.ActiveKeyID,
		},
	})
	return workspace, nil
}

func (s *CollabService) PeerAdd(ctx context.Context, addr, label string) (domain.Peer, error) {
	peer, err := s.peers.Add(ctx, addr, label)
	if err != nil {
		return domain.Peer{}, err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		if addErr := rt.transport.AddPeer(ctx, peer); addErr != nil {
			peer.LastError = addErr.Error()
			_, _ = s.peers.Add(ctx, peer.Address, peer.Label)
			return domain.Peer{}, addErr
		}
	}
	return peer, nil
}

func (s *CollabService) PeerApprove(ctx context.Context, peerID string) (domain.Peer, error) {
	peer, err := s.peers.Approve(ctx, peerID)
	if err != nil {
		return domain.Peer{}, err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		_ = rt.transport.ApprovePeer(ctx, peer)
	}
	return peer, nil
}

func (s *CollabService) PeerRevoke(ctx context.Context, peerID string) (domain.Peer, error) {
	peer, err := s.peers.Revoke(ctx, peerID)
	if err != nil {
		return domain.Peer{}, err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		_ = rt.transport.RevokePeer(ctx, peerID)
	}
	return peer, nil
}

func (s *CollabService) PeerDial(ctx context.Context, peerID string) (domain.Peer, error) {
	if strings.TrimSpace(peerID) == "" {
		return domain.Peer{}, domain.ErrPeerNotFound
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil {
		if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
			return s.ipcClient.PeerDial(ctx, s.daemon.SocketPath(), peerID)
		}
		return domain.Peer{}, domain.ErrDaemonNotRunning
	}
	peers, err := s.peers.List(ctx)
	if err != nil {
		return domain.Peer{}, err
	}
	var selected domain.Peer
	found := false
	for _, item := range peers {
		if item.PeerID == peerID {
			selected = item
			found = true
			break
		}
	}
	if !found {
		return domain.Peer{}, domain.ErrPeerNotFound
	}
	outcome, err := rt.transport.DialPeer(ctx, peerID)
	selected.LastDialAt = time.Now().UTC()
	selected.LastDialResult = outcome.Result
	selected.RTTMS = outcome.RTTMS
	if outcome.TraversalMode != "" {
		selected.TraversalMode = outcome.TraversalMode
	}
	if outcome.Reachability != "" {
		selected.Reachability = outcome.Reachability
	}
	if err != nil {
		selected.LastError = err.Error()
		_, _ = s.peers.Update(ctx, selected)
		_ = s.appendActivity(ctx, domain.ActivityEvent{
			Type:    domain.ActivityDialFail,
			Message: "peer dial failed",
			Fields: map[string]string{
				"peer_id": peerID,
				"error":   err.Error(),
			},
		})
		return selected, err
	}
	selected.LastError = ""
	updated, updateErr := s.peers.Update(ctx, selected)
	if updateErr == nil {
		selected = updated
	}
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityDialSuccess,
		Message: "peer dial succeeded",
		Fields: map[string]string{
			"peer_id":        peerID,
			"traversal_mode": string(selected.TraversalMode),
			"rtt_ms":         fmt.Sprintf("%d", selected.RTTMS),
		},
	})
	return selected, nil
}

func (s *CollabService) PeerLatency(ctx context.Context) ([]collabout.PeerLatency, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil {
		if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
			return s.ipcClient.PeerLatency(ctx, s.daemon.SocketPath())
		}
		return nil, domain.ErrDaemonNotRunning
	}
	latencies, err := rt.transport.PeerLatency(ctx)
	if err != nil {
		return nil, err
	}
	peers, _ := s.peers.List(ctx)
	byID := make(map[string]domain.Peer, len(peers))
	for _, item := range peers {
		byID[item.PeerID] = item
	}
	for _, item := range latencies {
		peer, ok := byID[item.PeerID]
		if !ok {
			continue
		}
		peer.RTTMS = item.RTTMS
		peer.LastDialAt = time.Now().UTC()
		peer.LastDialResult = domain.DialResultSuccess
		if peer.TraversalMode == "" {
			peer.TraversalMode = domain.TraversalDirect
		}
		if peer.Reachability == "" {
			peer.Reachability = domain.ReachabilityPublic
		}
		_, _ = s.peers.Update(ctx, peer)
	}
	return latencies, nil
}

func (s *CollabService) PeerRemove(ctx context.Context, peerID string) error {
	if err := s.peers.Remove(ctx, peerID); err != nil {
		return err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		_ = rt.transport.RemovePeer(ctx, peerID)
	}
	return nil
}

func (s *CollabService) PeerList(ctx context.Context) ([]domain.Peer, error) {
	return s.peers.List(ctx)
}

func (s *CollabService) Status(ctx context.Context) (collabout.DaemonStatus, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil {
		status := rt.status
		approved := 0
		if peers, err := s.peers.List(ctx); err == nil {
			for _, peer := range peers {
				if peer.IsApproved() {
					approved++
				}
			}
		}
		return collabout.DaemonStatus{
			Online:            status.Online,
			PeerCount:         status.PeerCount,
			ApprovedPeerCount: approved,
			PendingConflicts:  rt.conflictOpen,
			PendingOps:        rt.state.PendingOps,
			LastSyncAt:        status.LastSyncAt,
			NodeID:            rt.node.NodeID,
			WorkspaceID:       rt.workspace.ID,
			ListenAddrs:       status.ListenAddrs,
			Reachability:      status.Reachability,
			NATMode:           status.NATMode,
			Connectivity:      status.Connectivity,
			Counters:          status.Counters,
			MetricsAddress:    rt.metricsAddr,
		}, nil
	}

	if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		status, err := s.ipcClient.Status(ctx, s.daemon.SocketPath())
		if err == nil {
			return status, nil
		}
	}

	workspace, _, node, err := s.workspace.Load(ctx)
	if err != nil {
		return collabout.DaemonStatus{}, err
	}
	state, err := s.snapshot.Load(ctx)
	if err != nil {
		return collabout.DaemonStatus{}, err
	}
	peers, _ := s.peers.List(ctx)
	conflicts, _ := s.conflicts.List(ctx, "")
	openConflicts := 0
	for _, item := range conflicts {
		if item.Status == domain.ConflictStatusOpen {
			openConflicts++
		}
	}
	approved := 0
	for _, peer := range peers {
		if peer.IsApproved() {
			approved++
		}
	}
	return collabout.DaemonStatus{
		Online:            false,
		PeerCount:         len(peers),
		ApprovedPeerCount: approved,
		PendingConflicts:  openConflicts,
		PendingOps:        state.PendingOps,
		LastSyncAt:        state.LastSyncAt,
		NodeID:            node.NodeID,
		WorkspaceID:       workspace.ID,
		Reachability:      domain.ReachabilityUnknown,
		NATMode:           "direct",
		Connectivity:      domain.SyncHealthDown,
	}, nil
}

func (s *CollabService) NetStatus(ctx context.Context) (collabout.NetStatus, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return collabout.NetStatus{}, err
	}
	return collabout.NetStatus{
		Online:       status.Online,
		Reachability: status.Reachability,
		NATMode:      status.NATMode,
		Connectivity: status.Connectivity,
		ListenAddrs:  status.ListenAddrs,
		PeerCount:    status.PeerCount,
		LastSyncAt:   status.LastSyncAt,
	}, nil
}

func (s *CollabService) NetProbe(ctx context.Context) (collabout.NetProbe, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		return rt.transport.Probe(ctx)
	}
	if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		return s.ipcClient.NetProbe(ctx, s.daemon.SocketPath())
	}
	status, err := s.Status(ctx)
	if err != nil {
		return collabout.NetProbe{}, err
	}
	dialable := make([]string, 0, len(status.ListenAddrs))
	for _, addr := range status.ListenAddrs {
		if strings.Contains(addr, "/ip4/0.0.0.0/") || strings.Contains(addr, "/ip6/::/") {
			continue
		}
		dialable = append(dialable, addr)
	}
	return collabout.NetProbe{
		Reachability:  status.Reachability,
		NATMode:       status.NATMode,
		ListenAddrs:   status.ListenAddrs,
		DialableAddrs: dialable,
	}, nil
}

func (s *CollabService) DaemonLogs(_ context.Context, tail int) (string, error) {
	if tail <= 0 {
		tail = defaultLogTailLines
	}
	file, err := os.Open(s.daemon.LogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("open daemon log: %w", err)
	}
	defer file.Close()

	lines := make([]string, 0, tail)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(lines) < tail {
			lines = append(lines, line)
			continue
		}
		copy(lines, lines[1:])
		lines[len(lines)-1] = line
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("scan daemon log: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}

func (s *CollabService) ActivityTail(ctx context.Context, query collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil && s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		return s.ipcClient.ActivityTail(ctx, s.daemon.SocketPath(), query)
	}
	return s.activity.Tail(ctx, query)
}

func (s *CollabService) ConflictsList(ctx context.Context, entityKey string) ([]domain.ConflictRecord, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil && s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		return s.ipcClient.ConflictsList(ctx, s.daemon.SocketPath(), entityKey)
	}
	return s.conflicts.List(ctx, entityKey)
}

func (s *CollabService) ConflictResolve(ctx context.Context, conflictID string, strategy domain.ConflictStrategy) (domain.ConflictRecord, error) {
	if err := strategy.Validate(); err != nil {
		return domain.ConflictRecord{}, err
	}

	if s.ipcClient != nil {
		s.mu.RLock()
		rt := s.runtime
		s.mu.RUnlock()
		if rt == nil && socketReachable(s.daemon.SocketPath()) {
			return s.ipcClient.ConflictResolve(ctx, s.daemon.SocketPath(), conflictID, strategy)
		}
	}

	conflicts, err := s.conflicts.List(ctx, "")
	if err != nil {
		return domain.ConflictRecord{}, err
	}
	var record domain.ConflictRecord
	found := false
	for _, item := range conflicts {
		if item.ID == conflictID {
			record = item
			found = true
			break
		}
	}
	if !found {
		return domain.ConflictRecord{}, domain.ErrConflictNotFound
	}
	if record.Status == domain.ConflictStatusResolved {
		return record, nil
	}

	resolvedRaw := json.RawMessage(record.LocalValue)
	resolvedText := record.LocalValue
	switch strategy {
	case domain.ConflictStrategyLocal:
		resolvedRaw = json.RawMessage(record.LocalValue)
		resolvedText = record.LocalValue
	case domain.ConflictStrategyRemote:
		resolvedRaw = json.RawMessage(record.RemoteValue)
		resolvedText = record.RemoteValue
	case domain.ConflictStrategyMerge:
		localText := decodeJSONText(record.LocalValue)
		remoteText := decodeJSONText(record.RemoteValue)
		merged := strings.TrimSpace(localText)
		if merged == "" {
			merged = strings.TrimSpace(remoteText)
		} else if remote := strings.TrimSpace(remoteText); remote != "" && remote != merged {
			merged = merged + "\n" + remote
		}
		marshaled, err := json.Marshal(merged)
		if err != nil {
			return domain.ConflictRecord{}, err
		}
		resolvedRaw = marshaled
		resolvedText = merged
	}

	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil {
		payload, err := json.Marshal(domain.RegisterPayload{
			Field: record.Field,
			Value: resolvedRaw,
		})
		if err != nil {
			return domain.ConflictRecord{}, err
		}
		op := s.normalizeOp(rt, domain.OpEnvelope{
			EntityKey: record.EntityKey,
			OpKind:    domain.OpKindPutRegister,
			Payload:   payload,
		})
		if err := s.ingestLocalOp(ctx, op); err != nil {
			return domain.ConflictRecord{}, err
		}
		record.ResolutionOp = op.OpID
		if strategy == domain.ConflictStrategyLocal || strategy == domain.ConflictStrategyRemote {
			resolvedText = string(resolvedRaw)
		}
	}

	record.Status = domain.ConflictStatusResolved
	record.Strategy = string(strategy)
	record.ResolvedBy = "operator"
	record.ResolvedAt = time.Now().UTC()
	record.MergedValue = resolvedText
	if err := s.conflicts.Upsert(ctx, record); err != nil {
		return domain.ConflictRecord{}, err
	}
	s.mu.Lock()
	if s.runtime != nil && s.runtime.conflictOpen > 0 {
		s.runtime.conflictOpen--
	}
	s.mu.Unlock()
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityConflictSolved,
		Message: "conflict resolved",
		Fields: map[string]string{
			"conflict_id": conflictID,
			"strategy":    string(strategy),
		},
	})
	return record, nil
}

func decodeJSONText(raw string) string {
	var out string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out
	}
	return raw
}

func (s *CollabService) SyncNow(ctx context.Context) (int, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()

	if rt == nil {
		if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
			applied, err := s.ipcClient.SyncNow(ctx, s.daemon.SocketPath())
			if err == nil {
				return applied, nil
			}
		}
		return 0, domain.ErrDaemonNotRunning
	}
	return s.reconcileInProcess(ctx, rt)
}

func (s *CollabService) SyncHealth(ctx context.Context) (collabout.SyncHealth, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil {
		if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
			return s.ipcClient.SyncHealth(ctx, s.daemon.SocketPath())
		}
		return collabout.SyncHealth{
			State:  domain.SyncHealthDown,
			Reason: "daemon offline",
		}, nil
	}
	lag := int64(0)
	if !rt.status.LastSyncAt.IsZero() {
		lag = int64(time.Since(rt.status.LastSyncAt).Seconds())
	}
	health := collabout.SyncHealth{
		State:      domain.SyncHealthGood,
		Reason:     "healthy",
		LagSeconds: lag,
		PendingOps: rt.state.PendingOps,
		LastSyncAt: rt.status.LastSyncAt,
	}
	if !rt.status.Online {
		health.State = domain.SyncHealthDown
		health.Reason = "network offline"
		return health, nil
	}
	if rt.state.PendingOps >= pendingOpsDown || (!rt.status.LastSyncAt.IsZero() && time.Since(rt.status.LastSyncAt) >= syncLagDown) {
		health.State = domain.SyncHealthDown
		health.Reason = "sync lag too high"
		return health, nil
	}
	if rt.state.PendingOps >= pendingOpsDegraded || (!rt.status.LastSyncAt.IsZero() && time.Since(rt.status.LastSyncAt) >= syncLagDegraded) {
		health.State = domain.SyncHealthDegraded
		health.Reason = "sync degraded"
	}
	if rt.status.Counters.ReconnectAttempts > rt.status.Counters.ReconnectSuccesses+3 {
		health.State = domain.SyncHealthDegraded
		health.Reason = "frequent reconnect failures"
	}
	return health, nil
}

func (s *CollabService) SnapshotExport(ctx context.Context) (string, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil {
		payload, err := json.MarshalIndent(rt.state, "", "  ")
		if err != nil {
			return "", err
		}
		return string(payload), nil
	}
	if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		payload, err := s.ipcClient.SnapshotExport(ctx, s.daemon.SocketPath())
		if err == nil {
			return payload, nil
		}
	}
	state, err := s.snapshot.Load(ctx)
	if err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (s *CollabService) Metrics(ctx context.Context) (collabout.MetricsSnapshot, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil {
		return collabout.MetricsSnapshot{
			WorkspaceID:      rt.workspace.ID,
			NodeID:           rt.node.NodeID,
			ApprovedPeers:    rt.status.PeerCount,
			PendingConflicts: rt.conflictOpen,
			PendingOps:       rt.state.PendingOps,
			LastSyncAt:       rt.status.LastSyncAt,
			Counters:         rt.status.Counters,
			CollectedAt:      time.Now().UTC(),
		}, nil
	}
	if s.ipcClient != nil && socketReachable(s.daemon.SocketPath()) {
		return s.ipcClient.Metrics(ctx, s.daemon.SocketPath())
	}
	status, err := s.Status(ctx)
	if err != nil {
		return collabout.MetricsSnapshot{}, err
	}
	return collabout.MetricsSnapshot{
		WorkspaceID:      status.WorkspaceID,
		NodeID:           status.NodeID,
		ApprovedPeers:    status.ApprovedPeerCount,
		PendingConflicts: status.PendingConflicts,
		PendingOps:       status.PendingOps,
		LastSyncAt:       status.LastSyncAt,
		Counters:         status.Counters,
		CollectedAt:      time.Now().UTC(),
	}, nil
}

func (s *CollabService) Stop(ctx context.Context) error {
	return s.StopDaemon(ctx)
}

func (s *CollabService) ingestLocalOp(ctx context.Context, op domain.OpEnvelope) error {
	if err := s.ingest(ctx, op); err != nil {
		return err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		return rt.transport.Broadcast(ctx, op)
	}
	return nil
}

func (s *CollabService) ingestRemoteOp(ctx context.Context, op domain.OpEnvelope) error {
	return s.ingest(ctx, op)
}

func (s *CollabService) ingest(ctx context.Context, op domain.OpEnvelope) error {
	s.mu.Lock()
	rt := s.runtime
	if rt == nil {
		s.mu.Unlock()
		return domain.ErrWorkspaceNotInitialized
	}
	if op.WorkspaceID != rt.workspace.ID {
		rt.status.Counters.WorkspaceMismatch++
		s.mu.Unlock()
		return domain.ErrWorkspaceMismatch
	}
	key, keyErr := rt.keyRing.ResolveKey(op.WorkspaceKeyID, time.Now().UTC())
	if keyErr != nil || !op.Verify(key) {
		rt.status.Counters.InvalidAuthTag++
		s.mu.Unlock()
		return domain.ErrInvalidAuthTag
	}
	if _, exists := rt.state.AppliedOps[op.OpID]; exists {
		s.mu.Unlock()
		return nil
	}

	s.detectConflictLocked(ctx, rt, op)
	if err := rt.state.Apply(op); err != nil {
		s.mu.Unlock()
		return err
	}
	state := rt.state.Clone()
	s.mu.Unlock()

	if err := s.oplog.Append(ctx, op); err != nil {
		return err
	}
	if err := s.snapshot.Save(ctx, state); err != nil {
		return err
	}
	if err := s.applier.Apply(ctx, state); err != nil {
		return err
	}
	return nil
}

func (s *CollabService) detectConflictLocked(ctx context.Context, rt *runtimeState, op domain.OpEnvelope) {
	if op.OpKind != domain.OpKindPutRegister || op.NodeID == rt.node.NodeID {
		return
	}
	payload := domain.RegisterPayload{}
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return
	}
	entity := rt.state.EnsureEntity(op.EntityKey)
	current, ok := entity.Registers[payload.Field]
	if !ok {
		return
	}
	currentValue := strings.TrimSpace(string(current.Value))
	remoteValue := strings.TrimSpace(string(payload.Value))
	if currentValue == remoteValue {
		return
	}
	conflictID := hashHex(op.EntityKey + "|" + payload.Field + "|" + current.Meta.String() + "|" + op.HLCTimestamp)
	existing, _ := s.conflicts.List(ctx, op.EntityKey)
	for _, item := range existing {
		if item.ID == conflictID && item.Status == domain.ConflictStatusOpen {
			return
		}
	}
	record := domain.ConflictRecord{
		ID:            conflictID,
		EntityKey:     op.EntityKey,
		Field:         payload.Field,
		LocalValue:    currentValue,
		RemoteValue:   remoteValue,
		Status:        domain.ConflictStatusOpen,
		CreatedAt:     time.Now().UTC(),
		SourceNodeID:  op.NodeID,
		WorkspaceID:   rt.workspace.ID,
		WorkspaceKey:  op.WorkspaceKeyID,
		SchemaVersion: domain.SchemaVersionV2,
	}
	_ = s.conflicts.Upsert(ctx, record)
	rt.conflictOpen++
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityConflictCreated,
		Message: "register conflict detected",
		Fields: map[string]string{
			"conflict_id": record.ID,
			"entity_key":  record.EntityKey,
			"field":       record.Field,
		},
	})
}

func (s *CollabService) normalizeOp(rt *runtimeState, op domain.OpEnvelope) domain.OpEnvelope {
	op.WorkspaceID = rt.workspace.ID
	op.WorkspaceKeyID = rt.keyRing.ActiveKeyID
	op.NodeID = rt.node.NodeID
	op.PeerID = rt.node.NodeID
	if op.HLCTimestamp == "" {
		op.HLCTimestamp = domain.NextHLC(time.Now().UTC(), rt.state.LastApplied, rt.node.NodeID).String()
	}
	if op.OpID == "" {
		hash := sha256.Sum256([]byte(op.EntityKey + "|" + string(op.OpKind) + "|" + string(op.Payload) + "|" + op.HLCTimestamp + "|" + op.NodeID))
		op.OpID = hex.EncodeToString(hash[:])
	}
	op.SchemaVersion = domain.SchemaVersionV2
	if key, err := rt.keyRing.ResolveKey(rt.keyRing.ActiveKeyID, time.Now().UTC()); err == nil {
		return op.Signed(key)
	}
	return op
}

func (s *CollabService) materializeState(ctx context.Context) (domain.CRDTState, error) {
	state, err := s.snapshot.Load(ctx)
	if err != nil {
		return domain.CRDTState{}, err
	}
	ops, err := s.oplog.List(ctx)
	if err != nil {
		return domain.CRDTState{}, err
	}
	for _, op := range ops {
		_ = state.Apply(op)
	}
	if err := s.snapshot.Save(ctx, state); err != nil {
		return domain.CRDTState{}, err
	}
	if err := s.applier.Apply(ctx, state); err != nil {
		return domain.CRDTState{}, err
	}
	return state, nil
}

func (s *CollabService) reconcileInProcess(ctx context.Context, rt *runtimeState) (int, error) {
	ops, err := s.extractor.Extract(ctx, rt.workspace.ID, rt.node.NodeID, rt.keyRing.ActiveKeyID, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, op := range ops {
		normalized := s.normalizeOp(rt, op)
		if err := s.ingestLocalOp(ctx, normalized); err != nil {
			return applied, err
		}
		applied++
	}
	allOps, err := s.oplog.List(ctx)
	if err == nil && rt.transport != nil {
		_ = rt.transport.Reconcile(ctx, allOps)
	}
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivitySyncApplied,
		Message: "sync now completed",
		Fields: map[string]string{
			"applied": fmt.Sprintf("%d", applied),
		},
	})
	return applied, nil
}

func (s *CollabService) syncTick(ctx context.Context) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil {
		return
	}
	_, _ = s.reconcileInProcess(ctx, rt)
}

func (s *CollabService) cleanupRuntime(ctx context.Context) {
	s.mu.Lock()
	rt := s.runtime
	s.runtime = nil
	s.mu.Unlock()
	if rt == nil {
		_ = s.daemon.ClearPID(ctx)
		_ = os.Remove(s.daemon.SocketPath())
		return
	}
	if err := s.snapshot.Save(ctx, rt.state); err != nil {
		// best-effort flush on shutdown
	}
	if err := s.applier.Apply(ctx, rt.state); err != nil {
		// best-effort flush on shutdown
	}
	if rt.transport != nil {
		_ = rt.transport.Stop()
	}
	if rt.metricsSrv != nil {
		_ = rt.metricsSrv.Shutdown(context.Background())
	}
	if rt.metricsLn != nil {
		_ = rt.metricsLn.Close()
	}
	_ = s.daemon.ClearPID(ctx)
	_ = os.Remove(s.daemon.SocketPath())
}

func (s *CollabService) cleanupStaleArtifacts(ctx context.Context) error {
	pid, err := s.daemon.ReadPID(ctx)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if pid > 0 && !processAlive(pid) {
		_ = s.daemon.ClearPID(ctx)
		_ = os.Remove(s.daemon.SocketPath())
	}

	if _, statErr := os.Stat(s.daemon.SocketPath()); statErr == nil {
		if !socketReachable(s.daemon.SocketPath()) {
			if removeErr := os.Remove(s.daemon.SocketPath()); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove stale daemon socket: %w", removeErr)
			}
		}
	}
	return nil
}

func (s *CollabService) startMetricsEndpoint() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start metrics listener: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		snapshot, _ := s.Metrics(context.Background())
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot)
	})
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	s.mu.Lock()
	if s.runtime != nil {
		s.runtime.metricsLn = ln
		s.runtime.metricsSrv = srv
		s.runtime.metricsAddr = ln.Addr().String()
	}
	s.mu.Unlock()
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

func (s *CollabService) appendActivity(ctx context.Context, event domain.ActivityEvent) error {
	if s.activity == nil {
		return nil
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if err := s.activity.Append(ctx, event); err != nil {
		return err
	}
	encoded, _ := json.Marshal(event)
	_, _ = fmt.Fprintln(os.Stdout, string(encoded))
	return nil
}

func (s *CollabService) migrateIfNeeded(ctx context.Context) error {
	collabRoot := filepath.Join(s.vaultPath, ".mdht", "collab")
	legacyKey := filepath.Join(collabRoot, "workspace.key")
	keysV2 := filepath.Join(collabRoot, "keys.json")
	if _, err := os.Stat(keysV2); err == nil {
		return nil
	}
	if _, err := os.Stat(legacyKey); err != nil {
		return nil
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupDir := filepath.Join(collabRoot, "migrations", timestamp+"-v1-backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("create migration backup dir: %v", err))
	}

	files := []string{
		"workspace.json",
		"workspace.key",
		"node.ed25519",
		"peers.json",
		"snapshot.json",
		"oplog.jsonl",
	}
	for _, name := range files {
		src := filepath.Join(collabRoot, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("backup read %s: %v", name, err))
		}
		if err := os.WriteFile(filepath.Join(backupDir, name), raw, 0o644); err != nil {
			return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("backup write %s: %v", name, err))
		}
	}

	workspaceRaw, err := os.ReadFile(filepath.Join(collabRoot, "workspace.json"))
	if err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("read workspace for migration: %v", err))
	}
	workspaceAny := map[string]any{}
	if err := json.Unmarshal(workspaceRaw, &workspaceAny); err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("decode workspace for migration: %v", err))
	}
	workspaceID := asString(workspaceAny["workspace_id"])
	if workspaceID == "" {
		workspaceID = asString(workspaceAny["id"])
	}
	if workspaceID == "" {
		return domain.NewCodedError(domain.ErrorMigrationFailed, "workspace id missing in v1 state")
	}
	name := asString(workspaceAny["name"])
	createdAt := time.Now().UTC()
	if rawTime := asString(workspaceAny["created_at"]); rawTime != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, rawTime); parseErr == nil {
			createdAt = parsed
		}
	}
	workspacePayload, _ := json.MarshalIndent(map[string]any{
		"workspace_id":   workspaceID,
		"name":           name,
		"created_at":     createdAt,
		"schema_version": domain.SchemaVersionV2,
	}, "", "  ")
	if err := os.WriteFile(filepath.Join(collabRoot, "workspace.json"), workspacePayload, 0o644); err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("write migrated workspace: %v", err))
	}

	legacyKeyRaw, err := os.ReadFile(legacyKey)
	if err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("read legacy key: %v", err))
	}
	keyID := hashHex(workspaceID + ":" + string(legacyKeyRaw))[:16]
	keysPayload, _ := json.MarshalIndent(domain.KeyRing{
		ActiveKeyID: keyID,
		Keys: []domain.KeyRecord{
			{
				ID:        keyID,
				KeyBase64: strings.TrimSpace(string(legacyKeyRaw)),
				CreatedAt: time.Now().UTC(),
			},
		},
	}, "", "  ")
	if err := os.WriteFile(keysV2, keysPayload, 0o600); err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("write migrated keys: %v", err))
	}

	peerPath := filepath.Join(collabRoot, "peers.json")
	if raw, readErr := os.ReadFile(peerPath); readErr == nil && len(raw) > 0 {
		peers := []domain.Peer{}
		if err := json.Unmarshal(raw, &peers); err == nil {
			now := time.Now().UTC()
			for i := range peers {
				if peers[i].State == "" {
					peers[i].State = domain.PeerStateApproved
				}
				if peers[i].FirstSeen.IsZero() {
					peers[i].FirstSeen = now
				}
				if peers[i].LastSeen.IsZero() {
					peers[i].LastSeen = peers[i].FirstSeen
				}
				if peers[i].AddedAt.IsZero() {
					peers[i].AddedAt = peers[i].FirstSeen
				}
			}
			payload, _ := json.MarshalIndent(peers, "", "  ")
			_ = os.WriteFile(peerPath, payload, 0o644)
		}
	}

	opPath := filepath.Join(collabRoot, "oplog.jsonl")
	if file, openErr := os.Open(opPath); openErr == nil {
		defer file.Close()
		out := make([]string, 0, 128)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			op := domain.OpEnvelope{}
			if err := json.Unmarshal(line, &op); err != nil {
				continue
			}
			op.SchemaVersion = domain.SchemaVersionV2
			op.WorkspaceKeyID = keyID
			raw, _ := json.Marshal(op)
			out = append(out, string(raw))
		}
		if len(out) > 0 {
			_ = os.WriteFile(opPath, []byte(strings.Join(out, "\n")+"\n"), 0o644)
		}
	}

	migrationState := map[string]any{
		"from":       1,
		"to":         domain.SchemaVersionV2,
		"status":     "ok",
		"backup_dir": backupDir,
		"at":         time.Now().UTC(),
	}
	statePayload, _ := json.MarshalIndent(migrationState, "", "  ")
	if err := os.WriteFile(filepath.Join(collabRoot, "migration.state"), statePayload, 0o644); err != nil {
		return domain.NewCodedError(domain.ErrorMigrationFailed, fmt.Sprintf("write migration state: %v", err))
	}
	_ = s.appendActivity(ctx, domain.ActivityEvent{
		Type:    domain.ActivityMigration,
		Message: "migrated collab state from v1 to v2",
		Fields: map[string]string{
			"backup_dir": backupDir,
		},
	})
	return nil
}

func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if socketReachable(path) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon socket not ready: %s", path)
}

func socketReachable(path string) bool {
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func hashHex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}
