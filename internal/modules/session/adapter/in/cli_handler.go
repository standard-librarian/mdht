package in

import (
	"context"

	sessiondto "mdht/internal/modules/session/dto"
	sessionin "mdht/internal/modules/session/port/in"
)

type CLIHandler struct {
	usecase sessionin.Usecase
}

func NewCLIHandler(usecase sessionin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) Start(ctx context.Context, sourceID, sourceTitle, goal string) (sessiondto.StartOutput, error) {
	return h.usecase.Start(ctx, sessiondto.StartInput{SourceID: sourceID, SourceTitle: sourceTitle, Goal: goal})
}

func (h CLIHandler) End(ctx context.Context, sessionID, outcome string, delta float64) (sessiondto.EndOutput, error) {
	return h.usecase.End(ctx, sessiondto.EndInput{SessionID: sessionID, Outcome: outcome, DeltaProgress: delta})
}

func (h CLIHandler) GetActive(ctx context.Context) (sessiondto.ActiveSessionOutput, error) {
	return h.usecase.GetActive(ctx)
}
