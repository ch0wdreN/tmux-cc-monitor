---
date: 2026-05-07
status: accepted
tags: [ux]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md
  - docs/adr/20260506-self-implement-popup-mirror.md
---

# Mirror mode で `q` を target pane に forward し、list 復帰は `esc` 専用とする

## Context

mirror mode は target pane への "remote keyboard" として bubbletea `KeyMsg` を `tmux send-keys` で forward する設計（ADR `20260506-self-implement-popup-mirror.md` 採用）。当該 ADR の Decision 末尾には「`q`/`Esc`/`F1` は popup 自身の制御に予約し、転送しない」という規定があり、`internal/ui/mirror.go:349-350` でも `text == "q"` の場合のみ popup を quit して list mode に戻す特別扱いが実装されていた。

しかし v0.0.1 リリース後の実利用で、この `q` 予約が以下の問題を引き起こすことが判明した:

- `git log` や `less` / `man` から `q` で抜ける操作ができない（popup が `q` を吸う）
- vim で `:q` を打とうとした瞬間、`:` 前の偶発的な `q` 入力で popup が閉じる
- target pane で動いている対話プロセスへの素朴な操作が崩れる

mirror mode は本質的に target pane の入力プロキシであり、文字キーは default で forward されるべき。quit に当てるキーは「通常コマンド入力に現れない非印字キー」に限定するのが筋という結論に至った。

## Decision

mirror mode における `q` の特別扱いを撤廃し、`q` を他のテキストと同様 target pane へ `send-keys` 送信する。mirror mode から list mode への復帰は `tea.KeyEsc` 専用とする。footer help も `esc back · ...` のみに更新し、`q quit` の表記を削除する。

list mode の `q` quit は据え置く（list mode には入力欄がなく衝突しないため）。

本 ADR は ADR `20260506-self-implement-popup-mirror.md` の Decision 末尾「`q`/`Esc`/`F1` は popup 自身の制御に予約し、転送しない」のうち **`q` 部分のみを撤廃する補強** である。`Esc` の予約は list 復帰の唯一の手段として維持し、`F1` 予約も維持する。描画方式 (capture-pane + send-keys) を含む popup-mirror ADR の本質的決定は引き続き有効なため、本 ADR は supersede ではなく `related` で関連付ける扱いとする。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| `q` 特別扱いを維持 (現状維持) | popup-mirror ADR の規定どおり `q` を予約 | 既知の不便を維持するだけで本問題が解決しない |
| 別 escape 文字 (`Ctrl+\` 等) を新設して quit に当てる | `q` も forward しつつ、別キーで quit を提供 | 既に `esc` で list 復帰できる以上 quit 用追加バインドは冗長。ユーザーが覚えるキーが増え発見性が下がる |
| `Ctrl+q` のみ quit に | `q` 単独は forward、`Ctrl+q` で quit | `Ctrl+q` は端末の software flow control (XON/XOFF) に当たることがあり挙動が環境依存で不安定 |

## Consequences

### Pros

- `q` を含む対話プロセス操作 (`git log` / `less` / `man` / vim `:q` 等) が mirror mode 経由で素直に行える
- mirror mode の離脱手段が `esc` 単一になり、ユーザーが学習する keymap が単純化される
- 「文字キーは forward が default、quit は非印字キー」という mirror mode の設計原則が一貫する

### Cons

- ユーザーは「`q` で戻れない」ことを覚える必要がある（footer help と README で明示することで緩和）
- `esc` が唯一の出口になるため、`esc` を多用するアプリ (vim の normal mode 戻し等) と組み合わせる際に誤って list に戻ってしまう可能性が残る。発生頻度が高ければ別 ADR で `esc` の forward 化を再検討する

### References

- docs/design-doc/20260506_tmux_cc_monitor_v002_refactor_design.md (§6.6, §8 Decision 3)
- docs/adr/20260506-self-implement-popup-mirror.md（本 ADR で keymap 規定を補強）
- bubbletea `KeyMsg` / tmux `send-keys` の仕様
