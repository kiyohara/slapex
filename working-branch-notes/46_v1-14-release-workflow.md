# 作業ブランチメモ

- ブランチ: `v1/14-release-workflow`
- PR: #46
- 最終更新: 2026-06-16

## 目的

Issue #28 / v1-14 として、`v*` tag push をトリガーに GoReleaser で GitHub Releases へ 4 target の単一バイナリと checksum を添付する workflow を整備する。

## 現在の状況

- `main` は `origin/main` と同一 commit であることを確認済み。
- `progress.md` で依存 v1-13 が done であることを確認済み。
- 作業ブランチ `v1/14-release-workflow` を作成済み。
- `.github/workflows/release.yml` を追加済み。
- `doc/design/architecture.md` の配布方式へ release workflow の要点を追記済み。
- Issue 指定の検証は完了済み。
- PR #46 を作成済み。
- `progress.md` の v1-14 行を done / PR #46 に更新済み。

## 決定事項

- 既存 CI に合わせて `actions/checkout@v4` と `actions/setup-go@v5` を使う。
- `.goreleaser.yaml` の config v2 に合わせ、`goreleaser/goreleaser-action@v6` の `version: "~> v2"` で GoReleaser v2 系を固定する。

## 次にやること

- レビュー待ち。merge はユーザーが行う。

## 検証

- pass: `docker compose run --rm dev go build ./...`
- pass: `docker compose run --rm dev gofmt -l .`(出力なし)
- pass: `ruby -e 'require "yaml"; YAML.load_file(ARGV.fetch(0)); puts "YAML OK"' .github/workflows/release.yml`
- workflow YAML は目視で Issue 指示との差分を確認済み。実 tag による動作確認は v1-17 のスコープ。

## リスク・ブロッカー

- 実 tag push での release workflow 実行は今回のスコープ外であり、PR 時点では未検証として明記する必要がある。

## セッションログ

- 2026-06-16: Issue #28 を MCP で取得。依存 v1-13 が done であることを `progress.md` で確認し、作業を開始した。
- 2026-06-16: PR #46 を作成し、note を番号付きファイル名へ rename した。
- 2026-06-16: `progress.md` の v1-14 行を done / PR #46 に更新した。
