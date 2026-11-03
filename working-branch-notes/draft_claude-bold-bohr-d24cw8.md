# 作業ブランチメモ

- ブランチ: `claude/bold-bohr-d24cw8`(cloud session が指定。Issue に推奨ブランチ名の記載は無い)
- PR: (未採番)
- 最終更新: 2026-11-03

## 目的

Issue #303。Slack の MCP connector(Cursor など)経由で user が投稿したメッセージには、Slack 側で `*使用して送信されました* Cursor`(英語 locale では `*Sent using* <アプリ名>`)の footer が付く。slapex はこれを本文の一部として本文と同じ大きさで描画しており、Slack の表示(本文の下の控えめな 1 行)と見た目が合わない。footer を判別し、本文の下に控えめな 1 行として表示する。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。

## 現在の状況

- P1(実装と PR 作成)を進めている。

## 決定事項

- 実データの確認経路:
  - Issue のスクリーンショット 2 枚は cloud session から取得できなかった。`github.com/user-attachments/...` は agent proxy が「session は設定された repository に bound」として 403 を返し、配信元の `private-user-images.githubusercontent.com` は network policy で拒否された。迂回はしていない。
  - ユーザーから、slapex で取得した公開アーカイブ(`meetup.kzrb.org` の meetup #170 の slack_archive)の最終書き込みが該当投稿だと教わった。ユーザーが `meetup.kzrb.org` を environment の許可 host に追加した後に取得して確かめた。
  - 該当投稿の出力は、本文の末尾に改行なしで ` <strong>使用して送信されました</strong> Cursor` が続く形だった。`text` の末尾は ` *使用して送信されました* Cursor` と読める(mention でも link でもない、素のアプリ名)。投稿者は通常の user で、APP chip は出ていない(`bot_id` / `bot_profile` が無く、`is_bot` でもない)。
  - Web の報告(GitHub 上の公開 Issue / PR)では、Slack の hosted MCP server(`mcp.slack.com`)が `slack_send_message` で送る投稿の末尾に `*Sent using* <アプリ名>` を server 側で付ける。設定で外せない。英語 locale の形はこの報告に拠り、実データでは未確認。
- ユーザーの判断(2026-11-03):
  - 目標の表示は「本文の下に控えめな 1 行」。
  - `text` だけで判別できない場合は `blocks` の読み取りまで含めてよい。今回は `text` の末尾で判別できたため `blocks` は読まない。`blocks` の構造(context block の有無など)は実データで未確認のまま推測で実装しない。
- 判別の規則(`text` の末尾):
  - 末尾の行の終わりが `*<label>* <アプリ名>` で、`*<label>*` の直前が本文の先頭または空白(空白・改行)のときだけ footer とみなす。`<label>` は `使用して送信されました`(ja)と `Sent using`(en)の 2 つに限る。他の locale の文言は確かめる手段が無いため推測で足さない。
  - アプリ名は改行を含まない空でない文字列とする(素の名前のほか、`<@U…>` の mention や `Dimension (as you)` のような形も受ける)。
  - footer を除いた本文の末尾の空白は落とす。本文が空になる場合は本文を出さず footer だけを出す。
- 表示:
  - 本文の直下(画像、添付ファイル、unfurl より前)に `message-footer` の 1 行として出す。Slack では footer が本文と同じ投稿の中で添付より前に出るためである。
  - footer も本文と同じ mrkdwn 変換を通す(label の太字、mention、link をそのまま扱う)。文字を小さくし、muted color にする。
  - `(edited)` は従来どおり本文の末尾に付ける。
- mention の収集(`collectUserIDs`)と本文の emoji filter は `m.Text` 全体を見るため、footer の切り出しの影響を受けない。
- 実装:
  - `internal/export/message_view.go` の `splitSentUsingFooter` が footer を切り出し、`messageView` が本文と footer を別々に mrkdwn 変換する。`render.MessageView` に `Footer` を足した。
  - template は本文の直下に `<div class="message-footer">` を出す。`style.css` は `.message-footer`(muted color、12px)と `.message-footer .mention`(ハイライト無し)を足した。`.message-footer .mention` は `.mention` の後に置いた(`html_test.go` の `cssBlock` が selector の最初の出現を読むため、`.mention` の block を取り違えないようにした)。
- 仕様: `doc/design/html-rendering.md` の「本文の変換」に「app 経由の投稿の footer」を足した。比べた案があるため decision log 0070 を作り、`index.md` の「現在有効な主要方針」に行を足した。0026 は superseded にしない(`text` を正とする方針は変えない)。
- `progress.md`: #303 は索引に無い単発 Issue のため更新しない。
- 出力生成系 3 skill の適用判断:
  - `update-sample-exports`: 適用条件(`internal/render/**` の HTML / CSS)に当たるため実行した。`style.css` は template と同じ内容に更新された。`index.html` の差分は ja / en とも日時だけ(日時を伏せて比べると一致)で、skill の規定どおり commit しない。demo fixture に footer を持つ投稿は無い。
  - `update-readme-preview-screenshots`: 適用しない。sample export の `index.html` は変わらず、追加した CSS の selector に当たる要素が sample に無いため、screenshot の見た目は変わらない。
  - `update-readme-demo-gif`: 適用しない。CLI 出力と demo fixture が変わらない。

## 次にやること

- PR を作る。
- note を採番する。
- P1 の記録を note に残して push し、CI の完了を確かめる。
- review(P2)を subagent に委譲する。

## 検証

2026-11-03、cloud session で実行。開発コマンドはすべて Docker Compose(`docker compose run --rm dev ...`)経由。実 token は使わず架空 fixture だけを使った。

| 項目 | 結果 |
|---|---|
| `gofmt -l .` | 出力なし |
| `go vet ./...` | pass(exit 0) |
| `go build ./...` | pass |
| `go test -count=1 ./...` | pass |
| `git diff --check` | 出力なし |
| 新しい test が旧コードで失敗すること | `message_view.go` を main に戻し(`splitSentUsingFooter` は何もしない stub)、`TestSplitSentUsingFooter` と `TestRunIntegrationSentUsingFooter` が失敗した |
| `update-sample-exports`(`go run ./tools/gensample`、`TZ=Asia/Tokyo`) | `style.css` だけ commit。`index.html` は日時だけの差分のため戻した。sample が参照する asset はすべて存在する |
| 目視(参考) | 公開アーカイブの該当投稿の markup を新しい構造に置き換え、新しい `style.css` で host の Chromium(preinstall の Playwright)に描画して確かめた。footer は本文の下に小さい muted color の 1 行で出る。画像は scratchpad にだけ置き、commit しない(実在の名前を含むため) |

## リスク・ブロッカー

- 判別は ja / en の 2 つの label に限る。他の locale の footer は従来どおり本文として描画される。
- 人が本文の末尾に `*Sent using* X` と書いた場合も footer として控えめに表示される。内容は失われない。

## セッションログ

- 2026-11-03: Issue #303 を読み、スクリーンショットが取得できないためユーザーに実データを確認した。公開アーカイブで footer の形を確かめ、判別と表示の方針を決めた。
