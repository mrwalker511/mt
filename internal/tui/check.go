package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrwalker511/mt/internal/llm"
)

// Check loads the configuration exactly as mt would and reports anything
// that would get in the way of using it, without starting the interface:
// config errors, an ignored ./mt.yaml, broken sequences, programs that are
// not installed, missing dir: folders and an unusable AI provider. Warnings
// do not stop mt from working. It returns 1 if there are problems, else 0.
func Check(w io.Writer) int {
	r, err := loadConfigFiles()
	if err != nil {
		fmt.Fprintf(w, "✗ Config error: %v\n  mt would start with its built-in defaults until this is fixed.\n", err)
		return 1
	}

	domains, targets := 0, 0
	for _, ws := range r.workspaces {
		domains += len(ws.Domains)
		for _, d := range ws.Domains {
			targets += len(d.Targets)
		}
	}
	src := "built-in defaults (no config file found)"
	if r.source != "" {
		src = displayPath(r.source)
	}
	fmt.Fprintf(w, "✓ Config: %s\n  %d workspace(s), %d domain(s), %d target(s)\n", src, len(r.workspaces), domains, targets)
	if r.notice != "" {
		fmt.Fprintln(w, "! "+r.notice)
	}

	var problems, warnings []string
	missing := map[string][]string{} // program -> targets that run it
	for _, ws := range r.workspaces {
		m := Model{domains: ws.Domains}
		for _, d := range ws.Domains {
			if r.source != "" && len(d.Live) == 0 && (d.Name == "Context/Git" || d.Name == "Infrastructure") {
				warnings = append(warnings, fmt.Sprintf("Domain %q no longer gets a status header automatically; add a live: line (see mt.yaml.example).", d.Name))
			}
			if len(d.Live) > 0 && !onPath(d.Live[0]) {
				missing[d.Live[0]] = append(missing[d.Live[0]], d.Name+" (live:)")
			}
			for _, t := range d.Targets {
				where := d.Name + " › " + t.Name
				if ws.Name != "" {
					where = ws.Name + " › " + where
				}
				if len(t.Sequence) > 0 {
					if _, err := m.stepsFor(d.Name, t); err != nil {
						problems = append(problems, where+": "+err.Error())
					}
					continue
				}
				if len(t.Cmd) == 0 {
					continue // a hint-only target
				}
				prog := t.Cmd[0]
				if t.Host != "" {
					prog = "ssh"
				}
				if !onPath(prog) {
					missing[prog] = append(missing[prog], t.Name)
				}
				if t.Dir != "" && t.Host == "" {
					if info, err := os.Stat(t.Dir); err != nil || !info.IsDir() {
						warnings = append(warnings, fmt.Sprintf("%s: dir %s does not exist", where, displayPath(t.Dir)))
					}
				}
			}
		}
	}
	progs := make([]string, 0, len(missing))
	for p := range missing {
		progs = append(progs, p)
	}
	sort.Strings(progs)
	for _, p := range progs {
		warnings = append(warnings, fmt.Sprintf("%q is not installed or not on your PATH (used by %s)", p, strings.Join(missing[p], ", ")))
	}

	desc, problem := llm.Describe(r.llm)
	if problem != "" {
		warnings = append(warnings, "AI assistant ("+desc+"): "+problem)
	} else {
		fmt.Fprintf(w, "✓ AI assistant: %s (start it before pressing /)\n", desc)
	}

	for _, p := range problems {
		fmt.Fprintln(w, "✗ "+p)
	}
	for _, wn := range warnings {
		fmt.Fprintln(w, "! "+wn)
	}
	switch {
	case len(problems) > 0:
		fmt.Fprintf(w, "\n%d problem(s) and %d warning(s). mt will run, but the targets marked ✗ will refuse to start.\n", len(problems), len(warnings))
		return 1
	case len(warnings) > 0:
		fmt.Fprintf(w, "\nReady, with %d warning(s): the targets above will fail until fixed.\n", len(warnings))
	default:
		fmt.Fprintln(w, "\nReady: no problems found.")
	}
	return 0
}

// onPath reports whether prog can be run: a path that exists, or a name
// found on PATH.
func onPath(prog string) bool {
	if strings.Contains(prog, "/") {
		_, err := os.Stat(prog)
		return err == nil
	}
	_, err := exec.LookPath(prog)
	return err == nil
}

// displayPath shortens a path under the home folder to ~/….
func displayPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(abs, home+string(filepath.Separator)); ok {
			return "~/" + rest
		}
	}
	return abs
}
