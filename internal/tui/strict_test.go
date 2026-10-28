package tui

import (
	"os"
	"strings"
	"testing"
)

func TestLoadWorkspaces_UnknownKey_ExplainedWithSuggestion(t *testing.T) {
	_, err := loadConfig(t, `
domains:
  - name: Dev
    targets:
      - name: X
        comand: ["echo", "hi"]
`)
	if err == nil {
		t.Fatal("expected an error for the misspelled key")
	}
	for _, want := range []string{"mt.yaml", "line 6", `unknown key "comand"`, `did you mean "cmd"?`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestLoadWorkspaces_UnknownKey_NoCloseMatch(t *testing.T) {
	_, err := loadConfig(t, "domains:\n  - name: Dev\n    banana: 1\n")
	if err == nil || !strings.Contains(err.Error(), `unknown key "banana"`) || strings.Contains(err.Error(), "did you mean") {
		t.Errorf("expected plain unknown-key error, got %v", err)
	}
}

func TestLoadWorkspaces_UnknownLLMKey(t *testing.T) {
	_, err := loadConfig(t, "llm:\n  provder: mlx\ndomains:\n  - name: D\n    targets:\n      - name: X\n        run: echo\n")
	if err == nil || !strings.Contains(err.Error(), `did you mean "provider"?`) {
		t.Errorf("expected provider suggestion, got %v", err)
	}
}

func TestLoadWorkspaces_EmptyFile(t *testing.T) {
	_, err := loadConfig(t, "")
	if err == nil || !strings.Contains(err.Error(), "no domains or workspaces") {
		t.Errorf("expected 'no domains' error for an empty file, got %v", err)
	}
}

func TestExampleConfig_LoadsStrictly(t *testing.T) {
	data, err := os.ReadFile("../../mt.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := loadConfig(t, string(data))
	if err != nil {
		t.Fatalf("mt.yaml.example no longer loads: %v", err)
	}
	if len(ws) < 2 {
		t.Errorf("expected several workspaces, got %d", len(ws))
	}
}
