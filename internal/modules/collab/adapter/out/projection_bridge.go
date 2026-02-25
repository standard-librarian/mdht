package out

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
	librarydomain "mdht/internal/modules/library/domain"
	"mdht/internal/platform/markdown"
	"mdht/internal/platform/slug"
)

const (
	sessionManagedStart = "<!-- mdht:session:managed:start -->"
	sessionManagedEnd   = "<!-- mdht:session:managed:end -->"
	topicManagedStart   = "<!-- mdht:topic:managed:start -->"
	topicManagedEnd     = "<!-- mdht:topic:managed:end -->"
)

type VaultProjectionBridge struct {
	vaultPath string
}

func NewVaultProjectionBridge(vaultPath string) *VaultProjectionBridge {
	return &VaultProjectionBridge{vaultPath: vaultPath}
}

func (b *VaultProjectionBridge) Extract(ctx context.Context, workspaceID, nodeID string, now time.Time) ([]domain.OpEnvelope, error) {
	_ = ctx
	opBuilder := newOpBuilder(workspaceID, nodeID, now)

	sourceOps, err := b.extractSourceOps(opBuilder)
	if err != nil {
		return nil, err
	}
	sessionOps, err := b.extractSessionOps(opBuilder)
	if err != nil {
		return nil, err
	}
	topicOps, err := b.extractTopicOps(opBuilder)
	if err != nil {
		return nil, err
	}

	ops := make([]domain.OpEnvelope, 0, len(sourceOps)+len(sessionOps)+len(topicOps))
	ops = append(ops, sourceOps...)
	ops = append(ops, sessionOps...)
	ops = append(ops, topicOps...)
	return ops, nil
}

func (b *VaultProjectionBridge) Apply(_ context.Context, state domain.CRDTState) error {
	keys := make([]string, 0, len(state.Entities))
	for key := range state.Entities {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entity := state.Entities[key]
		switch {
		case strings.HasPrefix(key, "source/"):
			if err := b.applySource(key, entity); err != nil {
				return err
			}
		case strings.HasPrefix(key, "session/"):
			if err := b.applySession(key, entity); err != nil {
				return err
			}
		case strings.HasPrefix(key, "topic/"):
			if err := b.applyTopic(key, entity); err != nil {
				return err
			}
		}
	}
	return nil
}

type opBuilder struct {
	workspaceID string
	nodeID      string
	wall        int64
	counter     int64
}

func newOpBuilder(workspaceID, nodeID string, now time.Time) *opBuilder {
	return &opBuilder{workspaceID: workspaceID, nodeID: nodeID, wall: now.UTC().UnixMilli()}
}

func (b *opBuilder) make(entityKey string, kind domain.OpKind, payload any) (domain.OpEnvelope, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return domain.OpEnvelope{}, err
	}
	hlc := domain.HLC{Wall: b.wall, Counter: b.counter, NodeID: b.nodeID}
	b.counter++
	opID := hashHex(entityKey + "|" + string(kind) + "|" + string(rawPayload))
	return domain.OpEnvelope{
		WorkspaceID:  b.workspaceID,
		NodeID:       b.nodeID,
		EntityKey:    entityKey,
		OpID:         opID,
		HLCTimestamp: hlc.String(),
		OpKind:       kind,
		Payload:      rawPayload,
	}, nil
}

