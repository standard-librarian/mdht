package out

import (
	"context"

	"mdht/internal/modules/library/domain"
)

type SourceStore interface {
	Save(ctx context.Context, document domain.SourceDocument) (string, error)
	FindByID(ctx context.Context, id string) (domain.SourceDocument, error)
	List(ctx context.Context) ([]domain.SourceDocument, error)
}

type SourceIndexProjector interface {
	Reset(ctx context.Context) error
	UpsertSource(ctx context.Context, source domain.Source) error
}
