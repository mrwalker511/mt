package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// domainConfig is used only during YAML parsing to capture the apps: shorthand.
// It is converted to Domain via expandDomain before being stored in the Model.
type domainConfig struct {
	Name    string   `yaml:"name"`
	Apps    []string `yaml:"apps"`    // each entry expands to an open -a target
	Targets []Target `yaml:"targets"` // explicit full targets
}

// workspaceConfig mirrors Workspace but uses domainConfig for YAML parsing.
type workspaceConfig struct {
	Name    string         `yaml:"name"`
	Domains []domainConfig `yaml:"domains"`
}

type fileConfig struct {
	Workspaces []workspaceConfig `yaml:"workspaces"`
	Domains    []domainConfig    `yaml:"domains"`
}

// expandDomain converts a domainConfig to a Domain, expanding each apps: entry
// into a full Target with an open -a command. Explicit targets come first.
func expandDomain(dc domainConfig) Domain {
	d := Domain{Name: dc.Name, Targets: dc.Targets}
	for _, name := range dc.Apps {
		d.Targets = append(d.Targets, Target{
			Name:      name,
			Status:    "Press [Enter] to open",
			Cmd:       []string{"open", "-a", name},
			LaunchMsg: "Opening " + name + "…",
		})
	}
	return d
}

// LoadWorkspaces reads the first config file found in the search path and returns
// the workspaces defined there. Falls back to defaultWorkspaces if no file exists.
// Returns an error (and defaultWorkspaces) if a file is found but cannot be parsed.
func LoadWorkspaces() ([]Workspace, error) {
	for _, p := range configPaths() {
		data, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return defaultWorkspaces, fmt.Errorf("reading %s: %w", p, err)
		}
		var cfg fileConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return defaultWorkspaces, fmt.Errorf("parsing %s: %w", p, err)
		}
		if len(cfg.Workspaces) > 0 {
			workspaces := make([]Workspace, len(cfg.Workspaces))
			for i, wc := range cfg.Workspaces {
				domains := make([]Domain, len(wc.Domains))
				for j, dc := range wc.Domains {
					domains[j] = expandDomain(dc)
				}
				workspaces[i] = Workspace{Name: wc.Name, Domains: domains}
			}
			return workspaces, nil
		}
		if len(cfg.Domains) > 0 {
			domains := make([]Domain, len(cfg.Domains))
			for i, dc := range cfg.Domains {
				domains[i] = expandDomain(dc)
			}
			return []Workspace{{Domains: domains}}, nil
		}
		return defaultWorkspaces, fmt.Errorf("%s: no domains or workspaces defined", p)
	}
	return defaultWorkspaces, nil
}

// configPaths returns candidate config file locations in priority order.
func configPaths() []string {
	var paths []string
	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "mt", "config.yaml"))
	}
	paths = append(paths, "mt.yaml")
	return paths
}
