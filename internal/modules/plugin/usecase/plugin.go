package usecase

import (
	"context"

	"mdht/internal/modules/plugin/dto"
	pluginin "mdht/internal/modules/plugin/port/in"
	"mdht/internal/modules/plugin/service"
)

type Interactor struct {
	svc *service.PluginService
}

func NewInteractor(svc *service.PluginService) pluginin.Usecase {
	return &Interactor{svc: svc}
}

func (i *Interactor) List(ctx context.Context) ([]dto.PluginInfo, error) {
	return i.svc.List(ctx)
}

func (i *Interactor) Doctor(ctx context.Context) ([]dto.DoctorResult, error) {
	return i.svc.Doctor(ctx)
}

func (i *Interactor) ListCommands(ctx context.Context, pluginName string) ([]dto.CommandInfo, error) {
	return i.svc.ListCommands(ctx, pluginName)
}

func (i *Interactor) Execute(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return i.svc.Execute(ctx, input)
}

func (i *Interactor) Analyze(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return i.svc.Analyze(ctx, input)
}

func (i *Interactor) PrepareTTY(ctx context.Context, input dto.TTYPrepareInput) (dto.TTYPrepareOutput, error) {
	return i.svc.PrepareTTY(ctx, input)
}
