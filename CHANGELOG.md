# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.0.2] - 2026-05-07

### Changed

- **UI**: `Permission Waiting` セクションを `Action Waiting` に改名し、色を bright red から bright cyan に変更。「ユーザー対応が必要な pane」をハイライトする役割は維持しつつ、赤の主張を抑える。
- **UI (Running ハイライト)**: `Running` セクション見出しを bright green (color 10) + bold でハイライト。`Action Waiting` (cyan) と並んで「注目すべき状態」を見出しの色で示し、現在動いている pane も一目で識別できる。
- **UI (バッジ削除)**: 各行のステータスバッジ (`[PERM]` / `[ACTION]` / `[WAIT]` / `[RUN]` / `[IDLE]`) を完全削除。状態識別はセクション見出しの色で行うようになったため重複情報を整理し、message 表示領域を拡張。
- **State**: `Status` 定数を `StatusWaitingPermission` から `StatusWaitingAction` にリネーム。state JSON 値も `"waiting_permission"` → `"waiting_action"` に変更（pre-1.0 の特例として `schema_version` は据え置き）。
- **Hook**: Notification subtype の分類を見直し:
  - `permission_prompt`, `elicitation_dialog` → `waiting_action`
  - `idle_prompt`, `auth_success` → `idle`
  - `elicitation_response`, `elicitation_complete` → `running`
  - 未知の subtype → `waiting_other`（fallback、Claude Code 仕様変更検出器）
- **Mirror mode**: `q` と `Esc` の両方を target pane へ forward するように変更。これまで `q` は popup を閉じる予約キーで、`Esc` も popup 側が吸う仕様だったため、`git log` / `less` / `vim` 等の `q` 操作と Claude Code の ESC 割り込みが mirror 経由で機能しなかった。mirror mode から list view への復帰は **`Ctrl+G`** (popup を開く tmux binding と対称) に変更。
- **UI tick**: 経過時間表示を 60 秒間隔の独立 redraw tick で更新。秒粒度を廃止し `<1m / Nm / Nh / Nd` に粗化。state reload (`r` キー) とは別経路で I/O を伴わない軽量更新。

### Added

- **State sweep**: `state.ReadAll` 起動時に旧 `"waiting_permission"` 値の state ファイルを検出した場合、警告を出して skip する移行ロジック（次の hook 発火で新値に置き換わる）。
- **Tests**: `internal/state/state_test.go` に旧値 sweep テスト、`internal/ui/mirror_test.go` を新規追加して keymap・footer 検証。

### Documentation

- v0.0.2 リファクタ Design Doc を `docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md` に追加。
- ADR 3 件を新規記録:
  - `docs/adr/20260507-forward-q-in-mirror-mode.md` (発行同日に下記の ADR で superseded)
  - `docs/adr/20260507-decouple-view-redraw-from-state-reload.md`
  - `docs/adr/20260507-mirror-quit-via-ctrl-g.md` (ESC を target pane に forward し、`Ctrl+G` で list 復帰へ)

## [v0.0.1] - 2026-05-06

Initial release: per-pane JSON state files driven by Claude Code hooks (`UserPromptSubmit` / `Notification` / `Stop`), bubbletea TUI launched from a tmux popup, list view of waiting / running / idle panes, free-text + Enter send to selected pane.
