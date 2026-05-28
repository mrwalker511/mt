package tui

import (
	"context"
	"errors"
	"strings"
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

// testModel returns a known, filesystem-independent model for navigation tests.
// It has 2 domains with 3 and 2 targets respectively.
func testModel() Model {
	return Model{
		activePane:    paneLeft,
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
		targetOutputs: make(map[string]outputRecord),
		domains: []Domain{
			{Name: "Domain A", Targets: []Target{
				{Name: "T1", Status: "hint1"},
				{Name: "T2", Status: "hint2"},
				{Name: "T3", Status: "hint3"},
			}},
			{Name: "Domain B", Targets: []Target{
				{Name: "T4", Status: "hint4"},
				{Name: "T5", Status: "hint5"},
			}},
		},
	}
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
	if m.liveStatus == nil {
		t.Error("expected liveStatus map to be initialized")
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
	m := press(New(), key('h'))
	if m.activePane != paneLeft {
		t.Error("expected to stay at paneLeft")
	}
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
		m := press(testModel(), k)
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
	m := testModel()
	m.targetCursor = 2
	m = press(m, key('j'))
	if m.targetCursor != 0 {
		t.Errorf("expected targetCursor=0 after domain change, got %d", m.targetCursor)
	}
}

func TestUpdate_DomainNavigation_ClearsOutput(t *testing.T) {
	m := testModel()
	m.output = "stale output"
	m.cmdErr = "stale error"
	m = press(m, key('j'))
	if m.output != "" || m.cmdErr != "" {
		t.Error("expected output/cmdErr to clear on domain change")
	}
}

// --- Target cursor (middle pane) ---

func TestUpdate_TargetDown(t *testing.T) {
	m := testModel()
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
	m := testModel()
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
		domains:    []Domain{{Name: "Test", Targets: []Target{{Name: "No Cmd", Status: "hint"}}}},
		liveStatus: make(map[string]string),
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
		running:    true,
		liveStatus: make(map[string]string),
		domains:    []Domain{{Name: "Test", Targets: []Target{{Name: "Cmd", Cmd: []string{"echo", "hi"}}}}},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd while already running")
	}
}

func TestUpdate_Enter_WithCommand_SetsRunning(t *testing.T) {
	m := Model{
		liveStatus: make(map[string]string),
		domains:    []Domain{{Name: "Test", Targets: []Target{{Name: "Cmd", Cmd: []string{"echo", "hi"}}}}},
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
	m := Model{running: true, liveStatus: make(map[string]string)}
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
	m := Model{running: true, liveStatus: make(map[string]string)}
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
	m := Model{running: true, liveStatus: make(map[string]string)}
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

// --- View() boundary states ---

func TestView_Quitting(t *testing.T) {
	m := Model{quitting: true, liveStatus: make(map[string]string)}
	if got := m.View(); got != "" {
		t.Errorf("expected empty string when quitting, got %q", got)
	}
}

func TestView_Uninitialized(t *testing.T) {
	m := Model{liveStatus: make(map[string]string)}
	if got := m.View(); got != "Initializing…" {
		t.Errorf("expected initializing message, got %q", got)
	}
}

func TestView_TooNarrow(t *testing.T) {
	m := Model{width: 30, height: 24, liveStatus: make(map[string]string)}
	got := m.View()
	if !strings.Contains(got, "narrow") {
		t.Errorf("expected narrow message, got %q", got)
	}
}

// --- domainLiveHeader ---

func TestDomainLiveHeader_UnknownDomain(t *testing.T) {
	m := testModel() // "Domain A", "Domain B" — no live-status mapping
	if got := m.domainLiveHeader(); got != "" {
		t.Errorf("expected empty for unknown domain, got %q", got)
	}
}

func TestDomainLiveHeader_GitNoBranch(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Context/Git"}},
		liveStatus: make(map[string]string),
	}
	if got := m.domainLiveHeader(); got != "" {
		t.Errorf("expected empty when no branch data, got %q", got)
	}
}

func TestDomainLiveHeader_GitBranchOnly(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Context/Git"}},
		liveStatus: map[string]string{"git.branch": "main"},
	}
	got := m.domainLiveHeader()
	if !strings.Contains(got, "main") {
		t.Errorf("expected branch name in header, got %q", got)
	}
	if strings.Contains(got, "modified") {
		t.Error("should not show dirty count when git.dirty is not set")
	}
}

func TestDomainLiveHeader_GitBranchAndDirty(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Context/Git"}},
		liveStatus: map[string]string{"git.branch": "feature/x", "git.dirty": "3 modified"},
	}
	got := m.domainLiveHeader()
	if !strings.Contains(got, "feature/x") {
		t.Errorf("expected branch in header, got %q", got)
	}
	if !strings.Contains(got, "3 modified") {
		t.Errorf("expected dirty count in header, got %q", got)
	}
}

