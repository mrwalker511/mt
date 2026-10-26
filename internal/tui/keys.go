package tui

import (
	"fmt"
	"strings"
)

// action is something the user can do from the main view.
type action int

const (
	actNone action = iota
	actUp
	actDown
	actPaneLeft
	actPaneRight
	actPrevPane
	actNextPane
	actPageUp
	actPageDown
	actTop
	actBottom
	actRun
	actSelect
	actCancel
	actAI
	actCopy
	actSave
	actClear
	actReload
	actPrevWorkspace
	actNextWorkspace
	actJumpWorkspace
	actHelp
	actQuit
)

// binding ties keys to an action. keys are Bubble Tea key strings; label shows
// the key with Mac keyboard symbols. Bindings with a short name appear in the
// status bar. Most letter keys mirror the ⌘ shortcut a Mac app would use, since
// a terminal app never receives ⌘ itself.
type binding struct {
	action    action
	keys      []string
	label     string
	short     string // status-bar name; empty keeps it out of the bar
	desc      string
	group     string
	workspace bool // only relevant when more than one workspace is configured
}

var bindings = []binding{
	{action: actUp, keys: []string{"up", "k"}, label: "↑", desc: "Move up (scrolls output in the right pane)", group: "Move"},
	{action: actDown, keys: []string{"down", "j"}, label: "↓", desc: "Move down (scrolls output in the right pane)", group: "Move"},
	{action: actPaneLeft, keys: []string{"left", "h"}, label: "←", desc: "Focus the pane to the left", group: "Move"},
	{action: actPaneRight, keys: []string{"right", "l"}, label: "→", desc: "Focus the pane to the right", group: "Move"},
	{action: actNextPane, keys: []string{"tab"}, label: "⇥", short: "Pane", desc: "Next pane (wraps around)", group: "Move"},
	{action: actPrevPane, keys: []string{"shift+tab"}, label: "⇧⇥", desc: "Previous pane (wraps around)", group: "Move"},
	{action: actPageUp, keys: []string{"pgup"}, label: "fn↑", desc: "Page output up", group: "Move"},
	{action: actPageDown, keys: []string{"pgdown"}, label: "fn↓", desc: "Page output down", group: "Move"},
	{action: actTop, keys: []string{"home"}, label: "fn←", desc: "Jump to top of output", group: "Move"},
	{action: actBottom, keys: []string{"end"}, label: "fn→", desc: "Jump to bottom of output", group: "Move"},

	{action: actRun, keys: []string{"enter"}, label: "⏎", short: "Run", desc: "Run target (or every selected target)", group: "Run"},
	{action: actSelect, keys: []string{" "}, label: "␣", short: "Select", desc: "Select target for a parallel run", group: "Run"},
	{action: actCancel, keys: []string{"esc"}, label: "⎋", desc: "Stop running command / cancel AI", group: "Run"},
	{action: actAI, keys: []string{"/"}, label: "/", short: "Ask AI", desc: "Ask the AI assistant", group: "Run"},

	{action: actCopy, keys: []string{"c", "y"}, label: "c", short: "Copy", desc: "Copy output (like ⌘C)", group: "Output"},
	{action: actSave, keys: []string{"s", "S"}, label: "s", desc: "Save output to ~/.mt/logs (like ⌘S)", group: "Output"},
	{action: actClear, keys: []string{"backspace", "ctrl+k"}, label: "⌫", short: "Clear", desc: "Clear output (like ⌘K)", group: "Output"},

	{action: actPrevWorkspace, keys: []string{"["}, label: "[", desc: "Previous workspace (like ⌘⇧[)", group: "Workspaces", workspace: true},
	{action: actNextWorkspace, keys: []string{"]"}, label: "]", short: "Workspace", desc: "Next workspace (like ⌘⇧])", group: "Workspaces", workspace: true},
	{action: actJumpWorkspace, keys: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, label: "1–9", desc: "Jump to workspace N (like ⌘1–9)", group: "Workspaces", workspace: true},

	{action: actReload, keys: []string{"r", "R"}, label: "r", desc: "Reload config (like ⌘R)", group: "App"},
	{action: actHelp, keys: []string{"?"}, label: "?", short: "Keys", desc: "Show all keys", group: "App"},
	{action: actQuit, keys: []string{"q", "ctrl+c"}, label: "q", short: "Quit", desc: "Quit (like ⌘Q)", group: "App"},
}

