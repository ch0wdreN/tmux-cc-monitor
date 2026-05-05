---
date: 2026-05-06
status: accepted
tags: [architecture]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_design.md
  - docs/adr/20260506-adopt-per-pane-json-state-files.md
---

# v0.0.1 ではバックグラウンド常駐デーモンを置かない

> 本 ADR は v0.0.1 リリースに向けた決定事項。機能改善や調査の進展に伴い、後続バージョンで再評価する余地がある（その場合は本 ADR を Supersede する）。

## Context

tmux-cc-monitor の主用途は「ユーザーが popup を開いた瞬間に各 Claude Code セッションの状態を見て、必要なら send-keys でプロンプトを送り、即座に元の作業に戻る」ことである。状態は別途 hook 駆動で per-pane JSON ファイルに書き出されており、popup 起動時に読み込めば最新状態が得られる。

一方で、tmux status-line への待ち通知や OS 通知のような能動的な「待ちが発生した瞬間にユーザーへ知らせる」機能は、本ツールの v0.0.1 スコープには含まれていない。

このとき、バックグラウンドプロセスを常駐させるべきか、起動契機を hook と popup の 2 つだけに絞るかという判断が発生する。

## Decision

v0.0.1 ではバックグラウンドの常駐プロセスを持たない。tmux-cc-monitor は次の 2 つのプロセス起動経路だけを持つ:

1. Claude Code の hook から `tmux-cc-monitor hook <event>` が起動され、状態ファイルを更新して即終了する
2. ユーザーがキーバインドを押すと tmux popup から `tmux-cc-monitor ui` が起動され、UI を提示し、終了とともに popup が閉じる

idle 時のリソース使用は CPU 0% / メモリ 0 となる。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| `fsnotify` ベースの軽量デーモン | 状態ファイル群の変更を `fsnotify` で監視し、tmux status-line / OS 通知へ反映 | プロセスの lifecycle 管理（起動・停止・異常時の自己回復）が必要。能動通知は v0.0.1 スコープ外 |
| tmux `run-shell` を周期実行する疑似デーモン | tmux 側のフックや `set-hook` で定期的に状態集約スクリプトを起動し status-line に反映 | tmux 設定への侵襲が大きく、ユーザー側の `~/.tmux.conf` を本ツール固有のロジックで汚す。デバッグ性も低い |
| launchd で定期起動して状態を集約 | macOS の launchd で `tmux-cc-monitor aggregate` を N 秒ごとに起動 | launchd の plist 配布・再読み込み運用が必要となり、最小実装と一発インストールの方針に反する |
| tmux status-line 用の常駐デーモン | 自前の常駐プロセスで状態ファイルを監視し status-line を更新 | v0.0.1 のスコープを超える。本ツールが提供する価値は popup での横断操作であり、status-line 通知は v0.0.1 の核ではない |

## Consequences

### Pros

- プロセスの lifecycle 管理が不要（launchd、ユーザー起動スクリプト、健全性チェック等が一切要らない）
- idle 時のリソース消費がゼロ
- 状態が常にディスク上のファイルに書き出されるため、デバッグ時に `cat` で容易に確認できる
- 実装範囲が狭く、最小実装の境界に収まる

### Cons

- 「待ちが発生した瞬間に通知する」体験は提供できない。ユーザーが popup を開かないと気付けない
- popup を開くたびにディレクトリ走査・並列読み込みのコストが発生する（常駐していればキャッシュできる）
- tmux status-line 連携が必要になった段階で、本決定を見直してデーモンを足す必要がある（その際は本 ADR を Supersede する）

### References

- 関連 Design Doc: [tmux-cc-monitor Design Doc](../design-doc/20260506_tmux_cc_monitor_design.md) の「12. 設計上の意思決定 — Decision 2」
- 関連 ADR: [状態管理に per-pane JSON ファイル方式を採用する](20260506-adopt-per-pane-json-state-files.md)（本決定により、状態ファイルがそのままプロセス間通信を兼ねる構造になる）