func TestDomainLiveHeader_InfraNoData(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Infrastructure"}},
		liveStatus: make(map[string]string),
	}
	if got := m.domainLiveHeader(); got != "" {
		t.Errorf("expected empty when no docker data, got %q", got)
	}
}

func TestDomainLiveHeader_InfraPostgresOnly(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Infrastructure"}},
		liveStatus: map[string]string{"docker.postgres": "Up 2h"},
	}
	got := m.domainLiveHeader()
	if !strings.Contains(got, "postgres: Up 2h") {
		t.Errorf("expected postgres status in header, got %q", got)
	}
}

func TestDomainLiveHeader_InfraBothServices(t *testing.T) {
	m := Model{
		domains: []Domain{{Name: "Infrastructure"}},
		liveStatus: map[string]string{
			"docker.postgres": "Up 2h",
			"docker.redis":    "stopped",
		},
	}
	got := m.domainLiveHeader()
	if !strings.Contains(got, "postgres") || !strings.Contains(got, "redis") {
		t.Errorf("expected both services in header, got %q", got)
	}
}

func TestDomainLiveHeader_OutOfBounds(t *testing.T) {
	m := Model{
		domains:      []Domain{},
		domainCursor: 0,
		liveStatus:   make(map[string]string),
	}
	if got := m.domainLiveHeader(); got != "" {
		t.Errorf("expected empty for out-of-bounds cursor, got %q", got)
	}
}

// --- renderRightPane with live header ---

func TestRenderRightPane_LiveHeaderShown(t *testing.T) {
	m := Model{
		domains:    []Domain{{Name: "Context/Git", Targets: []Target{{Name: "Git Status", Status: "hint"}}}},
		liveStatus: map[string]string{"git.branch": "main"},
	}
	got := m.renderRightPane(40, 20)
	if !strings.Contains(got, "main") {
		t.Errorf("expected branch name in right pane output, got %q", got)
	}
}

// --- c key (clear output) ---

func TestUpdate_ClearKey_ClearsOutput(t *testing.T) {
	m := Model{
		output:     "some result",
		cmdErr:     "some error",
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
	}
	m = press(m, key('c'))
	if m.output != "" {
		t.Errorf("expected output cleared, got %q", m.output)
	}
	if m.cmdErr != "" {
		t.Errorf("expected cmdErr cleared, got %q", m.cmdErr)
	}
}

func TestUpdate_ClearKey_NoopWhenEmpty(t *testing.T) {
	m := Model{liveStatus: make(map[string]string), runStates: make(map[string]runResult)}
	m = press(m, key('c'))
	if m.output != "" || m.cmdErr != "" {
		t.Error("expected empty model to stay empty after c")
	}
}

// --- run badges ---

func TestUpdate_Badge_SetOnSuccess(t *testing.T) {
	m := Model{
		running:       true,
		pendingTarget: "Domain/Target",
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
	}
	m = send(m, cmdResultMsg{output: "ok"})
	if m.runStates["Domain/Target"] != runSuccess {
		t.Errorf("expected runSuccess, got %v", m.runStates["Domain/Target"])
	}
}

