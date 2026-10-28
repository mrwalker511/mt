package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// drive executes cmd and every command that follows from it, feeding each
// message back into Update, until done reports true. It is how these tests
// run real commands through the job runner.
func drive(t *testing.T, m Model, cmd tea.Cmd, done func(Model) bool) Model {
	t.Helper()
	msgs := make(chan tea.Msg, 256)
	start := func(c tea.Cmd) {
		if c != nil {
			go func() { msgs <- c() }()
		}
	}
	start(cmd)
	deadline := time.After(10 * time.Second)
	for !done(m) {
		select {
		case msg := <-msgs:
			switch msg := msg.(type) {
			case nil:
			case tea.BatchMsg:
				for _, c := range msg {
					start(c)
				}
			default:
				next, c := m.Update(msg)
				m = next.(Model)
				start(c)
			}
		case <-deadline:
			t.Fatal("timed out waiting for the run to finish")
		}
	}
	return m
}

func notRunning(m Model) bool { return !m.running }

// runModel returns a model with one domain "D" holding the given targets.
func runModel(targets ...Target) Model {
	m := testModel()
	m.domains = []Domain{{Name: "D", Targets: targets}}
	m.selectedTargets = make(map[string]bool)
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.width, m.height = 100, 30
	return m
}

// runFirst presses Enter on the first target and waits for the run to end.
func runFirst(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return drive(t, next.(Model), cmd, notRunning)
}

func TestRun_Success(t *testing.T) {
	m := runFirst(t, runModel(Target{Name: "T", Cmd: []string{"echo", "  hello world  "}}))
	if m.output != "hello world" || m.cmdErr != "" {
		t.Errorf("output=%q err=%q", m.output, m.cmdErr)
	}
	if m.runStates["D/T"] != runSuccess || m.pendingTarget != "" {
		t.Errorf("expected success badge and cleared pendingTarget, got %v %q", m.runStates["D/T"], m.pendingTarget)
	}
}

func TestRun_Failure(t *testing.T) {
	m := runFirst(t, runModel(Target{Name: "T", Cmd: []string{"sh", "-c", "echo partial output; exit 1"}}))
	if m.output != "partial output" || !strings.Contains(m.cmdErr, "exit status 1") {
		t.Errorf("output=%q err=%q", m.output, m.cmdErr)
	}
	if m.runStates["D/T"] != runFailure {
		t.Error("expected failure badge")
	}
}

func TestRun_NoOutputShowsLaunchMessage(t *testing.T) {
	m := runFirst(t, runModel(Target{Name: "T", Cmd: []string{"true"}, LaunchMsg: "Opening…"}))
	if m.output != "Opening…" {
		t.Errorf("output=%q", m.output)
	}
	m = runFirst(t, runModel(Target{Name: "T", Cmd: []string{"true"}}))
	if m.output != "(command completed, no output)" {
		t.Errorf("output=%q", m.output)
	}
}

func TestRun_OutputStreamsWhileRunning(t *testing.T) {
	m := runModel(Target{Name: "T", Cmd: []string{"sh", "-c", "echo first; sleep 2; echo second"}})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, next.(Model), cmd, func(m Model) bool { return strings.Contains(m.output, "first") })
	if !m.running {
		t.Fatal("output should appear before the command finishes")
	}
	if !strings.Contains(m.View(), "Running…") {
		t.Error("expected the running banner while output streams")
	}
	m.runCancel()
}

func TestRun_FollowsOutputUnlessScrolledUp(t *testing.T) {
	m := runModel(Target{Name: "T", Cmd: []string{"sh", "-c", "seq 1 200"}})
	m = runFirst(t, m)
	if m.scrollOffset != m.maxScroll() || m.maxScroll() == 0 {
		t.Errorf("expected the view to follow to the bottom, offset %d of %d", m.scrollOffset, m.maxScroll())
	}
	m.scrollOffset = 3
	m = m.showOutput(m.output + "\nmore")
	if m.scrollOffset != 3 {
		t.Errorf("scrolled-up view must stay put, got offset %d", m.scrollOffset)
	}
}