func (b *VaultProjectionBridge) extractSourceOps(builder *opBuilder) ([]domain.OpEnvelope, error) {
	glob := filepath.Join(b.vaultPath, "sources", "*.md")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("glob sources: %w", err)
	}
	sort.Strings(matches)

	ops := []domain.OpEnvelope{}
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		meta, body, err := markdown.SplitFrontmatter(string(content))
		if err != nil {
			continue
		}
		id := asString(meta["id"])
		if id == "" {
			continue
		}
		entityKey := "source/" + id

		tombstone, err := builder.make(entityKey, domain.OpKindTombstone, map[string]any{"reason": "snapshot_refresh", "digest": hashHex(string(content))})
		if err != nil {
			return nil, err
		}
		ops = append(ops, tombstone)

		scalars := map[string]any{
			"id":               id,
			"type":             asString(meta["type"]),
			"title":            asString(meta["title"]),
			"authors":          asStringSlice(meta["authors"]),
			"url":              asString(meta["url"]),
			"file_path":        asString(meta["file_path"]),
			"status":           asString(meta["status"]),
			"progress_percent": asFloat(meta["progress_percent"]),
			"unit_kind":        asString(meta["unit_kind"]),
			"unit_current":     int(asFloat(meta["unit_current"])),
			"unit_total":       int(asFloat(meta["unit_total"])),
			"last_session_id":  asString(meta["last_session_id"]),
			"note_path":        path,
		}
		for field, value := range scalars {
			op, err := builder.make(entityKey, domain.OpKindPutRegister, domain.RegisterPayload{Field: field, Value: mustRawJSON(value)})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}

		for _, tag := range asStringSlice(meta["tags"]) {
			op, err := builder.make(entityKey, domain.OpKindAddSet, domain.SetPayload{Field: "tags", Value: tag})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
		for _, topic := range asStringSlice(meta["topics"]) {
			op, err := builder.make(entityKey, domain.OpKindAddSet, domain.SetPayload{Field: "topics", Value: topic})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}

		lines := extractManagedLines(body, librarydomain.ManagedLinksStart, librarydomain.ManagedLinksEnd)
		previous := ""
		for index, line := range lines {
			lineID := hashHex(entityKey + "|managed_links|" + strconv.Itoa(index) + "|" + line)
			op, err := builder.make(entityKey, domain.OpKindInsertSeq, domain.InsertSequencePayload{
				Field:   "managed_links",
				LineID:  lineID,
				AfterID: previous,
				Value:   line,
			})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			previous = lineID
		}
	}
	return ops, nil
}

func (b *VaultProjectionBridge) extractSessionOps(builder *opBuilder) ([]domain.OpEnvelope, error) {
	root := filepath.Join(b.vaultPath, "sessions")
	entries := []string{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		entries = append(entries, path)
		return nil
	})
	sort.Strings(entries)

	ops := []domain.OpEnvelope{}
	for _, path := range entries {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		meta, body, err := markdown.SplitFrontmatter(string(content))
		if err != nil {
			continue
		}
		id := asString(meta["id"])
		if id == "" {
			continue
		}
		entityKey := "session/" + id

		tombstone, err := builder.make(entityKey, domain.OpKindTombstone, map[string]any{"reason": "snapshot_refresh", "digest": hashHex(string(content))})
		if err != nil {
			return nil, err
		}
		ops = append(ops, tombstone)

		scalars := map[string]any{
			"id":               id,
			"source_id":        asString(meta["source_id"]),
			"started_at":       asString(meta["started_at"]),
			"ended_at":         asString(meta["ended_at"]),
			"duration_minutes": int(asFloat(meta["duration_minutes"])),
			"goal":             asString(meta["goal"]),
			"outcome":          asString(meta["outcome"]),
			"delta_progress":   asFloat(meta["delta_progress"]),
			"progress_before":  asFloat(meta["progress_before"]),
			"progress_after":   asFloat(meta["progress_after"]),
			"note_path":        path,
		}
		for field, value := range scalars {
			op, err := builder.make(entityKey, domain.OpKindPutRegister, domain.RegisterPayload{Field: field, Value: mustRawJSON(value)})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}

		lines := extractManagedLines(body, sessionManagedStart, sessionManagedEnd)
		if len(lines) == 0 {
			goal := asString(meta["goal"])
			outcome := asString(meta["outcome"])
			if goal != "" {
				lines = append(lines, "Goal: "+goal)
			}
			if outcome != "" {
				lines = append(lines, "Outcome: "+outcome)
			}
		}
		previous := ""
		for index, line := range lines {
			lineID := hashHex(entityKey + "|managed_body|" + strconv.Itoa(index) + "|" + line)
			op, err := builder.make(entityKey, domain.OpKindInsertSeq, domain.InsertSequencePayload{
				Field:   "managed_body",
				LineID:  lineID,
				AfterID: previous,
				Value:   line,
			})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			previous = lineID
		}
	}
	return ops, nil
}

