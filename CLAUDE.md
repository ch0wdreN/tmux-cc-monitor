# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

`tmux-cc-monitor` is a personal CLI tool. No source code is committed yet — only `README.md`, `LICENSE`, and design documents under `docs/`. The first release target (v0.0.1) is fully designed but not implemented.

## Read these before changing design or implementing

- `docs/design-doc/20260506_tmux_cc_monitor_design.md` — Design Doc for v0.0.1. Authoritative for architecture, schema, data flow, error handling, and acceptance criteria.
- `docs/adr/adr-index.json` — tag-based ADR index. Look here first when wondering "why was X chosen."
- `docs/adr/*.md` — three accepted ADRs (per-pane JSON state files / no background daemon in v0.0.1 / hooks-driven state).

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

The Implementation Plan opens with a hook-spec investigation, not coding. Several spots in the Design Doc and ADR 3 are marked `⚠️ TBD` because they depend on what Claude Code actually passes to hooks (env vars, payload structure, `cwd` handover, `$TMUX_PANE` availability, how to distinguish permission-Notification from other Notifications). Probe a real hook invocation, write findings back into those documents, *then* implement.

## Build / lint / test

No `go.mod`, no source files, no commands yet. Add them here when implementation begins — do not invent them in advance.
