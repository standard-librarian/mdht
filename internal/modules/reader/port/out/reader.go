package out

import (
	"context"

	"mdht/internal/modules/reader/domain"
)

type MarkdownReader interface {
	Read(ctx context.Context, path string) (string, error)
}

type PDFReader interface {
	ReadPage(ctx context.Context, path string, page int) (domain.Page, int, error)
}

type ProgressPort interface {
	Update(ctx context.Context, sourceID string, percent float64, unitCurrent, unitTotal int) error
}

type SourceResolver interface {
	Resolve(ctx context.Context, sourceID string) (domain.SourceRef, error)
}

type ExternalLauncher interface {
	Open(ctx context.Context, target string) error
}
