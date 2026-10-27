# mt

A keyboard-driven terminal workspace launcher. Navigate domains and targets in a 3-pane TUI, press Enter to run commands, and see output inline — no mouse required.

## Install

Requires Go 1.25 or later.

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

Keys follow Mac conventions. A terminal app never receives ⌘, so each ⌘ shortcut becomes the same letter on its own: ⌘C is `c`, ⌘S is `s`, ⌘R is `r`, ⌘Q is `q`. Press `?` in mt for a compact reference, or run `mt --keys` to print the full list.

| Key | Action | Mac mnemonic |
|-----|--------|--------------|
| `⏎` Return | Run the selected target (or every selected target, in parallel) | Open |
| `␣` Space | Select / deselect a target for a parallel run (middle pane) | Finder selection |
| `⎋` Esc | Stop the running command(s), cancel an AI request, or close the prompt bar | Cancel |
| `/` | Ask the AI assistant | |
| `c` | Copy output to the clipboard | ⌘C |
| `s` | Save output to `~/.mt/logs/mt-TIMESTAMP.txt` | ⌘S |
| `⌫` Delete | Clear output (also `⌃K`) | ⌘K in Terminal |
| `↑` `↓` | Move the cursor; scroll output when the right pane has focus | |
| `←` `→` | Focus the pane to the left / right | Finder columns |
| `⇥` / `⇧⇥` | Cycle pane focus (wraps around) | Tab between controls |
| `fn↑` `fn↓` | Page output up / down (Page Up / Page Down) | |
| `fn←` `fn→` | Jump to top / bottom of output (Home / End) | |
| `[` / `]` | Previous / next workspace (2+ workspaces) | ⌘⇧[ / ⌘⇧] switch tabs |
| `1`–`9` | Jump to workspace N (2+ workspaces) | ⌘1–⌘9 |
| `r` | Reload the config file | ⌘R |
| `?` | Show / hide the key reference | ⌘? Help |
| `q` / `⌃C` | Quit (cancels any running commands) | ⌘Q |

Older keys still work: `y` (copy), `S` (save), `R` (reload) and `h` `j` `k` `l` (vim-style movement). Two keys changed meaning: `c` now copies (it used to clear; use `⌫`), and `Tab` now cycles panes (workspaces moved to `[` / `]`).

**Trackpad and mouse:**

| Gesture | Action |
|---|---|
| Click | Select a domain or target and focus its pane |
| Double-click a target | Run it, like opening a file in Finder |
| Two-finger scroll over the output | Scroll the output |

While mt is running, the terminal passes clicks to mt, so click-and-drag no longer selects text in the window. To select text anyway: in **iTerm2**, hold `⌥` while dragging. In **Terminal.app**, turn off *View → Allow Mouse Reporting* while you select. Or just press `c` to copy the whole output. To turn trackpad support off, start mt with `mt --no-mouse`.

In the AI prompt bar: `⌫` deletes a character, `⌥⌫` or `⌃W` deletes a word, and `⌃U` clears the line.

**Terminal setup tips (macOS):**
- **Terminal.app:** for `⌥⌫` to delete a word, tick *Settings → Profiles → Keyboard → Use Option as Meta key*.
- **iTerm2:** choose *Settings → Profiles → Keys → Key Mappings → Presets → Natural Text Editing*. Then `⌥⌫` deletes a word and `⌘⌫` clears the prompt line.
- **`fn↑` / `fn↓`:** if the terminal scrolls its own window instead of mt's output, hold `⇧` as well, or use `↑` / `↓` with the right pane focused.

## AI Assistant

Press `/` to open a natural-language prompt bar. Describe what you want in plain English and press `Enter`. The assistant maps your request to one of three actions:

- **Run a target**: selects and runs an existing target by name (sequences included)
- **Generate a command**: proposes a new command; nothing runs until you press `Enter` to confirm (`Esc` rejects it)
- **Answer a question**: replies inline in 2 sentences or fewer

The model sees the names and first status line of every target in the current workspace, plus the current time. Requests time out after 30 seconds; press `Esc` to cancel sooner.

