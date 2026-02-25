package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mdht/internal/modules/plugin/domain"
	"mdht/internal/modules/plugin/dto"
	pluginout "mdht/internal/modules/plugin/port/out"
)

type PluginService struct {
	store pluginout.ManifestStore
	host  pluginout.Host
}

func NewPluginService(store pluginout.ManifestStore, host pluginout.Host) *PluginService {
	return &PluginService{store: store, host: host}
}

func (s *PluginService) List(ctx context.Context) ([]dto.PluginInfo, error) {
	manifests, err := s.loadValidated(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PluginInfo, 0, len(manifests))
	for _, m := range manifests {
		caps := make([]string, 0, len(m.Capabilities))
		for _, c := range m.Capabilities {
			caps = append(caps, string(c))
		}
		out = append(out, dto.PluginInfo{Name: m.Name, Version: m.Version, Enabled: m.Enabled, Binary: m.Binary, Capabilities: caps})
	}
	return out, nil
}

func (s *PluginService) Doctor(ctx context.Context) ([]dto.DoctorResult, error) {
	manifests, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]dto.DoctorResult, 0, len(manifests))
	for _, m := range manifests {
		result := dto.DoctorResult{Name: m.Name}
		if err := m.Validate(); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		binaryOK := fileExists(m.Binary)
		result.BinaryReachable = binaryOK
		checksumOK := false
		if binaryOK {
			checksumOK = checksumMatches(m.Binary, m.SHA256) == nil
		}
		result.ChecksumValid = checksumOK
		if binaryOK && checksumOK && m.Enabled && s.host != nil {
			if err := s.host.CheckLifecycle(ctx, m); err != nil {
				result.Error = err.Error()
			} else {
				result.LifecycleOK = true
			}
		}
		if !binaryOK {
			result.Error = fmt.Sprintf("binary does not exist: %s", m.Binary)
		}
		if binaryOK && !checksumOK {
			result.Error = "checksum mismatch"
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *PluginService) ListCommands(ctx context.Context, pluginName string) ([]dto.CommandInfo, error) {
	manifest, err := s.getRunnableManifest(ctx, pluginName, "")
	if err != nil {
		return nil, err
	}
	commands, err := s.host.ListCommands(ctx, manifest)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CommandInfo, 0, len(commands))
	for _, command := range commands {
		out = append(out, dto.CommandInfo{
			ID:              command.ID,
			Title:           command.Title,
			Description:     command.Description,
			Kind:            string(command.Kind),
			InputSchemaJSON: command.InputSchemaJSON,
			TimeoutMS:       command.TimeoutMS,
		})
	}
	return out, nil
}

func (s *PluginService) Execute(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return s.runCommand(ctx, input, domain.CapabilityCommand, domain.CommandKindCommand)
}

func (s *PluginService) Analyze(ctx context.Context, input dto.ExecuteInput) (dto.ExecuteOutput, error) {
	return s.runCommand(ctx, input, domain.CapabilityAnalyze, domain.CommandKindAnalyze)
}

func (s *PluginService) PrepareTTY(ctx context.Context, input dto.TTYPrepareInput) (dto.TTYPrepareOutput, error) {
	manifest, err := s.getRunnableManifest(ctx, input.PluginName, domain.CapabilityFullscreenTTY)
	if err != nil {
		return dto.TTYPrepareOutput{}, err
	}
	if input.InputJSON != "" && !json.Valid([]byte(input.InputJSON)) {
		return dto.TTYPrepareOutput{}, fmt.Errorf("input-json must be valid JSON")
	}
	req := domain.ExecuteRequest{
		CommandID: input.CommandID,
		InputJSON: input.InputJSON,
		Context: domain.ExecuteContext{
			VaultPath: input.VaultPath,
			SourceID:  input.SourceID,
			SessionID: input.SessionID,
			Cwd:       input.Cwd,
			Env:       input.Env,
		},
	}
	if err := req.Validate(); err != nil {
		return dto.TTYPrepareOutput{}, err
	}
	commands, err := s.host.ListCommands(ctx, manifest)
	if err != nil {
		return dto.TTYPrepareOutput{}, err
	}
	if _, err := requireCommand(commands, input.CommandID, domain.CommandKindTTY); err != nil {
		return dto.TTYPrepareOutput{}, err
	}

	plan, err := s.host.PrepareTTY(ctx, manifest, req)
	if err != nil {
		return dto.TTYPrepareOutput{}, err
	}
	if err := plan.Validate(); err != nil {
		return dto.TTYPrepareOutput{}, err
	}
	return dto.TTYPrepareOutput{
		PluginName: input.PluginName,
		CommandID:  input.CommandID,
		Argv:       plan.Argv,
		Cwd:        plan.Cwd,
		Env:        plan.Env,
	}, nil
}

