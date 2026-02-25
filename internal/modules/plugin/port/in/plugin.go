package in

import (
	"context"

	"mdht/internal/modules/plugin/dto"
)

type Usecase interface {
	List(ctx context.Context) ([]dto.PluginInfo, error)
	Doctor(ctx context.Context) ([]dto.DoctorResult, error)
	ListCommands(ctx context.Context, pluginName string) ([]dto.CommandInfo, error)
	Execute(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error)
	Analyze(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error)
	PrepareTTY(ctx context.Context, input dto.TTYPrepareInput) (dto.TTYPrepareOutput, error)
}
