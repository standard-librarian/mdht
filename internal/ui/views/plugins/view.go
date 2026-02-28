package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	plugindto "mdht/internal/modules/plugin/dto"
	"mdht/internal/ui/theme"
)

// ─── port ────────────────────────────────────────────────────────────────────

// Port is the minimal interface this view needs from the plugin use-case.
type Port interface {
	ListCommands(ctx context.Context, pluginName string) ([]plugindto.CommandInfo, error)
	Execute(ctx context.Context, input plugindto.ExecuteInput) (plugindto.ExecuteOutput, error)
}

// ─── messages ────────────────────────────────────────────────────────────────

// CommandsLoadedMsg is sent when plugin commands finish loading.
type CommandsLoadedMsg struct {
	PluginName string
	Commands   []plugindto.CommandInfo
	Err        error
}

// ExecDoneMsg is sent when a plugin command finishes executing.
type ExecDoneMsg struct {
	Out plugindto.ExecuteOutput
	Err error
}

// ─── list item ───────────────────────────────────────────────────────────────

type commandItem struct{ cmd plugindto.CommandInfo }

func (i commandItem) Title() string       { return i.cmd.Title }
func (i commandItem) Description() string { return "[" + i.cmd.Kind + "] " + i.cmd.Description }
func (i commandItem) FilterValue() string { return i.cmd.ID + " " + i.cmd.Title }

// ─── pane ────────────────────────────────────────────────────────────────────

type pane int

const (
	paneInput    pane = iota // user types plugin name
	paneCommands             // user picks a command
	paneOutput               // result is displayed
)

// ─── model ───────────────────────────────────────────────────────────────────

// Model is the self-contained Bubble Tea model for the Plugins tab.
type Model struct {
	port      Port
	pane      pane
	nameInput textinput.Model
	cmdList   list.Model
	output    viewport.Model
	spinner   spinner.Model
	commands  []plugindto.CommandInfo
	lastOut   plugindto.ExecuteOutput
	loading   bool
	// context wired from the parent model
	vaultPath string
	sourceID  string
	sessionID string
	width     int
	height    int
}

// New creates a Plugins Model. vaultPath is used for plugin execution context.
func New(port Port, vaultPath string) Model {
	ti := textinput.New()
	ti.Placeholder = "plugin name (e.g. my-plugin)"
	ti.Focus()
	ti.CharLimit = 80

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.Lavender).BorderForeground(theme.Lavender)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.Sapphire).BorderForeground(theme.Lavender)

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Commands"
	l.Styles.Title = theme.Title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Background(theme.Mantle).
		Foreground(theme.Text).
		Padding(1)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Lavender)

	return Model{
		port:      port,
		pane:      paneInput,
		nameInput: ti,
		cmdList:   l,
		output:    vp,
		spinner:   sp,
		vaultPath: vaultPath,
	}
}

// SetContext updates the plugin execution context with the currently-selected
// source and active session. Called by the parent model whenever those change.
func (m *Model) SetContext(sourceID, sessionID string) {
	m.sourceID = sourceID
	m.sessionID = sessionID
}

// Filtering reports whether the command list's search filter is active.
func (m Model) Filtering() bool {
	return m.cmdList.FilterState() == list.Filtering
}

