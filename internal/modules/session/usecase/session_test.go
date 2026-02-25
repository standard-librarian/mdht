package usecase_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mdht/internal/modules/library/dto"
	sessionout "mdht/internal/modules/session/adapter/out"
	sessiondto "mdht/internal/modules/session/dto"
	"mdht/internal/modules/session/service"
	"mdht/internal/modules/session/usecase"
	apperrors "mdht/internal/platform/errors"
)

type fakeClock struct {
	values []time.Time
	idx    int
}

func (f *fakeClock) Now() time.Time {
	if f.idx >= len(f.values) {
		return f.values[len(f.values)-1]
	}
	v := f.values[f.idx]
	f.idx++
	return v
}

type fakeID struct{}

func (fakeID) New() string { return "sess-1" }

type fakeLibrary struct {
	source    dto.SourceDetailOutput
	updated   float64
	updatedID string
}

func (f *fakeLibrary) IngestFile(context.Context, dto.IngestFileInput) (dto.SourceOutput, error) {
	return dto.SourceOutput{}, nil
}
func (f *fakeLibrary) IngestURL(context.Context, dto.IngestURLInput) (dto.SourceOutput, error) {
	return dto.SourceOutput{}, nil
}
func (f *fakeLibrary) Reindex(context.Context, dto.ReindexInput) error         { return nil }
func (f *fakeLibrary) ListSources(context.Context) ([]dto.SourceOutput, error) { return nil, nil }
func (f *fakeLibrary) GetSource(context.Context, string) (dto.SourceDetailOutput, error) {
	return f.source, nil
}
func (f *fakeLibrary) UpdateProgress(_ context.Context, input dto.UpdateProgressInput) (dto.SourceOutput, error) {
	f.updated = input.Percent
	f.updatedID = input.SourceID
	f.source.Percent = input.Percent
	return dto.SourceOutput{ID: input.SourceID, Percent: input.Percent}, nil
}

func TestSessionLifecycleWithActiveStateAndCumulativeProgress(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	clk := &fakeClock{values: []time.Time{
		time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 25, 10, 45, 0, 0, time.UTC),
	}}
	library := &fakeLibrary{source: dto.SourceDetailOutput{ID: "src-1", Title: "Go Book", Percent: 20}}
	svc := service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(vault))
	uc := usecase.NewInteractor(svc, library, sessionout.NewFileActiveSessionStore(vault))

	start, err := uc.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1", SourceTitle: "Go Book", Goal: "Read chapter 1"})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if start.SessionID == "" {
		t.Fatalf("session id must be set")
	}

	active, err := uc.GetActive(context.Background())
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if active.SessionID != start.SessionID {
		t.Fatalf("expected same active session id, got %s vs %s", active.SessionID, start.SessionID)
	}

	end, err := uc.End(context.Background(), sessiondto.EndInput{Outcome: "Completed chapter", DeltaProgress: 35})
	if err != nil {
		t.Fatalf("end session: %v", err)
	}
	if end.DurationMin != 45 {
		t.Fatalf("expected 45 minutes, got %d", end.DurationMin)
	}
	if end.ProgressBefore != 20 || end.ProgressAfter != 55 {
		t.Fatalf("expected cumulative progress 20->55, got %.2f->%.2f", end.ProgressBefore, end.ProgressAfter)
	}
	if library.updated != 55 {
		t.Fatalf("expected library progress update to 55, got %.2f", library.updated)
	}

	if _, err := uc.GetActive(context.Background()); err != apperrors.ErrNoActiveSession {
		t.Fatalf("expected no active session after end, got %v", err)
	}
	b, err := os.ReadFile(end.Path)
	if err != nil {
		t.Fatalf("read session note: %v", err)
	}
	note := string(b)
	if !strings.Contains(note, "progress_before: 20") || !strings.Contains(note, "progress_after: 55") {
		t.Fatalf("session note missing progress before/after fields: %s", note)
	}
}

func TestStartFailsWhenActiveExistsAndLoadsTitleFromLibrary(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	clk := &fakeClock{values: []time.Time{time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)}}
	library := &fakeLibrary{source: dto.SourceDetailOutput{ID: "src-1", Title: "Loaded Title", Percent: 0}}
	uc := usecase.NewInteractor(
		service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(vault)),
		library,
		sessionout.NewFileActiveSessionStore(vault),
	)
	start, err := uc.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1"})
	if err != nil {
		t.Fatalf("first start should succeed: %v", err)
	}
	active, err := uc.GetActive(context.Background())
	if err != nil {
		t.Fatalf("get active after start: %v", err)
	}
	if active.SourceTitle != "Loaded Title" || start.SourceID != "src-1" {
		t.Fatalf("expected title loaded from library, got %+v", active)
	}
	if _, err := uc.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1", SourceTitle: "Go Book"}); err != apperrors.ErrActiveSessionExists {
		t.Fatalf("expected active session exists error, got %v", err)
	}
}

func TestEndFailsWithoutActiveNegativeDeltaAndMismatchedSessionID(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	clk := &fakeClock{values: []time.Time{
		time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 25, 10, 5, 0, 0, time.UTC),
	}}
	library := &fakeLibrary{source: dto.SourceDetailOutput{ID: "src-1", Title: "Go Book", Percent: 95}}
	uc := usecase.NewInteractor(
		service.NewSessionService(clk, fakeID{}, sessionout.NewVaultSessionStore(vault)),
		library,
		sessionout.NewFileActiveSessionStore(vault),
	)
	if _, err := uc.End(context.Background(), sessiondto.EndInput{Outcome: "x", DeltaProgress: 10}); err != apperrors.ErrNoActiveSession {
		t.Fatalf("expected no active session error, got %v", err)
	}
	if _, err := uc.End(context.Background(), sessiondto.EndInput{Outcome: "x", DeltaProgress: -1}); err == nil {
		t.Fatalf("negative delta must fail")
	}
	if _, err := uc.Start(context.Background(), sessiondto.StartInput{SourceID: "src-1", SourceTitle: "Go Book"}); err != nil {
		t.Fatalf("start for mismatch case: %v", err)
	}
	if _, err := uc.End(context.Background(), sessiondto.EndInput{SessionID: "other", Outcome: "x", DeltaProgress: 10}); err == nil {
		t.Fatalf("mismatched session id should fail")
	}
	end, err := uc.End(context.Background(), sessiondto.EndInput{Outcome: "x", DeltaProgress: 20})
	if err != nil {
		t.Fatalf("end session: %v", err)
	}
	if end.ProgressAfter != 100 {
		t.Fatalf("expected clamp to 100, got %.2f", end.ProgressAfter)
	}
}
