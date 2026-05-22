package tui

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const pollInterval = 5 * time.Second

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(tickCmd(), pollGit(), pollDocker())

	case statusUpdateMsg:
		m.liveStatus[msg.key] = msg.status
		return m, nil

	case cmdResultMsg:
		m.running = false
		m.output = strings.TrimSpace(msg.output)
		if msg.err != nil {
			m.cmdErr = msg.err.Error()
		} else {
			m.cmdErr = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.running {
				return m, nil
			}
			target, ok := m.currentTarget()
			if !ok || len(target.Cmd) == 0 {
				m.output = ""
				m.cmdErr = "No command configured for this target."
				return m, nil
			}
			m.running = true
			m.output = ""
			m.cmdErr = ""
			return m, runCmd(target.Cmd, target.LaunchMsg)

		case "left", "h":
			if m.activePane > paneLeft {
				m.activePane--
			}

		case "right", "l":
			if m.activePane < paneRight {
				m.activePane++
			}

		case "up", "k":
			switch m.activePane {
			case paneLeft:
				if m.domainCursor > 0 {
					m.domainCursor--
					m.targetCursor = 0
					m.output, m.cmdErr = "", ""
				}
			case paneMiddle:
				if m.targetCursor > 0 {
					m.targetCursor--
					m.output, m.cmdErr = "", ""
				}
			}

		case "down", "j":
			switch m.activePane {
			case paneLeft:
				if m.domainCursor < len(m.domains)-1 {
					m.domainCursor++
					m.targetCursor = 0
					m.output, m.cmdErr = "", ""
				}
			case paneMiddle:
				targets := m.domains[m.domainCursor].Targets
				if m.targetCursor < len(targets)-1 {
					m.targetCursor++
					m.output, m.cmdErr = "", ""
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// currentTarget returns the currently selected target and whether it exists.
func (m Model) currentTarget() (Target, bool) {
	if m.domainCursor >= len(m.domains) {
		return Target{}, false
	}
	targets := m.domains[m.domainCursor].Targets
	if m.targetCursor >= len(targets) {
		return Target{}, false
	}
	return targets[m.targetCursor], true
}

// tickCmd schedules the next status poll cycle.
func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// pollGit batches all git probes.
func pollGit() tea.Cmd {
	return tea.Batch(pollGitBranch(), pollGitDirty())
}

func pollGitBranch() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output() //nolint:gosec
		if err != nil {
			return statusUpdateMsg{key: "git.branch", status: ""}
		}
		return statusUpdateMsg{key: "git.branch", status: strings.TrimSpace(string(out))}
	}
}

func pollGitDirty() tea.Cmd {
	return func() tea.Msg {
		out, _ := exec.Command("git", "status", "--porcelain").Output() //nolint:gosec
		count := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		status := ""
		if count > 0 {
			status = strconv.Itoa(count) + " modified"
		}
		return statusUpdateMsg{key: "git.dirty", status: status}
	}
}

// pollDocker batches all docker container probes.
func pollDocker() tea.Cmd {
	return tea.Batch(
		pollDockerContainer("postgres"),
		pollDockerContainer("redis"),
	)
}

func pollDockerContainer(name string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("docker", "ps", "--filter", "name="+name, "--format", "{{.Status}}").Output() //nolint:gosec
		status := "stopped"
		if err == nil {
			if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
				status = trimmed
			}
		}
		return statusUpdateMsg{key: "docker." + name, status: status}
	}
}

// runCmd executes cmd asynchronously and returns the result as a cmdResultMsg.
func runCmd(cmd []string, launchMsg string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command(cmd[0], cmd[1:]...) //nolint:gosec
		out, err := c.CombinedOutput()
		output := strings.TrimSpace(string(out))
		if output == "" && err == nil {
			if launchMsg != "" {
				output = launchMsg
			} else {
				output = "(command completed — no output)"
			}
		}
		return cmdResultMsg{output: output, err: err}
	}
}
