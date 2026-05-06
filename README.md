# tmux-cc-monitor

A personal CLI tool for managing multiple Claude Code sessions running in parallel inside tmux.
Claude Code hooks fire on every `UserPromptSubmit`, `Notification`, and `Stop` event and write
per-pane state files; a bubbletea TUI launched from a tmux popup reads those files, shows you
which panes need user action and which are idle, and — since v0.0.2 — lets you mirror the
chosen pane right inside the popup so you can read its current state and forward keys (arrows,
Enter, free text, Ctrl modifiers, including `q`) directly to it. Close the popup and you are
back at the pane you came from, untouched.

## Requirements

- macOS
- Go 1.26+
- tmux 3.2+ (`display-popup -E` is required)
- Claude Code with hooks support

## Build

This project uses [Task](https://taskfile.dev/) for build automation. With `task` installed:

```sh
task build         # builds ./bin/tmux-cc-monitor and ./bin/probe-hook
```

If you prefer not to install Task, the equivalent raw commands are:

```sh
go build -o ./bin/tmux-cc-monitor ./cmd/tmux-cc-monitor
go build -o ./bin/probe-hook      ./cmd/probe-hook
```

## Install

```sh
task install
```

This builds the binaries and copies them to `~/.config/tmux-cc-monitor/bin/`,
which keeps everything related to this tool (binaries, `sessions/`, `hook-errors.log`)
under a single directory tree. Add that directory to your `$PATH`:

```sh
export PATH="$HOME/.config/tmux-cc-monitor/bin:$PATH"
```

`tmux-cc-monitor` must be on your `$PATH` for both the tmux popup keybinding and
the Claude Code hook configuration to find it.

## Configuration

### 1. tmux popup keybinding

Add the following to `~/.tmux.conf`:

```tmux
bind C-g display-popup -E -w 80% -h 80% 'tmux-cc-monitor ui'
```

`-E` causes the popup to close automatically when the TUI exits. The close key (`q` / `esc`) is
handled by the bubbletea app; pressing it returns you to the calling pane.

### 2. Claude Code hook configuration

Merge the following into `~/.claude/settings.json` (or the project-local `.claude/settings.json`).
`tmux-cc-monitor` must be on your `$PATH` so the hook can find it.

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          { "type": "command", "command": "tmux-cc-monitor hook UserPromptSubmit" }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          { "type": "command", "command": "tmux-cc-monitor hook Notification" }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          { "type": "command", "command": "tmux-cc-monitor hook Stop" }
        ]
      }
    ]
  }
}
```

## Usage

The TUI is two-stage: a **list** view to pick a session, and a **mirror** view that shows the
chosen pane and forwards your keystrokes to it.

1. Open the popup from any tmux pane with `Ctrl-b C-g` (or whatever prefix you use).
2. **List view**: shows sections for *Action Waiting*, *Waiting (other)*, *Running*, and
   *Idle*, with each row showing the pane target (`session:window.pane`), project name (cwd
   basename), elapsed time since the last event, and the last hook message in one line.
   Elapsed time is rendered at minute granularity (`<1m` / `Nm` / `Nh` / `Nd`) and refreshes
   on its own 60-second tick without re-reading state files.
   - `↑` `↓` / `j` `k` to move
   - `r` to reload (re-runs cleanup and re-reads state files)
   - `Enter` to enter mirror view for the selected pane
   - `q` / `Esc` to close the popup (you return to the pane you came from)
3. **Mirror view**: renders the target pane's current screen via `tmux capture-pane` and
   forwards keystrokes back to it via `tmux send-keys`. Use this to read what the pane is
   waiting on — the permission prompt, an `AskUserQuestion` choice list, the assistant's last
   reply — and respond in place.
   - Arrow keys, `Enter`, printable text (including Japanese and `q`), `Esc`, `Tab`, `Backspace`,
     `Delete`, `PageUp`/`PageDown`, function keys `F2`–`F12`, and `Ctrl`/`Alt` modifiers all
     forward to the target pane. In particular both `q` and `Esc` are forwarded so Claude Code's
     ESC interrupt and `q` exits in `git log` / `less` / `vim` work normally
   - The mirror auto-refreshes every 500 ms and immediately after each keystroke
   - `Ctrl+G` returns to the list view — symmetric with the tmux binding that opens the popup
     (`Ctrl-b C-g` opens, `Ctrl+G` returns to list, second `q`/`Esc` from list closes)
   - `F1` is reserved for future popup help and is not forwarded
4. Closing the popup (`q`/`Esc` from the list, or `Ctrl+C` anywhere) restores you to the
   original pane with no change to its size or focus — the popup is a tmux client overlay,
   not a real pane switch.

### Permission prompts and AskUserQuestion

Both are handled in the mirror view:

- **Permission**: read the tool name and arguments shown in the prompt, then send `1` /
  `2` / `Enter` (whatever the prompt asks).
- **AskUserQuestion**: navigate the choice list with arrow keys and confirm with `Enter`.

## Architecture

- v0.0.1 (state files, hook handler, list view, send-keys): see
  [docs/design-doc/20260506_tmux_cc_monitor_design.md](docs/design-doc/20260506_tmux_cc_monitor_design.md).
- v0.0.2 popup mirror (the two-stage TUI, capture-pane forwarding, `r` reload): see
  [docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md](docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md).
- v0.0.2 refactor (status rename to *Action Waiting*, `q` forwarded in mirror, decoupled view
  redraw tick): see
  [docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md](docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md).
- All accepted ADRs are indexed in [docs/adr/adr-index.json](docs/adr/adr-index.json).

## License

MIT — see [LICENSE](LICENSE).
