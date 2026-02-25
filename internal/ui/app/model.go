package app

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	collabdto "mdht/internal/modules/collab/dto"
	"mdht/internal/modules/library/dto"
	plugindto "mdht/internal/modules/plugin/dto"
	readerdto "mdht/internal/modules/reader/dto"
	sessiondto "mdht/internal/modules/session/dto"
	apperrors "mdht/internal/platform/errors"
	"mdht/internal/ui/theme"
)

type libraryPort interface {
	ListSources(ctx context.Context) ([]dto.SourceOutput, error)
	GetSource(ctx context.Context, id string) (dto.SourceDetailOutput, error)
}

type sessionPort interface {
	Start(ctx context.Context, sourceID, sourceTitle, goal string) (sessiondto.StartOutput, error)
	End(ctx context.Context, sessionID, outcome string, delta float64) (sessiondto.EndOutput, error)
	GetActive(ctx context.Context) (sessiondto.ActiveSessionOutput, error)
}

type readerPort interface {
	OpenSource(ctx context.Context, sourceID, mode string, page int, launchExternal bool) (readerdto.OpenResult, error)
}

type pluginPort interface {
	ListCommands(ctx context.Context, pluginName string) ([]plugindto.CommandInfo, error)
	Execute(ctx context.Context, input plugindto.ExecuteInput) (plugindto.ExecuteOutput, error)
	Analyze(ctx context.Context, input plugindto.ExecuteInput) (plugindto.ExecuteOutput, error)
	PrepareTTY(ctx context.Context, input plugindto.TTYPrepareInput) (plugindto.TTYPrepareOutput, error)
}

type collabPort interface {
	Status(ctx context.Context) (collabdto.StatusOutput, error)
	PeerList(ctx context.Context) ([]collabdto.PeerOutput, error)
	ReconcileNow(ctx context.Context) (collabdto.ReconcileOutput, error)
}

type sourcesLoadedMsg struct {
	sources []dto.SourceOutput
	err     error
}

type activeLoadedMsg struct {
	active sessiondto.ActiveSessionOutput
	err    error
}

type detailLoadedMsg struct {
	detail dto.SourceDetailOutput
	err    error
}

type readerOpenedMsg struct {
	result readerdto.OpenResult
	err    error
}

type sessionStartedMsg struct {
	active sessiondto.ActiveSessionOutput
	err    error
}

type sessionEndedMsg struct {
	out sessiondto.EndOutput
	err error
}

type pluginCommandsMsg struct {
	pluginName string
	commands   []plugindto.CommandInfo
	err        error
}

type pluginExecMsg struct {
	out plugindto.ExecuteOutput
	err error
}

type pluginTTYReadyMsg struct {
	plan plugindto.TTYPrepareOutput
	err  error
}

type pluginTTYDoneMsg struct {
	err error
}

type collabStatusMsg struct {
	status collabdto.StatusOutput
	err    error
}

type collabPeersMsg struct {
	peers []collabdto.PeerOutput
	err   error
}

type collabReconcileMsg struct {
	applied int
	err     error
}

type Model struct {
	vaultPath string
	library   libraryPort
	session   sessionPort
	reader    readerPort
	plugin    pluginPort
	collab    collabPort

	sources       []dto.SourceOutput
	selectedIndex int
	detail        dto.SourceDetailOutput
	readerResult  readerdto.OpenResult

	activeSession sessiondto.ActiveSessionOutput
	hasActive     bool

	pluginNameForCommands string
	pluginCommands        []plugindto.CommandInfo
	pluginResult          plugindto.ExecuteOutput
	collabStatus          collabdto.StatusOutput
	collabPeers           []collabdto.PeerOutput

	showPalette  bool
	paletteInput string
	status       string

	width  int
	height int
}

