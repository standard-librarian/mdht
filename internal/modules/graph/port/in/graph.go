package in

import (
	"context"

	"mdht/internal/modules/graph/dto"
)

type SyncSourceInput struct {
	SourceID    string
	SourceTitle string
	Topics      []string
}

type NeighborsInput struct {
	NodeID string
	Depth  int
}

type PathInput struct {
	FromID string
	ToID   string
}

type Usecase interface {
	SyncSource(ctx context.Context, input SyncSourceInput) error
	ListTopics(ctx context.Context, limit int) ([]dto.TopicSummaryOutput, error)
	Neighbors(ctx context.Context, input NeighborsInput) (dto.NeighborsOutput, error)
	Search(ctx context.Context, query string) ([]dto.NodeOutput, error)
	Path(ctx context.Context, input PathInput) (dto.PathOutput, error)
}
