package app

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	collabdto "mdht/internal/modules/collab/dto"
	"mdht/internal/modules/library/dto"
	plugindto "mdht/internal/modules/plugin/dto"
	readerdto "mdht/internal/modules/reader/dto"
	sessiondto "mdht/internal/modules/session/dto"
	apperrors "mdht/internal/platform/errors"
	"mdht/internal/ui/components"
	"mdht/internal/ui/theme"
	collabview "mdht/internal/ui/views/collab"
	graphview "mdht/internal/ui/views/graph"
	libraryview "mdht/internal/ui/views/library"
	pluginsview "mdht/internal/ui/views/plugins"
	readerview "mdht/internal/ui/views/reader"
)

// ─── ports ───────────────────────────────────────────────────────────────────
// Each port is the minimal interface that this orchestration layer requires.
// Sub-view ports are defined in their own packages and narrowed further.

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
	PrepareTTY(ctx context.Context, input plugindto.TTYPrepareInput) (plugindto.TTYPrepareOutput, error)
}

type collabPort interface {
	Status(ctx context.Context) (collabdto.StatusOutput, error)
	NetStatus(ctx context.Context) (collabdto.NetStatusOutput, error)
	NetProbe(ctx context.Context) (collabdto.NetProbeOutput, error)
	PeerList(ctx context.Context) ([]collabdto.PeerOutput, error)
	PeerDial(ctx context.Context, peerID string) (collabdto.PeerOutput, error)
	PeerLatency(ctx context.Context) ([]collabdto.PeerLatencyOutput, error)
	ActivityTail(ctx context.Context, since time.Time, limit int) ([]collabdto.ActivityOutput, error)
	ConflictsList(ctx context.Context, entityKey string) ([]collabdto.ConflictOutput, error)
	ConflictResolve(ctx context.Context, conflictID, strategy string) (collabdto.ConflictOutput, error)
	SyncNow(ctx context.Context) (collabdto.ReconcileOutput, error)
	SyncHealth(ctx context.Context) (collabdto.SyncHealthOutput, error)
}

// ─── tab index ───────────────────────────────────────────────────────────────

type tabID int

const (
	tabLibrary tabID = iota
	tabReader
	tabGraph
	tabCollab
	tabPlugins
	tabCount
)

var tabLabels = [tabCount]string{
	"Library", "Reader", "Graph", "Collab", "Plugins",
}

// ─── async messages ───────────────────────────────────────────────────────────

type activeLoadedMsg struct {
	active sessiondto.ActiveSessionOutput
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

type pluginTTYReadyMsg struct {
	plan plugindto.TTYPrepareOutput
	err  error
}

type pluginTTYDoneMsg struct{ err error }

// ─── key bindings ─────────────────────────────────────────────────────────────

type keyMap struct {
	Tab     key.Binding
	Help    key.Binding
	Palette key.Binding
	Quit    key.Binding
	Enter   key.Binding
	Session key.Binding
	PrevPg  key.Binding
	NextPg  key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Palette: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "palette")),
		Quit:    key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q", "quit")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Session: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start session")),
		PrevPg:  key.NewBinding(key.WithKeys("left"), key.WithHelp("←/→", "pdf page")),
		NextPg:  key.NewBinding(key.WithKeys("right"), key.WithHelp("←/→", "pdf page")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Help, k.Palette, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Enter, k.Session},
		{k.PrevPg, k.NextPg},
		{k.Help, k.Palette, k.Quit},
	}
}

// ─── model ───────────────────────────────────────────────────────────────────

// Model is the root Bubble Tea model. It owns tab routing, session state,
// the global help overlay, and the command palette. All business logic is
// delegated to port interfaces; all rendering is delegated to sub-views.
type Model struct {
	vaultPath string

	// ports used at this orchestration level only
	session sessionPort
	plugin  pluginPort
	collab  collabPort

	// sub-views (one per tab)
	libView    libraryview.Model
	readView   readerview.Model
	graphView  graphview.Model
	collabView collabview.Model
	pluginView pluginsview.Model

	// global UI state
	activeTab     tabID
	keys          keyMap
	help          help.Model
	showHelp      bool
	palette       components.Palette
	activeSession sessiondto.ActiveSessionOutput
	hasActive     bool
	status        string
	width         int
	height        int
}

