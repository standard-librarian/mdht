package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type runtimeState struct {
	workspace    domain.Workspace
	workspaceKey []byte
	node         domain.NodeIdentity
	state        domain.CRDTState
	transport    collabout.RuntimeTransport
	status       collabout.NetworkStatus
	cancel       context.CancelFunc
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
	}
}

func (s *CollabService) RunDaemon(ctx context.Context) error {
	workspace, key, node, err := s.workspace.Load(ctx)
	if err != nil {
		return err
	}
	peers, err := s.peers.List(ctx)
	if err != nil {
		return err
	}

	state, err := s.snapshot.Load(ctx)
	if err != nil {
		return err
	}
	ops, err := s.oplog.List(ctx)
	if err != nil {
		return err
	}
	for _, op := range ops {
		_ = state.Apply(op)
	}
	if err := s.snapshot.Save(ctx, state); err != nil {
		return err
	}
	if err := s.applier.Apply(ctx, state); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	rt, err := s.transport.Start(runCtx, collabout.TransportStartInput{
		WorkspaceID:  workspace.ID,
		WorkspaceKey: key,
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
		workspaceKey: key,
		node:         node,
		state:        state,
		transport:    rt,
		status:       rt.Status(),
		cancel:       cancel,
	}
	s.mu.Unlock()

	if err := s.daemon.WritePID(ctx, os.Getpid()); err != nil {
		cancel()
		_ = rt.Stop()
		return err
	}

	ipcErr := make(chan error, 1)
	go func() {
		if s.ipcServer == nil {
			ipcErr <- fmt.Errorf("ipc server is not configured")
			return
		}
		ipcErr <- s.ipcServer.Serve(runCtx, s.daemon.SocketPath(), s)
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-runCtx.Done():
			s.cleanupRuntime()
			return nil
		case err := <-ipcErr:
			if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
				s.cleanupRuntime()
				return err
			}
			s.cleanupRuntime()
			return nil
		case <-ticker.C:
			_, _ = s.ReconcileNow(context.Background())
			ops, listErr := s.oplog.List(context.Background())
			if listErr == nil {
				_ = rt.Reconcile(context.Background(), ops)
			}
		}
	}
}

func (s *CollabService) StartDaemon(ctx context.Context) error {
	if status, err := s.DaemonStatus(ctx); err == nil && status.Running {
		return nil
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.daemon.LogPath()), 0o755); err != nil {
		return fmt.Errorf("create daemon log dir: %w", err)
	}
	logFile, err := os.OpenFile(s.daemon.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(execPath, "collab", "daemon", "run", "--vault", s.vaultPath)
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
	return waitForSocket(s.daemon.SocketPath(), 3*time.Second)
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
			return nil
		}
		return err
	}
	if pid <= 0 {
		_ = s.daemon.ClearPID(ctx)
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

func (s *CollabService) DaemonStatus(ctx context.Context) (struct {
	Running    bool
	PID        int
	SocketPath string
	Status     collabout.DaemonStatus
}, error) {
	out := struct {
		Running    bool
		PID        int
		SocketPath string
		Status     collabout.DaemonStatus
	}{SocketPath: s.daemon.SocketPath()}

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

func (s *CollabService) PeerAdd(ctx context.Context, addr string) (domain.Peer, error) {
	peer, err := s.peers.Add(ctx, addr)
	if err != nil {
		return domain.Peer{}, err
	}
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt != nil && rt.transport != nil {
		_ = rt.transport.AddPeer(ctx, peer)
	}
	return peer, nil
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
	if rt == nil {
		workspace, _, node, err := s.workspace.Load(ctx)
		if err != nil {
			return collabout.DaemonStatus{}, err
		}
		state, err := s.snapshot.Load(ctx)
		if err != nil {
			return collabout.DaemonStatus{}, err
		}
		return collabout.DaemonStatus{
			Online:      false,
			PeerCount:   0,
			PendingOps:  state.PendingOps,
			LastSyncAt:  state.LastSyncAt,
			NodeID:      node.NodeID,
			WorkspaceID: workspace.ID,
		}, nil
	}

	status := rt.status
	return collabout.DaemonStatus{
		Online:      status.Online,
		PeerCount:   status.PeerCount,
		PendingOps:  rt.state.PendingOps,
		LastSyncAt:  status.LastSyncAt,
		NodeID:      rt.node.NodeID,
		WorkspaceID: rt.workspace.ID,
		ListenAddrs: status.ListenAddrs,
	}, nil
}

func (s *CollabService) Stop(ctx context.Context) error {
	return s.StopDaemon(ctx)
}

func (s *CollabService) ReconcileNow(ctx context.Context) (int, error) {
	s.mu.RLock()
	rt := s.runtime
	s.mu.RUnlock()
	if rt == nil {
		return 0, domain.ErrWorkspaceNotInitialized
	}

	ops, err := s.extractor.Extract(ctx, rt.workspace.ID, rt.node.NodeID, time.Now().UTC())
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
	return applied, nil
}

func (s *CollabService) ExportState(ctx context.Context) (string, error) {
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
		s.mu.Unlock()
		return domain.ErrWorkspaceMismatch
	}
	if !op.Verify(rt.workspaceKey) {
		s.mu.Unlock()
		return domain.ErrInvalidAuthTag
	}
	if _, exists := rt.state.AppliedOps[op.OpID]; exists {
		s.mu.Unlock()
		return nil
	}
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

func (s *CollabService) normalizeOp(rt *runtimeState, op domain.OpEnvelope) domain.OpEnvelope {
	op.WorkspaceID = rt.workspace.ID
	op.NodeID = rt.node.NodeID
	if op.HLCTimestamp == "" {
		op.HLCTimestamp = domain.NextHLC(time.Now().UTC(), rt.state.LastApplied, rt.node.NodeID).String()
	}
	if op.OpID == "" {
		hash := sha256.Sum256([]byte(op.EntityKey + "|" + string(op.OpKind) + "|" + string(op.Payload) + "|" + op.HLCTimestamp + "|" + op.NodeID))
		op.OpID = hex.EncodeToString(hash[:])
	}
	return op.Signed(rt.workspaceKey)
}

func (s *CollabService) cleanupRuntime() {
	s.mu.Lock()
	rt := s.runtime
	s.runtime = nil
	s.mu.Unlock()
	if rt != nil && rt.transport != nil {
		_ = rt.transport.Stop()
	}
	_ = s.daemon.ClearPID(context.Background())
	_ = os.Remove(s.daemon.SocketPath())
}

func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon socket not ready: %s", path)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
