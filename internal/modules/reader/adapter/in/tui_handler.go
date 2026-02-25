package in

import (
	"context"

	"mdht/internal/modules/reader/dto"
	readerin "mdht/internal/modules/reader/port/in"
)

type TUIHandler struct {
	usecase readerin.Usecase
}

func NewTUIHandler(usecase readerin.Usecase) TUIHandler {
	return TUIHandler{usecase: usecase}
}

func (h TUIHandler) OpenSource(ctx context.Context, sourceID, mode string, page int, launchExternal bool) (dto.OpenResult, error) {
	return h.usecase.OpenSource(ctx, dto.OpenSourceInput{SourceID: sourceID, Mode: mode, Page: page, LaunchExternal: launchExternal})
}
