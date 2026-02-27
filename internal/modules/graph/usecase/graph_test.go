package usecase_test

import (
	"context"
	"testing"

	"mdht/internal/modules/graph/domain"
	"mdht/internal/modules/graph/dto"
	graphin "mdht/internal/modules/graph/port/in"
	"mdht/internal/modules/graph/service"
	"mdht/internal/modules/graph/usecase"
)

type fakeTopicStore struct{ called int }

func (f *fakeTopicStore) AppendSourceLink(context.Context, string, string, string) error {
	f.called++
	return nil
}

type fakeLinkProjector struct{ called int }

func (f *fakeLinkProjector) Upsert(context.Context, domain.Link) error {
	f.called++
	return nil
}

type fakeQueryStore struct {
	topics []domain.TopicSummary
	nodes  []domain.Node
	path   []domain.Node
}

func (f *fakeQueryStore) ListTopics(context.Context, int) ([]domain.TopicSummary, error) {
	return f.topics, nil
}

func (f *fakeQueryStore) Neighbors(context.Context, string, int) ([]domain.Node, error) {
	return f.nodes, nil
}

func (f *fakeQueryStore) Search(context.Context, string) ([]domain.Node, error) {
	return f.nodes, nil
}

func (f *fakeQueryStore) ShortestPath(context.Context, string, string) ([]domain.Node, error) {
	return f.path, nil
}

func TestSyncSourceDelegatesToService(t *testing.T) {
	t.Parallel()
	topic := &fakeTopicStore{}
	proj := &fakeLinkProjector{}
	query := &fakeQueryStore{}
	svc := service.NewGraphService(topic, proj, query)
	uc := usecase.NewInteractor(svc)
	if err := uc.SyncSource(context.Background(), graphin.SyncSourceInput{SourceID: "s", SourceTitle: "t", Topics: []string{"go"}}); err != nil {
		t.Fatalf("sync source: %v", err)
	}
	if topic.called != 1 || proj.called != 1 {
		t.Fatalf("expected topic and projector calls, got %d/%d", topic.called, proj.called)
	}
}

func TestExploreMethodsMapDomainToDTO(t *testing.T) {
	t.Parallel()
	topic := &fakeTopicStore{}
	proj := &fakeLinkProjector{}
	query := &fakeQueryStore{
		topics: []domain.TopicSummary{{TopicSlug: "go", SourceCount: 2}},
		nodes: []domain.Node{
			{ID: "src-1", Label: "Go Book", Kind: domain.NodeKindSource},
			{ID: "go", Label: "go", Kind: domain.NodeKindTopic},
		},
		path: []domain.Node{
			{ID: "src-1", Label: "Go Book", Kind: domain.NodeKindSource},
			{ID: "go", Label: "go", Kind: domain.NodeKindTopic},
		},
	}
	svc := service.NewGraphService(topic, proj, query)
	uc := usecase.NewInteractor(svc)

	topics, err := uc.ListTopics(context.Background(), 10)
	if err != nil {
		t.Fatalf("list topics: %v", err)
	}
	if len(topics) != 1 || topics[0] != (dto.TopicSummaryOutput{TopicSlug: "go", SourceCount: 2}) {
		t.Fatalf("unexpected topics: %#v", topics)
	}

	neighbors, err := uc.Neighbors(context.Background(), graphin.NeighborsInput{NodeID: "src-1", Depth: 2})
	if err != nil {
		t.Fatalf("neighbors: %v", err)
	}
	if neighbors.FocusID != "src-1" || neighbors.Depth != 2 || len(neighbors.Nodes) != 2 {
		t.Fatalf("unexpected neighbors: %#v", neighbors)
	}

	search, err := uc.Search(context.Background(), "go")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(search) != 2 {
		t.Fatalf("expected two search results, got %d", len(search))
	}

	path, err := uc.Path(context.Background(), graphin.PathInput{FromID: "src-1", ToID: "go"})
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if !path.Found || len(path.Nodes) != 2 {
		t.Fatalf("unexpected path result: %#v", path)
	}
}
