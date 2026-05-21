package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "left", "h":
			if m.activePane > paneLeft {
				m.activePane--
			}

		case "right", "l":
			if m.activePane < paneRight {
				m.activePane++
			}

		case "up", "k":
			switch m.activePane {
			case paneLeft:
				if m.domainCursor > 0 {
					m.domainCursor--
					m.targetCursor = 0
				}
			case paneMiddle:
				if m.targetCursor > 0 {
					m.targetCursor--
				}
			}

		case "down", "j":
			switch m.activePane {
			case paneLeft:
				if m.domainCursor < len(m.domains)-1 {
					m.domainCursor++
					m.targetCursor = 0
				}
			case paneMiddle:
				targets := m.domains[m.domainCursor].Targets
				if m.targetCursor < len(targets)-1 {
					m.targetCursor++
				}
			}
		}
		return m, nil
	}

	return m, nil
}
