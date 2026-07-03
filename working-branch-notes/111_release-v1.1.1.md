# 作業ブランチメモ

- ブランチ: release-v1.1.1
- PR: #111
- 最終更新: 2026-07-03

## 目的

v1.1.1 release 前の doc bump を 1 PR にまとめる。README の install 例を新 version へ更新し、progress.md に release 準備状況を記録する。

## 現在の状況

- release 対象: main HEAD `f723a2a518f9af1456705d73d785f5be50a040d6`
- 直前 tag: `v1.1.0`
- 対象 version: `v1.1.1`
- 最新 main CI: run `28631721932` / success / headSha `f723a2a518f9af1456705d73d785f5be50a040d6`
- release 準備 PR: #111

## 決定事項

- ユーザー指定どおり `v1.1.1` として準備する。
- 方針変更はないため、release 準備 PR では decision log を更新しない。
- 公開後の Release assets / checksum / Linux `--version` / Homebrew cask / Homebrew upgrade 検証結果は、別ブランチ・別 PR で記録する。

## 次にやること

- PR #111 の CI / review を確認する。
- ユーザーが PR #111 を merge する。
- merge 後に main HEAD を確認し、署名付き `v1.1.1` tag の作成と tag push へ進む。

## 検証

- `git fetch origin`: success
- `op plugin run -- gh run list --branch main --limit 1 --json ...`: success。最新 main CI が対象 commit で success。
- `git diff --check`: success。

## リスク・ブロッカー

- PR merge と release tag push はユーザーが行う。
- macOS binary の `--version` と Homebrew upgrade 後の `slapex --version` は、公開後にユーザー手元で確認する。

## セッションログ

- 2026-07-03: `release` skill に従い、前提チェック、version 重複確認、release 準備 doc 更新を開始。
