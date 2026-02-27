package out

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type FileActivityStore struct {
	path string
}

func NewFileActivityStore(vaultPath string) collabout.ActivityStore {
	return &FileActivityStore{path: filepath.Join(vaultPath, ".mdht", "collab", "activity.log")}
}

func (s *FileActivityStore) Append(_ context.Context, event domain.ActivityEvent) error {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create activity dir: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open activity log: %w", err)
	}
	defer file.Close()
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode activity event: %w", err)
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write activity log: %w", err)
	}
	return nil
}

func (s *FileActivityStore) Tail(_ context.Context, query collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	if query.Limit <= 0 {
		query.Limit = 200
	}
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.ActivityEvent{}, nil
		}
		return nil, fmt.Errorf("open activity log: %w", err)
	}
	defer file.Close()

	buffer := make([]domain.ActivityEvent, 0, query.Limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		event := domain.ActivityEvent{}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if !query.Since.IsZero() && event.OccurredAt.Before(query.Since.UTC()) {
			continue
		}
		if len(buffer) < query.Limit {
			buffer = append(buffer, event)
			continue
		}
		copy(buffer, buffer[1:])
		buffer[len(buffer)-1] = event
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan activity log: %w", err)
	}
	return buffer, nil
}