func (s *PluginService) runCommand(ctx context.Context, input dto.ExecuteInput, requiredCapability domain.Capability, requiredKind domain.CommandKind) (dto.ExecuteOutput, error) {
	manifest, err := s.getRunnableManifest(ctx, input.PluginName, requiredCapability)
	if err != nil {
		return dto.ExecuteOutput{}, err
	}
	if input.InputJSON != "" && !json.Valid([]byte(input.InputJSON)) {
		return dto.ExecuteOutput{}, fmt.Errorf("input-json must be valid JSON")
	}
	req := domain.ExecuteRequest{
		CommandID: input.CommandID,
		InputJSON: input.InputJSON,
		Context: domain.ExecuteContext{
			VaultPath: input.VaultPath,
			SourceID:  input.SourceID,
			SessionID: input.SessionID,
			Cwd:       input.Cwd,
			Env:       input.Env,
		},
	}
	if err := req.Validate(); err != nil {
		return dto.ExecuteOutput{}, err
	}
	commands, err := s.host.ListCommands(ctx, manifest)
	if err != nil {
		return dto.ExecuteOutput{}, err
	}
	if _, err := requireCommand(commands, input.CommandID, requiredKind); err != nil {
		return dto.ExecuteOutput{}, err
	}

	result, err := s.host.Execute(ctx, manifest, req)
	if err != nil {
		return dto.ExecuteOutput{}, err
	}
	return dto.ExecuteOutput{
		PluginName: input.PluginName,
		CommandID:  input.CommandID,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		OutputJSON: result.OutputJSON,
		ExitCode:   result.ExitCode,
	}, nil
}

func (s *PluginService) loadValidated(ctx context.Context) ([]domain.Manifest, error) {
	manifests, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	seenNames := map[string]struct{}{}
	for _, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return nil, err
		}
		if _, ok := seenNames[manifest.Name]; ok {
			return nil, fmt.Errorf("duplicate plugin name: %s", manifest.Name)
		}
		seenNames[manifest.Name] = struct{}{}
	}
	return manifests, nil
}

func (s *PluginService) getRunnableManifest(ctx context.Context, pluginName string, requiredCapability domain.Capability) (domain.Manifest, error) {
	manifests, err := s.loadValidated(ctx)
	if err != nil {
		return domain.Manifest{}, err
	}
	manifest := domain.Manifest{}
	found := false
	for _, item := range manifests {
		if item.Name == pluginName {
			manifest = item
			found = true
			break
		}
	}
	if !found {
		return domain.Manifest{}, fmt.Errorf("plugin %q not found", pluginName)
	}
	if !manifest.Enabled {
		return domain.Manifest{}, fmt.Errorf("%w: %s", domain.ErrPluginDisabled, pluginName)
	}
	if requiredCapability != "" && !manifest.HasCapability(requiredCapability) {
		return domain.Manifest{}, fmt.Errorf("%w: %s", domain.ErrCapabilityMissing, requiredCapability)
	}
	if err := checksumMatches(manifest.Binary, manifest.SHA256); err != nil {
		return domain.Manifest{}, err
	}
	if s.host != nil {
		if err := s.host.CheckLifecycle(ctx, manifest); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return domain.Manifest{}, fmt.Errorf("%w: %s", domain.ErrPluginTimeout, pluginName)
			}
			return domain.Manifest{}, err
		}
	}
	return manifest, nil
}

func requireCommand(commands []domain.CommandDescriptor, commandID string, requiredKind domain.CommandKind) (domain.CommandDescriptor, error) {
	for _, command := range commands {
		if err := command.Validate(); err != nil {
			return domain.CommandDescriptor{}, err
		}
		if command.ID != commandID {
			continue
		}
		if requiredKind != "" && command.Kind != requiredKind {
			return domain.CommandDescriptor{}, fmt.Errorf("command kind mismatch: want=%s got=%s", requiredKind, command.Kind)
		}
		return command, nil
	}
	return domain.CommandDescriptor{}, fmt.Errorf("%w: %s", domain.ErrCommandNotFound, commandID)
}

func checksumMatches(path string, expected string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read plugin binary: %w", err)
	}
	hash := sha256.Sum256(payload)
	actual := hex.EncodeToString(hash[:])
	if actual != expected {
		return fmt.Errorf("%w: %s", domain.ErrChecksumMismatch, filepath.Base(path))
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
