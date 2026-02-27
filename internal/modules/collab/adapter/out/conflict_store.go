package out

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type FileConflictStore struct {
	path string
}

func NewFileConflictStore(vaultPath string) collabout.ConflictStore {
	return &FileConflictStore{path: filepath.Join(vaultPath, ".mdht", "collab", "conflicts.json")}
}

func (s *FileConflictStore) List(_ context.Context, entityKey string) ([]domain.ConflictRecord, error) {
	records, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	if entityKey == "" {
		return records, nil
	}
	filtered := make([]domain.ConflictRecord, 0, len(records))
	for _, record := range records {
		if record.EntityKey == entityKey {
			filtered = append(filtered, record)
		}
	}
	return filtered, nil
}

func (s *FileConflictStore) Upsert(_ context.Context, record domain.ConflictRecord) error {
	records, err := s.loadAll()
	if err != nil {
		return err
	}
	updated := false
	for i := range records {
		if records[i].ID == record.ID {
			records[i] = record
			updated = true
			break
		}
	}
	if !updated {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	payload, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conflicts: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create conflict dir: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write conflicts: %w", err)
	}
	return nil
}

func (s *FileConflictStore) loadAll() ([]domain.ConflictRecord, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.ConflictRecord{}, nil
		}
		return nil, fmt.Errorf("read conflicts: %w", err)
	}
	if len(raw) == 0 {
		return []domain.ConflictRecord{}, nil
	}
	out := []domain.ConflictRecord{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode conflicts: %w", err)
	}
	return out, nil
}
