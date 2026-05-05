---
date: 2026-05-06
status: accepted
tags: [architecture, dependency]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_design.md
---

# 状態取得は Claude Code hooks に依拠し、tmux capture-pane でのポーリングはしない

## Context

tmux-cc-monitor は「どの Claude Code セッションが permission 待ちか / 応答中か / idle か」を識別し、popup 上で分割表示する必要がある。状態の入力源として、以下の 2 系統が現実的に存在する。

1. Claude Code が発火する hook（`UserPromptSubmit`, `Notification`, `Stop` 等）に処理を仕込み、イベント駆動で状態を能動的に通知させる
2. 各 tmux pane の出力を `tmux capture-pane` で一定間隔スナップショットし、出力末尾の文字列パターンから状態を推定する

本ツールは Claude Code のラッパー的な位置付けにあり、Claude Code 本体の動作と密接に協調する前提で良い。一方で、capture-pane 方式は Claude Code 側の UI 実装の差分（言語・色・装飾・改行）に依存する。

## Decision

状態取得は Claude Code の hooks に依拠する。各 hook から `tmux-cc-monitor hook <event>` を呼び、`$TMUX_PANE` を読んで対応する状態ファイルを更新する。`tmux capture-pane` ベースのポーリング方式は採用しない。

イベントごとの状態遷移は次の通り（最後に発火した hook が状態を上書きする単純なステートとする）:

| hook | 遷移後の status |
|---|---|
| `UserPromptSubmit` | `running` |
| `Notification` (permission 系) | `waiting_permission` |
| `Notification` (その他) | `waiting_other` |
| `Stop` | `idle` |

`Notification` を一律で `waiting` に倒すと、permission 待ち以外の通知 (idle 通知、長時間応答通知など) が popup の「permission 待ち」セクションに紛れ込む。これは v0.0.1 の核体験「permission 待ちを popup で見つけて即送信」を機能不全にするため、payload を見て二段に分類する。具体的な判定キー（`message` の内容、`title`、permission 系を示すフィールドの有無など）は Phase 0 の実機調査で確定し、本 ADR と Design Doc §8.3 に書き戻す。

判定が確定するまでは v0.0.1 では `last_message` に hook payload の `message` フィールド（あるいは相当物）をそのまま入れる。要約や表示整形は v0.0.1 以降のバージョンで対応する。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| `tmux capture-pane` を一定間隔でスナップショットして状態推定 | 全 pane の出力末尾を文字列マッチして「permission 待ち」「応答中」「idle」を推定 | (1) 「待ち」かどうかの判定が文字列マッチに依存し、Claude Code の UI 装飾・言語・改行の差分で容易に破綻する。(2) 全 pane を一定間隔でポーリングする CPU/IO コストが、ペインが増えるほど線形に増加する。(3) capture-pane の出力からは「permission 内容」のような構造化情報が直接得られず、プロンプト推奨表示の質も下がる |

## Consequences

### Pros

- 状態の意味が明確に取れる（hook 名がそのままセマンティクスになる）
- イベント駆動なので、ポーリングと違って idle 時のリソース消費がない
- hook payload を `raw_payload` として状態 JSON に保持しておけば、判定ロジックを後付けで強化できる（Claude Code 仕様の追加情報に追従しやすい）
- Claude Code 本体の UI 装飾変更の影響を受けない

### Cons

- Claude Code の hooks 仕様変更（イベント名・引数・呼び出しタイミングの変更）に追従が必要になる。ラッパー的位置付けである以上不可避と割り切る
- hook の登録漏れ（グローバル設定にしないと一部プロジェクトで状態が取得できない）など、ユーザー側の設定運用が必要
- `Notification` の細分化判定を payload に依存するため、payload キーの仕様変更で誤分類が再発するリスクがある。`raw_payload` を保持して後追い修正できる構造にする
- hook バイナリが落ちた場合に状態が古いまま残る可能性がある。`hook-errors.log` への記録と UI フッタでの観測で対処する（Design Doc §9）

### References

- 関連 Design Doc: [tmux-cc-monitor Design Doc](../design-doc/20260506_tmux_cc_monitor_design.md) の「12. 設計上の意思決定 — Decision 3」
- Claude Code hooks 公式ドキュメント（`UserPromptSubmit` / `Notification` / `Stop` 仕様）
