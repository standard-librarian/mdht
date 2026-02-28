package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mdht/internal/ui/theme"
)

// PaletteSubmitMsg is emitted when the user confirms a command.
type PaletteSubmitMsg struct{ Input string }

// PaletteCancelMsg is emitted when the user presses esc.
type PaletteCancelMsg struct{}

var (
	paletteStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Peach).
			Background(theme.Mantle).
			Foreground(theme.Text).
			Padding(0, 1)

	hintStyle = lipgloss.NewStyle().Foreground(theme.Subtext0)
)

// hints must stay in sync with the switch in app/model.go executePalette.
var paletteHints = []string{
	"session:start",
	"session:end <delta> <outcome>",
	"reader:open [mode] [page]",
	"reader:next-page",
	"reader:prev-page",
	"source:open-external",
	"plugin:exec <plugin> <command> [json]",
	"plugin:commands",
	"plugin:tty <plugin> <command> [json]",
	"collab:status",
	"collab:peers",
	"collab:activity",
	"collab:conflicts",
	"collab:resolve <id> <local|remote|merge>",
	"collab:sync-now",
	"collab:sync-health",
}

// Palette is a command-palette overlay backed by bubbles/textinput.
type Palette struct {
	input   textinput.Model
	visible bool
	width   int
}

// NewPalette creates an inactive Palette ready to be opened.
func NewPalette() Palette {
	ti := textinput.New()
	ti.Placeholder = "type a command…"
	ti.CharLimit = 256
	return Palette{input: ti}
}

// Visible reports whether the palette is currently shown.
func (p Palette) Visible() bool { return p.visible }

// Open shows the palette, clears the input, and returns the focus command.
func (p *Palette) Open() tea.Cmd {
	p.visible = true
	p.input.SetValue("")
	return p.input.Focus()
}

// SetWidth sets the render width for the overlay.
func (p *Palette) SetWidth(w int) { p.width = w }

func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	if !p.visible {
		return p, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			p.visible = false
			p.input.Blur()
			return p, func() tea.Msg { return PaletteCancelMsg{} }
		case "enter":
			val := strings.TrimSpace(p.input.Value())
			p.visible = false
			p.input.Blur()
			return p, func() tea.Msg { return PaletteSubmitMsg{Input: val} }
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p Palette) View() string {
	if !p.visible {
		return ""
	}
	prefix := strings.ToLower(p.input.Value())
	var matching []string
	for _, h := range paletteHints {
		if prefix == "" || strings.HasPrefix(h, prefix) {
			matching = append(matching, h)
			if len(matching) == 5 {
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(theme.Title.Render("Command Palette") + "\n")
	sb.WriteString(": " + p.input.View() + "\n")
	if len(matching) > 0 {
		sb.WriteString("\n")
		for _, h := range matching {
			sb.WriteString(hintStyle.Render("  "+h) + "\n")
		}
	}

	w := p.width
	if w < 20 {
		w = 64
	}
	return paletteStyle.Width(w - 2).Render(sb.String())
}
