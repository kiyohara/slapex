# 作業ブランチメモ

- ブランチ: `v1/06-test-channel-resolution`
- PR: #38
- 最終更新: 2026-06-13

## 目的

GitHub Issue #20 に従い、`internal/export` の channel 解決ロジックを仕様分岐どおりにユニットテストで固定する。

## 現在の状況

- `main` から `v1/06-test-channel-resolution` を作成。
- `progress.md` で依存 v1-01 が done であることを確認済み。
- `chooseChannel` と `channelLine` のテストを追加済み。
- Issue 指定の検証は pass。
- PR review comment 3 件に対応済み。

## 決定事項

- TTY interactive selection 自体は Issue のスコープ外のため、自動テストしない。
- channel 未指定かつ non-TTY の場合は、候補が 1 件でも自動確定せず usage エラーにする。`usage-flow.md` の「channel 引数が未指定の場合も選択を求める」「TTY がない場合は interactive selection を開始しない」に基づく挙動修正。

## 次にやること

- レビュー待ち。

## 検証

- `docker compose run --rm dev go test ./internal/export/... -v` pass。
- `docker compose run --rm dev gofmt -l .` pass(出力なし)。
- `docker compose run --rm dev go vet ./...` pass。

## リスク・ブロッカー

- 現時点ではなし。

## セッションログ

- 2026-06-13: Issue #20 を GitHub MCP で確認し、依存 v1-01 done を `progress.md` で確認。作業ブランチを作成。
- 2026-06-13: `chooseChannel` / `channelLine` のユニットテストを追加。channel 未指定 + non-TTY で候補 1 件を自動確定していた仕様不整合を修正し、Issue 指定の検証を実行して pass。
- 2026-06-13: PR #38 作成後、note を `working-branch-notes/38_v1-06-test-channel-resolution.md` へ rename。
- 2026-06-13: PR review comment 3 件に対応。空 keyword の候補表示文言を専用化し、空 keyword + 候補 1 件の regression test を追加し、不要な `sprintf` test helper を削除。
