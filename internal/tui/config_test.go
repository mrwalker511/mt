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

func TestLoadDomains_NoFile_ReturnsDefaults(t *testing.T) {
	// Point config search at an empty temp dir so no file is found.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Also ensure ./mt.yaml doesn't accidentally exist.
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(t.TempDir())                //nolint:errcheck

	domains, err := LoadDomains()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(domains) != len(initialDomains) {
		t.Errorf("got %d domains, want %d (defaults)", len(domains), len(initialDomains))
	}
}

func TestLoadDomains_ValidFile(t *testing.T) {
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

	// Redirect the local-file lookup by changing cwd to the temp dir and
	// renaming the file to mt.yaml.
	dir := filepath.Dir(path)
	dest := filepath.Join(dir, "mt.yaml")
	if err := os.Rename(path, dest); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	os.Chdir(dir)                         //nolint:errcheck

	// Ensure the system config dir doesn't shadow our test file.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	domains, err := LoadDomains()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestLoadDomains_InvalidYAML_ReturnsDefaults(t *testing.T) {
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

	domains, err := LoadDomains()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	if len(domains) != len(initialDomains) {
		t.Errorf("got %d domains, want %d (defaults)", len(domains), len(initialDomains))
	}
}

func TestLoadDomains_EmptyDomains_ReturnsDefaults(t *testing.T) {
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

	domains, err := LoadDomains()
	if err == nil {
		t.Error("expected error for empty domains list")
	}
	if len(domains) != len(initialDomains) {
		t.Error("expected fallback to initial domains")
	}
}
