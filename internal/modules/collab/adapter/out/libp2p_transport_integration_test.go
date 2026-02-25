package out_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	out "mdht/internal/modules/collab/adapter/out"
	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

func TestLibp2pTwoNodeReplicationAndAuthRejection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	workspaceID := "ws-integration"
	workspaceKey := make([]byte, 32)
	if _, err := rand.Read(workspaceKey); err != nil {
		t.Fatalf("workspace key: %v", err)
	}
	node1 := mustNodeIdentity(t)
	node2 := mustNodeIdentity(t)

	received := make(chan domain.OpEnvelope, 16)
	transport := out.NewLibp2pTransport()
	rt1, err := transport.Start(ctx, collabout.TransportStartInput{
		WorkspaceID:  workspaceID,
		WorkspaceKey: workspaceKey,
		NodeIdentity: node1,
	}, collabout.TransportHandlers{})
	if err != nil {
		t.Fatalf("start node1: %v", err)
	}
	defer func() { _ = rt1.Stop() }()

	rt2, err := transport.Start(ctx, collabout.TransportStartInput{
		WorkspaceID:  workspaceID,
		WorkspaceKey: workspaceKey,
		NodeIdentity: node2,
	}, collabout.TransportHandlers{
		OnOp: func(op domain.OpEnvelope) {
			received <- op
		},
	})
	if err != nil {
		t.Fatalf("start node2: %v", err)
	}
	defer func() { _ = rt2.Stop() }()

	peerAddr := dialableAddr(t, rt1.Status().ListenAddrs)
	peerID := parsePeerID(t, peerAddr)
	if err := rt2.AddPeer(ctx, domain.Peer{PeerID: peerID, Address: peerAddr, AddedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("add peer node2->node1: %v", err)
	}

	op1 := newSignedOp(t, workspaceID, node1.NodeID, workspaceKey, "op-1", "source/1", "title", "A")
	if err := rt1.Broadcast(ctx, op1); err != nil {
		t.Fatalf("broadcast op1: %v", err)
	}
	waitForOp(t, received, "op-1")

	if err := rt2.RemovePeer(ctx, peerID); err != nil {
		t.Fatalf("remove peer: %v", err)
	}
	op2 := newSignedOp(t, workspaceID, node1.NodeID, workspaceKey, "op-2", "source/1", "title", "B")
	if err := rt1.Broadcast(ctx, op2); err != nil {
		t.Fatalf("broadcast op2 while disconnected: %v", err)
	}

	if err := rt2.AddPeer(ctx, domain.Peer{PeerID: peerID, Address: peerAddr, AddedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("re-add peer: %v", err)
	}
	if err := rt1.Reconcile(ctx, []domain.OpEnvelope{op1, op2}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	waitForOp(t, received, "op-2")

	wrongKey := make([]byte, 32)
	if _, err := rand.Read(wrongKey); err != nil {
		t.Fatalf("wrong key: %v", err)
	}
	node3 := mustNodeIdentity(t)
	rtBad, err := transport.Start(ctx, collabout.TransportStartInput{
		WorkspaceID:  workspaceID,
		WorkspaceKey: wrongKey,
		NodeIdentity: node3,
	}, collabout.TransportHandlers{})
	if err != nil {
		t.Fatalf("start bad node: %v", err)
	}
	defer func() { _ = rtBad.Stop() }()

	err = rtBad.AddPeer(ctx, domain.Peer{PeerID: peerID, Address: peerAddr, AddedAt: time.Now().UTC()})
	if err == nil {
		t.Fatalf("expected auth error from wrong workspace key")
	}
	if !errors.Is(err, domain.ErrInvalidAuthTag) && !errors.Is(err, domain.ErrWorkspaceMismatch) {
		t.Fatalf("unexpected auth error: %v", err)
	}
}

func mustNodeIdentity(t *testing.T) domain.NodeIdentity {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return domain.NodeIdentity{
		NodeID:     hex.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(priv),
	}
}

func newSignedOp(t *testing.T, workspaceID, nodeID string, key []byte, opID, entity, field, value string) domain.OpEnvelope {
	t.Helper()
	payload, err := json.Marshal(domain.RegisterPayload{Field: field, Value: mustRaw(value)})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	op := domain.OpEnvelope{
		WorkspaceID:  workspaceID,
		NodeID:       nodeID,
		EntityKey:    entity,
		OpID:         opID,
		HLCTimestamp: domain.HLC{Wall: time.Now().UTC().UnixMilli(), Counter: 1, NodeID: nodeID}.String(),
		OpKind:       domain.OpKindPutRegister,
		Payload:      payload,
	}
	return op.Signed(key)
}

func waitForOp(t *testing.T, ch <-chan domain.OpEnvelope, opID string) {
	t.Helper()
	deadline := time.After(8 * time.Second)
	for {
		select {
		case op := <-ch:
			if op.OpID == opID {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for op id %s", opID)
		}
	}
}

func dialableAddr(t *testing.T, addrs []string) string {
	t.Helper()
	for _, addr := range addrs {
		if strings.Contains(addr, "/ip4/") && strings.Contains(addr, "/tcp/") {
			addr = strings.Replace(addr, "/ip4/0.0.0.0/", "/ip4/127.0.0.1/", 1)
			return addr
		}
	}
	t.Fatalf("no dialable listen address in %+v", addrs)
	return ""
}

func parsePeerID(t *testing.T, addr string) string {
	t.Helper()
	parts := strings.Split(addr, "/")
	for idx := 0; idx < len(parts)-1; idx++ {
		if parts[idx] == "p2p" {
			return parts[idx+1]
		}
	}
	t.Fatalf("peer id missing in addr: %s", addr)
	return ""
}

func mustRaw(v string) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
