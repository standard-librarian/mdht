package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginout "mdht/internal/modules/plugin/adapter/out"
	"mdht/internal/modules/plugin/domain"
	"mdht/internal/modules/plugin/service"
)

func TestDoctorDetectsChecksumMismatch(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pluginsDir := filepath.Join(tmp, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	binPath := filepath.Join(tmp, "dummy-plugin")
	if err := os.WriteFile(binPath, []byte("not-a-real-plugin"), 0o755); err != nil {
		t.Fatalf("write plugin binary: %v", err)
	}
	manifests := []domain.Manifest{{
		Name:         "demo",
		Version:      "1.0.0",
		Binary:       binPath,
		SHA256:       strings.Repeat("0", 64),
		Enabled:      true,
		Capabilities: []domain.Capability{domain.CapabilityCommand},
	}}
	raw, _ := json.Marshal(manifests)
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugins.json"), raw, 0o644); err != nil {
		t.Fatalf("write plugins.json: %v", err)
	}

	svc := service.NewPluginService(pluginout.NewFileManifestStore(tmp), nil)
	results, err := svc.Doctor(context.Background())
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].ChecksumValid {
		t.Fatalf("expected checksum mismatch")
	}
}
