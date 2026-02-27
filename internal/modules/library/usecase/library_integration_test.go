package usecase_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	graphdto "mdht/internal/modules/graph/dto"
	graphin "mdht/internal/modules/graph/port/in"
	libraryout "mdht/internal/modules/library/adapter/out"
	"mdht/internal/modules/library/dto"
	"mdht/internal/modules/library/service"
	"mdht/internal/modules/library/usecase"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/id"

	_ "modernc.org/sqlite"
)

type fakeGraph struct{ called int }

func (f *fakeGraph) SyncSource(context.Context, graphin.SyncSourceInput) error {
	f.called++
	return nil
}

func (f *fakeGraph) ListTopics(context.Context, int) ([]graphdto.TopicSummaryOutput, error) {
	return nil, nil
}

func (f *fakeGraph) Neighbors(context.Context, graphin.NeighborsInput) (graphdto.NeighborsOutput, error) {
	return graphdto.NeighborsOutput{}, nil
}

func (f *fakeGraph) Search(context.Context, string) ([]graphdto.NodeOutput, error) {
	return nil, nil
}

func (f *fakeGraph) Path(context.Context, graphin.PathInput) (graphdto.PathOutput, error) {
	return graphdto.PathOutput{}, nil
}

func TestIngestListGetUpdateAndReindex(t *testing.T) {
	t.Parallel()
	vault := t.TempDir()
	dbPath := filepath.Join(vault, ".mdht", "mdht.db")
	sourceFile := filepath.Join(vault, "sample.md")
	if err := os.WriteFile(sourceFile, []byte("# sample"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	store := libraryout.NewVaultSourceStore(vault)
	projector, err := libraryout.NewSQLiteSourceProjector(dbPath)
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	fg := &fakeGraph{}
	uc := usecase.NewInteractor(service.NewSourceService(clock.SystemClock{}, id.RandomHex{}, store, projector), fg)

	out, err := uc.IngestFile(context.Background(), dto.IngestFileInput{
		Path:   sourceFile,
		Type:   "book",
		Title:  "Go In Action",
		Topics: []string{"Golang"},
	})
	if err != nil {
		t.Fatalf("ingest file: %v", err)
	}
	if fg.called != 1 {
		t.Fatalf("expected graph sync call, got %d", fg.called)
	}

	content, err := os.ReadFile(out.NotePath)
	if err != nil {
		t.Fatalf("read source note: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "<!-- mdht:links:start -->") || !strings.Contains(text, "[[golang]]") {
		t.Fatalf("managed links were not rendered as expected: %s", text)
	}

	list, err := uc.ListSources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(list) != 1 || list[0].ID != out.ID {
		t.Fatalf("unexpected list result: %+v", list)
	}

	detail, err := uc.GetSource(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if detail.FilePath != sourceFile {
		t.Fatalf("expected source file path %s, got %s", sourceFile, detail.FilePath)
	}

	if _, err := uc.UpdateProgress(context.Background(), dto.UpdateProgressInput{SourceID: out.ID, Percent: 45}); err != nil {
		t.Fatalf("update progress: %v", err)
	}
	detail, err = uc.GetSource(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("get source after progress update: %v", err)
	}
	if detail.Percent != 45 {
		t.Fatalf("expected 45%% progress, got %.2f", detail.Percent)
	}

	if err := uc.Reindex(context.Background(), dto.ReindexInput{}); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&count); err != nil {
		t.Fatalf("count sources: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one projected source, got %d", count)
	}
}
