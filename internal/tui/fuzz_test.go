package tui

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrwalker511/mt/internal/llm"
)

// fuzzSeeds is how many random runs to do: 2 by default, so the test stays
// quick under the race detector; set MT_FUZZ_SEEDS for a longer local soak
// (e.g. MT_FUZZ_SEEDS=50 go test -run Fuzz ./internal/tui).
func fuzzSeeds() int64 {
	if n, err := strconv.Atoi(os.Getenv("MT_FUZZ_SEEDS")); err == nil && n > 0 {
		return int64(n)
	}
	return 2
}

// safeDefaults returns the built-in workspace with every command replaced by a
// harmless echo, so random Enter presses never launch real programs.
func safeDefaults() []Workspace {
	var domains []Domain
	for _, d := range initialDomains {
		nd := Domain{Name: d.Name, Live: []string{"echo", "live " + d.Name}}
		for _, t := range d.Targets {
			if len(t.Cmd) > 0 {
				t.Cmd = []string{"echo", t.Name}
			}
			nd.Targets = append(nd.Targets, t)
		}
		domains = append(domains, nd)
	}
	var big []Target
	for i := 0; i < 60; i++ {
		big = append(big, Target{Name: fmt.Sprintf("Item %02d with a fairly long name", i), Cmd: []string{"echo", "x"}})
	}
	big = append(big, Target{Name: "Seq", Sequence: []string{"Item 01 with a fairly long name", "Item 02 with a fairly long name"}})
	return []Workspace{{Name: "One", Domains: domains}, {Name: "Two", Domains: []Domain{{Name: "Big", Targets: big}}}}
}

// TestFuzz_RandomInputNeverPanicsAndFitsScreen sends thousands of random keys,
// clicks, scrolls, resizes and run messages through Update and checks every
// frame renders without panicking and exactly fills the terminal.
func TestFuzz_RandomInputNeverPanicsAndFitsScreen(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyLeft}, {Type: tea.KeyRight},
		{Type: tea.KeyTab}, {Type: tea.KeyShiftTab}, {Type: tea.KeyEnter}, {Type: tea.KeySpace, Runes: []rune{' '}},
		{Type: tea.KeyEsc}, {Type: tea.KeyBackspace}, {Type: tea.KeyPgUp}, {Type: tea.KeyPgDown},
		{Type: tea.KeyHome}, {Type: tea.KeyEnd}, {Type: tea.KeyCtrlK}, {Type: tea.KeyCtrlW}, {Type: tea.KeyCtrlU},
	}
	for _, r := range "c?[]123/rsyjkhlx" { // no q: quitting ends the run early
		keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	for seed := int64(1); seed <= fuzzSeeds(); seed++ {
		rng := rand.New(rand.NewSource(seed))
		ws := safeDefaults()
		ctx, cancel := context.WithCancel(context.Background())
		m := Model{
			allWorkspaces: ws, domains: ws[0].Domains,
			liveStatus: map[string]string{}, runStates: map[string]runResult{},
			targetOutputs: map[string]outputRecord{}, selectedTargets: map[string]bool{},
			ctx: ctx, cancel: cancel, llmConfig: llm.Config{Provider: "nonexistent"},
			width: 100, height: 30,
		}
		var pending []tea.Cmd
		for step := 0; step < 600; step++ {
			var msg tea.Msg
			switch r := rng.Intn(100); {
			case r < 60:
				msg = keys[rng.Intn(len(keys))]
			case r < 72:
				msg = tea.MouseMsg{X: rng.Intn(m.width + 5), Y: rng.Intn(m.height + 5), Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
			case r < 78:
				b := tea.MouseButtonWheelUp
				if rng.Intn(2) == 0 {
					b = tea.MouseButtonWheelDown
				}
				msg = tea.MouseMsg{X: rng.Intn(m.width), Y: rng.Intn(m.height), Button: b, Action: tea.MouseActionPress}
			case r < 82:
				msg = tea.WindowSizeMsg{Width: 20 + rng.Intn(200), Height: 3 + rng.Intn(60)}
			case r < 86:
				msg = llmResponseMsg{response: []string{"INFO: " + strings.Repeat("line\n", rng.Intn(80)), "RUN:Open Outlook", "RUN:Nope", "CMD:[\"echo\",\"hi\"]", "CMD:garbage", ""}[rng.Intn(6)]}
			case r < 92 && len(pending) > 0:
				// Deliver the result of a command Update asked for, as Bubble Tea would.
				// Timer commands (output refresh, polling) are dropped rather
				// than waited for, so the fuzz run stays fast.
				i := rng.Intn(len(pending))
				c := pending[i]
				pending = append(pending[:i], pending[i+1:]...)
				done := make(chan tea.Msg, 1)
				go func() { done <- c() }()
				select {
				case msg = <-done:
				case <-time.After(20 * time.Millisecond):
					continue
				}
				if b, ok := msg.(tea.BatchMsg); ok {
					pending = append(pending, b...)
					continue
				}
			default:
				msg = outputTickMsg{run: m.runSeq}
			}
			next, cmd := m.Update(msg)
			m = next.(Model)
			if cmd != nil && len(pending) < 50 {
				pending = append(pending, cmd)
			}
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Fatalf("seed %d step %d: View panicked after %T: %v", seed, step, msg, p)
					}
				}()
				v := m.View()
				if _, _, _, _, ok := m.layout(); ok && m.height > 3 {
					if got := strings.Count(v, "\n") + 1; got != m.height {
						t.Fatalf("seed %d step %d: view is %d lines in a %dx%d terminal", seed, step, got, m.width, m.height)
					}
				}
			}()
		}
		cancel()
	}
}
