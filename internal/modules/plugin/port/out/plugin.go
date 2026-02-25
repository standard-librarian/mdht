package out

import (
	"context"

	"mdht/internal/modules/plugin/domain"
)

type ManifestStore interface {
	Load(ctx context.Context) ([]domain.Manifest, error)
}

type Host interface {
	CheckLifecycle(ctx context.Context, manifest domain.Manifest) error
	GetMetadata(ctx context.Context, manifest domain.Manifest) (domain.Metadata, error)
	ListCommands(ctx context.Context, manifest domain.Manifest) ([]domain.CommandDescriptor, error)
	Execute(ctx context.Context, manifest domain.Manifest, input domain.ExecuteRequest) (domain.ExecuteResult, error)
	PrepareTTY(ctx context.Context, manifest domain.Manifest, input domain.ExecuteRequest) (domain.TTYPlan, error)
}
