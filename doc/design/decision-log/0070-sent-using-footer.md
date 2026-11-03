# 0070 app 経由の投稿の footer の表示

- 状態: decided
- 作成日: 2026-11-03
- 最終更新日: 2026-11-03
- 関連: `doc/design/html-rendering.md`, [0026-mrkdwn-html-conversion.md](0026-mrkdwn-html-conversion.md)

## 背景

Issue #303。Slack の MCP connector を通じて app(Cursor)が user の代わりに投稿したメッセージを export すると、本文の末尾に `使用して送信されました Cursor` が本文と同じ大きさで続いて表示された。Slack 上では、この部分は本文の下に控えめな 1 行として表示される。

slapex で取得した公開アーカイブの該当投稿では、出力は本文の末尾に改行なしで ` <strong>使用して送信されました</strong> Cursor` が続く形だった。`text` の末尾は ` *使用して送信されました* Cursor` であり、app 名は mention でも link でもない素の文字列だった。投稿者は通常の user で、`bot_id` / `bot_profile` は無かった。

GitHub 上の公開 Issue / PR の報告では、Slack の hosted MCP server が `slack_send_message` で送る投稿の末尾に、英語 locale では `*Sent using* <app 名>` を server 側で付ける。設定で外せない。app 名の部分が mention の例も報告されている。

0026 は本文を `text` で表示し、`blocks` の描画を初期対象外としている。

## 候補

- (A) `text` の末尾の `*<label>* <app 名>` を、既知の label(`使用して送信されました`、`Sent using`)で判別して本文から切り離す。
- (B) `blocks` を読み、footer に当たる block(context block など)から判別する。
- (C) 投稿の metadata(`app_id` など)で app 経由の投稿を判別し、そのうえで `text` の末尾を切り離す。
- (D) 何もしない(本文の一部として表示する)。

## 検討内容

- (A) は実データの形をそのまま判別でき、0026 の `text` を正とする方針を変えない。label は Slack が locale ごとに翻訳するため、既知の label 以外は判別できない。人が本文の末尾に同じ形を書いた場合も footer として表示されるが、内容は失われず表示の強弱が変わるだけである。
- (B) は locale に依存しない判別ができる可能性がある。ただし該当投稿の `blocks` を確かめる手段が無く(公開アーカイブは HTML だけで、raw の API response は残っていない)、block の構造を推測で実装することになる。ユーザーは `text` で判別できない場合に `blocks` を読むことを許可したが、今回は `text` で判別できた。
- (C) は誤判別を減らせるが、user token で app が投稿したメッセージに `app_id` などが付くかを実データで確かめられない。条件にすると、付かない場合に footer を判別できなくなる。
- (D) は Issue の求める改善にならない。

## 決定

(A) を採用する。

- `text` の最後の行が `*<label>* <app 名>` で終わり、`*<label>*` の直前が `text` の先頭または空白の場合に、`*<label>*` から末尾までを footer とする。`<label>` は `使用して送信されました` と `Sent using` に限る。`<app 名>` は改行を含まない空でない文字列とする。
- footer は本文の直下に、本文より小さい muted color の 1 行として表示する。footer も本文と同じ mrkdwn 変換を通し、footer 内の mention はハイライトを付けない。
- 仕様は `html-rendering.md` の「app 経由の投稿の footer」に置く。

## 理由

- 実データで確かめられた形だけで判別でき、推測の実装を避けられる。
- `text` を表示の正とする 0026 の方針を変えずに済む。
- 誤判別の影響が表示の強弱にとどまる。

## 影響

- `internal/export/message_view.go` が本文の変換の前に footer を切り離し、`render.MessageView.Footer` に渡す。template は本文の直下に `message-footer` を出し、`style.css` が控えめな表示にする。
- mention の収集は `text` 全体を対象とするため、footer の mention も解決される。emoji filter も `text` 全体を見るため、判別の対象は変わらない。
- 既知の label 以外の locale の footer は、従来どおり本文として表示される。

## 後から見直す条件

- 他の locale の footer が実利用で問題になった場合。label の追加か、(B) への切り替えを検討する。
- 該当投稿の raw の API response(`blocks` を含む)を確かめられ、footer が locale に依存しない形で判別できると分かった場合。
- Slack が footer の形(太字の label、`text` への付加)を変えた場合。
