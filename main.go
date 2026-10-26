package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrwalker511/mt/internal/tui"
)

const usage = `mt: a keyboard-driven terminal workspace launcher

Usage:
  mt            start the launcher
  mt --keys     print the keyboard shortcuts
  mt --help     print this help

Config: ~/Library/Application Support/mt/config.yaml (macOS),
        ~/.config/mt/config.yaml (Linux), or ./mt.yaml
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--keys", "-k", "keys":
			fmt.Print(tui.KeyHelp())
			return
		case "--help", "-h", "help":
			fmt.Print(usage)
			return
		default:
			fmt.Fprintf(os.Stderr, "mt: unknown argument %q\n\n%s", os.Args[1], usage)
			os.Exit(2)
		}
	}
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
