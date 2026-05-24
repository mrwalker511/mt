package tui

import (
	"os"
	"path/filepath"
	"slices"
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

func TestLoadWorkspaces_AppsKey_ExpandsToTargets(t *testing.T) {
	yaml := `
domains:
  - name: "My Apps"
    apps:
      - "Microsoft Edge"
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
	targets := workspaces[0].Domains[0].Targets
	if len(targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(targets))
	}
	tgt := targets[0]
	if tgt.Name != "Microsoft Edge" {
		t.Errorf("name: got %q, want %q", tgt.Name, "Microsoft Edge")
	}
	if want := []string{"open", "-a", "Microsoft Edge"}; !slices.Equal(tgt.Cmd, want) {
		t.Errorf("cmd: got %v, want %v", tgt.Cmd, want)
	}
	if tgt.LaunchMsg == "" {
		t.Error("expected non-empty launch_msg from apps: expansion")
	}
}

func TestLoadWorkspaces_AppsAndTargets_OrderPreserved(t *testing.T) {
	yaml := `
domains:
  - name: "Mixed"
    targets:
      - name: "Explicit First"
        cmd: ["echo", "first"]
    apps:
      - "App Second"
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
	targets := workspaces[0].Domains[0].Targets
	if len(targets) != 2 {
		t.Fatalf("got %d targets, want 2", len(targets))
	}
	if targets[0].Name != "Explicit First" {
		t.Errorf("first target: got %q, want %q", targets[0].Name, "Explicit First")
	}
	if targets[1].Name != "App Second" {
		t.Errorf("second target: got %q, want %q", targets[1].Name, "App Second")
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

func TestLoadWorkspaces_Include_MergesDomains(t *testing.T) {
	dir := t.TempDir()

	included := `
domains:
  - name: "Included Domain"
    targets:
      - name: "Inc Target"
        cmd: ["echo", "included"]
`
	incPath := filepath.Join(dir, "extra.yaml")
	if err := os.WriteFile(incPath, []byte(included), 0600); err != nil {
		t.Fatal(err)
	}

	main := `
include:
  - extra.yaml
domains:
  - name: "Main Domain"
    targets:
      - name: "Main Target"
`
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.WriteFile(dest, []byte(main), 0600); err != nil {
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
	if len(domains) != 2 {
		t.Fatalf("got %d domains, want 2 (main + included)", len(domains))
	}
	names := []string{domains[0].Name, domains[1].Name}
	if !slices.Contains(names, "Main Domain") || !slices.Contains(names, "Included Domain") {
		t.Errorf("unexpected domain names: %v", names)
	}
}

func TestLoadWorkspaces_Include_CycleGuard(t *testing.T) {
	dir := t.TempDir()

	// a.yaml includes b.yaml which includes a.yaml — should not loop.
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")
	a := "include:\n  - b.yaml\ndomains:\n  - name: A\n    targets:\n      - name: TA\n"
	b := "include:\n  - a.yaml\ndomains:\n  - name: B\n    targets:\n      - name: TB\n"
	if err := os.WriteFile(aPath, []byte(a), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte(b), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(aPath, make(map[string]bool))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Domains) == 0 {
		t.Error("expected at least one domain from cyclic include")
	}
}

func TestLoadWorkspaces_SSHHostField(t *testing.T) {
	yaml := `
domains:
  - name: "Remote"
    targets:
      - name: "Deploy"
        host: "user@prod.example.com"
        cmd: ["./deploy.sh"]
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
	tgt := workspaces[0].Domains[0].Targets[0]
	if tgt.Host != "user@prod.example.com" {
		t.Errorf("host: got %q, want %q", tgt.Host, "user@prod.example.com")
	}
}

func TestLoadWorkspaces_SequenceField(t *testing.T) {
	yaml := `
domains:
  - name: "CI"
    targets:
      - name: "Full Pipeline"
        sequence: ["Build", "Test", "Deploy"]
      - name: "Build"
        cmd: ["make", "build"]
      - name: "Test"
        cmd: ["make", "test"]
      - name: "Deploy"
        cmd: ["make", "deploy"]
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
	tgt := workspaces[0].Domains[0].Targets[0]
	if tgt.Name != "Full Pipeline" {
		t.Fatalf("unexpected target name: %q", tgt.Name)
	}
	if want := []string{"Build", "Test", "Deploy"}; !slices.Equal(tgt.Sequence, want) {
		t.Errorf("sequence: got %v, want %v", tgt.Sequence, want)
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
