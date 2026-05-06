# tmux-cc-monitor v0.0.2 リファクタ・調整 Design Doc

| 項目 | 内容 |
|---|---|
| Author | ch0wdreN |
| Reviewer | ch0wdreN (self-review) |
| Status | Draft |
| Created | 2026-05-06 |
| Updated | 2026-05-07 |
| Target version | v0.0.2 |

---

## 1. 概要

v0.0.1 リリース後の v0.0.2 として、UI 文言・ステータス意味論・キー操作・経過時間表示・Notification 分類の 5 領域に渡る一連のリファクタと挙動調整を 1 本にまとめる。新規機能追加ではなく、既存機能の使い勝手と意味論を実利用フィードバックに基づいて改善する。

## 2. 背景と目的

v0.0.1 リリース後の実利用を通じて以下の不満が浮上した:

- **赤の主張が強すぎる**: 「Permission Waiting」セクションの赤色見出しが視覚的にうるさく、複数 pane が並んだ popup で全体の信号価値が下がる
- **mirror mode で `q` が target pane に送れない**: 現状 `q` は popup 側が吸って list に戻すため、`git log` / `less` / `vim` 等の素朴な対話操作が崩れる
- **経過時間が止まって見える**: ティックがないため、キー操作しない限り `now - updated_at` が再評価されず表示が静止する
- **Notification の `permission_prompt` 以外がすべて waiting_other に集約される**: `elicitation_dialog` (ユーザー入力待ち) と `auth_success` (情報通知) が同じカテゴリに落ち、UI から「対応が必要」を読み取りにくい

これらは個別には小さいが、ツールの「ペイン状態を一目で判別する」中核体験を支える。1 本の patch リリース v0.0.2 にまとめてレビュー・反映する。

## 3. スコープ (Scope)

### In Scope

1. UI 文言・色変更 (Permission Waiting → Action Waiting / red → cyan / `[PERM]` → `[ACTION]`)
2. Status リネーム (`StatusWaitingPermission` → `StatusWaitingAction`、JSON 値も `"waiting_action"` に変更)
3. Mirror mode の `q` キー forward 化 + `esc` のみで list へ戻る
4. 経過時間表示の粒度粗化 (`<1m` 集約) + 60 秒の view redraw tick
5. Notification 分類の見直し (subtype を 4 status に再振り分け、`waiting_other` を fallback 専用に)

### Out of Scope

- 新機能追加 (popup-mirror 拡張、別ペイン操作 UI、複数ホスト対応 等)
- ADR `20260506-no-background-daemon-in-v0-0-1` / `20260506-use-claude-code-hooks-for-state` のアーキテクチャ変更
- `tmux capture-pane` の利用範囲拡張 (既存「popup display のみ」境界を維持、`task check-state-purity` 維持)
- v0.1.0 以降のマイルストーン項目 (cleanup ロジック改修、`schema_version` 引き上げ 等)

## 4. 制約条件 (Constraints)

| 種別 | 内容 |
|---|---|
| アーキテクチャ制約 | ADR `20260506-no-background-daemon-in-v0-0-1` (no daemon) と ADR `20260506-use-claude-code-hooks-for-state` (hooks 駆動) を維持する。本 Doc の範囲ではどちらにも変更を加えない |
| 互換性制約 | 個人ツールのため外部互換性は考慮しない (破壊的変更可)。state ファイルは `~/.config/tmux-cc-monitor/sessions/` 配下で本人のみが触る |
| Schema 制約 (pre-stable 期の特例) | 現在 v0.0.x で minor / major リリース未到達のため、`schema_version` は揺らいでいる前提で運用する。`schema_version` 自体は据え置き、JSON 値リネームを patch リリースで実施する。**この特例は pre-1.0 期に限定し、v0.1.0 以降は schema 値変更時に必ず `schema_version` を引き上げる** |

## 5. 受け入れ基準 (Acceptance Criteria)

