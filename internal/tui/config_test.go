package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "mt-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoadWorkspaces_NoFile_ReturnsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(t.TempDir())                //nolint:errcheck

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(workspaces) != 1 || len(workspaces[0].Domains) != len(initialDomains) {
		t.Errorf("got %d workspace(s) with %d domains, want 1 workspace with %d domains (defaults)",
			len(workspaces), func() int {
				if len(workspaces) > 0 {
					return len(workspaces[0].Domains)
				}
				return 0
			}(), len(initialDomains))
	}
}

func TestLoadWorkspaces_ValidDomainsKey(t *testing.T) {
	yaml := `
domains:
  - name: "Test Domain"
    targets:
      - name: "Test Target"
        status: "hint"
        cmd: ["echo", "hello"]
        launch_msg: "Launched"
`
	path := writeTemp(t, yaml)

	dir := filepath.Dir(path)
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.Rename(path, dest); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                         //nolint:errcheck
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(workspaces))
	}
	domains := workspaces[0].Domains
	if len(domains) != 1 {
		t.Fatalf("got %d domains, want 1", len(domains))
	}
	if domains[0].Name != "Test Domain" {
		t.Errorf("domain name: got %q, want %q", domains[0].Name, "Test Domain")
	}
	if len(domains[0].Targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(domains[0].Targets))
	}
	tgt := domains[0].Targets[0]
	if tgt.Name != "Test Target" {
		t.Errorf("target name: got %q, want %q", tgt.Name, "Test Target")
	}
	if tgt.LaunchMsg != "Launched" {
		t.Errorf("launch_msg: got %q, want %q", tgt.LaunchMsg, "Launched")
	}
}

func TestLoadWorkspaces_WorkspacesKey(t *testing.T) {
	yaml := `
workspaces:
  - name: "Alpha"
    domains:
      - name: "Domain A"
        targets:
          - name: "Target 1"
  - name: "Beta"
    domains:
      - name: "Domain B"
        targets:
          - name: "Target 2"
`
	path := writeTemp(t, yaml)
	dir := filepath.Dir(path)
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.Rename(path, dest); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                         //nolint:errcheck
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	workspaces, err := LoadWorkspaces()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("got %d workspaces, want 2", len(workspaces))
	}
	if workspaces[0].Name != "Alpha" || workspaces[1].Name != "Beta" {
		t.Errorf("got names %q, %q; want Alpha, Beta", workspaces[0].Name, workspaces[1].Name)
	}
}

func TestLoadWorkspaces_InvalidYAML_ReturnsDefaults(t *testing.T) {
	path := writeTemp(t, "domains: [[[invalid")

	dir := filepath.Dir(path)
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.Rename(path, dest); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                         //nolint:errcheck
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	workspaces, err := LoadWorkspaces()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	if len(workspaces) != 1 || len(workspaces[0].Domains) != len(initialDomains) {
		t.Error("expected fallback to default workspaces")
	}
}

func TestLoadWorkspaces_EmptyDomains_ReturnsDefaults(t *testing.T) {
	path := writeTemp(t, "domains: []\n")

	dir := filepath.Dir(path)
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.Rename(path, dest); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                         //nolint:errcheck
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	workspaces, err := LoadWorkspaces()
	if err == nil {
		t.Error("expected error for empty domains list")
	}
	if len(workspaces) != 1 || len(workspaces[0].Domains) != len(initialDomains) {
		t.Error("expected fallback to default workspaces")
	}
}
