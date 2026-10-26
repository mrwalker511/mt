package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/mrwalker511/mt/internal/llm"
)

// maxOutputBytes caps the output retained from any single command execution.
// Output beyond this limit is truncated with a notice. Prevents OOM from
// runaway commands that produce large amounts of data.
const maxOutputBytes = 1 << 20 // 1 MB

// maxInputRunes caps the length of the AI prompt bar input.
const maxInputRunes = 500

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
		return m, tea.Batch(append(m.pollCmds(), tickCmd())...)

	case statusUpdateMsg:
		m.liveStatus[msg.key] = msg.status
		return m, nil

	case cmdResultMsg:
		output := strings.TrimSpace(msg.output)

		// Sequence in progress and step succeeded: advance to next step.
		if msg.err == nil && len(m.seqQueue) > 0 {
			if len(m.seqOutput)+len(output)+1 <= maxOutputBytes {
				m.seqOutput += output + "\n"
			} else {
				m.seqOutput += "…(sequence output truncated)\n"
			}
			next := m.seqQueue[0]
			m.seqQueue = m.seqQueue[1:]
			execCmd := effectiveCmd(next)
			if len(execCmd) == 0 {
				nextName := next.Name
				return m, func() tea.Msg {
					return cmdResultMsg{err: fmt.Errorf("sequence step %q has no command", nextName)}
				}
			}
			return m, runCmd(m.runCtx, execCmd, next.LaunchMsg)
		}

		// Sequence complete, sequence failed, or normal single command done.
		m = m.finishRun()
		output = strings.TrimSpace(m.seqOutput + output)
		m.seqOutput = ""
		m.seqQueue = nil
		cmdErr := ""
		if msg.err != nil {
			cmdErr = msg.err.Error()
		}
		key := m.pendingTarget
		m.pendingTarget = ""
		if key != "" {
			if msg.err != nil {
				m.runStates[key] = runFailure
			} else {
				m.runStates[key] = runSuccess
			}
			// The user may have moved to another target while this one ran:
			// file the result under the target that produced it.
			if cur, _ := m.currentKey(); cur != key {
				m.cacheOutput(key, output, cmdErr)
				return m, nil
			}
		}
		m.output, m.cmdErr, m.scrollOffset = output, cmdErr, 0
		return m, nil

	case parallelCmdResultMsg:
		out := strings.TrimSpace(msg.output)
		if msg.err != nil {
			m.runStates[msg.key] = runFailure
			out += "\n[ERROR: " + msg.err.Error() + "]"
		} else {
			m.runStates[msg.key] = runSuccess
		}
		m.parallelOutputs[msg.key] = "=== " + msg.label + " ===\n" + out
		m.multiRunPending--
		if m.multiRunPending == 0 {
			m = m.finishRun()
			keys := make([]string, 0, len(m.parallelOutputs))
			for k := range m.parallelOutputs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, m.parallelOutputs[k])
			}
			m.output = strings.Join(parts, "\n\n")
			m.parallelOutputs = make(map[string]string)
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

	case llmResponseMsg:
		if m.llmCancel != nil {
			m.llmCancel()
			m.llmCancel = nil
		}
		m.llmPending = false
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.cmdErr = "AI query cancelled."
			} else {
				m.cmdErr = "AI: " + msg.err.Error()
			}
			return m, nil
		}
		action, payload := parseLLMResponse(msg.response)
		switch action {
		case "run":
			if m.running {
				m.cmdErr = "AI: a command is already running."
				return m, nil
			}
			name := strings.TrimSpace(payload)
			for i, d := range m.domains {
				for j, t := range d.Targets {
					if strings.EqualFold(t.Name, name) {
						// Select the target so its result is shown and cached in place.
						m = m.saveTargetOutput()
						m.domainCursor, m.targetCursor = i, j
						return m.startTarget(d.Name, t)
					}
				}
			}
			m.cmdErr = "AI: target not found: " + name
		case "cmd":
			var cmdSlice []string
			if jsonErr := json.Unmarshal([]byte(payload), &cmdSlice); jsonErr != nil || len(cmdSlice) == 0 {
				m.cmdErr = "AI: malformed command from LLM"
				return m, nil
			}
			m.confirmCmd = cmdSlice // await user confirmation before running
		case "info":
			m.output, m.cmdErr, m.scrollOffset = payload, "", 0
		}
		return m, nil

	case tea.KeyMsg:
		// Input mode: capture all keystrokes for the AI prompt bar.
		if m.inputMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.inputMode, m.inputBuf = false, ""
			case tea.KeyEnter:
				if m.inputBuf != "" {
					query := m.inputBuf
					m.inputMode, m.inputBuf = false, ""
					m.llmPending = true
					llmCtx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
					m.llmCancel = cancel
					return m, runLLMQuery(llmCtx, m.llmConfig, m.buildSystemPrompt(), query)
				}
			case tea.KeyBackspace, tea.KeyCtrlH:
				if r := []rune(m.inputBuf); len(r) > 0 {
					m.inputBuf = string(r[:len(r)-1])
				}
			case tea.KeyRunes, tea.KeySpace:
				if len([]rune(m.inputBuf))+len(msg.Runes) <= maxInputRunes {
					m.inputBuf += string(msg.Runes)
				}
			}
			return m, nil
		}

		// Confirm overlay: AI-generated CMD awaiting user approval.
		if m.confirmCmd != nil {
			switch msg.String() {
			case "enter":
				cmd := m.confirmCmd
				m.confirmCmd = nil
				m = m.beginRun()
				return m, runCmd(m.runCtx, cmd, "")
			case "esc":
				m.confirmCmd = nil
				m.cmdErr = "AI command cancelled."
			case "q", "ctrl+c":
				return m.quit()
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m.quit()

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

		case "/":
			if !m.running && !m.llmPending {
				m.inputMode, m.inputBuf = true, ""
			}

		case "esc":
			// Cancel in-flight work. The cancelled command or query still reports
			// back through its result message, which clears the running state.
			if m.llmPending && m.llmCancel != nil {
				m.llmCancel()
			}
			if m.running && m.runCancel != nil {
				m.runCancel()
			}

		case " ":
			if m.activePane == paneMiddle {
				if key, ok := m.currentKey(); ok {
					if m.selectedTargets[key] {
						delete(m.selectedTargets, key)
					} else {
						m.selectedTargets[key] = true
					}
				}
			}

		case "R":
			workspaces, llmCfg, err := LoadWorkspaces()
			if err != nil {
				m.output, m.cmdErr = "", "Config reload error: "+err.Error()
			} else {
				m.allWorkspaces = workspaces
				m.llmConfig = llmCfg
				m = m.selectWorkspace(0)
				m.targetOutputs = make(map[string]outputRecord)
				m.output = fmt.Sprintf("Config reloaded — %d workspace(s) loaded.", len(workspaces))
				m.cmdErr = ""
			}

		case "tab":
			if n := len(m.allWorkspaces); n > 1 {
				m = m.saveTargetOutput().selectWorkspace((m.workspaceIdx + 1) % n).restoreTargetOutput()
			}

		case "shift+tab":
			if n := len(m.allWorkspaces); n > 1 {
				m = m.saveTargetOutput().selectWorkspace((m.workspaceIdx - 1 + n) % n).restoreTargetOutput()
			}

		case "enter":
			if m.running {
				return m, nil
			}

			// Multi-select: run all selected targets in parallel.
			if len(m.selectedTargets) > 0 {
				m = m.beginRun()
				var cmds []tea.Cmd
				for key := range m.selectedTargets {
					t, ok := m.findTargetByRunKey(key)
					if !ok || len(t.Cmd) == 0 {
						continue
					}
					cmds = append(cmds, runParallelCmd(m.runCtx, key, t.Name, effectiveCmd(t), t.LaunchMsg))
				}
				if len(cmds) == 0 {
					m = m.finishRun()
					m.cmdErr = "No executable targets selected."
					return m, nil
				}
				m.multiRunPending = len(cmds)
				m.parallelOutputs = make(map[string]string)
				m.selectedTargets = make(map[string]bool)
				return m, tea.Batch(cmds...)
			}

			if target, ok := m.currentTarget(); ok {
				return m.startTarget(m.domains[m.domainCursor].Name, target)
			}

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
					m = m.moveCursor(m.domainCursor-1, 0)
				}
			case paneMiddle:
				if m.targetCursor > 0 {
					m = m.moveCursor(m.domainCursor, m.targetCursor-1)
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
					m = m.moveCursor(m.domainCursor+1, 0)
				}
			case paneMiddle:
				if m.domainCursor < len(m.domains) && m.targetCursor < len(m.domains[m.domainCursor].Targets)-1 {
					m = m.moveCursor(m.domainCursor, m.targetCursor+1)
				}
			case paneRight:
				lineCount := strings.Count(m.output, "\n") + 1
				if m.scrollOffset < lineCount-m.rightPanePageSize() {
					m.scrollOffset++
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// quit cancels all in-flight work and exits the program.
func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	m.quitting = true
	return m, tea.Quit
}

// moveCursor selects a new domain/target, caching the old target's output and
// restoring the new one's.
func (m Model) moveCursor(domain, target int) Model {
	m = m.saveTargetOutput()
	m.domainCursor, m.targetCursor = domain, target
	m = m.restoreTargetOutput()
	m.showHelp = false
	return m
}

// selectWorkspace makes workspace i active and resets cursors and selection.
func (m Model) selectWorkspace(i int) Model {
	m.workspaceIdx = i
	m.domains = nil
	if i < len(m.allWorkspaces) {
		m.domains = m.allWorkspaces[i].Domains
	}
	m.domainCursor, m.targetCursor, m.scrollOffset = 0, 0, 0
	m.selectedTargets = make(map[string]bool)
	m.showHelp = false
	return m
}

// beginRun marks a run as started, clears the right pane, and creates the
// cancellable context the run's commands execute under (Esc cancels it).
func (m Model) beginRun() Model {
	parent := m.ctx
	if parent == nil {
		parent = context.Background()
	}
	m.runCtx, m.runCancel = context.WithCancel(parent)
	m.running = true
	m.output, m.cmdErr, m.scrollOffset = "", "", 0
	m.showHelp = false
	return m
}

// finishRun marks the current run as finished and releases its context.
func (m Model) finishRun() Model {
	if m.runCancel != nil {
		m.runCancel()
	}
	m.running, m.runCtx, m.runCancel = false, nil, nil
	return m
}

// startTarget runs target t (a single command or a sequence) from domain domainName.
func (m Model) startTarget(domainName string, t Target) (Model, tea.Cmd) {
	first := t
	if len(t.Sequence) > 0 {
		steps, err := m.resolveSequenceTargets(t.Sequence)
		if err != nil {
			m.output, m.cmdErr = "", err.Error()
			return m, nil
		}
		first = steps[0]
		m.seqQueue = steps[1:]
		m.seqOutput = ""
	}
	cmd := effectiveCmd(first)
	if len(cmd) == 0 {
		m.seqQueue = nil
		m.output = ""
		if len(t.Sequence) > 0 {
			m.cmdErr = fmt.Sprintf("Sequence step %q has no command.", first.Name)
		} else {
			m.cmdErr = "No command configured for this target."
		}
		return m, nil
	}
	m.pendingTarget = m.runKey(domainName, t.Name)
	m = m.beginRun()
	return m, runCmd(m.runCtx, cmd, first.LaunchMsg)
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
	if t.Host == "" || len(t.Cmd) == 0 {
		return t.Cmd
	}
	return append([]string{"ssh", t.Host}, t.Cmd...)
}

// pollCmds returns the live-status probes relevant to the active workspace.
func (m Model) pollCmds() []tea.Cmd {
	var cmds []tea.Cmd
	if m.hasDomain("Context/Git") {
		cmds = append(cmds, pollGit(m.ctx))
	}
	if m.hasDomain("Infrastructure") {
		cmds = append(cmds, pollDocker(m.ctx))
	}
	return cmds
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

// cappedBuffer keeps the first limit bytes written to it and discards the rest,
// so a runaway command cannot exhaust memory. Writes never fail, so the command
// is not killed by a broken pipe once the cap is reached.
type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := b.limit - b.buf.Len(); n > room {
		p = p[:max(room, 0)]
		b.truncated = true
	}
	b.buf.Write(p)
	return n, nil
}

// execCapture runs cmd and returns its combined stdout/stderr, capped at
// maxOutputBytes. launchMsg (or a generic note) replaces empty output on success.
func execCapture(ctx context.Context, cmd []string, launchMsg string) (string, error) {
	if len(cmd) == 0 {
		return "", fmt.Errorf("no command to execute")
	}
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...) //nolint:gosec
	out := &cappedBuffer{limit: maxOutputBytes}
	c.Stdout, c.Stderr = out, out
	// Don't hang on cancellation if a killed command left children holding the pipe.
	c.WaitDelay = 2 * time.Second
	err := c.Run()
	if err != nil && ctx.Err() != nil {
		err = errors.New("cancelled")
	}
	output := strings.TrimSpace(out.buf.String())
	if out.truncated {
		output += "\n…(output truncated at 1 MB)"
	}
	if output == "" && err == nil {
		output = launchMsg
		if output == "" {
			output = "(command completed — no output)"
		}
	}
	return output, err
}

