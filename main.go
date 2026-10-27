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
  mt --keys     print the keyboard and trackpad shortcuts
  mt --no-mouse start without trackpad/mouse support, so the terminal's own
                click-and-drag text selection works as usual
  mt --help     print this help

Config: ~/Library/Application Support/mt/config.yaml (macOS),
        ~/.config/mt/config.yaml (Linux), or ./mt.yaml
`

func main() {
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--keys", "-k", "keys":
			fmt.Print(tui.KeyHelp())
			return
		case "--help", "-h", "help":
			fmt.Print(usage)
			return
		case "--no-mouse":
			opts = []tea.ProgramOption{tea.WithAltScreen()}
		default:
			fmt.Fprintf(os.Stderr, "mt: unknown argument %q\n\n%s", arg, usage)
			os.Exit(2)
		}
	}
	p := tea.NewProgram(tui.New(), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
