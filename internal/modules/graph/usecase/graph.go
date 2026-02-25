package usecase

import (
	"context"

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
