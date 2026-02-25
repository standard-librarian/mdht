package domain

import (
	"errors"
	"fmt"
	"regexp"
)

type Capability string

const (
	CapabilityCommand       Capability = "command"
	CapabilityAnalyze       Capability = "analyze"
	CapabilityFullscreenTTY Capability = "fullscreen_tty"
)

var (
	ErrPluginDisabled    = errors.New("plugin is disabled")
	ErrChecksumMismatch  = errors.New("plugin checksum mismatch")
	ErrCapabilityMissing = errors.New("plugin capability missing")
	ErrCommandNotFound   = errors.New("plugin command not found")
	ErrPluginTimeout     = errors.New("plugin timeout")
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Manifest struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Binary       string       `json:"binary"`
	SHA256       string       `json:"sha256"`
	Enabled      bool         `json:"enabled"`
	Capabilities []Capability `json:"capabilities"`
}

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if m.Binary == "" {
		return fmt.Errorf("plugin binary path is required")
	}
	if !sha256Pattern.MatchString(m.SHA256) {
		return fmt.Errorf("plugin sha256 must be lowercase 64-char hex")
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("plugin capabilities are required")
	}
	seen := map[Capability]struct{}{}
	for _, capability := range m.Capabilities {
		if err := capability.Validate(); err != nil {
			return err
		}
		if _, ok := seen[capability]; ok {
			return fmt.Errorf("duplicate capability: %s", capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func (c Capability) Validate() error {
	switch c {
	case CapabilityCommand, CapabilityAnalyze, CapabilityFullscreenTTY:
		return nil
	default:
		return fmt.Errorf("unknown capability: %s", c)
	}
}

func (m Manifest) HasCapability(capability Capability) bool {
	for _, c := range m.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

type CommandKind string

const (
	CommandKindCommand CommandKind = "command"
	CommandKindAnalyze CommandKind = "analyze"
	CommandKindTTY     CommandKind = "fullscreen_tty"
)

func (k CommandKind) Validate() error {
	switch k {
	case CommandKindCommand, CommandKindAnalyze, CommandKindTTY:
		return nil
	default:
		return fmt.Errorf("unknown command kind: %s", k)
	}
}

type CommandDescriptor struct {
	ID              string
	Title           string
	Description     string
	Kind            CommandKind
	InputSchemaJSON string
	TimeoutMS       int
}

func (d CommandDescriptor) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("command id is required")
	}
	if err := d.Kind.Validate(); err != nil {
		return err
	}
	return nil
}

type Metadata struct {
	Name         string
	Version      string
	Capabilities []Capability
}

type ExecuteContext struct {
	VaultPath string
	SourceID  string
	SessionID string
	Cwd       string
	Env       map[string]string
}

func (c ExecuteContext) Validate() error {
	if c.VaultPath == "" {
		return fmt.Errorf("vault path is required")
	}
	if c.Cwd == "" {
		return fmt.Errorf("cwd is required")
	}
	return nil
}

type ExecuteRequest struct {
	CommandID string
	InputJSON string
	Context   ExecuteContext
}

func (r ExecuteRequest) Validate() error {
	if r.CommandID == "" {
		return fmt.Errorf("command id is required")
	}
	return r.Context.Validate()
}

type ExecuteResult struct {
	Stdout     string
	Stderr     string
	OutputJSON string
	ExitCode   int
}

type TTYPlan struct {
	Argv []string
	Cwd  string
	Env  map[string]string
}

func (p TTYPlan) Validate() error {
	if len(p.Argv) == 0 {
		return fmt.Errorf("tty argv is required")
	}
	if p.Cwd == "" {
		return fmt.Errorf("tty cwd is required")
	}
	return nil
}
