package tui

import (
	"bytes"
	"strings"
	"testing"
)

func check(t *testing.T, yaml string) (int, string) {
	t.Helper()
	if _, err := loadConfig(t, yaml); err != nil && !strings.Contains(yaml, "BROKEN") {
		t.Fatalf("config failed to load: %v", err)
	}
	var out bytes.Buffer
	code := Check(&out)
	return code, out.String()
}

func TestCheck_CleanConfig(t *testing.T) {
	code, out := check(t, "domains:\n  - name: D\n    live: echo hi\n    targets:\n      - name: T\n        run: echo hi\n      - name: Hint\n        status: just a note\n")
	if code != 0 || !strings.Contains(out, "Ready: no problems found.") || !strings.Contains(out, "mt.yaml") {
		t.Errorf("code=%d\n%s", code, out)
	}
}

func TestCheck_ConfigError(t *testing.T) {
	code, out := check(t, "# BROKEN\ndomains:\n  - name: D\n    targets:\n      - name: T\n        comand: [echo]\n")
	if code != 1 || !strings.Contains(out, "✗ Config error") || !strings.Contains(out, `did you mean "cmd"?`) {
		t.Errorf("code=%d\n%s", code, out)
	}
}

func TestCheck_ReportsEverythingThatWouldGetInTheWay(t *testing.T) {
	code, out := check(t, `
llm:
  provider: olama
domains:
  - name: Context/Git
    targets:
      - name: Pipeline
        sequence: [Build, Tset]
      - name: Build
        run: definitely-not-a-real-program --x
      - name: Here
        run: echo hi
        dir: /no/such/folder
`)
	for _, want := range []string{
		`✗ Context/Git › Pipeline: sequence "Pipeline": step "Tset" not found`,
		`"definitely-not-a-real-program" is not installed or not on your PATH (used by Build)`,
		"dir /no/such/folder does not exist",
		`unknown llm provider "olama"`,
		`Domain "Context/Git" no longer gets a status header`,
		"1 problem(s) and 4 warning(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if code != 1 {
		t.Errorf("a broken sequence should fail the check, code=%d", code)
	}
}

func TestCheck_WarningsOnlyStillReady(t *testing.T) {
	code, out := check(t, "domains:\n  - name: D\n    targets:\n      - name: T\n        run: definitely-not-a-real-program\n")
	if code != 0 || !strings.Contains(out, "Ready, with 1 warning(s)") {
		t.Errorf("code=%d\n%s", code, out)
	}
}

func TestCheck_UntrustedFolderNotice(t *testing.T) {
	trustEnv(t)
	var out bytes.Buffer
	Check(&out)
	if !strings.Contains(out.String(), "! Ignored ./mt.yaml") || !strings.Contains(out.String(), "config.yaml") {
		t.Errorf("expected the notice and the user config path:\n%s", out.String())
	}
}
