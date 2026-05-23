package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Workspaces []Workspace `yaml:"workspaces"`
	Domains    []Domain    `yaml:"domains"`
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
			return cfg.Workspaces, nil
		}
		if len(cfg.Domains) > 0 {
			return []Workspace{{Domains: cfg.Domains}}, nil
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
