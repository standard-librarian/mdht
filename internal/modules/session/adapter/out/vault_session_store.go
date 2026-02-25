package out

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/session/domain"
	sessionout "mdht/internal/modules/session/port/out"
	"mdht/internal/platform/markdown"
	"mdht/internal/platform/slug"
)

type VaultSessionStore struct {
	vaultPath string
}

func NewVaultSessionStore(vaultPath string) sessionout.SessionStore {
	return &VaultSessionStore{vaultPath: vaultPath}
}

func (s *VaultSessionStore) Save(_ context.Context, session domain.Session) (string, error) {
	date := session.StartedAt
	dir := filepath.Join(s.vaultPath, "sessions", date.Format("2006"), date.Format("01"), date.Format("02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create session dir: %w", err)
	}
	name := fmt.Sprintf("%s-%s.md", date.Format("150405"), slug.Make(session.SourceTitle))
	path := filepath.Join(dir, name)

	meta := map[string]any{
		"schema_version":   domain.SchemaVersion,
		"id":               session.ID,
		"source_id":        session.SourceID,
		"started_at":       session.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		"ended_at":         session.EndedAt.Format("2006-01-02T15:04:05Z07:00"),
		"duration_minutes": session.DurationMin,
		"goal":             session.Goal,
		"outcome":          session.Outcome,
		"delta_progress":   session.DeltaProgress,
		"progress_before":  session.ProgressBefore,
		"progress_after":   session.ProgressAfter,
	}
	body := fmt.Sprintf("# Session %s\n\n- Source: [[%s]]\n- Duration: %d minutes\n\n## Goal\n\n%s\n\n## Outcome\n\n%s\n", session.ID, session.SourceTitle, session.DurationMin, session.Goal, session.Outcome)
	rendered, err := markdown.RenderFrontmatter(meta, body)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return "", fmt.Errorf("write session note: %w", err)
	}
	return path, nil
}