- [ ] 見出しが `Action Waiting`、バッジが `[ACTION]` で表示される (`internal/ui/ui_test.go` で検証)
- [ ] `Action Waiting` セクション/バッジが cyan(14) bold で描画される (style 定数 grep で確認)
- [ ] `state.StatusWaitingAction` が定義され、`StatusWaitingPermission` は grep ヒット 0 件
- [ ] state JSON 値が `"waiting_action"` で書かれる (`internal/hook/hook_test.go` で確認)
- [ ] `state.ReadAll` が旧値 `"waiting_permission"` を読んだとき errlog に warn を append し、当該 state を skip する (`internal/state/state_test.go` で検証)
- [ ] mirror mode で `q` キーが target pane に send-keys される (`internal/ui/mirror_test.go` の keymap test)
- [ ] mirror mode の help footer に `esc back` が明示され、`q quit` の表記が削除されている (view test)
- [ ] `humanizeDuration(<1m)` が `"<1m"` を返す (unit test)
- [ ] 60 秒間隔の `redrawTickMsg` が schedule され、受信時に view が再描画される (model test、`time.Now()` 評価が反映されることを確認)
- [ ] hook の Notification 分類が §6.5 の表のとおりに振り分けられる (`internal/hook/hook_test.go` の table-driven test)
- [ ] `Waiting (other)` セクション見出しが faint で描画される (`styleSectionNeutral` 適用)
- [ ] `task verify` が緑

## 6. システム設計

### 6.1 影響範囲

| ファイル | 変更内容 |
|---|---|
| `internal/state/state.go` | `Status` 定数リネーム / 値変更 / 旧値の取り扱いコメント |
| `internal/state/io.go` | `ReadAll` で旧値 `"waiting_permission"` 検出時に errlog warn + skip |
| `internal/hook/hook.go` | `classifyEvent` の Notification 分岐を §6.5 の表のとおりに刷新 |
| `internal/ui/styles.go` | `styleSectionPermission` を cyan(14) bold へ / `styleSectionWaitingOther` を `styleSectionNeutral` 利用に置換 / `styleBadgePermission` リネーム |
| `internal/ui/ui.go` | section title `"Permission Waiting"` → `"Action Waiting"` / バッジ `[PERM]` → `[ACTION]` / 60s redraw tick 追加 / `humanizeDuration` 粒度変更 / footer help 更新 |
| `internal/ui/mirror.go` | `q` 専用 quit を撤廃。`text == "q"` の特別扱いを削除し他のテキストと同様 forward |
| `internal/ui/ui_test.go` 他 | 上記の検証テスト追加・更新 |
| `internal/state/state_test.go` | 旧値 skip の挙動テスト追加 |
| `cmd/probe-hook/main.go` | 影響なし (status enum 名は probe では参照しない) |

### 6.2 UI 文言・色変更

| 要素 | Before | After |
|---|---|---|
| セクション見出し | `Permission Waiting` (red 9, bold) | `Action Waiting` (cyan 14, bold) |
| バッジ | `[PERM]` (red 9, bold) | `[ACTION]` (cyan 14, bold) |
| `Waiting (other)` 見出し | yellow 11, bold | `styleSectionNeutral` (faint) |
| `Waiting (other)` バッジ | `[WAIT]` (yellow 11) | `[WAIT]` (faint) |
| `Running` 見出し / バッジ | green 10 | 変更なし |
| `Idle` 見出し / バッジ | faint | 変更なし |

(意図) 「ユーザーが対応すべき pane」を一目で識別する機能を維持しつつ、赤の主張を抑える。Running の green と被らない cyan を採る。`Waiting (other)` は fallback 専用化されるため、平時は静かに表示する。

### 6.3 Status リネーム

```go
// internal/state/state.go (after)
const (
    StatusWaitingAction Status = "waiting_action" // was StatusWaitingPermission / "waiting_permission"
    StatusWaitingOther  Status = "waiting_other"
    StatusRunning       Status = "running"
    StatusIdle          Status = "idle"
)
```

`schema_version` は 1 のまま据え置く。pre-1.0 の特例として §4 のとおり値リネームを patch リリースで実施する。

### 6.4 旧 state ファイルの sweep

`state.ReadAll` (`internal/state/io.go`) は per-pane JSON ファイルを順に decode する過程で `"status"` の値を確認する:

- `waiting_action` / `waiting_other` / `running` / `idle` のいずれかなら受理
- `"waiting_permission"` を見つけた場合: errlog に warn (既存の `reload-warning` 系統を流用) を append し、その state を skip
- それ以外の未知値も既存の警告ロジックに準じる

