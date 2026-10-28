package tui

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Folder trust: a ./mt.yaml in the current folder is only loaded when that
// folder has been trusted with `mt --trust`, so cloning a repository and
// starting mt inside it cannot hand control of your targets to whoever wrote
// that repository's mt.yaml. Trusted folders are listed one per line in
// ~/.mt/trusted.

// localConfigName is the per-folder config file mt looks for.
const localConfigName = "mt.yaml"

// dirIsTrusted reports whether dir's ./mt.yaml may be loaded. Tests replace it.
var dirIsTrusted = isTrustedDir

func trustStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mt", "trusted"), nil
}

// canonicalDir returns dir as an absolute path with symlinks resolved, so a
// folder is recognised however it is reached.
func canonicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// trustedDirs reads the trust store. A missing store means nothing is trusted.
func trustedDirs() ([]string, error) {
	path, err := trustStorePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var dirs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs, sc.Err()
}

func isTrustedDir(dir string) bool {
	canon, err := canonicalDir(dir)
	if err != nil {
		return false
	}
	dirs, err := trustedDirs()
	return err == nil && slices.Contains(dirs, canon)
}

// TrustDir adds dir to the trust store and returns its canonical path.
func TrustDir(dir string) (string, error) {
	return updateTrust(dir, func(dirs []string, canon string) []string {
		if slices.Contains(dirs, canon) {
			return dirs
		}
		return append(dirs, canon)
	})
}

// UntrustDir removes dir from the trust store and returns its canonical path.
func UntrustDir(dir string) (string, error) {
	return updateTrust(dir, func(dirs []string, canon string) []string {
		return slices.DeleteFunc(dirs, func(d string) bool { return d == canon })
	})
}

func updateTrust(dir string, change func([]string, string) []string) (string, error) {
	canon, err := canonicalDir(dir)
	if err != nil {
		return "", err
	}
	dirs, err := trustedDirs()
	if err != nil {
		return "", err
	}
	path, err := trustStorePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	data := strings.Join(change(dirs, canon), "\n")
	if data != "" {
		data += "\n"
	}
	return canon, os.WriteFile(path, []byte(data), 0o600)
}
