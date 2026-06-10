# 0027 message subtype の表示方針

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/html-rendering.md`, `doc/design/slack-api-usage.md`

## 背景

`conversations.history` は通常投稿のほかに subtype 付きメッセージ(join / leave、topic 変更、bot 投稿、thread broadcast、削除済み thread 親など)を返す。これらをどう表示・除外するかが未確定で、HTML レンダリングの実装範囲を決められなかった。

## 候補

- subtype 付きメッセージをすべて除外し、通常投稿だけ表示する。
- すべて通常投稿と同じ形式で表示する。
- 分類して扱う: 本文系は通常表示、システム系は控えめな 1 行表示、削除済みはプレースホルダ、未知は fallback。

## 検討内容

- 全除外は channel の文脈(参加・退出、topic 変更)が失われ、archive 用途として情報が欠落する。
- 全部同形式は、Slack 上の見た目(システムメッセージは小さく表示される)から離れ、「Slack default の投稿表示を模倣」(0012)に反する。
- 分類方式は Slack の表示に最も近い。未知 subtype への fallback を決めておくと、Slack 側の subtype 追加に対して安全になる。
- thread_broadcast は Slack 上で timeline と thread の両方に表示されるため、模倣方針に従い両方へ表示する(重複排除しない)。
- 削除済みだが thread が残る親(tombstone)を除外すると、その thread replies の表示位置が失われる。プレースホルダ表示が必要。

## 決定

`html-rendering.md` に次の分類を確定した。

- 通常表示: subtype なし、`file_share`、`thread_broadcast`、`bot_message`、`me_message`(斜体)。
- システム行: `channel_join` / `channel_leave` / `channel_topic` / `channel_purpose` / `channel_name` / `channel_archive` / `channel_unarchive` / `pinned_item`。
- 置換表示: `tombstone` は「(削除されたメッセージ)」とし、thread replies は通常表示。
- 未知 subtype: `text` があれば通常表示に準じ、無ければ「(未対応のメッセージ種別)」のシステム行。

編集済みメッセージは「(edited)」相当の控えめな表示を付ける。

## 理由

- channel の文脈を保ちつつ Slack の見た目の模倣を優先し、未知 subtype への安全な fallback で将来の API 変化にも壊れにくくするため。

## 影響

- HTML テンプレートは「通常投稿」「システム行」「プレースホルダ」の 3 形式を持つ。
- `--max-posts` のカウント対象(timeline メッセージ)に subtype 付きメッセージも含まれる(`output-format.md`)。

## 後から見直す条件

- システム行の種類が増えて表示がノイズになり、表示の選別 option が必要になった場合。
- Slack が subtype 体系を変更した場合。
