package domain

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestCRDTApplyRegisterLWW(t *testing.T) {
	t.Parallel()
	state := NewCRDTState()
	oldPayload, _ := json.Marshal(RegisterPayload{Field: "title", Value: mustRaw("old")})
	newPayload, _ := json.Marshal(RegisterPayload{Field: "title", Value: mustRaw("new")})

	old := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op-1", HLCTimestamp: HLC{Wall: 1, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindPutRegister, Payload: oldPayload}
	new := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op-2", HLCTimestamp: HLC{Wall: 2, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindPutRegister, Payload: newPayload}

	if err := state.Apply(old); err != nil {
		t.Fatalf("apply old: %v", err)
	}
	if err := state.Apply(new); err != nil {
		t.Fatalf("apply new: %v", err)
	}
	entity := state.Entities["source/1"]
	if got := decodeString(t, entity.Registers["title"].Value); got != "new" {
		t.Fatalf("expected newest register value, got %q", got)
	}
}

func TestCRDTApplySetAndSequence(t *testing.T) {
	t.Parallel()
	state := NewCRDTState()
	addTag, _ := json.Marshal(SetPayload{Field: "tags", Value: "distributed"})
	removeTag, _ := json.Marshal(SetPayload{Field: "tags", Value: "distributed"})
	insert1, _ := json.Marshal(InsertSequencePayload{Field: "managed_links", LineID: "l1", AfterID: "", Value: "- [[A]]"})
	insert2, _ := json.Marshal(InsertSequencePayload{Field: "managed_links", LineID: "l2", AfterID: "l1", Value: "- [[B]]"})
	delete2, _ := json.Marshal(DeleteSequencePayload{Field: "managed_links", LineID: "l2"})

	ops := []OpEnvelope{
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "1", HLCTimestamp: HLC{Wall: 1, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindAddSet, Payload: addTag},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "2", HLCTimestamp: HLC{Wall: 2, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindRemoveSet, Payload: removeTag},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "3", HLCTimestamp: HLC{Wall: 3, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindInsertSeq, Payload: insert1},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "4", HLCTimestamp: HLC{Wall: 4, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindInsertSeq, Payload: insert2},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "5", HLCTimestamp: HLC{Wall: 5, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindDeleteSeq, Payload: delete2},
	}
	for _, op := range ops {
		if err := state.Apply(op); err != nil {
			t.Fatalf("apply %s: %v", op.OpID, err)
		}
	}

	entity := state.Entities["source/1"]
	if tags := entity.RenderSet("tags"); len(tags) != 0 {
		t.Fatalf("expected tag to be removed, got %v", tags)
	}
	if lines := entity.RenderSequence("managed_links"); len(lines) != 1 || lines[0] != "- [[A]]" {
		t.Fatalf("unexpected sequence lines: %v", lines)
	}
}

func TestCRDTIdempotentAndTombstoneReset(t *testing.T) {
	t.Parallel()
	state := NewCRDTState()
	put, _ := json.Marshal(RegisterPayload{Field: "title", Value: mustRaw("one")})
	tombstone, _ := json.Marshal(map[string]any{"reason": "refresh"})
	op := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "topic/x", OpID: "dup", HLCTimestamp: HLC{Wall: 1, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindPutRegister, Payload: put}
	if err := state.Apply(op); err != nil {
		t.Fatalf("apply first op: %v", err)
	}
	if err := state.Apply(op); err != nil {
		t.Fatalf("apply duplicate op: %v", err)
	}
	if state.AppliedCount != 1 {
		t.Fatalf("expected idempotent count=1 got=%d", state.AppliedCount)
	}
	if err := state.Apply(OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "topic/x", OpID: "t1", HLCTimestamp: HLC{Wall: 2, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindTombstone, Payload: tombstone}); err != nil {
		t.Fatalf("apply tombstone: %v", err)
	}
	entity := state.Entities["topic/x"]
	if !entity.Tombstone {
		t.Fatalf("expected tombstone=true")
	}
	if len(entity.Registers) != 0 {
		t.Fatalf("expected tombstone reset registers")
	}
}

func TestSignedVerifyAndWorkspaceKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	op := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op", HLCTimestamp: HLC{Wall: time.Now().UnixMilli(), Counter: 1, NodeID: "n"}.String(), OpKind: OpKindReconcileHint, Payload: mustRaw(map[string]any{"x": 1})}
	signed := op.Signed(key)
	if !signed.Verify(key) {
		t.Fatalf("expected signed op to verify")
	}
	bad := make([]byte, len(key))
	copy(bad, key)
	bad[0] = 0
	if signed.Verify(bad) {
		t.Fatalf("expected verify failure with wrong key")
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	decoded, err := RandomWorkspaceKey(encoded)
	if err != nil {
		t.Fatalf("decode workspace key: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(decoded))
	}
}

func mustRaw(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func decodeString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	out := ""
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode string: %v", err)
	}
	return out
}
