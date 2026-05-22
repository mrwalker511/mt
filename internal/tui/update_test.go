package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press sends a key message and returns the updated model.
func press(m Model, k tea.KeyMsg) Model {
	next, _ := m.Update(k)
	return next.(Model)
}

// send delivers any tea.Msg to Update and returns the updated model.
func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// --- Model initialisation ---

func TestNew_InitialState(t *testing.T) {
	m := New()
	if len(m.domains) == 0 {
		t.Fatal("expected domains to be populated")
	}
	if m.activePane != paneLeft {
		t.Errorf("activePane: got %d, want %d (paneLeft)", m.activePane, paneLeft)
	}
	if m.domainCursor != 0 || m.targetCursor != 0 {
		t.Error("expected cursors to start at 0")
	}
}

func TestInit_ReturnsCmd(t *testing.T) {
	if cmd := New().Init(); cmd == nil {
		t.Error("Init() should return a non-nil cmd (tick + polls)")
	}
}

// --- Window resize ---

func TestUpdate_WindowSize(t *testing.T) {
	m, _ := New().Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	got := m.(Model)
	if got.width != 200 || got.height != 50 {
		t.Errorf("got %dx%d, want 200x50", got.width, got.height)
	}
}

// --- Quit ---

func TestUpdate_Quit(t *testing.T) {
	quitKeys := []tea.KeyMsg{
		key('q'),
		{Type: tea.KeyCtrlC},
	}
	for _, k := range quitKeys {
		m, cmd := New().Update(k)
		if !m.(Model).quitting {
			t.Errorf("key %q: expected quitting=true", k)
		}
		if cmd == nil {
			t.Errorf("key %q: expected non-nil quit cmd", k)
		}
	}
}

// --- Pane navigation ---

func TestUpdate_PaneRight(t *testing.T) {
	for _, k := range []tea.KeyMsg{key('l'), {Type: tea.KeyRight}} {
		m := press(New(), k)
		if m.activePane != paneMiddle {
			t.Errorf("key %q: expected paneMiddle, got %d", k, m.activePane)
		}
	}
}

func TestUpdate_PaneLeft(t *testing.T) {
	start := New()
	start.activePane = paneMiddle
	for _, k := range []tea.KeyMsg{key('h'), {Type: tea.KeyLeft}} {
		m := press(start, k)
		if m.activePane != paneLeft {
			t.Errorf("key %q: expected paneLeft, got %d", k, m.activePane)
		}
	}
}

func TestUpdate_PaneClampsAtBoundaries(t *testing.T) {
	// can't go left of paneLeft
	m := press(New(), key('h'))
	if m.activePane != paneLeft {
		t.Error("expected to stay at paneLeft")
	}
	// can't go right of paneRight
	right := New()
	right.activePane = paneRight
	m = press(right, key('l'))
	if m.activePane != paneRight {
		t.Error("expected to stay at paneRight")
	}
}

// --- Domain cursor (left pane) ---

func TestUpdate_DomainDown(t *testing.T) {
	for _, k := range []tea.KeyMsg{key('j'), {Type: tea.KeyDown}} {
		m := press(New(), k)
		if m.domainCursor != 1 {
			t.Errorf("key %q: expected domainCursor=1, got %d", k, m.domainCursor)
		}
	}
}

func TestUpdate_DomainUp_AtTop_NoChange(t *testing.T) {
	m := press(New(), key('k'))
	if m.domainCursor != 0 {
		t.Error("expected domainCursor to stay at 0")
	}
}

func TestUpdate_DomainChange_ResetsTargetCursor(t *testing.T) {
	m := New()
	m.targetCursor = 3
	m = press(m, key('j'))
	if m.targetCursor != 0 {
		t.Errorf("expected targetCursor=0 after domain change, got %d", m.targetCursor)
	}
}

func TestUpdate_DomainNavigation_ClearsOutput(t *testing.T) {
	m := New()
	m.output = "stale output"
	m.cmdErr = "stale error"
	m = press(m, key('j'))
	if m.output != "" || m.cmdErr != "" {
		t.Error("expected output/cmdErr to clear on domain change")
	}
}

// --- Target cursor (middle pane) ---

