package out_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	pluginout "mdht/internal/modules/plugin/adapter/out"
	"mdht/internal/modules/plugin/domain"
)

func TestGRPCHostIntegrationReferencePlugin(t *testing.T) {
	binPath, checksum := buildReferencePlugin(t)
	manifest := domain.Manifest{
		Name:         "reference",
		Version:      "1.0.0",
		Binary:       binPath,
		SHA256:       checksum,
		Enabled:      true,
		Capabilities: []domain.Capability{domain.CapabilityCommand, domain.CapabilityAnalyze, domain.CapabilityFullscreenTTY},
	}

	host := pluginout.NewGRPCHost()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := host.CheckLifecycle(ctx, manifest); err != nil {
		t.Fatalf("check lifecycle: %v", err)
	}
	metadata, err := host.GetMetadata(ctx, manifest)
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	if metadata.Name != "reference" {
		t.Fatalf("unexpected metadata name: %s", metadata.Name)
	}
	commands, err := host.ListCommands(ctx, manifest)
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	if len(commands) < 3 {
		t.Fatalf("expected at least 3 commands, got %d", len(commands))
	}

	execOut, err := host.Execute(ctx, manifest, domain.ExecuteRequest{
		CommandID: "echo",
		InputJSON: `{"message":"hello"}`,
		Context: domain.ExecuteContext{
			VaultPath: t.TempDir(),
			Cwd:       t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if execOut.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", execOut.ExitCode)
	}

	ttyOut, err := host.PrepareTTY(ctx, manifest, domain.ExecuteRequest{
		CommandID: "tty-echo",
		Context: domain.ExecuteContext{
			VaultPath: t.TempDir(),
			Cwd:       t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("prepare tty: %v", err)
	}
	if len(ttyOut.Argv) == 0 {
		t.Fatalf("expected tty argv")
	}
}

func buildReferencePlugin(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "reference-plugin")
	cmd := exec.Command("go", "build", "-o", binPath, "./plugins/reference")
	cmd.Dir = repositoryRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build reference plugin: %v\n%s", err, string(out))
	}
	payload, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read built plugin: %v", err)
	}
	hash := sha256.Sum256(payload)
	return binPath, hex.EncodeToString(hash[:])
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../"))
}