func (b *VaultProjectionBridge) extractTopicOps(builder *opBuilder) ([]domain.OpEnvelope, error) {
	glob := filepath.Join(b.vaultPath, "topics", "*.md")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("glob topics: %w", err)
	}
	sort.Strings(matches)

	ops := []domain.OpEnvelope{}
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		slugName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		entityKey := "topic/" + slugName

		tombstone, err := builder.make(entityKey, domain.OpKindTombstone, map[string]any{"reason": "snapshot_refresh", "digest": hashHex(string(content))})
		if err != nil {
			return nil, err
		}
		ops = append(ops, tombstone)

		op, err := builder.make(entityKey, domain.OpKindPutRegister, domain.RegisterPayload{Field: "title", Value: mustRawJSON(slugName)})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
		op, err = builder.make(entityKey, domain.OpKindPutRegister, domain.RegisterPayload{Field: "note_path", Value: mustRawJSON(path)})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)

		_, body, err := markdown.SplitFrontmatter(string(content))
		if err != nil {
			body = string(content)
		}
		lines := extractManagedLines(body, topicManagedStart, topicManagedEnd)
		if len(lines) == 0 {
			lines = extractTopicSourceLines(body)
		}

		previous := ""
		for index, line := range lines {
			lineID := hashHex(entityKey + "|managed_lines|" + strconv.Itoa(index) + "|" + line)
			op, err := builder.make(entityKey, domain.OpKindInsertSeq, domain.InsertSequencePayload{Field: "managed_lines", LineID: lineID, AfterID: previous, Value: line})
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			previous = lineID
		}
	}
	return ops, nil
}

func (b *VaultProjectionBridge) applySource(entityKey string, entity domain.EntityState) error {
	notePath := jsonString(entity.Registers["note_path"].Value)
	if notePath == "" {
		notePath = b.findSourceNoteByID(strings.TrimPrefix(entityKey, "source/"))
	}
	if notePath == "" {
		title := jsonString(entity.Registers["title"].Value)
		notePath = filepath.Join(b.vaultPath, "sources", slug.Make(title)+".md")
	}
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return err
	}

	meta := map[string]any{}
	body := "## Summary\n\n## Highlights\n\n## Questions\n\n## Next Actions\n"
	if content, err := os.ReadFile(notePath); err == nil {
		existingMeta, existingBody, splitErr := markdown.SplitFrontmatter(string(content))
		if splitErr == nil {
			meta = existingMeta
			body = existingBody
		}
	}

	setIfPresent(meta, "schema_version", 1)
	copyRegister(meta, entity, "id")
	copyRegister(meta, entity, "type")
	copyRegister(meta, entity, "title")
	copyRegister(meta, entity, "authors")
	copyRegister(meta, entity, "url")
	copyRegister(meta, entity, "file_path")
	copyRegister(meta, entity, "status")
	copyRegister(meta, entity, "progress_percent")
	copyRegister(meta, entity, "unit_kind")
	copyRegister(meta, entity, "unit_current")
	copyRegister(meta, entity, "unit_total")
	copyRegister(meta, entity, "last_session_id")
	setIfPresent(meta, "tags", entity.RenderSet("tags"))
	setIfPresent(meta, "topics", entity.RenderSet("topics"))

	links := strings.Join(entity.RenderSequence("managed_links"), "\n")
	body = markdown.ReplaceManagedBlock(body, librarydomain.ManagedLinksStart, librarydomain.ManagedLinksEnd, links)
	rendered, err := markdown.RenderFrontmatter(meta, body)
	if err != nil {
		return err
	}
	return os.WriteFile(notePath, []byte(rendered), 0o644)
}

