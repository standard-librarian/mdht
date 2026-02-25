package out

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/graph/domain"
	graphout "mdht/internal/modules/graph/port/out"

	_ "modernc.org/sqlite"
)

type SQLiteLinkProjector struct {
	db *sql.DB
}

func NewSQLiteLinkProjector(dbPath string) (graphout.LinkProjector, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	p := &SQLiteLinkProjector{db: db}
	if err := p.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *SQLiteLinkProjector) ensureSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS links (
  from_id TEXT NOT NULL,
  to_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  PRIMARY KEY (from_id, to_id, kind)
);
`
	if _, err := p.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create links table: %w", err)
	}
	return nil
}

func (p *SQLiteLinkProjector) Upsert(ctx context.Context, link domain.Link) error {
	const stmt = `
INSERT INTO links (from_id, to_id, kind)
VALUES (?, ?, ?)
ON CONFLICT(from_id, to_id, kind) DO NOTHING;
`
	if _, err := p.db.ExecContext(ctx, stmt, link.FromID, link.ToID, link.Kind); err != nil {
		return fmt.Errorf("upsert link: %w", err)
	}
	return nil
}
