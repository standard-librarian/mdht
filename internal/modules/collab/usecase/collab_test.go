package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
	"mdht/internal/modules/collab/usecase"
)

type fakeService struct {
	err error
}

func (f fakeService) RunDaemon(context.Context) error   { return f.err }
func (f fakeService) StartDaemon(context.Context) error { return f.err }
func (f fakeService) StopDaemon(context.Context) error  { return f.err }
func (f fakeService) DaemonStatus(context.Context) (collabout.DaemonRuntimeStatus, error) {
	if f.err != nil {
		return collabout.DaemonRuntimeStatus{}, f.err
	}
	return collabout.DaemonRuntimeStatus{
		Running:    true,
		PID:        123,
		SocketPath: "/tmp/sock",
		Status: collabout.DaemonStatus{
			Online:            true,
			PeerCount:         2,
			ApprovedPeerCount: 1,
			PendingConflicts:  1,
			PendingOps:        3,
			LastSyncAt:        time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			NodeID:            "node-a",
			WorkspaceID:       "ws-a",
			ListenAddrs:       []string{"/ip4/127.0.0.1/tcp/4001/p2p/peer"},
		},
	}, nil
}
func (f fakeService) WorkspaceInit(context.Context, string) (domain.Workspace, error) {
	if f.err != nil {
		return domain.Workspace{}, f.err
	}
	return domain.Workspace{ID: "ws-a", Name: "alpha", CreatedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC), SchemaVersion: domain.SchemaVersionV2}, nil
}
func (f fakeService) WorkspaceShow(context.Context) (domain.Workspace, string, []domain.Peer, error) {
	if f.err != nil {
		return domain.Workspace{}, "", nil, f.err
	}
	return domain.Workspace{ID: "ws-a", Name: "alpha", SchemaVersion: domain.SchemaVersionV2}, "node-a", []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}}, nil
}
func (f fakeService) WorkspaceRotateKey(context.Context, time.Duration) (domain.Workspace, error) {
	if f.err != nil {
		return domain.Workspace{}, f.err
	}
	return domain.Workspace{ID: "ws-a", Name: "alpha", SchemaVersion: domain.SchemaVersionV2}, nil
}
func (f fakeService) PeerAdd(context.Context, string, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	return domain.Peer{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStatePending}, nil
}
func (f fakeService) PeerApprove(context.Context, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	return domain.Peer{PeerID: "peer-a", State: domain.PeerStateApproved}, nil
}
func (f fakeService) PeerRevoke(context.Context, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	return domain.Peer{PeerID: "peer-a", State: domain.PeerStateRevoked}, nil
}
func (f fakeService) PeerDial(context.Context, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	return domain.Peer{
		PeerID:         "peer-a",
		State:          domain.PeerStateApproved,
		LastDialResult: domain.DialResultSuccess,
		RTTMS:          12,
		TraversalMode:  domain.TraversalDirect,
		Reachability:   domain.ReachabilityPublic,
		LastDialAt:     time.Now().UTC(),
	}, nil
}
func (f fakeService) PeerLatency(context.Context) ([]collabout.PeerLatency, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []collabout.PeerLatency{{PeerID: "peer-a", RTTMS: 12}}, nil
}
func (f fakeService) PeerRemove(context.Context, string) error { return f.err }
func (f fakeService) PeerList(context.Context) ([]domain.Peer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStateApproved}}, nil
}
func (f fakeService) Status(context.Context) (collabout.DaemonStatus, error) {
	if f.err != nil {
		return collabout.DaemonStatus{}, f.err
	}
	return collabout.DaemonStatus{
		Online:            true,
		PeerCount:         1,
		ApprovedPeerCount: 1,
		PendingConflicts:  0,
		PendingOps:        2,
		NodeID:            "node-a",
		WorkspaceID:       "ws-a",
		Reachability:      domain.ReachabilityPublic,
		NATMode:           "direct",
		Connectivity:      domain.SyncHealthGood,
	}, nil
}
func (f fakeService) NetStatus(context.Context) (collabout.NetStatus, error) {
	if f.err != nil {
		return collabout.NetStatus{}, f.err
	}
	return collabout.NetStatus{
		Online:       true,
		Reachability: domain.ReachabilityPublic,
		NATMode:      "direct",
		Connectivity: domain.SyncHealthGood,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/4001/p2p/peer-a"},
		PeerCount:    1,
		LastSyncAt:   time.Now().UTC(),
	}, nil
}
func (f fakeService) NetProbe(context.Context) (collabout.NetProbe, error) {
	if f.err != nil {
		return collabout.NetProbe{}, f.err
	}
	return collabout.NetProbe{
		Reachability: domain.ReachabilityPublic,
		NATMode:      "direct",
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/4001/p2p/peer-a"},
		DialableAddrs: []string{
			"/ip4/127.0.0.1/tcp/4001/p2p/peer-a",
		},
	}, nil
}
func (f fakeService) DaemonLogs(context.Context, int) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "line 1\nline 2", nil
}
func (f fakeService) ActivityTail(context.Context, collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.ActivityEvent{{ID: "1", Type: domain.ActivitySyncApplied, Message: "ok"}}, nil
}
func (f fakeService) ConflictsList(context.Context, string) ([]domain.ConflictRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.ConflictRecord{{ID: "c1", EntityKey: "source/s1", Field: "title", Status: domain.ConflictStatusOpen}}, nil
}
func (f fakeService) ConflictResolve(context.Context, string, domain.ConflictStrategy) (domain.ConflictRecord, error) {
	if f.err != nil {
		return domain.ConflictRecord{}, f.err
	}
	return domain.ConflictRecord{ID: "c1", Status: domain.ConflictStatusResolved, Strategy: "local"}, nil
}
func (f fakeService) SyncNow(context.Context) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return 4, nil
}
func (f fakeService) SyncHealth(context.Context) (collabout.SyncHealth, error) {
	if f.err != nil {
		return collabout.SyncHealth{}, f.err
	}
	return collabout.SyncHealth{
		State:      domain.SyncHealthGood,
		Reason:     "healthy",
		LagSeconds: 5,
		PendingOps: 2,
		LastSyncAt: time.Now().UTC(),
	}, nil
}
func (f fakeService) SnapshotExport(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "{}", nil
}
func (f fakeService) Metrics(context.Context) (collabout.MetricsSnapshot, error) {
	if f.err != nil {
		return collabout.MetricsSnapshot{}, f.err
	}
	return collabout.MetricsSnapshot{WorkspaceID: "ws-a", NodeID: "node-a", ApprovedPeers: 1, PendingOps: 2}, nil
}

