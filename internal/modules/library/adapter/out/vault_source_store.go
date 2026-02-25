package out

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mdht/internal/modules/library/domain"
	libraryout "mdht/internal/modules/library/port/out"
	apperrors "mdht/internal/platform/errors"
	"mdht/internal/platform/markdown"
)

type VaultSourceStore struct {
	vaultPath string
}

func NewVaultSourceStore(vaultPath string) libraryout.SourceStore {
	return &VaultSourceStore{vaultPath: vaultPath}
}

func (s *VaultSourceStore) Save(_ context.Context, document domain.SourceDocument) (string, error) {
	source := document.Source
	sourcePath := filepath.Join(s.vaultPath, "sources", source.Slug+".md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		return "", fmt.Errorf("create source directory: %w", err)
	}

	body := document.Body
	if existing, err := os.ReadFile(sourcePath); err == nil {
		_, existingBody, splitErr := markdown.SplitFrontmatter(string(existing))
		if splitErr == nil && strings.TrimSpace(body) == "" {
			body = existingBody
		}
	}

	if strings.TrimSpace(body) == "" {
		body = "## Summary\n\n## Highlights\n\n## Questions\n\n## Next Actions\n"
	}
	linksContent := strings.Join(source.ManagedWikilink, "\n")
	body = markdown.ReplaceManagedBlock(body, domain.ManagedLinksStart, domain.ManagedLinksEnd, linksContent)

	rendered, err := markdown.RenderFrontmatter(toFrontmatter(source), body)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(sourcePath, []byte(rendered), 0o644); err != nil {
		return "", fmt.Errorf("write source markdown: %w", err)
	}
	return sourcePath, nil
}

func (s *VaultSourceStore) FindByID(ctx context.Context, id string) (domain.SourceDocument, error) {
	docs, err := s.List(ctx)
	if err != nil {
		return domain.SourceDocument{}, err
	}
	for _, doc := range docs {
		if doc.Source.ID == id {
			return doc, nil
		}
	}
	return domain.SourceDocument{}, apperrors.ErrNotFound
}

func (s *VaultSourceStore) List(_ context.Context) ([]domain.SourceDocument, error) {
	glob := filepath.Join(s.vaultPath, "sources", "*.md")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("glob source notes: %w", err)
	}
	sort.Strings(matches)

	out := make([]domain.SourceDocument, 0, len(matches))
	for _, path := range matches {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", path, readErr)
		}
		meta, body, splitErr := markdown.SplitFrontmatter(string(content))
		if splitErr != nil {
			return nil, fmt.Errorf("parse %s: %w", path, splitErr)
		}
		source, convErr := fromFrontmatter(meta, path)
		if convErr != nil {
			return nil, fmt.Errorf("decode source %s: %w", path, convErr)
		}
		out = append(out, domain.SourceDocument{Source: source, Body: body})
	}
	return out, nil
}

func toFrontmatter(source domain.Source) map[string]any {
	return map[string]any{
		"schema_version":   domain.SchemaVersion,
		"id":               source.ID,
		"type":             string(source.Type),
		"title":            source.Title,
		"authors":          source.Authors,
		"url":              source.URL,
		"file_path":        source.FilePath,
		"tags":             source.Tags,
		"topics":           source.Topics,
		"status":           source.Status,
		"progress_percent": source.ProgressPct,
		"unit_kind":        source.UnitKind,
		"unit_current":     source.UnitCurrent,
		"unit_total":       source.UnitTotal,
		"added_at":         source.AddedAt.Format(time.RFC3339),
		"updated_at":       source.UpdatedAt.Format(time.RFC3339),
		"last_session_id":  source.LastSessionID,
	}
}

func fromFrontmatter(meta map[string]any, notePath string) (domain.Source, error) {
	source := domain.Source{
		ID:            asString(meta["id"]),
		Type:          domain.SourceType(asString(meta["type"])),
		Title:         asString(meta["title"]),
		Authors:       asStringSlice(meta["authors"]),
		URL:           asString(meta["url"]),
		FilePath:      asString(meta["file_path"]),
		NotePath:      notePath,
		Tags:          asStringSlice(meta["tags"]),
		Topics:        asStringSlice(meta["topics"]),
		Status:        asString(meta["status"]),
		ProgressPct:   asFloat(meta["progress_percent"]),
		UnitKind:      asString(meta["unit_kind"]),
		UnitCurrent:   int(asFloat(meta["unit_current"])),
		UnitTotal:     int(asFloat(meta["unit_total"])),
		LastSessionID: asString(meta["last_session_id"]),
	}
	source.Slug = strings.TrimSuffix(filepath.Base(notePath), filepath.Ext(notePath))
	addedAt, _ := time.Parse(time.RFC3339, asString(meta["added_at"]))
	updatedAt, _ := time.Parse(time.RFC3339, asString(meta["updated_at"]))
	source.AddedAt = addedAt
	source.UpdatedAt = updatedAt
	if err := source.Validate(); err != nil {
		return domain.Source{}, err
	}
	return source, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(v)
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case float32:
		return float64(x)
	case string:
		var out float64
		_, _ = fmt.Sscanf(x, "%f", &out)
		return out
	default:
		return 0
	}
}

func asStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}
