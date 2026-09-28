# 作業ブランチメモ

- ブランチ: `claude/issue-251-fu-17-u75f2x`(cloud session が指定。Issue の推奨ブランチ名は `skip-hidden-poster-resolution`)
- PR: #284
- 最終更新: 2026-09-28

## 目的

Issue #251(FU-17)。`collectUserIDs` は、message の表示の分類によらず、投稿者(`m.User`)と inviter(`m.Inviter`)を `users.info` の対象に加えていた。一方 `messageView` は、tombstone と本文の無い未知 subtype の行で user を表示せず、system 行でも avatar を表示せず、投稿者と inviter の表示名を actor の prefix と invited-by の補足でしか使わない。そのため、名前も avatar も使わない user について、`users.info` の呼び出し、avatar の download、`.cache/slack_api_cache.json` の `users` の entry が発生していた。投稿者と inviter の収集条件を、描画側で名前や avatar を使う条件に揃える。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 19 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 14 件目である。前のスレッド(#209 / PR #283)の報告が、`collectUserIDs` の収集条件を描画側に揃える一連(#205 → #209 → #251)の最後で、PF-06(#278)の前提にもなることから次の Issue に推した。

## 現在の状況

- 依存は無い。直列の条件だった #209(FU-08 / PR #283)は merge 済み。main `29614dd`(PR #283 の merge)から作業した。
- 収集条件の修正、test、設計文書の同期を実装し、Issue の「検証」を実行した(「検証」)。
- PR #284 作成済み(draft)。採番と `progress.md` の PR 欄の反映を済ませた。次は CI を確かめてから review を subagent に委譲する(P2)。

## 決定事項

### 着手時の現行コード

Issue の行番号は main `c7b0f44` 時点の値である。main `29614dd` では次のとおりだった。

