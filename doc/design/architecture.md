# 実装アーキテクチャ

このファイルは、実装・検証する担当者に向けて、採用方針と現行の実装構成をまとめる。CLI / cache / HTML の振る舞いは各 spec を正本とする。

言語・依存・配布の選定経緯は [0032](decision-log/0032-implementation-language.md) / [0033](decision-log/0033-go-dependency-policy.md) / [0034](decision-log/0034-distribution-method.md) を参照する。PoC の確認と初版の実装・配布は完了済みであり、以降のリリース履歴と進行中タスクは [progress.md](../../progress.md) に置く。

## 実装言語と依存方針

Go を採用する。必要な Go version と直接・間接依存の version は [go.mod](../../go.mod) を正本とする。

- 単一バイナリ配布とクロスコンパイルを重視し、標準ライブラリを第一候補とする。
- 外部依存は標準ライブラリでの実現が著しく困難な領域に限定する。`golang.org/x/*` は Go チーム管理の準標準ライブラリとして利用してよい。
- 依存を追加・変更するときは decision log に理由を記録する。

## 主要コンポーネントとライブラリ

| 領域 | 採用 | 補足 |
|---|---|---|
| CLI 引数 parse | 標準 `flag` | subcommand なし。option と help の仕様は [cli-interface.md](cli-interface.md) |
| Slack API client | 自前 thin client(`net/http` + `encoding/json`) | method 一覧・pagination・retry 方針は [slack-api-usage.md](slack-api-usage.md#使用する-api) を正本とする。JSON API と streaming download の retry は `internal/slack` に実装 |
| TTY interactive selection | `charm.land/huh/v2` | channel 選択 UI。操作と stream の仕様は [usage-flow.md](usage-flow.md) |
| TTY 判定・token 入力 | `golang.org/x/term` | 対話可否・styled 出力の判定と非表示入力 |
| label の Unicode 正規化 | `golang.org/x/text/unicode/norm` | directory label の NFC 正規化。[label の方針](decision-log/0029-directory-label-rules.md) |
| HTML 生成 | 標準 `html/template` | contextual auto-escaping。[html-rendering.md](html-rendering.md) のサニタイズ方針に従う |
| 標準絵文字データ | `go:embed` | shortcode → Unicode 対応表を JSON として同梱。生成入口は `tools/genemoji` |
| サイズ・時刻等の整形 | 標準ライブラリ | サイズの単位 parse と日時 parse は自前実装。入力書式は [cli-interface.md](cli-interface.md) |

## 現行の内部構成

表の依存先は production code の repository 内 package import を示す。標準・外部ライブラリと test だけの依存は省略する。

| package / 入口 | 責務 | 直接依存する内部 package |
|---|---|---|
| [cmd/slapex](../../cmd/slapex/main.go) | flag parse・入力検証、token 入力、通常/demo の起動、stdout の結果 path と exit code 制御 | datetime、demo、emoji、export、slack、ui |
| [internal/export](../../internal/export/export.go) | `Run` による workspace/channel 解決、対話選択、取得範囲・filter、history/replies、user/bot/emoji 解決、asset 保存、表示用データ組立、cache の組立・再利用・cleanup の判定 | datetime、emoji、output、render、slack、ui |
| [internal/slack](../../internal/slack/client.go) | API 型と thin client、pagination、method ごとの平準化、retry、認証送信先を制限した download | なし |
| [internal/output](../../internal/output/output.go) | 出力 root・label、asset 保存・内容 hash/extension 決定・再利用コピー、manifest entry、JSON 書き出し、`.cache/` の削除 | slack |
| [internal/render](../../internal/render/html.go) | 表示用データ型、mrkdwn 変換、HTML template、埋込み CSS/logo の書き出し | なし |
| [internal/emoji](../../internal/emoji/emoji.go) | 埋込み標準絵文字・渡された custom emoji map の解決、alias/skin tone 処理、除外名の正規化・照合 | なし |
| [internal/datetime](../../internal/datetime/parse.go) | CLI の日時書式と timezone に基づく parse | なし |
| [internal/ui](../../internal/ui/ui.go) | styled/plain 判定、進捗 phase・spinner・通知の出力 | なし |
| [internal/demo](../../internal/demo/export.go) | 架空 scenario と local fake Slack server、通常の `export.Run` を使う demo/sample 共通 driver | export、slack、ui |

通常実行は `cmd/slapex` が `slack.Client` と `ui.Printer` を用意して `export.Run` を呼ぶ。`--demo` は `demo.Export` を介して同じ工程を実行する。`export` が取得結果を `render` の表示用データへ変換し、asset の保存は `output` に委譲する。`emoji.list` の取得・cache 再利用は `export` と `slack` の責務であり、`emoji` 自体は API を呼ばない。

cache の schema に沿った object の組立は `export`、JSON の書き出しと asset manifest entry は `output` に分かれる。再利用の読込・検証は [export/reuse.go](../../internal/export/reuse.go)、保存済み asset のコピーは `output` が担う。確認済みの仕様差は [cache.md](cache.md#確認済みの仕様と実装の差) を参照する。

生成用入口は [tools/genemoji](../../tools/genemoji/main.go)(標準絵文字データ)、[tools/gensample](../../tools/gensample/main.go)(`demo` / `ui` を使う ja/en sample export)、[tools/genscreenshot](../../tools/genscreenshot/main.go)(同梱 sample の screenshot)、[tools/demo](../../tools/demo/record.sh)(terminal demo GIF)に置く。

将来の構成変更は [段階的リファクタリングの方針](decision-log/0056-incremental-refactoring-plan.md) と各 Issue で扱う。上表はその計画を先取りせず、構成変更を行う PR で該当箇所を同期する。

## 開発環境

- [compose.yaml](../../compose.yaml) の `dev` は `golang` 公式 image を使い、repository を mount し、Go module/build cache を named volume に保持する。`CGO_ENABLED=0` と host の `TZ` を環境変数に設定する。`vhs` と `screenshot` は生成物の録画・撮影用 service である。
- build / run / 依存取得は Docker Compose 経由を原則とする。実行方法と sample 更新の入口は [開発コマンド実行ルール](../guidelines/development-command-guidelines.md) を参照する。
- [CI workflow](../../.github/workflows/ci.yml) は PR と main への push で gofmt / vet / build / test、および配布対象の cross compile を検証する。Go version は `go.mod` を参照する。

## 配布方式

- [release workflow](../../.github/workflows/release.yml) は `v[0-9]*` tag の push で GoReleaser を実行する。
- [.goreleaser.yaml](../../.goreleaser.yaml) が `darwin` / `linux` × `amd64` / `arm64` の単一バイナリを `CGO_ENABLED=0` で build し、version を埋め込む。GitHub Releases にはバイナリと SHA-256 checksum ファイルを添付し、macOS 用 cask を専用 tap repository `kiyohara/homebrew-tap` へ更新する構成である。
- [install script](../../scripts/install.sh) は OS/arch 判定と checksum 検証を行う導入補助である。利用者向け導入手順は [README](../../README.md) と [help](../help/README.md) に置く。
- リリース作業の入口は [開発ループ](../guidelines/development-loop.md)、署名付き tag の操作規則は [Git 操作ルール](../guidelines/git-operation-guidelines.md) を参照する。
