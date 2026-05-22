package tui

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
}

var initialDomains = []Domain{
	{
		Name: "App Launch",
		Targets: []Target{
			{Name: "Microsoft Word",  Status: "App Status: Closed\n\nPress [Enter] to launch", Cmd: []string{"open", "-a", "Microsoft Word"},  LaunchMsg: "Launching Microsoft Word…"},
			{Name: "Microsoft Excel", Status: "App Status: Closed\n\nPress [Enter] to launch", Cmd: []string{"open", "-a", "Microsoft Excel"}, LaunchMsg: "Launching Microsoft Excel…"},
			{Name: "Notes",           Status: "Press [Enter] to open",                         Cmd: []string{"open", "-a", "Notes"},           LaunchMsg: "Opening Notes…"},
			{Name: "Safari",          Status: "Press [Enter] to open",                         Cmd: []string{"open", "-a", "Safari"},          LaunchMsg: "Opening Safari…"},
			{Name: "Terminal",        Status: "Press [Enter] to open new terminal",            Cmd: []string{"open", "-a", "Terminal"},        LaunchMsg: "Opening Terminal…"},
		},
	},
	{
		Name: "Dev Tools",
		Targets: []Target{
			{Name: "VS Code",   Status: "Press [Enter] to open editor",   Cmd: []string{"open", "-a", "Visual Studio Code"}, LaunchMsg: "Opening VS Code…"},
			{Name: "iTerm2",    Status: "Press [Enter] to open terminal",  Cmd: []string{"open", "-a", "iTerm"},             LaunchMsg: "Opening iTerm2…"},
			{Name: "Postman",   Status: "Press [Enter] to open Postman",   Cmd: []string{"open", "-a", "Postman"},           LaunchMsg: "Opening Postman…"},
			{Name: "TablePlus", Status: "Press [Enter] to open TablePlus", Cmd: []string{"open", "-a", "TablePlus"},         LaunchMsg: "Opening TablePlus…"},
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
		Name: "RunLayer",
		Targets: []Target{
			{Name: "Deploy",   Status: "No command configured.\n\nSee mt.yaml.example to add a deploy cmd."},
			{Name: "Status",   Status: "No command configured.\n\nSee mt.yaml.example to add a status cmd."},
			{Name: "Logs",     Status: "No command configured.\n\nSee mt.yaml.example to add a logs cmd."},
			{Name: "Rollback", Status: "No command configured.\n\nSee mt.yaml.example to add a rollback cmd."},
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
