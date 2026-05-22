package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	Domains []Domain `yaml:"domains"`
}

// LoadDomains reads the first config file found in the search path and returns
// the domains defined there. Falls back to initialDomains if no file exists.
// Returns an error (and initialDomains) if a file is found but cannot be parsed.
func LoadDomains() ([]Domain, error) {
	for _, p := range configPaths() {
		data, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return initialDomains, fmt.Errorf("reading %s: %w", p, err)
		}
		var cfg configFile
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return initialDomains, fmt.Errorf("parsing %s: %w", p, err)
		}
		if len(cfg.Domains) == 0 {
			return initialDomains, fmt.Errorf("%s: no domains defined", p)
		}
		return cfg.Domains, nil
	}
	return initialDomains, nil
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
