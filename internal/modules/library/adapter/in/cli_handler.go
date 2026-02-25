package in

import (
	"context"

	"mdht/internal/modules/library/dto"
	libraryin "mdht/internal/modules/library/port/in"
)

type CLIHandler struct {
	usecase libraryin.Usecase
}

func NewCLIHandler(usecase libraryin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) IngestFile(ctx context.Context, sourceType, path, title string, tags, topics []string) (dto.SourceOutput, error) {
	return h.usecase.IngestFile(ctx, dto.IngestFileInput{
		Path:   path,
		Type:   sourceType,
		Title:  title,
		Tags:   tags,
		Topics: topics,
	})
}

func (h CLIHandler) IngestURL(ctx context.Context, sourceType, url, title string, tags, topics []string) (dto.SourceOutput, error) {
	return h.usecase.IngestURL(ctx, dto.IngestURLInput{
		URL:    url,
		Type:   sourceType,
		Title:  title,
		Tags:   tags,
		Topics: topics,
	})
}

func (h CLIHandler) UpdateProgress(ctx context.Context, sourceID string, percent float64, unitCurrent, unitTotal int) (dto.SourceOutput, error) {
	return h.usecase.UpdateProgress(ctx, dto.UpdateProgressInput{SourceID: sourceID, Percent: percent, UnitCurrent: unitCurrent, UnitTotal: unitTotal})
}

func (h CLIHandler) ListSources(ctx context.Context) ([]dto.SourceOutput, error) {
	return h.usecase.ListSources(ctx)
}

func (h CLIHandler) GetSource(ctx context.Context, id string) (dto.SourceDetailOutput, error) {
	return h.usecase.GetSource(ctx, id)
}

func (h CLIHandler) Reindex(ctx context.Context) error {
	return h.usecase.Reindex(ctx, dto.ReindexInput{})
}
