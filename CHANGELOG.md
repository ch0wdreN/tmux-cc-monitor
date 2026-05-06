# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.0.2] - 2026-05-07

### Changed

- **UI**: `Permission Waiting` セクションを `Action Waiting` に改名し、色を bright red から bright cyan に変更。バッジも `[PERM]` から `[ACTION]` へ。「ユーザー対応が必要な pane」をハイライトする役割は維持しつつ、赤の主張を抑える。
- **State**: `Status` 定数を `StatusWaitingPermission` から `StatusWaitingAction` にリネーム。state JSON 値も `"waiting_permission"` → `"waiting_action"` に変更（pre-1.0 の特例として `schema_version` は据え置き）。
- **Hook**: Notification subtype の分類を見直し:
  - `permission_prompt`, `elicitation_dialog` → `waiting_action`
  - `idle_prompt`, `auth_success` → `idle`
  - `elicitation_response`, `elicitation_complete` → `running`
  - 未知の subtype → `waiting_other`（fallback、Claude Code 仕様変更検出器）
- **Mirror mode**: `q` キーを target pane へ forward するように変更。これまで `q` は popup を閉じる予約キーだったため `git log` / `less` / `vim` 等の対話操作が崩れていた。mirror mode から list view への復帰は **`Esc` のみ**。
- **UI tick**: 経過時間表示を 60 秒間隔の独立 redraw tick で更新。秒粒度を廃止し `<1m / Nm / Nh / Nd` に粗化。state reload (`r` キー) とは別経路で I/O を伴わない軽量更新。

### Added

- **State sweep**: `state.ReadAll` 起動時に旧 `"waiting_permission"` 値の state ファイルを検出した場合、警告を出して skip する移行ロジック（次の hook 発火で新値に置き換わる）。
- **Tests**: `internal/state/state_test.go` に旧値 sweep テスト、`internal/ui/mirror_test.go` を新規追加して keymap・footer 検証。

### Documentation

- v0.0.2 リファクタ Design Doc を `docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md` に追加。
- ADR 2 件を新規記録:
  - `docs/adr/20260507-forward-q-in-mirror-mode.md`
  - `docs/adr/20260507-decouple-view-redraw-from-state-reload.md`

## [v0.0.1] - 2026-05-06

Initial release: per-pane JSON state files driven by Claude Code hooks (`UserPromptSubmit` / `Notification` / `Stop`), bubbletea TUI launched from a tmux popup, list view of waiting / running / idle panes, free-text + Enter send to selected pane.
