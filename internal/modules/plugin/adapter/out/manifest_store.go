package out

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/plugin/domain"
	pluginout "mdht/internal/modules/plugin/port/out"
)

type FileManifestStore struct {
	basePath string
	path     string
}

func NewFileManifestStore(basePath string) pluginout.ManifestStore {
	return &FileManifestStore{basePath: basePath, path: filepath.Join(basePath, "plugins", "plugins.json")}
}

func (s *FileManifestStore) Load(_ context.Context) ([]domain.Manifest, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.Manifest{}, nil
		}
		return nil, fmt.Errorf("read plugin manifest store: %w", err)
	}
	var manifests []domain.Manifest
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifests); err != nil {
		return nil, fmt.Errorf("decode plugin manifests: %w", err)
	}
	for i := range manifests {
		if manifests[i].Binary != "" && !filepath.IsAbs(manifests[i].Binary) {
			manifests[i].Binary = filepath.Clean(filepath.Join(s.basePath, manifests[i].Binary))
		}
	}
	return manifests, nil
}
