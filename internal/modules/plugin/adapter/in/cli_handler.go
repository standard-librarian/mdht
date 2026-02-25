package in

import (
	"context"

	"mdht/internal/modules/plugin/dto"
	pluginin "mdht/internal/modules/plugin/port/in"
)

type CLIHandler struct {
	usecase pluginin.Usecase
}

func NewCLIHandler(usecase pluginin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) List(ctx context.Context) ([]dto.PluginInfo, error) {
	return h.usecase.List(ctx)
}

func (h CLIHandler) Doctor(ctx context.Context) ([]dto.DoctorResult, error) {
	return h.usecase.Doctor(ctx)
}

func (h CLIHandler) ListCommands(ctx context.Context, pluginName string) ([]dto.CommandInfo, error) {
	return h.usecase.ListCommands(ctx, pluginName)
}

func (h CLIHandler) Execute(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return h.usecase.Execute(ctx, input)
}

func (h CLIHandler) Analyze(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return h.usecase.Analyze(ctx, input)
}

func (h CLIHandler) PrepareTTY(ctx context.Context, input dto.TTYPrepareInput) (dto.TTYPrepareOutput, error) {
	return h.usecase.PrepareTTY(ctx, input)
}
