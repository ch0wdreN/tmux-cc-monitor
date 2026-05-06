---
date: 2026-05-06
status: accepted
tags: [architecture, ux]
supersedes: ""
related:
  - docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md
---

# popup 内ミラーは自前実装 (capture-pane + send-keys) で行う

## Context

v0.1.0 で popup の中で対象 pane を「映して操作する」UX を導入する。これは v0.0.1 の「自由テキスト + Enter 送信のみ」では満たせない次の要件に対応するもの:

- permission の対象コマンドや AskUserQuestion の質問・選択肢を見ながら応答したい
- 矢印キー等の任意のキー入力を対象 pane に送りたい
- 元 pane の表示サイズ・フォーカスに副作用を与えず、popup を閉じれば自然に元 pane に戻りたい (= 「画面全体を奪わない」)

実装方針として 2 案が挙がった:

- (A) popup 内 subprocess から `tmux attach-session -t <target>` を起動し、別 client として対象 session に attach させる
- (B) 自前実装 — `tmux capture-pane` で対象 pane を描画し、ユーザーのキー入力を `tmux send-keys` で対象 pane に転送する

詳細は Design Doc `docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md` の §1, §6 を参照。

## Decision

popup 内ミラーは **自前実装 (B)** を採用する。

具体的には:

- 描画は `tmux capture-pane -p -t <pane_id> -S -<N>` をポーリングして bubbletea で表示する
- キー入力は bubbletea の `tea.KeyMsg` を `tmux send-keys` の引数列に変換して対象 pane に転送する
- 印字可能文字は `-l` で literal 扱い、矢印・Enter・修飾キーは tmux キー名として送る
- `q`/`Esc`/`F1` は popup 自身の制御に予約し、転送しない

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| tmux attach-session in popup | popup 内 subprocess から `tmux attach-session -t <target>` を実行し、別 client として対象 session に attach。生 TUI が映る | 対象 session が現在の作業 pane と同一 session の場合、tmux の共有 attach 仕様により両 client の表示サイズが小さい方 (popup サイズ) に揃ってしまい、元 pane 側の作業画面まで縮小される。「画面全体を奪わない」要件と直接衝突する |

## Consequences

### Pros

- 同一 session / 別 session のいずれであっても popup 内に表示が閉じ、元 pane の表示サイズ・フォーカスに副作用が発生しない
- popup を閉じれば自動で元 pane に戻る (popup は client overlay であり、元 pane id を保存する仕組みや戻りキーバインドが不要)
- 依存ライブラリの追加なし — capture-pane と send-keys は v0.0.1 の段階で既に使っているコマンド系列の延長
- 「state 判定では capture-pane を使わない」(ADR-0003) との両立が、用途分離の論理で明確に成立する

### Cons

- TUI 装飾 (色・カーソル位置・部分更新) の完全再現はできない — capture-pane プレーンテキスト出力の制約。permission/AskUserQuestion/応答テキストの内容把握には十分だが、リッチな UI 表現は犠牲
- bubbletea の key event を tmux send-keys 引数に変換するマッピングを自前で書く必要がある (Phase 0 で実機検証して確定する)
- ポーリング (500ms) による定常的な capture-pane 呼び出しコストが発生する — 単一 pane に対しては軽微だが、原理的に「常時 0」ではない

### References

- Design Doc §1, §6 (アーキテクチャと描画戦略): docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md
- Design Doc §12 Decision 1 (採用根拠の詳細)
- ADR-0003 (state 判定で capture-pane を使わない): docs/adr/20260506-use-claude-code-hooks-for-state.md
- tmux(1) `capture-pane`, `send-keys`, `display-popup`
