package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A run is one or more jobs started together: one job for a single target,
// several for a multi-select. Each job runs its steps in order in its own
// goroutine and streams their output into a liveBuffer; the UI redraws from
// the buffers on every outputTickMsg, so output appears while it is produced.

// maxOutputBytes caps the output kept from any one job. Output beyond it is
// discarded with a notice, so a runaway command cannot exhaust memory.
const maxOutputBytes = 1 << 20 // 1 MB

// outputRefresh is how often a running job's output is redrawn.
const outputRefresh = 150 * time.Millisecond

// step is one command a job runs.
type step struct {
	name      string
	cmd       []string
	dir       string
	launchMsg string
}

// job is one target being run: a single command or a sequence of steps.
type job struct {
	key   string // runKey of the target; "" for an AI-proposed command
	label string
	steps []step
	out   *liveBuffer
	done  bool
	err   error
}

// jobDoneMsg reports that job idx of run generation run has finished.
type jobDoneMsg struct {
	run int
	idx int
	err error
}

// outputTickMsg asks Update to redraw the output of run generation run.
type outputTickMsg struct{ run int }

// liveBuffer collects a job's combined stdout and stderr while the UI reads
// it. It keeps the first limit bytes and discards the rest; writes never fail,
// so a command is not killed by a broken pipe once the cap is reached.
type liveBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newLiveBuffer() *liveBuffer { return &liveBuffer{limit: maxOutputBytes} }

func (b *liveBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if room := b.limit - b.buf.Len(); n > room {
		p = p[:max(room, 0)]
		b.truncated = true
	}
	b.buf.Write(p)
	return n, nil
}

// Len returns the number of bytes kept so far.
func (b *liveBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// String returns the output so far, with a notice if some was discarded.
func (b *liveBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.buf.String()
	if b.truncated {
		s += "\n…(output truncated at 1 MB)"
	}
	return s
}

// endLine writes a newline unless the output is empty or already ends in one.
func (b *liveBuffer) endLine() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n := b.buf.Len(); n > 0 && b.buf.Bytes()[n-1] != '\n' {
		b.buf.WriteByte('\n')
	}
}

// runJob runs a job's steps in order, stopping at the first failure.
func runJob(ctx context.Context, run, idx int, j *job) tea.Cmd {
	return func() tea.Msg {
		for i, s := range j.steps {
			if len(j.steps) > 1 {
				if i > 0 {
					j.out.endLine()
					fmt.Fprintln(j.out)
				}
				fmt.Fprintf(j.out, "▶ %s\n", s.name)
			}
			before := j.out.Len()
			if err := runStep(ctx, s, j.out); err != nil {
				if len(j.steps) > 1 {
					err = fmt.Errorf("step %q: %w", s.name, err)
				}
				return jobDoneMsg{run: run, idx: idx, err: err}
			}
			if j.out.Len() == before {
				msg := s.launchMsg
				if msg == "" {
					msg = "(command completed, no output)"
				}
				fmt.Fprintln(j.out, msg)
			}
		}
		return jobDoneMsg{run: run, idx: idx}
	}
}

// runStep runs one command, streaming its combined output into out.
func runStep(ctx context.Context, s step, out *liveBuffer) error {
	c := exec.CommandContext(ctx, s.cmd[0], s.cmd[1:]...) //nolint:gosec
	c.Dir = s.dir
	c.Stdout, c.Stderr = out, out
	// Don't hang on cancellation if a killed command left children holding the pipe.
	c.WaitDelay = 2 * time.Second
	err := c.Run()
	if err != nil && ctx.Err() != nil {
		return errors.New("cancelled")
	}
	return err
}

func outputTick(run int) tea.Cmd {
	return tea.Tick(outputRefresh, func(time.Time) tea.Msg { return outputTickMsg{run: run} })
}

// targetStep turns a target with a command into a job step.
func targetStep(t Target) step {
	s := step{name: t.Name, cmd: effectiveCmd(t), launchMsg: t.LaunchMsg}
	if t.Host == "" {
		s.dir = t.Dir // for SSH targets Dir is applied on the remote side
	}
	return s
}

