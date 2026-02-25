package out

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type FileOpLogStore struct {
	path string
	mu   sync.Mutex
}

type FileSnapshotStore struct {
	path string
	mu   sync.Mutex
}

func NewFileOpLogStore(vaultPath string) collabout.OpLogStore {
	return &FileOpLogStore{path: filepath.Join(vaultPath, ".mdht", "collab", "oplog.jsonl")}
}

func NewFileSnapshotStore(vaultPath string) collabout.SnapshotStore {
	return &FileSnapshotStore{path: filepath.Join(vaultPath, ".mdht", "collab", "snapshot.json")}
}

func (s *FileOpLogStore) Append(_ context.Context, op domain.OpEnvelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create oplog dir: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open oplog: %w", err)
	}
	defer file.Close()
	payload, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("encode op envelope: %w", err)
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write oplog: %w", err)
	}
	return nil
}

func (s *FileOpLogStore) List(_ context.Context) ([]domain.OpEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.OpEnvelope{}, nil
		}
		return nil, fmt.Errorf("open oplog: %w", err)
	}
	defer file.Close()

	out := []domain.OpEnvelope{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		op := domain.OpEnvelope{}
		if err := json.Unmarshal(line, &op); err != nil {
			return nil, fmt.Errorf("decode oplog line: %w", err)
		}
		out = append(out, op)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan oplog: %w", err)
	}
	return out, nil
}

func (s *FileSnapshotStore) Load(_ context.Context) (domain.CRDTState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.NewCRDTState(), nil
		}
		return domain.CRDTState{}, fmt.Errorf("read snapshot: %w", err)
	}
	if len(raw) == 0 {
		return domain.NewCRDTState(), nil
	}
	state := domain.NewCRDTState()
	if err := json.Unmarshal(raw, &state); err != nil {
		return domain.CRDTState{}, fmt.Errorf("decode snapshot: %w", err)
	}
	if state.Entities == nil {
		state.Entities = map[string]domain.EntityState{}
	}
	if state.AppliedOps == nil {
		state.AppliedOps = map[string]struct{}{}
	}
	return state, nil
}

func (s *FileSnapshotStore) Save(_ context.Context, state domain.CRDTState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}
