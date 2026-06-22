# HTML 表示仕様

このファイルには、最終成果物の `index.html` の見た目と表示方針をまとめる。

想定読者は、このツールを利用する人間と、HTML / CSS を実装・検証する担当者である。

本ファイルの表示方針と変換規則は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

出力ディレクトリ構造や保存される assets は `output-format.md`、利用者の操作の流れは `usage-flow.md`、user / emoji の解決は `slack-api-usage.md` を参照する。表示仕様の決定経緯は `decision-log/0012-html-rendering-style.md`、処理対象 label の表示は `decision-log/0020-target-label-display.md`、本文変換・subtype・時刻表示は `decision-log/0026-mrkdwn-html-conversion.md` / `decision-log/0027-message-subtypes-rendering.md` / `decision-log/0028-timestamp-timezone-display.md` を参照する。

## 表示方針

最終成果物の `index.html` は、Slack default の投稿表示を模倣した見た目にする。

1. 冒頭に取得対象 workspace、channel、export 実行時刻を折りたたみ可能なメタ情報として表示する。
2. workspace 表示には workspace 名、workspace URL または domain、短い `team_id` を含める。
3. channel 表示には channel 名、channel ID、public/private、archived 状態、bot membership を含める。
4. 投稿は channel timeline と同じく、上から oldest、下へ latest の順に表示する。
5. 日付と時刻は相対表現ではなく、絶対時刻として表示する。
6. thread replies は親投稿の下に、親投稿よりインデントを下げて表示する。
7. thread replies は `details` / `summary` による開閉表示にし、初期表示では折りたたむ。
8. reaction は、絵文字 icon と件数を可能な限り Slack default 風に表示する。
9. reaction した user の一覧や名前は表示しない。
10. JavaScript は一切使わない。
11. style は `style.css` に分離し、HTML 内に固定的に inline style として埋め込まない。
12. CSS で表現可能な interaction は活用してよい。
13. 折りたたみや開閉は、JavaScript ではなく HTML native の `<details>` / `<summary>` など、JavaScript なしで動作する仕組みを使う。

冒頭に表示する workspace / channel label の内容と、実行中の画面表示との対応は `usage-flow.md` の「処理対象の表示」を参照する。チャンネル名見出しは常時表示し、workspace / channel / export 実行時刻 / 取得範囲のメタ情報は初期状態で折りたたむ。

Slack default 風の avatar、投稿者名、絶対時刻、本文、reactions、attachments を CSS で整え、HTML 自体は静的 file として閲覧できるようにする。

thread replies は初期状態では折りたたみ、thread label をクリックして開閉できるようにする。日付区切りと混同しないよう、thread label には横罫線を付けず、Slack の thread summary に近い控えめな chip として表示する。chip には返信者 avatar を最大 3 件表示し、4 人以上の場合は残り人数を `+N` で示す。label 文言は `N messages` のように件数だけを表示し、`Thread (` / `)` は付けない。chip は親投稿本文の左端近くに置き、クリックしやすいように横幅を確保する一方で、狭い表示では親幅を超えないようにする。返信群はそこからさらにインデントして会話の枝であることを示す。URL preview と同じ「左罫線 + インデント」だけに寄せすぎると隣接時に判別しづらいため、URL preview 側の表示は据え置き、thread 側で thread label、左ガイドライン、返信位置を示す節点を使って会話の入れ子であることを示す。長文でも読みやすいように、thread 全体を背景色の面で囲う表現は避ける。投稿の hover ハイライトは個別投稿単位に限定し、thread を持つ親投稿に hover しても返信群全体へ背景色が広がらない構造にする。

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