実態として、popup を起動した次に hook が発火すれば新値で上書きされるため、ペイン側の状態ファイルは自然に置き換わる。残るのは「pane が hook を発火しないまま放置されている」極端なケースだけで、cleanup の TTL でいずれ消える。

### 6.5 Notification 分類見直し

`internal/hook/hook.go::classifyEvent` の Notification 分岐を以下に置き換える:

| `notification_type` | 新分類 | 備考 |
|---|---|---|
| `permission_prompt` | `waiting_action` | 既存ロジック踏襲、`composePermissionMessage` 維持 |
| `idle_prompt` | `idle` | 一定時間応答なしの催促。pane が席を外した状態をハイライトすると常時 cyan になり信号価値が下がるため idle に分類 |
| `elicitation_dialog` | `waiting_action` | フォーム表示中、ユーザー入力待ち |
| `elicitation_response` | `running` | ユーザー入力受領後、処理続行中 |
| `elicitation_complete` | `running` | フォーム完了後、処理続行中 |
| `auth_success` | `idle` | 情報通知のみ、ユーザー即時アクション不要 |
| 上記以外 (将来追加) | `waiting_other` | 未知シグナルの可視化 (Claude Code 仕様変更検出器) |

`waiting_other` セクションは UI 上 neutral 表示 (faint) に降格し、「想定外の `notification_type` を発見するためのフォールバック領域」という運用役割に純化する。平時は空であることが期待値で、何か出ていれば Claude Code 側の仕様追加 / 変更のシグナルとして手動調査する。

`composePermissionMessage` のロジックは `permission_prompt` 専用のまま据え置き (今回スコープ外)。

### 6.6 Mirror mode の q forward

現状 `internal/ui/mirror.go:349-350`:

```go
// 'q' is reserved as quit in mirror mode.
if !msg.Alt && text == "q" {
    return ..., quit
}
```

これを撤廃。mirror mode における list 復帰は `tea.KeyEsc` のみとなり、`q` は他のテキストと同様に target pane へ send-keys される。footer help を以下のとおり更新:

- Before: `q quit · esc back · ...`
- After: `esc back · ...`

list mode の `q` quit は不変 (list には入力欄がなく衝突しない)。

### 6.7 経過時間表示と redraw tick

`humanizeDuration` を以下に変更:

| `d` | 表示 |
|---|---|
| `< 1m` | `<1m` |
| `< 1h` | `Nm` |
| `< 24h` | `Nh` |
| `>= 24h` | `Nd` |

(変更点: 秒粒度の `Ns` を廃止し `<1m` に集約)

新規 message type:

```go
type redrawTickMsg struct{}

func scheduleRedrawTick() tea.Cmd {
    return tea.Tick(60*time.Second, func(time.Time) tea.Msg { return redrawTickMsg{} })
}
```

`Init` および `redrawTickMsg` 受信時に次の tick を schedule する (mirror tick と同様のパターン)。`Update` の `redrawTickMsg` 分岐は state を変えず次 tick を返すだけ。view の `time.Now()` が再評価されることで経過時間表示が更新される。

state reload (`reloadStates`) とは別経路: reload は `tmuxutil.ListPanes` / `cleanup.Run` / `state.ReadAll` の I/O を伴うため `r` キー起動のままとし、redraw tick は表示の表計算のみに留める。

## 7. リスクと懸念事項

| リスク | 影響度 | 対応方針 |
|---|---|---|
| Notification 分類の「仮説」性 — `idle_prompt` 他を `idle` に振った判断が実用で外す可能性 | Medium | 実運用で「`idle` だと見逃した」ケースが観測されたら、ADR を起こして該当分類を `waiting_action` に戻す。当面は `notification_type` を `last_message` に残しておくことで grep 観測可能 |
| 60s redraw tick の CPU 負荷 — mirror mode の周期 capture と並列で走ると無駄 | Low | popup プロセスは利用中のみ起動し、tick 間隔が 60s なので実害は小さい。mirror mode 中は経過時間表示そのものを使わない (list mode を抜けている) ため、将来 mirror 中は redraw tick を停止する最適化を検討 |
| `q` forward により popup 終了手段が減る — 「うっかり閉じる」操作がしづらくなる | Low | mirror mode は `esc` で list 復帰、list mode は `q` / `esc` / `Ctrl+C` で終了、と二段階手順を README と footer help で明示 |
| Schema 値変更を patch リリースで行う前例化 | Medium | pre-1.0 限定の運用と Constraints および ADR で明示。v0.1.0 以降は `schema_version` 引き上げを必須とする方針を §10 Future Work で言及 |