func NewModel(vaultPath string, library libraryPort, session sessionPort, reader readerPort, plugin pluginPort, collab collabPort) Model {
	return Model{
		vaultPath:   vaultPath,
		library:     library,
		session:     session,
		reader:      reader,
		plugin:      plugin,
		collab:      collab,
		status:      "ready",
		hasActive:   false,
		showPalette: false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSourcesCmd(), m.loadActiveCmd(), m.loadCollabStatusCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case sourcesLoadedMsg:
		if msg.err != nil {
			m.status = "source load failed: " + msg.err.Error()
			return m, nil
		}
		m.sources = msg.sources
		if len(m.sources) > 0 && m.selectedIndex >= len(m.sources) {
			m.selectedIndex = len(m.sources) - 1
		}
		if len(m.sources) > 0 {
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil
	case activeLoadedMsg:
		if msg.err != nil {
			if msg.err == apperrors.ErrNoActiveSession {
				m.hasActive = false
				return m, nil
			}
			m.status = "active session check failed: " + msg.err.Error()
			return m, nil
		}
		m.hasActive = true
		m.activeSession = msg.active
		m.status = "active session recovered: " + msg.active.SourceTitle
		return m, nil
	case detailLoadedMsg:
		if msg.err != nil {
			m.status = "source detail load failed: " + msg.err.Error()
			return m, nil
		}
		m.detail = msg.detail
		return m, nil
	case readerOpenedMsg:
		if msg.err != nil {
			m.status = "reader open failed: " + msg.err.Error()
			return m, nil
		}
		m.readerResult = msg.result
		m.status = "reader opened: " + msg.result.Mode
		if msg.result.ExternalLaunched {
			m.status = "external source launched"
		}
		return m, nil
	case sessionStartedMsg:
		if msg.err != nil {
			m.status = "session start failed: " + msg.err.Error()
			return m, nil
		}
		m.activeSession = msg.active
		m.hasActive = true
		m.status = "session started"
		return m, nil
	case sessionEndedMsg:
		if msg.err != nil {
			m.status = "session end failed: " + msg.err.Error()
			return m, nil
		}
		m.hasActive = false
		m.activeSession = sessiondto.ActiveSessionOutput{}
		m.status = fmt.Sprintf("session ended (+%.2f%%)", msg.out.DeltaProgress)
		return m, tea.Batch(m.loadSourcesCmd(), m.loadSelectedDetailCmd())
	case pluginCommandsMsg:
		if msg.err != nil {
			m.status = "plugin commands failed: " + msg.err.Error()
			return m, nil
		}
		m.pluginNameForCommands = msg.pluginName
		m.pluginCommands = msg.commands
		m.status = fmt.Sprintf("plugin commands loaded: %d", len(msg.commands))
		return m, nil
	case pluginExecMsg:
		if msg.err != nil {
			m.status = "plugin execute failed: " + msg.err.Error()
			return m, nil
		}
		m.pluginResult = msg.out
		m.status = fmt.Sprintf("plugin %s:%s exit=%d", msg.out.PluginName, msg.out.CommandID, msg.out.ExitCode)
		return m, nil
	case pluginTTYReadyMsg:
		if msg.err != nil {
			m.status = "plugin tty prepare failed: " + msg.err.Error()
			return m, nil
		}
		if len(msg.plan.Argv) == 0 {
			m.status = "plugin tty prepare failed: empty argv"
			return m, nil
		}
		cmd := osexec.Command(msg.plan.Argv[0], msg.plan.Argv[1:]...)
		if msg.plan.Cwd != "" {
			cmd.Dir = msg.plan.Cwd
		}
		env := os.Environ()
		for key, value := range msg.plan.Env {
			env = append(env, key+"="+value)
		}
		if len(env) > 0 {
			cmd.Env = env
		}
		m.status = "plugin tty running"
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return pluginTTYDoneMsg{err: err}
		})
	case pluginTTYDoneMsg:
		if msg.err != nil {
			m.status = "plugin tty exited with error: " + msg.err.Error()
			return m, nil
		}
		m.status = "plugin tty completed"
		return m, nil
	case collabStatusMsg:
		if msg.err != nil {
			m.status = "collab status failed: " + msg.err.Error()
			return m, nil
		}
		m.collabStatus = msg.status
		return m, nil
	case collabPeersMsg:
		if msg.err != nil {
			m.status = "collab peers failed: " + msg.err.Error()
			return m, nil
		}
		m.collabPeers = msg.peers
		m.status = fmt.Sprintf("collab peers loaded: %d", len(msg.peers))
		return m, nil
	case collabReconcileMsg:
		if msg.err != nil {
			m.status = "collab reconcile failed: " + msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("collab reconciled ops=%d", msg.applied)
		return m, m.loadCollabStatusCmd()
	case tea.KeyMsg:
		if m.showPalette {
			return m.handlePaletteInput(msg)
		}
		return m.handleNormalInput(msg)
	}
	return m, nil
}