// ─── constructor ─────────────────────────────────────────────────────────────

func NewModel(
	vaultPath string,
	library libraryPort,
	session sessionPort,
	reader readerPort,
	plugin pluginPort,
	collab collabPort,
	graph graphview.GraphPort,
) Model {
	var collabV collabview.Model
	if collab != nil {
		collabV = collabview.New(collabPortBridge{p: collab})
	} else {
		collabV = collabview.New(nil)
	}

	var graphV graphview.Model
	if graph != nil {
		graphV = graphview.New(graph)
	} else {
		graphV = graphview.New(nil)
	}

	var pluginV pluginsview.Model
	if plugin != nil {
		pluginV = pluginsview.New(pluginPortBridge{p: plugin}, vaultPath)
	} else {
		pluginV = pluginsview.New(nil, vaultPath)
	}

	return Model{
		vaultPath:  vaultPath,
		session:    session,
		plugin:     plugin,
		collab:     collab,
		libView:    libraryview.New(libraryPortBridge{p: library}),
		readView:   readerview.New(readerPortBridge{p: reader}),
		graphView:  graphV,
		collabView: collabV,
		pluginView: pluginV,
		activeTab:  tabLibrary,
		keys:       defaultKeys(),
		help:       help.New(),
		palette:    components.NewPalette(),
		status:     "ready",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.libView.Init(),
		m.graphView.Init(),
		m.collabView.Init(),
		m.loadActiveCmd(),
	)
}

