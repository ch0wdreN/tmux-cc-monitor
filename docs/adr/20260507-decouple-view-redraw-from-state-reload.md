---
date: 2026-05-07
status: accepted
tags: [architecture, ux]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md
  - docs/adr/20260506-no-background-daemon-in-v0-0-1.md
---

# 経過時間表示用の view redraw tick を state reload と分離する

## Context

v0.0.1 の popup TUI は state reload を `r` キー手動起動と popup 起動時のみに限定し、それ以外の経路で view を周期再描画する仕組みを持っていなかった。経過時間表示 (`now.Sub(state.UpdatedAt)`) は `view()` 呼び出し時の `time.Now()` で評価されるが、bubbletea は新規 message を受け取らない限り view を再評価しないため、ユーザーがキー入力しない間は経過時間が静止して見える挙動になる。

実利用で「30 秒以上前に更新された pane なのに表示が `0s` のまま」「いつ最後に更新されたか UI から読み取れない」という不満が顕在化した。

単純な対処として「reload を周期化する」案もあるが、これは `tmuxutil.ListPanes` + `cleanup.Run` + `state.ReadAll` の I/O を伴い、ADR `20260506-no-background-daemon-in-v0-0-1.md` の精神（popup プロセス以外で無駄な処理を走らせない、I/O は明示起点でのみ）に反する方向に近づく。view 表示だけ更新できれば十分なので、reload とは別経路の軽量 tick を持たせるのが妥当という結論に至った。

## Decision

popup TUI に表示専用の `redrawTickMsg` 型を新設し、`tea.Tick(60*time.Second, ...)` で 60 秒間隔の周期 tick を回す。`Update` での `redrawTickMsg` 受信時は state を一切変更せず、次の tick を schedule するだけに留める。view 内の `time.Now() - state.UpdatedAt` 評価が再走することで経過時間表示のみが更新される。

合わせて `humanizeDuration` の表示粒度を `<1m / Nm / Nh / Nd` に粗化し、秒粒度 `Ns` を廃止する。粒度が分単位になるため 60 秒周期の redraw で表示遷移として違和感がない（60s redraw 周期 = 分単位粒度のちょうど更新点）。

state reload は引き続き `r` キーと popup 起動時に限定する。redraw tick の中では reload を呼ばない。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| 30s / 10s tick で再描画 | tick を細かくする | 表示粒度が分単位なので細かくする利得がない。tick 評価コストが無駄 |
| reload と redraw を統合し 60s ごとに reload も走らせる | tick 1 種で reload も兼ねる | 起動中 popup が常時 cleanup を走らせるのは ADR `20260506-no-background-daemon-in-v0-0-1` の精神に反する。reload 中の I/O ブロックで UI が一瞬止まるリスクもある |
| tick を入れず手動 `r` のみに依存 | 現状維持 | 経過時間が止まって見える問題そのもので採用不可 |

## Consequences

### Pros

- 経過時間表示が「止まって見える」問題を解消
- redraw と reload を分離することで、片方を停止して debug する自由度が残る
- I/O を伴わない軽量 tick なので、popup を開いている間の追加コストが極小（`time.Now()` 評価と view 文字列構築のみ）
- ADR `20260506-no-background-daemon-in-v0-0-1` の「重い I/O は明示起点でのみ」原則を守りつつ、UI の生存信号を出せる

### Cons

- mirror mode 中も redraw tick が走り続ける（mirror 中は list の経過時間表示を見ていないので無駄）。将来 mirror 中は tick を停止する最適化を検討
- 表示粒度が分単位になり、初動の最大 60 秒は `<1m` のまま動かないように見える（粒度粗化の副作用として許容）
- 60s tick は wall-clock との同期がないため、pane の `1m → 2m` 遷移が tick 周期の影響で最大 60 秒ずれる（実用上問題なし）

### References

- docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md (§6.7, §8 Decision 4)
- docs/adr/20260506-no-background-daemon-in-v0-0-1.md（本 ADR は当該 ADR の「重い I/O は popup 起動時のみ」原則の延長で「軽量 tick は popup 表示中なら可」を明確化）
- bubbletea `tea.Tick` 仕様
