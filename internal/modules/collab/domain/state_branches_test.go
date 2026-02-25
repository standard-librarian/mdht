package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestOpKindAndWorkspaceValidation(t *testing.T) {
	t.Parallel()
	if err := OpKindPutRegister.Validate(); err != nil {
		t.Fatalf("put_register should be valid: %v", err)
	}
	if err := OpKind("bad").Validate(); !errors.Is(err, ErrUnknownOpKind) {
		t.Fatalf("expected unknown op kind error, got %v", err)
	}

	if err := (Workspace{}).Validate(); err == nil {
		t.Fatalf("expected workspace validation error")
	}
	if err := (Workspace{ID: "ws-1", Name: "alpha"}).Validate(); err != nil {
		t.Fatalf("workspace should validate: %v", err)
	}
}

func TestHLCHelpers(t *testing.T) {
	t.Parallel()
	if parsed := ParseHLC("invalid"); parsed != (HLC{}) {
		t.Fatalf("invalid hlc should parse to zero value")
	}
	a := HLC{Wall: 10, Counter: 2, NodeID: "a"}
	b := HLC{Wall: 10, Counter: 2, NodeID: "b"}
	if CompareHLC(a, b) >= 0 {
		t.Fatalf("node id tie-break expected a < b")
	}
	if CompareHLC(b, a) <= 0 {
		t.Fatalf("node id tie-break expected b > a")
	}

	next := NextHLC(time.UnixMilli(9), HLC{Wall: 10, Counter: 3, NodeID: "a"}, "n")
	if next.Wall != 10 || next.Counter != 4 {
		t.Fatalf("expected monotonic next hlc, got %+v", next)
	}
}

func TestOpEnvelopeValidateAndVerifyBranches(t *testing.T) {
	t.Parallel()
	base := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op1", HLCTimestamp: HLC{Wall: 1, Counter: 0, NodeID: "n"}.String(), OpKind: OpKindReconcileHint, Payload: mustRaw(map[string]any{"x": 1})}
	if err := base.Validate(); err != nil {
		t.Fatalf("base envelope should validate: %v", err)
	}
	cases := []OpEnvelope{
		{NodeID: "n", EntityKey: "source/1", OpID: "op1", HLCTimestamp: base.HLCTimestamp, OpKind: OpKindReconcileHint},
		{WorkspaceID: "w", EntityKey: "source/1", OpID: "op1", HLCTimestamp: base.HLCTimestamp, OpKind: OpKindReconcileHint},
		{WorkspaceID: "w", NodeID: "n", OpID: "op1", HLCTimestamp: base.HLCTimestamp, OpKind: OpKindReconcileHint},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", HLCTimestamp: base.HLCTimestamp, OpKind: OpKindReconcileHint},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op1", OpKind: OpKindReconcileHint},
		{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op1", HLCTimestamp: base.HLCTimestamp, OpKind: OpKind("invalid")},
	}
	for i, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("case %d expected validation error", i)
		}
	}

	key := []byte("12345678901234567890123456789012")
	signed := base.Signed(key)
	if !signed.Verify(key) {
		t.Fatalf("signed envelope should verify")
	}

	noTag := base
	if noTag.Verify(key) {
		t.Fatalf("empty auth tag must not verify")
	}

	badHex := signed
	badHex.AuthTag = "zz"
	if badHex.Verify(key) {
		t.Fatalf("invalid auth tag hex must not verify")
	}
}

func TestCRDTStateCloneAndCompaction(t *testing.T) {
	t.Parallel()
	state := NewCRDTState()
	state.Entities["source/1"] = EntityState{Registers: map[string]RegisterValue{"title": {Value: mustRaw("book"), Meta: HLC{Wall: 1, Counter: 0, NodeID: "n"}}}}
	state.AppliedOps["op-1"] = struct{}{}
	clone := state.Clone()
	clone.Entities["source/1"] = EntityState{}
	if _, ok := state.Entities["source/1"]; !ok {
		t.Fatalf("clone mutation should not affect original")
	}

	compacted := NewCRDTState()
	for i := 0; i < 51000; i++ {
		compacted.AppliedOps[fmt.Sprintf("op-%05d", i)] = struct{}{}
	}
	compacted.compactAppliedOps()
	if len(compacted.AppliedOps) != 30000 {
		t.Fatalf("expected compacted applied op size 30000, got %d", len(compacted.AppliedOps))
	}
}

func TestApplyErrorBranchesAndReconcileNoop(t *testing.T) {
	t.Parallel()
	state := NewCRDTState()
	validBase := OpEnvelope{WorkspaceID: "w", NodeID: "n", EntityKey: "source/1", OpID: "op", HLCTimestamp: HLC{Wall: 1, Counter: 0, NodeID: "n"}.String()}

	badPayloadCases := []struct {
		kind OpKind
		opID string
	}{
		{kind: OpKindPutRegister, opID: "a"},
		{kind: OpKindAddSet, opID: "b"},
		{kind: OpKindRemoveSet, opID: "c"},
		{kind: OpKindInsertSeq, opID: "d"},
		{kind: OpKindDeleteSeq, opID: "e"},
	}
	for _, tc := range badPayloadCases {
		op := validBase
		op.OpKind = tc.kind
		op.OpID = tc.opID
		op.Payload = json.RawMessage("{")
		if err := state.Apply(op); err == nil {
			t.Fatalf("expected decode error for kind=%s", tc.kind)
		}
	}

	reconcile := validBase
	reconcile.OpKind = OpKindReconcileHint
	reconcile.OpID = "hint"
	reconcile.Payload = mustRaw(map[string]any{"tick": 1})
	if err := state.Apply(reconcile); err != nil {
		t.Fatalf("reconcile hint should apply as no-op metadata update: %v", err)
	}
}

func TestRenderSetAndRandomWorkspaceKeyErrors(t *testing.T) {
	t.Parallel()
	entity := EntityState{
		Sets: map[string]ORSetValue{
			"topics": {
				Adds: map[string]HLC{
					"a": {Wall: 1, Counter: 0, NodeID: "n"},
					"b": {Wall: 1, Counter: 1, NodeID: "n"},
				},
				Removes: map[string]HLC{
					"a": {Wall: 2, Counter: 0, NodeID: "n"},
				},
			},
		},
	}
	if got := entity.RenderSet("topics"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("unexpected rendered set: %v", got)
	}

	if _, err := RandomWorkspaceKey("not-base64"); err == nil {
		t.Fatalf("expected decode error for invalid base64")
	}
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := RandomWorkspaceKey(short); err == nil {
		t.Fatalf("expected short key error")
	}
}
