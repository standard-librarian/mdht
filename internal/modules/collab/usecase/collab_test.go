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
			Online:      true,
			PeerCount:   2,
			PendingOps:  3,
			LastSyncAt:  time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			NodeID:      "node-a",
			WorkspaceID: "ws-a",
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/4001/p2p/peer"},
			Counters: collabout.ValidationCounters{
				ReconnectAttempts:  3,
				ReconnectSuccesses: 2,
			},
		},
	}, nil
}
func (f fakeService) WorkspaceInit(context.Context, string) (domain.Workspace, error) {
	if f.err != nil {
		return domain.Workspace{}, f.err
	}
	return domain.Workspace{ID: "ws-a", Name: "alpha", CreatedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)}, nil
}
func (f fakeService) WorkspaceShow(context.Context) (domain.Workspace, string, []domain.Peer, error) {
	if f.err != nil {
		return domain.Workspace{}, "", nil, f.err
	}
	return domain.Workspace{ID: "ws-a", Name: "alpha"}, "node-a", []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}}, nil
}
func (f fakeService) PeerAdd(context.Context, string) (domain.Peer, error) {
	if f.err != nil {
		return domain.Peer{}, f.err
	}
	return domain.Peer{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", AddedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)}, nil
}
func (f fakeService) PeerRemove(context.Context, string) error { return f.err }
func (f fakeService) PeerList(context.Context) ([]domain.Peer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}}, nil
}
func (f fakeService) Status(context.Context) (collabout.DaemonStatus, error) {
	if f.err != nil {
		return collabout.DaemonStatus{}, f.err
	}
	return collabout.DaemonStatus{Online: true, PeerCount: 1, PendingOps: 2, NodeID: "node-a", WorkspaceID: "ws-a", Counters: collabout.ValidationCounters{InvalidAuthTag: 1}}, nil
}
func (f fakeService) Doctor(context.Context) ([]collabout.DoctorCheck, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []collabout.DoctorCheck{
		{Name: "workspace", OK: true, Details: "ok"},
		{Name: "daemon", OK: false, Details: "stopped"},
	}, nil
}
func (f fakeService) DaemonLogs(context.Context, int) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "line 1\nline 2", nil
}
func (f fakeService) ReconcileNow(context.Context) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return 4, nil
}
func (f fakeService) ExportState(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "{}", nil
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

	ds, err := uc.DaemonStatus(ctx)
	if err != nil {
		t.Fatalf("daemon status: %v", err)
	}
	if !ds.Running || ds.Status.PeerCount != 2 {
		t.Fatalf("unexpected daemon status: %+v", ds)
	}

	ws, err := uc.WorkspaceInit(ctx, "alpha")
	if err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	if ws.ID == "" {
		t.Fatalf("workspace id must be set")
	}

	shown, err := uc.WorkspaceShow(ctx)
	if err != nil {
		t.Fatalf("workspace show: %v", err)
	}
	if shown.NodeID != "node-a" || shown.Peers != 1 {
		t.Fatalf("unexpected workspace show: %+v", shown)
	}

	peer, err := uc.PeerAdd(ctx, "/ip4/127.0.0.1/tcp/4001/p2p/peer-a")
	if err != nil {
		t.Fatalf("peer add: %v", err)
	}
	if peer.PeerID != "peer-a" {
		t.Fatalf("unexpected peer output: %+v", peer)
	}

	if err := uc.PeerRemove(ctx, "peer-a"); err != nil {
		t.Fatalf("peer remove: %v", err)
	}
	peers, err := uc.PeerList(ctx)
	if err != nil {
		t.Fatalf("peer list: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("unexpected peer count: %d", len(peers))
	}

	status, err := uc.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Online || status.WorkspaceID != "ws-a" {
		t.Fatalf("unexpected collab status: %+v", status)
	}
	if status.Counters.InvalidAuthTag != 1 {
		t.Fatalf("expected mapped counters in status output")
	}

	doctor, err := uc.Doctor(ctx)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if len(doctor.Checks) != 2 {
		t.Fatalf("unexpected doctor checks: %+v", doctor)
	}

	logs, err := uc.DaemonLogs(ctx, 10)
	if err != nil {
		t.Fatalf("daemon logs: %v", err)
	}
	if logs == "" {
		t.Fatalf("expected daemon logs payload")
	}

	reconciled, err := uc.ReconcileNow(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if reconciled.Applied != 4 {
		t.Fatalf("unexpected reconcile output: %+v", reconciled)
	}

	exported, err := uc.ExportState(ctx)
	if err != nil {
		t.Fatalf("export state: %v", err)
	}
	if exported.Payload != "{}" {
		t.Fatalf("unexpected export payload: %q", exported.Payload)
	}
}

func TestInteractorErrors(t *testing.T) {
	t.Parallel()
	uc := usecase.NewInteractor(fakeService{err: errors.New("boom")})
	ctx := context.Background()

	if err := uc.RunDaemon(ctx); err == nil {
		t.Fatalf("expected run daemon error")
	}
	if err := uc.StartDaemon(ctx); err == nil {
		t.Fatalf("expected start daemon error")
	}
	if err := uc.StopDaemon(ctx); err == nil {
		t.Fatalf("expected stop daemon error")
	}
	if _, err := uc.DaemonStatus(ctx); err == nil {
		t.Fatalf("expected daemon status error")
	}
	if _, err := uc.WorkspaceInit(ctx, "alpha"); err == nil {
		t.Fatalf("expected workspace init error")
	}
	if _, err := uc.WorkspaceShow(ctx); err == nil {
		t.Fatalf("expected workspace show error")
	}
	if _, err := uc.PeerAdd(ctx, "addr"); err == nil {
		t.Fatalf("expected peer add error")
	}
	if err := uc.PeerRemove(ctx, "peer"); err == nil {
		t.Fatalf("expected peer remove error")
	}
	if _, err := uc.PeerList(ctx); err == nil {
		t.Fatalf("expected peer list error")
	}
	if _, err := uc.Status(ctx); err == nil {
		t.Fatalf("expected status error")
	}
	if _, err := uc.Doctor(ctx); err == nil {
		t.Fatalf("expected doctor error")
	}
	if _, err := uc.DaemonLogs(ctx, 20); err == nil {
		t.Fatalf("expected daemon logs error")
	}
	if _, err := uc.ReconcileNow(ctx); err == nil {
		t.Fatalf("expected reconcile error")
	}
	if _, err := uc.ExportState(ctx); err == nil {
		t.Fatalf("expected export error")
	}
}
