package theme

import "github.com/charmbracelet/lipgloss"

var (
	Base     = lipgloss.Color("#1e1e2e")
	Mantle   = lipgloss.Color("#181825")
	Surface0 = lipgloss.Color("#313244")
	Surface1 = lipgloss.Color("#45475a")
	Text     = lipgloss.Color("#cdd6f4")
	Subtext0 = lipgloss.Color("#a6adc8")
	Lavender = lipgloss.Color("#b4befe")
	Sapphire = lipgloss.Color("#74c7ec")
	Green    = lipgloss.Color("#a6e3a1")
	Peach    = lipgloss.Color("#fab387")

	App = lipgloss.NewStyle().
		Background(Base).
		Foreground(Text).
		Padding(1, 2)

	Pane = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Surface1).
		Background(Mantle).
		Foreground(Text).
		Padding(1)

	PaneActive = Pane.BorderForeground(Lavender)

	Title = lipgloss.NewStyle().Foreground(Sapphire).Bold(true)
	Muted = lipgloss.NewStyle().Foreground(Subtext0)
	Hot   = lipgloss.NewStyle().Foreground(Peach).Bold(true)
)