func TestRun_LargeOutputTruncated(t *testing.T) {
	m := runFirst(t, runModel(Target{Name: "T", Cmd: []string{"sh", "-c", "yes | head -c 1100000"}}))
	if len(m.output) > maxOutputBytes+100 || !strings.Contains(m.output, "truncated") {
		t.Errorf("expected capped output with notice, got %d bytes", len(m.output))
	}
}

func TestRun_ResultCachedUnderRunningTarget(t *testing.T) {
	m := runModel(Target{Name: "T1", Cmd: []string{"echo", "t1 output"}}, Target{Name: "T2"})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.activePane = paneMiddle
	m = press(m, tea.KeyMsg{Type: tea.KeyDown}) // move to T2 while T1 runs
	m = drive(t, m, cmd, notRunning)
	if m.output != "" {
		t.Errorf("T1's output must not show on T2, got %q", m.output)
	}
	if rec := m.targetOutputs["D/T1"]; rec.output != "t1 output" {
		t.Errorf("expected T1 output cached, got %q", rec.output)
	}
}

func TestRun_EscCancels(t *testing.T) {
	m := runModel(Target{Name: "T", Cmd: []string{"sleep", "30"}})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = press(next.(Model), tea.KeyMsg{Type: tea.KeyEsc})
	start := time.Now()
	m = drive(t, m, cmd, notRunning)
	if time.Since(start) > 5*time.Second || m.cmdErr != "cancelled" {
		t.Errorf("expected a prompt cancel, got err=%q after %s", m.cmdErr, time.Since(start))
	}
}

func TestRun_SequenceRunsStepsInOrder(t *testing.T) {
	m := runModel(
		Target{Name: "Pipeline", Sequence: []string{"Build", "Test"}},
		Target{Name: "Build", Cmd: []string{"echo", "built"}},
		Target{Name: "Test", Cmd: []string{"echo", "tested"}},
	)
	m = runFirst(t, m)
	b, ts := strings.Index(m.output, "built"), strings.Index(m.output, "tested")
	if b < 0 || ts < b || !strings.Contains(m.output, "▶ Build") || m.cmdErr != "" {
		t.Errorf("unexpected sequence output %q err=%q", m.output, m.cmdErr)
	}
}

func TestRun_SequenceStopsAtFailure(t *testing.T) {
	m := runModel(
		Target{Name: "Pipeline", Sequence: []string{"Build", "Deploy"}},
		Target{Name: "Build", Cmd: []string{"false"}},
		Target{Name: "Deploy", Cmd: []string{"echo", "DEPLOYED"}},
	)
	m = runFirst(t, m)
	if strings.Contains(m.output, "DEPLOYED") || !strings.Contains(m.cmdErr, `step "Build"`) {
		t.Errorf("deploy must not run after a failed build: output=%q err=%q", m.output, m.cmdErr)
	}
}

func TestRun_SequenceValidatedBeforeStarting(t *testing.T) {
	cases := map[string][]Target{
		`step "Tset" not found`:  {{Name: "P", Sequence: []string{"Build", "Tset"}}, {Name: "Build", Cmd: []string{"true"}}},
		`"Empty" has no command`: {{Name: "P", Sequence: []string{"Empty"}}, {Name: "Empty"}},
		"can't be nested":        {{Name: "P", Sequence: []string{"Q"}}, {Name: "Q", Sequence: []string{"P"}}},
	}
	for want, targets := range cases {
		next, cmd := runModel(targets...).Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := next.(Model)
		if cmd != nil || m.running || !strings.Contains(m.cmdErr, want) {
			t.Errorf("want error %q and nothing started, got running=%v err=%q", want, m.running, m.cmdErr)
		}
	}
}