| `provider:` | Default endpoint | Notes |
|---|---|---|
| `litellm` (default) | `http://localhost:4000` | LiteLLM proxy in front of any backend |
| `llamacpp` | `http://localhost:8080` | `llama-server`; also any other OpenAI-compatible server via `base_url` (e.g. Ollama at `http://localhost:11434`, with `model:` set) |
| `mlx` | `http://localhost:8000` | `mlx_lm.server` on Apple Silicon |
| `openai` | `https://api.openai.com` | Needs `api_key` or `OPENAI_API_KEY`; model defaults to `gpt-4o-mini` |
| `apple` | (local binary) | On-device Foundation Models via `mt-apple-bridge`, macOS 26+ |

`base_url` must not include `/v1` (mt appends `/v1/chat/completions`). Set `debug: true` under `llm:` to log every request and response to `~/.mt/logs/debug.log`.

### On-device: Apple Foundation Models (macOS 26+)

Zero network dependency. Requires Apple Intelligence enabled on your Mac.

```sh
make apple-bridge                      # compile mt-apple-bridge (run once)
cp mt-apple-bridge "$(dirname $(which mt))/"   # place it alongside the mt binary
```

```yaml
llm:
  provider: apple
```

### Local: llama.cpp with Metal (Apple Silicon)

Full GPU acceleration via Metal. Works with any GGUF model.

```sh
brew install llama.cpp
llama-server -m ~/models/mistral-7b-instruct.Q4_K_M.gguf \
             --n-gpu-layers 99 --port 8080
```

```yaml
llm:
  provider: llamacpp
  # base_url: http://localhost:8080   # default
```

### Cloud: OpenAI

```yaml
llm:
  provider: openai
  model: gpt-4o-mini
  api_key: sk-...   # or set OPENAI_API_KEY env var
```

---

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
        run: npm run dev

      - name: "Run Tests"
        status: "Press [Enter] to run tests"
        run: npm test

      # Sequence: runs Build → Test → Deploy in order; stops on first failure
      - name: "Full Pipeline"
        sequence: ["Build", "Test", "Deploy"]

      # SSH target: runs the command on a remote host
      - name: "Deploy"
        host: "deploy@prod.example.com"
        run: ./deploy.sh --env production
```

**Writing commands:** `run:` takes the command as you would type it. Quotes group words, so `run: open -a "Microsoft Word"` passes `Microsoft Word` as one argument. There is no shell behind it, so pipes, `&&`, redirects, `$VARS` and `~` are rejected when the config loads, with a message naming the target. When you need them, wrap the command in a shell explicitly:

```yaml
      - name: "Count Go files"
        cmd: ["sh", "-c", "ls *.go | wc -l"]
```

On SSH targets (`host:` set), the `run:` string is sent to the remote shell exactly as written, so `run: journalctl -u myapp | grep -i error` works there.

`cmd:` is the same thing as an explicit list (`cmd: ["git", "status"]`), handy when an argument is awkward to quote, such as a multi-line AppleScript. A target uses one or the other.

**Include directive:** Split configs across files with `include:`:

```yaml
include:
  - ~/shared/team-tools.yaml  # ~/ expands to $HOME; must resolve within $HOME

domains:
  - name: "Personal"
    targets:
      - name: "Check Mail"
        run: open -a Mail
```

Relative include paths resolve from the including file's directory. A missing or invalid include is reported as a config error (mt then falls back to the built-in defaults) rather than silently skipped.

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

**Multiple workspaces:** Switch contexts live with `[` / `]`, or jump straight to one with `1`–`9`:

```yaml
workspaces:
  - name: "Microsoft"
    domains:
      - name: "Outlook"
        targets:
          - name: "New Email"
            run: open ms-outlook://compose
            launch_msg: "Opening new email…"
  - name: "Dev"
    domains:
      - name: "Context/Git"
        targets:
          - name: "Git Status"
            run: git status
