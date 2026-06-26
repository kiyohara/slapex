# CLI インターフェース仕様

このファイルには、`slapex` CLI のコマンド形式、環境変数、option、入出力ストリーム、exit code、対象プラットフォームの仕様をまとめる。

想定読者は、このツールを利用する人間と、CLI を実装・検証する担当者である。

本ファイルの option 名、default 値、exit code は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

利用者の操作の流れは `usage-flow.md`、取得範囲と出力構造は `output-format.md`、Slack API の利用方針は `slack-api-usage.md` を参照する。決定経緯は `decision-log/0024-cli-options-and-exit-codes.md`、`decision-log/0031-supported-platforms.md`、`decision-log/0042-default-user-token.md` を参照する。

## コマンド形式

```sh
slapex [channel] [options]
```

- subcommand は採用しない(`decision-log/0006-no-subcommands-initially.md`)。
- `[channel]` は optional positional argument。channel 名、channel ID、または名前の一部を受け取る。解決の流れは `usage-flow.md` の「channel の指定と選択」を参照する。

## 環境変数

| 変数 | 必須 | 用途 |
|---|---|---|
| `SLACK_TOKEN` | 必須 | Slack OAuth token。デフォルト利用方法は user token(`xoxp-`)とし、CI / automation では bot token(`xoxb-`)も正式サポートする |

`SLACK_BOT_TOKEN` は旧 bot token 前提設計で使っていた環境変数名であり、現在は参照しない。bot token を使う場合も `SLACK_TOKEN` に渡す。

token を CLI option や引数として受け取る経路は提供しない。プロセス一覧や shell history への漏えいを避けるため、受け渡しは環境変数だけにする。

## option 一覧

| option | 値 | default | 制約 | 目的 |
|---|---|---:|---|---|
| `--output <path>` | path | 実行時刻から生成 | | 出力 root を指定する(省略時の動作は `output-format.md`) |
| `--max-posts <count>` | 整数 | `1000` | `1`〜`10000` | timeline 上の親投稿の最大取得件数(`output-format.md`) |
| `--days <days>` | 整数 | `30` | `1`〜`90` | 現在時刻から何日前までの投稿を取得するか(`output-format.md`) |
| `--max-attachment-size <size>` | サイズ | `10MB` | `1KB` 以上 | 添付ファイル / original 画像 1 件あたりの保存上限(`output-format.md`) |
| `--keep-cache` | flag | off | | `.cache/` を成否に関係なく残す(`cache.md`) |
| `--reuse-cache <path>` | path | なし | | 以前の `.cache/` を再利用する(`cache.md`) |
| `--no-interactive` | flag | off | | TTY があっても interactive selection を開始しない(`usage-flow.md`) |
| `--version` | flag | | | version を表示して終了する |
| `--help` | flag | | | usage を表示して終了する |

`<size>` の書式は、単位なしの整数(バイト)または `KB` / `MB` / `GB` の単位付き整数とする(例: `10485760`、`10MB`、`512KB`)。単位は 1024 基数で解釈する。

制約を外れた値、未知の option、不正な書式は usage を表示して exit code `2` で終了する。

### 将来検討とする option

以下は初期実装では採用せず、必要になった時点で再検討する。

- `--quiet` / `--verbose` / `--no-color` などの出力制御。
- `--expect-team-id` / `--expect-workspace-domain` などの workspace guard(`decision-log/0020-target-label-display.md`)。
- 差分取得・再実行に関わる option(`decision-log/index.md` の未決事項)。
- `--no-cache`(採用しない理由は `cache.md`)。

## 入出力ストリーム

| stream | 内容 |
|---|---|
| stdout | 機械処理しやすい最終結果だけを出力する。成功時に出力ディレクトリ(`<workspace-label>/<channel-label>/` まで)の絶対 path を 1 行出力する |
| stderr | 進捗、診断、workspace / channel label、候補 list、interactive prompt、エラー、完了 summary |

この分離により、script や CI では `out=$(slapex ...)` の形で出力先 path を後続処理に渡せる。進捗や label の表示内容は `usage-flow.md` の「処理対象の表示」を参照する。

色付けやカーソル制御は stderr が TTY の場合だけ使う。non-TTY ではプレーンテキストを出力する。

## exit code

| code | 意味 | 例 |
|---:|---|---|
| `0` | 成功 | export が完了し、HTML と assets を書き込んだ |
| `1` | その他の想定外の失敗 | 内部エラー、分類できない異常 |
| `2` | 引数・指定の誤り、対象を確定できない | 不正な option、channel 候補が 11 件以上、該当 channel なし、non-TTY または `--no-interactive` で選択が必要になった |
| `3` | 認証・権限の問題 | Slack token 未設定・無効、scope 不足、bot token 利用時に bot が対象 channel に未参加、user token 利用時にユーザーが対象 channel を参照できない |
| `4` | 取得・保存の実行時失敗 | リトライ上限到達、ネットワーク断、出力先への書き込み失敗 |

部分失敗の扱い: 個別 asset(添付ファイル、絵文字画像など)の取得失敗は exit code `4` にせず、HTML 上の置換表示と `.cache/assets_manifest.json` への記録で export を継続する(`output-format.md`)。メッセージ本文の取得が完了できない場合は exit code `4` で失敗とする。

## 対象プラットフォーム

- 対象: macOS、Linux(CI は GitHub Actions の Linux runner を想定)。
- Windows は初期対象外とする。directory label の正規化(`output-format.md`)は将来の Windows 対応を妨げない範囲で設計するが、動作保証はしない(`decision-log/0031-supported-platforms.md`)。
