package out

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	pluginrpc "mdht/internal/modules/plugin/adapter/out/rpc"
	"mdht/internal/modules/plugin/domain"
	pluginout "mdht/internal/modules/plugin/port/out"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

const (
	defaultStartTimeout = 3 * time.Second
	defaultCallTimeout  = 5 * time.Second
)

type GRPCHost struct{}

func NewGRPCHost() pluginout.Host {
	return &GRPCHost{}
}

func (h *GRPCHost) CheckLifecycle(ctx context.Context, manifest domain.Manifest) error {
	client, closeFn, err := h.connect(ctx, manifest, defaultStartTimeout)
	if err != nil {
		return err
	}
	defer closeFn()

	callCtx, cancel := h.callContext(ctx, defaultCallTimeout)
	defer cancel()
	if _, err := client.GetMetadata(callCtx); err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}
	return nil
}

func (h *GRPCHost) GetMetadata(ctx context.Context, manifest domain.Manifest) (domain.Metadata, error) {
	client, closeFn, err := h.connect(ctx, manifest, defaultStartTimeout)
	if err != nil {
		return domain.Metadata{}, err
	}
	defer closeFn()

	callCtx, cancel := h.callContext(ctx, defaultCallTimeout)
	defer cancel()

	meta, err := client.GetMetadata(callCtx)
	if err != nil {
		return domain.Metadata{}, fmt.Errorf("get metadata: %w", err)
	}
	capabilities := make([]domain.Capability, 0, len(meta.Capabilities))
	for _, capability := range meta.Capabilities {
		capabilities = append(capabilities, domain.Capability(capability))
	}
	return domain.Metadata{Name: meta.Name, Version: meta.Version, Capabilities: capabilities}, nil
}

func (h *GRPCHost) ListCommands(ctx context.Context, manifest domain.Manifest) ([]domain.CommandDescriptor, error) {
	client, closeFn, err := h.connect(ctx, manifest, defaultStartTimeout)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	callCtx, cancel := h.callContext(ctx, defaultCallTimeout)
	defer cancel()

	response, err := client.ListCommands(callCtx)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	out := make([]domain.CommandDescriptor, 0, len(response.Commands))
	for _, cmd := range response.Commands {
		out = append(out, domain.CommandDescriptor{
			ID:              cmd.ID,
			Title:           cmd.Title,
			Description:     cmd.Description,
			Kind:            domain.CommandKind(cmd.Kind),
			InputSchemaJSON: cmd.InputSchemaJSON,
			TimeoutMS:       int(cmd.TimeoutMS),
		})
	}
	return out, nil
}

func (h *GRPCHost) Execute(ctx context.Context, manifest domain.Manifest, input domain.ExecuteRequest) (domain.ExecuteResult, error) {
	client, closeFn, err := h.connect(ctx, manifest, defaultStartTimeout)
	if err != nil {
		return domain.ExecuteResult{}, err
	}
	defer closeFn()

	timeout := timeoutFor(input, defaultCallTimeout)
	callCtx, cancel := h.callContext(ctx, timeout)
	defer cancel()
	response, err := client.Execute(callCtx, &pluginrpc.ExecuteRequest{
		CommandID: input.CommandID,
		InputJSON: input.InputJSON,
		Context: pluginrpc.ExecuteContext{
			VaultPath: input.Context.VaultPath,
			SourceID:  input.Context.SourceID,
			SessionID: input.Context.SessionID,
			Cwd:       input.Context.Cwd,
			Env:       input.Context.Env,
		},
	})
	if err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			return domain.ExecuteResult{}, fmt.Errorf("%w: command %s", domain.ErrPluginTimeout, input.CommandID)
		}
		return domain.ExecuteResult{}, fmt.Errorf("execute command: %w", err)
	}
	return domain.ExecuteResult{
		Stdout:     response.Stdout,
		Stderr:     response.Stderr,
		OutputJSON: response.OutputJSON,
		ExitCode:   int(response.ExitCode),
	}, nil
}

func (h *GRPCHost) PrepareTTY(ctx context.Context, manifest domain.Manifest, input domain.ExecuteRequest) (domain.TTYPlan, error) {
	client, closeFn, err := h.connect(ctx, manifest, defaultStartTimeout)
	if err != nil {
		return domain.TTYPlan{}, err
	}
	defer closeFn()

	timeout := timeoutFor(input, defaultCallTimeout)
	callCtx, cancel := h.callContext(ctx, timeout)
	defer cancel()
	response, err := client.PrepareTTY(callCtx, &pluginrpc.PrepareTTYRequest{
		CommandID: input.CommandID,
		InputJSON: input.InputJSON,
		Context: pluginrpc.ExecuteContext{
			VaultPath: input.Context.VaultPath,
			SourceID:  input.Context.SourceID,
			SessionID: input.Context.SessionID,
			Cwd:       input.Context.Cwd,
			Env:       input.Context.Env,
		},
	})
	if err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			return domain.TTYPlan{}, fmt.Errorf("%w: command %s", domain.ErrPluginTimeout, input.CommandID)
		}
		return domain.TTYPlan{}, fmt.Errorf("prepare tty: %w", err)
	}
	return domain.TTYPlan{Argv: response.Argv, Cwd: response.Cwd, Env: response.Env}, nil
}

func (h *GRPCHost) connect(ctx context.Context, manifest domain.Manifest, startTimeout time.Duration) (pluginrpc.MdhtPluginClient, func(), error) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  pluginrpc.HandshakeConfig,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Plugins:          pluginrpc.PluginMap(nil),
		Cmd:              exec.Command(manifest.Binary),
		Managed:          true,
		StartTimeout:     startTimeout,
		Logger:           hclog.New(&hclog.LoggerOptions{Output: io.Discard, Level: hclog.NoLevel}),
	})
	closeFn := func() { client.Kill() }

	rpcClient, err := client.Client()
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("start plugin client: %w", err)
	}
	raw, err := rpcClient.Dispense(pluginrpc.PluginMapKey)
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("dispense plugin: %w", err)
	}
	typed, ok := raw.(pluginrpc.MdhtPluginClient)
	if !ok {
		closeFn()
		return nil, nil, fmt.Errorf("plugin rpc client type mismatch")
	}
	return typed, closeFn, nil
}

func timeoutFor(input domain.ExecuteRequest, fallback time.Duration) time.Duration {
	return fallback
}

func (h *GRPCHost) callContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := parent.Deadline(); ok {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}
