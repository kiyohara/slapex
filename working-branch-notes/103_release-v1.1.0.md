# 作業ブランチメモ

- ブランチ: release-v1.1.0
- PR: #103
- 最終更新: 2026-07-02

## 目的

v1.1.0 の tag 前準備として、README の install 例、進捗管理表、リリース PR 用の作業文脈を 1 ブランチにまとめる。

## 現在の状況

- `main` / `origin/main` は `87aa0b40e71f2543d809f15feaa180b8495781e1` で一致。
- `v1.1.0` tag は未使用。
- 最新 `main` の CI run は success。
- 直前 tag は `v1.0.1`。

## 決定事項

- ユーザー指定どおり、今回のリリース準備対象は `v1.1.0` とする。
- 直前 tag からの差分には token prompt、`op run` 配下の interactive selection、CLI output UX、開発ループ / release 運用整備が含まれるため、patch ではなく minor release として扱う。
- 配布方式自体は変えないため、この準備 PR では decision log を新規作成しない。公開後の検証結果追記が必要な場合は別ブランチ / 別 PR で扱う。

## 次にやること

- README / progress.md / note の差分を確認する。
- 必要な軽量検証を行う。
- commit / push / PR 作成を行う。

## 検証

- 前提確認: `git fetch origin` 成功。
- 前提確認: 最新 `main` CI は `completed success`。

## リスク・ブロッカー

- PR merge と tag push の最終実行はユーザー担当。
- tag push 後の release assets / checksum / Linux `--version` / Homebrew cask 検証は tag 公開後に行う。

## セッションログ

- 2026-07-02: `release` skill に従い、前提チェックと v1.1.0 準備 doc 更新を開始。
