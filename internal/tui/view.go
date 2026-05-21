package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	statusBarHeight = 1
	// Each pane: 1 left border + 1 right border + 1 left padding + 1 right padding
	paneFrameWidth = 4
	// Each pane: 1 top border + 1 bottom border
	borderHeight = 2
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "Initializing…"
	}

	// Distribute usable content width across three panes, absorbing frame overhead.
	usable := m.width - paneFrameWidth*3
	if usable < 30 {
		return "Terminal too narrow — please resize."
	}

	leftW := usable * 20 / 100
	if leftW < 14 {
		leftW = 14
	}
	midW   := usable * 38 / 100
	rightW := usable - leftW - midW

	contentH := m.height - statusBarHeight - borderHeight
	if contentH < 1 {
		contentH = 1
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderLeftPane(leftW, contentH),
		m.renderMiddlePane(midW, contentH),
		m.renderRightPane(rightW, contentH),
	)

	bar := statusBarStyle.Width(m.width).Render(
		"[↑↓/jk] Move  [←→/hl] Change Pane  [↵] Execute  [q] Quit",
	)

	return lipgloss.JoinVertical(lipgloss.Left, body, bar)
}

func (m Model) renderLeftPane(w, h int) string {
	focused := m.activePane == paneLeft
	title   := titleStyle.Render("DOMAINS")
	items   := m.renderList(m.domainNames(), m.domainCursor, focused)
	content := lipgloss.JoinVertical(lipgloss.Left, title, items)
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

func (m Model) renderMiddlePane(w, h int) string {
	focused := m.activePane == paneMiddle
	title   := titleStyle.Render("TARGETS & ACTIONS")
	names   := m.targetNames()
	items   := m.renderList(names, m.targetCursor, focused)
	content := lipgloss.JoinVertical(lipgloss.Left, title, items)
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

func (m Model) renderRightPane(w, h int) string {
	focused := m.activePane == paneRight
	title   := titleStyle.Render("LIVE STATUS / INFO")

	status := "Select a target to view status."
	targets := m.domains[m.domainCursor].Targets
	if len(targets) > 0 && m.targetCursor < len(targets) {
		status = targets[m.targetCursor].Status
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, normalItemStyle.Render(status))
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

// renderList builds a styled string for a selectable list of items.
func (m Model) renderList(items []string, cursor int, paneActive bool) string {
	var sb strings.Builder
	for i, item := range items {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		label := prefix + item

		var s lipgloss.Style
		switch {
		case i == cursor && paneActive:
			s = selectedItemStyle
		case i == cursor:
			s = dimItemStyle.Bold(true)
		default:
			s = normalItemStyle
		}

		sb.WriteString(s.Render(label))
		if i < len(items)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (m Model) domainNames() []string {
	names := make([]string, len(m.domains))
	for i, d := range m.domains {
		names[i] = d.Name
	}
	return names
}

func (m Model) targetNames() []string {
	targets := m.domains[m.domainCursor].Targets
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}
	return names
}