func TestUpdate_Badge_SetOnFailure(t *testing.T) {
	m := Model{
		running:       true,
		pendingTarget: "Domain/Target",
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
	}
	m = send(m, cmdResultMsg{output: "", err: errors.New("exit 1")})
	if m.runStates["Domain/Target"] != runFailure {
		t.Errorf("expected runFailure, got %v", m.runStates["Domain/Target"])
	}
}

func TestUpdate_Badge_PendingTargetCleared(t *testing.T) {
	m := Model{
		running:       true,
		pendingTarget: "Domain/Target",
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
	}
	m = send(m, cmdResultMsg{output: "done"})
	if m.pendingTarget != "" {
		t.Errorf("expected pendingTarget cleared, got %q", m.pendingTarget)
	}
}

// --- targetNamesWithBadges ---

func TestTargetNamesWithBadges_NoRuns(t *testing.T) {
	m := Model{
		domains:   []Domain{{Name: "D", Targets: []Target{{Name: "T1"}, {Name: "T2"}}}},
		runStates: make(map[string]runResult),
	}
	names := m.targetNamesWithBadges()
	for _, n := range names {
		if strings.Contains(n, "✓") || strings.Contains(n, "✗") {
			t.Errorf("expected no badge before any run, got %q", n)
		}
	}
}

func TestTargetNamesWithBadges_SuccessBadge(t *testing.T) {
	m := Model{
		domains:   []Domain{{Name: "D", Targets: []Target{{Name: "T1"}}}},
		runStates: map[string]runResult{"D/T1": runSuccess},
	}
	names := m.targetNamesWithBadges()
	if !strings.HasSuffix(names[0], " ✓") {
		t.Errorf("expected ✓ badge, got %q", names[0])
	}
}

func TestTargetNamesWithBadges_FailureBadge(t *testing.T) {
	m := Model{
		domains:   []Domain{{Name: "D", Targets: []Target{{Name: "T1"}}}},
		runStates: map[string]runResult{"D/T1": runFailure},
	}
	names := m.targetNamesWithBadges()
	if !strings.HasSuffix(names[0], " ✗") {
		t.Errorf("expected ✗ badge, got %q", names[0])
	}
}

// --- Workspace switching ---

// testWorkspaceModel returns a model with 3 named workspaces for workspace tests.
func testWorkspaceModel() Model {
	ws := []Workspace{
		{Name: "Alpha", Domains: []Domain{
			{Name: "Domain A", Targets: []Target{{Name: "T1"}, {Name: "T2"}}},
		}},
		{Name: "Beta", Domains: []Domain{
			{Name: "Domain B", Targets: []Target{{Name: "T3"}}},
		}},
		{Name: "Gamma", Domains: []Domain{
			{Name: "Domain C", Targets: []Target{{Name: "T4"}, {Name: "T5"}, {Name: "T6"}}},
		}},
	}
	return Model{
		activePane:    paneLeft,
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
		targetOutputs: make(map[string]outputRecord),
		allWorkspaces: ws,
		workspaceIdx:  0,
		domains:       ws[0].Domains,
	}
}

func TestUpdate_WorkspaceTab_CyclesForward(t *testing.T) {
	m := testWorkspaceModel()
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.workspaceIdx != 1 {
		t.Errorf("expected workspaceIdx=1 after tab, got %d", m.workspaceIdx)
	}
}

