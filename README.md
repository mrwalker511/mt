# mt

A keyboard-driven terminal workspace launcher. Navigate domains and targets in a 3-pane TUI, press Enter to run commands, and see output inline — no mouse required.

## Install

```sh
go install github.com/mrwalker511/mt@latest
```

Or build from source:

```sh
git clone https://github.com/mrwalker511/mt
cd mt
make build   # produces ./mt
```

## Usage

```sh
mt
```

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up / scroll output up (right pane) |
| `↓` / `j` | Move down / scroll output down (right pane) |
| `←` / `h` | Focus left pane |
| `→` / `l` | Focus right pane |
| `Enter` | Execute selected target (or all multi-selected targets in parallel) |
| `Space` | Toggle multi-select on current target (middle pane) |
| `y` | Copy right-pane output to clipboard |
| `S` | Save right-pane output to `~/.mt/logs/mt-TIMESTAMP.txt` |
| `c` | Clear right-pane output |
| `R` | Reload config file without restarting |
| `?` | Toggle help overlay |
| `Tab` / `Shift+Tab` | Switch workspace (when multiple workspaces configured) |
| `q` / `Ctrl+C` | Quit (cancels any running commands) |

## Configuration

`mt` looks for a config file in order:

1. System config dir — platform-dependent:
   - **macOS:** `~/Library/Application Support/mt/config.yaml`
   - **Linux:** `~/.config/mt/config.yaml`
2. `./mt.yaml` (local override in current directory)

If neither exists, built-in defaults are used (macOS app launchers, Docker, and Git targets).

To get started with your own config:

```sh
# macOS
cp mt.yaml.example ~/Library/Application\ Support/mt/config.yaml

# Linux
cp mt.yaml.example ~/.config/mt/config.yaml
```

Then edit the file. The format is straightforward:

```yaml
domains:
  - name: "My Project"
    targets:
      - name: "Start Server"
        status: "Press [Enter] to start"
        cmd: ["npm", "run", "dev"]

      - name: "Run Tests"
        status: "Press [Enter] to run tests"
        cmd: ["npm", "test"]

      # Sequence: runs Build → Test → Deploy in order; stops on first failure
      - name: "Full Pipeline"
        sequence: ["Build", "Test", "Deploy"]

      # SSH target: runs the command on a remote host
      - name: "Deploy"
        host: "deploy@prod.example.com"
        cmd: ["./deploy.sh", "--env", "production"]
```

**Include directive:** Split configs across files with `include:`:

```yaml
include:
  - ~/shared/team-tools.yaml  # must resolve within $HOME

domains:
  - name: "Personal"
    targets:
      - name: "Check Mail"
        cmd: ["open", "-a", "Mail"]
```

**Mac app shorthand:** Use `apps:` to add launchers without writing the full command:

```yaml
domains:
  - name: "Apps"
    apps:
      - "Microsoft Outlook"
      - "Microsoft Edge"
      - "Visual Studio Code"
      - "Slack"
```

Each entry auto-expands to `open -a <name>` with a launch message.

**Multiple workspaces:** Switch contexts live with `Tab`/`Shift+Tab`:

```yaml
workspaces:
  - name: "Microsoft"
    domains:
      - name: "Outlook"
        targets:
          - name: "New Email"
            cmd: ["open", "ms-outlook://compose"]
            launch_msg: "Opening new email…"
  - name: "Dev"
    domains:
      - name: "Context/Git"
        targets:
          - name: "Git Status"
            cmd: ["git", "status"]
```

See [`mt.yaml.example`](mt.yaml.example) for a full multi-workspace template with per-app actions for Outlook, Word, Excel, Edge, VS Code, and more.

For the full field reference, include directive rules, sequence/parallel/SSH examples, and security model, see [`docs/manual.html`](docs/manual.html).

## Security

`mt` runs commands exactly as written in your config — no shell expansion. Protections applied at config load time:

- Config files with world-writable permissions are rejected
- `include:` paths must resolve within `$HOME` (symlinks checked)
- SSH `host:` values are validated against `[a-zA-Z0-9][a-zA-Z0-9._@%-]*` to block option injection
- Command output is bounded to 1 MB; per-target cache is bounded to 256 KB

## Development

```sh
make test    # run tests
make vet     # go vet
make build   # compile
make clean   # remove binary
```

CI runs on every push via GitHub Actions (`.github/workflows/ci.yml`).
