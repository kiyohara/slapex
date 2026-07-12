# CLI インターフェース仕様

このファイルには、`slapex` CLI のコマンド形式、環境変数、option、入出力ストリーム、exit code、対象プラットフォームの仕様をまとめる。

想定読者は、このツールを利用する人間と、CLI を実装・検証する担当者である。

本ファイルの option 名、default 値、exit code は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

利用者の操作の流れは `usage-flow.md`、取得範囲と出力構造は `output-format.md`、Slack API の利用方針は `slack-api-usage.md` を参照する。決定経緯は `decision-log/0024-cli-options-and-exit-codes.md`、`decision-log/0031-supported-platforms.md`、`decision-log/0042-default-user-token.md`、`decision-log/0043-interactive-selection-streams.md`、`decision-log/0044-interactive-token-prompt.md`、`decision-log/0045-cli-output-style.md`、`decision-log/0046-api-base-url-override.md` を参照する。

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

token を CLI option や引数として受け取る経路は提供しない。プロセス一覧や shell history への漏えいを避けるため、受け渡しは環境変数だけにする。

このほかに内部用途の環境変数として `SLAPEX_API_BASE_URL` がある。非空のときだけ Slack Web API の接続先 base URL を差し替える(未設定時は `https://slack.com/api/` のまま)。デモ録画・ローカル fixture 実行(`tools/gensample -serve` / `tools/demo/`)のためのもので、利用者向けドキュメントや `--help` には載せない。接続先の上書きは token の送信先が変わることを意味するため、変更時は `doc/guidelines/credential-scope-guidelines.md` の checklist(positive / negative test)に従う(`decision-log/0046-api-base-url-override.md`)。

`SLACK_TOKEN` が未設定の場合、controlling terminal (`/dev/tty`) を開けて `--no-interactive` が指定されていないときに限り、token を対話入力するプロンプトを `/dev/tty` に表示する。入力は echo せず、値はそのプロセス内でだけ使い、設定ファイル・cache・log・HTML 出力には保存しない。これは secret manager をまだ用意していない個人評価・PoC 利用者が、token を shell history に残さず一時的に渡すための導線である(`decision-log/0044-interactive-token-prompt.md`)。この対話入力は token を CLI option / 引数で受け取る経路ではなく、環境変数以外の保存経路も追加しない。controlling terminal が無い環境(CI・pipe 実行など)や `--no-interactive` 指定時は、対話入力を行わず、従来どおり未設定エラー(exit code `3`)と案内を表示して終了する。

## option 一覧

| option | 値 | default | 制約 | 目的 |
|---|---|---:|---|---|
| `--output <path>` | path | 実行時刻から生成 | | 出力 root を指定する(省略時の動作は `output-format.md`) |
| `--max-posts <count>` | 整数 | `1000` | `1`〜`10000` | timeline 上の親投稿の最大取得件数(`output-format.md`) |
| `--date <date-time>` | 日付または日時 | なし | 下記の明示形式 | 入力が属する local calendar date の timeline 投稿だけを取得する(`output-format.md`) |
| `--from <date-time>` | 日付または日時 | なし | `--to` とペア、下記の明示形式 | 任意期間の開始境界を指定する(境界を含む) |
| `--to <date-time>` | 日付または日時 | なし | `--from` とペア、下記の明示形式 | 任意期間の終了境界を指定する(境界を含まない) |
| `--days <days>` | 整数 | `30` | `1`〜`90` | 現在時刻から何日前までの投稿を取得するか(`output-format.md`) |
| `--max-attachment-size <size>` | サイズ | `10MB` | `1KB` 以上 | 添付ファイル / original 画像 1 件あたりの保存上限(`output-format.md`) |
| `--keep-cache` | flag | off | | `.cache/` を成否に関係なく残す(`cache.md`) |
| `--reuse-cache <path>` | path | なし | | 以前の出力ディレクトリまたは `.cache/` を再利用する(`cache.md`) |
| `--no-interactive` | flag | off | | TTY があっても interactive prompt を開始しない(channel selection と、`SLACK_TOKEN` 未設定時の token 入力の両方が対象)(`usage-flow.md`) |
| `--no-color` | flag | off | | stderr の進捗・診断を plain output にする(色に加えて、アイコン・spinner などの装飾全体を抑止する。「出力制御」を参照) |
| `--demo` | flag | off | | token / Slack App なしで同梱の架空サンプルを export する(「demo モード」を参照) |
| `--version` | flag | | | version を表示して終了する |
| `--help` | flag | | | usage を表示して終了する |

