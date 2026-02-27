package in

import (
	"context"

	"mdht/internal/modules/graph/dto"
	graphin "mdht/internal/modules/graph/port/in"
)

type CLIHandler struct {
	usecase graphin.Usecase
}

func NewCLIHandler(usecase graphin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) SyncSource(ctx context.Context, sourceID, sourceTitle string, topics []string) error {
	return h.usecase.SyncSource(ctx, graphin.SyncSourceInput{SourceID: sourceID, SourceTitle: sourceTitle, Topics: topics})
}

func (h CLIHandler) ListTopics(ctx context.Context, limit int) ([]dto.TopicSummaryOutput, error) {
	return h.usecase.ListTopics(ctx, limit)
}

func (h CLIHandler) Neighbors(ctx context.Context, nodeID string, depth int) (dto.NeighborsOutput, error) {
	return h.usecase.Neighbors(ctx, graphin.NeighborsInput{NodeID: nodeID, Depth: depth})
}

func (h CLIHandler) Search(ctx context.Context, query string) ([]dto.NodeOutput, error) {
	return h.usecase.Search(ctx, query)
}

func (h CLIHandler) Path(ctx context.Context, fromID, toID string) (dto.PathOutput, error) {
	return h.usecase.Path(ctx, graphin.PathInput{FromID: fromID, ToID: toID})
}
