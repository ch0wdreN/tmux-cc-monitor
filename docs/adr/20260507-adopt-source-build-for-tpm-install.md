---
date: 2026-05-07
status: accepted
tags: [infrastructure]
supersedes: ""
related: [docs/design-doc/20260507_tmux_cc_monitor_tpm_support_design.md]
---

# TPM インストール時にソースビルド方式を採用する

## Context

tmux-cc-monitor の TPM (Tmux Plugin Manager) 対応にあたり、バイナリの配布方式を決定する必要があった。TPM は `prefix + I` でプラグインリポジトリをクローンした後、リポジトリルートの `*.tmux` スクリプトを source する。このタイミングでバイナリを利用可能にする必要がある。

選択肢として GitHub Releases にプリビルドバイナリを配置してダウンロードする方式と、クローンしたソースから `go build` でビルドする方式があった。

## Decision

TPM インストール時に `go build` でソースからビルドする方式を採用する。ユーザー環境に Go のインストールが前提条件となる。Go が見つからない場合は `tmux display-message` でエラーを表示し、ビルドをスキップする。

## Alternatives Considered

| 選択肢 | 概要 | 採用しなかった理由 |
|--------|------|-------------------|
| GitHub Releases にプリビルドバイナリを配置 | リリース時にクロスコンパイルしたバイナリを GitHub Releases にアップロードし、TPM インストール時に `curl` / `wget` でダウンロードする | goreleaser 等のリリースパイプラインの構築・維持が必要。個人プロジェクトの規模に対してオーバーヘッドが大きい |

## Consequences

### Pros

- リリースパイプラインの構築・維持が不要。タグを打つだけでソースが配布される
- ユーザーの環境で常にネイティブバイナリがビルドされるため、アーキテクチャ別のクロスコンパイルが不要
- TPM の標準的なクローン → ビルドのフローに乗るため、プラグインとしての体裁が自然

### Cons

- ユーザー環境に Go のインストールが必須（Go がない環境ではプラグインが機能しない）
- 初回インストール・ソース更新時にビルド時間がかかる（数秒程度）
- ユーザーの Go バージョンが `go.mod` の要求を満たさない場合にビルドが失敗する可能性がある

### References

- [TPM: How to create a plugin](https://github.com/tmux-plugins/tpm/blob/master/docs/how_to_create_plugin.md)
- [Design Doc: TPM Support](docs/design-doc/20260507_tmux_cc_monitor_tpm_support_design.md) §8 Decision 1
