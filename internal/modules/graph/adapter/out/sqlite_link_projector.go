package out

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mdht/internal/modules/graph/domain"

	_ "modernc.org/sqlite"
)

type SQLiteLinkProjector struct {
	db *sql.DB
}

func NewSQLiteLinkProjector(dbPath string) (*SQLiteLinkProjector, error) {
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
CREATE INDEX IF NOT EXISTS idx_links_to_kind ON links(to_id, kind);
CREATE INDEX IF NOT EXISTS idx_links_kind_from ON links(kind, from_id);
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

func (p *SQLiteLinkProjector) ListTopics(ctx context.Context, limit int) ([]domain.TopicSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := p.db.QueryContext(ctx, `
SELECT to_id, COUNT(*) AS source_count
FROM links
WHERE kind = 'topic'
GROUP BY to_id
ORDER BY source_count DESC, to_id ASC
LIMIT ?;
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TopicSummary, 0, limit)
	for rows.Next() {
		item := domain.TopicSummary{}
		if err := rows.Scan(&item.TopicSlug, &item.SourceCount); err != nil {
			return nil, fmt.Errorf("scan topic summary: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topic summary: %w", err)
	}
	return out, nil
}

func (p *SQLiteLinkProjector) Neighbors(ctx context.Context, nodeID string, depth int) ([]domain.Node, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return []domain.Node{}, nil
	}
	if depth < 1 {
		depth = 1
	}
	adjacency, kinds, err := p.loadAdjacency(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := adjacency[nodeID]; !ok {
		return []domain.Node{}, nil
	}

	type queueItem struct {
		ID    string
		Depth int
	}
	seen := map[string]struct{}{nodeID: {}}
	queue := []queueItem{{ID: nodeID, Depth: 0}}
	neighborIDs := make([]string, 0)
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.Depth >= depth {
			continue
		}
		nextIDs := sortedNeighborIDs(adjacency[item.ID])
		for _, nextID := range nextIDs {
			if _, ok := seen[nextID]; ok {
				continue
			}
			seen[nextID] = struct{}{}
			neighborIDs = append(neighborIDs, nextID)
			queue = append(queue, queueItem{ID: nextID, Depth: item.Depth + 1})
		}
	}
	return p.mapNodes(ctx, neighborIDs, kinds)
}

func (p *SQLiteLinkProjector) Search(ctx context.Context, query string) ([]domain.Node, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []domain.Node{}, nil
	}
	adjacency, kinds, err := p.loadAdjacency(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(adjacency))
	for id := range adjacency {
		ids = append(ids, id)
	}
	nodes, err := p.mapNodes(ctx, ids, kinds)
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.Node, 0, len(nodes))
	for _, node := range nodes {
		if strings.Contains(strings.ToLower(node.ID), query) || strings.Contains(strings.ToLower(node.Label), query) {
			filtered = append(filtered, node)
		}
	}
	return filtered, nil
}

func (p *SQLiteLinkProjector) ShortestPath(ctx context.Context, fromID, toID string) ([]domain.Node, error) {
	fromID = strings.TrimSpace(fromID)
	toID = strings.TrimSpace(toID)
	if fromID == "" || toID == "" {
		return []domain.Node{}, nil
	}
	adjacency, kinds, err := p.loadAdjacency(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := adjacency[fromID]; !ok {
		return []domain.Node{}, nil
	}
	if _, ok := adjacency[toID]; !ok {
		return []domain.Node{}, nil
	}
	if fromID == toID {
		return p.mapNodes(ctx, []string{fromID}, kinds)
	}

	queue := []string{fromID}
	visited := map[string]struct{}{fromID: {}}
	prev := map[string]string{}
	found := false

	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]
		for _, nextID := range sortedNeighborIDs(adjacency[current]) {
			if _, ok := visited[nextID]; ok {
				continue
			}
			visited[nextID] = struct{}{}
			prev[nextID] = current
			if nextID == toID {
				found = true
				break
			}
			queue = append(queue, nextID)
		}
	}
	if !found {
		return []domain.Node{}, nil
	}

	pathIDs := []string{toID}
	for current := toID; current != fromID; {
		parent, ok := prev[current]
		if !ok {
			return []domain.Node{}, nil
		}
		pathIDs = append(pathIDs, parent)
		current = parent
	}
	reverse(pathIDs)
	return p.mapNodes(ctx, pathIDs, kinds)
}

func (p *SQLiteLinkProjector) loadAdjacency(ctx context.Context) (map[string]map[string]struct{}, map[string]domain.NodeKind, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT from_id, to_id
FROM links
WHERE kind = 'topic'
`)
	if err != nil {
		return nil, nil, fmt.Errorf("load adjacency: %w", err)
	}
	defer rows.Close()

	adjacency := map[string]map[string]struct{}{}
	kinds := map[string]domain.NodeKind{}
	for rows.Next() {
		var fromID, toID string
		if err := rows.Scan(&fromID, &toID); err != nil {
			return nil, nil, fmt.Errorf("scan adjacency row: %w", err)
		}
		if adjacency[fromID] == nil {
			adjacency[fromID] = map[string]struct{}{}
		}
		if adjacency[toID] == nil {
			adjacency[toID] = map[string]struct{}{}
		}
		adjacency[fromID][toID] = struct{}{}
		adjacency[toID][fromID] = struct{}{}
		kinds[fromID] = domain.NodeKindSource
		kinds[toID] = domain.NodeKindTopic
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate adjacency rows: %w", err)
	}
	return adjacency, kinds, nil
}

func (p *SQLiteLinkProjector) mapNodes(ctx context.Context, ids []string, kinds map[string]domain.NodeKind) ([]domain.Node, error) {
	if len(ids) == 0 {
		return []domain.Node{}, nil
	}
	deduped := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	sourceIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		deduped = append(deduped, id)
		if kinds[id] == domain.NodeKindSource {
			sourceIDs = append(sourceIDs, id)
		}
	}

	sourceTitles, err := p.lookupSourceTitles(ctx, sourceIDs)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Node, 0, len(deduped))
	for _, id := range deduped {
		kind := kinds[id]
		if kind == "" {
			kind = domain.NodeKindSource
		}
		label := id
		if kind == domain.NodeKindSource {
			if title := sourceTitles[id]; strings.TrimSpace(title) != "" {
				label = title
			}
		} else {
			label = strings.ReplaceAll(id, "-", " ")
		}
		out = append(out, domain.Node{ID: id, Label: label, Kind: kind})
	}
	return out, nil
}

func (p *SQLiteLinkProjector) lookupSourceTitles(ctx context.Context, sourceIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(sourceIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(sourceIDs))
	args := make([]any, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf("SELECT id, title FROM sources WHERE id IN (%s)", strings.Join(placeholders, ","))
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return out, nil
		}
		return nil, fmt.Errorf("lookup source titles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, fmt.Errorf("scan source title: %w", err)
		}
		out[id] = title
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source titles: %w", err)
	}
	return out, nil
}

func sortedNeighborIDs(neighbors map[string]struct{}) []string {
	out := make([]string, 0, len(neighbors))
	for id := range neighbors {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func reverse(ids []string) {
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
}
