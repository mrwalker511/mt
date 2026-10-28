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
		return "Terminal too narrow, please resize."
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
		titleText = "DOMAINS: " + m.allWorkspaces[m.workspaceIdx].Name
	}
	names := m.domainNames()
	title := titleStyle.Render(titleText + m.listPosition(m.domainCursor, len(names)))
	items := m.renderList(names, m.domainCursor, m.domainTop, focused, w)
	return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, items))
}

func (m Model) renderMiddlePane(w, h int) string {
	focused := m.activePane == paneMiddle
	names := m.targetNamesWithBadges()
	title := titleStyle.Render("TARGETS & ACTIONS" + m.listPosition(m.targetCursor, len(names)))
	items := m.renderList(names, m.targetCursor, m.targetTop, focused, w)
	return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, items))
}

func (m Model) renderRightPane(w, h int) string {
	focused := m.activePane == paneRight
	title := titleStyle.Render("LIVE STATUS / INFO")

	if m.showHelp {
		return renderPane(focused, w, h, lipgloss.JoinVertical(lipgloss.Left, title, m.renderHelp()))
	}

	var sections []string
	switch {
	case m.confirmCmd != nil:
		sections = append(sections, normalItemStyle.Render("AI wants to run:\n\n  "+strings.Join(m.confirmCmd, " ")+"\n\n⏎ confirm  ·  ⎋ cancel"))
	case m.output != "":
		sections = append(sections, m.renderScrollableOutput(m.output))
	case !m.running && m.cmdErr == "":
		status := "Select a target to view status."
		if t, ok := m.currentTarget(); ok {
			status = t.Status
		}
		sections = append(sections, normalItemStyle.Render(status))
	}
	if m.running && m.confirmCmd == nil {
		sections = append(sections, dimItemStyle.Render(m.runBanner()))
	}
	if m.cmdErr != "" && m.confirmCmd == nil {
		sections = append(sections, errorStyle.Render("Error: "+m.cmdErr))
	}
	text := strings.Join(sections, "\n\n")

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
// string as written (or the cmd: list, quoted where needed), with any dir: as
// a leading cd and any host: as a leading ssh.
func displayCmd(t Target) string {
	cmd := t.Run
	if cmd == "" {
		cmd = quoteArgs(t.Cmd)
	}
	if t.Dir != "" {
		dir := quoteArgs([]string{t.Dir})
		if t.Host != "" {
			dir = remoteDir(t.Dir)
		}
		cmd = "cd " + dir + " && " + cmd
	}
	if t.Host != "" {
		cmd = "ssh " + t.Host + " " + cmd
	}
	return cmd
}

// quoteArgs joins args with spaces, single-quoting any that need it.
func quoteArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\n'\"") {
			a = shellQuote(a)
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}

// listRows is how many list items fit under a pane's title.
func (m Model) listRows() int {
	_, _, _, contentH, _ := m.layout()
	return max(contentH-(listTopRow-1), 1)
}

// scrollIntoView returns the first visible index of a list of n items, moved
// as little as possible from top so that cursor is on screen.
func scrollIntoView(top, cursor, n, rows int) int {
	if cursor < top {
		top = cursor
	}
	if cursor >= top+rows {
		top = cursor - rows + 1
	}
	return min(max(top, 0), max(n-rows, 0))
}

// syncListTops keeps the remembered first visible row of each list in step
// with its cursor and the window size. Update calls it after every message.
func (m Model) syncListTops() Model {
	rows := m.listRows()
	m.domainTop = scrollIntoView(m.domainTop, m.domainCursor, len(m.domains), rows)
	n := 0
	if m.domainCursor < len(m.domains) {
		n = len(m.domains[m.domainCursor].Targets)
	}
	m.targetTop = scrollIntoView(m.targetTop, m.targetCursor, n, rows)
	return m
}

// listPosition returns " (3/40)" for a list too long to show at once, so the
// title tells you there is more above or below; otherwise "".
func (m Model) listPosition(cursor, n int) string {
	if n <= m.listRows() {
		return ""
	}
	return fmt.Sprintf(" (%d/%d)", cursor+1, n)
}

// renderList builds a styled string for the visible part of a selectable list,
// starting at item top. Each item is cut to one line of the pane's width w so
// that item i is on screen row listTopRow+i-top, which mouse clicks rely on.
func (m Model) renderList(items []string, cursor, top int, paneActive bool, w int) string {
	top = scrollIntoView(top, cursor, len(items), m.listRows())
	items = items[top:min(top+m.listRows(), len(items))]
	cursor -= top
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
	total := strings.Count(output, "\n") + 1
	pageSize := m.rightPanePageSize()
	if total <= pageSize {
		return normalItemStyle.Render(output)
	}
	// Clamp so a stale offset (output replaced, window resized) can never index
	// past the end. Only the visible lines are cut out, so a long streaming
	// output is not split into lines on every redraw.
	offset := min(max(m.scrollOffset, 0), total-pageSize)
	visible := lineWindow(output, offset, pageSize)
	return normalItemStyle.Render(visible) + "\n" + dimItemStyle.Render(fmt.Sprintf(
		"  ── %d/%d lines  [↑↓ fn↑↓] ──", offset+1, total))
}

// lineWindow returns n lines of s starting at line offset (0-based).
func lineWindow(s string, offset, n int) string {
	for ; offset > 0; offset-- {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
	end := 0
	for ; n > 0; n-- {
		i := strings.IndexByte(s[end:], '\n')
		if i < 0 {
			return s
		}
		end += i + 1
	}
	return s[:end-1]
}
