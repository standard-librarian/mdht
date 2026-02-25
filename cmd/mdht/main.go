package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mdht/internal/bootstrap"
	plugindto "mdht/internal/modules/plugin/dto"
	"mdht/internal/platform/config"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var vaultPath string

	root := &cobra.Command{
		Use:           "mdht",
		Short:         "Markdown Terminal Hub Tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&vaultPath, "vault", ".", "Obsidian vault path")

	root.AddCommand(newTUICmd(&vaultPath))
	root.AddCommand(newIngestCmd(&vaultPath))
	root.AddCommand(newSourceCmd(&vaultPath))
	root.AddCommand(newSessionCmd(&vaultPath))
	root.AddCommand(newReaderCmd(&vaultPath))
	root.AddCommand(newReindexCmd(&vaultPath))
	root.AddCommand(newPluginCmd(&vaultPath))
	root.AddCommand(newCollabCmd(&vaultPath))
	return root
}

func loadApp(vaultPath string) (*bootstrap.App, error) {
	cfg, err := config.New(vaultPath)
	if err != nil {
		return nil, err
	}
	return bootstrap.New(cfg)
}

func newTUICmd(vaultPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Run mdht terminal UI",
		RunE: func(_ *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			return bootstrap.RunTUI(*vaultPath, app)
		},
	}
}

func newIngestCmd(vaultPath *string) *cobra.Command {
	ingest := &cobra.Command{Use: "ingest", Short: "Ingest learning sources"}

	var sourceType, title string
	var tags, topics []string

	fileCmd := &cobra.Command{
		Use:   "file <path>",
		Short: "Ingest a local file source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.LibraryCLI.IngestFile(context.Background(), sourceType, args[0], title, tags, topics)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ingested %s (%s) note=%s\n", out.Title, out.ID, out.NotePath)
			return nil
		},
	}
	fileCmd.Flags().StringVar(&sourceType, "type", "book", "source type: book|article|paper|video|course")
	fileCmd.Flags().StringVar(&title, "title", "", "source title (optional)")
	fileCmd.Flags().StringSliceVar(&tags, "tags", nil, "tags")
	fileCmd.Flags().StringSliceVar(&topics, "topics", nil, "topics")

	urlCmd := &cobra.Command{
		Use:   "url <url>",
		Short: "Ingest a URL source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.LibraryCLI.IngestURL(context.Background(), sourceType, args[0], title, tags, topics)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ingested %s (%s) note=%s\n", out.Title, out.ID, out.NotePath)
			return nil
		},
	}
	urlCmd.Flags().StringVar(&sourceType, "type", "article", "source type: book|article|paper|video|course")
	urlCmd.Flags().StringVar(&title, "title", "", "source title (optional)")
	urlCmd.Flags().StringSliceVar(&tags, "tags", nil, "tags")
	urlCmd.Flags().StringSliceVar(&topics, "topics", nil, "topics")

	ingest.AddCommand(fileCmd, urlCmd)
	return ingest
}

func newSourceCmd(vaultPath *string) *cobra.Command {
	source := &cobra.Command{Use: "source", Short: "Source query commands"}

	source.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List known sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			sources, err := app.LibraryCLI.ListSources(context.Background())
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no sources")
				return nil
			}
			for _, s := range sources {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%.1f%%\n", s.ID, s.Type, s.Title, s.Percent)
			}
			return nil
		},
	})

	var sourceID string
	show := &cobra.Command{
		Use:   "show --id <id>",
		Short: "Show source details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(sourceID) == "" {
				return fmt.Errorf("--id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			s, err := app.LibraryCLI.GetSource(context.Background(), sourceID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "id: %s\ntitle: %s\ntype: %s\nprogress: %.1f%%\nfile: %s\nurl: %s\nnote: %s\n", s.ID, s.Title, s.Type, s.Percent, s.FilePath, s.URL, s.NotePath)
			return nil
		},
	}
	show.Flags().StringVar(&sourceID, "id", "", "source id")
	source.AddCommand(show)
	return source
}

