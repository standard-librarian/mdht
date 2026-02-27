package usecase_test

import (
	"context"
	"path/filepath"
	"testing"

	graphdto "mdht/internal/modules/graph/dto"
	graphin "mdht/internal/modules/graph/port/in"
	libraryout "mdht/internal/modules/library/adapter/out"
	"mdht/internal/modules/library/dto"
	"mdht/internal/modules/library/service"
	"mdht/internal/modules/library/usecase"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/id"
)

type fakeURLGraph struct{ called int }

func (f *fakeURLGraph) SyncSource(context.Context, graphin.SyncSourceInput) error {
	f.called++
	return nil
}

func (f *fakeURLGraph) ListTopics(context.Context, int) ([]graphdto.TopicSummaryOutput, error) {
	return nil, nil
}

func (f *fakeURLGraph) Neighbors(context.Context, graphin.NeighborsInput) (graphdto.NeighborsOutput, error) {
	return graphdto.NeighborsOutput{}, nil
}

func (f *fakeURLGraph) Search(context.Context, string) ([]graphdto.NodeOutput, error) {
	return nil, nil
}

func (f *fakeURLGraph) Path(context.Context, graphin.PathInput) (graphdto.PathOutput, error) {
	return graphdto.PathOutput{}, nil
}

func TestIngestURLCreatesSourceAndCanBeRead(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	projector, err := libraryout.NewSQLiteSourceProjector(filepath.Join(vault, ".mdht", "mdht.db"))
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	graph := &fakeURLGraph{}
	uc := usecase.NewInteractor(service.NewSourceService(clock.SystemClock{}, id.RandomHex{}, libraryout.NewVaultSourceStore(vault), projector), graph)
	created, err := uc.IngestURL(context.Background(), dto.IngestURLInput{
		URL:   "https://example.com/article",
		Type:  "article",
		Title: "Example Article",
	})
	if err != nil {
		t.Fatalf("ingest url: %v", err)
	}
	if graph.called != 1 {
		t.Fatalf("expected graph sync for URL ingest")
	}
	source, err := uc.GetSource(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if source.URL != "https://example.com/article" {
		t.Fatalf("expected URL to be set, got %s", source.URL)
	}
}

func TestGetAndUpdateProgressErrorsOnUnknownSource(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	projector, err := libraryout.NewSQLiteSourceProjector(filepath.Join(vault, ".mdht", "mdht.db"))
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	uc := usecase.NewInteractor(service.NewSourceService(clock.SystemClock{}, id.RandomHex{}, libraryout.NewVaultSourceStore(vault), projector), nil)
	if _, err := uc.GetSource(context.Background(), "missing"); err == nil {
		t.Fatalf("expected missing source error")
	}
	if _, err := uc.UpdateProgress(context.Background(), dto.UpdateProgressInput{SourceID: "missing", Percent: 10}); err == nil {
		t.Fatalf("expected update progress error for missing source")
	}
}
