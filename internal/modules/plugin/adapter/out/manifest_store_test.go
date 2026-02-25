package out_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pluginout "mdht/internal/modules/plugin/adapter/out"
)

func TestFileManifestStoreLoadMissingReturnsEmpty(t *testing.T) {
	t.Parallel()
	store := pluginout.NewFileManifestStore(t.TempDir())
	manifests, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("expected empty manifests, got %d", len(manifests))
	}
}

func TestFileManifestStoreResolvesRelativeBinary(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	pluginsDir := filepath.Join(base, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	raw := `[
  {
    "name": "reference",
    "version": "1.0.0",
    "binary": "plugins/reference/reference-plugin",
    "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "enabled": true,
    "capabilities": ["command"]
  }
]`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugins.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write plugins.json: %v", err)
	}
	store := pluginout.NewFileManifestStore(base)
	manifests, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected one manifest, got %d", len(manifests))
	}
	if !filepath.IsAbs(manifests[0].Binary) {
		t.Fatalf("expected absolute binary path, got %s", manifests[0].Binary)
	}
}

func TestFileManifestStoreRejectsUnknownField(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	pluginsDir := filepath.Join(base, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	raw := `[
  {
    "name": "reference",
    "version": "1.0.0",
    "binary": "/tmp/reference-plugin",
    "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "enabled": true,
    "capabilities": ["command"],
    "unknown_field": true
  }
]`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugins.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write plugins.json: %v", err)
	}
	store := pluginout.NewFileManifestStore(base)
	if _, err := store.Load(context.Background()); err == nil {
		t.Fatalf("expected unknown field error")
	}
}
