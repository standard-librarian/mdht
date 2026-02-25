package tx

import "context"

// Manager wraps transactional boundaries for multi-adapter operations.
type Manager interface {
	Within(ctx context.Context, fn func(context.Context) error) error
}

type NoopManager struct{}

func (NoopManager) Within(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