## 8. 設計上の意思決定 (Design Decisions)

### Decision 1: Status `waiting_permission` を `waiting_action` に意味論ごと改名

| | 内容 |
|---|---|
| 決定事項 | Go 定数名・JSON 値・UI 表記を `waiting_permission` → `waiting_action` に改名し、permission 待ち以外のユーザーアクション要求 (例: `elicitation_dialog`) も同一カテゴリに集約する |
| 理由 | Claude Code の Notification subtype に permission_prompt 以外で user 操作を要求するもの (`elicitation_dialog`) が含まれることが Phase 0 調査で判明している。「permission 待ち」という意味論で UI セクションを切り続けると、新 subtype の落とし所がなくなる |
| 検討した代替案 | (A) UI 表記だけ "Action Waiting" に変え内部は `waiting_permission` のまま。(B) `waiting_action` を新設し `waiting_permission` を残す (両 status 並存) |
| 代替案を選ばなかった理由 | (A) 内部名と UI 表記の乖離が将来 contributor を混乱させる。(B) 同義の status が 2 つあると分類ロジックが分岐し続け、Notification の意味論統合という本来目的が果たせない |

### Decision 2: Notification subtype を 4 status に振り分ける (waiting_other は fallback 専用)

| | 内容 |
|---|---|
| 決定事項 | §6.5 の表に従い、`idle_prompt` / `auth_success` を `idle`、`elicitation_response` / `elicitation_complete` を `running`、`permission_prompt` / `elicitation_dialog` を `waiting_action`、未知の subtype を `waiting_other` (= 想定外検出器) に分類する |
| 理由 | UI の "Action Waiting" セクションをユーザーアクションが必要な pane だけに絞ることで、cyan ハイライトの過剰発火を防ぎ信号価値を保つ。`waiting_other` を未知 subtype の検出器として残せば Claude Code 仕様変更を即時に発見できる |
| 検討した代替案 | (A) すべて `waiting_action` に集約、(B) すべて従来通り `waiting_other` に集約、(C) subtype ごとに新 status を新設 |
| 代替案を選ばなかった理由 | (A) `idle_prompt` や `auth_success` まで cyan ハイライトされ、本当に対応すべき pane が埋もれる。(B) UI 改善の目的が達成されない。(C) status 数を増やすと list の section 数が増え、popup の縦領域を圧迫する |

### Decision 3: Mirror mode の `q` を target pane に forward する

