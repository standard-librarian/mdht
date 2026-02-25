package out

import (
	"context"

	"mdht/internal/modules/session/domain"
)

type SessionStore interface {
	Save(ctx context.Context, session domain.Session) (string, error)
}

type ActiveSessionStore interface {
	SaveActive(ctx context.Context, session domain.ActiveSession) error
	LoadActive(ctx context.Context) (domain.ActiveSession, error)
	ClearActive(ctx context.Context) error
}