```

See [`mt.yaml.example`](mt.yaml.example) for a full multi-workspace template with per-app actions for Outlook, Word, Excel, Edge, VS Code, and more.

For the full field reference, include directive rules, sequence/parallel/SSH examples, and security model, see [`docs/manual.html`](docs/manual.html). For a detailed breakdown of the GitHub Actions CI workflow, see [`docs/ci.html`](docs/ci.html).

## macOS Deployment

### Install

```sh
make install                 # builds and copies mt → /usr/local/bin/mt
make install-apple-bridge    # builds and copies mt-apple-bridge → /usr/local/bin/mt-apple-bridge
```

After installing the Apple bridge, clear Gatekeeper's quarantine attribute (ad-hoc signing satisfies the FoundationModels entitlement but not Gatekeeper's network-download check):

```sh
xattr -d com.apple.quarantine /usr/local/bin/mt-apple-bridge
```

### Config file permissions

```sh
# macOS
chmod 600 ~/Library/Application\ Support/mt/config.yaml
```

`mt` rejects world-writable configs but doesn't enforce 0600 — set it manually so other local users can't read your targets or API keys.

### API keys

Prefer the environment variable over storing the key in YAML:

```sh
export OPENAI_API_KEY=sk-...   # add to ~/.zshrc or ~/.bash_profile
```

If you do store a key in the config file, the `chmod 600` above protects it from other local users.

### Apple Silicon / Metal GPU (M-series)

**Apple Foundation Models** (on-device, zero network, macOS 26+): see [On-device: Apple Foundation Models](#on-device-apple-foundation-models-macos-26) above.

**llama.cpp with Metal** — optimal settings for M5 Max (36 GB unified memory):

```sh
brew install llama.cpp
llama-server -m ~/models/mistral-7b-instruct.Q4_K_M.gguf \
             --n-gpu-layers 99 \
             --ctx-size 8192 \
             --threads 10 \
             --port 8080
```

| Flag | Why |
|------|-----|
| `--n-gpu-layers 99` | Offloads all transformer layers to Metal; maximises GPU utilisation |
| `--ctx-size 8192` | 36 GB of unified memory fits large contexts; raise to `16384` for large models |
| `--threads 10` | Matches M5 Max's 10 performance cores |

Model size guide for 36 GB unified memory: by default macOS lets the GPU use roughly 70–75% of RAM (about 26 GB here). Q4_K_M weighs about 0.6 GB per billion parameters, so ~30B-class models fit with room for context; 70B models do not. Q8_0 weighs about 1.1 GB per billion parameters, so stay at ~20B or below.

## Security

Your config is the trust boundary: anyone who can write it can run commands as you. `mt` runs local commands exactly as written, as argument arrays with no shell expansion. SSH targets are the exception, because `ssh` hands the command to the remote shell. Protections:

- Config files (and includes) with world-writable permissions are rejected
- `include:` paths must resolve within `$HOME` (symlinks checked)
- SSH `host:` values are validated against `[a-zA-Z0-9][a-zA-Z0-9._@%-]*` to block option injection
- AI-generated commands never run without your explicit `Enter`
- Command output is capped at 1 MB as it is read; per-target cache is bounded to 256 KB

A `./mt.yaml` in the current directory overrides your user config, so only run `mt` in directories you trust.

## Development

```sh
make test    # run tests
make vet     # go vet
make build   # compile
make clean   # remove binary
```

CI runs on every push and pull request via GitHub Actions (`.github/workflows/ci.yml`): vet + test on Linux, a Linux build, and a macOS job that also builds the Apple bridge. See [`docs/ci.html`](docs/ci.html).

### Project layout

| Path | Purpose |
|---|---|
| `main.go` | Entry point: starts the Bubble Tea program |
| `internal/tui/model.go` | `Model` state, message types, per-target output cache helpers |
| `internal/tui/update.go` | Key handling, command execution, sequences, parallel runs, live-status polling, AI prompt handling |
| `internal/tui/view.go`, `styles.go` | Three-pane rendering and colours |
| `internal/tui/config.go` | Config discovery, YAML parsing, `include:` resolution, `apps:` expansion |
| `internal/tui/data.go` | `Workspace`/`Domain`/`Target` types and the built-in default targets |
| `internal/llm/` | Provider dispatch (`llm.go`), OpenAI-compatible HTTP client (`openai.go`), Apple bridge client (`apple.go`) |
| `cmd/apple-llm/` | Swift source and entitlements for `mt-apple-bridge` |