func TestUpdate_TargetDown(t *testing.T) {
	m := New()
	m.activePane = paneMiddle
	m = press(m, key('j'))
	if m.targetCursor != 1 {
		t.Errorf("expected targetCursor=1, got %d", m.targetCursor)
	}
}

func TestUpdate_TargetUp_AtTop_NoChange(t *testing.T) {
	m := New()
	m.activePane = paneMiddle
	m = press(m, key('k'))
	if m.targetCursor != 0 {
		t.Error("expected targetCursor to stay at 0")
	}
}

func TestUpdate_TargetNavigation_ClearsOutput(t *testing.T) {
	m := New()
	m.activePane = paneMiddle
	m.output = "stale output"
	m = press(m, key('j'))
	if m.output != "" {
		t.Error("expected output to clear on target change")
	}
}

// --- Enter key ---

func TestUpdate_Enter_NoCommand(t *testing.T) {
	m := Model{
		domains: []Domain{
			{Name: "Test", Targets: []Target{{Name: "No Cmd", Status: "hint"}}},
		},
	}
	m, cmd := func() (Model, tea.Cmd) {
		next, c := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		return next.(Model), c
	}()
	if m.cmdErr == "" {
		t.Error("expected cmdErr when no command is configured")
	}
	if m.running {
		t.Error("expected running=false")
	}
	if cmd != nil {
		t.Error("expected nil cmd when no command configured")
	}
}

func TestUpdate_Enter_WhileRunning_IsNoop(t *testing.T) {
	m := Model{
		running: true,
		domains: []Domain{
			{Name: "Test", Targets: []Target{{Name: "Cmd", Cmd: []string{"echo", "hi"}}}},
		},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd while already running")
	}
}

func TestUpdate_Enter_WithCommand_SetsRunning(t *testing.T) {
	m := Model{
		domains: []Domain{
			{Name: "Test", Targets: []Target{{Name: "Cmd", Cmd: []string{"echo", "hi"}}}},
		},
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if !got.running {
		t.Error("expected running=true after Enter with command")
	}
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd")
	}
}

// --- cmdResultMsg ---

func TestUpdate_CmdResult_Success(t *testing.T) {
	m := Model{running: true}
	m = send(m, cmdResultMsg{output: "  hello world  "})
	if m.running {
		t.Error("expected running=false after result")
	}
	if m.output != "hello world" {
		t.Errorf("output: got %q, want %q", m.output, "hello world")
	}
	if m.cmdErr != "" {
		t.Errorf("expected empty cmdErr, got %q", m.cmdErr)
	}
}

func TestUpdate_CmdResult_Error(t *testing.T) {
	m := Model{running: true}
	m = send(m, cmdResultMsg{output: "partial output", err: errors.New("exit status 1")})
	if m.running {
		t.Error("expected running=false after error result")
	}
	if m.cmdErr == "" {
		t.Error("expected cmdErr to be set")
	}
	if m.output != "partial output" {
		t.Errorf("output: got %q, want %q", m.output, "partial output")
	}
}

func TestUpdate_CmdResult_EmptyOutput(t *testing.T) {
	m := Model{running: true}
	m = send(m, cmdResultMsg{output: "   "})
	if m.output != "" {
		t.Errorf("expected empty output after trimming, got %q", m.output)
	}
}

// --- Live status polling ---

func TestUpdate_StatusUpdate_WritesToMap(t *testing.T) {
	m := Model{liveStatus: make(map[string]string)}
	m = send(m, statusUpdateMsg{key: "git.branch", status: "main"})
	if m.liveStatus["git.branch"] != "main" {
		t.Errorf("got %q, want %q", m.liveStatus["git.branch"], "main")
	}
}

func TestUpdate_StatusUpdate_Overwrites(t *testing.T) {
	m := Model{liveStatus: map[string]string{"git.branch": "old"}}
	m = send(m, statusUpdateMsg{key: "git.branch", status: "feature/x"})
	if m.liveStatus["git.branch"] != "feature/x" {
		t.Errorf("got %q, want %q", m.liveStatus["git.branch"], "feature/x")
	}
}

func TestUpdate_Tick_ReturnsNewCmd(t *testing.T) {
	m := Model{liveStatus: make(map[string]string)}
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Error("tickMsg should return a non-nil batch cmd for the next cycle")
	}
}
