package tui

// Domain groups a named category with its list of actionable targets.
type Domain struct {
	Name    string
	Targets []Target
}

// Target is a single actionable item within a domain.
type Target struct {
	Name   string
	Status string // displayed in the right info pane
}

var initialDomains = []Domain{
	{
		Name: "App Launch",
		Targets: []Target{
			{Name: "Microsoft Word", Status: "App Status: Closed\n\nPress [Enter] to launch\nPress [N] for new doc"},
			{Name: "Microsoft Excel", Status: "App Status: Closed\n\nPress [Enter] to launch"},
			{Name: "Notes", Status: "App Status: Running\n\nPress [Enter] to focus"},
			{Name: "Safari", Status: "Press [Enter] to open"},
			{Name: "Terminal", Status: "Press [Enter] to open new terminal"},
		},
	},
	{
		Name: "Dev Tools",
		Targets: []Target{
			{Name: "VS Code", Status: "Press [Enter] to open editor"},
			{Name: "iTerm2", Status: "Press [Enter] to open terminal"},
			{Name: "Postman", Status: "Press [Enter] to open Postman"},
			{Name: "TablePlus", Status: "Press [Enter] to open TablePlus"},
		},
	},
	{
		Name: "Infrastructure",
		Targets: []Target{
			{Name: "Docker Up", Status: "Docker: Stopped\n\nPress [Enter] to start containers"},
			{Name: "Docker Down", Status: "Press [Enter] to stop all containers"},
			{Name: "Postgres", Status: "Container: postgres-pgvector\nStatus: unknown"},
			{Name: "Redis", Status: "Container: redis\nStatus: unknown"},
		},
	},
	{
		Name: "RunLayer",
		Targets: []Target{
			{Name: "Deploy", Status: "Environment: staging\n\nPress [Enter] to deploy"},
			{Name: "Status", Status: "Last deploy: unknown\n\nPress [Enter] to check"},
			{Name: "Logs", Status: "Press [Enter] to stream logs"},
			{Name: "Rollback", Status: "Press [Enter] to rollback last deploy"},
		},
	},
	{
		Name: "Context/Git",
		Targets: []Target{
			{Name: "Git Status", Status: "Branch: unknown\n\nPress [Enter] to run git status"},
			{Name: "Git Diff", Status: "Press [Enter] to view staged diff"},
			{Name: "Branches", Status: "Press [Enter] to list branches"},
			{Name: "Stash", Status: "Press [Enter] to manage stash"},
		},
	},
}
