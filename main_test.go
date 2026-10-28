package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cli runs the command line with a stub TUI and returns exit code, stdout,
// stderr, and whether the TUI started (and with mouse support or not).
func cli(t *testing.T, args ...string) (code int, out, errOut string, started, mouse bool) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code = run(args, &stdout, &stderr, func(m bool) error {
		started, mouse = true, m
		return nil
	})
	return code, stdout.String(), stderr.String(), started, mouse
}

func TestRun_NoArgsStartsTUIWithMouse(t *testing.T) {
	code, _, _, started, mouse := cli(t)
	if code != 0 || !started || !mouse {
		t.Errorf("code=%d started=%v mouse=%v", code, started, mouse)
	}
}

func TestRun_NoMouse(t *testing.T) {
	if _, _, _, started, mouse := cli(t, "--no-mouse"); !started || mouse {
		t.Errorf("started=%v mouse=%v", started, mouse)
	}
}

func TestRun_InfoFlagsDoNotStartTUI(t *testing.T) {
	for flag, want := range map[string]string{
		"--keys":    "Trackpad / mouse",
		"--help":    "mt --trust",
		"--version": "mt ",
	} {
		code, out, _, started, _ := cli(t, flag)
		if code != 0 || started || !strings.Contains(out, want) {
			t.Errorf("%s: code=%d started=%v out=%q", flag, code, started, out)
		}
	}
}

func TestRun_VersionFromLdflags(t *testing.T) {
	version = "v9.9.9"
	defer func() { version = "" }()
	if _, out, _, _, _ := cli(t, "--version"); out != "mt v9.9.9\n" {
		t.Errorf("out=%q", out)
	}
}

func TestRun_UnknownArgument(t *testing.T) {
	code, _, errOut, started, _ := cli(t, "--bogus")
	if code != 2 || started || !strings.Contains(errOut, `unknown argument "--bogus"`) {
		t.Errorf("code=%d started=%v stderr=%q", code, started, errOut)
	}
}

func TestRun_TrustAndUntrust(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	if code, out, _, _, _ := cli(t, "--trust"); code != 0 || !strings.Contains(out, "Trusted") {
		t.Fatalf("--trust: code=%d out=%q", code, out)
	}
	store, _ := os.ReadFile(filepath.Join(home, ".mt", "trusted"))
	if !strings.Contains(string(store), "project") {
		t.Errorf("trust store = %q", store)
	}
	if code, out, _, _, _ := cli(t, "--untrust"); code != 0 || !strings.Contains(out, "No longer trusting") {
		t.Fatalf("--untrust: code=%d out=%q", code, out)
	}
	store, _ = os.ReadFile(filepath.Join(home, ".mt", "trusted"))
	if strings.Contains(string(store), "project") {
		t.Errorf("folder still listed after --untrust: %q", store)
	}
}
