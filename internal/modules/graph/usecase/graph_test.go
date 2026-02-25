package usecase_test

import (
	"context"
	"testing"

	"mdht/internal/modules/graph/domain"
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

func TestSyncSourceDelegatesToService(t *testing.T) {
	t.Parallel()
	topic := &fakeTopicStore{}
	proj := &fakeLinkProjector{}
	svc := service.NewGraphService(topic, proj)
	uc := usecase.NewInteractor(svc)
	if err := uc.SyncSource(context.Background(), graphin.SyncSourceInput{SourceID: "s", SourceTitle: "t", Topics: []string{"go"}}); err != nil {
		t.Fatalf("sync source: %v", err)
	}
	if topic.called != 1 || proj.called != 1 {
		t.Fatalf("expected topic and projector calls, got %d/%d", topic.called, proj.called)
	}
}
