package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trustEnv gives a test its own home (and so trust store), a user config with
// one domain "User", and a project folder with a ./mt.yaml defining "Local".
// It switches the real trust check back on and cds into the project.
func trustEnv(t *testing.T) (project string) {
	t.Helper()
	dirIsTrusted = isTrustedDir
	t.Cleanup(func() { dirIsTrusted = func(string) bool { return true } })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	// Ask Go where the user config lives: XDG_CONFIG_HOME on Linux, but always
	// ~/Library/Application Support on macOS.
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "mt"), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "domains:\n  - name: User\n    targets:\n      - name: U\n        run: echo\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "mt", "config.yaml"), []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	project = filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	local := "domains:\n  - name: Local\n    targets:\n      - name: L\n        run: echo\n"
	if err := os.WriteFile(filepath.Join(project, "mt.yaml"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestTrust_UntrustedLocalConfigIgnoredWithNotice(t *testing.T) {
	trustEnv(t)
	r, err := loadConfigFiles()
	if err != nil {
		t.Fatal(err)
	}
	if r.workspaces[0].Domains[0].Name != "User" {
		t.Errorf("expected the user config, got %q", r.workspaces[0].Domains[0].Name)
	}
	if !strings.Contains(r.notice, "mt --trust") {
		t.Errorf("expected a notice explaining how to trust, got %q", r.notice)
	}
}

func TestTrust_TrustedLocalConfigOverridesUserConfig(t *testing.T) {
	project := trustEnv(t)
	if _, err := TrustDir(project); err != nil {
		t.Fatal(err)
	}
	r, err := loadConfigFiles()
	if err != nil {
		t.Fatal(err)
	}
	if r.workspaces[0].Domains[0].Name != "Local" || r.notice != "" {
		t.Errorf("expected the trusted local config, got %q (notice %q)", r.workspaces[0].Domains[0].Name, r.notice)
	}
}

func TestTrust_UntrustRevokes(t *testing.T) {
	project := trustEnv(t)
	if _, err := TrustDir(project); err != nil {
		t.Fatal(err)
	}
	if _, err := UntrustDir(project); err != nil {
		t.Fatal(err)
	}
	if isTrustedDir(project) {
		t.Error("folder still trusted after UntrustDir")
	}
}

func TestTrust_StoreIsPrivateAndDeduplicated(t *testing.T) {
	project := trustEnv(t)
	for i := 0; i < 2; i++ {
		if _, err := TrustDir(project); err != nil {
			t.Fatal(err)
		}
	}
	path, _ := trustStorePath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("trust store mode = %v, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "\n") != 1 {
		t.Errorf("expected one entry, got %q", data)
	}
}

func TestTrust_SymlinkedPathIsSameFolder(t *testing.T) {
	project := trustEnv(t)
	link := filepath.Join(filepath.Dir(project), "link")
	if err := os.Symlink(project, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := TrustDir(link); err != nil {
		t.Fatal(err)
	}
	if !isTrustedDir(project) {
		t.Error("trusting via a symlink should trust the real folder")
	}
}

func TestTrust_NoLocalConfigNoNotice(t *testing.T) {
	project := trustEnv(t)
	os.Remove(filepath.Join(project, "mt.yaml"))
	if r, _ := loadConfigFiles(); r.notice != "" {
		t.Errorf("unexpected notice %q", r.notice)
	}
}
