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

// expandDomain converts a domainConfig to a Domain: each apps: entry becomes a
// full Target with an open -a command (after the explicit targets), and each
// run: string becomes Cmd (split into words locally; kept whole for SSH targets).
func expandDomain(dc domainConfig) (Domain, error) {
	d := Domain{Name: dc.Name, Targets: make([]Target, 0, len(dc.Targets)+len(dc.Apps))}
	for _, t := range dc.Targets {
		if t.Run != "" {
			if len(t.Cmd) > 0 {
				return Domain{}, fmt.Errorf("target %q: set either run or cmd, not both", t.Name)
			}
			if t.Host != "" {
				// ssh hands its command to the remote shell, so send the string
				// exactly as written: pipes, quotes and $VARS work remotely.
				if strings.TrimSpace(t.Run) == "" {
					return Domain{}, fmt.Errorf("target %q: run: empty command", t.Name)
				}
				t.Cmd = []string{t.Run}
			} else {
				cmd, err := splitCommand(t.Run)
				if err != nil {
					return Domain{}, fmt.Errorf("target %q: run: %w", t.Name, err)
				}
				t.Cmd = cmd
			}
		}
		d.Targets = append(d.Targets, t)
	}
	for _, name := range dc.Apps {
		d.Targets = append(d.Targets, Target{
			Name:      name,
			Status:    "Press [Enter] to open",
			Cmd:       []string{"open", "-a", name},
			LaunchMsg: "Opening " + name + "…",
		})
	}
	return d, nil
}

// splitCommand splits a run: string into arguments the way a shell splits
// words: whitespace separates them, 'single quotes' keep text literally, and
// "double quotes" keep text but allow \" and \\ escapes. A backslash outside
// quotes escapes the next character. Nothing is expanded, so unquoted shell
// syntax (pipes, redirects, $VARS, ~) is rejected rather than silently passed
// through as literal text.
func splitCommand(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inWord := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				args = append(args, cur.String())
				cur.Reset()
				inWord = false
			}
		case r == '\'':
			end := indexRune(runes, i+1, '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated ' quote in %q", s)
			}
			cur.WriteString(string(runes[i+1 : end]))
			i, inWord = end, true
		case r == '"':
			i++
			for ; i < len(runes) && runes[i] != '"'; i++ {
				if runes[i] == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\') {
					i++
				}
				cur.WriteRune(runes[i])
			}
			if i == len(runes) {
				return nil, fmt.Errorf("unterminated \" quote in %q", s)
			}
			inWord = true
		case r == '\\':
			if i+1 == len(runes) {
				return nil, fmt.Errorf("trailing backslash in %q", s)
			}
			i++
			cur.WriteRune(runes[i])
			inWord = true
		case strings.ContainsRune("|&;<>`$", r) || (r == '~' && !inWord):
			return nil, fmt.Errorf("%q: %q is shell syntax, but mt does not run commands through a shell; quote it to pass it literally, or use cmd: [\"sh\", \"-c\", \"...\"]", s, string(r))
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		args = append(args, cur.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return args, nil
}

// indexRune returns the index of the first r in runes at or after from, or -1.
func indexRune(runes []rune, from int, r rune) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == r {
			return i
		}
	}
	return -1
}

// LoadWorkspaces reads the first config file found in the search path and returns
// the workspaces and LLM config defined there. Falls back to defaultWorkspaces if
// no file exists. Returns an error (and defaultWorkspaces) if a file is found but
// is invalid, including when one of its include: files is missing.
func LoadWorkspaces() ([]Workspace, llm.Config, error) {
	for _, p := range configPaths() {
		// Only a missing top-level file moves on to the next candidate. Checking
		// here, rather than the error from resolveConfig, keeps a missing include
		// from silently discarding the whole config.
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			continue
		}
		cfg, err := resolveConfig(p, make(map[string]bool))
		if err != nil {
			return defaultWorkspaces, llm.Config{}, err
		}
		wcs := cfg.Workspaces
		if len(wcs) == 0 && len(cfg.Domains) > 0 {
			wcs = []workspaceConfig{{Domains: cfg.Domains}}
		}
		if len(wcs) == 0 {
			return defaultWorkspaces, llm.Config{}, fmt.Errorf("%s: no domains or workspaces defined", p)
		}
		workspaces := make([]Workspace, len(wcs))
		for i, wc := range wcs {
			domains := make([]Domain, len(wc.Domains))
			for j, dc := range wc.Domains {
				if domains[j], err = expandDomain(dc); err != nil {
					return defaultWorkspaces, llm.Config{}, fmt.Errorf("%s: %w", p, err)
				}
			}
			workspaces[i] = Workspace{Name: wc.Name, Domains: domains}
		}
		if err := validateWorkspaces(workspaces); err != nil {
			return defaultWorkspaces, llm.Config{}, err
		}
		return workspaces, cfg.LLM, nil
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
	// Resolve symlinks in home so the prefix check is consistent with the
	// symlink-resolved include paths below (on macOS /var → /private/var).
	if realHome, err := filepath.EvalSymlinks(home); err == nil {
		home = realHome
	}
	dir := filepath.Dir(absPath)
	for _, inc := range cfg.Include {
		if rest, ok := strings.CutPrefix(inc, "~/"); ok {
			inc = filepath.Join(home, rest)
		}
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