func TestRun_SequencePrefersStepInOwnDomain(t *testing.T) {
	m := runModel()
	m.domains = []Domain{
		{Name: "Other", Targets: []Target{{Name: "Build", Cmd: []string{"echo", "WRONG"}}}},
		{Name: "Mine", Targets: []Target{
			{Name: "P", Sequence: []string{"Build"}},
			{Name: "Build", Cmd: []string{"echo", "RIGHT"}},
		}},
	}
	m.domainCursor = 1
	m = runFirst(t, m)
	if !strings.Contains(m.output, "RIGHT") || strings.Contains(m.output, "WRONG") {
		t.Errorf("expected the step from the sequence's own domain, got %q", m.output)
	}
}

func TestRun_ParallelIncludesSequencesAndSameNames(t *testing.T) {
	m := runModel()
	m.domains = []Domain{
		{Name: "A", Targets: []Target{{Name: "Open", Cmd: []string{"echo", "from A"}}}},
		{Name: "B", Targets: []Target{
			{Name: "Open", Cmd: []string{"echo", "from B"}},
			{Name: "Both", Sequence: []string{"Open"}},
		}},
	}
	m.selectedTargets = map[string]bool{"A/Open": true, "B/Open": true, "B/Both": true}
	m = runFirst(t, m)
	for _, want := range []string{"=== Open ===\nfrom A", "=== Open ===\nfrom B", "=== Both ==="} {
		if !strings.Contains(m.output, want) {
			t.Errorf("parallel output missing %q:\n%s", want, m.output)
		}
	}
	for _, k := range []string{"A/Open", "B/Open", "B/Both"} {
		if m.runStates[k] != runSuccess {
			t.Errorf("expected success badge on %s", k)
		}
	}
	if len(m.selectedTargets) != 0 {
		t.Error("selection should be cleared after the run")
	}
}

func TestRun_ParallelRefusesInvalidSelection(t *testing.T) {
	m := runModel(Target{Name: "Good", Cmd: []string{"true"}}, Target{Name: "Hint"})
	m.selectedTargets = map[string]bool{"D/Good": true, "D/Hint": true}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil || m.running || !strings.Contains(m.cmdErr, "Hint") {
		t.Errorf("expected refusal naming Hint, got running=%v err=%q", m.running, m.cmdErr)
	}
	if len(m.selectedTargets) != 2 {
		t.Error("selection should be kept so the user can fix it")
	}
}

func TestRun_AICommandStreams(t *testing.T) {
	m := runModel()
	m.confirmCmd = []string{"echo", "from the AI"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, next.(Model), cmd, notRunning)
	if m.output != "from the AI" {
		t.Errorf("output=%q", m.output)
	}
}

func TestRun_StaleJobMessageIgnored(t *testing.T) {
	m := runModel(Target{Name: "T", Cmd: []string{"sleep", "30"}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	defer m.runCancel()
	m = send(m, jobDoneMsg{run: m.runSeq - 1, idx: 0})
	if !m.running {
		t.Error("a message from an earlier run must not end the current one")
	}
}

func TestRun_DirSetsWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	real, _ := filepath.EvalSymlinks(dir)
	m := runFirst(t, runModel(Target{Name: "T", Cmd: []string{"pwd"}, Dir: dir}))
	if m.output != real && m.output != dir {
		t.Errorf("pwd = %q, want %q", m.output, dir)
	}
}

func TestLoadWorkspaces_DirExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws, err := loadConfig(t, "domains:\n  - name: D\n    targets:\n      - name: T\n        run: pwd\n        dir: ~/proj\n      - name: R\n        host: me@box\n        run: pwd\n        dir: ~/proj\n")
	if err != nil {
		t.Fatal(err)
	}
	ts := ws[0].Domains[0].Targets
	if ts[0].Dir != filepath.Join(home, "proj") {
		t.Errorf("local dir = %q", ts[0].Dir)
	}
	if ts[1].Dir != "~/proj" {
		t.Errorf("SSH dir must be left for the remote shell, got %q", ts[1].Dir)
	}
}

