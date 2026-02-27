package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
	"mdht/internal/modules/collab/service"
)

type fakeWorkspaceStore struct {
	workspace domain.Workspace
	keyRing   domain.KeyRing
	node      domain.NodeIdentity
	err       error
}

func (f fakeWorkspaceStore) Init(context.Context, string) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error) {
	if f.err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, f.err
	}
	return f.workspace, f.keyRing, f.node, nil
}

func (f fakeWorkspaceStore) Load(context.Context) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error) {
	if f.err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, f.err
	}
	return f.workspace, f.keyRing, f.node, nil
}

func (f fakeWorkspaceStore) RotateKey(context.Context, time.Duration) (domain.KeyRing, error) {
	if f.err != nil {
		return domain.KeyRing{}, f.err
	}
	return f.keyRing, nil
}

type fakePeerStore struct {
	peers []domain.Peer
	err   error
}

func (f *fakePeerStore) Add(context.Context, string, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	peer := domain.Peer{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStateApproved, AddedAt: time.Now().UTC()}
	f.peers = append(f.peers, peer)
	return peer, nil
}
func (f *fakePeerStore) Approve(_ context.Context, peerID string) (domain.Peer, error) {
	for i := range f.peers {
		if f.peers[i].PeerID == peerID {
			f.peers[i].State = domain.PeerStateApproved
			return f.peers[i], nil
		}
	}
	return domain.Peer{}, domain.ErrPeerNotFound
}
func (f *fakePeerStore) Revoke(_ context.Context, peerID string) (domain.Peer, error) {
	for i := range f.peers {
		if f.peers[i].PeerID == peerID {
			f.peers[i].State = domain.PeerStateRevoked
			return f.peers[i], nil
		}
	}
	return domain.Peer{}, domain.ErrPeerNotFound
}
func (f *fakePeerStore) Update(_ context.Context, peer domain.Peer) (domain.Peer, error) {
	for i := range f.peers {
		if f.peers[i].PeerID == peer.PeerID {
			f.peers[i] = peer
			return f.peers[i], nil
		}
	}
	return domain.Peer{}, domain.ErrPeerNotFound
}
func (f *fakePeerStore) Remove(_ context.Context, peerID string) error {
	out := make([]domain.Peer, 0, len(f.peers))
	for _, peer := range f.peers {
		if peer.PeerID != peerID {
			out = append(out, peer)
		}
	}
	f.peers = out
	return nil
}
func (f *fakePeerStore) List(context.Context) ([]domain.Peer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.Peer{}, f.peers...), nil
}

type fakeOpLogStore struct{ ops []domain.OpEnvelope }

func (f *fakeOpLogStore) Append(_ context.Context, op domain.OpEnvelope) error {
	f.ops = append(f.ops, op)
	return nil
}
func (f *fakeOpLogStore) List(context.Context) ([]domain.OpEnvelope, error) {
	return append([]domain.OpEnvelope{}, f.ops...), nil
}

type fakeSnapshotStore struct{ state domain.CRDTState }

func (f *fakeSnapshotStore) Load(context.Context) (domain.CRDTState, error) {
	if f.state.Entities == nil {
		f.state = domain.NewCRDTState()
	}
	return f.state, nil
}
func (f *fakeSnapshotStore) Save(_ context.Context, state domain.CRDTState) error {
	f.state = state
	return nil
}

type fakeExtractor struct{}

func (fakeExtractor) Extract(context.Context, string, string, string, time.Time) ([]domain.OpEnvelope, error) {
	return nil, nil
}

type fakeApplier struct{}

func (fakeApplier) Apply(context.Context, domain.CRDTState) error { return nil }

type fakeTransport struct{}

func (fakeTransport) Start(context.Context, collabout.TransportStartInput, collabout.TransportHandlers) (collabout.RuntimeTransport, error) {
	return fakeRuntimeTransport{}, nil
}

type fakeRuntimeTransport struct{}

