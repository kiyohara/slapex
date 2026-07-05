# よくある質問・制限事項

このページは、`slapex` の取得範囲や現状の制限、初めての export でつまずきやすい点を Q&A 形式でまとめたものです。初回 export の前に目を通すと、「取得されない範囲」や「再現されない表示」を事前に把握できます。

各項目は要点と対処だけを載せます。仕様の正本(開発者向けドキュメント)への参照が必要な項目には、文末の脚注へのリンクを付けています。

## 取得範囲

### 一度にどれくらいの範囲が取得されますか

default では、直近 30 日以内かつ timeline 上の親投稿 1000 件までを取得します。2 つの条件は AND で結合し、両方を満たす投稿だけが対象です。

- 期間を変えるには `--days`(1〜90)、件数を変えるには `--max-posts`(1〜10000)を指定します。
- ここでの「親投稿」は channel の timeline に現れるメッセージで、スレッド内の返信は件数に含めません。対象になった親投稿の返信は一緒に取得されます。[^spec-range]

### スレッドの返信はすべて取得されますか

1 つのスレッドの返信が 1000 件を超える場合、それ以降の返信は取得を打ち切り、HTML 上では「取り扱える件数の上限に達しました。」という置換表示になります。[^spec-range]

## ファイル・画像

### 大きい添付ファイルや画像は保存されますか

添付ファイルとアップロード画像の original は、1 件あたり default で 10MB(`--max-attachment-size` で変更可)を超えると保存しません。

- 画像以外の添付ファイルが上限を超えた場合、HTML 上では「サイズオーバーのため保存されませんでした。」という置換表示になります。
- アップロード画像の original が上限を超えた場合は、thumbnail 表示を残したうえで original が保存されなかったことを示します。
- URL preview 画像、preview の service icon、workspace icon は第三者 host 由来の public asset のため、`--max-attachment-size` とは別に 1 件あたり 5MiB の上限があり、超えると表示されません。[^spec-size]

## 表示・レンダリング

### Slack の blocks レイアウトはそのまま再現されますか

いいえ。凝った block レイアウト(`blocks` の `rich_text` など)を構造そのままに再現することは初期対象外です。本文は Slack が保持する `text` フィールド(fallback テキストを含む)を正として描画します。多くの投稿は問題なく読めますが、block で組まれた特殊なレイアウトは簡略化されて表示される場合があります。[^spec-render]

### URL preview(unfurl)はどこまで再現されますか

Slack 上で preview として表示されていた画像、service 名、service icon、タイトル・本文テキストの範囲を再現します。色付き枠やフィールド構造の完全な模倣は初期対象外です。また、ツール自身が外部サイトへ favicon や Open Graph 情報を取りに行くことはしません(Slack API が返す情報だけを使います)。[^spec-render]

## 対応環境・token

### Enterprise Grid / org-wide install に対応していますか

いいえ。Enterprise Grid の org-wide install(1 つの token が複数 workspace を表す形態)は初期対象外です。単一 workspace に install した App の token を使ってください。手順の前提は [Slack App 準備手順](slack-app-setup.md#前提) を参照してください。

### Windows で動きますか

いいえ。対象プラットフォームは macOS と Linux(それぞれ amd64 / arm64)で、Windows は初期対象外です。[^spec-platform]

### token をコマンド引数で渡せますか

いいえ。プロセス一覧や shell history への漏えいを避けるため、token は環境変数 `SLACK_TOKEN` からのみ受け取ります。`SLACK_TOKEN` が未設定で、操作可能な terminal がある場合は対話入力もできます(入力値は画面に表示されず、どこにも保存されません)。

token の安全な渡し方は [Token の渡し方](token-injection.md) を参照してください。[^spec-token]

## その他

### 実行中に Slack 側で投稿が更新されたらどうなりますか

export は単発実行で、取得開始時点のスナップショットとして扱います。実行中に Slack 側で起きた更新との完全な整合は保証しません。[^spec-consistency]

## うまくいかないとき

初回 export でつまずきやすい典型例と対処です。

| 症状 | 主な原因 | 対処 |
|---|---|---|
| `SLACK_TOKEN is not set` などで終了する | token 未設定、または interactive prompt が使えない環境 | [Token の渡し方](token-injection.md) を確認し、実行環境に合う方法で `SLACK_TOKEN` を渡します |
| token が無効、または権限不足で終了する | token の保存値が古い、scope 不足、App の再 install 未実施 | token の保存値は [token を更新したとき](token-injection.md#token-を更新したとき)、scope と再 install は [Slack App 準備手順](slack-app-setup.md#scope-変更後の再-install) を確認します |
| channel が見つからない(bot token 利用時) | bot / app が対象 channel に未参加 | 対象 channel で `/invite @slapex` を実行します([bot / app を channel に参加させる](slack-app-setup.md#bot--app-を-channel-に参加させる)) |
| channel が見つからない(user token 利用時) | 認可したユーザーが対象 channel を参照できない | [Channel access](slack-app-setup.md#channel-access) を確認します |
| 候補が多すぎると表示されて終了する | channel keyword が曖昧で候補が 11 件以上 | より具体的な channel 名の一部、または channel ID を指定して再実行します |

終了コード(exit code)の意味の全一覧は脚注を参照してください。[^spec-exit]

初回セットアップからやり直したい場合は [クイックスタート](quickstart.md) を参照してください。

[^spec-range]: 仕様の正本(開発者向け): [`doc/design/output-format.md` の取得範囲](../design/output-format.md#取得範囲)、[`doc/design/cli-interface.md` の option 一覧](../design/cli-interface.md#option-一覧)。
[^spec-size]: 仕様の正本(開発者向け): [`doc/design/output-format.md` の添付ファイルのサイズ制限](../design/output-format.md#添付ファイルのサイズ制限)。
[^spec-render]: 仕様の正本(開発者向け): [`doc/design/html-rendering.md` の本文の変換](../design/html-rendering.md#本文の変換mrkdwn--html)。
[^spec-platform]: 仕様の正本(開発者向け): [`doc/design/cli-interface.md` の対象プラットフォーム](../design/cli-interface.md#対象プラットフォーム)。
[^spec-token]: 仕様の正本(開発者向け): [`doc/design/cli-interface.md` の環境変数](../design/cli-interface.md#環境変数)。
[^spec-consistency]: 仕様の正本(開発者向け): [`doc/design/slack-api-usage.md` の取得の整合性](../design/slack-api-usage.md#取得の整合性)。
[^spec-exit]: exit code の全一覧(開発者向け): [`doc/design/cli-interface.md` の exit code](../design/cli-interface.md#exit-code)。
