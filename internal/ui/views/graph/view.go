package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	graphdto "mdht/internal/modules/graph/dto"
	"mdht/internal/ui/theme"
)

// ─── port ────────────────────────────────────────────────────────────────────

type GraphPort interface {
	ListTopics(ctx context.Context, limit int) ([]graphdto.TopicSummaryOutput, error)
	Neighbors(ctx context.Context, nodeID string, depth int) (graphdto.NeighborsOutput, error)
	Search(ctx context.Context, query string) ([]graphdto.NodeOutput, error)
}

// ─── messages ────────────────────────────────────────────────────────────────

type TopicsLoadedMsg struct {
	Topics []graphdto.TopicSummaryOutput
	Err    error
}

type NeighborsLoadedMsg struct {
	Out graphdto.NeighborsOutput
	Err error
}

type SearchResultMsg struct {
	Nodes []graphdto.NodeOutput
	Err   error
}

// ─── list item ───────────────────────────────────────────────────────────────

type topicItem struct {
	topic graphdto.TopicSummaryOutput
}

func (i topicItem) Title() string       { return i.topic.TopicSlug }
func (i topicItem) Description() string { return fmt.Sprintf("%d sources", i.topic.SourceCount) }
func (i topicItem) FilterValue() string { return i.topic.TopicSlug }

// ─── model ───────────────────────────────────────────────────────────────────

type Model struct {
	port      GraphPort
	list      list.Model
	detail    viewport.Model
	spinner   spinner.Model
	neighbors graphdto.NeighborsOutput
	loading   bool
	width     int
	height    int
}

func New(port GraphPort) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(theme.Green).BorderForeground(theme.Green)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(theme.Sapphire).BorderForeground(theme.Green)

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Graph"
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
	sp.Style = lipgloss.NewStyle().Foreground(theme.Green)

	return Model{
		port:    port,
		list:    l,
		detail:  vp,
		spinner: sp,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadTopicsCmd(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case TopicsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.list.Title = "Graph — " + msg.Err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.Topics))
		for i, t := range msg.Topics {
			items[i] = topicItem{topic: t}
		}
		cmds = append(cmds, m.list.SetItems(items))

	case NeighborsLoadedMsg:
		if msg.Err == nil {
			m.neighbors = msg.Out
			m.detail.SetContent(m.renderNeighbors())
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if msg.String() == "enter" {
			if item, ok := m.list.SelectedItem().(topicItem); ok {
				cmds = append(cmds, m.loadNeighborsCmd(item.topic.TopicSlug))
			}
		}
	}

	if !m.loading {
		var lCmd tea.Cmd
		m.list, lCmd = m.list.Update(msg)
		cmds = append(cmds, lCmd)

		var vCmd tea.Cmd
		m.detail, vCmd = m.detail.Update(msg)
		cmds = append(cmds, vCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading graph…")
	}

	listW := m.width * 35 / 100
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
		Render(m.detail.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

// ─── private ─────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	listW := m.width * 35 / 100
	detailW := m.width - listW
	m.list.SetSize(listW, m.height)
	m.detail.Width = detailW - 4
	m.detail.Height = m.height - 4
}

func (m Model) renderNeighbors() string {
	if m.neighbors.FocusID == "" {
		return theme.Muted.Render("Select a topic and press enter to explore neighbors")
	}
	var sb strings.Builder
	sb.WriteString(theme.Title.Render("Neighbors of: "+m.neighbors.FocusID) + "\n")
	sb.WriteString(theme.Muted.Render(fmt.Sprintf("depth %d  —  %d nodes\n\n", m.neighbors.Depth, len(m.neighbors.Nodes))))
	for _, n := range m.neighbors.Nodes {
		icon := "○"
		switch n.Kind {
		case "source":
			icon = "◈"
		case "topic":
			icon = "◇"
		}
		sb.WriteString(fmt.Sprintf(" %s  %s  %s\n",
			icon,
			theme.Title.Render(n.Label),
			theme.Muted.Render("["+n.Kind+"]")))
	}
	return sb.String()
}

// Filtering reports whether the list's search filter is currently active.
// The app model checks this to avoid consuming global keys (e.g. "q") during
// a search.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) loadTopicsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return TopicsLoadedMsg{}
		}
		topics, err := m.port.ListTopics(context.Background(), 200)
		return TopicsLoadedMsg{Topics: topics, Err: err}
	}
}

func (m Model) loadNeighborsCmd(nodeID string) tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return NeighborsLoadedMsg{}
		}
		out, err := m.port.Neighbors(context.Background(), nodeID, 1)
		return NeighborsLoadedMsg{Out: out, Err: err}
	}
}
