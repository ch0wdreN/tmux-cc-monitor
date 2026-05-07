---
date: 2026-05-07
status: accepted
tags: [infrastructure]
supersedes: ""
related: [docs/design-doc/20260507_tmux_cc_monitor_tpm_support_design.md, docs/adr/20260507-adopt-source-build-for-tpm-install.md]
---

# ビルド済みバイナリを TPM プラグインディレクトリ内に配置する

## Context

TPM インストール時にソースビルドする方式を採用した（ADR: adopt-source-build-for-tpm-install）。ビルドしたバイナリの配置先を決定する必要がある。

候補は 2 つ:

1. TPM がクローンしたプラグインディレクトリ内（`~/.tmux/plugins/tmux-cc-monitor/bin/`）
2. 既存の `task install` と同じ場所（`~/.config/tmux-cc-monitor/bin/`）

## Decision

ビルド済みバイナリを TPM プラグインディレクトリ内（`~/.tmux/plugins/tmux-cc-monitor/bin/tmux-cc-monitor`）に配置する。キーバインドではこのバイナリの絶対パスを使用する。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| `~/.config/tmux-cc-monitor/bin/` に配置 | 既存の `task install` と同じパスにバイナリを置く | TPM アンインストール（`prefix + alt + u`）時にバイナリが残留する。また `task install` と TPM でパスを共有すると管理主体が曖昧になる |

## Consequences

### Pros

- TPM アンインストール時にプラグインディレクトリごと削除されるため、バイナリも自動的にクリーンアップされる
- `task install` によるインストールとパスが分離されるため、両方式が干渉しない
- エントリポイントスクリプト（`tmux-cc-monitor.tmux`）から相対パスでバイナリを参照できる

### Cons

- Claude Code hooks の設定で TPM のインストール先パス（`~/.tmux/plugins/tmux-cc-monitor/bin/tmux-cc-monitor`）を指定する必要がある（`task install` 時の `~/.config/tmux-cc-monitor/bin/tmux-cc-monitor` とは異なるパス）
- TPM のインストール先をカスタマイズしているユーザーはパスが変わる

### References

- [Design Doc: TPM Support](docs/design-doc/20260507_tmux_cc_monitor_tpm_support_design.md) §8 Decision 2
