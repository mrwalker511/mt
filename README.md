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
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Focus left pane |
| `→` / `l` | Focus right pane |
| `Enter` | Execute selected target |
| `q` / `Ctrl+C` | Quit |

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
```

See [`mt.yaml.example`](mt.yaml.example) for the full set of options and a commented template of the default targets.

## Development

```sh
make test    # run tests
make vet     # go vet
make build   # compile
make clean   # remove binary
```

CI runs on every push via GitHub Actions (`.github/workflows/ci.yml`).
