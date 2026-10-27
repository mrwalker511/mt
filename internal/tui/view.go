package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	statusBarHeight = 1
	// Each pane: 1 left border + 1 right border. Padding is not counted here
	// because lipgloss's Width() already includes it.
	paneFrameWidth = 2
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

	leftW, midW, rightW, contentH, ok := m.layout()
	if !ok {
		return "Terminal too narrow — please resize."
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderLeftPane(leftW, contentH),
		m.renderMiddlePane(midW, contentH),
		m.renderRightPane(rightW, contentH),
	)

	var hint string
	switch {
	case m.inputMode:
		hint = "AI › " + m.inputBuf + "█   ⏎ send  ⎋ close  ⌥⌫ delete word"
	case m.llmPending:
		hint = "AI › thinking…   ⎋ cancel"
	case m.confirmCmd != nil:
		hint = "AI › run: " + strings.Join(m.confirmCmd, " ") + "   ⏎ confirm  ⎋ cancel"
	case m.running:
		hint = "Running…   ⎋ stop   " + statusBarHints(len(m.allWorkspaces))
	default:
		hint = statusBarHints(len(m.allWorkspaces))
	}
	// MaxHeight keeps the bar to one line on narrow terminals; a wrapped bar would
	// push the frame past the terminal height.
	bar := statusBarStyle.Width(m.width).MaxHeight(1).Render(hint)

	return lipgloss.JoinVertical(lipgloss.Left, body, bar)
}

// layout returns the inner width (including padding) of each pane and the
// content height shared by all three. ok is false when the terminal is too
// narrow to draw the panes. Mouse hit-testing uses the same numbers.
func (m Model) layout() (leftW, midW, rightW, contentH int, ok bool) {
	// Distribute usable content width across three panes, absorbing frame overhead.
	usable := m.width - paneFrameWidth*3
	if usable < 30 {
		return 0, 0, 0, 0, false
	}
	// The domain pane grows to fit the longest name (plus the "> " prefix and
	// padding), between 20% and 30% of the width, taking the space from the
	// target pane so the output pane keeps its share.
	base := usable * 20 / 100
	leftW = base
	for _, d := range m.domains {
		leftW = max(leftW, lipgloss.Width(d.Name)+4)
	}
	leftW = max(min(leftW, usable*30/100), 14)
	midW = usable*38/100 - max(leftW-base, 0)
	rightW = usable - leftW - midW
	contentH = max(m.height-statusBarHeight-borderHeight, 1)
	return leftW, midW, rightW, contentH, true
}

func (m Model) renderLeftPane(w, h int) string {
	focused := m.activePane == paneLeft
	titleText := "DOMAINS"
	if len(m.allWorkspaces) > 1 && m.allWorkspaces[m.workspaceIdx].Name != "" {
		titleText = "DOMAINS — " + m.allWorkspaces[m.workspaceIdx].Name
	}
	title := titleStyle.Render(titleText)
	items := m.renderList(m.domainNames(), m.domainCursor, focused, w)
	return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, items))
}

func (m Model) renderMiddlePane(w, h int) string {
	focused := m.activePane == paneMiddle
	title := titleStyle.Render("TARGETS & ACTIONS")
	names := m.targetNamesWithBadges()
	items := m.renderList(names, m.targetCursor, focused, w)
	return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, items))
}