// ExecCommand triggers an execution of the named command without going through
// the interactive pane flow — used by the command palette.
func (m *Model) ExecCommand(pluginName, commandID, inputJSON string) tea.Cmd {
	m.loading = true
	return tea.Batch(m.execNamedCmd(pluginName, commandID, inputJSON), m.spinner.Tick)
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case CommandsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.output.SetContent(theme.Hot.Render("Error loading commands: " + msg.Err.Error()))
			m.pane = paneOutput
			return m, nil
		}
		m.commands = msg.Commands
		items := make([]list.Item, len(msg.Commands))
		for i, c := range msg.Commands {
			items[i] = commandItem{cmd: c}
		}
		cmds = append(cmds, m.cmdList.SetItems(items))
		m.pane = paneCommands

	case ExecDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.output.SetContent(theme.Hot.Render("Error: " + msg.Err.Error()))
		} else {
			m.lastOut = msg.Out
			m.output.SetContent(m.renderOutput())
		}
		m.output.GotoTop()
		m.pane = paneOutput

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch m.pane {
		case paneInput:
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.nameInput.Value())
				if name != "" && m.port != nil {
					m.loading = true
					cmds = append(cmds, m.loadCommandsCmd(name), m.spinner.Tick)
				}
			default:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				cmds = append(cmds, cmd)
			}

		case paneCommands:
			switch msg.String() {
			case "enter":
				if item, ok := m.cmdList.SelectedItem().(commandItem); ok && m.port != nil {
					m.loading = true
					cmds = append(cmds,
						m.execNamedCmd(strings.TrimSpace(m.nameInput.Value()), item.cmd.ID, ""),
						m.spinner.Tick,
					)
				}
			case "esc":
				m.pane = paneInput
				m.nameInput.Focus()
			default:
				var lCmd tea.Cmd
				m.cmdList, lCmd = m.cmdList.Update(msg)
				cmds = append(cmds, lCmd)
			}

		case paneOutput:
			switch msg.String() {
			case "esc":
				m.pane = paneCommands
			default:
				var vCmd tea.Cmd
				m.output, vCmd = m.output.Update(msg)
				cmds = append(cmds, vCmd)
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Non-key messages pass through to the active pane component.
	switch m.pane {
	case paneInput:
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case paneCommands:
		var lCmd tea.Cmd
		m.cmdList, lCmd = m.cmdList.Update(msg)
		cmds = append(cmds, lCmd)
	case paneOutput:
		var vCmd tea.Cmd
		m.output, vCmd = m.output.Update(msg)
		cmds = append(cmds, vCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Working…")
	}

	header := m.renderHeader()
	headerH := lipgloss.Height(header)
	bodyH := m.height - headerH
	if bodyH < 1 {
		bodyH = 1
	}

	var body string
	switch m.pane {
	case paneInput:
		hint := theme.Muted.Render("Enter a plugin name and press enter to list its commands.\n\n")
		input := lipgloss.NewStyle().Width(m.width - 4).Render(m.nameInput.View())
		body = lipgloss.Place(m.width, bodyH, lipgloss.Left, lipgloss.Center, hint+input)

	case paneCommands:
		listW := m.width * 4 / 10
		detailW := m.width - listW
		listPane := lipgloss.NewStyle().Width(listW).Height(bodyH).Render(m.cmdList.View())
		hint := theme.Muted.Render("enter: execute  esc: back to plugin name")
		detailPane := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).BorderForeground(theme.Surface1).
			Background(theme.Mantle).Width(detailW - 2).Height(bodyH - 2).
			Render(hint)
		body = lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	case paneOutput:
		hint := theme.Muted.Render("esc: back to commands  ↑/↓: scroll\n")
		hintH := lipgloss.Height(hint)
		m.output.Height = bodyH - hintH
		if m.output.Height < 1 {
			m.output.Height = 1
		}
		body = lipgloss.JoinVertical(lipgloss.Left, hint, m.output.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// ─── private ─────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	m.cmdList.SetSize(m.width*4/10, m.height-3)
	m.output.Width = m.width - 4
	m.output.Height = m.height - 4
}

func (m Model) renderHeader() string {
	name := m.nameInput.Value()
	if name == "" {
		name = "(none)"
	}
	return theme.Title.Render("Plugins") + "  " +
		theme.Muted.Render("plugin: "+name) + "\n"
}

func (m Model) renderOutput() string {
	out := m.lastOut
	var sb strings.Builder
	sb.WriteString(theme.Title.Render(
		fmt.Sprintf("%s:%s  exit=%d", out.PluginName, out.CommandID, out.ExitCode),
	) + "\n\n")
	if out.Stdout != "" {
		sb.WriteString(theme.Muted.Render("stdout:\n") + out.Stdout + "\n")
	}
	if out.Stderr != "" {
		sb.WriteString(theme.Hot.Render("stderr:\n") + out.Stderr + "\n")
	}
	if out.OutputJSON != "" {
		sb.WriteString(theme.Muted.Render("output JSON:\n") + out.OutputJSON + "\n")
	}
	return sb.String()
}

func (m Model) loadCommandsCmd(pluginName string) tea.Cmd {
	return func() tea.Msg {
		cmds, err := m.port.ListCommands(context.Background(), pluginName)
		return CommandsLoadedMsg{PluginName: pluginName, Commands: cmds, Err: err}
	}
}

func (m Model) execNamedCmd(pluginName, commandID, inputJSON string) tea.Cmd {
	input := plugindto.ExecuteInput{
		PluginName: pluginName,
		CommandID:  commandID,
		InputJSON:  inputJSON,
		SourceID:   m.sourceID,
		SessionID:  m.sessionID,
		VaultPath:  m.vaultPath,
		Cwd:        m.vaultPath,
	}
	return func() tea.Msg {
		out, err := m.port.Execute(context.Background(), input)
		return ExecDoneMsg{Out: out, Err: err}
	}
}
