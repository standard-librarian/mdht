package in

import (
	"context"

	"mdht/internal/modules/library/dto"
)

type Usecase interface {
	IngestFile(ctx context.Context, input dto.IngestFileInput) (dto.SourceOutput, error)
	IngestURL(ctx context.Context, input dto.IngestURLInput) (dto.SourceOutput, error)
	UpdateProgress(ctx context.Context, input dto.UpdateProgressInput) (dto.SourceOutput, error)
	ListSources(ctx context.Context) ([]dto.SourceOutput, error)
	GetSource(ctx context.Context, id string) (dto.SourceDetailOutput, error)
	Reindex(ctx context.Context, input dto.ReindexInput) error
}
