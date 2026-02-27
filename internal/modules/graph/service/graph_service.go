package service

import (
	"context"
	"fmt"
	"strings"

	"mdht/internal/modules/graph/domain"
	graphout "mdht/internal/modules/graph/port/out"
	"mdht/internal/platform/slug"
)

type GraphService struct {
	topicStore graphout.TopicStore
	projector  graphout.LinkProjector
	queryStore graphout.GraphQueryStore
}

func NewGraphService(topicStore graphout.TopicStore, projector graphout.LinkProjector, queryStore graphout.GraphQueryStore) *GraphService {
	return &GraphService{topicStore: topicStore, projector: projector, queryStore: queryStore}
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

func (s *GraphService) ListTopics(ctx context.Context, limit int) ([]domain.TopicSummary, error) {
	if s.queryStore == nil {
		return nil, fmt.Errorf("graph query store is not configured")
	}
	return s.queryStore.ListTopics(ctx, limit)
}

func (s *GraphService) Neighbors(ctx context.Context, nodeID string, depth int) ([]domain.Node, int, error) {
	if s.queryStore == nil {
		return nil, 0, fmt.Errorf("graph query store is not configured")
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 2 {
		depth = 2
	}
	nodes, err := s.queryStore.Neighbors(ctx, nodeID, depth)
	if err != nil {
		return nil, 0, err
	}
	return nodes, depth, nil
}

func (s *GraphService) Search(ctx context.Context, query string) ([]domain.Node, error) {
	if s.queryStore == nil {
		return nil, fmt.Errorf("graph query store is not configured")
	}
	return s.queryStore.Search(ctx, query)
}

func (s *GraphService) ShortestPath(ctx context.Context, fromID, toID string) ([]domain.Node, error) {
	if s.queryStore == nil {
		return nil, fmt.Errorf("graph query store is not configured")
	}
	return s.queryStore.ShortestPath(ctx, fromID, toID)
}