func TestInteractorSuccess(t *testing.T) {
	t.Parallel()
	uc := usecase.NewInteractor(fakeService{})
	ctx := context.Background()

	if err := uc.RunDaemon(ctx); err != nil {
		t.Fatalf("run daemon: %v", err)
	}
	if err := uc.StartDaemon(ctx); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	if err := uc.StopDaemon(ctx); err != nil {
		t.Fatalf("stop daemon: %v", err)
	}
	if _, err := uc.DaemonStatus(ctx); err != nil {
		t.Fatalf("daemon status: %v", err)
	}
	if _, err := uc.WorkspaceInit(ctx, "alpha"); err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	if _, err := uc.WorkspaceShow(ctx); err != nil {
		t.Fatalf("workspace show: %v", err)
	}
	if _, err := uc.WorkspaceRotateKey(ctx, time.Hour); err != nil {
		t.Fatalf("workspace rotate key: %v", err)
	}
	if _, err := uc.PeerAdd(ctx, "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", "alpha"); err != nil {
		t.Fatalf("peer add: %v", err)
	}
	if _, err := uc.PeerApprove(ctx, "peer-a"); err != nil {
		t.Fatalf("peer approve: %v", err)
	}
	if _, err := uc.PeerRevoke(ctx, "peer-a"); err != nil {
		t.Fatalf("peer revoke: %v", err)
	}
	if _, err := uc.PeerDial(ctx, "peer-a"); err != nil {
		t.Fatalf("peer dial: %v", err)
	}
	if _, err := uc.PeerLatency(ctx); err != nil {
		t.Fatalf("peer latency: %v", err)
	}
	if err := uc.PeerRemove(ctx, "peer-a"); err != nil {
		t.Fatalf("peer remove: %v", err)
	}
	if _, err := uc.PeerList(ctx); err != nil {
		t.Fatalf("peer list: %v", err)
	}
	if _, err := uc.Status(ctx); err != nil {
		t.Fatalf("status: %v", err)
	}
	if _, err := uc.NetStatus(ctx); err != nil {
		t.Fatalf("net status: %v", err)
	}
	if _, err := uc.NetProbe(ctx); err != nil {
		t.Fatalf("net probe: %v", err)
	}
	if _, err := uc.DaemonLogs(ctx, 20); err != nil {
		t.Fatalf("daemon logs: %v", err)
	}
	if _, err := uc.ActivityTail(ctx, time.Time{}, 10); err != nil {
		t.Fatalf("activity tail: %v", err)
	}
	if _, err := uc.ConflictsList(ctx, ""); err != nil {
		t.Fatalf("conflicts list: %v", err)
	}
	if _, err := uc.ConflictResolve(ctx, "c1", "local"); err != nil {
		t.Fatalf("conflict resolve: %v", err)
	}
	if _, err := uc.SyncNow(ctx); err != nil {
		t.Fatalf("sync now: %v", err)
	}
	if _, err := uc.SyncHealth(ctx); err != nil {
		t.Fatalf("sync health: %v", err)
	}
	if _, err := uc.SnapshotExport(ctx); err != nil {
		t.Fatalf("snapshot export: %v", err)
	}
	if _, err := uc.Metrics(ctx); err != nil {
		t.Fatalf("metrics: %v", err)
	}
}