func (m Model) View() string {
	headline := theme.Title.Render("mdht") + " " + theme.Muted.Render("Internal Alpha")
	sub := theme.Muted.Render(fmt.Sprintf("vault: %s", m.vaultPath))

	if m.hasActive {
		headline += " " + theme.Hot.Render("[ACTIVE SESSION: "+m.activeSession.SourceTitle+"]")
	}

	paneWidth := max(25, (m.width-8)/3)
	paneHeight := max(10, m.height-9)
	libraryPane := m.renderLibraryPane(paneWidth, paneHeight)
	readerPane := m.renderReaderPane(paneWidth, paneHeight)
	contextPane := m.renderContextPane(paneWidth, paneHeight)
	row := lipgloss.JoinHorizontal(lipgloss.Top, libraryPane, readerPane, contextPane)

	palette := ""
	if m.showPalette {
		palette = "\n" + theme.Pane.BorderForeground(theme.Peach).Render("Command Palette\n:"+m.paletteInput)
	}

	collabStatus := "offline"
	if m.collabStatus.Online {
		collabStatus = "online"
	}
	lastSync := "-"
	if !m.collabStatus.LastSyncAt.IsZero() {
		lastSync = time.Since(m.collabStatus.LastSyncAt).Round(time.Second).String()
	}
	footer := "\n" + theme.Hot.Render("status: "+m.status) + "\n" + theme.Muted.Render(fmt.Sprintf("collab:%s peers:%d pending_ops:%d last_sync:%s", collabStatus, m.collabStatus.PeerCount, m.collabStatus.PendingOps, lastSync)) + "\n" + theme.Muted.Render("keys: up/down select, enter open, s start session, : palette, q quit")
	body := strings.Join([]string{headline, sub, row}, "\n") + palette + footer
	return theme.App.Render(body)
}

func (m Model) handleNormalInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up":
		if len(m.sources) > 0 {
			m.selectedIndex = max(0, m.selectedIndex-1)
			return m, m.loadSelectedDetailCmd()
		}
	case "down":
		if len(m.sources) > 0 {
			m.selectedIndex = min(len(m.sources)-1, m.selectedIndex+1)
			return m, m.loadSelectedDetailCmd()
		}
	case ":":
		m.showPalette = true
		m.paletteInput = ""
		return m, nil
	case "enter", "o":
		if selected, ok := m.selectedSourceID(); ok {
			return m, m.openSourceCmd(selected, "auto", max(1, m.readerResult.Page), false)
		}
	case "s":
		if selected, ok := m.selectedSourceID(); ok {
			title := m.detail.Title
			if title == "" && len(m.sources) > m.selectedIndex {
				title = m.sources[m.selectedIndex].Title
			}
			return m, m.startSessionCmd(selected, title)
		}
	}
	return m, nil
}

func (m Model) handlePaletteInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showPalette = false
		m.paletteInput = ""
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.paletteInput)
		m.showPalette = false
		m.paletteInput = ""
		return m.executePalette(input)
	case "backspace":
		if len(m.paletteInput) > 0 {
			m.paletteInput = m.paletteInput[:len(m.paletteInput)-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.paletteInput += msg.String()
		}
		return m, nil
	}
}

