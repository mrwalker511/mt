package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mrwalker511/mt/internal/llm"
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
	Include    []string          `yaml:"include"`
	Workspaces []workspaceConfig `yaml:"workspaces"`
	Domains    []domainConfig    `yaml:"domains"`
	LLM        llm.Config        `yaml:"llm"`
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
// the workspaces and LLM config defined there. Falls back to defaultWorkspaces if
// no file exists. Returns an error (and defaultWorkspaces) if a file is found but
// cannot be parsed. Files may declare include: paths to merge domains/workspaces.
func LoadWorkspaces() ([]Workspace, llm.Config, error) {
	for _, p := range configPaths() {
		cfg, err := resolveConfig(p, make(map[string]bool))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return defaultWorkspaces, llm.Config{}, err
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
			if err := validateWorkspaces(workspaces); err != nil {
				return defaultWorkspaces, llm.Config{}, err
			}
			return workspaces, cfg.LLM, nil
		}
		if len(cfg.Domains) > 0 {
			domains := make([]Domain, len(cfg.Domains))
			for i, dc := range cfg.Domains {
				domains[i] = expandDomain(dc)
			}
			ws := []Workspace{{Domains: domains}}
			if err := validateWorkspaces(ws); err != nil {
				return defaultWorkspaces, llm.Config{}, err
			}
			return ws, cfg.LLM, nil
		}
		return defaultWorkspaces, llm.Config{}, fmt.Errorf("%s: no domains or workspaces defined", p)
	}
	return defaultWorkspaces, llm.Config{}, nil
}

// resolveConfig loads a config file and recursively merges any include: entries into it.
// visited guards against circular includes.
func resolveConfig(path string, visited map[string]bool) (fileConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("resolving path %s: %w", path, err)
	}
	if visited[absPath] {
		return fileConfig{}, nil // cycle guard
	}
	visited[absPath] = true

	if info, statErr := os.Stat(absPath); statErr == nil && info.Mode().Perm()&0002 != 0 {
		return fileConfig{}, fmt.Errorf("refusing to load %s: file is world-writable", absPath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fileConfig{}, fmt.Errorf("reading %s: %w", absPath, err)
	}
	var cfg fileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("parsing %s: %w", absPath, err)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return fileConfig{}, fmt.Errorf("cannot determine home directory: %w", homeErr)
	}
	dir := filepath.Dir(absPath)
	for _, inc := range cfg.Include {
		if !filepath.IsAbs(inc) {
			inc = filepath.Join(dir, inc)
		}
		// Resolve symlinks so a link inside $HOME pointing outside cannot escape.
		if real, err := filepath.EvalSymlinks(inc); err == nil {
			inc = real
		}
		if !strings.HasPrefix(inc, home+string(filepath.Separator)) {
			return cfg, fmt.Errorf("include %s: path must be within home directory (%s)", inc, home)
		}
		incCfg, err := resolveConfig(inc, visited)
		if err != nil {
			return cfg, fmt.Errorf("include %s: %w", inc, err)
		}
		cfg.Workspaces = append(cfg.Workspaces, incCfg.Workspaces...)
		cfg.Domains = append(cfg.Domains, incCfg.Domains...)
	}
	return cfg, nil
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
