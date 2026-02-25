package in

import (
	"context"

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