func (m Model) executePalette(input string) (tea.Model, tea.Cmd) {
	if input == "" {
		return m, nil
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	selected, hasSelected := m.selectedSourceID()

	switch parts[0] {
	case "session:start":
		if !hasSelected {
			m.status = "no source selected"
			return m, nil
		}
		title := m.detail.Title
		if title == "" {
			title = m.sources[m.selectedIndex].Title
		}
		return m, m.startSessionCmd(selected, title)
	case "session:end":
		if len(parts) < 3 {
			m.status = "usage: session:end <delta> <outcome>"
			return m, nil
		}
		delta, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			m.status = "invalid delta"
			return m, nil
		}
		outcome := strings.TrimSpace(strings.TrimPrefix(input, parts[0]+" "+parts[1]))
		return m, m.endSessionCmd(delta, outcome)
	case "reader:open":
		if !hasSelected {
			m.status = "no source selected"
			return m, nil
		}
		mode := "auto"
		if len(parts) >= 2 {
			mode = parts[1]
		}
		page := max(1, m.readerResult.Page)
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(parts[2]); err == nil {
				page = p
			}
		}
		return m, m.openSourceCmd(selected, mode, page, false)
	case "reader:next-page":
		if !hasSelected {
			m.status = "no source selected"
			return m, nil
		}
		page := max(1, m.readerResult.Page+1)
		return m, m.openSourceCmd(selected, "pdf", page, false)
	case "reader:prev-page":
		if !hasSelected {
			m.status = "no source selected"
			return m, nil
		}
		page := max(1, m.readerResult.Page-1)
		return m, m.openSourceCmd(selected, "pdf", page, false)
	case "source:open-external":
		if !hasSelected {
			m.status = "no source selected"
			return m, nil
		}
		return m, m.openSourceCmd(selected, "auto", 1, true)
	case "plugin:commands":
		if len(parts) < 2 {
			m.status = "usage: plugin:commands <plugin>"
			return m, nil
		}
		return m, m.listPluginCommandsCmd(parts[1])
	case "plugin:exec":
		if len(parts) < 3 {
			m.status = "usage: plugin:exec <plugin> <command> [input-json]"
			return m, nil
		}
		prefix := parts[0] + " " + parts[1] + " " + parts[2]
		inputJSON := strings.TrimSpace(strings.TrimPrefix(input, prefix))
		return m, m.execPluginCmd(false, plugindto.ExecuteInput{
			PluginName: parts[1],
			CommandID:  parts[2],
			InputJSON:  inputJSON,
			SourceID:   selected,
			SessionID:  m.activeSession.SessionID,
			VaultPath:  m.vaultPath,
			Cwd:        m.vaultPath,
		})
	case "plugin:analyze":
		if len(parts) < 4 {
			m.status = "usage: plugin:analyze <plugin> <command> <source-id> [input-json]"
			return m, nil
		}
		prefix := parts[0] + " " + parts[1] + " " + parts[2] + " " + parts[3]
		inputJSON := strings.TrimSpace(strings.TrimPrefix(input, prefix))
		return m, m.execPluginCmd(true, plugindto.ExecuteInput{
			PluginName: parts[1],
			CommandID:  parts[2],
			SourceID:   parts[3],
			InputJSON:  inputJSON,
			SessionID:  m.activeSession.SessionID,
			VaultPath:  m.vaultPath,
			Cwd:        m.vaultPath,
		})
	case "plugin:tty":
		if len(parts) < 3 {
			m.status = "usage: plugin:tty <plugin> <command> [input-json]"
			return m, nil
		}
		prefix := parts[0] + " " + parts[1] + " " + parts[2]
		inputJSON := strings.TrimSpace(strings.TrimPrefix(input, prefix))
		return m, m.preparePluginTTYCmd(plugindto.TTYPrepareInput{
			PluginName: parts[1],
			CommandID:  parts[2],
			InputJSON:  inputJSON,
			SourceID:   selected,
			SessionID:  m.activeSession.SessionID,
			VaultPath:  m.vaultPath,
			Cwd:        m.vaultPath,
		})
	case "collab:status":
		return m, m.loadCollabStatusCmd()
	case "collab:peers":
		return m, m.loadCollabPeersCmd()
	case "collab:reconcile":
		return m, m.reconcileCollabCmd()
	default:
		m.status = "unknown command"
		return m, nil
	}
}