func newSessionCmd(vaultPath *string) *cobra.Command {
	session := &cobra.Command{Use: "session", Short: "Study session lifecycle"}

	var sourceID, goal string
	start := &cobra.Command{
		Use:   "start --source-id <id>",
		Short: "Start a session for a source",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(sourceID) == "" {
				return fmt.Errorf("--source-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.SessionCLI.Start(context.Background(), sourceID, "", goal)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "session started: %s source=%s at=%s\n", out.SessionID, out.SourceID, out.StartedAt.Format("2006-01-02T15:04:05Z07:00"))
			return nil
		},
	}
	start.Flags().StringVar(&sourceID, "source-id", "", "source id")
	start.Flags().StringVar(&goal, "goal", "", "study goal")

	var outcome, sessionID string
	var delta float64
	end := &cobra.Command{
		Use:   "end --outcome <text> --delta-progress <value>",
		Short: "End active session and apply progress delta",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(outcome) == "" {
				return fmt.Errorf("--outcome is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.SessionCLI.End(context.Background(), sessionID, outcome, delta)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "session ended: %s source=%s duration=%dmin delta=%.2f before=%.2f after=%.2f note=%s\n", out.SessionID, out.SourceID, out.DurationMin, out.DeltaProgress, out.ProgressBefore, out.ProgressAfter, out.Path)
			return nil
		},
	}
	end.Flags().StringVar(&sessionID, "session-id", "", "optional session id (defaults to active session)")
	end.Flags().StringVar(&outcome, "outcome", "", "session outcome")
	end.Flags().Float64Var(&delta, "delta-progress", 0, "delta progress to add (0..100)")

	session.AddCommand(start, end)
	return session
}

func newReaderCmd(vaultPath *string) *cobra.Command {
	reader := &cobra.Command{Use: "reader", Short: "Reader operations"}

	var sourceID, mode string
	var page int
	var external bool
	open := &cobra.Command{
		Use:   "open --source-id <id>",
		Short: "Open source in reader flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(sourceID) == "" {
				return fmt.Errorf("--source-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.ReaderCLI.OpenSource(context.Background(), sourceID, mode, page, external)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "source=%s title=%q type=%s mode=%s progress=%.1f%%\n", out.SourceID, out.Title, out.Type, out.Mode, out.Percent)
			if out.Page > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "page=%d/%d\n", out.Page, out.TotalPage)
			}
			if out.ExternalTarget != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "target=%s launched=%t\n", out.ExternalTarget, out.ExternalLaunched)
			}
			if strings.TrimSpace(out.Content) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), out.Content)
			}
			return nil
		},
	}
	open.Flags().StringVar(&sourceID, "source-id", "", "source id")
	open.Flags().StringVar(&mode, "mode", "auto", "reader mode: auto|markdown|pdf")
	open.Flags().IntVar(&page, "page", 1, "pdf page")
	open.Flags().BoolVar(&external, "external", false, "launch external target when applicable")

	reader.AddCommand(open)
	return reader
}

func newReindexCmd(vaultPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild SQLite projections from vault markdown",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			if err := app.LibraryCLI.Reindex(context.Background()); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "reindex completed")
			return nil
		},
	}
}