func TestUpdate_WorkspaceShiftTab_CyclesPrev(t *testing.T) {
	m := testWorkspaceModel()
	m.workspaceIdx = 1
	m.domains = m.allWorkspaces[1].Domains
	m = press(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.workspaceIdx != 0 {
		t.Errorf("expected workspaceIdx=0 after shift+tab, got %d", m.workspaceIdx)
	}
}

func TestUpdate_WorkspaceTab_Wraps(t *testing.T) {
	m := testWorkspaceModel()
	m.workspaceIdx = 2
	m.domains = m.allWorkspaces[2].Domains
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.workspaceIdx != 0 {
		t.Errorf("expected wrap to 0, got %d", m.workspaceIdx)
	}
}

func TestUpdate_WorkspaceSwitch_ResetsCursors(t *testing.T) {
	m := testWorkspaceModel()
	m.domainCursor = 0
	m.targetCursor = 1
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.domainCursor != 0 || m.targetCursor != 0 {
		t.Errorf("expected cursors reset to 0, got domain=%d target=%d", m.domainCursor, m.targetCursor)
	}
}

func TestUpdate_WorkspaceSwitch_ClearsOutput(t *testing.T) {
	m := testWorkspaceModel()
	m.output = "stale output"
	m.cmdErr = "stale error"
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.output != "" || m.cmdErr != "" {
		t.Error("expected output/cmdErr cleared on workspace switch")
	}
}

func TestHasGitDomain_True(t *testing.T) {
	m := Model{
		domains: []Domain{
			{Name: "App Launch"},
			{Name: "Context/Git"},
		},
	}
	if !m.hasGitDomain() {
		t.Error("expected hasGitDomain=true when Context/Git domain is present")
	}
}

func TestHasDockerDomain_False(t *testing.T) {
	m := Model{
		domains: []Domain{
			{Name: "App Launch"},
			{Name: "Context/Git"},
		},
	}
	if m.hasDockerDomain() {
		t.Error("expected hasDockerDomain=false when no Infrastructure domain")
	}
}

func TestUpdate_HelpToggle(t *testing.T) {
	m := Model{liveStatus: make(map[string]string), runStates: make(map[string]runResult)}
	m = press(m, key('?'))
	if !m.showHelp {
		t.Error("expected showHelp=true after first ?")
	}
	m = press(m, key('?'))
	if m.showHelp {
		t.Error("expected showHelp=false after second ?")
	}
}

func TestUpdate_WorkspaceTab_SingleWorkspace_NoOp(t *testing.T) {
	m := Model{
		activePane: paneLeft,
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
		allWorkspaces: []Workspace{
			{Name: "Only", Domains: []Domain{{Name: "D", Targets: []Target{{Name: "T"}}}}},
		},
		workspaceIdx: 0,
		domains:      []Domain{{Name: "D", Targets: []Target{{Name: "T"}}}},
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.workspaceIdx != 0 {
		t.Errorf("expected no change for single workspace, got idx=%d", m.workspaceIdx)
	}
}

// --- Phase 8: output scrolling ---

func TestUpdate_RightPane_ScrollDown(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.activePane = paneRight
	m.height = 30
	m.output = strings.Repeat("line\n", 50)
	m = press(m, key('j'))
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after j in right pane, got %d", m.scrollOffset)
	}
}

func TestUpdate_RightPane_ScrollUp(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.activePane = paneRight
	m.height = 30
	m.output = strings.Repeat("line\n", 50)
	m.scrollOffset = 5
	m = press(m, key('k'))
	if m.scrollOffset != 4 {
		t.Errorf("expected scrollOffset=4 after k in right pane, got %d", m.scrollOffset)
	}
	// clamp at 0
	m.scrollOffset = 0
	m = press(m, key('k'))
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to clamp at 0, got %d", m.scrollOffset)
	}
}

func TestUpdate_ScrollOffset_ResetOnNewOutput(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.scrollOffset = 10
	m.pendingTarget = "some/target"
	m = send(m, cmdResultMsg{output: "fresh output", err: nil})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after cmdResultMsg, got %d", m.scrollOffset)
	}
}

// --- Phase 8: per-target output memory ---

func TestUpdate_TargetOutputMemory_SavesOnNavigate(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.activePane = paneMiddle
	m.output = "result of T1"
	m.cmdErr = ""
	// navigate down from T1 to T2
	m = press(m, key('j'))
	key1 := m.runKey("Domain A", "T1")
	rec, ok := m.targetOutputs[key1]
	if !ok {
		t.Fatal("expected T1 output to be saved on navigate")
	}
	if rec.output != "result of T1" {
		t.Errorf("saved output: got %q, want %q", rec.output, "result of T1")
	}
}

