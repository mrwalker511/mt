package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mrwalker511/mt/internal/llm"
)

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

// Update handles one message and keeps the list scroll positions in step with
// the cursors and window size.
// When the selected domain changes, its live status is fetched right away.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevLive, _ := m.liveKey()
	next, cmd := m.update(msg)
	next = next.syncListTops()
	if key, _ := next.liveKey(); key != "" && key != prevLive {
		var poll tea.Cmd
		next, poll = next.pollLive()
		cmd = tea.Batch(cmd, poll)
	}
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		next, poll := m.pollLive()
		return next, tea.Batch(poll, tickCmd())

	case liveStatusMsg:
		delete(m.liveInFlight, msg.key)
		m.liveStatus[msg.key] = msg.status
		return m, nil

	case outputTickMsg:
		if msg.run != m.runSeq || !m.running {
			return m, nil
		}
		return m.refreshRunOutput(), outputTick(m.runSeq)

	case jobDoneMsg:
		return m.jobDone(msg), nil

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

	case tea.MouseMsg:
		// The prompt bar and the AI confirmation take the keyboard; ignore the
		// pointer meanwhile so a stray click cannot change the selection.
		if m.inputMode || m.confirmCmd != nil {
			return m, nil
		}
		return m.handleMouse(msg)

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
				if msg.Alt { // ⌥⌫ with Option sending Meta
					m.inputBuf = deleteLastWord(m.inputBuf)
				} else if r := []rune(m.inputBuf); len(r) > 0 {
					m.inputBuf = string(r[:len(r)-1])
				}
			case tea.KeyCtrlW:
				m.inputBuf = deleteLastWord(m.inputBuf)
			case tea.KeyCtrlU:
				m.inputBuf = ""
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
				return m.startAICommand(cmd)
			case "esc":
				m.confirmCmd = nil
				m.cmdErr = "AI command cancelled."
			case "q", "ctrl+c":
				return m.quit()
			}
			return m, nil
		}

		switch act := keyActions[msg.String()]; act {
		case actQuit:
			return m.quit()

		case actHelp:
			m.showHelp = !m.showHelp

		case actClear:
			m.output = ""
			m.cmdErr = ""
			m.scrollOffset = 0

		case actCopy:
			if m.output != "" {
				return m, copyToClipboard(m.output)
			}

		case actSave:
			if m.output != "" {
				return m, saveOutputToFile(m.output)
			}

		case actAI:
			if !m.running && !m.llmPending {
				m.inputMode, m.inputBuf = true, ""
			}

		case actCancel:
			// Cancel in-flight work. The cancelled command or query still reports
			// back through its result message, which clears the running state.
			if m.llmPending && m.llmCancel != nil {
				m.llmCancel()
			}
			if m.running && m.runCancel != nil {
				m.runCancel()
			}

		case actSelect:
			if m.activePane == paneMiddle {
				if key, ok := m.currentKey(); ok {
					if m.selectedTargets[key] {
						delete(m.selectedTargets, key)
					} else {
						m.selectedTargets[key] = true
					}
				}
			}

		case actReload:
			r, err := loadConfigFiles()
			if err != nil {
				m.output, m.cmdErr = "", "Config reload error: "+err.Error()
			} else {
				m.allWorkspaces = r.workspaces
				m.llmConfig = r.llm
				m = m.selectWorkspace(0)
				m.targetOutputs = make(map[string]outputRecord)
				m.output = fmt.Sprintf("Config reloaded: %d workspace(s) loaded.", len(r.workspaces))
				m.cmdErr = r.notice
			}

		case actNextWorkspace:
			if n := len(m.allWorkspaces); n > 1 {
				m = m.switchWorkspace((m.workspaceIdx + 1) % n)
			}

		case actPrevWorkspace:
			if n := len(m.allWorkspaces); n > 1 {
				m = m.switchWorkspace((m.workspaceIdx - 1 + n) % n)
			}

		case actJumpWorkspace:
			if i := int(msg.String()[0] - '1'); i < len(m.allWorkspaces) && i != m.workspaceIdx {
				m = m.switchWorkspace(i)
			}

		case actRun:
			if m.running {
				return m, nil
			}

			// Multi-select: run all selected targets in parallel.
			if len(m.selectedTargets) > 0 {
				return m.startSelected()
			}

			if target, ok := m.currentTarget(); ok {
				return m.startTarget(m.domains[m.domainCursor].Name, target)
			}

		case actPaneLeft:
			if m.activePane > paneLeft {
				m.activePane--
			}

		case actPaneRight:
			if m.activePane < paneRight {
				m.activePane++
			}

		case actNextPane:
			m.activePane = (m.activePane + 1) % 3

		case actPrevPane:
			m.activePane = (m.activePane + 2) % 3

		case actPageUp, actPageDown, actTop, actBottom:
			// These always scroll the output, whichever pane has focus.
			maxOffset := m.maxScroll()
			switch act {
			case actPageUp:
				m.scrollOffset -= m.rightPanePageSize()
			case actPageDown:
				m.scrollOffset += m.rightPanePageSize()
			case actTop:
				m.scrollOffset = 0
			case actBottom:
				m.scrollOffset = maxOffset
			}
			m.scrollOffset = min(max(m.scrollOffset, 0), maxOffset)

		case actUp:
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

		case actDown:
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
				m.scrollOffset = min(m.scrollOffset+1, m.maxScroll())
			}
		}
		return m, nil
	}

	return m, nil
}

// quit cancels all in-flight work and exits the program.
func (m Model) quit() (Model, tea.Cmd) {
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

// switchWorkspace moves to workspace i, keeping each target's output cached.
func (m Model) switchWorkspace(i int) Model {
	return m.saveTargetOutput().selectWorkspace(i).restoreTargetOutput()
}

// deleteLastWord removes the last word (and any spaces after it) from s.
func deleteLastWord(s string) string {
	s = strings.TrimRight(s, " ")
	if i := strings.LastIndex(s, " "); i >= 0 {
		return s[:i+1]
	}
	return ""
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
	sb.WriteString("RUN:<target_name>   - run an existing target by its exact name\n")
	sb.WriteString("CMD:<json_array>    - run a new command as a JSON array, e.g. CMD:[\"osascript\",\"-e\",\"...\"]\n")
	sb.WriteString("INFO:<text>         - answer a question in ≤2 sentences\n\n")
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