func newPluginCmd(vaultPath *string) *cobra.Command {
	plugin := &cobra.Command{Use: "plugin", Short: "Plugin operations"}
	plugin.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List plugin manifests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			plugins, err := app.PluginCLI.List(context.Background())
			if err != nil {
				return err
			}
			if len(plugins) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no plugins configured")
				return nil
			}
			for _, p := range plugins {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s@%s enabled=%t binary=%s\n", p.Name, p.Version, p.Enabled, p.Binary)
			}
			return nil
		},
	})

	plugin.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Validate plugin checksums and lifecycle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			results, err := app.PluginCLI.Doctor(context.Background())
			if err != nil {
				return err
			}
			if len(results) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no plugins configured")
				return nil
			}
			for _, r := range results {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s checksum=%t binary=%t lifecycle=%t", r.Name, r.ChecksumValid, r.BinaryReachable, r.LifecycleOK)
				if r.Error != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), " error=%q", r.Error)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	})

	var commandPluginName string
	commandsCmd := &cobra.Command{
		Use:   "commands --plugin <name>",
		Short: "List commands exposed by a plugin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(commandPluginName) == "" {
				return fmt.Errorf("--plugin is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			commands, err := app.PluginCLI.ListCommands(context.Background(), commandPluginName)
			if err != nil {
				return err
			}
			if len(commands) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no commands")
				return nil
			}
			for _, item := range commands {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s kind=%s timeout_ms=%d title=%q\n", item.ID, item.Kind, item.TimeoutMS, item.Title)
			}
			return nil
		},
	}
	commandsCmd.Flags().StringVar(&commandPluginName, "plugin", "", "plugin name")
	plugin.AddCommand(commandsCmd)

	var execPluginName, execCommandID, execInputJSON, execSourceID, execSessionID string
	execCmd := &cobra.Command{
		Use:   "exec --plugin <name> --command <id>",
		Short: "Execute a plugin command capability",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(execPluginName) == "" || strings.TrimSpace(execCommandID) == "" {
				return fmt.Errorf("--plugin and --command are required")
			}
			if err := validateJSONInput(execInputJSON); err != nil {
				return err
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.PluginCLI.Execute(context.Background(), plugindto.ExecuteInput{
				PluginName: execPluginName,
				CommandID:  execCommandID,
				InputJSON:  execInputJSON,
				SourceID:   execSourceID,
				SessionID:  execSessionID,
				VaultPath:  *vaultPath,
				Cwd:        *vaultPath,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "plugin=%s command=%s exit=%d\n", out.PluginName, out.CommandID, out.ExitCode)
			if strings.TrimSpace(out.Stdout) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), out.Stdout)
			}
			if strings.TrimSpace(out.Stderr) != "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), out.Stderr)
			}
			if strings.TrimSpace(out.OutputJSON) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), out.OutputJSON)
			}
			return nil
		},
	}
	execCmd.Flags().StringVar(&execPluginName, "plugin", "", "plugin name")
	execCmd.Flags().StringVar(&execCommandID, "command", "", "command id")
	execCmd.Flags().StringVar(&execInputJSON, "input-json", "", "JSON input payload")
	execCmd.Flags().StringVar(&execSourceID, "source-id", "", "optional source id")
	execCmd.Flags().StringVar(&execSessionID, "session-id", "", "optional session id")
	plugin.AddCommand(execCmd)

	var analyzePluginName, analyzeCommandID, analyzeInputJSON, analyzeSourceID string
	analyzeCmd := &cobra.Command{
		Use:   "analyze --plugin <name> --command <id> --source-id <id>",
		Short: "Execute an analyze-capability plugin command",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(analyzePluginName) == "" || strings.TrimSpace(analyzeCommandID) == "" || strings.TrimSpace(analyzeSourceID) == "" {
				return fmt.Errorf("--plugin, --command, and --source-id are required")
			}
			if err := validateJSONInput(analyzeInputJSON); err != nil {
				return err
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.PluginCLI.Analyze(context.Background(), plugindto.ExecuteInput{
				PluginName: analyzePluginName,
				CommandID:  analyzeCommandID,
				InputJSON:  analyzeInputJSON,
				SourceID:   analyzeSourceID,
				VaultPath:  *vaultPath,
				Cwd:        *vaultPath,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "plugin=%s command=%s exit=%d\n", out.PluginName, out.CommandID, out.ExitCode)
			if strings.TrimSpace(out.Stdout) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), out.Stdout)
			}
			if strings.TrimSpace(out.Stderr) != "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), out.Stderr)
			}
			if strings.TrimSpace(out.OutputJSON) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), out.OutputJSON)
			}
			return nil
		},
	}
	analyzeCmd.Flags().StringVar(&analyzePluginName, "plugin", "", "plugin name")
	analyzeCmd.Flags().StringVar(&analyzeCommandID, "command", "", "command id")
	analyzeCmd.Flags().StringVar(&analyzeInputJSON, "input-json", "", "JSON input payload")
	analyzeCmd.Flags().StringVar(&analyzeSourceID, "source-id", "", "source id")
	plugin.AddCommand(analyzeCmd)

	var ttyPluginName, ttyCommandID, ttyInputJSON, ttySourceID, ttySessionID string
	ttyCmd := &cobra.Command{
		Use:   "tty --plugin <name> --command <id>",
		Short: "Prepare and run fullscreen tty plugin command",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(ttyPluginName) == "" || strings.TrimSpace(ttyCommandID) == "" {
				return fmt.Errorf("--plugin and --command are required")
			}
			if err := validateJSONInput(ttyInputJSON); err != nil {
				return err
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			plan, err := app.PluginCLI.PrepareTTY(context.Background(), plugindto.TTYPrepareInput{
				PluginName: ttyPluginName,
				CommandID:  ttyCommandID,
				InputJSON:  ttyInputJSON,
				SourceID:   ttySourceID,
				SessionID:  ttySessionID,
				VaultPath:  *vaultPath,
				Cwd:        *vaultPath,
			})
			if err != nil {
				return err
			}
			return runTTYPlan(plan)
		},
	}
	ttyCmd.Flags().StringVar(&ttyPluginName, "plugin", "", "plugin name")
	ttyCmd.Flags().StringVar(&ttyCommandID, "command", "", "command id")
	ttyCmd.Flags().StringVar(&ttyInputJSON, "input-json", "", "JSON input payload")
	ttyCmd.Flags().StringVar(&ttySourceID, "source-id", "", "optional source id")
	ttyCmd.Flags().StringVar(&ttySessionID, "session-id", "", "optional session id")
	plugin.AddCommand(ttyCmd)

	return plugin
}

