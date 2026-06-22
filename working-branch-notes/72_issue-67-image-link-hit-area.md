# 作業ブランチメモ

- ブランチ: issue-67-image-link-hit-area
- PR: #72
- 最終更新: 2026-06-22

## 目的

GitHub Issue #67 に従い、画像投稿の原寸リンクのクリック可能領域を画像表示領域のみに限定する。

## 現在の状況

- `.image-block a` に `display: inline-block` を追加し、画像リンクの hit area を画像幅に合わせた。
- `internal/render/html_test.go` に CSS 出力の回帰テストを追加した。

## 決定事項

- JavaScript は使わず、`.image-block a` を `inline-block` にしてリンク要素の幅を画像に合わせる。
- テンプレート構造と `.upload-thumb` の既存見た目は変更しない。

## 次にやること

- レビュー待ち。

## 検証

- `docker compose run --rm --no-deps dev go test ./internal/render` 成功。
- `docker compose run --rm --no-deps dev go test ./...` 成功。
- localhost で最小 HTML をブラウザ表示し DOM 計測:
  - `linkDisplay = inline-block`
  - `thumbDisplay = block`
  - `linkWidth = 215`, `imageWidth = 215`, `widthDelta = 0`
  - 画像右側の空白の hit test は `DIV` で、リンク配下ではない。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-06-22: Issue #67 を確認。`progress.md` に #67 の行はなく、Issue 本文にも依存タスク記載がないため依存なしで着手。
- 2026-06-22: CSS とテストを実装。Docker Compose 経由の Go テストと localhost ブラウザ DOM 計測で検証。