func (m Model) selectedSourceID() (string, bool) {
	if len(m.sources) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.sources) {
		return "", false
	}
	return m.sources[m.selectedIndex].ID, true
}

func (m Model) loadSourcesCmd() tea.Cmd {
	return func() tea.Msg {
		sources, err := m.library.ListSources(context.Background())
		return sourcesLoadedMsg{sources: sources, err: err}
	}
}

func (m Model) loadActiveCmd() tea.Cmd {
	return func() tea.Msg {
		active, err := m.session.GetActive(context.Background())
		return activeLoadedMsg{active: active, err: err}
	}
}

func (m Model) loadSelectedDetailCmd() tea.Cmd {
	sourceID, ok := m.selectedSourceID()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		detail, err := m.library.GetSource(context.Background(), sourceID)
		return detailLoadedMsg{detail: detail, err: err}
	}
}

func (m Model) openSourceCmd(sourceID, mode string, page int, launchExternal bool) tea.Cmd {
	return func() tea.Msg {
		result, err := m.reader.OpenSource(context.Background(), sourceID, mode, page, launchExternal)
		return readerOpenedMsg{result: result, err: err}
	}
}

func (m Model) startSessionCmd(sourceID, sourceTitle string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.session.Start(context.Background(), sourceID, sourceTitle, "")
		if err != nil {
			return sessionStartedMsg{err: err}
		}
		return sessionStartedMsg{active: sessiondto.ActiveSessionOutput{SessionID: out.SessionID, SourceID: out.SourceID, SourceTitle: sourceTitle, StartedAt: out.StartedAt}}
	}
}

func (m Model) endSessionCmd(delta float64, outcome string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.session.End(context.Background(), "", outcome, delta)
		return sessionEndedMsg{out: out, err: err}
	}
}

func (m Model) listPluginCommandsCmd(pluginName string) tea.Cmd {
	return func() tea.Msg {
		if m.plugin == nil {
			return pluginCommandsMsg{err: fmt.Errorf("plugin adapter is not configured")}
		}
		commands, err := m.plugin.ListCommands(context.Background(), pluginName)
		return pluginCommandsMsg{pluginName: pluginName, commands: commands, err: err}
	}
}

func (m Model) execPluginCmd(analyze bool, input plugindto.ExecuteInput) tea.Cmd {
	return func() tea.Msg {
		if m.plugin == nil {
			return pluginExecMsg{err: fmt.Errorf("plugin adapter is not configured")}
		}
		var (
			out plugindto.ExecuteOutput
			err error
		)
		if analyze {
			out, err = m.plugin.Analyze(context.Background(), input)
		} else {
			out, err = m.plugin.Execute(context.Background(), input)
		}
		return pluginExecMsg{out: out, err: err}
	}
}

func (m Model) preparePluginTTYCmd(input plugindto.TTYPrepareInput) tea.Cmd {
	return func() tea.Msg {
		if m.plugin == nil {
			return pluginTTYReadyMsg{err: fmt.Errorf("plugin adapter is not configured")}
		}
		plan, err := m.plugin.PrepareTTY(context.Background(), input)
		return pluginTTYReadyMsg{plan: plan, err: err}
	}
}

func (m Model) loadCollabStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.collab == nil {
			return collabStatusMsg{}
		}
		status, err := m.collab.Status(context.Background())
		return collabStatusMsg{status: status, err: err}
	}
}

func (m Model) loadCollabPeersCmd() tea.Cmd {
	return func() tea.Msg {
		if m.collab == nil {
			return collabPeersMsg{}
		}
		peers, err := m.collab.PeerList(context.Background())
		return collabPeersMsg{peers: peers, err: err}
	}
}