func (m Model) renderRightPane(w, h int) string {
	focused := m.activePane == paneRight
	title := titleStyle.Render("LIVE STATUS / INFO")

	if m.showHelp {
		return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, m.renderHelp()))
	}

	var text string
	switch {
	case m.confirmCmd != nil:
		text = normalItemStyle.Render("AI wants to run:\n\n  " + strings.Join(m.confirmCmd, " ") + "\n\n[Enter] confirm  ·  [Esc] cancel")
	case m.running:
		text = dimItemStyle.Render("Running…")
	case m.cmdErr != "" && m.output != "":
		text = m.renderScrollableOutput(m.output) + "\n\n" + errorStyle.Render("Error: "+m.cmdErr)
	case m.cmdErr != "":
		text = errorStyle.Render("Error: " + m.cmdErr)
	case m.output != "":
		text = m.renderScrollableOutput(m.output)
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
	return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// renderPane draws content in a bordered pane of w×h, cutting off lines that
// would not fit (after wrapping) so the pane never grows past the terminal.
func renderPane(focused bool, w, h int, content string) string {
	style := paneStyle(focused).Width(w).Height(h)
	wrapped := lipgloss.NewStyle().Width(w - style.GetHorizontalPadding()).Render(content)
	if lines := strings.Split(wrapped, "\n"); len(lines) > h {
		wrapped = strings.Join(lines[:h], "\n")
	}
	return style.Render(wrapped)
}

// renderHelp builds the key reference displayed when showHelp is true. The
// current target's command comes first so it stays visible in short panes.
func (m Model) renderHelp() string {
	var lines []string
	if t, ok := m.currentTarget(); ok {
		lines = append(lines, dimItemStyle.Render(t.Name))
		switch {
		case len(t.Sequence) > 0:
			lines = append(lines, dimItemStyle.Render("sequence: "+strings.Join(t.Sequence, " → ")))
		case len(t.Cmd) > 0:
			lines = append(lines, dimItemStyle.Render("$ "+displayCmd(t)))
		default:
			lines = append(lines, dimItemStyle.Render("(no command configured)"))
		}
		lines = append(lines, "")
	}
	for _, l := range compactHelp(len(m.allWorkspaces)) {
		lines = append(lines, normalItemStyle.Render(l))
	}
	lines = append(lines, "", dimItemStyle.Render("Full list: mt --keys"))
	return strings.Join(lines, "\n")
}

// truncate shortens s to at most width display cells, ending in "…" when cut.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// displayCmd returns the target's command as the user would type it: the run:
// string as written, or the cmd: list with arguments containing spaces quoted.
func displayCmd(t Target) string {
	if t.Run != "" {
		if t.Host != "" {
			return "ssh " + t.Host + " " + t.Run
		}
		return t.Run
	}
	parts := effectiveCmd(t)
	out := make([]string, len(parts))
	for i, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t\n'\"") {
			p = "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
		}
		out[i] = p
	}
	return strings.Join(out, " ")
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

// renderList builds a styled string for a selectable list of items. Each item
// is cut to one line of the pane's width w so that row i is always on screen
// row i, which mouse clicks rely on.
func (m Model) renderList(items []string, cursor int, paneActive bool, w int) string {
	var sb strings.Builder
	for i, item := range items {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		label := truncate(prefix+item, w-paneStyle(false).GetHorizontalPadding())

		var s lipgloss.Style
		switch {
		case i == cursor && paneActive:
			s = selectedItemStyle
		case i == cursor:
			s = dimBoldItemStyle
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
// when the target has been executed, and a ◆ prefix when selected for parallel run.
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
		prefix := ""
		if m.selectedTargets[key] {
			prefix = "◆ "
		}
		names[i] = prefix + t.Name + badge
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

// renderScrollableOutput renders output text with scrolling when it exceeds the page size.
func (m Model) renderScrollableOutput(output string) string {
	lines := strings.Split(output, "\n")
	pageSize := m.rightPanePageSize()
	if len(lines) <= pageSize {
		return normalItemStyle.Render(output)
	}
	// Clamp so a stale offset (output replaced, window resized) can never index
	// past the end of lines.
	offset := min(max(m.scrollOffset, 0), len(lines)-pageSize)
	visible := strings.Join(lines[offset:offset+pageSize], "\n")
	var sb strings.Builder
	sb.WriteString(normalItemStyle.Render(visible))
	sb.WriteString("\n")
	sb.WriteString(dimItemStyle.Render(fmt.Sprintf(
		"  ── %d/%d lines  [j↓ k↑] ──", offset+1, len(lines))))
	return sb.String()
}
