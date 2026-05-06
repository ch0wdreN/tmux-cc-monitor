# tmux-cc-monitor

A personal CLI tool for managing multiple Claude Code sessions running in parallel inside tmux.
Claude Code hooks fire on every `UserPromptSubmit`, `Notification`, and `Stop` event and write
per-pane state files; a bubbletea TUI launched from a tmux popup reads those files, shows you
which panes are waiting for permission and which are idle, and lets you send text to any of them
with `tmux send-keys` — all without leaving your current pane.

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

1. Open the popup from any tmux pane with `Ctrl-b C-g` (or whatever prefix you use).
2. The TUI shows two sections: panes waiting for permission and panes that are idle/stopped.
   Each row shows the pane ID, project name (cwd basename), and the time of the last event.
3. Navigate with arrow keys, select a pane, type your message, and press Enter.
4. The text is sent to the target pane via `tmux send-keys`. The popup closes and you are
   returned to the pane you came from.
5. Press `q` or `Esc` at any time to close the popup without sending.

## Architecture

See [docs/design-doc/20260506_tmux_cc_monitor_design.md](docs/design-doc/20260506_tmux_cc_monitor_design.md)
for the full architecture, data schema, error handling strategy, and ADR links.

## License

MIT — see [LICENSE](LICENSE).