// ─── update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// The palette intercepts all input while open.
	if m.palette.Visible() {
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.palette.SetWidth(min(m.width-4, 80))
		m.help.Width = m.width
		m.propagateSize()

	case activeLoadedMsg:
		if msg.err != nil {
			if msg.err != apperrors.ErrNoActiveSession {
				m.status = "active session check: " + msg.err.Error()
			}
			m.hasActive = false
		} else {
			m.hasActive = true
			m.activeSession = msg.active
			m.status = "session recovered: " + msg.active.SourceTitle
			m.pluginView.SetContext(msg.active.SourceID, msg.active.SessionID)
		}

	case sessionStartedMsg:
		if msg.err != nil {
			m.status = "session start failed: " + msg.err.Error()
		} else {
			m.hasActive = true
			m.activeSession = msg.active
			m.status = "session started: " + msg.active.SourceTitle
			m.pluginView.SetContext(msg.active.SourceID, msg.active.SessionID)
		}

	case sessionEndedMsg:
		if msg.err != nil {
			m.status = "session end failed: " + msg.err.Error()
		} else {
			m.hasActive = false
			m.activeSession = sessiondto.ActiveSessionOutput{}
			m.status = fmt.Sprintf("session ended (+%.2f%%)", msg.out.DeltaProgress)
			m.pluginView.SetContext("", "")
		}

	case pluginTTYReadyMsg:
		if msg.err != nil {
			m.status = "plugin tty prepare: " + msg.err.Error()
			return m, nil
		}
		if len(msg.plan.Argv) == 0 {
			m.status = "plugin tty: empty argv"
			return m, nil
		}
		cmd := osexec.Command(msg.plan.Argv[0], msg.plan.Argv[1:]...)
		if msg.plan.Cwd != "" {
			cmd.Dir = msg.plan.Cwd
		}
		env := os.Environ()
		for k, v := range msg.plan.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
		m.status = "plugin tty running"
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return pluginTTYDoneMsg{err: err}
		})

	case pluginTTYDoneMsg:
		if msg.err != nil {
			m.status = "plugin tty error: " + msg.err.Error()
		} else {
			m.status = "plugin tty completed"
		}

	case components.PaletteSubmitMsg:
		return m.executePalette(msg.Input)

	case components.PaletteCancelMsg:
		m.status = "ready"

	// OpenedMsg is produced by the reader view but bubbles up through the top
	// level so we can auto-switch to the Reader tab and update status.
	case readerview.OpenedMsg:
		if msg.Err != nil {
			m.status = "reader: " + msg.Err.Error()
		} else {
			m.status = fmt.Sprintf("reader: %s [%s]", msg.Result.Title, msg.Result.Mode)
			m.activeTab = tabReader
			// Keep plugin context up-to-date with the selected source.
			if m.activeSession.SessionID == "" {
				m.pluginView.SetContext(msg.Result.SourceID, "")
			}
		}
		var cmd tea.Cmd
		m.readView, cmd = m.readView.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" {
				m.showHelp = false
			}
			return m, nil
		}

		// Yield to sub-view when its search filter is active.
		if m.subViewFiltering() {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % tabCount
		case "shift+tab":
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
		case "?":
			m.showHelp = !m.showHelp
		case ":":
			cmds = append(cmds, m.palette.Open())
			return m, tea.Batch(cmds...)
		case "s":
			if m.activeTab == tabLibrary {
				if id, ok := m.libView.SelectedSourceID(); ok {
					cmds = append(cmds, m.startSessionCmd(id, m.libView.SelectedSourceTitle()))
				}
			}
		case "enter":
			if m.activeTab == tabLibrary {
				if id, ok := m.libView.SelectedSourceID(); ok {
					cmds = append(cmds, m.readView.OpenSource(id, "auto", 1, false))
				}
			}
		case "left":
			if m.activeTab == tabReader {
				cmds = append(cmds, m.readView.PrevPage())
			}
		case "right":
			if m.activeTab == tabReader {
				cmds = append(cmds, m.readView.NextPage())
			}
		}
	}

	// Propagate the message to the active tab's sub-view.
	var tabCmd tea.Cmd
	switch m.activeTab {
	case tabLibrary:
		m.libView, tabCmd = m.libView.Update(msg)
	case tabReader:
		m.readView, tabCmd = m.readView.Update(msg)
	case tabGraph:
		m.graphView, tabCmd = m.graphView.Update(msg)
	case tabCollab:
		m.collabView, tabCmd = m.collabView.Update(msg)
	case tabPlugins:
		m.pluginView, tabCmd = m.pluginView.Update(msg)
	}
	cmds = append(cmds, tabCmd)

	return m, tea.Batch(cmds...)
}

// ─── view ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	tabBar := m.renderTabBar()
	statusBar := m.renderStatusBar()
	tabBarH := lipgloss.Height(tabBar)
	statusBarH := lipgloss.Height(statusBar)

	contentH := m.height - tabBarH - statusBarH
	if contentH < 1 {
		contentH = 1
	}

	var content string
	switch {
	case m.showHelp:
		content = lipgloss.NewStyle().Width(m.width).Height(contentH).
			Render(m.help.View(m.keys))
	case m.palette.Visible():
		content = lipgloss.Place(m.width, contentH,
			lipgloss.Center, lipgloss.Center, m.palette.View())
	default:
		content = m.activeView()
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content, statusBar)
}

func (m Model) activeView() string {
	switch m.activeTab {
	case tabLibrary:
		return m.libView.View()
	case tabReader:
		return m.readView.View()
	case tabGraph:
		return m.graphView.View()
	case tabCollab:
		return m.collabView.View()
	case tabPlugins:
		return m.pluginView.View()
	}
	return ""
}

func (m Model) renderTabBar() string {
	parts := make([]string, tabCount)
	for i := tabID(0); i < tabCount; i++ {
		label := tabLabels[i]
		if i == m.activeTab {
			parts[i] = theme.Hot.Render(" " + label + " ")
		} else {
			parts[i] = theme.Muted.Render(" " + label + " ")
		}
	}
	sep := theme.Muted.Render(" │ ")
	bar := "mdht  " + strings.Join(parts, sep)
	return lipgloss.NewStyle().Background(theme.Mantle).Width(m.width).Render(bar) + "\n"
}