func (fakeRuntimeTransport) Broadcast(context.Context, domain.OpEnvelope) error { return nil }
func (fakeRuntimeTransport) Reconcile(context.Context, []domain.OpEnvelope) error {
	return nil
}
func (fakeRuntimeTransport) AddPeer(context.Context, domain.Peer) error     { return nil }
func (fakeRuntimeTransport) ApprovePeer(context.Context, domain.Peer) error { return nil }
func (fakeRuntimeTransport) RevokePeer(context.Context, string) error       { return nil }
func (fakeRuntimeTransport) RemovePeer(context.Context, string) error       { return nil }
func (fakeRuntimeTransport) UpdateKeyRing(context.Context, domain.KeyRing) error {
	return nil
}
func (fakeRuntimeTransport) Probe(context.Context) (collabout.NetProbe, error) {
	return collabout.NetProbe{Reachability: domain.ReachabilityPublic, NATMode: "direct"}, nil
}
func (fakeRuntimeTransport) DialPeer(context.Context, string) (domain.DialOutcome, error) {
	return domain.DialOutcome{
		Result:        domain.DialResultSuccess,
		TraversalMode: domain.TraversalDirect,
		RTTMS:         10,
		At:            time.Now().UTC(),
		Reachability:  domain.ReachabilityPublic,
	}, nil
}
func (fakeRuntimeTransport) PeerLatency(context.Context) ([]collabout.PeerLatency, error) {
	return []collabout.PeerLatency{{PeerID: "peer-a", RTTMS: 10}}, nil
}
func (fakeRuntimeTransport) Status() collabout.NetworkStatus { return collabout.NetworkStatus{} }
func (fakeRuntimeTransport) Stop() error                     { return nil }

type fakeDaemonStore struct {
	pidPath    string
	socketPath string
	logPath    string
}

func newFakeDaemonStore(vaultPath string) *fakeDaemonStore {
	base := filepath.Join(vaultPath, ".mdht", "collab")
	return &fakeDaemonStore{
		pidPath:    filepath.Join(base, "daemon.pid"),
		socketPath: filepath.Join(base, "daemon.sock"),
		logPath:    filepath.Join(base, "daemon.log"),
	}
}

