package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func longListModel(n int) Model {
	m := mouseModel()
	m.height = 15
	var ts []Target
	for i := 0; i < n; i++ {
		ts = append(ts, Target{Name: fmt.Sprintf("Target-%02d", i), Cmd: []string{"true"}})
	}
	m.domains = []Domain{{Name: "Big", Targets: ts}}
	m.activePane = paneMiddle
	return m
}

func TestList_CursorStaysVisibleWhileMoving(t *testing.T) {
	m := longListModel(40)
	for i := 0; i < 39; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyDown})
		if want := fmt.Sprintf("> Target-%02d", m.targetCursor); !strings.Contains(m.View(), want) {
			t.Fatalf("cursor at %d not visible", m.targetCursor)
		}
	}
	for i := 0; i < 39; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyUp})
		if want := fmt.Sprintf("> Target-%02d", m.targetCursor); !strings.Contains(m.View(), want) {
			t.Fatalf("cursor at %d not visible on the way up", m.targetCursor)
		}
	}
}

func TestList_TitleShowsPositionOnlyWhenOverflowing(t *testing.T) {
	m := longListModel(40)
	m.targetCursor = 4
	if !strings.Contains(m.View(), "TARGETS & ACTIONS (5/40)") {
		t.Error("expected position indicator for a long list")
	}
	if strings.Contains(mouseModel().View(), "/3)") {
		t.Error("short list should not show a position indicator")
	}
}

func TestList_ScrollMovesMinimally(t *testing.T) {
	m := longListModel(40)
	rows := m.listRows()
	for i := 0; i < rows+5; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	top := m.targetTop
	m = press(m, tea.KeyMsg{Type: tea.KeyUp}) // still on screen: no scroll
	if m.targetTop != top {
		t.Errorf("moving up within the window scrolled from %d to %d", top, m.targetTop)
	}
}

func TestMouse_ClickInScrolledList(t *testing.T) {
	m := longListModel(40)
	for i := 0; i < 30; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	// Click an item above the cursor: it must be selected, and the list must not jump.
	x, y := locate(t, m, "Target-25", 0)
	top := m.targetTop
	m, _ = click(m, x, y)
	m = m.syncListTops()
	if m.targetCursor != 25 {
		t.Errorf("expected Target-25 selected, got %d", m.targetCursor)
	}
	if m.targetTop != top {
		t.Errorf("list jumped from %d to %d after a click", top, m.targetTop)
	}
}

func TestMouse_DoubleClickInScrolledList(t *testing.T) {
	m := longListModel(40)
	m.ctx, m.cancel = context.WithCancel(context.Background())
	defer m.cancel()
	for i := 0; i < 30; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	now = func() time.Time { return time.Unix(1000, 0) }
	defer func() { now = time.Now }()
	x, y := locate(t, m, "Target-24", 0)
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	next, _ = next.(Model).Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := next.(Model).pendingTarget; got != "Big/Target-24" {
		t.Errorf("double-click ran %q, want Big/Target-24", got)
	}
}
