package out_test

import (
	"context"
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
	return domain.Workspace{ID: "ws-1", Name: name, CreatedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)}, nil
}
func (h *fakeIPCHandler) WorkspaceShow(context.Context) (domain.Workspace, string, []domain.Peer, error) {
	return domain.Workspace{ID: "ws-1", Name: "alpha"}, "node-1", []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}}, nil
}
func (h *fakeIPCHandler) PeerAdd(context.Context, string) (domain.Peer, error) {
	return domain.Peer{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}, nil
}
func (h *fakeIPCHandler) PeerRemove(context.Context, string) error { return nil }
func (h *fakeIPCHandler) PeerList(context.Context) ([]domain.Peer, error) {
	return []domain.Peer{{PeerID: "peer-a", Address: "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"}}, nil
}
func (h *fakeIPCHandler) Status(context.Context) (collabout.DaemonStatus, error) {
	return collabout.DaemonStatus{Online: true, PeerCount: 1, PendingOps: 2, NodeID: "node-1", WorkspaceID: "ws-1"}, nil
}
func (h *fakeIPCHandler) ReconcileNow(context.Context) (int, error)   { return 3, nil }
func (h *fakeIPCHandler) ExportState(context.Context) (string, error) { return `{"ok":true}`, nil }
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
	socketPath := filepath.Join(t.TempDir(), "daemon.sock")
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

	ws, err := client.WorkspaceInit(context.Background(), socketPath, "alpha")
	if err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	if ws.Name != "alpha" {
		t.Fatalf("unexpected workspace init output: %+v", ws)
	}

	shown, nodeID, peers, err := client.WorkspaceShow(context.Background(), socketPath)
	if err != nil {
		t.Fatalf("workspace show: %v", err)
	}
	if shown.ID != "ws-1" || nodeID != "node-1" || len(peers) != 1 {
		t.Fatalf("unexpected workspace show output")
	}

	if _, err := client.PeerAdd(context.Background(), socketPath, "/ip4/127.0.0.1/tcp/4001/p2p/peer-a"); err != nil {
		t.Fatalf("peer add: %v", err)
	}
	if err := client.PeerRemove(context.Background(), socketPath, "peer-a"); err != nil {
		t.Fatalf("peer remove: %v", err)
	}
	listed, err := client.PeerList(context.Background(), socketPath)
	if err != nil {
		t.Fatalf("peer list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("unexpected peer list output")
	}

	status, err := client.Status(context.Background(), socketPath)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Online || status.WorkspaceID != "ws-1" {
		t.Fatalf("unexpected status output: %+v", status)
	}

	reconciled, err := client.ReconcileNow(context.Background(), socketPath)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if reconciled != 3 {
		t.Fatalf("unexpected reconcile output: %d", reconciled)
	}

	exported, err := client.ExportState(context.Background(), socketPath)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exported == "" {
		t.Fatalf("expected export payload")
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