func (b *VaultProjectionBridge) applySession(entityKey string, entity domain.EntityState) error {
	notePath := jsonString(entity.Registers["note_path"].Value)
	if notePath == "" {
		id := strings.TrimPrefix(entityKey, "session/")
		notePath = filepath.Join(b.vaultPath, "sessions", id+".md")
	}
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return err
	}
	meta := map[string]any{}
	body := "# Session\n"
	if content, err := os.ReadFile(notePath); err == nil {
		existingMeta, existingBody, splitErr := markdown.SplitFrontmatter(string(content))
		if splitErr == nil {
			meta = existingMeta
			body = existingBody
		}
	}

	setIfPresent(meta, "schema_version", 1)
	copyRegister(meta, entity, "id")
	copyRegister(meta, entity, "source_id")
	copyRegister(meta, entity, "started_at")
	copyRegister(meta, entity, "ended_at")
	copyRegister(meta, entity, "duration_minutes")
	copyRegister(meta, entity, "goal")
	copyRegister(meta, entity, "outcome")
	copyRegister(meta, entity, "delta_progress")
	copyRegister(meta, entity, "progress_before")
	copyRegister(meta, entity, "progress_after")

	managedLines := entity.RenderSequence("managed_body")
	body = markdown.ReplaceManagedBlock(body, sessionManagedStart, sessionManagedEnd, strings.Join(managedLines, "\n"))
	rendered, err := markdown.RenderFrontmatter(meta, body)
	if err != nil {
		return err
	}
	return os.WriteFile(notePath, []byte(rendered), 0o644)
}

func (b *VaultProjectionBridge) applyTopic(entityKey string, entity domain.EntityState) error {
	slugName := strings.TrimPrefix(entityKey, "topic/")
	notePath := jsonString(entity.Registers["note_path"].Value)
	if notePath == "" {
		notePath = filepath.Join(b.vaultPath, "topics", slugName+".md")
	}
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return err
	}

	title := jsonString(entity.Registers["title"].Value)
	if title == "" {
		title = slugName
	}
	body := "# " + title + "\n"
	if content, err := os.ReadFile(notePath); err == nil {
		_, existingBody, splitErr := markdown.SplitFrontmatter(string(content))
		if splitErr == nil {
			body = existingBody
		} else {
			body = string(content)
		}
	}
	lines := entity.RenderSequence("managed_lines")
	body = markdown.ReplaceManagedBlock(body, topicManagedStart, topicManagedEnd, strings.Join(lines, "\n"))
	if !strings.Contains(body, "# ") {
		body = "# " + title + "\n\n" + body
	}
	return os.WriteFile(notePath, []byte(body), 0o644)
}

func (b *VaultProjectionBridge) findSourceNoteByID(id string) string {
	glob := filepath.Join(b.vaultPath, "sources", "*.md")
	matches, _ := filepath.Glob(glob)
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		meta, _, err := markdown.SplitFrontmatter(string(content))
		if err != nil {
			continue
		}
		if asString(meta["id"]) == id {
			return path
		}
	}
	return ""
}

func extractManagedLines(body, startMarker, endMarker string) []string {
	start := strings.Index(body, startMarker)
	end := strings.Index(body, endMarker)
	if start < 0 || end <= start {
		return nil
	}
	inner := body[start+len(startMarker) : end]
	lines := strings.Split(strings.Trim(inner, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func extractTopicSourceLines(body string) []string {
	idx := strings.Index(body, "## Sources")
	if idx < 0 {
		return nil
	}
	lines := strings.Split(body[idx:], "\n")
	out := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			out = append(out, line)
		}
	}
	return out
}

func mustRawJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func hashHex(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	out := ""
	if err := json.Unmarshal(raw, &out); err == nil {
		return out
	}
	return ""
}

func copyRegister(meta map[string]any, entity domain.EntityState, field string) {
	register, ok := entity.Registers[field]
	if !ok {
		return
	}
	value := any(nil)
	if err := json.Unmarshal(register.Value, &value); err != nil {
		return
	}
	setIfPresent(meta, field, value)
}

func setIfPresent(meta map[string]any, key string, value any) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return
		}
		meta[key] = v
	case []string:
		meta[key] = v
	case []any:
		meta[key] = v
	default:
		meta[key] = v
	}
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
		parsed, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err == nil {
			return parsed
		}
		return 0
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
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}
