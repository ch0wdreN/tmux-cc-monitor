---
date: 2026-05-06
status: accepted
tags: [architecture, database]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_design.md
---

# 状態管理に per-pane JSON ファイル方式を採用する

## Context

tmux-cc-monitor は複数プロジェクトで並列起動された Claude Code の状態を hooks 経由で取得し、tmux popup 上の TUI で横断表示するためのローカルツールである。複数の Claude Code が同時刻に hook を発火し、状態ファイルを書き込もうとする状況は定常的に発生するため、書き込みの整合性を確保する仕組みが必要となる。

加えて、本ツールは「最小実装」を方針としており、常駐デーモンを置かずに hook と popup 起動の 2 経路だけで成立させたい。状態を一箇所に集約し排他制御で守るアプローチは flock やトランザクションの実装コストを伴い、最小実装の境界を越えるおそれがある。

## Decision

状態を per-pane で独立した JSON ファイルに保持する。

- 保存先: `~/.config/tmux-cc-monitor/sessions/<pane_id>.json`（1 ペイン 1 ファイル）
- 書き込み: 各 hook 呼び出しが対応する 1 ファイルだけを tmpfile + atomic rename で更新
- 読み込み: popup 起動時に `sessions/` ディレクトリ全件を並列で読み込み

書き込みパスがペインごとに完全分離されるため、書き込み競合は構造的に発生しない。flock やロックファイル、トランザクション機構は導入しない。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| 単一 JSON ファイル + flock | `state.json` 1 ファイルに全エントリを格納し flock で排他制御 | 書き込みのたびに read-modify-write が必要で非効率。flock の取り扱いと異常時の解放処理が実装コストとして無視できない |
| SQLite | 組み込み DB として SQLite を導入 (`modernc.org/sqlite` 等の純 Go 実装も含む) | (1) 単一プロセスからの単一行更新が大半でトランザクション・索引といった SQLite の利点が活きない (2) `cat` / `jq` でデバッグ・grep 可能な JSON テキストの方がメンテ性が高い（Pros の「ディスク上の状態を直接読める」と整合する） (3) DB ファイル破損時の復旧コストが、JSON 1 ファイル削除よりも高い |

## Consequences

### Pros

- 書き込み競合が原理的に発生しないため、ロック機構を一切実装しなくてよい
- hook 1 回あたりの書き込みは 1 ファイルの atomic rename のみで完結し、Claude Code 本体に与えるブロッキング時間を最小化できる
- ペインの寿命とファイルの寿命が 1:1 で対応するため、ステイル除去ロジックがシンプル（生存 pane との突き合わせで完結）
- 依存ライブラリが増えず、Go 標準パッケージのみで完結する

### Cons

- popup 起動時にディレクトリ走査が必要で、ファイル数の増大に対してリニアにコストが増える（現状想定の 50 セッション程度では問題ないが、100 を超えると要再評価）
- 全体のスナップショットを 1 トランザクションで取得することはできない（複数ファイルを並列に読む間に hook が走る可能性があるが、本ツールでは厳密整合性を要求しない）
- フォーマット変更（スキーマ進化）時に複数ファイルへの一括移行を考える必要がある。状態 JSON に `schema_version` を含め、UI 起動時に未知バージョンは警告のうえスキップする運用で対応する
- pane id (`%N`) は tmux サーバ再起動で振り直されるため、ファイル名 = pane_id 数値だけではサーバ世代を跨いだ衝突を防げない。状態 JSON 内に `tmux_server_pid` を保持し、cleanup 時に世代不一致を一律削除する設計で対応する（詳細は Design Doc §8.4）

### References

- 関連 Design Doc: [tmux-cc-monitor Design Doc](../design-doc/20260506_tmux_cc_monitor_design.md) の「12. 設計上の意思決定 — Decision 1」
