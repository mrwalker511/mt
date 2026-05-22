package tui

import "github.com/charmbracelet/lipgloss"

const (
	colorFocused   = lipgloss.Color("#7D56F4")
	colorUnfocused = lipgloss.Color("#3a3a3a")
	colorTitle     = lipgloss.Color("#FFFDF5")
	colorSelected  = lipgloss.Color("#EE6FF8")
	colorNormal    = lipgloss.Color("#DDDDDD")
	colorDim       = lipgloss.Color("#626262")
	colorHelp      = lipgloss.Color("#999999")
)

// paneStyle returns a border+padding style with color reflecting focus state.
// Width and Height are NOT set here — callers apply .Width(w).Height(h) so
// layout computation stays in view.go.
func paneStyle(focused bool) lipgloss.Style {
	borderColor := colorUnfocused
	if focused {
		borderColor = colorFocused
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTitle).
			MarginBottom(1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorSelected).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(colorNormal)

	dimItemStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1C1C1C")).
			Foreground(colorHelp).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F5F"))

	liveHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5fd7af")).
				Bold(true)
)
