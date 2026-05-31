package tui

// Workspace groups a named set of domains for multi-project switching.
type Workspace struct {
	Name    string   `yaml:"name"`
	Domains []Domain `yaml:"domains"`
}

// Domain groups a named category with its list of actionable targets.
type Domain struct {
	Name    string   `yaml:"name"`
	Targets []Target `yaml:"targets"`
}

// Target is a single actionable item within a domain.
type Target struct {
	Name      string   `yaml:"name"`
	Status    string   `yaml:"status"`     // static hint shown before any execution
	Cmd       []string `yaml:"cmd"`        // shell command to run on Enter; nil = not configured
	LaunchMsg string   `yaml:"launch_msg"` // shown when Cmd succeeds but produces no output (e.g. open -a)
	Sequence  []string `yaml:"sequence"`   // ordered list of target names to run in series
	Host      string   `yaml:"host"`       // SSH host; if set, cmd is run via ssh host <cmd>
}

// Outlook AppleScript templates — osascript is passed via -e so the entire script
// is a single string argument, avoiding temp-file creation.
const outlookMeeting30 = `tell application "Microsoft Outlook"
set s to (current date) + (1 * hours)
set newEvent to make new calendar event
set subject of newEvent to "30-Min Meeting"
set start time of newEvent to s
set end time of newEvent to s + (30 * minutes)
open newEvent
activate
end tell`

const outlookMeeting60 = `tell application "Microsoft Outlook"
set s to (current date) + (1 * hours)
set newEvent to make new calendar event
set subject of newEvent to "1-Hour Meeting"
set start time of newEvent to s
set end time of newEvent to s + (60 * minutes)
open newEvent
activate
end tell`

const outlookTeamsMeeting = `tell application "Microsoft Outlook"
set s to (current date) + (1 * hours)
set newEvent to make new calendar event
set subject of newEvent to "Teams Meeting"
set start time of newEvent to s
set end time of newEvent to s + (30 * minutes)
set location of newEvent to "Microsoft Teams"
open newEvent
activate
end tell`

const wordNewDoc = `tell application "Microsoft Word"
activate
make new document
end tell`

const excelNewWorkbook = `tell application "Microsoft Excel"
activate
make new workbook
end tell`

