package in

import (
	"context"

	"mdht/internal/modules/reader/dto"
	readerin "mdht/internal/modules/reader/port/in"
)

type CLIHandler struct {
	usecase readerin.Usecase
}

func NewCLIHandler(usecase readerin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) OpenSource(ctx context.Context, sourceID, mode string, page int, launchExternal bool) (dto.OpenResult, error) {
	return h.usecase.OpenSource(ctx, dto.OpenSourceInput{SourceID: sourceID, Mode: mode, Page: page, LaunchExternal: launchExternal})
}
