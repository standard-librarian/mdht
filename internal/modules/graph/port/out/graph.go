package out

import (
	"context"

	"mdht/internal/modules/graph/domain"
)

type TopicStore interface {
	AppendSourceLink(ctx context.Context, topic, sourceTitle, sourceID string) error
}

type LinkProjector interface {
	Upsert(ctx context.Context, link domain.Link) error
}

type GraphQueryStore interface {
	ListTopics(ctx context.Context, limit int) ([]domain.TopicSummary, error)
	Neighbors(ctx context.Context, nodeID string, depth int) ([]domain.Node, error)
	Search(ctx context.Context, query string) ([]domain.Node, error)
	ShortestPath(ctx context.Context, fromID, toID string) ([]domain.Node, error)
}
