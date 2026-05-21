package tui

import tea "github.com/charmbracelet/bubbletea"

type pane int

const (
	paneLeft   pane = 0
	paneMiddle pane = 1
	paneRight  pane = 2
)

// Model is the single source of truth for the entire TUI.
type Model struct {
	width  int
	height int

	activePane   pane
	domainCursor int
	targetCursor int

	domains  []Domain
	quitting bool
}

// New returns a freshly initialized Model with placeholder data.
func New() Model {
	return Model{
		domains:    initialDomains,
		activePane: paneLeft,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
