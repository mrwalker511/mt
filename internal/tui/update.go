package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// maxOutputBytes caps the output retained from any single command execution.
// Output beyond this limit is truncated with a notice. Prevents OOM from
// runaway commands that produce large amounts of data.
const maxOutputBytes = 1 << 20 // 1 MB

// validHostRe matches SSH destinations of the form [user@]hostname where
// hostname is composed of alphanumerics, dots, hyphens, and percent signs
// (percent-encoding for scoped IPv6). A leading dash would be interpreted by
// SSH as an option flag, enabling ProxyCommand injection.
var validHostRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._@%-]*$`)

// validateHost returns an error if host is not a safe SSH destination.
func validateHost(host string) error {
	if !validHostRe.MatchString(host) {
		return fmt.Errorf("invalid ssh host %q: must match [a-zA-Z0-9][a-zA-Z0-9._@%%-]*", host)
	}
	return nil
}

// validateWorkspaces returns an error if any target contains an invalid SSH host.
func validateWorkspaces(workspaces []Workspace) error {
	for _, ws := range workspaces {
		for _, d := range ws.Domains {
			for _, t := range d.Targets {
				if t.Host != "" {
					if err := validateHost(t.Host); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

const pollInterval = 5 * time.Second

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		if m.hasGitDomain() {
			cmds = append(cmds, pollGit(m.ctx))
		}
		if m.hasDockerDomain() {
			cmds = append(cmds, pollDocker(m.ctx))
		}
		return m, tea.Batch(cmds...)

	case statusUpdateMsg:
		m.liveStatus[msg.key] = msg.status
		return m, nil

	case cmdResultMsg:
		output := strings.TrimSpace(msg.output)

		// Sequence in progress and step succeeded: advance to next step.
		if msg.err == nil && len(m.seqQueue) > 0 {
			m.seqOutput += output + "\n"
			next := m.seqQueue[0]
			m.seqQueue = m.seqQueue[1:]
			execCmd := effectiveCmd(next)
			if len(execCmd) == 0 {
				nextName := next.Name
				return m, func() tea.Msg {
					return cmdResultMsg{err: fmt.Errorf("sequence step %q has no command", nextName)}
				}
			}
			return m, runCmd(m.ctx, execCmd, next.LaunchMsg)
		}

		// Sequence complete, sequence failed, or normal single command done.
		m.running = false
		m.output = strings.TrimSpace(m.seqOutput + output)
		m.seqOutput = ""
		m.seqQueue = nil
		m.scrollOffset = 0
		if msg.err != nil {
			m.cmdErr = msg.err.Error()
			if m.pendingTarget != "" {
				m.runStates[m.pendingTarget] = runFailure
			}
		} else {
			m.cmdErr = ""
			if m.pendingTarget != "" {
				m.runStates[m.pendingTarget] = runSuccess
			}
		}
		m.pendingTarget = ""
		return m, nil

	case parallelCmdResultMsg:
		out := strings.TrimSpace(msg.output)
		if msg.err != nil {
			m.runStates[msg.key] = runFailure
			out += "\n[ERROR: " + msg.err.Error() + "]"
		} else {
			m.runStates[msg.key] = runSuccess
		}
		m.parallelOutputs[msg.label] = out
		m.multiRunPending--
		if m.multiRunPending == 0 {
			m.running = false
			labels := make([]string, 0, len(m.parallelOutputs))
			for l := range m.parallelOutputs {
				labels = append(labels, l)
			}
			sort.Strings(labels)
			parts := make([]string, 0, len(labels))
			for _, l := range labels {
				parts = append(parts, "=== "+l+" ===\n"+m.parallelOutputs[l])
			}
			m.output = strings.Join(parts, "\n\n")
			m.parallelOutputs = make(map[string]string)
			m.pendingTarget = ""
			m.scrollOffset = 0
		}
		return m, nil

	case clipboardMsg:
		if msg.err != nil {
			m.cmdErr = "Copy failed: " + msg.err.Error()
		} else {
			m.cmdErr = "Copied to clipboard."
		}
		return m, nil

	case saveOutputMsg:
		if msg.err != nil {
			m.cmdErr = "Save failed: " + msg.err.Error()
		} else {
			m.cmdErr = "Saved → " + msg.path
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			m.quitting = true
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp

		case "c":
			m.output = ""
			m.cmdErr = ""
			m.scrollOffset = 0

		case "y":
			if m.output != "" {
				return m, copyToClipboard(m.output)
			}

		case "S":
			if m.output != "" {
				return m, saveOutputToFile(m.output)
			}

		case " ":
			if m.activePane == paneMiddle {
				if m.domainCursor < len(m.domains) {
					targets := m.domains[m.domainCursor].Targets
					if m.targetCursor < len(targets) {
						t := targets[m.targetCursor]
						key := m.runKey(m.domains[m.domainCursor].Name, t.Name)
						if m.selectedTargets[key] {
							delete(m.selectedTargets, key)
						} else {
							m.selectedTargets[key] = true
						}
					}
				}
			}

		case "R":
			workspaces, err := LoadWorkspaces()
			m.allWorkspaces = workspaces
			m.workspaceIdx = 0
			if len(workspaces) > 0 {
				m.domains = workspaces[0].Domains
			} else {
				m.domains = nil
			}
			m.domainCursor, m.targetCursor, m.scrollOffset = 0, 0, 0
			m.targetOutputs = make(map[string]outputRecord)
			m.selectedTargets = make(map[string]bool)
			m.showHelp = false
			if err != nil {
				m.output, m.cmdErr = "", "Config reload error: "+err.Error()
			} else {
				m.output = fmt.Sprintf("Config reloaded — %d workspace(s) loaded.", len(workspaces))
				m.cmdErr = ""
			}

		case "tab":
			if len(m.allWorkspaces) > 1 {
				m = m.saveTargetOutput()
				m.workspaceIdx = (m.workspaceIdx + 1) % len(m.allWorkspaces)
				m.domains = m.allWorkspaces[m.workspaceIdx].Domains
				m.domainCursor = 0
				m.targetCursor = 0
				m.selectedTargets = make(map[string]bool)
				m = m.restoreTargetOutput()
				m.showHelp = false
			}

		case "shift+tab":
			if len(m.allWorkspaces) > 1 {
				m = m.saveTargetOutput()
				m.workspaceIdx = (m.workspaceIdx - 1 + len(m.allWorkspaces)) % len(m.allWorkspaces)
				m.domains = m.allWorkspaces[m.workspaceIdx].Domains
				m.domainCursor = 0
				m.targetCursor = 0
				m.selectedTargets = make(map[string]bool)
				m = m.restoreTargetOutput()
				m.showHelp = false
			}

		case "enter":
			if m.running {
				return m, nil
			}

			// Multi-select: run all selected targets in parallel.
			if len(m.selectedTargets) > 0 {
				var cmds []tea.Cmd
				for key := range m.selectedTargets {
					t, ok := m.findTargetByRunKey(key)
					if !ok || len(t.Cmd) == 0 {
						continue
					}
					cmds = append(cmds, runParallelCmd(m.ctx, key, t.Name, effectiveCmd(t), t.LaunchMsg))
				}
				if len(cmds) == 0 {
					m.cmdErr = "No executable targets selected."
					return m, nil
				}
				m.multiRunPending = len(cmds)
				m.parallelOutputs = make(map[string]string)
				m.running = true
				m.output = ""
				m.cmdErr = ""
				m.showHelp = false
				m.selectedTargets = make(map[string]bool)
				return m, tea.Batch(cmds...)
			}

			target, ok := m.currentTarget()
			if !ok {
				return m, nil
			}

			// Sequence target: run steps one by one in order.
			if len(target.Sequence) > 0 {
				steps := m.resolveSequenceTargets(target.Sequence)
				if len(steps) == 0 || len(steps[0].Cmd) == 0 {
					m.cmdErr = "Sequence has no executable steps."
					return m, nil
				}
				if m.domainCursor < len(m.domains) {
					m.pendingTarget = m.runKey(m.domains[m.domainCursor].Name, target.Name)
				}
				m.seqQueue = steps[1:]
				m.seqOutput = ""
				m.running = true
				m.output = ""
				m.cmdErr = ""
				m.showHelp = false
				return m, runCmd(m.ctx, effectiveCmd(steps[0]), steps[0].LaunchMsg)
			}

			// Normal single command.
			if len(target.Cmd) == 0 {
				m.output = ""
				m.cmdErr = "No command configured for this target."
				return m, nil
			}
			if m.domainCursor < len(m.domains) {
				m.pendingTarget = m.runKey(m.domains[m.domainCursor].Name, target.Name)
			}
			m.running = true
			m.output = ""
			m.cmdErr = ""
			m.showHelp = false
			return m, runCmd(m.ctx, effectiveCmd(target), target.LaunchMsg)

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
					m = m.saveTargetOutput()
					m.domainCursor--
					m.targetCursor = 0
					m = m.restoreTargetOutput()
					m.showHelp = false
				}
			case paneMiddle:
				if m.targetCursor > 0 {
					m = m.saveTargetOutput()
					m.targetCursor--
					m = m.restoreTargetOutput()
					m.showHelp = false
				}
			case paneRight:
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			}

		case "down", "j":
			switch m.activePane {
			case paneLeft:
				if m.domainCursor < len(m.domains)-1 {
					m = m.saveTargetOutput()
					m.domainCursor++
					m.targetCursor = 0
					m = m.restoreTargetOutput()
					m.showHelp = false
				}
			case paneMiddle:
				if m.domainCursor >= len(m.domains) {
					break
				}
				targets := m.domains[m.domainCursor].Targets
				if m.targetCursor < len(targets)-1 {
					m = m.saveTargetOutput()
					m.targetCursor++
					m = m.restoreTargetOutput()
					m.showHelp = false
				}
			case paneRight:
				lines := strings.Split(m.output, "\n")
				pageSize := m.rightPanePageSize()
				if m.scrollOffset < len(lines)-pageSize {
					m.scrollOffset++
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

// effectiveCmd returns the command to run for a target, prepending ssh if Host is set.
func effectiveCmd(t Target) []string {
	if t.Host == "" {
		return t.Cmd
	}
	return append([]string{"ssh", t.Host}, t.Cmd...)
}

// tickCmd schedules the next status poll cycle.
func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// pollGit batches all git probes.
func pollGit(ctx context.Context) tea.Cmd {
	return tea.Batch(pollGitBranch(ctx), pollGitDirty(ctx))
}

func pollGitBranch(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(pollCtx, "git", "rev-parse", "--abbrev-ref", "HEAD").Output() //nolint:gosec
		if err != nil {
			return statusUpdateMsg{key: "git.branch", status: ""}
		}
		return statusUpdateMsg{key: "git.branch", status: strings.TrimSpace(string(out))}
	}
}

func pollGitDirty(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(pollCtx, "git", "status", "--porcelain").Output() //nolint:gosec
		if err != nil {
			return statusUpdateMsg{key: "git.dirty", status: ""}
		}
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
func pollDocker(ctx context.Context) tea.Cmd {
	return tea.Batch(
		pollDockerContainer(ctx, "postgres"),
		pollDockerContainer(ctx, "redis"),
	)
}

func pollDockerContainer(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		if ctx == nil {
			ctx = context.Background()
		}
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(pollCtx, "docker", "ps", "--filter", "name="+name, "--format", "{{.Status}}").Output() //nolint:gosec
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
// ctx is used to cancel the command when the user quits.
func runCmd(ctx context.Context, cmd []string, launchMsg string) tea.Cmd {
	return func() tea.Msg {
		if len(cmd) == 0 {
			return cmdResultMsg{err: fmt.Errorf("no command to execute")}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		c := exec.CommandContext(ctx, cmd[0], cmd[1:]...) //nolint:gosec
		out, err := c.CombinedOutput()
		if len(out) > maxOutputBytes {
			out = append(out[:maxOutputBytes], []byte("\n…(output truncated at 1 MB)")...)
		}
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

// runParallelCmd executes a command and wraps the result as a parallelCmdResultMsg.
// ctx is used to cancel the command when the user quits.
func runParallelCmd(ctx context.Context, key, label string, cmd []string, launchMsg string) tea.Cmd {
	return func() tea.Msg {
		if len(cmd) == 0 {
			return parallelCmdResultMsg{key: key, label: label, err: fmt.Errorf("no command to execute")}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		c := exec.CommandContext(ctx, cmd[0], cmd[1:]...) //nolint:gosec
		out, err := c.CombinedOutput()
		if len(out) > maxOutputBytes {
			out = append(out[:maxOutputBytes], []byte("\n…(output truncated at 1 MB)")...)
		}
		output := strings.TrimSpace(string(out))
		if output == "" && err == nil {
			if launchMsg != "" {
				output = launchMsg
			} else {
				output = "(command completed — no output)"
			}
		}
		return parallelCmdResultMsg{key: key, label: label, output: output, err: err}
	}
}

// copyToClipboard writes text to the system clipboard asynchronously.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy") //nolint:gosec
		default:
			if _, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command("xclip", "-selection", "clipboard") //nolint:gosec
			} else if _, err := exec.LookPath("xsel"); err == nil {
				cmd = exec.Command("xsel", "--clipboard", "--input") //nolint:gosec
			} else if _, err := exec.LookPath("wl-copy"); err == nil {
				cmd = exec.Command("wl-copy") //nolint:gosec
			} else {
				return clipboardMsg{err: fmt.Errorf("no clipboard tool found (install xclip, xsel, or wl-clipboard)")}
			}
		}
		cmd.Stdin = strings.NewReader(text)
		return clipboardMsg{err: cmd.Run()}
	}
}

// saveOutputToFile writes output to a timestamped file under ~/.mt/logs/.
func saveOutputToFile(output string) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return saveOutputMsg{err: fmt.Errorf("home dir: %w", err)}
		}
		dir := filepath.Join(home, ".mt", "logs")
		if err := os.MkdirAll(dir, 0750); err != nil {
			return saveOutputMsg{err: fmt.Errorf("creating log dir: %w", err)}
		}
		ts := time.Now().Format("20060102-150405")
		path := filepath.Join(dir, "mt-"+ts+".txt")
		// O_EXCL prevents following a symlink placed at this path (TOCTOU guard).
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if err != nil {
			return saveOutputMsg{err: fmt.Errorf("creating log file: %w", err)}
		}
		_, writeErr := f.WriteString(output)
		closeErr := f.Close()
		if writeErr != nil {
			return saveOutputMsg{err: fmt.Errorf("writing log file: %w", writeErr)}
		}
		if closeErr != nil {
			return saveOutputMsg{err: fmt.Errorf("closing log file: %w", closeErr)}
		}
		return saveOutputMsg{path: path}
	}
}
