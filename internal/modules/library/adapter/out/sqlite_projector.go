package out

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/library/domain"
	libraryout "mdht/internal/modules/library/port/out"

	_ "modernc.org/sqlite"
)

type SQLiteSourceProjector struct {
	db *sql.DB
}

func NewSQLiteSourceProjector(dbPath string) (libraryout.SourceIndexProjector, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	projector := &SQLiteSourceProjector{db: db}
	if err := projector.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return projector, nil
}

func (s *SQLiteSourceProjector) ensureSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS sources (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  slug TEXT NOT NULL,
  url TEXT,
  file_path TEXT,
  status TEXT,
  progress_percent REAL NOT NULL,
  unit_kind TEXT,
  unit_current INTEGER,
  unit_total INTEGER,
  updated_at TEXT NOT NULL
);
`
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create sources table: %w", err)
	}
	return nil
}

func (s *SQLiteSourceProjector) Reset(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sources`); err != nil {
		return fmt.Errorf("reset sources: %w", err)
	}
	return nil
}

func (s *SQLiteSourceProjector) UpsertSource(ctx context.Context, source domain.Source) error {
	const stmt = `
INSERT INTO sources (id, type, title, slug, url, file_path, status, progress_percent, unit_kind, unit_current, unit_total, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  type=excluded.type,
  title=excluded.title,
  slug=excluded.slug,
  url=excluded.url,
  file_path=excluded.file_path,
  status=excluded.status,
  progress_percent=excluded.progress_percent,
  unit_kind=excluded.unit_kind,
  unit_current=excluded.unit_current,
  unit_total=excluded.unit_total,
  updated_at=excluded.updated_at;
`
	_, err := s.db.ExecContext(ctx, stmt,
		source.ID,
		string(source.Type),
		source.Title,
		source.Slug,
		source.URL,
		source.FilePath,
		source.Status,
		source.ProgressPct,
		source.UnitKind,
		source.UnitCurrent,
		source.UnitTotal,
		source.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	if err != nil {
		return fmt.Errorf("upsert source: %w", err)
	}
	return nil
}