// keyActions maps every key string in bindings to its action.
var keyActions = func() map[string]action {
	m := make(map[string]action)
	for _, b := range bindings {
		for _, k := range b.keys {
			m[k] = b.action
		}
	}
	return m
}()

// shownBindings returns the bindings that apply to a config with the given
// number of workspaces.
func shownBindings(workspaces int) []binding {
	var out []binding
	for _, b := range bindings {
		if !b.workspace || workspaces > 1 {
			out = append(out, b)
		}
	}
	return out
}

// statusBarOrder is the order of status-bar hints: essentials first, so a
// narrow terminal cuts off the least important ones.
var statusBarOrder = []action{actRun, actSelect, actNextPane, actAI, actCopy, actClear, actHelp, actQuit, actNextWorkspace}

// statusBarHints returns the compact key list shown in the status bar.
func statusBarHints(workspaces int) string {
	shown := map[action]binding{}
	for _, b := range shownBindings(workspaces) {
		shown[b.action] = b
	}
	var parts []string
	for _, a := range statusBarOrder {
		b, ok := shown[a]
		if !ok {
			continue
		}
		label := b.label
		if a == actNextWorkspace {
			label = "[ ]"
		}
		parts = append(parts, label+" "+b.short)
	}
	return strings.Join(parts, "  ")
}

// KeyHelp returns the full key reference as plain text, grouped by purpose.
// It is printed by `mt --keys` and mirrors the in-app ? overlay.
func KeyHelp() string {
	var sb strings.Builder
	group := ""
	for _, b := range bindings {
		if b.group != group {
			if group != "" {
				sb.WriteString("\n")
			}
			group = b.group
			heading := group
			if b.workspace {
				heading += " (when more than one is configured)"
			}
			sb.WriteString(heading + "\n")
		}
		fmt.Fprintf(&sb, "  %-9s %s\n", b.label, b.desc)
	}
	sb.WriteString("\nAI prompt bar\n")
	for _, l := range promptKeys {
		fmt.Fprintf(&sb, "  %-9s %s\n", l[0], l[1])
	}
	return sb.String()
}

// helpRows is the compact in-app key reference: related keys share a row and
// the most-used rows come first, so a short pane cuts off the least useful ones.
var helpRows = []struct {
	actions []action
	text    string
}{
	{[]action{actRun}, "Run (or run selected)"},
	{[]action{actSelect}, "Select (multi-run)"},
	{[]action{actCancel}, "Stop / cancel"},
	{[]action{actAI}, "Ask AI"},
	{[]action{actCopy, actSave}, "Copy / save output"},
	{[]action{actClear}, "Clear output"},
	{[]action{actUp, actDown}, "Move / scroll"},
	{[]action{actPaneLeft, actPaneRight}, "Focus pane"},
	{[]action{actNextPane, actPrevPane}, "Cycle panes"},
	{[]action{actPrevWorkspace, actNextWorkspace}, "Prev / next workspace"},
	{[]action{actJumpWorkspace}, "Go to workspace N"},
	{[]action{actReload}, "Reload config"},
	{[]action{actQuit}, "Quit"},
	{[]action{actPageUp, actPageDown}, "Page output"},
	{[]action{actTop, actBottom}, "Output top / bottom"},
	{[]action{actHelp}, "Close this help"},
}

// compactHelp returns the helpRows lines that apply to the given number of
// workspaces, each as "labels  text".
func compactHelp(workspaces int) []string {
	shown := map[action]binding{}
	for _, b := range shownBindings(workspaces) {
		shown[b.action] = b
	}
	var lines []string
	for _, r := range helpRows {
		var labels []string
		for _, a := range r.actions {
			if b, ok := shown[a]; ok {
				labels = append(labels, b.label)
			}
		}
		if len(labels) > 0 {
			lines = append(lines, fmt.Sprintf("%-7s %s", strings.Join(labels, " "), r.text))
		}
	}
	return lines
}

// promptKeys lists the editing keys of the AI prompt bar.
var promptKeys = [][2]string{
	{"⏎", "Send"},
	{"⎋", "Close"},
	{"⌫", "Delete character"},
	{"⌥⌫ / ⌃W", "Delete word"},
	{"⌃U", "Clear line (⌘⌫ in iTerm2's Natural Text Editing preset)"},
}
