package usecase

import (
	"context"

	"mdht/internal/modules/graph/domain"
	"mdht/internal/modules/graph/dto"
	graphin "mdht/internal/modules/graph/port/in"
	"mdht/internal/modules/graph/service"
)

type Interactor struct {
	svc *service.GraphService
}

func NewInteractor(svc *service.GraphService) graphin.Usecase {
	return &Interactor{svc: svc}
}

func (i *Interactor) SyncSource(ctx context.Context, input graphin.SyncSourceInput) error {
	return i.svc.SyncSource(ctx, input.SourceID, input.SourceTitle, input.Topics)
}

func (i *Interactor) ListTopics(ctx context.Context, limit int) ([]dto.TopicSummaryOutput, error) {
	topics, err := i.svc.ListTopics(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TopicSummaryOutput, 0, len(topics))
	for _, item := range topics {
		out = append(out, dto.TopicSummaryOutput{
			TopicSlug:   item.TopicSlug,
			SourceCount: item.SourceCount,
		})
	}
	return out, nil
}

func (i *Interactor) Neighbors(ctx context.Context, input graphin.NeighborsInput) (dto.NeighborsOutput, error) {
	nodes, depth, err := i.svc.Neighbors(ctx, input.NodeID, input.Depth)
	if err != nil {
		return dto.NeighborsOutput{}, err
	}
	return dto.NeighborsOutput{
		FocusID: input.NodeID,
		Depth:   depth,
		Nodes:   mapNodes(nodes),
	}, nil
}

func (i *Interactor) Search(ctx context.Context, query string) ([]dto.NodeOutput, error) {
	nodes, err := i.svc.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return mapNodes(nodes), nil
}

func (i *Interactor) Path(ctx context.Context, input graphin.PathInput) (dto.PathOutput, error) {
	nodes, err := i.svc.ShortestPath(ctx, input.FromID, input.ToID)
	if err != nil {
		return dto.PathOutput{}, err
	}
	return dto.PathOutput{
		FromID: input.FromID,
		ToID:   input.ToID,
		Found:  len(nodes) > 0,
		Nodes:  mapNodes(nodes),
	}, nil
}

func mapNodes(nodes []domain.Node) []dto.NodeOutput {
	out := make([]dto.NodeOutput, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, dto.NodeOutput{
			ID:    node.ID,
			Label: node.Label,
			Kind:  string(node.Kind),
		})
	}
	return out
}
