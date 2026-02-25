package service

import (
	"context"
	"strings"

	"mdht/internal/modules/graph/domain"
	graphout "mdht/internal/modules/graph/port/out"
	"mdht/internal/platform/slug"
)

type GraphService struct {
	topicStore graphout.TopicStore
	projector  graphout.LinkProjector
}

func NewGraphService(topicStore graphout.TopicStore, projector graphout.LinkProjector) *GraphService {
	return &GraphService{topicStore: topicStore, projector: projector}
}

func (s *GraphService) SyncSource(ctx context.Context, sourceID, sourceTitle string, topics []string) error {
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		topicSlug := slug.Make(topic)
		if err := s.topicStore.AppendSourceLink(ctx, topicSlug, sourceTitle, sourceID); err != nil {
			return err
		}
		if err := s.projector.Upsert(ctx, domain.Link{FromID: sourceID, ToID: topicSlug, Kind: "topic"}); err != nil {
			return err
		}
	}
	return nil
}
