package in

import (
	"context"

	"mdht/internal/modules/reader/dto"
)

type Usecase interface {
	OpenMarkdown(ctx context.Context, input dto.OpenMarkdownInput) (dto.OpenMarkdownOutput, error)
	OpenPDF(ctx context.Context, input dto.OpenPDFInput) (dto.OpenPDFOutput, error)
	OpenSource(ctx context.Context, input dto.OpenSourceInput) (dto.OpenResult, error)
	UpdateProgress(ctx context.Context, input dto.UpdateProgressInput) error
}
