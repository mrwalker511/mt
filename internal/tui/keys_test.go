package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBindings_NoKeyBoundTwice(t *testing.T) {
	seen := map[string]action{}
	for _, b := range bindings {
		for _, k := range b.keys {
			if prev, ok := seen[k]; ok {
				t.Errorf("key %q bound to both action %d and %d", k, prev, b.action)
			}
			seen[k] = b.action
		}
	}
}

func TestKeyHelp_ListsEveryBinding(t *testing.T) {
	help := KeyHelp()
	for _, b := range bindings {
		if !strings.Contains(help, b.desc) {
			t.Errorf("KeyHelp missing %q", b.desc)
		}
	}
}

func TestStatusBarHints_WorkspaceKeysOnlyWhenMultiple(t *testing.T) {
	if strings.Contains(statusBarHints(1), "Workspace") {
		t.Error("single workspace should not advertise workspace keys")
	}
	if !strings.Contains(statusBarHints(2), "[ ] Workspace") {
		t.Errorf("expected workspace hint, got %q", statusBarHints(2))
	}
}

func TestUpdate_Tab_WrapsPanes(t *testing.T) {
	m := testModel()
	m.activePane = paneRight
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.activePane != paneLeft {
		t.Errorf("expected Tab to wrap to left pane, got %d", m.activePane)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.activePane != paneRight {
		t.Errorf("expected Shift+Tab to wrap to right pane, got %d", m.activePane)
	}
}

func TestUpdate_DigitJumpsToWorkspace(t *testing.T) {
	m := testWorkspaceModel()
	m = press(m, key('3'))
	if m.workspaceIdx != 2 {
		t.Errorf("expected workspace 3 (index 2), got %d", m.workspaceIdx)
	}
	m = press(m, key('9')) // out of range: no change
	if m.workspaceIdx != 2 {
		t.Errorf("expected out-of-range digit to be ignored, got %d", m.workspaceIdx)
	}
}

func TestUpdate_PageKeys_ScrollAndClamp(t *testing.T) {
	m := testModel()
	m.height = 20
	m.output = strings.Repeat("line\n", 99) + "last"
	page := m.rightPanePageSize()
	maxOffset := 100 - page

	m = press(m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.scrollOffset != page {
		t.Errorf("PgDown: expected %d, got %d", page, m.scrollOffset)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset != maxOffset {
		t.Errorf("End: expected %d, got %d", maxOffset, m.scrollOffset)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.scrollOffset != maxOffset {
		t.Errorf("PgDown past end should clamp to %d, got %d", maxOffset, m.scrollOffset)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset != 0 {
		t.Errorf("Home: expected 0, got %d", m.scrollOffset)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.scrollOffset != 0 {
		t.Errorf("PgUp past top should clamp to 0, got %d", m.scrollOffset)
	}
}

func TestUpdate_OldKeysStillWork(t *testing.T) {
	m := testModel()
	m.output = "x"
	for _, r := range []rune{'y', 'S'} {
		if _, cmd := m.Update(key(r)); cmd == nil {
			t.Errorf("legacy key %q should still trigger its action", r)
		}
	}
	m = press(m, key('R'))
	if !strings.Contains(m.output+m.cmdErr, "onfig") {
		t.Errorf("legacy R should reload config, got output=%q err=%q", m.output, m.cmdErr)
	}
}

func TestUpdate_InputMode_WordAndLineDelete(t *testing.T) {
	m := testModel()
	m.inputMode = true
	m.inputBuf = "open the calendar  "
	m = press(m, tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
	if m.inputBuf != "open the " {
		t.Errorf("⌥⌫: got %q", m.inputBuf)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if m.inputBuf != "open " {
		t.Errorf("⌃W: got %q", m.inputBuf)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.inputBuf != "" {
		t.Errorf("⌃U: got %q", m.inputBuf)
	}
}

func TestHelpRows_CoverEveryAction(t *testing.T) {
	covered := map[action]bool{}
	for _, r := range helpRows {
		for _, a := range r.actions {
			covered[a] = true
		}
	}
	for _, b := range bindings {
		if !covered[b.action] {
			t.Errorf("action for %q missing from helpRows", b.desc)
		}
	}
}
