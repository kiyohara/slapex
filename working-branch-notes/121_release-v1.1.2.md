# 作業ブランチメモ

- ブランチ: release/v1.1.2
- PR: #121
- 最終更新: 2026-07-04

## 目的

v1.1.2 release 前の doc bump を 1 PR にまとめる。README の install 例を新 version へ更新し、progress.md に release 準備状況を記録する。

## 現在の状況

- release 対象: main HEAD `49c46d565e139543d837f08ce036228d1babb946`
- 直前 tag: `v1.1.1`
- 対象 version: `v1.1.2`(ユーザー指定)
- 最新 main CI: run `28699329364` / success / headSha `49c46d565e139543d837f08ce036228d1babb946`
- release 準備 PR: #121

## 決定事項

- ユーザー指定どおり `v1.1.2` として準備する。
- v1.1.1 からの主な差分: #114(出力プレビュー・サンプル export)、#116(terminal demo GIF)、#117(token 不要 `--demo`)、#119(Slack App セットアップ help スクリーンショット)、#120(export footer への tool version)。
- 方針変更はないため、release 準備 PR では decision log を更新しない。
- 公開後の Release assets / checksum / Linux `--version` / Homebrew cask / Homebrew upgrade 検証結果は、別ブランチ・別 PR で記録する。

## 次にやること

- PR #121 の CI / review を確認する。
- ユーザーが PR を merge する。
- merge 後に main HEAD を確認し、署名付き `v1.1.2` tag の作成と tag push へ進む。

## 検証

- `git fetch origin`: success。main は `origin/main` と同一。
- `op plugin run -- gh run list --branch main --limit 1`: success。最新 main CI が success。
- working tree clean を確認済み。

## リスク・ブロッカー

- PR merge と release tag push はユーザーが行う。
- macOS binary の `--version` と Homebrew upgrade 後の `slapex --version` は、公開後にユーザー手元で確認する。

## セッションログ

- 2026-07-04: `release` skill に従い、前提チェック、version 重複確認、release 準備 doc 更新を開始。