- `internal/export/message_collect.go` 98 行目の `collectUserIDs` は、106〜111 行目で分類によらず `m.User` と `m.Inviter` を集め、その後 `messageKindOf` で mention の走査だけを切り替えていた(#205 / PR #241 は mention の走査だけを切り替え、投稿者と inviter は範囲外とした)。
- `internal/export/message_view.go` 58 行目の `messageKindOf` が表示の分類を返す。`messageView`(106 行目)は、通常表示(`messageFull`)で投稿者の表示名、avatar、`is_bot`(`APP` chip)を使う。system 行(`messageSystem`)は avatar を持たず、本文を `systemBody`(149 行目)で組み立てる。tombstone は `(削除)` と `?`、本文の無い未知 subtype は「(未対応のメッセージ種別: subtype名)」を表示し、どちらも user を使わない。
- `systemBody` は、`channel_topic` / `channel_purpose` / `channel_name`(`actorPrefixSystemSubtypes`)で、本文が投稿者の mention(`<@ID>` / `<@ID|`)か `@表示名` で始まらないとき、投稿者の表示名を prefix にする(154〜155 行目、`systemTextStartsWithActor` は 236 行目)。`channelJoinInviterSuffix`(162 行目)は、`channel_join` で inviter が空でなく、参加した user と異なり、本文に inviter の mention(`systemTextMentionsUser`)が無いとき、inviter の表示名で `(invited by @表示名)` を補足する(163〜166 行目)。
- 解決した user は全員の avatar を保存し(`internal/export/page.go` の `newMessageViewBuilder`)、`slack_api_cache.json` の `users` に入る(`internal/export/cache.go`)。

### 方針

- 投稿者は、通常表示の message と、actor の prefix を補完し得る system 行(`channel_topic` / `channel_purpose` / `channel_name` で、本文が投稿者の mention で始まらないもの)でだけ集める。本文が `@表示名` で始まるかは表示名を解決するまで分からないため、mention で始まらない行は集める。
- inviter は、invited-by を補足し得る `channel_join` の行(inviter が空でなく、参加した user と異なり、本文に inviter の mention が無いもの)でだけ集める。本文が inviter を label の無い mention で含む場合は、mention として集める。
- tombstone と本文の無い未知 subtype の行では、投稿者も inviter も集めない。`channel_join` / `channel_leave` / `channel_archive` / `channel_unarchive` / `pinned_item` の投稿者も集めない(本文の label の無い mention に現れれば、mention として集める)。
- 通常表示の message の inviter は集めない。`inviter` を使うのは `channel_join` の system 行だけで、通常表示の経路は `inviter` を読まない。
- Issue の作業内容の「system 行は描画側の条件と突き合わせて決める」は、上の 2 条件を描画側と同じ関数で判定することで満たした。完了条件の「投稿者と inviter の収集条件が、描画側で名前や avatar を使う条件と一致する」に合わせ、system 行の投稿者も、名前を使わない行では集めない。

### 実装

- `internal/export/message_view.go` に `systemActorPrefixCandidate`(投稿者の表示名が要る system 行)と `channelJoinInviterSuffixCandidate`(inviter の表示名が要る `channel_join` の行)を置き、`systemBody` と `channelJoinInviterSuffix` もこの 2 関数で判定するようにした。描画側と収集側が同じ条件を読むため、片方だけを変えることが無い。#205 が `messageKindOf` を両側で共有した形に揃えた。
- `systemTextStartsWithActor` は、mention の 2 形を `systemActorPrefixCandidate` へ、`@表示名` の判定を `systemBody` へ分けて削除した。表示は変えていない(条件を展開すると元の判定と一致する。既存の `TestRunIntegrationSystemRows` も通る)。
- `collectUserIDs` は、投稿者と inviter の追加を `messageKindOf` の switch の中へ移した。`messageFull` は投稿者を、`messageSystem` は上の 2 関数が真のときだけ投稿者と inviter を集める。doc comment に、分類ごとに集める user とテキストを書いた。`messageKind` の doc comment も、user の収集に使うことを足した。

### test

- `TestCollectUserIDsMatchesMessageView`(`internal/export/message_collect_test.go`): message の形ごとに、`messageView` が want の user だけを解決したときと全員を解決したときで同じ view を返すこと(描画側が want の外の user を使わない)と、`collectUserIDs` が want を返すことを確かめる contract test。通常表示、本文のある未知 subtype、通常表示の inviter、tombstone、本文の無い未知 subtype、`channel_join` 6 通り(mention の有無、inviter、参加した user と同じ inviter、inviter の mention の label の有無)、`channel_leave`、`pinned_item`、`channel_topic` / `channel_purpose` / `channel_name` の 4 通り(prefix を補う、mention で始まる、label 付きの mention で始まる、`@表示名` で始まる)、投稿者の無い `channel_topic` の 18 通り。`@表示名` で始まる行は、投稿者を解決しなくても同じ view になるが、そう分かるには名前が要るため want に残す(doc comment に書いた)。
- `TestRunIntegrationUnshownPoster`(`integration_rendering_test.go` の case 5e、case 5d の直後): tombstone の親の投稿者(U04)、本文の無い未知 subtype の投稿者(U05)、label 付きの mention で投稿者を示す旧形式の `channel_join`(U06)と `channel_purpose`(U07)について、`users.info` を呼ばず、`slack_api_cache.json` の `users` に入らず、avatar の manifest entry も download も無いことを確かめる。actor の prefix の U02、invited-by の U03、投稿と mention の U01 は解決され、表示は変わらない。`users.info` は 3 回。
- 投稿者の U01 の avatar が保存されることを、avatar の検査の陽性対照として確かめる。名前だけを使う U02 / U03 の avatar の有無は検査しない。名前だけを使う user の avatar download(#251 のスコープ外)を固定しないためである(#209 の case 5d と同じ扱い)。

### 設計文書

- `doc/design/slack-api-usage.md` の「user 解決」に 2 行を足した。投稿者と inviter は HTML でその user の表示名や avatar を使う行からだけ集めること(通常表示は投稿者を集め、tombstone と本文の無い未知 subtype は集めない)と、system 行で集める投稿者と inviter の条件(`@表示名` は解決するまで分からないため、その行の投稿者も集める)である。PR #241 と PR #283 が収集条件の変更に合わせて同じ節を更新した前例に従う。
- `doc/design/html-rendering.md` の「メッセージ種別(subtype)の表示」は変えていない。表示の規則は変わらず、矛盾しないことを確かめた。topic / purpose / name の行は「`text` 先頭に actor が含まれない場合は、`user` field を…解決し」とあり、収集の条件と一致する。`channel_join` の行は「`inviter` field がある場合は…解決し…補足を表示する。…同一…本文にすでに inviter mention が含まれる場合は補足しない」とあり、補足しない場合に解決しなくても表示は同じである。解決する対象の条件は `slack-api-usage.md` に置いた。
- decision log は作らない。描画側の既存の挙動に収集側を揃える修正で、方針を変えていないため(#205、#209 と同じ)。decision log 0027 の「一部 subtype について `user` 解決結果も表示に使う」、0035 の「投稿・thread replies に登場する投稿者の avatar」とも矛盾しない。
- `progress.md` の FU-17 の行を `done(PR merge後)`、次にやることを「merge後は対応なし」にした。PR 欄は採番後に記入する。

### 出力生成系 3 skill の適用判断

- `update-sample-exports`: `internal/export/**` の変更だが、表示変換ではなく `users.info` の対象の収集の変更で、`systemBody` の変更も表示を変えない整理である。demo の fixture では、ja / en とも 4 人が全員通常表示の投稿者で、tombstone(ja だけ)は `User` を持たず、本文の無い未知 subtype は無い。`channel_join` / `channel_topic` の本文は投稿者の label の無い mention で始まる。念のため `-time 2026-07-04T16:32:41+09:00` に固定して別ディレクトリへ再生成し、commit 済みの `doc/samples/ja` / `en`(各 18 ファイル)と `diff -r` で無差分だった。Users phase も `4 users, 1 bot resolved` のままだった。sample は commit しない。
- `update-readme-preview-screenshots`: 適用しない。sample export に差分が無く、screenshot に映る出力は変わらない。
- `update-readme-demo-gif`: 適用しない。phase 名、summary、進捗表示の文言は変えない。demo の fixture では集める user が変わらないため、Users phase の件数も変わらない。

## 次にやること

- PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。(完了)
- CI を確かめてから review を subagent に委譲する(P2)。
- review の指摘に対応する(P4)。
- CI を確かめてから再確認を subagent に委譲する(P5)。
- Codex のクロスレビュー(他の Agent 種別の review cycle)。指摘があれば対応する。
- review thread の resolve と、PR の merge(ユーザー)。

## 検証

2026-09-28、cloud session(project の thread)で実行。すべて Docker Compose(`docker compose run --rm dev ...`)経由。実 token は使わず架空 fixture だけを使った。

| 項目 | 結果 |
|---|---|
| 追加した test が修正前に失敗すること | `TestCollectUserIDsMatchesMessageView` は 18 通りのうち 11 通り(本文の無い未知 subtype、tombstone、通常表示の inviter、`channel_join` 5 通り、`channel_leave`、`pinned_item`、label 付きの mention で始まる `channel_name`)で `collectUserIDs` 側が失敗し、`messageView` 側の検査はすべて通った。`TestRunIntegrationUnshownPoster` は `users.info` が 7 回(U01〜U07)で失敗した。HTML の表示の検査は修正前から通った(表示は変わらない) |
| 結合 test の各検査が単独でも失敗を検出すること | 修正前の収集に戻し、`users.info` の回数の検査を外すと `slack_api_cache.json` の `users` が U01〜U07 で失敗した。さらに cache の検査も外すと、U04 の avatar の manifest entry で失敗した(変更は戻した) |
| 変異の検出(使い捨ての書き換え。いずれも戻した) | 8 種すべて検出した。収集側を修正前に戻す(contract test 11 通りと case 5e)、system 行に投稿者の avatar を出す(contract test の描画側 8 通り)、tombstone に投稿者名を出す(contract test の描画側と既存の case 3)、`systemActorPrefixCandidate` の label 付き mention の判定を外す(contract test の描画側と case 5e)、`channelJoinInviterSuffixCandidate` の参加者と同一の判定を外す(contract test の描画側)、system 行の投稿者を常に集める(contract test 8 通りと case 5e)、通常表示の投稿者を集めない(contract test 3 通り)、`channel_join` の inviter を常に集める(contract test 2 通り) |
| `go test ./internal/export` | pass |
| `go test ./...` | pass |
| `go test ./internal/export ./internal/render -count=5 -shuffle=on` | pass |
| `go test -race -count=2 ./internal/export ./internal/render`(`CGO_ENABLED=1`) | pass |
| `go vet ./...` | pass |
| `go build ./...` | pass |
| `gofmt -l .` | 出力なし |
| `git diff --check` | 出力なし |
| cross-compile(`CGO_ENABLED=0`、darwin / linux × amd64 / arm64 の `go build ./cmd/slapex`) | 4 通りとも成功 |
| sample export の再生成(`TZ=Asia/Tokyo`、`-time 2026-07-04T16:32:41+09:00`、別ディレクトリ) | `doc/samples/ja` / `en` と `diff -r` で無差分(各 18 ファイル)。Users phase は ja / en とも `4 users, 1 bot resolved` |

## リスク・ブロッカー

- 名前だけを使う user(actor の prefix の投稿者、invited-by の inviter、label の無い mention にだけ現れる user)も、引き続き avatar を download している。#251 はこれを「名前だけを表示する user の avatar download を省くこと」としてスコープ外に挙げ、「必要なら別 Issue にする」としている。本 PR では扱わない(#209 / PR #283 と同じ扱い)。
- 実 workspace の message で、system 行の本文がどの形で投稿者を含むか(label の無い mention、label 付きの mention、actor を含まない)は確かめていない。どの形でも表示は変わらず、収集の対象だけが変わる。

## セッションログ

- 2026-09-28: Issue #251、`progress.md`、前のスレッドの報告を読んだ。Issue は open でコメントは無く、依存は無い。直列の条件だった #209 は PR #283 で merge 済み。`collectUserIDs` は main `29614dd` の `internal/export/message_collect.go` の 98 行目にある。
- 2026-09-28: test を先に足し、修正前に失敗することを確かめてから収集条件を直した(`ec5e69d`)。設計文書と `progress.md` を更新し、Issue の「検証」と sample export の一致の確認を済ませた。
- 2026-09-28: PR #284 を draft で作成し、note を採番した(`4705409`)。`progress.md` の FU-17 の PR 欄を #284 にした(P1)。検証はすべて pass(「検証」)。出力生成系 3 skill は適用しない(「出力生成系 3 skill の適用判断」。`update-sample-exports` の適用条件を確かめるため、固定時刻で再生成して無差分を確かめた)。
  - `run-issue-task` の報告から引き上げた項目。`number-working-branch-note` の報告の「書き換えた行の一覧」: note の `PR:` 欄(`未作成` → `#284`)と、PR description の note のファイル名参照 1 行(`draft_` → `284_`)。title は書き換えていない。「触らずに残した行の一覧」: note の「現在の状況」の「次は PR の作成(draft)と採番。」(定型に当てはまらない)と、「次にやること」の「PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。」(`progress.md` の反映を含む複合行)。この 2 行は、`progress.md` の反映の後に上のとおり書き換えた。情報統制チェックで直した箇所は無い。出力生成系 3 skill は呼ばなかった(「いつ使うか」に当たらない)。
  - PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。
