package collab

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	collabdto "mdht/internal/modules/collab/dto"
	"mdht/internal/ui/theme"
)

// ─── port ────────────────────────────────────────────────────────────────────

type CollabPort interface {
	Status(ctx context.Context) (collabdto.StatusOutput, error)
	PeerList(ctx context.Context) ([]collabdto.PeerOutput, error)
	ActivityTail(ctx context.Context, since time.Time, limit int) ([]collabdto.ActivityOutput, error)
	ConflictsList(ctx context.Context, entityKey string) ([]collabdto.ConflictOutput, error)
	ConflictResolve(ctx context.Context, conflictID, strategy string) (collabdto.ConflictOutput, error)
	SyncNow(ctx context.Context) (collabdto.ReconcileOutput, error)
	SyncHealth(ctx context.Context) (collabdto.SyncHealthOutput, error)
}

// ─── messages ────────────────────────────────────────────────────────────────

type StatusMsg struct {
	Status collabdto.StatusOutput
	Err    error
}

type PeersMsg struct {
	Peers []collabdto.PeerOutput
	Err   error
}

type ActivityMsg struct {
	Events []collabdto.ActivityOutput
	Err    error
}

type ConflictsMsg struct {
	Items []collabdto.ConflictOutput
	Err   error
}

type ResolveMsg struct {
	Item collabdto.ConflictOutput
	Err  error
}

type SyncMsg struct {
	Applied int
	Err     error
}

// ─── sub-tab ─────────────────────────────────────────────────────────────────

type subTab int

const (
	subTabPeers subTab = iota
	subTabActivity
	subTabConflicts
)

// ─── list items ──────────────────────────────────────────────────────────────

type peerItem struct{ p collabdto.PeerOutput }

func (i peerItem) Title() string       { return i.p.Label + " (" + i.p.State + ")" }
func (i peerItem) Description() string { return i.p.PeerID }
func (i peerItem) FilterValue() string { return i.p.PeerID + " " + i.p.Label }

type activityItem struct{ a collabdto.ActivityOutput }

func (i activityItem) Title() string       { return i.a.Type + ": " + i.a.Message }
func (i activityItem) Description() string { return i.a.OccurredAt.Format("15:04:05") }
func (i activityItem) FilterValue() string { return i.a.Message }

type conflictItem struct{ c collabdto.ConflictOutput }

func (i conflictItem) Title() string       { return i.c.EntityKey + "." + i.c.Field }
func (i conflictItem) Description() string { return "[" + i.c.Status + "] " + i.c.ID }
func (i conflictItem) FilterValue() string { return i.c.EntityKey + " " + i.c.Field }

// ─── model ───────────────────────────────────────────────────────────────────

type Model struct {
	port       CollabPort
	activeTab  subTab
	list       list.Model
	detail     viewport.Model
	spinner    spinner.Model
	status     collabdto.StatusOutput
	peers      []collabdto.PeerOutput
	activity   []collabdto.ActivityOutput
	conflicts  []collabdto.ConflictOutput
	loading    bool
	statusLine string
	width      int
	height     int
}

func New(port CollabPort) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(theme.Peach).BorderForeground(theme.Peach)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(theme.Sapphire).BorderForeground(theme.Peach)

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Collab — Peers"
	l.Styles.Title = theme.Title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Background(theme.Mantle).Foreground(theme.Text).Padding(1)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Peach)

	m := Model{
		port:    port,
		list:    l,
		detail:  vp,
		spinner: sp,
		loading: port != nil, // if no port, nothing to load
	}
	if port == nil {
		vp.SetContent(theme.Muted.Render("Collaboration is not available in this session."))
		m.detail = vp
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadStatusCmd(), m.loadPeersCmd(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case StatusMsg:
		if msg.Err == nil {
			m.status = msg.Status
		}
		m.detail.SetContent(m.renderStatus())

	case PeersMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusLine = "peers load failed: " + msg.Err.Error()
			return m, nil
		}
		m.peers = msg.Peers
		if m.activeTab == subTabPeers {
			cmds = append(cmds, m.list.SetItems(peersToItems(m.peers)))
		}
		m.detail.SetContent(m.renderStatus())

	case ActivityMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusLine = "activity load failed: " + msg.Err.Error()
			return m, nil
		}
		m.activity = msg.Events
		if m.activeTab == subTabActivity {
			cmds = append(cmds, m.list.SetItems(activityToItems(m.activity)))
		}

	case ConflictsMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusLine = "conflicts load failed: " + msg.Err.Error()
			return m, nil
		}
		m.conflicts = msg.Items
		if m.activeTab == subTabConflicts {
			cmds = append(cmds, m.list.SetItems(conflictsToItems(m.conflicts)))
		}

	case ResolveMsg:
		if msg.Err != nil {
			m.statusLine = "resolve failed: " + msg.Err.Error()
		} else {
			m.statusLine = "resolved: " + msg.Item.ID
		}
		cmds = append(cmds, m.loadConflictsCmd())

	case SyncMsg:
		if msg.Err != nil {
			m.statusLine = "sync failed: " + msg.Err.Error()
		} else {
			m.statusLine = fmt.Sprintf("synced: %d ops applied", msg.Applied)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			m.activeTab = subTabPeers
			m.list.Title = "Collab — Peers"
			m.loading = true
			cmds = append(cmds, m.loadPeersCmd(), m.spinner.Tick)
		case "2":
			m.activeTab = subTabActivity
			m.list.Title = "Collab — Activity"
			m.loading = true
			cmds = append(cmds, m.loadActivityCmd(), m.spinner.Tick)
		case "3":
			m.activeTab = subTabConflicts
			m.list.Title = "Collab — Conflicts"
			m.loading = true
			cmds = append(cmds, m.loadConflictsCmd(), m.spinner.Tick)
		case "S":
			cmds = append(cmds, m.syncNowCmd())
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

// Filtering reports whether the list's search filter is currently active.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading collab…")
	}

	tabs := m.renderSubTabs()
	tabH := lipgloss.Height(tabs)
	bodyH := m.height - tabH
	if bodyH < 1 {
		bodyH = 1
	}

	listW := m.width * 4 / 10
	detailW := m.width - listW

	listPane := lipgloss.NewStyle().Width(listW).Height(bodyH).Render(m.list.View())
	detailPane := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Surface1).
		Background(theme.Mantle).
		Width(detailW - 2).
		Height(bodyH - 2).
		Render(m.detail.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
	return lipgloss.JoinVertical(lipgloss.Left, tabs, body)
}

