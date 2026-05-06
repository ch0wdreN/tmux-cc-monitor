# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

`tmux-cc-monitor` is a personal CLI tool. v0.0.1 has been released and merged into `main` (PR #2): `go.mod`, `cmd/tmux-cc-monitor/main.go`, `cmd/probe-hook/main.go`, and `internal/{cleanup,errlog,hook,state,tmuxutil,ui}/` are all in place, and `Taskfile.yml` provides `task build` / `task install`.

v0.1.0 is currently in progress on a worktree, adding the popup mirror feature (real pane content rendered inside the popup TUI). New work must respect the existing v0.0.1 code and be implemented as an extension of it, not a rewrite.

## Read these before changing design or implementing

- `docs/design-doc/20260506_tmux_cc_monitor_design.md` — Design Doc for v0.0.1. Authoritative for architecture, schema, data flow, error handling, and acceptance criteria of the released baseline.
- `docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md` — Design Doc for v0.1.0's additional popup mirror feature. Authoritative for the mirror rendering pipeline, key forwarding, and the capture-pane usage boundary that extends v0.0.1.
- `docs/adr/adr-index.json` — tag-based ADR index. Look here first when wondering "why was X chosen." The two new v0.1.0 ADRs (`20260506-self-implement-popup-mirror.md`, `20260506-allow-capture-pane-for-popup-display.md`) are already registered there alongside the original three.
- `docs/adr/*.md` — accepted ADRs covering per-pane JSON state files, no background daemon in v0.0.1, hooks-driven state, and the v0.1.0 mirror decisions.

When a request conflicts with an accepted ADR, surface it explicitly rather than silently diverging. If the change is genuine, write a new ADR that Supersedes the old one rather than editing the old one in place.

## Tech stack and hard constraints

- Go 1.26.1, bubbletea (TUI)
- tmux 3.2+ (`display-popup -E` is required)
- macOS only, single-user personal install
- No background daemon in v0.0.1 (popup and hook are the only process-launch paths)
- Claude Code hooks (`UserPromptSubmit` / `Notification` / `Stop`) drive all state updates — no `tmux capture-pane` polling

## Architecture pointer

Hook writes → per-pane JSON file at `~/.config/tmux-cc-monitor/sessions/<pane_id>.json` (atomic rename) → popup TUI reads all files on launch → `tmux send-keys -t <pane_id> -l '<text>' Enter` to the chosen pane. State files include `schema_version` and `tmux_server_pid`; cleanup drops files belonging to previous tmux server generations (post-`kill-server`) and stale panes whose mtime is older than the cleanup threshold. The two paths share nothing beyond the filesystem. Schema, data flow, and error handling live in §6–§9 of the Design Doc.

## Phase 0 must come before any code

The v0.0.1 Phase 0 (hook-spec investigation: env vars, payload structure, `cwd` handover, `$TMUX_PANE` availability, distinguishing permission-Notification from other Notifications) is **complete** — findings have been written back into the v0.0.1 Design Doc and ADR 3, and the implementation merged on that basis.

The v0.1.0 Phase 0 (bubbletea `KeyMsg` → `tmux send-keys` mapping, and `tmux capture-pane` behavior for the mirror viewport) is also **complete** — its conclusions are recorded in the v0.1.0 Design Doc §6.3 / §6.4 / §13, so implementation can proceed directly from the doc.

For any future version that introduces a new external dependency or undocumented behavior, the same rule still applies: probe the real behavior first, write findings back into the corresponding Design Doc, *then* implement.

## Build / lint / test

- `task build` — compiles `tmux-cc-monitor` and `probe-hook` into `./bin/`.
- `task install` — installs the built binaries to `~/.config/tmux-cc-monitor/bin/` (the personal install location).
- `go test ./...` — runs the full test suite.
- `task check-state-purity` — CI-style guard added in v0.1.0 that asserts `tmux capture-pane` does not appear on any state-decision code path (the mirror feature must keep capture-pane confined to popup display).
- `task verify` — convenience target that runs build + test + `check-state-purity` together; treat this as the green-bar before merging v0.1.0 work.