`<size>` の書式は、単位なしの整数(バイト)または `KB` / `MB` / `GB` の単位付き整数とする(例: `10485760`、`10MB`、`512KB`)。単位は 1024 基数で解釈する。

取得範囲 option は、利用者への案内と文書内の記載順を `--date`、`--from` / `--to`、`--days` の優先順とする。特定日には `--date`、任意期間には `--from` / `--to`、実行時刻基準の相対範囲にだけ `--days` を使う。

制約を外れた値、未知の option、不正な書式は usage を表示して exit code `2` で終了する。

`--from` と `--to` は必ずペアで指定し、開始は終了より前にする。範囲は開始を含み終了を含まない半開区間 `[from, to)` とする。`--from` / `--to` は `--date` および利用者が明示した `--days` と併用できない。`--date` または `--from` / `--to` だけを指定した場合、既定値 `--days 30` は適用しない。

`--date` / `--from` / `--to` は RFC3339 / RFC3339Nano、または local timezone として解釈する次の形式を受け入れる。日付区切りは `-` / `/`、日付と時刻の区切りは `T` / 半角スペースを許可し、時刻は `HH` / `HH:MM` / `HH:MM:SS` の不足部分を `0` で補う。日付だけの場合も時刻を `00:00:00` とする。許可 layout は実装で明示的に列挙し、timezone abbreviation、自然言語、日本語日付、年を省略した形式は受け入れない。

offset なしの入力は local timezone として parse する。offset 付き入力は、その offset が表す絶対時刻として扱う。`--date` は parse した瞬間が属する local calendar date の 00:00 以上、翌日 00:00 未満を取得範囲とし、入力の時刻部分は対象日の決定にだけ使う。`--from` / `--to` は parse 後の瞬間をそのまま開始・終了境界に使う。たとえば `--from 2026/07/03 --to 2026/07/04` は local timezone の 2026-07-03 全体を表す。

### 将来検討とする option

以下は初期実装では採用せず、必要になった時点で再検討する。

