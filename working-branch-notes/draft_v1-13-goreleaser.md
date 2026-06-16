# 作業ブランチメモ

- ブランチ: `v1/13-goreleaser`
- PR: 未作成
- 最終更新: 2026-06-16

## 目的

Issue #27 の v1-13 として、GoReleaser による 4 target 単一バイナリ配布設定と `--version` の build-time 埋め込みを整備する。

## 現在の状況

- `main.version` を ldflags で上書きできる `var version = "dev"` に変更。
- `.goreleaser.yaml` を追加し、`darwin/linux` x `amd64/arm64`、`CGO_ENABLED=0`、binary archive、sha256 checksum を設定。
- GoReleaser image の Go patch version が `go.mod` より古い場合に備え、`GOTOOLCHAIN=auto` を設定。
- `.gitignore` に `/dist/` を追加。
- decision log 0034 に GoReleaser 設定追加を追記。
- 検証は完了。

## 決定事項

- GoReleaser 検証 image は公式 blog / docs で確認した `goreleaser/goreleaser:v2.16.0` を使用する。
- snapshot version は `{{ incpatch .Version }}-snapshot` とし、`--version` 検証で `dev` ではない値を確認する。

## 次にやること

- `progress.md` を PR 番号付きで更新する。
- commit / push / PR 作成を行う。

## 検証

- Docker:
  - `docker --version`: pass (`Docker version 29.5.3`)
  - `docker compose version`: pass (`Docker Compose version v5.1.4`)
  - `docker info`: pass
- `docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:v2.16.0 release --snapshot --clean`: pass。`dist/` に 4 target の binary と `slapex_checksums.txt` を生成。
- `find dist -maxdepth 2 -type f -print`: pass。生成確認対象は `slapex_linux_arm64_v8.0/slapex`、`slapex_linux_amd64_v1/slapex`、`slapex_darwin_arm64_v8.0/slapex`、`slapex_darwin_amd64_v1/slapex`、`slapex_checksums.txt`。
- `docker compose run --rm dev ./dist/slapex_linux_arm64_v8.0/slapex --version`: pass (`slapex 0.0.1-snapshot`)。
- `docker compose run --rm dev go test ./...`: pass。
- `docker compose run --rm dev gofmt -l .`: pass(出力なし)。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-06-16: Issue #27 と依存 `v1-01` done を確認し、`v1/13-goreleaser` ブランチを作成。実装に着手。
- 2026-06-16: `goreleaser/goreleaser:v2.16.0` の初回 snapshot build は image 内の Go が `go.mod` より古く失敗。`GOTOOLCHAIN=auto` を GoReleaser 設定に追加。
- 2026-06-16: GoReleaser snapshot build、dist 生成物、linux/arm64 binary の `--version`、`go test ./...`、`gofmt -l .` を検証済み。
