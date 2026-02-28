package library

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	libdto "mdht/internal/modules/library/dto"
	"mdht/internal/ui/theme"
)

// ─── port ────────────────────────────────────────────────────────────────────

type LibraryPort interface {
	ListSources(ctx context.Context) ([]libdto.SourceOutput, error)
	GetSource(ctx context.Context, id string) (libdto.SourceDetailOutput, error)
}

// ─── messages ────────────────────────────────────────────────────────────────

type SourcesLoadedMsg struct {
	Sources []libdto.SourceOutput
	Err     error
}

type DetailLoadedMsg struct {
	Detail libdto.SourceDetailOutput
	Err    error
}

type OpenReaderMsg struct {
	SourceID string
}

// ─── list item ───────────────────────────────────────────────────────────────

type sourceItem struct {
	source libdto.SourceOutput
}

func (i sourceItem) Title() string       { return i.source.Title }
func (i sourceItem) Description() string { return fmt.Sprintf("%s  %.0f%%", i.source.Type, i.source.Percent) }
func (i sourceItem) FilterValue() string { return i.source.Title }

// ─── model ───────────────────────────────────────────────────────────────────

type Model struct {
	port    LibraryPort
	list    list.Model
	detail  libdto.SourceDetailOutput
	preview viewport.Model
	spinner spinner.Model
	loading bool
	width   int
	height  int
}

func New(port LibraryPort) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(theme.Lavender).BorderForeground(theme.Lavender)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(theme.Sapphire).BorderForeground(theme.Lavender)

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Library"
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
		port:    port,
		list:    l,
		preview: vp,
		spinner: sp,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSourcesCmd(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case SourcesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.list.Title = "Library — " + msg.Err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.Sources))
		for i, s := range msg.Sources {
			items[i] = sourceItem{source: s}
		}
		cmds = append(cmds, m.list.SetItems(items))
		if len(msg.Sources) > 0 {
			cmds = append(cmds, m.loadDetailCmd(msg.Sources[0].ID))
		}

	case DetailLoadedMsg:
		if msg.Err == nil {
			m.detail = msg.Detail
			m.preview.SetContent(m.renderDetail())
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !m.loading {
		var lCmd tea.Cmd
		prevIdx := m.list.Index()
		m.list, lCmd = m.list.Update(msg)
		cmds = append(cmds, lCmd)
		if m.list.Index() != prevIdx {
			if item, ok := m.list.SelectedItem().(sourceItem); ok {
				cmds = append(cmds, m.loadDetailCmd(item.source.ID))
			}
		}

		var vCmd tea.Cmd
		m.preview, vCmd = m.preview.Update(msg)
		cmds = append(cmds, vCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading library…")
	}

	listW := m.width * 4 / 10
	detailW := m.width - listW

	listPane := lipgloss.NewStyle().
		Width(listW).
		Height(m.height).
		Render(m.list.View())

	detailPane := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Surface1).
		Background(theme.Mantle).
		Width(detailW - 2).
		Height(m.height - 2).
		Render(m.preview.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

// SelectedSourceID returns the current selection's source ID, if any.
func (m Model) SelectedSourceID() (string, bool) {
	if item, ok := m.list.SelectedItem().(sourceItem); ok {
		return item.source.ID, true
	}
	return "", false
}

// Filtering reports whether the list's search filter is currently active.
// The app model checks this to avoid consuming global keys during a search.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// SelectedSourceTitle returns the current selection's title.
func (m Model) SelectedSourceTitle() string {
	if item, ok := m.list.SelectedItem().(sourceItem); ok {
		return item.source.Title
	}
	return ""
}

// ─── private ─────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	listW := m.width * 4 / 10
	detailW := m.width - listW
	m.list.SetSize(listW, m.height)
	m.preview.Width = detailW - 4
	m.preview.Height = m.height - 4
}

func (m Model) renderDetail() string {
	d := m.detail
	if d.ID == "" {
		return theme.Muted.Render("Select a source to see details")
	}
	var sb strings.Builder
	sb.WriteString(theme.Title.Render(d.Title) + "\n\n")
	sb.WriteString(theme.Muted.Render("id:     ") + d.ID + "\n")
	sb.WriteString(theme.Muted.Render("type:   ") + d.Type + "\n")
	sb.WriteString(theme.Muted.Render("status: ") + d.Status + "\n")
	sb.WriteString(fmt.Sprintf("%s%.1f%%\n", theme.Muted.Render("prog:   "), d.Percent))
	if d.FilePath != "" {
		sb.WriteString(theme.Muted.Render("file:   ") + d.FilePath + "\n")
	}
	if d.URL != "" {
		sb.WriteString(theme.Muted.Render("url:    ") + d.URL + "\n")
	}
	if d.NotePath != "" {
		sb.WriteString(theme.Muted.Render("note:   ") + d.NotePath + "\n")
	}
	if len(d.Topics) > 0 {
		sb.WriteString(theme.Muted.Render("topics: ") + strings.Join(d.Topics, ", ") + "\n")
	}
	if len(d.Tags) > 0 {
		sb.WriteString(theme.Muted.Render("tags:   ") + strings.Join(d.Tags, ", ") + "\n")
	}
	if d.UnitKind != "" {
		sb.WriteString(fmt.Sprintf("%s%d / %d %s\n",
			theme.Muted.Render("units:  "), d.UnitCurrent, d.UnitTotal, d.UnitKind))
	}
	sb.WriteString("\n" + theme.Muted.Render("enter: open in Reader  s: start session"))
	return sb.String()
}

func (m Model) loadSourcesCmd() tea.Cmd {
	return func() tea.Msg {
		sources, err := m.port.ListSources(context.Background())
		return SourcesLoadedMsg{Sources: sources, Err: err}
	}
}

func (m Model) loadDetailCmd(id string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.port.GetSource(context.Background(), id)
		return DetailLoadedMsg{Detail: detail, Err: err}
	}
}
