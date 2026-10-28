package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Live status: a domain may set a live: command whose output is shown above
// the right pane while that domain is selected. Only the selected domain is
// polled, every pollInterval and as soon as it is selected, and never twice at
// once.

const (
	pollInterval = 5 * time.Second
	pollTimeout  = 3 * time.Second
	maxLiveLines = 3
)

// tickMsg fires on each polling interval.
type tickMsg time.Time

// liveStatusMsg carries the output of a domain's live command.
type liveStatusMsg struct {
	key    string
	status string
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// liveKey returns the key and live command of the selected domain, or "" if
// it has none.
func (m Model) liveKey() (string, []string) {
	if m.domainCursor >= len(m.domains) || len(m.domains[m.domainCursor].Live) == 0 {
		return "", nil
	}
	d := m.domains[m.domainCursor]
	return m.runKey(d.Name, ""), d.Live
}

// pollLive starts the selected domain's live command unless it is already
// running.
func (m Model) pollLive() (Model, tea.Cmd) {
	key, cmd := m.liveKey()
	if key == "" || m.liveInFlight[key] {
		return m, nil
	}
	if m.liveInFlight == nil {
		m.liveInFlight = make(map[string]bool)
	}
	m.liveInFlight[key] = true
	return m, probeLive(m.ctx, key, cmd)
}

// probeLive runs a live command and reports its first lines of output, or ""
// if it fails or times out.
func probeLive(ctx context.Context, key string, cmd []string) tea.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	return func() tea.Msg {
		pctx, cancel := context.WithTimeout(ctx, pollTimeout)
		defer cancel()
		out, err := exec.CommandContext(pctx, cmd[0], cmd[1:]...).Output() //nolint:gosec
		if err != nil {
			return liveStatusMsg{key: key}
		}
		var lines []string
		for _, l := range strings.Split(string(out), "\n") {
			if l = strings.TrimSpace(l); l != "" && len(lines) < maxLiveLines {
				lines = append(lines, l)
			}
		}
		return liveStatusMsg{key: key, status: strings.Join(lines, "\n")}
	}
}

// liveStatusText returns the live status of the selected domain, or "".
func (m Model) liveStatusText() string {
	key, _ := m.liveKey()
	if key == "" {
		return ""
	}
	return m.liveStatus[key]
}

// domainLiveHeader returns the styled live status for the right pane, or "".
func (m Model) domainLiveHeader() string {
	if s := m.liveStatusText(); s != "" {
		return liveHeaderStyle.Render(s)
	}
	return ""
}
