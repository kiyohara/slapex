# 作業ブランチメモ

- ブランチ: `issue-81-slack-token-v101`
- PR: #84
- 最終更新: 2026-06-26

## 目的

Issue #81 の対応として、`slapex` のデフォルト利用方法を user token に切り替え、`SLACK_TOKEN` を唯一の token 環境変数として実装・文書化する。既存 `SLACK_BOT_TOKEN` との互換性は維持しない。今回の変更は v1.0.1 に進められる状態にする。

## 現在の状況

- `main` は `origin/main` と一致している状態から作業ブランチを作成した。
- Slack 公式 docs は token 種別、OAuth install、scope、`conversations.history` / `conversations.replies` の token type ごとの差分を確認してから文書へ反映する。
- CLI 実装、README、help、設計文書、decision log、progress、install 例を `SLACK_TOKEN` / user token default / v1.0.1 方針に更新した。

## 決定事項

- `SLACK_TOKEN` を唯一の入力環境変数にする。
- `SLACK_BOT_TOKEN` fallback、両方指定時の優先順位、互換警告は実装しない。
- bot token は引き続き正式サポートするが、CI / automation 向けの選択肢として `SLACK_TOKEN` に渡す。
- Issue #48 のスクリーンショット付き UI 手順追加は今回の主作業に含めない。

## 次にやること

- 差分を最終確認する。
- 実 token E2E はユーザー協働で実施し、token 実値や workspace 固有情報を書かない形で結果だけ記録する。

## 検証

- `docker compose run --rm --no-deps dev gofmt -w cmd/slapex/main.go cmd/slapex/main_test.go internal/slack/client.go`
- `docker compose run --rm --no-deps dev go test ./...`
- `docker compose run --rm --no-deps dev sh scripts/install_test.sh`
- `git diff --check`

## リスク・ブロッカー

- 実 token E2E は token 実値を扱うため、note や PR には結果の抽象化された要約だけを残す。

## セッションログ

- 2026-06-26: Issue #81 を確認。互換なしで `SLACK_TOKEN` に切り替えるユーザー方針を受け、作業ブランチを作成。
- 2026-06-26: `SLACK_TOKEN` 専用の CLI env 読み取りと未設定診断を実装し、`SLACK_BOT_TOKEN` fallback がないことをテストで固定。
- 2026-06-26: README / Slack App setup help / design docs / decision log を user token default、bot token support、v1.0.1 方針へ更新。
- 2026-06-26: PR #84 作成後、note を採番し、progress の PR 列を更新。
- 2026-06-26: ユーザー向け docs から旧 env 名の移行説明を削除し、`SLACK_TOKEN` だけを案内する表現へ調整。