- `--quiet` / `--verbose` などの出力量の制御(`--no-color` は Issue #100 で正式 option 化した)。
- `--expect-team-id` / `--expect-workspace-domain` などの workspace guard(`decision-log/0020-target-label-display.md`)。
- 差分取得・再実行に関わる option(`decision-log/index.md` の未決事項)。
- `--no-cache`(採用しない理由は `cache.md`)。

## demo モード(token 不要試用)

`--demo` を指定すると、Slack App の作成や token 発行なしで、同梱の架空サンプルデータから HTML export を生成する(Issue #113、`decision-log/0047-token-free-demo-run.md`)。Slack App 準備前に、手元で実際の CLI を実行して成果物を確認するための導線である。

- `SLACK_TOKEN` は不要とする。設定されていても demo モードでは使わない。
- 実行時に in-process の fake Slack API server を起動し、内部専用の fake token でその server にだけ接続する。実 Slack host への通信や実 token の送信は行わない(`doc/guidelines/credential-scope-guidelines.md`)。接続先の指定は CLI 内部で直接行い、公開環境変数を経由しない。
- サンプルデータは架空の workspace / channel / user / asset で構成し、実 workspace 名・個人名・実 token を含めない(#51 と同じ匿名化方針)。ja / en の 2 シナリオを同梱し、locale(`LC_ALL` → `LC_MESSAGES` → `LANG` の順に最初の非空値)が `ja` で始まる場合は日本語シナリオ、それ以外は英語シナリオを使う。
- 対象 channel は 1 つに固定されるため channel selection は行わない(non-interactive で自動解決)。positional な `[channel]` 引数を渡しても demo では無視する。
- 出力先(`--output`)、`--no-color`、取得範囲 option(`--max-posts` / `--date` / `--from` / `--to` / `--days` / `--max-attachment-size` など)は通常実行と同じく尊重する。stdout の契約(成功時に出力ディレクトリ path を 1 行)も通常実行と同じで、token 不要の案内は stderr に出す。
- fixture は in-process 配信で実際の rate limit が無いため、通常実行が行う Slack API pacing は demo では省略し、待ち時間を入れない。
- デモ録画(`tools/demo/`)が使う内部環境変数 `SLAPEX_API_BASE_URL`(`decision-log/0046-api-base-url-override.md`)とは別経路である。録画は token 入力プロンプトを見せる目的でその機構を引き続き使い、demo モードは利用者向けの token 不要経路として別に提供する。

## 入出力ストリーム

| stream | 内容 |
|---|---|
| stdout | 機械処理しやすい最終結果だけを出力する。成功時に出力ディレクトリ(`<workspace-label>/<channel-label>/` まで)の絶対 path を 1 行出力する |
| stderr | 進捗、診断、workspace / channel label、候補 list、エラー、完了 summary |
| /dev/tty | interactive selection の prompt と、`SLACK_TOKEN` 未設定時の token 入力 prompt(controlling terminal がある場合のみ)。stdin / stdout / stderr の redirect や wrap から独立させるため。token 入力は echo しない |

この分離により、script や CI では `out=$(slapex ...)` の形で出力先 path を後続処理に渡せる。進捗や label の表示内容は `usage-flow.md` の「処理対象の表示」を参照する。

interactive selection と token prompt は controlling terminal (`/dev/tty`) を開ける場合だけ開始し、prompt の入出力も `/dev/tty` に固定する。stdin / stdout / stderr の TTY 状態はこの prompt 可否判定に使わない。これは stdout の機械可読契約を維持しつつ、1Password CLI (`op run`) の既定 secret masking で stdout / stderr が pipe 化される実行経路でも prompt を出せるようにするためである。

## 出力制御

stderr の進捗・診断表示には styled / plain の 2 モードがあり、既定は自動判定(`auto`)とする(Issue #100、`decision-log/0045-cli-output-style.md`)。判定は実際の出力先である stderr を基準にし、stdout の状態や interactive prompt の可否判定(`/dev/tty`)とは独立させる。

次のいずれかに該当する場合は plain output に倒し、それ以外で stderr が TTY の場合だけ styled output を使う。

- `--no-color` が指定されている。
- 環境変数 `NO_COLOR` が空でない値に設定されている。
- `TERM=dumb` が設定されている。
- 環境変数 `CI` が空でない値に設定されている(主要 CI サービスは `CI=true` を設定するが、それ以外の truthy 値を設定する環境もあるため、値は問わない)。
- stderr が TTY ではない(pipe / redirect / non-TTY 実行)。

### styled output(TTY)

- 処理の各段階をフェーズ行(状態記号 + ラベル列 + 本文)で表示する。状態記号は `✓`(成功、green)/ `!`(警告、yellow)/ `✗`(エラー、red)を使う。
- 色は基本 ANSI 8 色と bold / dim だけを使い、状態記号の着色・ラベルの bold・補足メタ情報の dim にとどめる(値そのものは端末デフォルト色)。256 色 / TrueColor は使わない。
- 長く待つ可能性がある処理(Slack API rate limit 待機、履歴取得、asset download など)に限り、braille spinner(`⠋⠙⠹…`)で進行中のフェーズ行を上書き更新する。カーソル制御は「行頭復帰 + 行クリア」だけを使う。
- 確定した情報は通常の行として積み、live 更新は進行中の 1 行に限定する。

### plain output

- ANSI escape sequence、spinner、CR による行上書き、装飾専用の記号(`✓` などの Unicode 記号)を一切出さない。
- 1 イベント 1 行の追記のみとし、状態は ASCII prefix(`OK:` / `WARN:` / `ERROR:` / `INFO:`)で明示する。
- 開始・待機・完了はそれぞれ独立した行として出力し、ログ収集環境で安定して読めるようにする。

いずれのモードでも、token 実値、cookie、Authorization header、Slack private file URL などの機密情報は出力しない(`doc/guidelines/credential-scope-guidelines.md`)。

## exit code

| code | 意味 | 例 |
|---:|---|---|
| `0` | 成功 | export が完了し、HTML と assets を書き込んだ |
| `1` | その他の想定外の失敗 | 内部エラー、分類できない異常 |
| `2` | 引数・指定の誤り、対象を確定できない | 不正な option、存在しない日時、取得範囲 option の不正な組み合わせ、`from >= to`、channel 候補が 11 件以上、該当 channel なし、non-TTY または `--no-interactive` で選択が必要になった |
| `3` | 認証・権限の問題 | Slack token 未設定・無効、scope 不足、bot token 利用時に bot が対象 channel に未参加、user token 利用時にユーザーが対象 channel を参照できない |
| `4` | 取得・保存の実行時失敗 | リトライ上限到達、ネットワーク断、出力先への書き込み失敗 |

部分失敗の扱い: 個別 asset(添付ファイル、絵文字画像など)の取得失敗は exit code `4` にせず、HTML 上の置換表示と `.cache/assets_manifest.json` への記録で export を継続する(`output-format.md`)。メッセージ本文の取得が完了できない場合は exit code `4` で失敗とする。

## 対象プラットフォーム

- 対象: macOS、Linux(CI は GitHub Actions の Linux runner を想定)。
- Windows は初期対象外とする。directory label の正規化(`output-format.md`)は将来の Windows 対応を妨げない範囲で設計するが、動作保証はしない(`decision-log/0031-supported-platforms.md`)。