func TestUpdate_TargetOutputMemory_RestoresOnReturn(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.activePane = paneMiddle
	// pre-populate T1's output in the cache
	m.targetOutputs[m.runKey("Domain A", "T1")] = outputRecord{output: "cached T1 output", cmdErr: ""}
	// start on T2 with empty output
	m.targetCursor = 1
	m.output = ""
	// navigate up to T1
	m = press(m, key('k'))
	if m.targetCursor != 0 {
		t.Fatalf("expected targetCursor=0, got %d", m.targetCursor)
	}
	if m.output != "cached T1 output" {
		t.Errorf("expected restored output %q, got %q", "cached T1 output", m.output)
	}
}

func TestUpdate_TargetOutputMemory_EmptyForNewTarget(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.activePane = paneMiddle
	m.output = "some prior output"
	// navigate to T2 which has no cached output
	m = press(m, key('j'))
	if m.output != "" {
		t.Errorf("expected empty output for unvisited target, got %q", m.output)
	}
}

// --- Phase 8: config hot-reload ---

func TestUpdate_HotReload_UpdatesWorkspaces(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.allWorkspaces = []Workspace{
		{Name: "Old", Domains: m.domains},
	}
	m.domainCursor = 1
	m.targetCursor = 1
	m.scrollOffset = 5
	m.showHelp = true

	// Press R — will call LoadWorkspaces() which falls back to defaults since no config file
	m = press(m, key('R'))

	if m.domainCursor != 0 {
		t.Errorf("expected domainCursor reset to 0, got %d", m.domainCursor)
	}
	if m.targetCursor != 0 {
		t.Errorf("expected targetCursor reset to 0, got %d", m.targetCursor)
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset reset to 0, got %d", m.scrollOffset)
	}
	if m.showHelp {
		t.Error("expected showHelp=false after reload")
	}
	if len(m.allWorkspaces) == 0 {
		t.Error("expected allWorkspaces to be populated after reload")
	}
}

// --- Multi-select ---

func testModelWithSelect() Model {
	m := testModel()
	m.selectedTargets = make(map[string]bool)
	m.parallelOutputs = make(map[string]string)
	m.activePane = paneMiddle
	return m
}

func TestUpdate_SpaceTogglesSelection(t *testing.T) {
	m := testModelWithSelect()
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}

	m = press(m, spaceMsg)
	key := m.runKey("Domain A", "T1")
	if !m.selectedTargets[key] {
		t.Error("expected T1 to be selected after Space")
	}

	m = press(m, spaceMsg)
	if m.selectedTargets[key] {
		t.Error("expected T1 to be deselected after second Space")
	}
}

func TestUpdate_SpaceOnlyWorksInMiddlePane(t *testing.T) {
	m := testModelWithSelect()
	m.activePane = paneLeft
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}

	m = press(m, spaceMsg)
	if len(m.selectedTargets) != 0 {
		t.Error("Space should not select when focused on left pane")
	}
}

func TestUpdate_ReloadClearsSelection(t *testing.T) {
	m := testModelWithSelect()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.selectedTargets[m.runKey("Domain A", "T1")] = true

	m = press(m, key('R'))
	if len(m.selectedTargets) != 0 {
		t.Error("expected selectedTargets to be cleared on reload")
	}
}

func TestUpdate_WorkspaceSwitchClearsSelection(t *testing.T) {
	m := testModelWithSelect()
	m.targetOutputs = make(map[string]outputRecord)
	m.allWorkspaces = []Workspace{
		{Name: "WS1", Domains: m.domains},
		{Name: "WS2", Domains: m.domains},
	}
	m.selectedTargets[m.runKey("Domain A", "T1")] = true

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	m = press(m, tabMsg)
	if len(m.selectedTargets) != 0 {
		t.Error("expected selectedTargets to be cleared on workspace switch")
	}
}

