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

// Model is the single source of truth for the entire TUI.
type Model struct {
	width  int
	height int

	activePane   pane
	domainCursor int
	targetCursor int

	domains  []Domain
	quitting bool

	output  string // last command output to display in right pane
	cmdErr  string // last command error message
	running bool   // true while a command is executing

	liveStatus    map[string]string    // live probe results keyed by semantic name
	runStates     map[string]runResult // last run outcome keyed by "DomainName/TargetName"
	pendingTarget string               // key of the target currently executing
}

// New returns a freshly initialized Model. Domains are loaded from the user's
// config file if one exists; otherwise the built-in defaults are used.
func New() Model {
	domains, err := LoadDomains()
	m := Model{
		domains:    domains,
		activePane: paneLeft,
		liveStatus: make(map[string]string),
		runStates:  make(map[string]runResult),
	}
	if err != nil {
		m.output = "Config error: " + err.Error()
		m.cmdErr = "Using built-in defaults."
	}
	return m
}

// Init fires the initial status polls and starts the refresh tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), pollGit(), pollDocker())
}