// ─── private ─────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	listW := m.width * 4 / 10
	detailW := m.width - listW
	// sub-tab bar ≈ 1 line; border top+bottom = 2; leave 1 for status
	contentH := m.height - 4
	if contentH < 1 {
		contentH = 1
	}
	m.list.SetSize(listW, contentH)
	m.detail.Width = detailW - 4
	m.detail.Height = contentH - 2 // border overhead
}

func (m Model) renderSubTabs() string {
	labels := []string{"1:Peers", "2:Activity", "3:Conflicts"}
	var parts []string
	for i, label := range labels {
		if subTab(i) == m.activeTab {
			parts = append(parts, theme.Hot.Render(" "+label+" "))
		} else {
			parts = append(parts, theme.Muted.Render(" "+label+" "))
		}
	}
	hint := theme.Muted.Render("  S:sync-now")
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...) + hint + "\n"
}

func (m Model) renderStatus() string {
	s := m.status
	var sb strings.Builder
	sb.WriteString(theme.Title.Render("Collab Status") + "\n\n")
	online := theme.Muted.Render("offline")
	if s.Online {
		online = theme.Hot.Render("online")
	}
	sb.WriteString("status:       " + online + "\n")
	sb.WriteString(fmt.Sprintf("peers:        %d (%d approved)\n", s.PeerCount, s.ApprovedPeerCount))
	sb.WriteString(fmt.Sprintf("pending ops:  %d\n", s.PendingOps))
	sb.WriteString(fmt.Sprintf("conflicts:    %d\n", s.PendingConflicts))
	if s.NodeID != "" {
		sb.WriteString("node id:      " + s.NodeID + "\n")
	}
	if s.WorkspaceID != "" {
		sb.WriteString("workspace:    " + s.WorkspaceID + "\n")
	}
	sb.WriteString("reachability: " + s.Reachability + "\n")
	sb.WriteString("nat:          " + s.NATMode + "\n")
	sb.WriteString("connectivity: " + s.Connectivity + "\n")
	if !s.LastSyncAt.IsZero() {
		sb.WriteString("last sync:    " + time.Since(s.LastSyncAt).Round(time.Second).String() + " ago\n")
	}
	if m.statusLine != "" {
		sb.WriteString("\n" + theme.Hot.Render(m.statusLine) + "\n")
	}
	return sb.String()
}

func peersToItems(peers []collabdto.PeerOutput) []list.Item {
	items := make([]list.Item, len(peers))
	for i, p := range peers {
		items[i] = peerItem{p: p}
	}
	return items
}

func activityToItems(events []collabdto.ActivityOutput) []list.Item {
	items := make([]list.Item, len(events))
	for i, a := range events {
		items[i] = activityItem{a: a}
	}
	return items
}

func conflictsToItems(conflicts []collabdto.ConflictOutput) []list.Item {
	items := make([]list.Item, len(conflicts))
	for i, c := range conflicts {
		items[i] = conflictItem{c: c}
	}
	return items
}

func (m Model) loadStatusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return StatusMsg{}
		}
		status, err := m.port.Status(context.Background())
		return StatusMsg{Status: status, Err: err}
	}
}

func (m Model) loadPeersCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return PeersMsg{}
		}
		peers, err := m.port.PeerList(context.Background())
		return PeersMsg{Peers: peers, Err: err}
	}
}

func (m Model) loadActivityCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return ActivityMsg{}
		}
		events, err := m.port.ActivityTail(context.Background(), time.Time{}, 100)
		return ActivityMsg{Events: events, Err: err}
	}
}

func (m Model) loadConflictsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return ConflictsMsg{}
		}
		items, err := m.port.ConflictsList(context.Background(), "")
		return ConflictsMsg{Items: items, Err: err}
	}
}

func (m Model) syncNowCmd() tea.Cmd {
	return func() tea.Msg {
		if m.port == nil {
			return SyncMsg{}
		}
		out, err := m.port.SyncNow(context.Background())
		return SyncMsg{Applied: out.Applied, Err: err}
	}
}