// stepsFor returns the steps that running t (from domain domainName) takes,
// checking every step before anything starts.
func (m Model) stepsFor(domainName string, t Target) ([]step, error) {
	if len(t.Sequence) == 0 {
		if len(t.Cmd) == 0 {
			return nil, errors.New("no command configured for this target")
		}
		return []step{targetStep(t)}, nil
	}
	steps := make([]step, 0, len(t.Sequence))
	for _, name := range t.Sequence {
		st, ok := m.findStep(domainName, name)
		switch {
		case !ok:
			return nil, fmt.Errorf("sequence %q: step %q not found in this workspace", t.Name, name)
		case len(st.Sequence) > 0:
			return nil, fmt.Errorf("sequence %q: step %q is itself a sequence; sequences can't be nested", t.Name, name)
		case len(st.Cmd) == 0:
			return nil, fmt.Errorf("sequence %q: step %q has no command", t.Name, name)
		}
		steps = append(steps, targetStep(st))
	}
	return steps, nil
}

// findStep finds the target a sequence step names, looking in the sequence's
// own domain first and then in the workspace's other domains in order.
func (m Model) findStep(domainName, name string) (Target, bool) {
	for _, d := range m.domains {
		if d.Name != domainName {
			continue
		}
		for _, t := range d.Targets {
			if t.Name == name {
				return t, true
			}
		}
	}
	for _, d := range m.domains {
		for _, t := range d.Targets {
			if t.Name == name {
				return t, true
			}
		}
	}
	return Target{}, false
}

// startTarget runs target t (a single command or a sequence) from domain domainName.
func (m Model) startTarget(domainName string, t Target) (Model, tea.Cmd) {
	steps, err := m.stepsFor(domainName, t)
	if err != nil {
		m.output, m.cmdErr = "", err.Error()
		return m, nil
	}
	key := m.runKey(domainName, t.Name)
	m.pendingTarget = key
	return m.beginRun([]*job{{key: key, label: t.Name, steps: steps}})
}

// startSelected runs every selected target in parallel. Nothing starts if any
// of them cannot run, and the error names each one.
func (m Model) startSelected() (Model, tea.Cmd) {
	keys := make([]string, 0, len(m.selectedTargets))
	for k := range m.selectedTargets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var jobs []*job
	var problems []string
	for _, key := range keys {
		domainName, t, ok := m.findTargetByRunKey(key)
		if !ok {
			continue
		}
		steps, err := m.stepsFor(domainName, t)
		if err != nil {
			problems = append(problems, t.Name+": "+err.Error())
			continue
		}
		jobs = append(jobs, &job{key: key, label: t.Name, steps: steps})
	}
	switch {
	case len(problems) > 0:
		m.cmdErr = "Can't start the selected targets. " + strings.Join(problems, "; ")
		return m, nil
	case len(jobs) == 0:
		m.cmdErr = "No executable targets selected."
		return m, nil
	}
	m.selectedTargets = make(map[string]bool)
	m.pendingTarget = ""
	return m.beginRun(jobs)
}

// startAICommand runs a command the AI proposed and the user confirmed.
func (m Model) startAICommand(cmd []string) (Model, tea.Cmd) {
	m.pendingTarget = ""
	return m.beginRun([]*job{{label: "AI command", steps: []step{{name: "AI command", cmd: cmd}}}})
}

// beginRun starts jobs under a new cancellable context (Esc cancels it) and
// clears the right pane for their output.
func (m Model) beginRun(jobs []*job) (Model, tea.Cmd) {
	parent := m.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	m.runCancel = cancel
	m.running = true
	m.runSeq++
	m.runStarted = now()
	m.jobs = jobs
	m.output, m.cmdErr, m.scrollOffset = "", "", 0
	m.showHelp = false
	cmds := []tea.Cmd{outputTick(m.runSeq)}
	for i, j := range jobs {
		j.out = newLiveBuffer()
		cmds = append(cmds, runJob(ctx, m.runSeq, i, j))
	}
	return m, tea.Batch(cmds...)
}

// finishRun marks the current run as finished and releases its context.
func (m Model) finishRun() Model {
	if m.runCancel != nil {
		m.runCancel()
	}
	m.running, m.runCancel, m.jobs = false, nil, nil
	return m
}

