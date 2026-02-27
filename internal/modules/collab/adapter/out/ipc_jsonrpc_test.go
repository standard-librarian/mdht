package out_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	out "mdht/internal/modules/collab/adapter/out"
	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type fakeIPCHandler struct {
	mu      sync.Mutex
	stopped bool
}

func (h *fakeIPCHandler) WorkspaceInit(_ context.Context, name string) (domain.Workspace, error) {
	return domain.Workspace{ID: "ws-1", Name: name, CreatedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC), SchemaVersion: domain.SchemaVersionV2}, nil
}
func (h *fakeIPCHandler) WorkspaceShow(context.Context) (domain.Workspace, string, []domain.Peer, error) {
	return domain.Workspace{ID: "ws-1", Name: "alpha", SchemaVersion: domain.SchemaVersionV2}, "node-1", []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStateApproved}}, nil
}
func (h *fakeIPCHandler) WorkspaceRotateKey(context.Context, time.Duration) (domain.Workspace, error) {
	return domain.Workspace{ID: "ws-1", Name: "alpha", SchemaVersion: domain.SchemaVersionV2}, nil
}
func (h *fakeIPCHandler) PeerAdd(context.Context, string, string) (domain.Peer, error) {
	return domain.Peer{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStatePending}, nil
}
func (h *fakeIPCHandler) PeerApprove(context.Context, string) (domain.Peer, error) {
	return domain.Peer{PeerID: "peer-a", State: domain.PeerStateApproved}, nil
}
func (h *fakeIPCHandler) PeerRevoke(context.Context, string) (domain.Peer, error) {
	return domain.Peer{PeerID: "peer-a", State: domain.PeerStateRevoked}, nil
}
func (h *fakeIPCHandler) PeerRemove(context.Context, string) error { return nil }
func (h *fakeIPCHandler) PeerList(context.Context) ([]domain.Peer, error) {
	return []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a", State: domain.PeerStateApproved}}, nil
}
func (h *fakeIPCHandler) Status(context.Context) (collabout.DaemonStatus, error) {
	return collabout.DaemonStatus{Online: true, PeerCount: 1, ApprovedPeerCount: 1, PendingOps: 2, NodeID: "node-1", WorkspaceID: "ws-1"}, nil
}
func (h *fakeIPCHandler) ActivityTail(context.Context, collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	return []domain.ActivityEvent{{ID: "1", Type: domain.ActivitySyncApplied, Message: "ok"}}, nil
}
func (h *fakeIPCHandler) ConflictsList(context.Context, string) ([]domain.ConflictRecord, error) {
	return []domain.ConflictRecord{{ID: "c1", EntityKey: "source/s1", Field: "title", Status: domain.ConflictStatusOpen}}, nil
}
func (h *fakeIPCHandler) ConflictResolve(context.Context, string, domain.ConflictStrategy) (domain.ConflictRecord, error) {
	return domain.ConflictRecord{ID: "c1", Status: domain.ConflictStatusResolved, Strategy: "local"}, nil
}
func (h *fakeIPCHandler) SyncNow(context.Context) (int, error) { return 3, nil }
func (h *fakeIPCHandler) SnapshotExport(context.Context) (string, error) {
	return `{"ok":true}`, nil
}
func (h *fakeIPCHandler) Metrics(context.Context) (collabout.MetricsSnapshot, error) {
	return collabout.MetricsSnapshot{WorkspaceID: "ws-1", NodeID: "node-1", ApprovedPeers: 1, PendingOps: 2}, nil
}
func (h *fakeIPCHandler) Stop(context.Context) error {
	h.mu.Lock()
	h.stopped = true
	h.mu.Unlock()
	return nil
}

func TestJSONRPCServerClientContract(t *testing.T) {
	t.Parallel()
	h := &fakeIPCHandler{}
	server := out.NewJSONRPCServer()
	client := out.NewJSONRPCClient()
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("mdht-collab-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx, socketPath, h)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := client.Status(context.Background(), socketPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if _, err := client.WorkspaceRotateKey(context.Background(), socketPath, time.Hour); err != nil {
		t.Fatalf("rotate key: %v", err)
	}
	if _, err := client.PeerApprove(context.Background(), socketPath, "peer-a"); err != nil {
		t.Fatalf("peer approve: %v", err)
	}
	if _, err := client.ActivityTail(context.Background(), socketPath, collabout.ActivityQuery{Limit: 10}); err != nil {
		t.Fatalf("activity tail: %v", err)
	}
	if _, err := client.ConflictsList(context.Background(), socketPath, ""); err != nil {
		t.Fatalf("conflicts list: %v", err)
	}
	if _, err := client.ConflictResolve(context.Background(), socketPath, "c1", domain.ConflictStrategyLocal); err != nil {
		t.Fatalf("conflict resolve: %v", err)
	}
	if _, err := client.SyncNow(context.Background(), socketPath); err != nil {
		t.Fatalf("sync now: %v", err)
	}
	if _, err := client.SnapshotExport(context.Background(), socketPath); err != nil {
		t.Fatalf("snapshot export: %v", err)
	}
	if _, err := client.Metrics(context.Background(), socketPath); err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if err := client.Stop(context.Background(), socketPath); err != nil {
		t.Fatalf("stop rpc: %v", err)
	}

	h.mu.Lock()
	stopped := h.stopped
	h.mu.Unlock()
	if !stopped {
		t.Fatalf("expected stop hook to run")
	}

	cancel()
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("serve exit error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not shut down")
	}
}