func TestEffectiveCmd_SSHWithDir(t *testing.T) {
	got := effectiveCmd(Target{Host: "me@box", Dir: "~/my app", Cmd: []string{"git status"}})
	want := []string{"ssh", "-o", "BatchMode=yes", "me@box", "cd ~/'my app' && git status"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
	got = effectiveCmd(Target{Host: "me@box", Dir: "/srv/it's", Cmd: []string{"ls"}})
	if last := got[len(got)-1]; last != `cd '/srv/it'\''s' && ls` {
		t.Errorf("quoting: got %q", last)
	}
}

func TestLive_PollsOnlySelectedDomain(t *testing.T) {
	m := runModel()
	m.domains = []Domain{
		{Name: "Quiet"},
		{Name: "Git", Live: []string{"echo", "Branch: main"}},
	}
	// Selecting the domain fetches its status right away.
	m.activePane = paneLeft
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = drive(t, next.(Model), cmd, func(m Model) bool { return m.liveStatusText() != "" })
	if m.liveStatusText() != "Branch: main" || !strings.Contains(m.View(), "Branch: main") {
		t.Errorf("live header = %q", m.liveStatusText())
	}
	// A tick for a domain without live: starts nothing.
	m = press(m, tea.KeyMsg{Type: tea.KeyUp})
	if _, poll := m.pollLive(); poll != nil {
		t.Error("a domain without live: must not be polled")
	}
	if m.domainLiveHeader() != "" {
		t.Error("no header for a domain without live:")
	}
}

func TestLive_NoOverlappingPolls(t *testing.T) {
	m := runModel()
	m.domains = []Domain{{Name: "Slow", Live: []string{"sleep", "1"}}}
	m, first := m.pollLive()
	if first == nil {
		t.Fatal("expected a poll")
	}
	if _, second := m.pollLive(); second != nil {
		t.Error("a second poll must not start while the first is running")
	}
}

func TestLive_FailureHidesHeaderAndLinesAreCapped(t *testing.T) {
	msg := probeLive(context.Background(), "k", []string{"false"})()
	if msg.(liveStatusMsg).status != "" {
		t.Error("a failing live command should show nothing")
	}
	msg = probeLive(context.Background(), "k", []string{"printf", "a\\nb\\nc\\nd\\n"})()
	if got := msg.(liveStatusMsg).status; got != "a\nb\nc" {
		t.Errorf("expected first 3 lines, got %q", got)
	}
}

func TestLive_DefaultGitHeader(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                        //nolint:errcheck
	var live []string
	for _, d := range initialDomains {
		if d.Name == "Context/Git" {
			live = d.Live
		}
	}
	if msg := probeLive(context.Background(), "k", live)(); msg.(liveStatusMsg).status != "" {
		t.Errorf("outside a repo the git header should be hidden, got %q", msg.(liveStatusMsg).status)
	}
}

func TestLoadWorkspaces_LiveField(t *testing.T) {
	ws, err := loadConfig(t, "domains:\n  - name: D\n    live: git status --short --branch\n    targets:\n      - name: T\n        run: echo\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ws[0].Domains[0].Live, " "); got != "git status --short --branch" {
		t.Errorf("live = %q", got)
	}
	_, err = loadConfig(t, "domains:\n  - name: D\n    live: git log | head\n    targets:\n      - name: T\n        run: echo\n")
	if err == nil || !strings.Contains(err.Error(), `domain "D": live:`) {
		t.Errorf("expected live: shell-syntax error naming the domain, got %v", err)
	}
}

func TestLineWindow(t *testing.T) {
	s := "a\nb\nc\nd\ne"
	cases := []struct {
		off, n int
		want   string
	}{{0, 2, "a\nb"}, {2, 2, "c\nd"}, {3, 5, "d\ne"}, {4, 1, "e"}, {9, 1, ""}}
	for _, c := range cases {
		if got := lineWindow(s, c.off, c.n); got != c.want {
			t.Errorf("lineWindow(%d,%d) = %q, want %q", c.off, c.n, got, c.want)
		}
	}
}