- Slack は inline code / code block の内部でも、URL の auto-link(`<https://...>`)や mention などの `<...>` 構文を `text` に格納する。code 内の構文はリンクや mention のマークアップを生成せず、表示テキスト(label があれば label、URL はその URL、mention は `@表示名` など)へ展開して code の内容として表示する。
- mention の表示名解決は `slack-api-usage.md` の user 解決に従う。解決できない場合は user ID を表示する。
- rich text(`blocks` の `rich_text`)の構造を直接レンダリングすることは初期対象外とし、`text` フィールド(Slack が生成する fallback テキストを含む)を表示の正とする。`blocks` の完全レンダリングは将来検討とする。
- legacy attachment(URL unfurl など)は、service 名、service icon、URL preview 画像(`output-format.md`)とタイトル・本文テキストの範囲で表示し、色付き枠やフィールド構造の完全模倣は初期対象外とする。service icon は Slack API の attachment / unfurl 情報に URL が含まれる場合だけ表示し、ツール自身による favicon / Open Graph fetch は行わない。

## エスケープとサニタイズ

- Slack 由来のすべてのテキストは HTML エスケープしたうえで、本ツールが生成するマークアップだけを組み立てる。Slack 本文に HTML 断片が含まれていても、そのまま出力しない。
- code(inline code / code block)の内容も同じ方針の対象とする。`<...>` 構文を表示テキストへ展開した後に残る生の `<` `>` は HTML エスケープして出力する(利用者が入力した `<` `>` は Slack API 側でエンティティ化済みのため、二重エスケープにはならない)。
- リンクとして出力する URL は `http` / `https` scheme のみ許可する。その他の scheme はリンク化せずテキストとして表示する。
- 「JavaScript を一切使わない」方針(表示方針 10)を、サニタイズの面でも維持する。生成 HTML に script 要素や inline event handler を含めない。

## メッセージ種別(subtype)の表示

| 分類 | subtype の例 | 表示 |
|---|---|---|
| 通常表示 | subtype なし、`file_share`、`thread_broadcast`、`bot_message`、`me_message` | 通常の投稿として表示する(`me_message` は斜体) |
| システム行 | `channel_join`、`channel_leave`、`channel_topic`、`channel_purpose`、`channel_name`、`channel_archive`、`channel_unarchive`、`pinned_item` | avatar なしの 1 行システムメッセージとして控えめに表示する |
| 置換表示 | `tombstone`(削除済みだが thread が残る親投稿) | 「(削除されたメッセージ)」のプレースホルダを表示し、その thread replies は通常どおり表示する |
| 未知の subtype | 上記以外 | `text` があれば通常表示に準じ、無ければ「(未対応のメッセージ種別: subtype名)」のシステム行にする |

- `channel_join` の system 行で `inviter` field がある場合は、追加操作を行ったユーザーを display name 優先で解決し、本文末尾に `(invited by @表示名)` 相当の補足を表示する。`inviter` が空、参加した `user` と同一、解決不能、または本文にすでに inviter mention が含まれる場合は補足しない。
- `channel_topic`、`channel_purpose`、`channel_name` の system 行で `text` 先頭に actor が含まれない場合は、`user` field を display name 優先で解決し、`@表示名` 相当の prefix を補完する。`user` が空または解決不能の場合は `text` のみ表示する。
- `thread_broadcast` は Slack の表示と同様、channel timeline と thread 内の両方に表示する。
- 編集済みメッセージは本文末尾に「(edited)」相当の控えめな表示を付ける。reaction や編集の履歴は表示しない。
- 本文がない編集済みメッセージでは、添付ファイルや URL preview などの主要表示の後に控えめな fallback として「(edited)」相当の表示を付ける。

## 時刻表示

- 日時は実行環境の local timezone を使い、`YYYY-MM-DD HH:MM` 形式(24 時間制)の絶対時刻で表示する(表示方針 5 の具体化)。
- `index.html` 冒頭のヘッダに、export 実行時刻と使用 timezone(UTC offset)を明記する。
- 各時刻要素の `title` 属性に ISO 8601(UTC)のフル時刻を入れ、hover で正確な時刻を確認できるようにする。
- timeline 上で日付が変わる位置に date divider(日付区切り行)を表示する。
