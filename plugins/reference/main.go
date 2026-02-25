package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pluginrpc "mdht/internal/modules/plugin/adapter/out/rpc"

	"github.com/hashicorp/go-plugin"
)

type server struct{}

func (s *server) GetMetadata(_ context.Context, _ *pluginrpc.Empty) (*pluginrpc.Metadata, error) {
	return &pluginrpc.Metadata{
		Name:         "reference",
		Version:      "1.0.0",
		Capabilities: []string{"command", "analyze", "fullscreen_tty"},
	}, nil
}

func (s *server) ListCommands(_ context.Context, _ *pluginrpc.Empty) (*pluginrpc.ListCommandsResponse, error) {
	return &pluginrpc.ListCommandsResponse{Commands: []pluginrpc.CommandDescriptor{
		{ID: "echo", Title: "Echo", Description: "Echoes provided input", Kind: "command", TimeoutMS: 2000},
		{ID: "summarize", Title: "Summarize", Description: "Returns a deterministic summary payload", Kind: "analyze", TimeoutMS: 2500},
		{ID: "tty-echo", Title: "TTY Echo", Description: "Prepares a tty command", Kind: "fullscreen_tty", TimeoutMS: 1500},
	}}, nil
}

func (s *server) Execute(_ context.Context, in *pluginrpc.ExecuteRequest) (*pluginrpc.ExecuteResponse, error) {
	switch in.CommandID {
	case "echo":
		if strings.TrimSpace(in.InputJSON) == "" {
			return &pluginrpc.ExecuteResponse{Stdout: "echo", OutputJSON: `{"echo":""}`, ExitCode: 0}, nil
		}
		return &pluginrpc.ExecuteResponse{Stdout: in.InputJSON, OutputJSON: fmt.Sprintf(`{"echo":%q}`, in.InputJSON), ExitCode: 0}, nil
	case "summarize":
		payload := map[string]any{}
		if strings.TrimSpace(in.InputJSON) != "" {
			_ = json.Unmarshal([]byte(in.InputJSON), &payload)
		}
		summary := map[string]any{
			"source_id":  in.Context.SourceID,
			"session_id": in.Context.SessionID,
			"result":     "summary-ready",
			"input_keys": len(payload),
		}
		raw, _ := json.Marshal(summary)
		return &pluginrpc.ExecuteResponse{Stdout: "analysis complete", OutputJSON: string(raw), ExitCode: 0}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", in.CommandID)
	}
}

func (s *server) PrepareTTY(_ context.Context, in *pluginrpc.PrepareTTYRequest) (*pluginrpc.PrepareTTYResponse, error) {
	if in.CommandID != "tty-echo" {
		return nil, fmt.Errorf("unknown tty command: %s", in.CommandID)
	}
	return &pluginrpc.PrepareTTYResponse{
		Argv: []string{"/bin/sh", "-lc", "echo mdht-reference-tty"},
		Cwd:  in.Context.Cwd,
		Env: map[string]string{
			"MDHT_PLUGIN": "reference",
		},
	}, nil
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: pluginrpc.HandshakeConfig,
		Plugins:         pluginrpc.PluginMap(&server{}),
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
