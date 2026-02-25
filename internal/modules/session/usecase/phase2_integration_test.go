package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	libraryout "mdht/internal/modules/library/adapter/out"
	"mdht/internal/modules/library/dto"
	"mdht/internal/modules/library/service"
	libraryusecase "mdht/internal/modules/library/usecase"
	readerout "mdht/internal/modules/reader/adapter/out"
	readerdto "mdht/internal/modules/reader/dto"
	readerservice "mdht/internal/modules/reader/service"
	readerusecase "mdht/internal/modules/reader/usecase"
	sessionout "mdht/internal/modules/session/adapter/out"
	sessiondto "mdht/internal/modules/session/dto"
	sessionservice "mdht/internal/modules/session/service"
	sessionusecase "mdht/internal/modules/session/usecase"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/id"
)

type noopLauncher struct{}

func (noopLauncher) Open(context.Context, string) error { return nil }

func TestEndToEndStudyLoop(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".mdht", "mdht.db")
	sourceFile := filepath.Join(vault, "book.md")
	if err := os.WriteFile(sourceFile, []byte("# chapter 1"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	store := libraryout.NewVaultSourceStore(vault)
	projector, err := libraryout.NewSQLiteSourceProjector(dbPath)
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	libraryUC := libraryusecase.NewInteractor(service.NewSourceService(clock.SystemClock{}, id.RandomHex{}, store, projector), nil)
	sessionUC := sessionusecase.NewInteractor(
		sessionservice.NewSessionService(clock.SystemClock{}, id.RandomHex{}, sessionout.NewVaultSessionStore(vault)),
		libraryUC,
		sessionout.NewFileActiveSessionStore(vault),
	)
	readerUC := readerusecase.NewInteractor(readerservice.NewReaderService(
		readerout.NewLocalMarkdownReader(),
		readerout.NewLocalPDFReader(),
		readerout.NewLibraryProgressAdapter(libraryUC),
		readerout.NewLibrarySourceAdapter(libraryUC),
		noopLauncher{},
	))

	ingested, err := libraryUC.IngestFile(context.Background(), dto.IngestFileInput{Path: sourceFile, Type: "book", Title: "Book One"})
	if err != nil {
		t.Fatalf("ingest file: %v", err)
	}
	if _, err := sessionUC.Start(context.Background(), sessiondto.StartInput{SourceID: ingested.ID, Goal: "Study"}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	opened, err := readerUC.OpenSource(context.Background(), readerdto.OpenSourceInput{SourceID: ingested.ID, Mode: "auto"})
	if err != nil {
		t.Fatalf("reader open: %v", err)
	}
	if opened.Mode != "markdown" {
		t.Fatalf("expected markdown mode, got %s", opened.Mode)
	}
	ended, err := sessionUC.End(context.Background(), sessiondto.EndInput{Outcome: "Done", DeltaProgress: 30})
	if err != nil {
		t.Fatalf("end session: %v", err)
	}
	if err := libraryUC.Reindex(context.Background(), dto.ReindexInput{}); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	source, err := libraryUC.GetSource(context.Background(), ingested.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if source.Percent != 30 {
		t.Fatalf("expected 30%% progress after session end, got %.2f", source.Percent)
	}
	if _, err := os.Stat(ended.Path); err != nil {
		t.Fatalf("expected session note to exist: %v", err)
	}
}