func validateJSONInput(input string) error {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	if !json.Valid([]byte(input)) {
		return fmt.Errorf("--input-json must be valid JSON")
	}
	return nil
}

func runTTYPlan(plan plugindto.TTYPrepareOutput) error {
	if len(plan.Argv) == 0 {
		return fmt.Errorf("plugin tty plan has empty argv")
	}
	cmd := exec.Command(plan.Argv[0], plan.Argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if plan.Cwd != "" {
		cmd.Dir = plan.Cwd
	}
	env := os.Environ()
	for key, value := range plan.Env {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	return cmd.Run()
}

func newCollabCmd(vaultPath *string) *cobra.Command {
	collab := &cobra.Command{Use: "collab", Short: "Collaboration transport commands"}

	daemon := &cobra.Command{Use: "daemon", Short: "Manage collab daemon lifecycle"}
	daemon.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run collab daemon in foreground",
		RunE: func(_ *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			return app.CollabCLI.RunDaemon(context.Background())
		},
	})
	daemon.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start collab daemon in background",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			if err := app.CollabCLI.StartDaemon(context.Background()); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "daemon started")
			return nil
		},
	})
	daemon.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop collab daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			if err := app.CollabCLI.StopDaemon(context.Background()); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "daemon stopped")
			return nil
		},
	})
	daemon.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show collab daemon status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			status, err := app.CollabCLI.DaemonStatus(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "running=%t pid=%d socket=%s\n", status.Running, status.PID, status.SocketPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "online=%t peers=%d pending_ops=%d workspace=%s node=%s\n", status.Status.Online, status.Status.PeerCount, status.Status.PendingOps, status.Status.WorkspaceID, status.Status.NodeID)
			if len(status.Status.ListenAddrs) > 0 {
				for _, addr := range status.Status.ListenAddrs {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), addr)
				}
			}
			return nil
		},
	})
	var daemonLogTail int
	daemonLogs := &cobra.Command{
		Use:   "logs",
		Short: "Show collab daemon logs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			payload, err := app.CollabCLI.DaemonLogs(context.Background(), daemonLogTail)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), payload)
			return nil
		},
	}
	daemonLogs.Flags().IntVar(&daemonLogTail, "tail", 200, "log lines to show from the end")
	daemon.AddCommand(daemonLogs)
	collab.AddCommand(daemon)

	var workspaceName string
	workspace := &cobra.Command{Use: "workspace", Short: "Manage collab workspace"}
	initCmd := &cobra.Command{
		Use:   "init --name <name>",
		Short: "Initialize workspace keys and identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(workspaceName) == "" {
				return fmt.Errorf("--name is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.WorkspaceInit(context.Background(), workspaceName)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace initialized: %s (%s)\n", out.Name, out.ID)
			return nil
		},
	}
	initCmd.Flags().StringVar(&workspaceName, "name", "", "workspace name")
	workspace.AddCommand(initCmd)
	workspace.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show workspace metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.WorkspaceShow(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace=%s id=%s node=%s peers=%d\n", out.Workspace.Name, out.Workspace.ID, out.NodeID, out.Peers)
			return nil
		},
	})
	collab.AddCommand(workspace)

	var peerAddr, peerID string
	peer := &cobra.Command{Use: "peer", Short: "Manage collab peers"}
	addPeer := &cobra.Command{
		Use:   "add --addr <multiaddr>",
		Short: "Add a peer bootstrap address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerAddr) == "" {
				return fmt.Errorf("--addr is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.PeerAdd(context.Background(), peerAddr)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer added: %s %s\n", out.PeerID, out.Address)
			return nil
		},
	}
	addPeer.Flags().StringVar(&peerAddr, "addr", "", "peer multiaddr")
	peer.AddCommand(addPeer)
	removePeer := &cobra.Command{
		Use:   "remove --peer-id <id>",
		Short: "Remove a configured peer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerID) == "" {
				return fmt.Errorf("--peer-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			if err := app.CollabCLI.PeerRemove(context.Background(), peerID); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer removed: %s\n", peerID)
			return nil
		},
	}
	removePeer.Flags().StringVar(&peerID, "peer-id", "", "peer id")
	peer.AddCommand(removePeer)
	peer.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured peers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.PeerList(context.Background())
			if err != nil {
				return err
			}
			if len(out) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no peers configured")
				return nil
			}
			for _, item := range out {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", item.PeerID, item.Address)
			}
			return nil
		},
	})
	collab.AddCommand(peer)

	collab.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show collab runtime status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			status, err := app.CollabCLI.Status(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "online=%t peers=%d pending_ops=%d workspace=%s node=%s\n", status.Online, status.PeerCount, status.PendingOps, status.WorkspaceID, status.NodeID)
			if !status.LastSyncAt.IsZero() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "last_sync=%s\n", status.LastSyncAt.Format(time.RFC3339))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "counters invalid_auth=%d workspace_mismatch=%d unauthenticated=%d decode_errors=%d reconnect_attempts=%d reconnect_successes=%d\n",
				status.Counters.InvalidAuthTag,
				status.Counters.WorkspaceMismatch,
				status.Counters.UnauthenticatedPeer,
				status.Counters.DecodeErrors,
				status.Counters.ReconnectAttempts,
				status.Counters.ReconnectSuccesses,
			)
			return nil
		},
	})

	collab.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Run collaboration health checks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.Doctor(context.Background())
			if err != nil {
				return err
			}
			exitErr := false
			for _, check := range out.Checks {
				marker := "OK"
				if !check.OK {
					marker = "FAIL"
					exitErr = true
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", marker, check.Name, check.Details)
			}
			if exitErr {
				return fmt.Errorf("collab doctor found failing checks")
			}
			return nil
		},
	})

	collab.AddCommand(&cobra.Command{
		Use:   "reconcile",
		Short: "Trigger anti-entropy reconciliation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.ReconcileNow(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reconciled ops=%d\n", out.Applied)
			return nil
		},
	})

	collab.AddCommand(&cobra.Command{
		Use:   "export-state",
		Short: "Export local collab state payload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			payload, err := app.CollabCLI.ExportState(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), payload)
			return nil
		},
	})

	return collab
}
