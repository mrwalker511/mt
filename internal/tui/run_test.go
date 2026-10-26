package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"git status", []string{"git", "status"}},
		{"  git   log  -n 5 ", []string{"git", "log", "-n", "5"}},
		{"open -a 'Microsoft Word'", []string{"open", "-a", "Microsoft Word"}},
		{`open -a "Visual Studio Code"`, []string{"open", "-a", "Visual Studio Code"}},
		{`git commit -m "say \"hi\""`, []string{"git", "commit", "-m", `say "hi"`}},
		{`echo My\ Dir`, []string{"echo", "My Dir"}},
		{`awk '{print $1}'`, []string{"awk", "{print $1}"}},
		{`echo "a | b" 'c;d'`, []string{"echo", "a | b", "c;d"}},
		{`printf ''`, []string{"printf", ""}},
		{`pre'fix'"ed"`, []string{"prefixed"}},
		{"ls a~b", []string{"ls", "a~b"}},
	}
	for _, c := range cases {
		got, err := splitCommand(c.in)
		if err != nil {
			t.Errorf("splitCommand(%q): unexpected error %v", c.in, err)
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("splitCommand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitCommand_Errors(t *testing.T) {
	cases := map[string]string{
		"":                   "empty",
		"   ":                "empty",
		"echo 'oops":         "unterminated '",
		`echo "oops`:         `unterminated "`,
		`echo oops\`:         "trailing backslash",
		"ls | grep x":        "shell syntax",
		"make build && make": "shell syntax",
		"echo hi > out.txt":  "shell syntax",
		"echo $HOME":         "shell syntax",
		"code ~/projects":    "shell syntax",
		"echo `date`":        "shell syntax",
		"sleep 1; echo done": "shell syntax",
	}
	for in, want := range cases {
		_, err := splitCommand(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("splitCommand(%q): want error containing %q, got %v", in, want, err)
		}
	}
}

func loadConfig(t *testing.T, yaml string) ([]Workspace, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mt.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                        //nolint:errcheck
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ws, _, err := LoadWorkspaces()
	return ws, err
}

func TestLoadWorkspaces_RunShorthand(t *testing.T) {
	ws, err := loadConfig(t, `
domains:
  - name: Dev
    targets:
      - name: Git Log
        run: git log --oneline -n 10
      - name: Word
        run: open -a "Microsoft Word"
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	targets := ws[0].Domains[0].Targets
	if !slices.Equal(targets[0].Cmd, []string{"git", "log", "--oneline", "-n", "10"}) {
		t.Errorf("Git Log cmd = %q", targets[0].Cmd)
	}
	if !slices.Equal(targets[1].Cmd, []string{"open", "-a", "Microsoft Word"}) {
		t.Errorf("Word cmd = %q", targets[1].Cmd)
	}
}

func TestLoadWorkspaces_RunAndCmd_IsError(t *testing.T) {
	_, err := loadConfig(t, `
domains:
  - name: Dev
    targets:
      - name: Both
        run: git status
        cmd: ["git", "status"]
`)
	if err == nil || !strings.Contains(err.Error(), "either run or cmd") {
		t.Errorf("expected run/cmd conflict error, got %v", err)
	}
}

func TestLoadWorkspaces_RunShellSyntax_NamesTarget(t *testing.T) {
	_, err := loadConfig(t, `
domains:
  - name: Dev
    targets:
      - name: Count Go Files
        run: ls | wc -l
`)
	if err == nil || !strings.Contains(err.Error(), `"Count Go Files"`) || !strings.Contains(err.Error(), "sh") {
		t.Errorf("expected error naming the target and suggesting sh -c, got %v", err)
	}
}

func TestLoadWorkspaces_RunOnSSHTarget_SentWhole(t *testing.T) {
	ws, err := loadConfig(t, `
domains:
  - name: Prod
    targets:
      - name: Errors
        host: deploy@prod.example.com
        run: journalctl -u myapp | grep -i error
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := effectiveCmd(ws[0].Domains[0].Targets[0])
	want := []string{"ssh", "deploy@prod.example.com", "journalctl -u myapp | grep -i error"}
	if !slices.Equal(got, want) {
		t.Errorf("effectiveCmd = %q, want %q", got, want)
	}
}

func TestDisplayCmd(t *testing.T) {
	cases := []struct {
		t    Target
		want string
	}{
		{Target{Run: `open -a "Microsoft Word"`, Cmd: []string{"open", "-a", "Microsoft Word"}}, `open -a "Microsoft Word"`},
		{Target{Run: "uptime", Host: "me@box", Cmd: []string{"uptime"}}, "ssh me@box uptime"},
		{Target{Cmd: []string{"open", "-a", "Microsoft Word"}}, "open -a 'Microsoft Word'"},
		{Target{Cmd: []string{"echo", "it's"}}, `echo 'it'\''s'`},
	}
	for _, c := range cases {
		if got := displayCmd(c.t); got != c.want {
			t.Errorf("displayCmd(%+v) = %q, want %q", c.t, got, c.want)
		}
	}
}
