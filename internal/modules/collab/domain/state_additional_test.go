package domain

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestCodedErrorsAndErrorCodeMapping(t *testing.T) {
	t.Parallel()

	withMessage := NewCodedError(ErrorMigrationFailed, "failed to migrate")
	if got := withMessage.Error(); got != "ERR_MIGRATION_FAILED: failed to migrate" {
		t.Fatalf("unexpected coded error string: %s", got)
	}
	withoutMessage := CodedError{Code: ErrorAuthInvalid}
	if got := withoutMessage.Error(); got != "ERR_AUTH_INVALID" {
		t.Fatalf("unexpected coded error without message: %s", got)
	}

	tests := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{name: "nil", err: nil, want: ""},
		{name: "coded", err: NewCodedError(ErrorPeerNotApproved, "nope"), want: ErrorPeerNotApproved},
		{name: "workspace", err: ErrWorkspaceNotInitialized, want: ErrorWorkspaceUninitialized},
		{name: "peer", err: ErrPeerNotApproved, want: ErrorPeerNotApproved},
		{name: "auth", err: ErrInvalidAuthTag, want: ErrorAuthInvalid},
		{name: "mismatch", err: ErrWorkspaceMismatch, want: ErrorWorkspaceMismatch},
		{name: "conflict", err: ErrConflictNotFound, want: ErrorConflictNotFound},
		{name: "strategy", err: ErrConflictStrategyInvalid, want: ErrorConflictStrategy},
		{name: "daemon", err: ErrDaemonNotRunning, want: ErrorDaemonNotRunning},
		{name: "migration", err: ErrMigrationFailed, want: ErrorMigrationFailed},
		{name: "unknown", err: errors.New("x"), want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ErrorCodeFromError(tc.err); got != tc.want {
				t.Fatalf("expected %s got %s", tc.want, got)
			}
		})
	}
}

func TestPeerApprovalAndConflictStrategyValidation(t *testing.T) {
	t.Parallel()

	if !(Peer{State: PeerStateApproved}).IsApproved() {
		t.Fatalf("approved peer should be approved")
	}
	if (Peer{State: PeerStatePending}).IsApproved() {
		t.Fatalf("pending peer should not be approved")
	}
	if (Peer{State: PeerStateRevoked}).IsApproved() {
		t.Fatalf("revoked peer should not be approved")
	}

	for _, strategy := range []ConflictStrategy{ConflictStrategyLocal, ConflictStrategyRemote, ConflictStrategyMerge} {
		if err := strategy.Validate(); err != nil {
			t.Fatalf("strategy %q should validate: %v", strategy, err)
		}
	}
	if err := ConflictStrategy("bad").Validate(); !errors.Is(err, ErrConflictStrategyInvalid) {
		t.Fatalf("expected invalid strategy error, got %v", err)
	}
}

func TestKeyRingResolveKey(t *testing.T) {
	t.Parallel()
	active := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	legacy := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxzy123456"))
	now := time.Now().UTC()
	ring := KeyRing{
		ActiveKeyID: "k2",
		Keys: []KeyRecord{
			{ID: "k1", KeyBase64: legacy, CreatedAt: now.Add(-time.Hour)},
			{ID: "k2", KeyBase64: active, CreatedAt: now},
		},
		GraceUntil: now.Add(time.Hour),
	}

	if key, err := ring.ResolveKey("k2", now); err != nil || len(key) != 32 {
		t.Fatalf("active key should resolve, got len=%d err=%v", len(key), err)
	}
	if key, err := ring.ResolveKey("k1", now); err != nil || len(key) != 32 {
		t.Fatalf("grace key should resolve, got len=%d err=%v", len(key), err)
	}

	expired := ring
	expired.GraceUntil = now.Add(-time.Minute)
	if _, err := expired.ResolveKey("k1", now); !errors.Is(err, ErrInvalidAuthTag) {
		t.Fatalf("expected expired grace key error, got %v", err)
	}
	if _, err := ring.ResolveKey("missing", now); !errors.Is(err, ErrInvalidAuthTag) {
		t.Fatalf("expected missing key error, got %v", err)
	}
}
