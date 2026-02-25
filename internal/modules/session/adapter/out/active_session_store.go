package out

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/session/domain"
	sessionout "mdht/internal/modules/session/port/out"
	apperrors "mdht/internal/platform/errors"
)

type FileActiveSessionStore struct {
	path string
}

func NewFileActiveSessionStore(vaultPath string) sessionout.ActiveSessionStore {
	return &FileActiveSessionStore{path: filepath.Join(vaultPath, ".mdht", "active-session.json")}
}

func (s *FileActiveSessionStore) SaveActive(_ context.Context, session domain.ActiveSession) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create active session dir: %w", err)
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal active session: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write active session: %w", err)
	}
	return nil
}

func (s *FileActiveSessionStore) LoadActive(_ context.Context) (domain.ActiveSession, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.ActiveSession{}, apperrors.ErrNoActiveSession
		}
		return domain.ActiveSession{}, fmt.Errorf("read active session: %w", err)
	}
	active := domain.ActiveSession{}
	if err := json.Unmarshal(payload, &active); err != nil {
		return domain.ActiveSession{}, fmt.Errorf("decode active session: %w", err)
	}
	if active.SessionID == "" {
		return domain.ActiveSession{}, apperrors.ErrNoActiveSession
	}
	return active, nil
}

func (s *FileActiveSessionStore) ClearActive(_ context.Context) error {
	if err := os.Remove(s.path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("clear active session: %w", err)
	}
	return nil
}
