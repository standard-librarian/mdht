package domain_test

import (
	"testing"

	"mdht/internal/modules/plugin/domain"
)

func TestManifestValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		manifest  domain.Manifest
		shouldErr bool
	}{
		{name: "valid", manifest: domain.Manifest{Name: "p", Version: "1", Binary: "/tmp/p", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, Capabilities: []domain.Capability{domain.CapabilityCommand}}, shouldErr: false},
		{name: "missing name", manifest: domain.Manifest{Version: "1", Binary: "/tmp/p", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, Capabilities: []domain.Capability{domain.CapabilityCommand}}, shouldErr: true},
		{name: "missing version", manifest: domain.Manifest{Name: "p", Binary: "/tmp/p", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, Capabilities: []domain.Capability{domain.CapabilityCommand}}, shouldErr: true},
		{name: "missing binary", manifest: domain.Manifest{Name: "p", Version: "1", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, Capabilities: []domain.Capability{domain.CapabilityCommand}}, shouldErr: true},
		{name: "missing sha", manifest: domain.Manifest{Name: "p", Version: "1", Binary: "/tmp/p", Enabled: true, Capabilities: []domain.Capability{domain.CapabilityCommand}}, shouldErr: true},
		{name: "invalid capability", manifest: domain.Manifest{Name: "p", Version: "1", Binary: "/tmp/p", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, Capabilities: []domain.Capability{"invalid"}}, shouldErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.manifest.Validate()
			if tc.shouldErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.shouldErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestCapabilityAndKindValidation(t *testing.T) {
	t.Parallel()
	if err := domain.CapabilityCommand.Validate(); err != nil {
		t.Fatalf("validate capability: %v", err)
	}
	if err := domain.Capability("invalid").Validate(); err == nil {
		t.Fatalf("expected invalid capability error")
	}
	if err := domain.CommandKindCommand.Validate(); err != nil {
		t.Fatalf("validate kind: %v", err)
	}
	if err := domain.CommandKind("bad").Validate(); err == nil {
		t.Fatalf("expected invalid kind error")
	}
}

func TestDescriptorContextAndTTYValidation(t *testing.T) {
	t.Parallel()
	manifest := domain.Manifest{
		Name:         "p",
		Version:      "1",
		Binary:       "/tmp/p",
		SHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Enabled:      true,
		Capabilities: []domain.Capability{domain.CapabilityCommand, domain.CapabilityAnalyze},
	}
	if !manifest.HasCapability(domain.CapabilityAnalyze) {
		t.Fatalf("expected capability to exist")
	}
	if manifest.HasCapability(domain.CapabilityFullscreenTTY) {
		t.Fatalf("did not expect tty capability")
	}
	if err := (domain.CommandDescriptor{ID: "cmd", Kind: domain.CommandKindCommand}).Validate(); err != nil {
		t.Fatalf("descriptor validate: %v", err)
	}
	if err := (domain.ExecuteContext{VaultPath: "/tmp", Cwd: "/tmp"}).Validate(); err != nil {
		t.Fatalf("context validate: %v", err)
	}
	if err := (domain.ExecuteRequest{CommandID: "cmd", Context: domain.ExecuteContext{VaultPath: "/tmp", Cwd: "/tmp"}}).Validate(); err != nil {
		t.Fatalf("request validate: %v", err)
	}
	if err := (domain.TTYPlan{Argv: []string{"/bin/echo"}, Cwd: "/tmp"}).Validate(); err != nil {
		t.Fatalf("tty validate: %v", err)
	}
}