func (m Model) renderStatusBar() string {
	left := m.status
	if m.hasActive {
		left = theme.Hot.Render("● "+m.activeSession.SourceTitle) + "  " + left
	}
	right := theme.Muted.Render("?:help  tab:switch  :::palette  q:quit")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return "\n" + lipgloss.NewStyle().Background(theme.Mantle).Width(m.width).Render(bar)
}

// ─── palette execution ────────────────────────────────────────────────────────

func (m Model) executePalette(input string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(input) == "" {
		return m, nil
	}
	parts := strings.Fields(input)
	selected, _ := m.libView.SelectedSourceID()

	switch parts[0] {
	case "session:start":
		if selected == "" {
			m.status = "no source selected"
			return m, nil
		}
		return m, m.startSessionCmd(selected, m.libView.SelectedSourceTitle())

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
		outcome := strings.TrimSpace(strings.TrimPrefix(input, parts[0]+" "+parts[1]+" "))
		return m, m.endSessionCmd(delta, outcome)

	case "reader:open":
		if selected == "" {
			m.status = "no source selected"
			return m, nil
		}
		mode := "auto"
		if len(parts) >= 2 {
			mode = parts[1]
		}
		page := 1
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(parts[2]); err == nil {
				page = p
			}
		}
		return m, m.readView.OpenSource(selected, mode, page, false)

	case "reader:next-page":
		return m, m.readView.NextPage()

	case "reader:prev-page":
		return m, m.readView.PrevPage()

	case "source:open-external":
		if selected == "" {
			m.status = "no source selected"
			return m, nil
		}
		return m, m.readView.OpenSource(selected, "auto", 1, true)

	case "plugin:exec":
		if len(parts) < 3 {
			m.status = "usage: plugin:exec <plugin> <command> [json]"
			return m, nil
		}
		prefix := parts[0] + " " + parts[1] + " " + parts[2]
		inputJSON := strings.TrimSpace(strings.TrimPrefix(input, prefix))
		m.activeTab = tabPlugins
		return m, m.pluginView.ExecCommand(parts[1], parts[2], inputJSON)

	case "plugin:commands":
		m.activeTab = tabPlugins
		m.status = "switched to Plugins tab"
		return m, nil

	case "plugin:tty":
		if len(parts) < 3 {
			m.status = "usage: plugin:tty <plugin> <command> [json]"
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

	case "collab:status", "collab:peers", "collab:activity",
		"collab:conflicts", "collab:sync-now", "collab:sync-health":
		m.activeTab = tabCollab
		m.status = "switched to Collab tab"
		return m, nil

	case "collab:resolve":
		if len(parts) < 3 {
			m.status = "usage: collab:resolve <id> <local|remote|merge>"
			return m, nil
		}
		m.activeTab = tabCollab
		return m, m.resolveConflictCmd(parts[1], parts[2])

	default:
		m.status = "unknown command: " + parts[0]
	}
	return m, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// subViewFiltering reports whether the active tab's list filter is open,
// in which case global key bindings must yield to allow free typing.
func (m Model) subViewFiltering() bool {
	switch m.activeTab {
	case tabLibrary:
		return m.libView.Filtering()
	case tabGraph:
		return m.graphView.Filtering()
	case tabCollab:
		return m.collabView.Filtering()
	case tabPlugins:
		return m.pluginView.Filtering()
	}
	return false
}

func (m *Model) propagateSize() {
	sz := tea.WindowSizeMsg{Width: m.width, Height: m.height - 3}
	m.libView, _ = m.libView.Update(sz)
	m.readView, _ = m.readView.Update(sz)
	m.graphView, _ = m.graphView.Update(sz)
	m.collabView, _ = m.collabView.Update(sz)
	m.pluginView, _ = m.pluginView.Update(sz)
}

// ─── async commands ───────────────────────────────────────────────────────────

func (m Model) loadActiveCmd() tea.Cmd {
	return func() tea.Msg {
		active, err := m.session.GetActive(context.Background())
		return activeLoadedMsg{active: active, err: err}
	}
}

func (m Model) startSessionCmd(sourceID, title string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.session.Start(context.Background(), sourceID, title, "")
		if err != nil {
			return sessionStartedMsg{err: err}
		}
		return sessionStartedMsg{active: sessiondto.ActiveSessionOutput{
			SessionID:   out.SessionID,
			SourceID:    out.SourceID,
			SourceTitle: title,
			StartedAt:   out.StartedAt,
		}}
	}
}

