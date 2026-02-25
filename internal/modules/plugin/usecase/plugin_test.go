package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"mdht/internal/modules/plugin/domain"
	"mdht/internal/modules/plugin/dto"
	"mdht/internal/modules/plugin/service"
	"mdht/internal/modules/plugin/usecase"
)

type fakeManifestStore struct {
	manifests []domain.Manifest
}

func (s fakeManifestStore) Load(context.Context) ([]domain.Manifest, error) {
	return s.manifests, nil
}

type fakeHost struct{}

func (fakeHost) CheckLifecycle(context.Context, domain.Manifest) error { return nil }
func (fakeHost) GetMetadata(context.Context, domain.Manifest) (domain.Metadata, error) {
	return domain.Metadata{Name: "p1", Version: "1"}, nil
}
func (fakeHost) ListCommands(context.Context, domain.Manifest) ([]domain.CommandDescriptor, error) {
	return []domain.CommandDescriptor{
		{ID: "echo", Kind: domain.CommandKindCommand, TimeoutMS: 1000},
		{ID: "summarize", Kind: domain.CommandKindAnalyze, TimeoutMS: 1200},
		{ID: "tty-echo", Kind: domain.CommandKindTTY, TimeoutMS: 800},
	}, nil
}
func (fakeHost) Execute(context.Context, domain.Manifest, domain.ExecuteRequest) (domain.ExecuteResult, error) {
	return domain.ExecuteResult{Stdout: "ok", OutputJSON: `{"ok":true}`, ExitCode: 0}, nil
}
func (fakeHost) PrepareTTY(context.Context, domain.Manifest, domain.ExecuteRequest) (domain.TTYPlan, error) {
	return domain.TTYPlan{Argv: []string{"/bin/echo", "ok"}, Cwd: "/"}, nil
}

func TestUsecaseListDoctorAndOperations(t *testing.T) {
	t.Parallel()
	manifest := manifestWithBinary(t)
	uc := usecase.NewInteractor(service.NewPluginService(fakeManifestStore{manifests: []domain.Manifest{manifest}}, fakeHost{}))

	list, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "p1" {
		t.Fatalf("unexpected list: %+v", list)
	}

	docs, err := uc.Doctor(context.Background())
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("unexpected doctor result: %+v", docs)
	}

	commands, err := uc.ListCommands(context.Background(), "p1")
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("unexpected command count: %d", len(commands))
	}

	execOut, err := uc.Execute(context.Background(), dto.ExecuteInput{PluginName: "p1", CommandID: "echo", VaultPath: t.TempDir(), Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if execOut.ExitCode != 0 {
		t.Fatalf("unexpected execute result: %+v", execOut)
	}

	analyzeOut, err := uc.Analyze(context.Background(), dto.ExecuteInput{PluginName: "p1", CommandID: "summarize", SourceID: "s-1", VaultPath: t.TempDir(), Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analyzeOut.ExitCode != 0 {
		t.Fatalf("unexpected analyze result: %+v", analyzeOut)
	}

	ttyOut, err := uc.PrepareTTY(context.Background(), dto.TTYPrepareInput{PluginName: "p1", CommandID: "tty-echo", VaultPath: t.TempDir(), Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("prepare tty: %v", err)
	}
	if len(ttyOut.Argv) == 0 {
		t.Fatalf("expected tty argv")
	}
}

func manifestWithBinary(t *testing.T) domain.Manifest {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "plugin-bin")
	if err := os.WriteFile(binPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	hash := sha256.Sum256([]byte("binary"))
	return domain.Manifest{
		Name:         "p1",
		Version:      "1",
		Binary:       binPath,
		SHA256:       hex.EncodeToString(hash[:]),
		Enabled:      true,
		Capabilities: []domain.Capability{domain.CapabilityCommand, domain.CapabilityAnalyze, domain.CapabilityFullscreenTTY},
	}
}
