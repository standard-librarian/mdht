package usecase_test

import (
	"context"
	"testing"
	"time"

	sessionout "mdht/internal/modules/session/adapter/out"
	sessiondto "mdht/internal/modules/session/dto"
	"mdht/internal/modules/session/service"
	"mdht/internal/modules/session/usecase"
	apperrors "mdht/internal/platform/errors"
)

func TestStartWithoutActiveStore(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{values: []time.Time{time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)}}
	library := &fakeLibrary{}
	uc := usecase.NewInteractor(service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(t.TempDir())), library, nil)
	out, err := uc.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1", SourceTitle: "Title"})
	if err != nil {
		t.Fatalf("start should succeed without active store: %v", err)
	}
	if out.SessionID == "" {
		t.Fatalf("session id should be set")
	}
}

func TestGetActiveWithNilStoreAndEndWithoutLibrary(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	clk := &fakeClock{values: []time.Time{
		time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 25, 10, 10, 0, 0, time.UTC),
	}}
	activeStore := sessionout.NewFileActiveSessionStore(vault)
	ucWithNilStore := usecase.NewInteractor(service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(vault)), &fakeLibrary{}, nil)
	if _, err := ucWithNilStore.GetActive(context.Background()); err != apperrors.ErrNoActiveSession {
		t.Fatalf("expected no active session for nil store, got %v", err)
	}
	ucNoLibrary := usecase.NewInteractor(service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(vault)), nil, activeStore)
	if _, err := ucNoLibrary.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1", SourceTitle: "Title"}); err != nil {
		t.Fatalf("start should succeed: %v", err)
	}
	if _, err := ucNoLibrary.End(context.Background(), sessiondto.EndInput{Outcome: "x", DeltaProgress: 10}); err == nil {
		t.Fatalf("expected end to fail without library")
	}
}
