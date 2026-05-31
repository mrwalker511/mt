package tui

import (
	"context"
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

// cmdResultMsg carries the result of an async command execution back to Update.
type cmdResultMsg struct {
	output string
	err    error
}

// parallelCmdResultMsg carries the result of one command in a parallel multi-select run.
type parallelCmdResultMsg struct {
	key    string // runKey of the target
	label  string // display name for combined output header
	output string
	err    error
}

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

// tickMsg fires on each polling interval.
type tickMsg time.Time

// statusUpdateMsg carries a single live-status value back to Update.
type statusUpdateMsg struct {
	key    string
	status string
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

	domains       []Domain    // active workspace's domains
	allWorkspaces []Workspace
	workspaceIdx  int

	quitting bool

	output  string // last command output to display in right pane
	cmdErr  string // last command error message
	running bool   // true while a command is executing

	liveStatus    map[string]string    // live probe results keyed by semantic name
	runStates     map[string]runResult // last run outcome keyed by runKey()
	pendingTarget string               // key of the target currently executing

	targetOutputs map[string]outputRecord // cached output per target, keyed by runKey()
	scrollOffset  int                     // first visible line in right-pane output

	showHelp bool // true when the ? help overlay is shown in the right pane

	// multi-select (Space to toggle, Enter runs all in parallel)
	selectedTargets map[string]bool   // set of runKeys checked for parallel execution
	parallelOutputs map[string]string // per-label output accumulator during a parallel run
	multiRunPending int               // number of parallel commands still in flight

	// sequence execution (target.Sequence drives steps in order)
	seqQueue  []Target // remaining steps not yet started
	seqOutput string   // output accumulated from completed sequence steps

	// ctx is cancelled when the user quits, terminating all in-flight commands.
	ctx    context.Context
	cancel context.CancelFunc

	// AI natural-language input (/ key)
	llmConfig  llm.Config
	inputMode  bool               // true while the / input overlay is active
	inputBuf   string             // characters typed so far
	llmPending bool               // LLM request in-flight
	llmCancel  context.CancelFunc // cancels the in-flight LLM request
}

// hasGitDomain reports whether the active workspace has a Context/Git domain.
func (m Model) hasGitDomain() bool {
	for _, d := range m.domains {
		if d.Name == "Context/Git" {
			return true
		}
	}
	return false
}

// hasDockerDomain reports whether the active workspace has an Infrastructure domain.
func (m Model) hasDockerDomain() bool {
	for _, d := range m.domains {
		if d.Name == "Infrastructure" {
			return true
		}
	}
	return false
}

// runKey returns a unique key for a target, scoped to the current workspace name.
// When the workspace has no name (default single-workspace), the key is
// "DomainName/TargetName" — identical to the pre-workspace format.
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

// rightPanePageSize returns the number of content lines visible in the right pane.
func (m Model) rightPanePageSize() int {
	size := m.height - statusBarHeight - borderHeight - 3
	if size < 1 {
		return 1
	}
	return size
}

// saveTargetOutput stores the current output/cmdErr under the active target's key.
// Output is capped at maxCachedOutputBytes to bound per-target memory use.
func (m Model) saveTargetOutput() Model {
	if m.domainCursor < len(m.domains) {
		targets := m.domains[m.domainCursor].Targets
		if m.targetCursor < len(targets) {
			key := m.runKey(m.domains[m.domainCursor].Name, targets[m.targetCursor].Name)
			out := m.output
			if len(out) > maxCachedOutputBytes {
				out = out[:maxCachedOutputBytes] + "\n…(cached output truncated)"
			}
			m.targetOutputs[key] = outputRecord{out, m.cmdErr}
		}
	}
	return m
}

// restoreTargetOutput loads cached output for the newly active target, or clears if none.
func (m Model) restoreTargetOutput() Model {
	m.output, m.cmdErr, m.scrollOffset = "", "", 0
	if m.domainCursor < len(m.domains) {
		targets := m.domains[m.domainCursor].Targets
		if m.targetCursor < len(targets) {
			key := m.runKey(m.domains[m.domainCursor].Name, targets[m.targetCursor].Name)
			if rec, ok := m.targetOutputs[key]; ok {
				m.output, m.cmdErr = rec.output, rec.cmdErr
			}
		}
	}
	return m
}

// New returns a freshly initialized Model. Workspaces are loaded from the user's
// config file if one exists; otherwise the built-in defaults are used.
func New() Model {
	workspaces, llmCfg, err := LoadWorkspaces()
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
		parallelOutputs: make(map[string]string),
		ctx:             ctx,
		cancel:          cancel,
		llmConfig:       llmCfg,
	}
	if err != nil {
		m.cmdErr = "Config error — using built-in defaults."
	}
	return m
}

// findTargetByRunKey searches the active workspace for a target matching key.
func (m Model) findTargetByRunKey(key string) (Target, bool) {
	for _, d := range m.domains {
		for _, t := range d.Targets {
			if m.runKey(d.Name, t.Name) == key {
				return t, true
			}
		}
	}
	return Target{}, false
}

// resolveSequenceTargets returns the Target structs for each name in order,
// searching all domains of the active workspace.
func (m Model) resolveSequenceTargets(names []string) []Target {
	lookup := make(map[string]Target)
	for _, d := range m.domains {
		for _, t := range d.Targets {
			lookup[t.Name] = t
		}
	}
	out := make([]Target, 0, len(names))
	for _, n := range names {
		if t, ok := lookup[n]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Init fires the initial status polls and starts the refresh tick.
// Git and Docker polls are skipped when the active workspace has no relevant domains.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if m.hasGitDomain() {
		cmds = append(cmds, pollGit(m.ctx))
	}
	if m.hasDockerDomain() {
		cmds = append(cmds, pollDocker(m.ctx))
	}
	return tea.Batch(cmds...)
}

