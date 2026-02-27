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
	root.AddCommand(newGraphCmd(&vaultPath))
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

func newGraphCmd(vaultPath *string) *cobra.Command {
	graph := &cobra.Command{Use: "graph", Short: "Knowledge graph explore commands"}

	var topicsLimit int
	var topicsJSON bool
	topics := &cobra.Command{
		Use:   "topics",
		Short: "List projected topics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.GraphCLI.ListTopics(context.Background(), topicsLimit)
			if err != nil {
				return err
			}
			if topicsJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			if len(out) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no topics")
				return nil
			}
			for _, item := range out {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\n", item.TopicSlug, item.SourceCount)
			}
			return nil
		},
	}
	topics.Flags().IntVar(&topicsLimit, "limit", 50, "max topics to return")
	topics.Flags().BoolVar(&topicsJSON, "json", false, "output topics as JSON")
	graph.AddCommand(topics)

	var neighborsNode string
	var neighborsDepth int
	var neighborsJSON bool
	neighbors := &cobra.Command{
		Use:   "neighbors --node <source-id|topic-slug>",
		Short: "List graph neighbors for a source/topic node",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(neighborsNode) == "" {
				return fmt.Errorf("--node is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.GraphCLI.Neighbors(context.Background(), neighborsNode, neighborsDepth)
			if err != nil {
				return err
			}
			if neighborsJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "focus=%s depth=%d neighbors=%d\n", out.FocusID, out.Depth, len(out.Nodes))
			for _, item := range out.Nodes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Kind, item.ID, item.Label)
			}
			return nil
		},
	}
	neighbors.Flags().StringVar(&neighborsNode, "node", "", "source id or topic slug")
	neighbors.Flags().IntVar(&neighborsDepth, "depth", 1, "neighbor traversal depth (1..2)")
	neighbors.Flags().BoolVar(&neighborsJSON, "json", false, "output neighbors as JSON")
	graph.AddCommand(neighbors)

	var searchQuery string
	var searchJSON bool
	search := &cobra.Command{
		Use:   "search --query <text>",
		Short: "Search source/topic graph nodes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(searchQuery) == "" {
				return fmt.Errorf("--query is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.GraphCLI.Search(context.Background(), searchQuery)
			if err != nil {
				return err
			}
			if searchJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			if len(out) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no results")
				return nil
			}
			for _, item := range out {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Kind, item.ID, item.Label)
			}
			return nil
		},
	}
	search.Flags().StringVar(&searchQuery, "query", "", "search text")
	search.Flags().BoolVar(&searchJSON, "json", false, "output search results as JSON")
	graph.AddCommand(search)

	var pathFrom, pathTo string
	var pathJSON bool
	path := &cobra.Command{
		Use:   "path --from <id> --to <id>",
		Short: "Find shortest path between two graph nodes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(pathFrom) == "" || strings.TrimSpace(pathTo) == "" {
				return fmt.Errorf("--from and --to are required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.GraphCLI.Path(context.Background(), pathFrom, pathTo)
			if err != nil {
				return err
			}
			if pathJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			if !out.Found {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no path")
				return nil
			}
			for _, item := range out.Nodes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Kind, item.ID, item.Label)
			}
			return nil
		},
	}
	path.Flags().StringVar(&pathFrom, "from", "", "from node id")
	path.Flags().StringVar(&pathTo, "to", "", "to node id")
	path.Flags().BoolVar(&pathJSON, "json", false, "output path as JSON")
	graph.AddCommand(path)

	return graph
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
		Use:    "__run",
		Short:  "Run collab daemon in foreground (internal)",
		Hidden: true,
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "online=%t peers=%d pending_ops=%d workspace=%s node=%s reachability=%s nat_mode=%s connectivity=%s\n",
				status.Status.Online,
				status.Status.PeerCount,
				status.Status.PendingOps,
				status.Status.WorkspaceID,
				status.Status.NodeID,
				status.Status.Reachability,
				status.Status.NATMode,
				status.Status.Connectivity,
			)
			if len(status.Status.ListenAddrs) > 0 {
				for _, addr := range status.Status.ListenAddrs {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), addr)
				}
			}
			return nil
		},
	})
	var daemonLogTail int
	var daemonLogsJSON bool
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
			if daemonLogsJSON {
				lines := strings.Split(strings.TrimSpace(payload), "\n")
				encoded, _ := json.Marshal(lines)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), payload)
			}
			return nil
		},
	}
	daemonLogs.Flags().IntVar(&daemonLogTail, "tail", 200, "log lines to show from the end")
	daemonLogs.Flags().BoolVar(&daemonLogsJSON, "json", false, "output daemon logs as JSON array")
	daemon.AddCommand(daemonLogs)
	collab.AddCommand(daemon)

	var workspaceName string
	var workspaceGracePeriod time.Duration
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
	rotateKeyCmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate workspace key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.WorkspaceRotateKey(context.Background(), workspaceGracePeriod)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace key rotated: %s (%s)\n", out.Name, out.ID)
			return nil
		},
	}
	rotateKeyCmd.Flags().DurationVar(&workspaceGracePeriod, "grace-period", 0, "rotation grace period, e.g. 1h")
	workspace.AddCommand(rotateKeyCmd)
	collab.AddCommand(workspace)

	var peerAddr, peerID, peerLabel string
	var peerListJSON bool
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
			out, err := app.CollabCLI.PeerAdd(context.Background(), peerAddr, peerLabel)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer added: %s %s state=%s\n", out.PeerID, out.Address, out.State)
			return nil
		},
	}
	addPeer.Flags().StringVar(&peerAddr, "addr", "", "peer multiaddr")
	addPeer.Flags().StringVar(&peerLabel, "label", "", "peer label")
	peer.AddCommand(addPeer)
	approvePeer := &cobra.Command{
		Use:   "approve --peer-id <id>",
		Short: "Approve a configured peer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerID) == "" {
				return fmt.Errorf("--peer-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.PeerApprove(context.Background(), peerID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer approved: %s state=%s\n", out.PeerID, out.State)
			return nil
		},
	}
	approvePeer.Flags().StringVar(&peerID, "peer-id", "", "peer id")
	peer.AddCommand(approvePeer)
	revokePeer := &cobra.Command{
		Use:   "revoke --peer-id <id>",
		Short: "Revoke a configured peer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerID) == "" {
				return fmt.Errorf("--peer-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.PeerRevoke(context.Background(), peerID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer revoked: %s state=%s\n", out.PeerID, out.State)
			return nil
		},
	}
	revokePeer.Flags().StringVar(&peerID, "peer-id", "", "peer id")
	peer.AddCommand(revokePeer)
	dialPeer := &cobra.Command{
		Use:   "dial --peer-id <id>",
		Short: "Dial a peer and update connectivity health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerID) == "" {
				return fmt.Errorf("--peer-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.PeerDial(context.Background(), peerID)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "peer dial: %s result=%s traversal=%s rtt_ms=%d reachability=%s\n", out.PeerID, out.LastDialResult, out.TraversalMode, out.RTTMS, out.Reachability)
			return nil
		},
	}
	dialPeer.Flags().StringVar(&peerID, "peer-id", "", "peer id")
	peer.AddCommand(dialPeer)
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
	peerList := &cobra.Command{
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
			if peerListJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			for _, item := range out {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\treachability=%s\tdial=%s\ttraversal=%s\trtt_ms=%d\n",
					item.PeerID,
					item.State,
					item.Label,
					item.Address,
					item.Reachability,
					item.LastDialResult,
					item.TraversalMode,
					item.RTTMS,
				)
			}
			return nil
		},
	}
	peerList.Flags().BoolVar(&peerListJSON, "json", false, "output peers as JSON")
	peer.AddCommand(peerList)
	var peerLatencyJSON bool
	peerLatency := &cobra.Command{
		Use:   "latency",
		Short: "Measure peer latency",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			items, err := app.CollabCLI.PeerLatency(context.Background())
			if err != nil {
				return err
			}
			if len(items) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no peer latency data")
				return nil
			}
			if peerLatencyJSON {
				encoded, _ := json.Marshal(items)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			for _, item := range items {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%dms\n", item.PeerID, item.RTTMS)
			}
			return nil
		},
	}
	peerLatency.Flags().BoolVar(&peerLatencyJSON, "json", false, "output peer latency as JSON")
	peer.AddCommand(peerLatency)
	var peerTimelineSince time.Duration
	var peerTimelineLimit int
	var peerTimelineJSON bool
	peerTimeline := &cobra.Command{
		Use:   "timeline --peer-id <id>",
		Short: "Show presence timeline events for a peer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(peerID) == "" {
				return fmt.Errorf("--peer-id is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			since := time.Time{}
			if peerTimelineSince > 0 {
				since = time.Now().UTC().Add(-peerTimelineSince)
			}
			events, err := app.CollabCLI.ActivityTail(context.Background(), since, peerTimelineLimit)
			if err != nil {
				return err
			}
			filtered := make([]map[string]any, 0, len(events))
			for _, event := range events {
				if !isPresenceActivityType(event.Type) {
					continue
				}
				if event.Fields["peer_id"] != peerID {
					continue
				}
				filtered = append(filtered, map[string]any{
					"occurred_at": event.OccurredAt,
					"type":        event.Type,
					"message":     event.Message,
					"fields":      event.Fields,
				})
			}
			if peerTimelineJSON {
				encoded, _ := json.Marshal(filtered)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			if len(filtered) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no timeline events")
				return nil
			}
			for _, item := range filtered {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", item["occurred_at"].(time.Time).Format(time.RFC3339), item["type"], item["message"])
			}
			return nil
		},
	}
	peerTimeline.Flags().StringVar(&peerID, "peer-id", "", "peer id")
	peerTimeline.Flags().DurationVar(&peerTimelineSince, "since", 0, "filter events newer than duration (e.g. 2h)")
	peerTimeline.Flags().IntVar(&peerTimelineLimit, "limit", 200, "max number of events")
	peerTimeline.Flags().BoolVar(&peerTimelineJSON, "json", false, "output timeline as JSON")
	peer.AddCommand(peerTimeline)
	collab.AddCommand(peer)

	var statusJSON bool
	statusCmd := &cobra.Command{
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
			if statusJSON {
				encoded, _ := json.Marshal(status)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "online=%t peers=%d approved_peers=%d pending_conflicts=%d pending_ops=%d workspace=%s node=%s reachability=%s nat_mode=%s connectivity=%s\n",
				status.Online,
				status.PeerCount,
				status.ApprovedPeerCount,
				status.PendingConflicts,
				status.PendingOps,
				status.WorkspaceID,
				status.NodeID,
				status.Reachability,
				status.NATMode,
				status.Connectivity,
			)
			if !status.LastSyncAt.IsZero() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "last_sync=%s\n", status.LastSyncAt.Format(time.RFC3339))
			}
			if status.MetricsAddress != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "metrics=%s\n", status.MetricsAddress)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "counters auth_invalid=%d auth_workspace_mismatch=%d auth_unauthenticated_peer=%d transport_decode_errors=%d transport_reconnect_attempts=%d transport_reconnect_successes=%d transport_dial_attempts=%d transport_dial_successes=%d transport_dial_failures=%d transport_hole_punch_attempts=%d transport_hole_punch_successes=%d\n",
				status.Counters.InvalidAuthTag,
				status.Counters.WorkspaceMismatch,
				status.Counters.UnauthenticatedPeer,
				status.Counters.DecodeErrors,
				status.Counters.ReconnectAttempts,
				status.Counters.ReconnectSuccesses,
				status.Counters.DialAttempts,
				status.Counters.DialSuccesses,
				status.Counters.DialFailures,
				status.Counters.HolePunchAttempts,
				status.Counters.HolePunchSuccesses,
			)
			return nil
		},
	}
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output status as JSON")
	collab.AddCommand(statusCmd)

	var netStatusJSON bool
	netCmd := &cobra.Command{Use: "net", Short: "Connectivity diagnostics"}
	netStatus := &cobra.Command{
		Use:   "status",
		Short: "Show network/traversal status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			status, err := app.CollabCLI.NetStatus(context.Background())
			if err != nil {
				return err
			}
			if netStatusJSON {
				encoded, _ := json.Marshal(status)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "online=%t reachability=%s nat_mode=%s connectivity=%s peers=%d\n", status.Online, status.Reachability, status.NATMode, status.Connectivity, status.PeerCount)
			if !status.LastSyncAt.IsZero() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "last_sync=%s\n", status.LastSyncAt.Format(time.RFC3339))
			}
			for _, addr := range status.ListenAddrs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "listen=%s\n", addr)
			}
			return nil
		},
	}
	netStatus.Flags().BoolVar(&netStatusJSON, "json", false, "output net status as JSON")
	netProbe := &cobra.Command{
		Use:   "probe",
		Short: "Probe local network reachability and dialable addresses",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			probe, err := app.CollabCLI.NetProbe(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reachability=%s nat_mode=%s\n", probe.Reachability, probe.NATMode)
			for _, addr := range probe.ListenAddrs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "listen=%s\n", addr)
			}
			for _, addr := range probe.DialableAddrs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dialable=%s\n", addr)
			}
			return nil
		},
	}
	netCmd.AddCommand(netStatus, netProbe)
	collab.AddCommand(netCmd)

	var activitySince time.Duration
	var activityLimit int
	activity := &cobra.Command{Use: "activity", Short: "Read collaboration activity"}
	activityTail := &cobra.Command{
		Use:   "tail",
		Short: "Tail collaboration activity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			since := time.Time{}
			if activitySince > 0 {
				since = time.Now().UTC().Add(-activitySince)
			}
			events, err := app.CollabCLI.ActivityTail(context.Background(), since, activityLimit)
			if err != nil {
				return err
			}
			if len(events) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no activity")
				return nil
			}
			for _, event := range events {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", event.OccurredAt.Format(time.RFC3339), event.Type, event.Message)
			}
			return nil
		},
	}
	activityTail.Flags().DurationVar(&activitySince, "since", 0, "filter events newer than duration (e.g. 30m)")
	activityTail.Flags().IntVar(&activityLimit, "limit", 100, "max number of events")
	activity.AddCommand(activityTail)
	collab.AddCommand(activity)

	var presenceJSON bool
	presence := &cobra.Command{
		Use:   "presence",
		Short: "Show collaboration presence and peer health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			status, err := app.CollabCLI.Status(context.Background())
			if err != nil {
				return err
			}
			peers, err := app.CollabCLI.PeerList(context.Background())
			if err != nil {
				return err
			}
			if presenceJSON {
				payload := map[string]any{
					"status": status,
					"peers":  peers,
				}
				encoded, _ := json.Marshal(payload)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "connectivity=%s nat_mode=%s reachability=%s peers=%d approved=%d pending_conflicts=%d pending_ops=%d\n",
				status.Connectivity, status.NATMode, status.Reachability, status.PeerCount, status.ApprovedPeerCount, status.PendingConflicts, status.PendingOps)
			for _, item := range peers {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\treachability=%s\trtt_ms=%d\tdial=%s\ttraversal=%s\n",
					item.PeerID, item.State, item.Reachability, item.RTTMS, item.LastDialResult, item.TraversalMode)
			}
			return nil
		},
	}
	presence.Flags().BoolVar(&presenceJSON, "json", false, "output presence state as JSON")
	var presenceSince time.Duration
	var presenceLimit int
	presenceTail := &cobra.Command{
		Use:   "tail",
		Short: "Tail presence-related activity events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			since := time.Time{}
			if presenceSince > 0 {
				since = time.Now().UTC().Add(-presenceSince)
			}
			events, err := app.CollabCLI.ActivityTail(context.Background(), since, presenceLimit)
			if err != nil {
				return err
			}
			count := 0
			for _, event := range events {
				if !isPresenceActivityType(event.Type) {
					continue
				}
				count++
				peer := event.Fields["peer_id"]
				if strings.TrimSpace(peer) == "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", event.OccurredAt.Format(time.RFC3339), event.Type, event.Message)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s peer=%s %s\n", event.OccurredAt.Format(time.RFC3339), event.Type, peer, event.Message)
			}
			if count == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no presence events")
			}
			return nil
		},
	}
	presenceTail.Flags().DurationVar(&presenceSince, "since", 0, "filter events newer than duration (e.g. 30m)")
	presenceTail.Flags().IntVar(&presenceLimit, "limit", 100, "max number of events")
	presence.AddCommand(presenceTail)
	collab.AddCommand(presence)

	var conflictsEntity string
	var conflictID, conflictStrategy string
	conflicts := &cobra.Command{Use: "conflicts", Short: "Inspect and resolve conflicts"}
	conflictsList := &cobra.Command{
		Use:   "list",
		Short: "List open/resolved conflicts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			items, err := app.CollabCLI.ConflictsList(context.Background(), conflictsEntity)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no conflicts")
				return nil
			}
			for _, item := range items {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", item.ID, item.Status, item.EntityKey, item.Field)
			}
			return nil
		},
	}
	conflictsList.Flags().StringVar(&conflictsEntity, "entity", "", "optional entity key filter")
	conflictsResolve := &cobra.Command{
		Use:   "resolve --id <conflict-id> --strategy <local|remote|merge>",
		Short: "Resolve a conflict record",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(conflictID) == "" {
				return fmt.Errorf("--id is required")
			}
			if strings.TrimSpace(conflictStrategy) == "" {
				return fmt.Errorf("--strategy is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.ConflictResolve(context.Background(), conflictID, conflictStrategy)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resolved: %s strategy=%s status=%s\n", out.ID, out.Strategy, out.Status)
			return nil
		},
	}
	conflictsResolve.Flags().StringVar(&conflictID, "id", "", "conflict id")
	conflictsResolve.Flags().StringVar(&conflictStrategy, "strategy", "", "resolution strategy: local|remote|merge")
	conflicts.AddCommand(conflictsList, conflictsResolve)
	collab.AddCommand(conflicts)

	sync := &cobra.Command{Use: "sync", Short: "Synchronization commands"}
	syncNow := &cobra.Command{
		Use:   "now",
		Short: "Trigger anti-entropy synchronization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.SyncNow(context.Background())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "synced ops=%d\n", out.Applied)
			return nil
		},
	}
	sync.AddCommand(syncNow)
	var syncHealthJSON bool
	syncHealth := &cobra.Command{
		Use:   "health",
		Short: "Show synchronization health state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.SyncHealth(context.Background())
			if err != nil {
				return err
			}
			if syncHealthJSON {
				encoded, _ := json.Marshal(out)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "state=%s reason=%s lag_seconds=%d pending_ops=%d\n", out.State, out.Reason, out.LagSeconds, out.PendingOps)
			if !out.LastSyncAt.IsZero() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "last_sync=%s\n", out.LastSyncAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	syncHealth.Flags().BoolVar(&syncHealthJSON, "json", false, "output sync health as JSON")
	sync.AddCommand(syncHealth)
	collab.AddCommand(sync)

	var snapshotOut string
	snapshot := &cobra.Command{Use: "snapshot", Short: "Snapshot operations"}
	snapshotExport := &cobra.Command{
		Use:   "export --out <file>",
		Short: "Export local collab snapshot payload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(snapshotOut) == "" {
				return fmt.Errorf("--out is required")
			}
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			payload, err := app.CollabCLI.SnapshotExport(context.Background())
			if err != nil {
				return err
			}
			if err := os.WriteFile(snapshotOut, []byte(payload.Payload), 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "snapshot exported: %s\n", snapshotOut)
			return nil
		},
	}
	snapshotExport.Flags().StringVar(&snapshotOut, "out", "", "output file path")
	snapshot.AddCommand(snapshotExport)
	collab.AddCommand(snapshot)

	collab.AddCommand(&cobra.Command{
		Use:   "metrics",
		Short: "Show collab metrics snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadApp(*vaultPath)
			if err != nil {
				return err
			}
			out, err := app.CollabCLI.Metrics(context.Background())
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(out, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	})

	return collab
}
