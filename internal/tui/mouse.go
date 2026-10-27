package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// doubleClickWindow is how quickly a second click on the same target must
// follow the first to count as a double-click (macOS defaults to about 0.5s).
const doubleClickWindow = 500 * time.Millisecond

// listTopRow is the screen row of the first list item: the pane's top border,
// its title, and the blank line under the title come first.
const listTopRow = 3

// mouseHelp lists the trackpad and mouse gestures: short is for the in-app ?
// overlay, desc for `mt --keys`.
var mouseHelp = []struct{ gesture, short, desc string }{
	{"click", "Select item", "Select a domain or target and focus its pane"},
	{"2×click", "Run target", "Run the target (like opening a file in Finder)"},
	{"scroll", "Scroll output", "Two-finger scroll over the right pane scrolls the output"},
}

// now is the clock used for double-click detection; tests replace it.
var now = time.Now

// handleMouse applies a trackpad or mouse event: a click selects the item under
// the pointer and focuses its pane, a double-click on a target runs it, and
// scrolling over the right pane scrolls the output.
func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	p, ok := m.paneAt(msg.X)
	if !ok || msg.Y >= m.height-statusBarHeight {
		return m, nil
	}

	if tea.MouseEvent(msg).IsWheel() {
		if p == paneRight {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.scrollOffset = max(m.scrollOffset-1, 0)
			case tea.MouseButtonWheelDown:
				maxOffset := max(strings.Count(m.output, "\n")+1-m.rightPanePageSize(), 0)
				m.scrollOffset = min(m.scrollOffset+1, maxOffset)
			}
		}
		return m, nil
	}

	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	m.activePane = p
	row := msg.Y - listTopRow

	switch p {
	case paneLeft:
		if row >= 0 && row < len(m.domains) && row != m.domainCursor {
			m = m.moveCursor(row, 0)
		}
	case paneMiddle:
		if m.domainCursor >= len(m.domains) || row < 0 || row >= len(m.domains[m.domainCursor].Targets) {
			return m, nil
		}
		if row != m.targetCursor {
			m = m.moveCursor(m.domainCursor, row)
		}
		key, _ := m.currentKey()
		t := now()
		if key == m.lastClickKey && t.Sub(m.lastClickAt) <= doubleClickWindow {
			m.lastClickKey = ""
			if m.running {
				return m, nil
			}
			target, _ := m.currentTarget()
			return m.startTarget(m.domains[m.domainCursor].Name, target)
		}
		m.lastClickKey, m.lastClickAt = key, t
	}
	return m, nil
}

// paneAt returns the pane under screen column x.
func (m Model) paneAt(x int) (pane, bool) {
	leftW, midW, _, _, ok := m.layout()
	if !ok || x < 0 || x >= m.width {
		return 0, false
	}
	// Each pane's outer width is its inner width plus the left and right border.
	switch {
	case x < leftW+paneFrameWidth:
		return paneLeft, true
	case x < leftW+midW+2*paneFrameWidth:
		return paneMiddle, true
	default:
		return paneRight, true
	}
}