// runCmd executes cmd asynchronously and returns the result as a cmdResultMsg.
func runCmd(ctx context.Context, cmd []string, launchMsg string) tea.Cmd {
	return func() tea.Msg {
		output, err := execCapture(ctx, cmd, launchMsg)
		return cmdResultMsg{output: output, err: err}
	}
}

// runParallelCmd executes cmd asynchronously and returns the result as a
// parallelCmdResultMsg tagged with the target's runKey and display label.
func runParallelCmd(ctx context.Context, key, label string, cmd []string, launchMsg string) tea.Cmd {
	return func() tea.Msg {
		output, err := execCapture(ctx, cmd, launchMsg)
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

// buildSystemPrompt constructs the LLM system prompt listing all available targets
// and the structured response format the LLM must follow.
func (m Model) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are an AI assistant embedded in mt, a macOS terminal workspace launcher.\n")
	sb.WriteString("Map the user's request to exactly one response.\n\n")
	sb.WriteString("Available targets:\n")
	for _, d := range m.domains {
		for _, t := range d.Targets {
			line := "- " + t.Name
			if t.Status != "" {
				first := strings.SplitN(t.Status, "\n", 2)[0]
				if first != "" {
					line += ": " + first
				}
			}
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("\nRespond with EXACTLY ONE of (nothing else):\n")
	sb.WriteString("RUN:<target_name>   — run an existing target by its exact name\n")
	sb.WriteString("CMD:<json_array>    — run a new command as a JSON array, e.g. CMD:[\"osascript\",\"-e\",\"...\"]\n")
	sb.WriteString("INFO:<text>         — answer a question in ≤2 sentences\n\n")
	sb.WriteString("For Outlook meetings use CMD with osascript. Outlook AppleScript properties: subject, start time, end time, location.\n")
	sb.WriteString("To add an attendee: make new attendee at end of attendees of newEvent with properties {email address:{address:\"EMAIL\"}}\n")
	sb.WriteString("Current time: " + time.Now().Format("Monday, January 2 2006 15:04") + "\n")
	return sb.String()
}

// parseLLMResponse parses a structured LLM response into an action and payload.
// Recognised prefixes: RUN:, CMD:, INFO:. Falls back to "info" for unrecognised format.
func parseLLMResponse(response string) (action, payload string) {
	response = strings.TrimSpace(response)
	for _, prefix := range []string{"RUN:", "CMD:", "INFO:"} {
		if strings.HasPrefix(response, prefix) {
			key := strings.ToLower(strings.TrimSuffix(prefix, ":"))
			return key, strings.TrimSpace(response[len(prefix):])
		}
	}
	return "info", response
}

// runLLMQuery sends the user query to the configured LLM and returns the response as a msg.
func runLLMQuery(ctx context.Context, cfg llm.Config, systemPrompt, query string) tea.Cmd {
	return func() tea.Msg {
		resp, err := llm.Generate(ctx, cfg, systemPrompt, query)
		if cfg.Debug {
			writeLLMDebugLog(cfg, systemPrompt, query, resp, err)
		}
		return llmResponseMsg{response: resp, err: err}
	}
}

// writeLLMDebugLog appends a timestamped entry to ~/.mt/logs/debug.log.
func writeLLMDebugLog(cfg llm.Config, systemPrompt, query, resp string, callErr error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".mt", "logs")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().UTC().Format(time.RFC3339)
	if callErr != nil {
		fmt.Fprintf(f, "[%s] provider=%s model=%s\nSystem: %s\nUser: %s\nError: %v\n\n",
			ts, cfg.Provider, cfg.Model, systemPrompt, query, callErr)
	} else {
		fmt.Fprintf(f, "[%s] provider=%s model=%s\nSystem: %s\nUser: %s\nResponse: %s\n\n",
			ts, cfg.Provider, cfg.Model, systemPrompt, query, resp)
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
