package reader

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	readerdto "mdht/internal/modules/reader/dto"
	"mdht/internal/ui/theme"
)

// ─── port ────────────────────────────────────────────────────────────────────

// Port is the minimal interface this view needs from the reader use-case.
type Port interface {
	OpenSource(ctx context.Context, sourceID, mode string, page int, launchExternal bool) (readerdto.OpenResult, error)
}

// ─── messages ────────────────────────────────────────────────────────────────

// OpenedMsg is sent when a source has been opened (or failed to open).
type OpenedMsg struct {
	Result readerdto.OpenResult
	Err    error
}

// ─── model ───────────────────────────────────────────────────────────────────

// Model is the self-contained Bubble Tea model for the Reader tab.
type Model struct {
	port     Port
	viewport viewport.Model
	spinner  spinner.Model
	result   readerdto.OpenResult
	renderer *glamour.TermRenderer
	loading  bool
	width    int
	height   int
}

// New creates a Reader Model backed by the given port.
func New(port Port) Model {
	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Lavender)

	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)

	return Model{
		port:     port,
		viewport: vp,
		spinner:  sp,
		renderer: r,
	}
}

// Init is a no-op: the reader is idle until OpenSource is called.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		if m.result.SourceID != "" {
			m.viewport.SetContent(m.renderContent())
		}

	case OpenedMsg:
		m.loading = false
		if msg.Err != nil {
			m.viewport.SetContent(theme.Hot.Render("Error: " + msg.Err.Error()))
			return m, nil
		}
		m.result = msg.Result
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	var vCmd tea.Cmd
	m.viewport, vCmd = m.viewport.Update(msg)
	cmds = append(cmds, vCmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	header := m.renderHeader()
	headerH := lipgloss.Height(header)
	footerH := 1

	vpHeight := m.height - headerH - footerH
	if vpHeight < 1 {
		vpHeight = 1
	}
	// Apply without mutating persisted state — lipgloss renders a copy.
	vpView := m.viewportAt(vpHeight)

	if m.loading {
		loading := lipgloss.Place(m.width, vpHeight, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Opening source…")
		return lipgloss.JoinVertical(lipgloss.Left, header, loading)
	}

	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, vpView, footer)
}

// OpenSource triggers loading a source. The returned Cmd produces an OpenedMsg.
func (m *Model) OpenSource(sourceID, mode string, page int, external bool) tea.Cmd {
	m.loading = true
	return tea.Batch(m.openCmd(sourceID, mode, page, external), m.spinner.Tick)
}

// NextPage advances to the next PDF page.
func (m Model) NextPage() tea.Cmd {
	if m.result.SourceID == "" || m.result.Mode != "pdf" {
		return nil
	}
	return m.openCmd(m.result.SourceID, "pdf", m.result.Page+1, false)
}

// PrevPage goes back one PDF page (floor: 1).
func (m Model) PrevPage() tea.Cmd {
	if m.result.SourceID == "" || m.result.Mode != "pdf" {
		return nil
	}
	page := m.result.Page - 1
	if page < 1 {
		page = 1
	}
	return m.openCmd(m.result.SourceID, "pdf", page, false)
}

// ─── private ─────────────────────────────────────────────────────────────────

func (m *Model) resize() {
	m.viewport.Width = m.width
	// header ≈ 2 lines, footer = 1 line
	m.viewport.Height = m.height - 3
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
	// Rebuild the glamour renderer so it word-wraps at the new terminal width.
	if r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(m.width),
	); err == nil {
		m.renderer = r
	}
}

// viewportAt renders the viewport content at a temporary height without
// mutating the persisted viewport.Height set by resize().
func (m Model) viewportAt(h int) string {
	vp := m.viewport
	vp.Height = h
	return vp.View()
}

func (m Model) renderHeader() string {
	if m.result.SourceID == "" {
		return theme.Title.Render("Reader") +
			theme.Muted.Render("  Open a source from the Library tab (enter)") + "\n"
	}
	parts := []string{
		theme.Title.Render(m.result.Title),
		theme.Muted.Render(fmt.Sprintf("[%s/%s]", m.result.Type, m.result.Mode)),
		theme.Muted.Render(fmt.Sprintf("%.1f%%", m.result.Percent)),
	}
	if m.result.Mode == "pdf" {
		parts = append(parts, theme.Muted.Render(
			fmt.Sprintf("p.%d/%d", m.result.Page, m.result.TotalPage),
		))
	}
	nav := theme.Muted.Render("  ←/→: page  ↑/↓: scroll  e: external")
	return strings.Join(parts, "  ") + nav + "\n"
}

func (m Model) renderFooter() string {
	return theme.Muted.Render(fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100))
}

func (m Model) renderContent() string {
	r := m.result
	if r.ExternalLaunched {
		return theme.Muted.Render("Opened in external application: " + r.ExternalTarget)
	}
	if r.Content == "" {
		return theme.Muted.Render("(no content)")
	}
	if m.renderer != nil && (r.Mode == "markdown" || r.Type == "markdown" || r.Type == "note") {
		if rendered, err := m.renderer.Render(r.Content); err == nil {
			return rendered
		}
	}
	return r.Content
}

func (m Model) openCmd(sourceID, mode string, page int, external bool) tea.Cmd {
	return func() tea.Msg {
		result, err := m.port.OpenSource(context.Background(), sourceID, mode, page, external)
		return OpenedMsg{Result: result, Err: err}
	}
}