// jobDone records a finished job and completes the run once all are done.
func (m Model) jobDone(msg jobDoneMsg) Model {
	if msg.run != m.runSeq || msg.idx >= len(m.jobs) {
		return m // from an earlier run
	}
	j := m.jobs[msg.idx]
	j.done, j.err = true, msg.err
	if j.key != "" {
		if msg.err != nil {
			m.runStates[j.key] = runFailure
		} else {
			m.runStates[j.key] = runSuccess
		}
	}
	for _, j := range m.jobs {
		if !j.done {
			return m
		}
	}

	output := m.runOutput()
	cmdErr := ""
	if len(m.jobs) == 1 && m.jobs[0].err != nil {
		cmdErr = m.jobs[0].err.Error()
	}
	key := m.pendingTarget
	m.pendingTarget = ""
	m = m.finishRun()
	// The user may have moved to another target while this one ran: file the
	// result under the target that produced it.
	if cur, _ := m.currentKey(); key != "" && cur != key {
		m.cacheOutput(key, output, cmdErr)
		return m
	}
	m.cmdErr = cmdErr
	return m.showOutput(output)
}

// refreshRunOutput redraws the running jobs' output, unless the user is
// looking at a different target than the one running.
func (m Model) refreshRunOutput() Model {
	if cur, _ := m.currentKey(); m.pendingTarget != "" && cur != m.pendingTarget {
		return m
	}
	return m.showOutput(m.runOutput())
}

// showOutput replaces the right-pane output. If the view was at the bottom
// it stays there, following new output the way a terminal does.
func (m Model) showOutput(output string) Model {
	following := m.scrollOffset >= m.maxScroll()
	m.output = output
	if following {
		m.scrollOffset = m.maxScroll()
	}
	m.scrollOffset = min(m.scrollOffset, m.maxScroll())
	return m
}

// runOutput returns the output of the current run's jobs: a single job's
// output as is, or each job's under a "=== name ===" header.
func (m Model) runOutput() string {
	if len(m.jobs) == 1 {
		return strings.TrimSpace(m.jobs[0].out.String())
	}
	parts := make([]string, len(m.jobs))
	for i, j := range m.jobs {
		s := "=== " + j.label + " ===\n" + strings.TrimSpace(j.out.String())
		switch {
		case j.err != nil:
			s += "\n[ERROR: " + j.err.Error() + "]"
		case !j.done:
			s += "\n…"
		}
		parts[i] = s
	}
	return strings.Join(parts, "\n\n")
}

// runBanner describes the run in progress, e.g. "Running… 12s  ⎋ stop".
func (m Model) runBanner() string {
	what := "Running…"
	if len(m.jobs) > 1 {
		done := 0
		for _, j := range m.jobs {
			if j.done {
				done++
			}
		}
		what = fmt.Sprintf("Running %d targets (%d done)…", len(m.jobs), done)
	} else if cur, _ := m.currentKey(); m.pendingTarget != "" && cur != m.pendingTarget && len(m.jobs) == 1 {
		what = "Running " + m.jobs[0].label + "…"
	}
	return fmt.Sprintf("%s %s   ⎋ stop", what, now().Sub(m.runStarted).Round(time.Second))
}

// effectiveCmd returns the command to run for a target. SSH targets run as
// `ssh -o BatchMode=yes host <command>`: BatchMode makes ssh fail at once
// instead of waiting for a password prompt mt cannot show. A target's Dir is
// applied on the remote side with cd.
func effectiveCmd(t Target) []string {
	if t.Host == "" || len(t.Cmd) == 0 {
		return t.Cmd
	}
	ssh := []string{"ssh", "-o", "BatchMode=yes", t.Host}
	if t.Dir == "" {
		return append(ssh, t.Cmd...)
	}
	return append(ssh, "cd "+remoteDir(t.Dir)+" && "+strings.Join(t.Cmd, " "))
}

// remoteDir quotes dir for the remote shell, leaving a leading ~/ unquoted so
// it still expands to the remote home directory.
func remoteDir(dir string) string {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		return "~/" + shellQuote(rest)
	}
	if dir == "~" {
		return "~"
	}
	return shellQuote(dir)
}

// shellQuote wraps s in single quotes for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