func (d *fakeDaemonStore) WritePID(_ context.Context, pid int) error {
	if err := os.MkdirAll(filepath.Dir(d.pidPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(d.pidPath, []byte(strconv.Itoa(pid)), 0o644)
}
func (d *fakeDaemonStore) ReadPID(_ context.Context) (int, error) {
	raw, err := os.ReadFile(d.pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(raw)))
}
func (d *fakeDaemonStore) ClearPID(_ context.Context) error {
	if err := os.Remove(d.pidPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
func (d *fakeDaemonStore) SocketPath() string { return d.socketPath }
func (d *fakeDaemonStore) LogPath() string    { return d.logPath }

type fakeIPCServer struct{}

func (fakeIPCServer) Serve(context.Context, string, collabout.IPCHandler) error { return nil }

type fakeIPCClient struct{}

func (fakeIPCClient) WorkspaceInit(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, errors.New("not implemented")
}
func (fakeIPCClient) WorkspaceShow(context.Context, string) (domain.Workspace, string, []domain.Peer, error) {
	return domain.Workspace{}, "", nil, errors.New("not implemented")
}
func (fakeIPCClient) WorkspaceRotateKey(context.Context, string, time.Duration) (domain.Workspace, error) {
	return domain.Workspace{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerAdd(context.Context, string, string, string) (domain.Peer, error) {
	return domain.Peer{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerApprove(context.Context, string, string) (domain.Peer, error) {
	return domain.Peer{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerRevoke(context.Context, string, string) (domain.Peer, error) {
	return domain.Peer{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerRemove(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (fakeIPCClient) PeerList(context.Context, string) ([]domain.Peer, error) {
	return nil, errors.New("not implemented")
}
func (fakeIPCClient) Status(context.Context, string) (collabout.DaemonStatus, error) {
	return collabout.DaemonStatus{}, errors.New("not implemented")
}
func (fakeIPCClient) ActivityTail(context.Context, string, collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	return nil, errors.New("not implemented")
}
func (fakeIPCClient) ConflictsList(context.Context, string, string) ([]domain.ConflictRecord, error) {
	return nil, errors.New("not implemented")
}
func (fakeIPCClient) ConflictResolve(context.Context, string, string, domain.ConflictStrategy) (domain.ConflictRecord, error) {
	return domain.ConflictRecord{}, errors.New("not implemented")
}
func (fakeIPCClient) SyncNow(context.Context, string) (int, error) {
	return 0, errors.New("not implemented")
}
func (fakeIPCClient) SyncHealth(context.Context, string) (collabout.SyncHealth, error) {
	return collabout.SyncHealth{}, errors.New("not implemented")
}
func (fakeIPCClient) NetStatus(context.Context, string) (collabout.NetStatus, error) {
	return collabout.NetStatus{}, errors.New("not implemented")
}
func (fakeIPCClient) NetProbe(context.Context, string) (collabout.NetProbe, error) {
	return collabout.NetProbe{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerDial(context.Context, string, string) (domain.Peer, error) {
	return domain.Peer{}, errors.New("not implemented")
}
func (fakeIPCClient) PeerLatency(context.Context, string) ([]collabout.PeerLatency, error) {
	return nil, errors.New("not implemented")
}
func (fakeIPCClient) SnapshotExport(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}
func (fakeIPCClient) Metrics(context.Context, string) (collabout.MetricsSnapshot, error) {
	return collabout.MetricsSnapshot{}, errors.New("not implemented")
}
func (fakeIPCClient) Stop(context.Context, string) error { return nil }

type fakeActivityStore struct{}

func (fakeActivityStore) Append(context.Context, domain.ActivityEvent) error { return nil }
func (fakeActivityStore) Tail(context.Context, collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	return nil, nil
}

type fakeConflictStore struct{}

func (fakeConflictStore) List(context.Context, string) ([]domain.ConflictRecord, error) {
	return nil, nil
}
func (fakeConflictStore) Upsert(context.Context, domain.ConflictRecord) error { return nil }

func newServiceForTest(vaultPath string, workspaceErr error, peers []domain.Peer) (*service.CollabService, *fakeDaemonStore) {
	ws := fakeWorkspaceStore{
		workspace: domain.Workspace{ID: "ws-1", Name: "alpha", SchemaVersion: domain.SchemaVersionV2, CreatedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)},
		keyRing: domain.KeyRing{
			ActiveKeyID: "k1",
			Keys:        []domain.KeyRecord{{ID: "k1", KeyBase64: "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=", CreatedAt: time.Now().UTC()}},
		},
		node: domain.NodeIdentity{NodeID: "node-a", PrivateKey: ""},
		err:  workspaceErr,
	}
	peerStore := &fakePeerStore{peers: peers}
	daemonStore := newFakeDaemonStore(vaultPath)
	return service.NewCollabService(
		vaultPath,
		ws,
		peerStore,
		&fakeOpLogStore{},
		&fakeSnapshotStore{state: domain.NewCRDTState()},
		fakeExtractor{},
		fakeApplier{},
		fakeTransport{},
		daemonStore,
		fakeIPCServer{},
		fakeIPCClient{},
		fakeActivityStore{},
		fakeConflictStore{},
	), daemonStore
}

func TestStopDaemonIdempotentAndStaleCleanup(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	svc, daemonStore := newServiceForTest(vault, domain.ErrWorkspaceNotInitialized, nil)

	if err := svc.StopDaemon(context.Background()); err != nil {
		t.Fatalf("stop daemon first call: %v", err)
	}
	if err := svc.StopDaemon(context.Background()); err != nil {
		t.Fatalf("stop daemon second call: %v", err)
	}

	if err := daemonStore.WritePID(context.Background(), 999999); err != nil {
		t.Fatalf("write stale pid: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(daemonStore.SocketPath()), 0o755); err != nil {
		t.Fatalf("mk socket dir: %v", err)
	}
	if err := os.WriteFile(daemonStore.SocketPath(), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale socket: %v", err)
	}
	if err := svc.StopDaemon(context.Background()); err != nil {
		t.Fatalf("stop daemon stale cleanup: %v", err)
	}
	if _, err := daemonStore.ReadPID(context.Background()); !os.IsNotExist(err) {
		t.Fatalf("expected pid file removed, got err=%v", err)
	}
	if _, err := os.Stat(daemonStore.SocketPath()); !os.IsNotExist(err) {
		t.Fatalf("expected socket removed, got err=%v", err)
	}
}

func TestDaemonLogsTail(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	svc, daemonStore := newServiceForTest(vault, domain.ErrWorkspaceNotInitialized, nil)

	if err := os.MkdirAll(filepath.Dir(daemonStore.LogPath()), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(daemonStore.LogPath(), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatalf("write logs: %v", err)
	}
	logs, err := svc.DaemonLogs(context.Background(), 2)
	if err != nil {
		t.Fatalf("daemon logs: %v", err)
	}
	if strings.TrimSpace(logs) != "l2\nl3" {
		t.Fatalf("unexpected tail output: %q", logs)
	}
}

func TestSyncNowWithoutDaemon(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	svc, _ := newServiceForTest(vault, nil, []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStateApproved}})
	if _, err := svc.SyncNow(context.Background()); !errors.Is(err, domain.ErrDaemonNotRunning) {
		t.Fatalf("expected daemon not running error, got %v", err)
	}
}
