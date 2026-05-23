package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
func (m Model) saveTargetOutput() Model {
	if m.domainCursor < len(m.domains) {
		targets := m.domains[m.domainCursor].Targets
		if m.targetCursor < len(targets) {
			key := m.runKey(m.domains[m.domainCursor].Name, targets[m.targetCursor].Name)
			m.targetOutputs[key] = outputRecord{m.output, m.cmdErr}
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
	workspaces, err := LoadWorkspaces()
	var domains []Domain
	if len(workspaces) > 0 {
		domains = workspaces[0].Domains
	}
	m := Model{
		allWorkspaces: workspaces,
		workspaceIdx:  0,
		domains:       domains,
		activePane:    paneLeft,
		liveStatus:    make(map[string]string),
		runStates:     make(map[string]runResult),
		targetOutputs: make(map[string]outputRecord),
	}
	if err != nil {
		m.cmdErr = "Config error — using built-in defaults."
	}
	return m
}

// Init fires the initial status polls and starts the refresh tick.
// Git and Docker polls are skipped when the active workspace has no relevant domains.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if m.hasGitDomain() {
		cmds = append(cmds, pollGit())
	}
	if m.hasDockerDomain() {
		cmds = append(cmds, pollDocker())
	}
	return tea.Batch(cmds...)
}