func TestUpdate_ParallelRun_CombinesOutput(t *testing.T) {
	m := testModelWithSelect()
	m.running = false

	m = send(m, parallelCmdResultMsg{key: "k1", label: "Alpha", output: "out-a", err: nil})
	if m.running {
		t.Error("should not be done yet — second result pending")
	}

	// Simulate a second in-flight result arriving (set pending manually)
	m.multiRunPending = 1
	m.parallelOutputs["Beta"] = "out-b"
	m = send(m, parallelCmdResultMsg{key: "k2", label: "Alpha", output: "out-a2", err: nil})
	if m.running {
		t.Error("should be done after last result")
	}
	if !strings.Contains(m.output, "=== Alpha ===") {
		t.Errorf("expected combined output header, got: %q", m.output)
	}
}

// --- Clipboard / Save messages ---

func TestUpdate_ClipboardMsg_Success(t *testing.T) {
	m := testModelWithSelect()
	m = send(m, clipboardMsg{err: nil})
	if m.cmdErr != "Copied to clipboard." {
		t.Errorf("unexpected cmdErr: %q", m.cmdErr)
	}
}

func TestUpdate_ClipboardMsg_Error(t *testing.T) {
	m := testModelWithSelect()
	m = send(m, clipboardMsg{err: errors.New("no tool")})
	if !strings.Contains(m.cmdErr, "Copy failed") {
		t.Errorf("expected 'Copy failed' in cmdErr, got: %q", m.cmdErr)
	}
}

func TestUpdate_SaveOutputMsg_Success(t *testing.T) {
	m := testModelWithSelect()
	m = send(m, saveOutputMsg{path: "/home/user/.mt/logs/mt-20260524-120000.txt"})
	if !strings.Contains(m.cmdErr, "Saved") {
		t.Errorf("expected 'Saved' in cmdErr, got: %q", m.cmdErr)
	}
}

func TestUpdate_SaveOutputMsg_Error(t *testing.T) {
	m := testModelWithSelect()
	m = send(m, saveOutputMsg{err: errors.New("disk full")})
	if !strings.Contains(m.cmdErr, "Save failed") {
		t.Errorf("expected 'Save failed' in cmdErr, got: %q", m.cmdErr)
	}
}

// --- Sequence helpers ---

func TestModel_ResolveSequenceTargets(t *testing.T) {
	m := testModel()
	m.selectedTargets = make(map[string]bool)

	steps := m.resolveSequenceTargets([]string{"T3", "T1"})
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(steps))
	}
	if steps[0].Name != "T3" || steps[1].Name != "T1" {
		t.Errorf("unexpected order: %v", []string{steps[0].Name, steps[1].Name})
	}
}

func TestModel_ResolveSequenceTargets_SkipsMissing(t *testing.T) {
	m := testModel()
	m.selectedTargets = make(map[string]bool)

	steps := m.resolveSequenceTargets([]string{"T1", "NonExistent"})
	if len(steps) != 1 || steps[0].Name != "T1" {
		t.Errorf("expected [T1], got %v", steps)
	}
}

func TestModel_FindTargetByRunKey(t *testing.T) {
	m := testModel()
	m.selectedTargets = make(map[string]bool)

	k := m.runKey("Domain A", "T2")
	tgt, ok := m.findTargetByRunKey(k)
	if !ok {
		t.Fatal("expected to find T2")
	}
	if tgt.Name != "T2" {
		t.Errorf("got %q, want T2", tgt.Name)
	}

	_, ok = m.findTargetByRunKey("no/such/key")
	if ok {
		t.Error("expected not found for unknown key")
	}
}

// --- effectiveCmd ---

func TestEffectiveCmd_NoHost(t *testing.T) {
	tgt := Target{Cmd: []string{"echo", "hi"}}
	got := effectiveCmd(tgt)
	if len(got) != 2 || got[0] != "echo" {
		t.Errorf("unexpected cmd: %v", got)
	}
}