func (m Model) reconcileCollabCmd() tea.Cmd {
	return func() tea.Msg {
		if m.collab == nil {
			return collabReconcileMsg{}
		}
		out, err := m.collab.ReconcileNow(context.Background())
		return collabReconcileMsg{applied: out.Applied, err: err}
	}
}

func (m Model) renderLibraryPane(width, height int) string {
	lines := []string{theme.Title.Render("Library")}
	if len(m.sources) == 0 {
		lines = append(lines, theme.Muted.Render("No sources yet"))
	} else {
		for i, source := range m.sources {
			prefix := "  "
			if i == m.selectedIndex {
				prefix = "> "
			}
			line := fmt.Sprintf("%s%s [%s] %.1f%%", prefix, source.Title, source.Type, source.Percent)
			lines = append(lines, line)
		}
	}
	return theme.PaneActive.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderReaderPane(width, height int) string {
	lines := []string{theme.Title.Render("Reader")}
	if m.readerResult.SourceID == "" {
		lines = append(lines, theme.Muted.Render("Open a source to render content"))
	} else {
		lines = append(lines,
			fmt.Sprintf("%s [%s] mode=%s", m.readerResult.Title, m.readerResult.Type, m.readerResult.Mode),
			fmt.Sprintf("progress: %.1f%%", m.readerResult.Percent),
		)
		if m.readerResult.Mode == "pdf" {
			lines = append(lines, fmt.Sprintf("page %d/%d", m.readerResult.Page, m.readerResult.TotalPage))
		}
		if m.readerResult.ExternalTarget != "" {
			lines = append(lines, "target: "+m.readerResult.ExternalTarget)
		}
		lines = append(lines, "", trimText(m.readerResult.Content, 320))
	}
	if m.pluginResult.PluginName != "" {
		lines = append(lines,
			"",
			theme.Title.Render("Plugin Output"),
			fmt.Sprintf("%s:%s exit=%d", m.pluginResult.PluginName, m.pluginResult.CommandID, m.pluginResult.ExitCode),
		)
		if m.pluginResult.Stdout != "" {
			lines = append(lines, trimText(m.pluginResult.Stdout, 220))
		}
		if m.pluginResult.OutputJSON != "" {
			lines = append(lines, trimText(m.pluginResult.OutputJSON, 220))
		}
	}
	return theme.Pane.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderContextPane(width, height int) string {
	lines := []string{theme.Title.Render("Context")}
	if m.detail.ID == "" {
		lines = append(lines, theme.Muted.Render("Select a source"))
	} else {
		lines = append(lines,
			"id: "+m.detail.ID,
			"note: "+m.detail.NotePath,
		)
		if m.detail.FilePath != "" {
			lines = append(lines, "file: "+m.detail.FilePath)
		}
		if m.detail.URL != "" {
			lines = append(lines, "url: "+m.detail.URL)
		}
		if len(m.detail.Topics) > 0 {
			lines = append(lines, "topics: "+strings.Join(m.detail.Topics, ", "))
		}
	}
	if m.hasActive {
		lines = append(lines,
			"",
			theme.Hot.Render("Active Session"),
			"session: "+m.activeSession.SessionID,
			"source: "+m.activeSession.SourceTitle,
		)
	}
	if m.pluginNameForCommands != "" {
		lines = append(lines, "", theme.Title.Render("Plugin Commands: "+m.pluginNameForCommands))
		if len(m.pluginCommands) == 0 {
			lines = append(lines, theme.Muted.Render("No commands discovered"))
		}
		for _, item := range m.pluginCommands {
			lines = append(lines, fmt.Sprintf("- %s (%s)", item.ID, item.Kind))
		}
	}
	if len(m.collabPeers) > 0 {
		lines = append(lines, "", theme.Title.Render("Collab Peers"))
		for _, item := range m.collabPeers {
			lines = append(lines, fmt.Sprintf("- %s", item.PeerID))
		}
	}
	return theme.Pane.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func trimText(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