func TestInteractorErrors(t *testing.T) {
	t.Parallel()
	uc := usecase.NewInteractor(fakeService{err: errors.New("boom")})
	ctx := context.Background()

	if err := uc.RunDaemon(ctx); err == nil {
		t.Fatalf("expected run daemon error")
	}
	if _, err := uc.WorkspaceInit(ctx, "alpha"); err == nil {
		t.Fatalf("expected workspace init error")
	}
	if _, err := uc.WorkspaceShow(ctx); err == nil {
		t.Fatalf("expected workspace show error")
	}
	if _, err := uc.WorkspaceRotateKey(ctx, time.Hour); err == nil {
		t.Fatalf("expected workspace rotate key error")
	}
	if _, err := uc.PeerAdd(ctx, "addr", "label"); err == nil {
		t.Fatalf("expected peer add error")
	}
	if _, err := uc.PeerDial(ctx, "peer-a"); err == nil {
		t.Fatalf("expected peer dial error")
	}
	if _, err := uc.PeerLatency(ctx); err == nil {
		t.Fatalf("expected peer latency error")
	}
	if _, err := uc.PeerApprove(ctx, "peer-a"); err == nil {
		t.Fatalf("expected peer approve error")
	}
	if _, err := uc.PeerRevoke(ctx, "peer-a"); err == nil {
		t.Fatalf("expected peer revoke error")
	}
	if err := uc.PeerRemove(ctx, "peer-a"); err == nil {
		t.Fatalf("expected peer remove error")
	}
	if _, err := uc.PeerList(ctx); err == nil {
		t.Fatalf("expected peer list error")
	}
	if _, err := uc.Status(ctx); err == nil {
		t.Fatalf("expected status error")
	}
	if _, err := uc.NetStatus(ctx); err == nil {
		t.Fatalf("expected net status error")
	}
	if _, err := uc.NetProbe(ctx); err == nil {
		t.Fatalf("expected net probe error")
	}
	if _, err := uc.DaemonStatus(ctx); err == nil {
		t.Fatalf("expected daemon status error")
	}
	if _, err := uc.DaemonLogs(ctx, 10); err == nil {
		t.Fatalf("expected daemon logs error")
	}
	if _, err := uc.ActivityTail(ctx, time.Time{}, 10); err == nil {
		t.Fatalf("expected activity tail error")
	}
	if _, err := uc.ConflictsList(ctx, "source/s1"); err == nil {
		t.Fatalf("expected conflicts list error")
	}
	if _, err := uc.ConflictResolve(ctx, "c1", "local"); err == nil {
		t.Fatalf("expected conflict resolve error")
	}
	if _, err := uc.SyncNow(ctx); err == nil {
		t.Fatalf("expected sync now error")
	}
	if _, err := uc.SyncHealth(ctx); err == nil {
		t.Fatalf("expected sync health error")
	}
	if _, err := uc.SnapshotExport(ctx); err == nil {
		t.Fatalf("expected snapshot export error")
	}
	if _, err := uc.Metrics(ctx); err == nil {
		t.Fatalf("expected metrics error")
	}
	if err := uc.StartDaemon(ctx); err == nil {
		t.Fatalf("expected start daemon error")
	}
	if err := uc.StopDaemon(ctx); err == nil {
		t.Fatalf("expected stop daemon error")
	}
}