| | 内容 |
|---|---|
| 決定事項 | mirror mode における `q` の特別扱い (popup quit) を撤廃し、他のテキストと同じく target pane に send-keys する。list 復帰は `esc` のみ |
| 理由 | mirror mode は target pane への "remote keyboard" として機能する。`q` を popup 側で吸ってしまうと「git log から抜ける」「less / man を抜ける」「vim を `:q` で終了する手前の `q`」等の素朴な操作が崩れる。`esc` は通常コマンド入力に現れず衝突しない |
| 検討した代替案 | (A) `q` 維持、(B) escape 文字 (`Ctrl+\` 等) を新設、(C) modifier 付き (`Ctrl+q`) のみ quit に |
| 代替案を選ばなかった理由 | (A) 既知の不便を維持するだけ。(B) ユーザーが覚える必要のあるキーが増え発見性が下がる。(C) `Ctrl+q` は端末の software flow control に当たることがあり挙動が不安定 |

### Decision 4: 経過時間表示は 60 秒単位の独立 tick で再描画する

| | 内容 |
|---|---|
| 決定事項 | popup の経過時間表示を分単位に粗化し、60 秒間隔の `redrawTickMsg` を回す。state I/O を伴う reload は別経路 (`r` キー) に分離維持 |
| 理由 | (1) 経過時間は分単位で十分。(2) state reload は I/O を伴う重い処理で 60s ごとに走らせる必要はない。(3) 表示再描画と state reload を別経路にすれば、片方を止めて debug する自由度が残る |
| 検討した代替案 | (A) 30s / 10s tick、(B) reload と redraw を統合し 60s ごとに reload も走らせる、(C) tick を入れず手動 `r` のみに依存 |
| 代替案を選ばなかった理由 | (A) 表示粒度が分単位なら細かくする利得がない。(B) 起動済み popup が常時 cleanup を走らせるのは ADR `20260506-no-background-daemon-in-v0-0-1` の精神に反する。(C) 現状の不満そのものなので採用不可 |

## 9. 実装計画

| フェーズ | 内容 | 検証 |
|---|---|---|
| Phase 1 | state リネームと旧値 sweep — `internal/state/state.go`, `internal/state/io.go` | `state_test.go` 追加 |
| Phase 2 | hook の Notification 分類刷新 — `internal/hook/hook.go` | `hook_test.go` の table-driven test |
| Phase 3 | UI 文言・色・バッジ・section style — `internal/ui/styles.go`, `internal/ui/ui.go` | `ui_test.go` の view test |
| Phase 4 | mirror mode の q forward — `internal/ui/mirror.go` | `mirror_test.go` の keymap test |
| Phase 5 | redraw tick + `humanizeDuration` 粒度変更 — `internal/ui/ui.go` | unit test + model test |
| Phase 6 | `task verify` 緑化、CHANGELOG / README 反映、v0.0.2 タグ | manual smoke test |

各 Phase 終了時点で `task verify` が緑であることを保証する。Phase は機能境界に対応するため、PR は基本 Phase ごとに分けることを推奨する (PR 粒度ルール準拠)。

## 10. Future Work

- v0.1.0 リリース時に `schema_version` を 2 に引き上げ、本 Doc の `waiting_permission` 旧値読み取り時の warn ロジックを撤去する
- v0.1.0 以降の運用ルール: schema 値の変更は必ず `schema_version` 引き上げを伴う (本 Doc の「pre-1.0 特例」を v0.1.0 で終了する)
- Notification subtype 分類を実運用で観測し、`idle_prompt` / `auth_success` の振り分けを再評価する ADR を起こすか判断
- mirror mode 中は redraw tick を停止する最適化 (list を抜けているため経過時間表示が不要)

## 11. 参考資料

- `docs/design-doc/20260506_tmux_cc_monitor_design.md` — v0.0.1 の Authority
- `docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md` — mirror feature の Authority
- `docs/adr/adr-index.json` — ADR 一覧
- ADR `20260506-use-claude-code-hooks-for-state.md` — Notification 分類の根拠 (本 Doc で意味論を拡張)
- ADR `20260506-self-implement-popup-mirror.md` — mirror mode 設計の根拠 (本 Doc でキーマップを修正)
- Claude Code hooks 公式仕様 (`UserPromptSubmit` / `Notification` / `Stop` の payload structure)

## 12. Related ADRs

本 Doc から派生した新規 ADR:

- [Mirror mode で q を target pane に forward し、list 復帰は esc 専用とする](../adr/20260507-forward-q-in-mirror-mode.md) — Decision 3 を ADR 化
- [経過時間表示用の view redraw tick を state reload と分離する](../adr/20260507-decouple-view-redraw-from-state-reload.md) — Decision 4 を ADR 化

本 Doc が参照・延長する既存 ADR:

- [popup 内ミラーは自前実装 (capture-pane + send-keys) で行う](../adr/20260506-self-implement-popup-mirror.md) — `q` 予約規定を新 ADR で部分的に補強
- [v0.0.1 ではバックグラウンド常駐デーモンを置かない](../adr/20260506-no-background-daemon-in-v0-0-1.md) — 「重い I/O は明示起点でのみ」原則を view redraw tick ADR が継承
- [状態取得は Claude Code hooks に依拠し、tmux capture-pane でのポーリングはしない](../adr/20260506-use-claude-code-hooks-for-state.md) — Notification 分類見直し (§6.5) の意味論的根拠

ADR 化されなかった本 Doc 内の決定事項（Decision 1: status `waiting_action` 改名、Decision 2: Notification subtype 分類）の根拠は本 Doc §8 が単独の参照源となる。
