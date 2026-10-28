// Package tui implements mt's three-pane Bubble Tea interface: config loading,
// target execution (single, sequence, parallel, SSH), live status polling and
// the AI prompt bar.
package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mrwalker511/mt/internal/llm"
)

// maxCachedOutputBytes caps the size of a single entry stored in targetOutputs.
// Prevents the per-target output cache from growing unbounded across many runs.
const maxCachedOutputBytes = 256 * 1024 // 256 KB

type pane int

const (
	paneLeft   pane = 0
	paneMiddle pane = 1
	paneRight  pane = 2
)

// llmResponseMsg carries the result of an async LLM query back to Update.
type llmResponseMsg struct {
	response string
	err      error
}

// clipboardMsg reports the outcome of a copy-to-clipboard operation.
type clipboardMsg struct{ err error }

// saveOutputMsg reports the outcome of saving output to a file.
type saveOutputMsg struct {
	path string
	err  error
}

// runResult records the outcome of the last execution of a target.
type runResult int

const (
	runNone    runResult = 0
	runSuccess runResult = 1
	runFailure runResult = 2
)

// outputRecord stores the output and error text of a previously executed target.
type outputRecord struct {
	output string
	cmdErr string
}

// Model is the single source of truth for the entire TUI.
type Model struct {
	width  int
	height int

	activePane   pane
	domainCursor int
	targetCursor int
	domainTop    int // first visible row of the domain list
	targetTop    int // first visible row of the target list

	domains       []Domain // active workspace's domains
	allWorkspaces []Workspace
	workspaceIdx  int

	quitting bool

	output  string // output shown in the right pane (live while a run streams)
	cmdErr  string // last command error message
	running bool   // true while a run is in progress

	liveStatus    map[string]string    // live: command output keyed by domain runKey
	liveInFlight  map[string]bool      // live: commands currently running
	runStates     map[string]runResult // last run outcome keyed by runKey()
	pendingTarget string               // key of the single target running; "" for multi or AI runs

	targetOutputs map[string]outputRecord // cached output per target, keyed by runKey()
	scrollOffset  int                     // first visible line in right-pane output

	showHelp bool // true when the ? help overlay is shown in the right pane

	// multi-select (Space to toggle, Enter runs all in parallel)
	selectedTargets map[string]bool // set of runKeys checked for parallel execution

	// the current run (see run.go)
	jobs       []*job
	runSeq     int                // generation of the current run; stale messages are ignored
	runStarted time.Time          // when the current run began, for the elapsed time
	runCancel  context.CancelFunc // stops the current run's commands (Esc)

	// ctx is cancelled when the user quits, terminating all in-flight commands.
	ctx    context.Context
	cancel context.CancelFunc

	// AI natural-language input (/ key)
	llmConfig  llm.Config
	inputMode  bool               // true while the / input overlay is active
	inputBuf   string             // characters typed so far
	llmPending bool               // LLM request in-flight
	llmCancel  context.CancelFunc // cancels the in-flight LLM request
	confirmCmd []string           // non-nil when an AI-generated CMD awaits confirmation

	// double-click detection for trackpad/mouse clicks on targets
	lastClickKey string
	lastClickAt  time.Time
}

// runKey returns a unique key for a target, scoped to the current workspace name.
// When the workspace has no name (default single-workspace), the key is
// "DomainName/TargetName", identical to the pre-workspace format.
func (m Model) runKey(domainName, targetName string) string {
	ws := ""
	if m.workspaceIdx < len(m.allWorkspaces) {
		ws = m.allWorkspaces[m.workspaceIdx].Name
	}
	if ws == "" {
		return domainName + "/" + targetName
	}
	return ws + "/" + domainName + "/" + targetName
}

// currentKey returns the runKey of the selected target, or false if there is none.
func (m Model) currentKey() (string, bool) {
	t, ok := m.currentTarget()
	if !ok {
		return "", false
	}
	return m.runKey(m.domains[m.domainCursor].Name, t.Name), true
}

// rightPanePageSize returns the number of output lines visible in the right pane:
// the pane height minus the title (2 lines), the scroll indicator (1 line), the
// live-status header (its lines plus a blank, when shown), and the running
// banner and trailing error (2 lines each, when shown).
func (m Model) rightPanePageSize() int {
	size := m.height - statusBarHeight - borderHeight - 3
	if s := m.liveStatusText(); s != "" {
		size -= strings.Count(s, "\n") + 2
	}
	if m.running {
		size -= 2
	}
	if m.cmdErr != "" {
		size -= 2
	}
	return max(size, 1)
}

// maxScroll returns the largest useful scroll offset for the current output.
func (m Model) maxScroll() int {
	return max(strings.Count(m.output, "\n")+1-m.rightPanePageSize(), 0)
}

// cacheOutput stores output/cmdErr under key, capping output at maxCachedOutputBytes
// to bound per-target memory use.
func (m Model) cacheOutput(key, output, cmdErr string) {
	if len(output) > maxCachedOutputBytes {
		output = output[:maxCachedOutputBytes] + "\n…(cached output truncated)"
	}
	m.targetOutputs[key] = outputRecord{output, cmdErr}
}

// saveTargetOutput stores the current output/cmdErr under the active target's key.
func (m Model) saveTargetOutput() Model {
	if key, ok := m.currentKey(); ok {
		m.cacheOutput(key, m.output, m.cmdErr)
	}
	return m
}

// restoreTargetOutput loads cached output for the newly active target, or clears if none.
func (m Model) restoreTargetOutput() Model {
	m.output, m.cmdErr, m.scrollOffset = "", "", 0
	if key, ok := m.currentKey(); ok {
		if rec, ok := m.targetOutputs[key]; ok {
			m.output, m.cmdErr = rec.output, rec.cmdErr
		}
	}
	return m
}

// New returns a freshly initialized Model. Workspaces are loaded from the user's
// config file if one exists; otherwise the built-in defaults are used.
func New() Model {
	r, err := loadConfigFiles()
	workspaces, llmCfg := r.workspaces, r.llm
	var domains []Domain
	if len(workspaces) > 0 {
		domains = workspaces[0].Domains
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		allWorkspaces:   workspaces,
		workspaceIdx:    0,
		domains:         domains,
		activePane:      paneLeft,
		liveStatus:      make(map[string]string),
		runStates:       make(map[string]runResult),
		targetOutputs:   make(map[string]outputRecord),
		selectedTargets: make(map[string]bool),
		ctx:             ctx,
		cancel:          cancel,
		llmConfig:       llmCfg,
	}
	switch {
	case err != nil:
		m.cmdErr = "Config error (using built-in defaults): " + err.Error()
	case r.notice != "":
		m.cmdErr = r.notice
	}
	return m
}

// findTargetByRunKey searches the active workspace for a target matching key
// and returns it with its domain's name.
func (m Model) findTargetByRunKey(key string) (string, Target, bool) {
	for _, d := range m.domains {
		for _, t := range d.Targets {
			if m.runKey(d.Name, t.Name) == key {
				return d.Name, t, true
			}
		}
	}
	return "", Target{}, false
}

// Init fetches the first domain's live status and starts the polling tick.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if key, cmd := m.liveKey(); key != "" {
		cmds = append(cmds, probeLive(m.ctx, key, cmd))
	}
	return tea.Batch(cmds...)
}