var initialDomains = []Domain{
	{
		Name: "Outlook",
		Targets: []Target{
			{
				Name:      "Open Outlook",
				Status:    "Press [Enter] to launch Outlook",
				Cmd:       []string{"open", "-a", "Microsoft Outlook"},
				LaunchMsg: "Opening Outlook…",
			},
			{
				Name:      "New Email",
				Status:    "Opens a new email compose window",
				Cmd:       []string{"open", "ms-outlook://compose"},
				LaunchMsg: "Opening new email…",
			},
			{
				Name:      "Open Inbox",
				Status:    "Opens the Outlook inbox",
				Cmd:       []string{"open", "ms-outlook://mail"},
				LaunchMsg: "Opening inbox…",
			},
			{
				Name:      "Open Calendar",
				Status:    "Opens the Outlook calendar view",
				Cmd:       []string{"open", "ms-outlook://calendar"},
				LaunchMsg: "Opening calendar…",
			},
			{
				Name:      "Open Tasks",
				Status:    "Opens the Outlook tasks view",
				Cmd:       []string{"open", "ms-outlook://tasks"},
				LaunchMsg: "Opening tasks…",
			},
			{
				Name:      "New 30-Min Meeting",
				Status:    "Creates a new 30-minute meeting starting 1 hour from now",
				Cmd:       []string{"osascript", "-e", outlookMeeting30},
				LaunchMsg: "Opening 30-min meeting in Outlook…",
			},
			{
				Name:      "New 1-Hr Meeting",
				Status:    "Creates a new 1-hour meeting starting 1 hour from now",
				Cmd:       []string{"osascript", "-e", outlookMeeting60},
				LaunchMsg: "Opening 1-hour meeting in Outlook…",
			},
			{
				Name:      "New Teams Meeting",
				Status:    "Creates a new Teams meeting starting 1 hour from now",
				Cmd:       []string{"osascript", "-e", outlookTeamsMeeting},
				LaunchMsg: "Opening Teams meeting in Outlook…",
			},
			{
				Name:   "Schedule with AI",
				Status: "Press [/] and type your request, e.g.:\n\"schedule 30-min meeting with john@co.com tomorrow at 2pm\"",
			},
		},
	},
	{
		Name: "Word",
		Targets: []Target{
			{
				Name:      "Open Word",
				Status:    "Press [Enter] to launch Microsoft Word",
				Cmd:       []string{"open", "-a", "Microsoft Word"},
				LaunchMsg: "Opening Word…",
			},
			{
				Name:      "New Document",
				Status:    "Creates a new blank Word document",
				Cmd:       []string{"osascript", "-e", wordNewDoc},
				LaunchMsg: "Creating new Word document…",
			},
			{
				Name:      "Word Online",
				Status:    "Opens Word Online in Microsoft Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "https://office.live.com/start/Word.aspx"},
				LaunchMsg: "Opening Word Online…",
			},
			{
				Name:   "Copilot in Word",
				Status: "Use AI to draft, rewrite, or summarise\n\nOpen a document in Word, then press [/] below and describe what you need",
			},
		},
	},
	{
		Name: "Excel",
		Targets: []Target{
			{
				Name:      "Open Excel",
				Status:    "Press [Enter] to launch Microsoft Excel",
				Cmd:       []string{"open", "-a", "Microsoft Excel"},
				LaunchMsg: "Opening Excel…",
			},
			{
				Name:      "New Workbook",
				Status:    "Creates a new blank Excel workbook",
				Cmd:       []string{"osascript", "-e", excelNewWorkbook},
				LaunchMsg: "Creating new Excel workbook…",
			},
			{
				Name:      "Excel Online",
				Status:    "Opens Excel Online in Microsoft Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "https://office.live.com/start/Excel.aspx"},
				LaunchMsg: "Opening Excel Online…",
			},
			{
				Name:   "Copilot in Excel",
				Status: "Use AI to analyse data, write formulas, or generate charts\n\nOpen a workbook in Excel, then press [/] below and describe what you need",
			},
		},
	},
	{
		Name: "Microsoft Edge",
		Targets: []Target{
			{
				Name:      "Open Edge",
				Status:    "Press [Enter] to launch Microsoft Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge"},
				LaunchMsg: "Opening Edge…",
			},
			{
				Name:      "New Window",
				Status:    "Opens a new Edge browser window",
				Cmd:       []string{"open", "-na", "Microsoft Edge"},
				LaunchMsg: "Opening new Edge window…",
			},
			{
				Name:      "InPrivate Window",
				Status:    "Opens a new private browsing window in Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "--args", "--inprivate"},
				LaunchMsg: "Opening InPrivate window…",
			},
			{
				Name:      "Copilot Chat",
				Status:    "Opens Microsoft Copilot chat in Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "https://copilot.microsoft.com"},
				LaunchMsg: "Opening Copilot…",
			},
			{
				Name:      "Microsoft 365",
				Status:    "Opens the Microsoft 365 home page",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "https://www.microsoft365.com"},
				LaunchMsg: "Opening Microsoft 365…",
			},
			{
				Name:      "Outlook Web",
				Status:    "Opens Outlook Web Access in Edge",
				Cmd:       []string{"open", "-a", "Microsoft Edge", "https://outlook.office.com"},
				LaunchMsg: "Opening Outlook Web…",
			},
		},
	},
	{
		Name: "VS Code",
		Targets: []Target{
			{
				Name:      "Open VS Code",
				Status:    "Press [Enter] to launch Visual Studio Code",
				Cmd:       []string{"open", "-a", "Visual Studio Code"},
				LaunchMsg: "Opening VS Code…",
			},
			{
				Name:      "Open Current Dir",
				Status:    "Opens the current working directory in VS Code",
				Cmd:       []string{"code", "."},
				LaunchMsg: "Opening folder in VS Code…",
			},
			{
				Name:      "New Window",
				Status:    "Opens a new empty VS Code window",
				Cmd:       []string{"code", "-n"},
				LaunchMsg: "Opening new VS Code window…",
			},
			{
				Name:      "Open Settings",
				Status:    "Opens VS Code settings UI",
				Cmd:       []string{"code", "--command", "workbench.action.openSettings2"},
				LaunchMsg: "Opening VS Code settings…",
			},
			{
				Name:      "Open Extensions",
				Status:    "Opens the Extensions panel in VS Code",
				Cmd:       []string{"code", "--command", "workbench.view.extensions"},
				LaunchMsg: "Opening Extensions panel…",
			},
		},
	},
	{
		Name: "Terminal",
		Targets: []Target{
			{
				Name:      "New Terminal Window",
				Status:    "Opens a new macOS Terminal window",
				Cmd:       []string{"open", "-a", "Terminal"},
				LaunchMsg: "Opening Terminal…",
			},
			{
				Name:      "New iTerm2 Window",
				Status:    "Opens a new iTerm2 window",
				Cmd:       []string{"open", "-a", "iTerm"},
				LaunchMsg: "Opening iTerm2…",
			},
			{
				Name:      "Open Windows App",
				Status:    "Opens Microsoft Windows App (Remote Desktop)\nDouble-click a saved connection to connect",
				Cmd:       []string{"open", "-a", "Windows App"},
				LaunchMsg: "Opening Windows App…",
			},
		},
	},
	{
		Name: "Infrastructure",
		Targets: []Target{
			{Name: "Docker Up",   Status: "Docker: Stopped\n\nPress [Enter] to start containers", Cmd: []string{"docker", "compose", "up", "-d"}},
			{Name: "Docker Down", Status: "Press [Enter] to stop all containers",                 Cmd: []string{"docker", "compose", "down"}},
			{Name: "Postgres",    Status: "Container: postgres\nPress [Enter] to check status",   Cmd: []string{"docker", "ps", "--filter", "name=postgres", "--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}"}},
			{Name: "Redis",       Status: "Container: redis\nPress [Enter] to check status",      Cmd: []string{"docker", "ps", "--filter", "name=redis",    "--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}"}},
		},
	},
	{
		Name: "Context/Git",
		Targets: []Target{
			{Name: "Git Status", Status: "Press [Enter] to run git status",  Cmd: []string{"git", "status"}},
			{Name: "Git Diff",   Status: "Press [Enter] to view diff",       Cmd: []string{"git", "diff"}},
			{Name: "Branches",   Status: "Press [Enter] to list branches",   Cmd: []string{"git", "branch", "-a"}},
			{Name: "Stash",      Status: "Press [Enter] to list stash",      Cmd: []string{"git", "stash", "list"}},
		},
	},
}

var defaultWorkspaces = []Workspace{
	{Domains: initialDomains},
}
