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

	hint := "[↑↓/jk] Move  [←→/hl] Change Pane  [↵] Execute  [c] Clear  [?] Help  [q] Quit"
	if len(m.allWorkspaces) > 1 {
		hint += "  [tab] Workspace"
	}
	bar := statusBarStyle.Width(m.width).Render(hint)

	return lipgloss.JoinVertical(lipgloss.Left, body, bar)
}

func (m Model) renderLeftPane(w, h int) string {
	focused := m.activePane == paneLeft
	titleText := "DOMAINS"
	if len(m.allWorkspaces) > 1 && m.allWorkspaces[m.workspaceIdx].Name != "" {
		titleText = "DOMAINS — " + m.allWorkspaces[m.workspaceIdx].Name
	}
	title := titleStyle.Render(titleText)
	items := m.renderList(m.domainNames(), m.domainCursor, focused)
	content := lipgloss.JoinVertical(lipgloss.Left, title, items)
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

func (m Model) renderMiddlePane(w, h int) string {
	focused := m.activePane == paneMiddle
	title   := titleStyle.Render("TARGETS & ACTIONS")
	names   := m.targetNamesWithBadges()
	items   := m.renderList(names, m.targetCursor, focused)
	content := lipgloss.JoinVertical(lipgloss.Left, title, items)
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

func (m Model) renderRightPane(w, h int) string {
	focused := m.activePane == paneRight
	title   := titleStyle.Render("LIVE STATUS / INFO")

	if m.showHelp {
		content := lipgloss.JoinVertical(lipgloss.Left, title, m.renderHelp())
		return paneStyle(focused).Width(w).Height(h).Render(content)
	}

	var text string
	switch {
	case m.running:
		text = dimItemStyle.Render("Running…")
	case m.cmdErr != "" && m.output != "":
		text = normalItemStyle.Render(m.output) + "\n\n" + errorStyle.Render("Error: "+m.cmdErr)
	case m.cmdErr != "":
		text = errorStyle.Render("Error: " + m.cmdErr)
	case m.output != "":
		text = normalItemStyle.Render(m.output)
	default:
		status := "Select a target to view status."
		if m.domainCursor < len(m.domains) {
			targets := m.domains[m.domainCursor].Targets
			if len(targets) > 0 && m.targetCursor < len(targets) {
				status = targets[m.targetCursor].Status
			}
		}
		text = normalItemStyle.Render(status)
	}

	parts := []string{title}
	if header := m.domainLiveHeader(); header != "" {
		parts = append(parts, header, "")
	}
	parts = append(parts, text)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return paneStyle(focused).Width(w).Height(h).Render(content)
}

// renderHelp builds the keybinding reference displayed when showHelp is true.
func (m Model) renderHelp() string {
	lines := []string{
		dimItemStyle.Render("Navigation"),
		normalItemStyle.Render("  ↑ / k        Move up"),
		normalItemStyle.Render("  ↓ / j        Move down"),
		normalItemStyle.Render("  ← / h        Focus left pane"),
		normalItemStyle.Render("  → / l        Focus right pane"),
		"",
		dimItemStyle.Render("Actions"),
		normalItemStyle.Render("  Enter        Execute selected target"),
		normalItemStyle.Render("  c            Clear output"),
	}
	if len(m.allWorkspaces) > 1 {
		lines = append(lines,
			normalItemStyle.Render("  Tab          Next workspace"),
			normalItemStyle.Render("  Shift+Tab    Previous workspace"),
		)
	}
	lines = append(lines,
		normalItemStyle.Render("  ?            Toggle this help"),
		normalItemStyle.Render("  q / Ctrl+C   Quit"),
	)
	if t, ok := m.currentTarget(); ok {
		lines = append(lines, "", dimItemStyle.Render("Current target"))
		lines = append(lines, normalItemStyle.Render("  "+t.Name))
		if len(t.Cmd) > 0 {
			lines = append(lines, dimItemStyle.Render("  $ "+strings.Join(t.Cmd, " ")))
		} else {
			lines = append(lines, dimItemStyle.Render("  (no command configured)"))
		}
	}
	return strings.Join(lines, "\n")
}

// domainLiveHeader returns a styled one-liner of live probe data for the current
// domain, or "" if no live data is available for that domain.
func (m Model) domainLiveHeader() string {
	if m.domainCursor >= len(m.domains) {
		return ""
	}
	switch m.domains[m.domainCursor].Name {
	case "Context/Git":
		branch := m.liveStatus["git.branch"]
		if branch == "" {
			return ""
		}
		label := "Branch: " + branch
		if dirty := m.liveStatus["git.dirty"]; dirty != "" {
			label += "  (" + dirty + ")"
		}
		return liveHeaderStyle.Render(label)

	case "Infrastructure":
		pg := m.liveStatus["docker.postgres"]
		rd := m.liveStatus["docker.redis"]
		if pg == "" && rd == "" {
			return ""
		}
		var parts []string
		if pg != "" {
			parts = append(parts, "postgres: "+pg)
		}
		if rd != "" {
			parts = append(parts, "redis: "+rd)
		}
		return liveHeaderStyle.Render(strings.Join(parts, "  |  "))
	}
	return ""
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

// targetNamesWithBadges returns target display names with a ✓/✗ badge appended
// when the target has been executed in the current session.
func (m Model) targetNamesWithBadges() []string {
	if m.domainCursor >= len(m.domains) {
		return []string{}
	}
	domain := m.domains[m.domainCursor]
	names := make([]string, len(domain.Targets))
	for i, t := range domain.Targets {
		key := m.runKey(domain.Name, t.Name)
		badge := ""
		switch m.runStates[key] {
		case runSuccess:
			badge = " ✓"
		case runFailure:
			badge = " ✗"
		}
		names[i] = t.Name + badge
	}
	return names
}

func (m Model) domainNames() []string {
	names := make([]string, len(m.domains))
	for i, d := range m.domains {
		names[i] = d.Name
	}
	return names
}

func (m Model) targetNames() []string {
	if m.domainCursor >= len(m.domains) {
		return []string{}
	}
	targets := m.domains[m.domainCursor].Targets
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}
	return names
}
