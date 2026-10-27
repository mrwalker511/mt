package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// locate returns the screen cell where text first appears in the rendered view
// at or after column minX, so clicks are aimed using the real layout.
func locate(t *testing.T, m Model, text string, minX int) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(m.View(), "\n") {
		runes := []rune(line)
		for i := range runes {
			col := lipgloss.Width(string(runes[:i]))
			if col >= minX && strings.HasPrefix(string(runes[i:]), text) {
				return col, row
			}
		}
	}
	t.Fatalf("%q not found on screen:\n%s", text, m.View())
	return 0, 0
}

func click(m Model, x, y int) (Model, tea.Cmd) {
	next, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	return next.(Model), cmd
}

func wheel(m Model, x, y int, b tea.MouseButton) Model {
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: b, Action: tea.MouseActionPress})
	return next.(Model)
}

func mouseModel() Model {
	m := testModel()
	m.width, m.height = 100, 30
	m.selectedTargets = make(map[string]bool)
	return m
}

func TestMouse_ClickDomainSelectsAndFocuses(t *testing.T) {
	m := mouseModel()
	m.activePane = paneRight
	x, y := locate(t, m, "Domain B", 0)
	m, _ = click(m, x, y)
	if m.domainCursor != 1 || m.activePane != paneLeft {
		t.Errorf("expected Domain B selected with left pane focused, got cursor=%d pane=%d", m.domainCursor, m.activePane)
	}
}

func TestMouse_ClickTargetSelectsAndFocuses(t *testing.T) {
	m := mouseModel()
	x, y := locate(t, m, "T3", 0)
	m, cmd := click(m, x, y)
	if m.targetCursor != 2 || m.activePane != paneMiddle {
		t.Errorf("expected T3 selected with middle pane focused, got cursor=%d pane=%d", m.targetCursor, m.activePane)
	}
	if cmd != nil || m.running {
		t.Error("a single click must not run the target")
	}
}

func TestMouse_DoubleClickRunsTarget(t *testing.T) {
	m := mouseModel()
	m.ctx, m.cancel = context.WithCancel(context.Background())
	defer m.cancel()
	m.domains[0].Targets[1].Cmd = []string{"true"}
	clock := time.Unix(1000, 0)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()

	x, y := locate(t, m, "T2", 0)
	m, _ = click(m, x, y)
	clock = clock.Add(200 * time.Millisecond)
	m, cmd := click(m, x, y)
	if !m.running || cmd == nil {
		t.Fatal("expected a double-click to run the target")
	}
	if want := m.runKey("Domain A", "T2"); m.pendingTarget != want {
		t.Errorf("expected %q running, got %q", want, m.pendingTarget)
	}
}

func TestMouse_SlowSecondClickDoesNotRun(t *testing.T) {
	m := mouseModel()
	m.domains[0].Targets[1].Cmd = []string{"true"}
	clock := time.Unix(1000, 0)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()

	x, y := locate(t, m, "T2", 0)
	m, _ = click(m, x, y)
	clock = clock.Add(2 * time.Second)
	m, cmd := click(m, x, y)
	if m.running || cmd != nil {
		t.Error("two slow clicks must not count as a double-click")
	}
}

func TestMouse_ClickOnDifferentTargetsIsNotDoubleClick(t *testing.T) {
	m := mouseModel()
	m.domains[0].Targets[1].Cmd = []string{"true"}
	now = func() time.Time { return time.Unix(1000, 0) }
	defer func() { now = time.Now }()

	x1, y1 := locate(t, m, "T1", 0)
	x2, y2 := locate(t, m, "T2", 0)
	m, _ = click(m, x1, y1)
	m, cmd := click(m, x2, y2)
	if m.running || cmd != nil {
		t.Error("clicks on two different targets must not run anything")
	}
}

func TestMouse_ClickBelowListIsIgnored(t *testing.T) {
	m := mouseModel()
	x, _ := locate(t, m, "T1", 0)
	m, _ = click(m, x, 20)
	if m.targetCursor != 0 {
		t.Errorf("click on empty space changed the cursor to %d", m.targetCursor)
	}
}

func TestMouse_WheelScrollsOutputOverRightPane(t *testing.T) {
	m := mouseModel()
	m.output = strings.Repeat("line\n", 99) + "last"
	x, y := locate(t, m, "LIVE STATUS", 0)
	m = wheel(m, x, y+5, tea.MouseButtonWheelDown)
	m = wheel(m, x, y+5, tea.MouseButtonWheelDown)
	if m.scrollOffset != 2 {
		t.Errorf("expected scrollOffset 2, got %d", m.scrollOffset)
	}
	m = wheel(m, x, y+5, tea.MouseButtonWheelUp)
	m = wheel(m, x, y+5, tea.MouseButtonWheelUp)
	m = wheel(m, x, y+5, tea.MouseButtonWheelUp)
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset clamped at 0, got %d", m.scrollOffset)
	}

	tx, ty := locate(t, m, "T1", 0)
	m = wheel(m, tx, ty, tea.MouseButtonWheelDown)
	if m.scrollOffset != 0 || m.targetCursor != 0 {
		t.Error("scrolling over the target list should do nothing")
	}
}

func TestMouse_IgnoredWhilePromptOpen(t *testing.T) {
	m := mouseModel()
	m.inputMode = true
	x, y := locate(t, m, "Domain B", 0)
	m, _ = click(m, x, y)
	if m.domainCursor != 0 {
		t.Error("clicks must be ignored while the AI prompt bar is open")
	}
}

func TestRenderList_TruncatesLongNames(t *testing.T) {
	m := mouseModel()
	m.domains[0].Targets[0].Name = strings.Repeat("Very Long Target Name ", 10)
	for _, line := range strings.Split(m.View(), "\n") {
		if lipgloss.Width(line) > m.width {
			t.Fatalf("line wider than terminal: %q", line)
		}
	}
	// The next target must still be on its own row directly below.
	_, y1 := locate(t, m, "Very Long", 0)
	_, y2 := locate(t, m, "T2", 0)
	if y2 != y1+1 {
		t.Errorf("long name wrapped: T2 on row %d, want %d", y2, y1+1)
	}
}

func TestPaneAt_MatchesRenderedBorders(t *testing.T) {
	for _, width := range []int{60, 80, 100, 137} {
		m := mouseModel()
		m.width = width
		top := []rune(strings.Split(m.View(), "\n")[0])
		var starts []int
		for i, r := range top {
			if r == '╭' {
				starts = append(starts, lipgloss.Width(string(top[:i])))
			}
		}
		if len(starts) != 3 {
			t.Fatalf("width %d: expected 3 panes, found %d", width, len(starts))
		}
		for i, start := range starts {
			end := width
			if i < 2 {
				end = starts[i+1]
			}
			for _, x := range []int{start, end - 1} {
				if p, _ := m.paneAt(x); p != pane(i) {
					t.Errorf("width %d: column %d is in pane %d on screen, paneAt says %d", width, x, i, p)
				}
			}
		}
	}
}
