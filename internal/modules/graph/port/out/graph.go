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
