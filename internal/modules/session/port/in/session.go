package in

import (
	"context"

	"mdht/internal/modules/session/dto"
)

type Usecase interface {
	Start(ctx context.Context, input dto.StartInput) (dto.StartOutput, error)
	End(ctx context.Context, input dto.EndInput) (dto.EndOutput, error)
	GetActive(ctx context.Context) (dto.ActiveSessionOutput, error)
}
