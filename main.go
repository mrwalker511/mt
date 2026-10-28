package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrwalker511/mt/internal/tui"
)

// version is set by release builds: go build -ldflags "-X main.version=v1.2.3".
var version = ""

const usage = `mt: a keyboard-driven terminal workspace launcher

Usage:
  mt              start the launcher
  mt --keys       print the keyboard and trackpad shortcuts
  mt --no-mouse   start without trackpad/mouse support, so the terminal's own
                  click-and-drag text selection works as usual
  mt --trust      trust this folder, so mt loads its ./mt.yaml
  mt --untrust    stop trusting this folder
  mt --version    print the version
  mt --help       print this help

Config: ./mt.yaml in a trusted folder, otherwise
        ~/Library/Application Support/mt/config.yaml (macOS) or
        ~/.config/mt/config.yaml (Linux)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, startTUI))
}

// run handles the command line and returns the exit code. start launches the
// TUI; tests pass a stub.
func run(args []string, stdout, stderr io.Writer, start func(mouse bool) error) int {
	mouse := true
	for _, arg := range args {
		switch arg {
		case "--keys", "-k", "keys":
			fmt.Fprint(stdout, tui.KeyHelp())
			return 0
		case "--help", "-h", "help":
			fmt.Fprint(stdout, usage)
			return 0
		case "--version", "-v", "version":
			fmt.Fprintln(stdout, "mt", resolveVersion())
			return 0
		case "--trust":
			dir, err := tui.TrustDir(".")
			if err != nil {
				fmt.Fprintln(stderr, "mt: trust:", err)
				return 1
			}
			fmt.Fprintf(stdout, "Trusted %s: mt will load ./mt.yaml in this folder.\n", dir)
			return 0
		case "--untrust":
			dir, err := tui.UntrustDir(".")
			if err != nil {
				fmt.Fprintln(stderr, "mt: untrust:", err)
				return 1
			}
			fmt.Fprintf(stdout, "No longer trusting %s.\n", dir)
			return 0
		case "--no-mouse":
			mouse = false
		default:
			fmt.Fprintf(stderr, "mt: unknown argument %q\n\n%s", arg, usage)
			return 2
		}
	}
	if err := start(mouse); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func startTUI(mouse bool) error {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	_, err := tea.NewProgram(tui.New(), opts...).Run()
	return err
}

// resolveVersion returns the release version, or for other builds the module
// version (set by `go install ...@v1.2.3`) plus the commit it was built from.
func resolveVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	v := info.Main.Version
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			v += " " + s.Value[:7]
		}
		if s.Key == "vcs.modified" && s.Value == "true" {
			v += "-dirty"
		}
	}
	return v
}
