# HTML 表示仕様

このファイルには、最終成果物の `index.html` の見た目と表示方針をまとめる。

想定読者は、このツールを利用する人間と、HTML / CSS を実装・検証する担当者である。

本ファイルの表示方針と変換規則は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

出力ディレクトリ構造や保存される assets は `output-format.md`、利用者の操作の流れは `usage-flow.md`、user / emoji の解決は `slack-api-usage.md` を参照する。表示仕様の決定経緯は `decision-log/0012-html-rendering-style.md`、処理対象 label の表示は `decision-log/0020-target-label-display.md`、本文変換・subtype・時刻表示は `decision-log/0026-mrkdwn-html-conversion.md` / `decision-log/0027-message-subtypes-rendering.md` / `decision-log/0028-timestamp-timezone-display.md` を参照する。

## 表示方針

最終成果物の `index.html` は、Slack default の投稿表示を模倣した見た目にする。

1. 冒頭に取得対象 workspace、channel、export 実行時刻を表示する。
2. workspace 表示には workspace 名、workspace URL または domain、短い `team_id` を含める。
3. channel 表示には channel 名、channel ID、public/private、archived 状態、bot membership を含める。
4. 投稿は channel timeline と同じく、上から oldest、下へ latest の順に表示する。
5. 日付と時刻は相対表現ではなく、絶対時刻として表示する。
6. thread replies は親投稿の下に、親投稿よりインデントを下げて表示する。
7. thread replies は初期表示で展開済みにする。
8. reaction は、絵文字 icon と件数を可能な限り Slack default 風に表示する。
9. reaction した user の一覧や名前は表示しない。
10. JavaScript は一切使わない。
11. style は `style.css` に分離し、HTML 内に固定的に inline style として埋め込まない。
12. CSS で表現可能な interaction は活用してよい。
13. thread の開閉を入れる場合は、JavaScript ではなく HTML native の `<details open>` / `<summary>` など、JavaScript なしで動作する仕組みを使う。

冒頭に表示する workspace / channel label の内容と、実行中の画面表示との対応は `usage-flow.md` の「処理対象の表示」を参照する。

Slack default 風の avatar、投稿者名、絶対時刻、本文、reactions、attachments を CSS で整え、HTML 自体は静的 file として閲覧できるようにする。

## 画像と添付ファイルの表示

ユーザーがアップロードした画像は、Slack file object の available な thumbnail のうち表示に適したものを保存し、HTML 上の inline image として使う。あわせて original 画像も保存し、inline image をクリックすると original を開けるようにする。

original 画像の保存には `--max-attachment-size` を適用する。original がサイズ上限を超える場合、original は download せず、HTML では thumbnail 表示を残したうえで original がサイズ上限超過により保存されなかったことを示す。thumbnail も取得できない場合は、通常の添付ファイル表示または置換メッセージとして扱う。

保存対象 asset とサイズ上限の方針は `output-format.md` を参照する。

## 本文の変換(mrkdwn → HTML)

Slack message の本文は mrkdwn 形式の `text` フィールドを正とし、次の規則で HTML へ変換する。

| mrkdwn | HTML 上の表示 |
|---|---|
| `*bold*` | 太字 |
| `_italic_` | 斜体 |
| `~strike~` | 取り消し線 |
| `` `code` `` | inline code |
| ` ```pre``` ` | code block |
| `> quote` | 引用 block |
| `<https://example.com>` / `<https://example.com\|label>` | リンク(label があれば label を表示) |
| `<@U0123456789>` | `@表示名`(mention 風のハイライト表示) |
| `<#C0123456789\|general>` | `#channel名` |
| `<!here>` / `<!channel>` / `<!everyone>` | `@here` などのハイライト表示 |
| `<!subteam^S...\|@group>` | `@group名` |
| `<!date^...^{fallback}>` | fallback 文字列をそのまま表示 |
| `:emoji_name:` | 絵文字(Unicode 直接表示またはローカル画像、`slack-api-usage.md`) |
| 改行 | 改行として表示 |

- mention の表示名解決は `slack-api-usage.md` の user 解決に従う。解決できない場合は user ID を表示する。
- rich text(`blocks` の `rich_text`)の構造を直接レンダリングすることは初期対象外とし、`text` フィールド(Slack が生成する fallback テキストを含む)を表示の正とする。`blocks` の完全レンダリングは将来検討とする。
- legacy attachment(URL unfurl など)は、URL preview 画像(`output-format.md`)とタイトル・本文テキストの範囲で表示し、色付き枠やフィールド構造の完全模倣は初期対象外とする。

## エスケープとサニタイズ

- Slack 由来のすべてのテキストは HTML エスケープしたうえで、本ツールが生成するマークアップだけを組み立てる。Slack 本文に HTML 断片が含まれていても、そのまま出力しない。
- リンクとして出力する URL は `http` / `https` scheme のみ許可する。その他の scheme はリンク化せずテキストとして表示する。
- 「JavaScript を一切使わない」方針(表示方針 10)を、サニタイズの面でも維持する。生成 HTML に script 要素や inline event handler を含めない。

## メッセージ種別(subtype)の表示

| 分類 | subtype の例 | 表示 |
|---|---|---|
| 通常表示 | subtype なし、`file_share`、`thread_broadcast`、`bot_message`、`me_message` | 通常の投稿として表示する(`me_message` は斜体) |
| システム行 | `channel_join`、`channel_leave`、`channel_topic`、`channel_purpose`、`channel_name`、`channel_archive`、`channel_unarchive`、`pinned_item` | avatar なしの 1 行システムメッセージとして控えめに表示する |
| 置換表示 | `tombstone`(削除済みだが thread が残る親投稿) | 「(削除されたメッセージ)」のプレースホルダを表示し、その thread replies は通常どおり表示する |
| 未知の subtype | 上記以外 | `text` があれば通常表示に準じ、無ければ「(未対応のメッセージ種別: subtype名)」のシステム行にする |

- `thread_broadcast` は Slack の表示と同様、channel timeline と thread 内の両方に表示する。
- 編集済みメッセージは本文末尾に「(edited)」相当の控えめな表示を付ける。reaction や編集の履歴は表示しない。

## 時刻表示

- 日時は実行環境の local timezone を使い、`YYYY-MM-DD HH:MM` 形式(24 時間制)の絶対時刻で表示する(表示方針 5 の具体化)。
- `index.html` 冒頭のヘッダに、export 実行時刻と使用 timezone(UTC offset)を明記する。
- 各時刻要素の `title` 属性に ISO 8601(UTC)のフル時刻を入れ、hover で正確な時刻を確認できるようにする。
- timeline 上で日付が変わる位置に date divider(日付区切り行)を表示する。
