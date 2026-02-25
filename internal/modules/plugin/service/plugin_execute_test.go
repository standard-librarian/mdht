package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mdht/internal/modules/plugin/domain"
	"mdht/internal/modules/plugin/dto"
	"mdht/internal/modules/plugin/service"
)

type fakeStore struct {
	manifests []domain.Manifest
}

func (s fakeStore) Load(context.Context) ([]domain.Manifest, error) {
	return s.manifests, nil
}

type fakeHost struct {
	commands []domain.CommandDescriptor
}

func (fakeHost) CheckLifecycle(context.Context, domain.Manifest) error { return nil }
func (fakeHost) GetMetadata(context.Context, domain.Manifest) (domain.Metadata, error) {
	return domain.Metadata{Name: "fake", Version: "1"}, nil
}
func (h fakeHost) ListCommands(context.Context, domain.Manifest) ([]domain.CommandDescriptor, error) {
	return h.commands, nil
}
func (fakeHost) Execute(context.Context, domain.Manifest, domain.ExecuteRequest) (domain.ExecuteResult, error) {
	return domain.ExecuteResult{Stdout: "ok", ExitCode: 0}, nil
}
func (fakeHost) PrepareTTY(context.Context, domain.Manifest, domain.ExecuteRequest) (domain.TTYPlan, error) {
	return domain.TTYPlan{Argv: []string{"/bin/echo", "ok"}, Cwd: "/"}, nil
}

func TestExecuteRejectsDisabledPlugin(t *testing.T) {
	t.Parallel()
	manifest := manifestWithBinary(t, false, []domain.Capability{domain.CapabilityCommand})
	svc := service.NewPluginService(fakeStore{manifests: []domain.Manifest{manifest}}, fakeHost{})
	_, err := svc.Execute(context.Background(), dto.ExecuteInput{PluginName: manifest.Name, CommandID: "echo", VaultPath: "/tmp", Cwd: "/tmp"})
	if !errors.Is(err, domain.ErrPluginDisabled) {
		t.Fatalf("expected ErrPluginDisabled, got %v", err)
	}
}

func TestAnalyzeRejectsMissingCapability(t *testing.T) {
	t.Parallel()
	manifest := manifestWithBinary(t, true, []domain.Capability{domain.CapabilityCommand})
	svc := service.NewPluginService(fakeStore{manifests: []domain.Manifest{manifest}}, fakeHost{})
	_, err := svc.Analyze(context.Background(), dto.ExecuteInput{PluginName: manifest.Name, CommandID: "summarize", VaultPath: "/tmp", Cwd: "/tmp"})
	if !errors.Is(err, domain.ErrCapabilityMissing) {
		t.Fatalf("expected ErrCapabilityMissing, got %v", err)
	}
}

func TestExecuteRejectsUnknownCommand(t *testing.T) {
	t.Parallel()
	manifest := manifestWithBinary(t, true, []domain.Capability{domain.CapabilityCommand})
	svc := service.NewPluginService(fakeStore{manifests: []domain.Manifest{manifest}}, fakeHost{commands: []domain.CommandDescriptor{{ID: "other", Kind: domain.CommandKindCommand}}})
	_, err := svc.Execute(context.Background(), dto.ExecuteInput{PluginName: manifest.Name, CommandID: "echo", VaultPath: "/tmp", Cwd: "/tmp"})
	if !errors.Is(err, domain.ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	t.Parallel()
	manifest := manifestWithBinary(t, true, []domain.Capability{domain.CapabilityCommand})
	svc := service.NewPluginService(fakeStore{manifests: []domain.Manifest{manifest}}, fakeHost{commands: []domain.CommandDescriptor{{ID: "echo", Kind: domain.CommandKindCommand}}})
	out, err := svc.Execute(context.Background(), dto.ExecuteInput{PluginName: manifest.Name, CommandID: "echo", VaultPath: "/tmp", Cwd: "/tmp", InputJSON: `{"v":1}`})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
}

func manifestWithBinary(t *testing.T, enabled bool, capabilities []domain.Capability) domain.Manifest {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "plugin-bin")
	if err := os.WriteFile(binPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	hash := sha256.Sum256([]byte("binary"))
	return domain.Manifest{
		Name:         "demo",
		Version:      "1.0.0",
		Binary:       binPath,
		SHA256:       hex.EncodeToString(hash[:]),
		Enabled:      enabled,
		Capabilities: capabilities,
	}
}
