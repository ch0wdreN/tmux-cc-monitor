---
date: 2026-05-06
status: accepted
tags: [architecture, ux]
supersedes: ""
related:
  - docs/adr/20260506-use-claude-code-hooks-for-state.md
  - docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md
---

# popup mirror の描画用途に限り capture-pane の利用を許容する

## Context

ADR-0003 (`docs/adr/20260506-use-claude-code-hooks-for-state.md`) は「状態取得は Claude Code hooks に依拠し、tmux capture-pane でのポーリングはしない」と決めている。理由は (1) 文字列マッチによる state 判定が脆弱で誤検知/見逃しが発生する (2) Claude Code 側の UI 装飾変更で破綻する (3) permission の構造化情報が取れず popup 表示の質が落ちる、という 3 点だった。

v0.1.0 で popup mirror を導入するにあたり、対象 pane の TUI を popup 内に映すために `tmux capture-pane` を**新規に呼び出す**ことになる。これが ADR-0003 と整合するかを明示的に決める必要が生じた。

## Decision

ADR-0003 を **維持** する。すなわち:

- **state 判定 (どのセッションが permission 待ちか / idle か等の分類)** には引き続き capture-pane を**使わない**。状態の一次情報源は Claude Code hooks の payload (`raw_payload.notification_type` 等) のままとする
- 一方で **popup mirror の表示パス (描画用)** に限っては capture-pane の利用を**許容**する
- 用途分離を保つため、Design Doc §5 の Acceptance Criteria に「state 判定経路に `capture-pane` 文字列が現れない」ことを grep ベースで自動チェックする項目を入れる

本 ADR は ADR-0003 を Refines / 補完する関係にあり、Supersede ではない。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| ADR-0003 を Supersede し state 判定にも capture-pane を許可する | 用途を分けず capture-pane を全面解禁する | ADR-0003 が排除した「文字列マッチによる state 判定の脆弱性」がそのまま戻る。Claude Code 側の UI 装飾変更で破綻するリスクも復活する |
| capture-pane を一切使わず、mirror も hook payload (`raw_payload`) の文字列で代替する | 状態取得・表示ともに hooks 一本にする | `raw_payload` には「対象 pane の現在の TUI 表示」が含まれていない (hook 発火時点のスナップショットのみ)。permission 後の応答経過などの「いま画面に出ているもの」を映せず、popup mirror の主目的が達成できない |

## Consequences

### Pros

- ADR-0003 の核心 (state 判定の脆弱性回避) を維持したまま popup mirror の主目的を達成できる
- 用途分離 (state 判定 vs 表示) を Acceptance Criteria の grep AC として明文化することで、将来の混入を機械的に防止できる
- 表示用途の capture-pane は文字列の意味解釈を伴わないため、ADR-0003 が指摘した脆弱性とは構造的に無関係

### Cons

- 「state 判定では使わない / 表示では使う」という用途別ルールを維持し続ける規律コストが発生する。Acceptance Criteria の grep AC によって機械化はするが、CI に組み込まないと脆い
- 将来 capture-pane の出力を「状態の補助情報として使いたくなった」場合、本 ADR の用途制限を見直す必要が出てくる (その時点で Supersede / 改訂の議論が必要)

### References

- ADR-0003 (state 判定で capture-pane を使わない): docs/adr/20260506-use-claude-code-hooks-for-state.md
- Design Doc §3 (Out of Scope に「capture-pane を state 判定に使う」を明記), §5 (Acceptance Criteria に grep AC を含む), §12 Decision 2: docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md
- tmux(1) `capture-pane`
