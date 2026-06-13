# 作業ブランチメモ

- ブランチ: v1/11-container-tz
- PR: #43
- 最終更新: 2026-06-13

## 目的

Issue #25 / v1-11 として、Docker Compose 経由の dev / E2E 実行時に host の `TZ` をコンテナへ forward できるようにし、コンテナ内 UTC による時刻表示・出力ディレクトリ名のずれを解消する。

## 現在の状況

- `compose.yaml` の `dev` service に `TZ: ${TZ:-}` を追加済み。
- guideline / decision log / index / `progress.md` を更新済み。
- CLI option は追加しない。配布バイナリを host で直接実行する本来の利用形態では host local timezone を使う既存仕様のままとする。
- Issue 指定の検証は完了済み。

## 決定事項

- `--tz` option は導入しない。
- dev / E2E のコンテナ実行では、Compose の environment と実行時 `-e TZ=...` で timezone を明示・引き継ぎできる形にする。

## 次にやること

- レビュー待ち。merge はユーザーが行う。

## 検証

- `docker compose run --rm dev date`: `Universal` 表示(UTC 相当)を確認。
- `docker compose run --rm -e TZ=Asia/Tokyo dev date`: `JST` 表示を確認。
- `env TZ=Asia/Tokyo docker compose run --rm dev date`: `JST` 表示を確認。通常 sandbox では Docker socket 権限で失敗したため、制約のない環境で再実行。
- `docker compose run --rm -e TZ=Asia/Tokyo dev go test ./...`: pass。
- `docker compose run --rm dev go test ./...`: pass。

## リスク・ブロッカー

- `git fetch origin` は 1Password SSH agent との通信に失敗したため、remote `main` の再取得は未完了。作業開始時点のローカル `main` と `origin/main` は同一 SHA。

## セッションログ

- 2026-06-13: Issue #25 を `github-op-integrated` MCP で確認。依存 v1-01 が `progress.md` で done であることを確認し、`v1/11-container-tz` ブランチを作成。
- 2026-06-13: PR #43 を作成し、note 採番と `progress.md` の PR 番号反映を実施。
