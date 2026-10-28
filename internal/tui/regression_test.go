package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_InputMode_AcceptsSpace(t *testing.T) {
	m := testModel()
	m.inputMode = true
	m = press(m, key('a'))
	m = press(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m = press(m, key('b'))
	if m.inputBuf != "a b" {
		t.Errorf("expected %q, got %q", "a b", m.inputBuf)
	}
}

func TestLoadWorkspaces_MissingInclude_ReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "include: [missing.yaml]\ndomains:\n  - name: Mine\n    targets:\n      - name: X\n        cmd: [echo]\n"
	if err := os.WriteFile(filepath.Join(dir, "mt.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                        //nolint:errcheck

	_, _, err := LoadWorkspaces()
	if err == nil || !strings.Contains(err.Error(), "missing.yaml") {
		t.Errorf("expected error naming the missing include, got %v", err)
	}
}

func TestUpdate_LLMInfo_ResetsScroll_NoPanic(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 30
	m.scrollOffset = 150
	m = send(m, llmResponseMsg{response: "INFO: " + strings.Repeat("x\n", 40)})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset reset, got %d", m.scrollOffset)
	}
	m.scrollOffset = 150 // stale offset must still render safely
	m.View()
}

func TestView_FitsTerminalHeight(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 30
	m.domains = append(m.domains, Domain{Name: "Context/Git", Targets: []Target{{Name: "Git Diff"}}})
	m.domainCursor = len(m.domains) - 1
	m.liveStatus["git.branch"] = "main"
	m.output = strings.Repeat("a fairly long line of output that will need to wrap in the pane\n", 200)
	m.cmdErr = "boom"
	if got := strings.Count(m.View(), "\n") + 1; got != m.height {
		t.Errorf("view is %d lines, want %d", got, m.height)
	}
}

func TestUpdate_Sequence_MissingStep_DoesNotRun(t *testing.T) {
	m := testModel()
	m.domains[0].Targets = append(m.domains[0].Targets,
		Target{Name: "Pipeline", Sequence: []string{"T1", "Tset"}})
	m.domains[0].Targets[0].Cmd = []string{"true"}
	m.activePane = paneMiddle
	m.targetCursor = 3
	m, cmd := m.startTarget("Domain A", m.domains[0].Targets[3])
	if cmd != nil || m.running {
		t.Fatal("sequence with a missing step must not start")
	}
	if !strings.Contains(m.cmdErr, "Tset") {
		t.Errorf("expected error naming the missing step, got %q", m.cmdErr)
	}
}

func TestLoadWorkspaces_Include_TildeExpandsToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	inc := "domains:\n  - name: Shared\n    targets:\n      - name: S\n        cmd: [echo]\n"
	if err := os.WriteFile(filepath.Join(home, "shared", "team.yaml"), []byte(inc), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "include: [~/shared/team.yaml]\ndomains:\n  - name: Mine\n    targets:\n      - name: X\n        cmd: [echo]\n"
	if err := os.WriteFile(filepath.Join(dir, "mt.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                        //nolint:errcheck

	ws, _, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ws[0].Domains); got != 2 {
		t.Errorf("expected 2 domains (local + included), got %d", got)
	}
}
