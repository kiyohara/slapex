# 実装アーキテクチャ

このファイルには、`slapex` の実装言語、依存方針、主要ライブラリ、内部構成、開発環境、配布方式をまとめる。

想定読者は、実装・検証する担当者である。

本ファイルの内容は確定仕様として扱う。決定経緯は `decision-log/0032-implementation-language.md` / `decision-log/0033-go-dependency-policy.md` / `decision-log/0034-distribution-method.md` を参照する。選定アーキテクチャの機能充足性は PoC で確認する(状況は `progress.md`)。

## 実装言語

Go(執筆時点の安定版は 1.26 系)を採用する。

採用理由の要点(比較の全体は decision log 0032):

- 単一バイナリ配布(最重要基準)とクロスコンパイル(`GOOS` / `GOARCH`、`CGO_ENABLED=0`)が言語標準機能で完結する。
- 標準ライブラリだけで HTTP client、JSON、HTML テンプレート(contextual auto-escaping)、CLI flag、ファイル埋め込みを賄え、外部依存を最小化できる。
- HTML の自動エスケープが `html-rendering.md` のサニタイズ方針(全テキストエスケープ + 自前マークアップのみ)とそのまま一致する。
- リリース自動化(GitHub Releases への multi-platform バイナリ添付)の実績が厚い。

## 依存方針

- 標準ライブラリを第一候補とし、外部依存は「標準ライブラリでの実現が著しく困難な領域」に限定する。サプライチェーンリスクの最小化は開発基盤方針(`decision-log/0002-docker-compose-baseline.md`)と同根の価値判断である。
- `golang.org/x/*` は Go チーム管理の準標準ライブラリとして利用してよい。
- 依存を追加・変更するときは decision log に理由を記録する。

## 主要コンポーネントとライブラリ

| 領域 | 採用 | 補足 |
|---|---|---|
| CLI 引数 parse | 標準 `flag` | subcommand なし(`decision-log/0006-no-subcommands-initially.md`)。help 表示の GNU 風整形が必要になったら再検討 |
| Slack API client | 自前 thin client(`net/http` + `encoding/json`) | 使用 method は 7 種のみ(`slack-api-usage.md`)。429 / `Retry-After` と指数バックオフは自前の HTTP wrapper で実装(`decision-log/0025-slack-api-usage-policy.md`) |
| TTY interactive selection | `charm.land/huh/v2`(charmbracelet 製) | カーソル上下 + Enter の選択 UI(`usage-flow.md`)。MIT license。v2 の module path は `charm.land` 配下 |
| TTY 判定 | `golang.org/x/term` | stdin / stderr の TTY 判定(`--no-interactive` 制御、`usage-flow.md`、`decision-log/0043-interactive-selection-streams.md`) |
| label の Unicode 正規化 | `golang.org/x/text`(`unicode/norm`) | directory label の NFC 正規化(`decision-log/0029-directory-label-rules.md`) |
| HTML 生成 | 標準 `html/template` | contextual auto-escaping が `decision-log/0026-mrkdwn-html-conversion.md` のエスケープ方針と一致 |
| 標準絵文字データ | `go:embed` で組込み | shortcode → Unicode 対応表を vendored JSON として同梱(`slack-api-usage.md`)。生成は `tools/genemoji`(取得元: iamcal/emoji-data) |
| サイズ・時刻等の整形 | 標準ライブラリ | `--max-attachment-size` の単位 parse は自前実装(`cli-interface.md` の書式) |

## 内部構成(パッケージ構成の目安)

```text
cmd/slapex/        エントリポイント、flag parse、exit code 制御
internal/slack/    thin API client、rate limit / retry、API 型定義
internal/export/   取得の orchestration(channel 解決、history / replies、assets)
internal/render/   mrkdwn 変換、HTML テンプレート、style.css
internal/output/   出力ディレクトリ、label slug、.cache 書き出し
internal/emoji/    絵文字解決(組込みデータセット + emoji.list)
```

この構成は実装開始時の基準であり、PoC・実装の進行に応じて調整してよい。大きく変える場合は decision log に記録する。

## 開発環境

- 開発コマンド(build / run / 依存取得)は Docker Compose 経由を原則とする(`decision-log/0002-docker-compose-baseline.md`、`doc/guidelines/development-command-guidelines.md`)。
- `golang` 公式 image を base にした dev service を定義する。Compose 構成は PoC ブランチで追加し、確定後に `development-command-guidelines.md` の具体コマンド例を更新する。

## 配布方式

- GitHub Releases に各 OS / arch の単一バイナリを添付する。build target は `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`(`decision-log/0031-supported-platforms.md`)。
- リリース自動化は goreleaser を想定する。checksum ファイルを同梱する。
- リリース時は `doc/guidelines/git-operation-guidelines.md` に従って署名付き tag `vX.Y.Z` を `git tag -s` で作成し、GitHub remote へ push する。
- `v*` tag の push をトリガーに release workflow が goreleaser を実行し、GitHub Releases へ各 target のバイナリと checksum を添付する。
- 導入補助として、OS / arch を自動判定し checksum を検証する install script(`scripts/install.sh`、`curl | sh`)を提供する(`decision-log/0041-install-convenience.md`)。
- macOS 向け Homebrew tap は専用 tap repo（`kiyohara/homebrew-tap`）に cask として提供する(`decision-log/0034-distribution-method.md` / `decision-log/0041-install-convenience.md`)。
- リリース workflow と tap の整備は PoC 後の実装フェーズで行う(`decision-log/0034-distribution-method.md`)。

## 参考

- Go release history: <https://go.dev/doc/devel/release>
- charmbracelet/huh: <https://github.com/charmbracelet/huh>
- goreleaser: <https://goreleaser.com/>