func TestEffectiveCmd_WithHost(t *testing.T) {
	tgt := Target{Host: "user@host", Cmd: []string{"./deploy.sh", "prod"}}
	got := effectiveCmd(tgt)
	want := []string{"ssh", "user@host", "./deploy.sh", "prod"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// --- Round 3: crash guards, output truncation, efficiency ---

func TestUpdate_SequenceAdvance_EmptyCmd_ReturnsError(t *testing.T) {
	m := Model{
		running:    true,
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
		seqQueue:   []Target{{Name: "EmptyStep"}}, // no Cmd or Host
	}
	_, cmd := m.Update(cmdResultMsg{output: "step1 done", err: nil})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for sequence advance with empty next step")
	}
	msg := cmd()
	result, ok := msg.(cmdResultMsg)
	if !ok {
		t.Fatalf("expected cmdResultMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("expected error for sequence step with no command")
	}
	if !strings.Contains(result.err.Error(), "EmptyStep") {
		t.Errorf("expected step name in error, got %q", result.err.Error())
	}
}

func TestRunCmd_EmptySlice_ReturnsError(t *testing.T) {
	cmd := runCmd(context.Background(), nil, "")
	msg := cmd()
	result, ok := msg.(cmdResultMsg)
	if !ok {
		t.Fatalf("expected cmdResultMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("expected error for empty command slice")
	}
}

func TestRunCmd_LargeOutput_Truncates(t *testing.T) {
	cmd := runCmd(context.Background(), []string{"sh", "-c", "yes | head -c 1100000"}, "")
	msg := cmd()
	result, ok := msg.(cmdResultMsg)
	if !ok {
		t.Fatalf("expected cmdResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if !strings.Contains(result.output, "truncated") {
		t.Error("expected truncation notice in output for >1 MB command")
	}
}

func TestRunParallelCmd_LargeOutput_Truncates(t *testing.T) {
	cmd := runParallelCmd(context.Background(), "key1", "LargeLabel", []string{"sh", "-c", "yes | head -c 1100000"}, "")
	msg := cmd()
	result, ok := msg.(parallelCmdResultMsg)
	if !ok {
		t.Fatalf("expected parallelCmdResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if !strings.Contains(result.output, "truncated") {
		t.Error("expected truncation notice in output for >1 MB parallel command")
	}
}

func TestSaveTargetOutput_LargeOutput_Truncates(t *testing.T) {
	m := testModel()
	m.targetOutputs = make(map[string]outputRecord)
	m.runStates = make(map[string]runResult)
	m.output = strings.Repeat("x", maxCachedOutputBytes+1000)
	m = m.saveTargetOutput()
	k := m.runKey("Domain A", "T1")
	rec, ok := m.targetOutputs[k]
	if !ok {
		t.Fatal("expected target output to be saved")
	}
	if len(rec.output) > maxCachedOutputBytes+100 {
		t.Errorf("cached output too large: %d bytes (want ≤%d)", len(rec.output), maxCachedOutputBytes+100)
	}
	if !strings.Contains(rec.output, "truncated") {
		t.Error("expected truncation notice in cached output")
	}
}

func TestUpdate_SKey_TriggersCmd(t *testing.T) {
	m := Model{
		output:     "some command output",
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
	}
	_, cmd := m.Update(key('S'))
	if cmd == nil {
		t.Error("expected non-nil cmd when pressing S with output present")
	}
}

func TestUpdate_SKey_NoOutput_IsNoop(t *testing.T) {
	m := Model{
		output:     "",
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
	}
	_, cmd := m.Update(key('S'))
	if cmd != nil {
		t.Error("expected nil cmd when pressing S with no output")
	}
}

func TestUpdate_Quit_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
		ctx:        ctx,
		cancel:     cancel,
	}
	press(m, key('q'))
	select {
	case <-ctx.Done():
		// context was cancelled as expected
	default:
		t.Error("expected context to be cancelled after pressing q")
	}
}
