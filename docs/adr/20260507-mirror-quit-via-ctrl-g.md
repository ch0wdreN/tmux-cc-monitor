---
date: 2026-05-07
status: accepted
tags: [ux]
supersedes: docs/adr/20260507-forward-q-in-mirror-mode.md
related:
  - docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md
  - docs/adr/20260506-self-implement-popup-mirror.md
---

# Mirror mode の離脱キーを Ctrl+G に変更し、ESC を target pane に forward する

## Context

ADR `20260507-forward-q-in-mirror-mode.md`（同日付）では mirror mode の離脱キーを `tea.KeyEsc` 専用に限定し、`q` を含む文字キーは target pane に forward する設計を採用した。これは popup-mirror 元 ADR の `q`/`Esc`/`F1` 予約のうち `q` の予約だけを撤廃する補強 ADR だった。

しかし当該設計を実装直後にレビューしたところ、決定的な見落としが判明した:

**Claude Code 自身が ESC を割り込み (interrupt) キーとして使う**

target pane で動いている Claude Code に対してユーザーが ESC で割り込みをかけたい場合、popup 側が ESC を吸って list view に戻ってしまうと、ESC を Claude Code に送ることが永続的にできなくなる。これは popup mirror が target pane の "remote keyboard" として機能するという基本的な設計目的を損なう。

問題の構造は `q` の場合と同じ: 「target pane で意味のあるキーを popup 側で吸ってはいけない」。ESC は `q` 以上に Claude Code 操作で頻用されるため優先度が高い。

直前の ADR が発行同日に覆ることになるが、実装が main にマージされる前 (= v0.0.2 リリース前) に発見できたので、リリース前に supersede する。

## Decision

mirror mode の離脱キーを **`Ctrl+G`** (`tea.KeyCtrlG`) に変更する。

具体的には:

- `tea.KeyEsc` を target pane に forward する (`actionSendKeyName "Escape"`)。これにより Claude Code の ESC 割り込みが mirror 経由で動くようになる
- `q` は引き続き target pane に forward する (前 ADR の決定を踏襲)
- `Ctrl+G` を mirror mode の唯一の離脱キーとし、list view に戻す
- footer help は `ctrl-g → list` 表記に変更する
- list mode の `q` / `Esc` / `Ctrl+C` quit は据え置き (list mode には入力欄がなく衝突しないため)

`Ctrl+G` を選んだ根拠:

- popup を開く tmux binding (例: `bind C-g display-popup -E ... 'tmux-cc-monitor ui'`) と対称的なキーで、popup 開閉の操作系列が一貫する
- Claude Code は `Ctrl+G` を機能キーとして使わない
- Emacs 文化圏では `Ctrl+G` は cancel / abort の意味を持ち、「離脱」の認知バイアスと整合する

本 ADR は ADR `20260507-forward-q-in-mirror-mode.md` を **supersedes** する。前 ADR の「mirror mode から list 復帰は esc 専用」の決定は本 ADR で覆る。前 ADR の `q` forward の決定は本 ADR でも維持する。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| F2 (F1 を popup help 用に温存し、F2 を quit に) | function key で他と衝突しない | macOS の Touch Bar / fn 修飾キーとの併用が面倒。覚えやすさで Ctrl+G に劣る |
| Ctrl+] (telnet escape と同じ) | ヘビーユーザーには馴染みあり | JIS キーボードでは `]` が打鍵しづらく、Mac 環境で発見性が低い |
| ESC ダブルタップ (1 秒以内に 2 回 ESC で quit) | ESC を forward しつつ「連打」だけ quit | 1 回目の ESC は必ず先に forward されるため、Claude Code に意図せず割り込みが発火する。ESC 連打が必要なケースで誤発火リスクが残る |
| Ctrl+Q | quit 系として直感的 | 端末の software flow control (XON/XOFF) に当たることがあり挙動が環境依存。前 ADR でも棄却済み |

## Consequences

### Pros

- `q` と `Esc` の両方が target pane に forward されるため、Claude Code の主要操作 (ESC 割り込み、`q` で less / git log 抜け、vim の `:q` 直前の `q` 等) が mirror 経由で素直に行える
- mirror mode の入力プロキシとしての完全性が高まる (target pane で意味を持つ文字キー / ESC を popup が吸わない)
- 離脱キーが popup を開く binding (`Ctrl+G`) と対称になり、ユーザーの認知負荷が下がる

### Cons

- 離脱キーが文字キー / ESC ではない非自明なキーになるため、初見ユーザーは README や footer help を見ないと離脱できない (footer に `ctrl-g → list` を常時表示することで緩和)
- 前 ADR `20260507-forward-q-in-mirror-mode.md` を発行同日に supersede するため ADR ログ上は珍しい形になる (supersede 関係を明示することで履歴は追跡可能)
- `Ctrl+G` は端末で BEL (`\a`, ASCII 7) を発生させる文字でもあるが、bubbletea が `tea.KeyCtrlG` として捕獲するため target pane に伝搬しない (副作用なし)

### References

- docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md (§6.6 を本 ADR の決定で更新)
- docs/adr/20260507-forward-q-in-mirror-mode.md (本 ADR で supersede)
- docs/adr/20260506-self-implement-popup-mirror.md (元 ADR、`q`/`Esc`/`F1` 予約のうち `q` と `Esc` の予約を本 ADR で撤廃。`F1` 予約は維持)
- bubbletea `KeyCtrlG`、tmux `send-keys -t <pane> Escape`