func (m Model) endSessionCmd(delta float64, outcome string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.session.End(context.Background(), "", outcome, delta)
		return sessionEndedMsg{out: out, err: err}
	}
}

func (m Model) preparePluginTTYCmd(input plugindto.TTYPrepareInput) tea.Cmd {
	return func() tea.Msg {
		if m.plugin == nil {
			return pluginTTYReadyMsg{err: fmt.Errorf("plugin adapter not configured")}
		}
		plan, err := m.plugin.PrepareTTY(context.Background(), input)
		return pluginTTYReadyMsg{plan: plan, err: err}
	}
}

func (m Model) resolveConflictCmd(conflictID, strategy string) tea.Cmd {
	return func() tea.Msg {
		if m.collab == nil {
			return collabview.ResolveMsg{}
		}
		item, err := m.collab.ConflictResolve(context.Background(), conflictID, strategy)
		return collabview.ResolveMsg{Item: item, Err: err}
	}
}

// ─── port bridges ─────────────────────────────────────────────────────────────
// Each bridge narrows a broad port interface to the minimal interface needed by
// a specific sub-view, keeping view packages free of knowledge about the wider
// port surface.

type libraryPortBridge struct{ p libraryPort }

func (b libraryPortBridge) ListSources(ctx context.Context) ([]dto.SourceOutput, error) {
	return b.p.ListSources(ctx)
}
func (b libraryPortBridge) GetSource(ctx context.Context, id string) (dto.SourceDetailOutput, error) {
	return b.p.GetSource(ctx, id)
}

type readerPortBridge struct{ p readerPort }

func (b readerPortBridge) OpenSource(ctx context.Context, id, mode string, page int, ext bool) (readerdto.OpenResult, error) {
	return b.p.OpenSource(ctx, id, mode, page, ext)
}

type collabPortBridge struct{ p collabPort }

func (b collabPortBridge) Status(ctx context.Context) (collabdto.StatusOutput, error) {
	return b.p.Status(ctx)
}
func (b collabPortBridge) PeerList(ctx context.Context) ([]collabdto.PeerOutput, error) {
	return b.p.PeerList(ctx)
}
func (b collabPortBridge) ActivityTail(ctx context.Context, since time.Time, limit int) ([]collabdto.ActivityOutput, error) {
	return b.p.ActivityTail(ctx, since, limit)
}
func (b collabPortBridge) ConflictsList(ctx context.Context, key string) ([]collabdto.ConflictOutput, error) {
	return b.p.ConflictsList(ctx, key)
}
func (b collabPortBridge) ConflictResolve(ctx context.Context, id, strategy string) (collabdto.ConflictOutput, error) {
	return b.p.ConflictResolve(ctx, id, strategy)
}
func (b collabPortBridge) SyncNow(ctx context.Context) (collabdto.ReconcileOutput, error) {
	return b.p.SyncNow(ctx)
}
func (b collabPortBridge) SyncHealth(ctx context.Context) (collabdto.SyncHealthOutput, error) {
	return b.p.SyncHealth(ctx)
}

type pluginPortBridge struct{ p pluginPort }

func (b pluginPortBridge) ListCommands(ctx context.Context, name string) ([]plugindto.CommandInfo, error) {
	return b.p.ListCommands(ctx, name)
}
func (b pluginPortBridge) Execute(ctx context.Context, input plugindto.ExecuteInput) (plugindto.ExecuteOutput, error) {
	return b.p.Execute(ctx, input)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
