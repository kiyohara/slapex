# 作業ブランチメモ

- ブランチ: `claude/pf-04-issue-276-6d4pux`(cloud session が指定。Issue の推奨ブランチ名は `perf-lane-failure-control`)
- PR: #292
- 最終更新: 2026-09-29

## 目的

Issue #276(PF-04)。PF-03(#275)の origin ごとの lane の上で、download が 429 を受けたら、その origin の lane 全体が `Retry-After` の間 request を出さず、同時数の上限を半分にする。他の origin は止めない。5xx と network error、body の読み取り中の失敗の扱いを決める。#192 で共通化した `withRetry` から lane へ 429 と待ちを伝える。Web API の retry の挙動は変えない。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 26 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #291(PF-03)の merge の後に着手した。

## 現在の状況

- 依存の #275(PF-03、PR #291)と #192(PR #271)は merge 済み。main `6f52108`(PR #291 の merge)から作業した。
- 実装、test、benchmark、設計文書、decision log 0068 を済ませ、Issue の「検証」を実行した(「検証」)。
- PR #292 を draft で作り、note を採番した。`progress.md` の PR 欄を反映した(P1)。
- 第三者 host の `Retry-After` の上限は、ユーザーが推奨の「60 秒まで」を選んだ(2026-09-29 07:51Z、card)。実装は変えず、decision log 0068 に選択を記録した。
- review(P2)の指摘 2 件(`[must]` 1、`[fyi]` 1)を採用して直した(P4。「review の指摘への対応(P4)」)。
- 再確認(P5)で、指摘 2 件とも修正確認済み(resolve 可)になり、未対応は 0 件だった。review cycle `claude-code-e64f479-20260929075701` は完了した。
- 終了時の状態(P6): head `8c6846a` の check runs は 5 件すべて success。note だけの commit の push の後に、PR を Ready for review にする。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `0a83bf7` と PR #271 の head 時点の記載で、main `6f52108` でも同じだった。`withRetry`(`internal/slack/client.go`)は Web API と download で共通で、429 は `Retry-After`(jitter 付き)を `c.sleep` で待ち、`Retry-After` の無い 429、5xx、network error は指数 backoff で待つ。download の body の途中の失敗は retry しない。`lane.Run`(`internal/lane`)は、job の関数が返るまで枠を空けないため、retry を待つ download も枠を持ち続けていた。

### 第三者 host の `Retry-After` の上限(ユーザーへの確認)

#276 のコメント(PR #291 からの申し送り)の件は、ユーザーに card で判断を仰いだ(2026-09-29 06:53Z。選択肢は「60 秒まで」(推奨)、「5 分まで」、「上限なし」)。回答を待つ間は推奨の案で進め、ユーザーはその「60 秒まで」を選んだ(07:51Z)。Slack の file(`downloadNeedsAuth`)以外の download は、429 の `Retry-After` が 60 秒(`downloadPublicMaxWait` = `maxBackoff`)を超えると待たずに失敗にする。

### 設計(decision log 0068)

- `internal/lane`: job は、request を出している間と body を読んでいる間だけ枠を持つ。失敗したら枠を手放し(`Yield`)、次の request の前に取り直す(`Wait`)。取り直す job は、まだ始まっていない job より先に、始まった順に枠を得る。
- 429(`RateLimited`): lane の上限を半分にする(前回半分にした後に枠を得た job の 429 に限る。`gen`)。`Retry-After` があれば、その間 lane は枠を渡さない(`paused`、timer で再開)。待っている間に始まる job は枠を得ずに始まる。待ちが job の許す長さ(`maxWait`)を超えると、`Wait` は `TooLongError` を返す。
- 成功(`Succeeded`、200 の応答): 半分にした後に枠を得た job の成功 `RestoreAfter`(既定 4)件ごとに上限を 1 戻す(開いたときの上限まで)。
- `Detach`: lane に答えない context。benchmark の `parallel-275` が #275 の lane を再現するのに使う。
- `internal/slack`: `withRetry` に `maxWait` を足し、各 request の前に `waitLane`(既定 `lane.Wait`)を呼ぶ。失敗で `lane.Yield`、429 で `lane.RateLimited`、成功で `lane.Succeeded`。lane の外(Web API など)では何もしない。Slack の file 以外の download は `maxWait` 60 秒で、それを超える `Retry-After` は待たずに `rate limited (429): the server asks to wait <待ち>, over the 1m0s limit` の error にする。HTTP trace は lane の待ちを pacing / retry の待ちに数える。
- 5xx と network error は上限を変えない。body の途中の失敗は retry しない(今のまま)。
- `tools/assetbench`: strategy `parallel`(本 Issue の lane)と `parallel-275`、origin の rate limit(token bucket と一時的な拒否、`Retry-After`)、workload `limited` と `spike`、flag `-restore-after`、報告の 429 の件数。

### 戻し方の benchmark

`limited`(持続する制限)、`spike`(一時的な制限)、`traced`(429 なし)で、`-restore-after` を 0、1、2、4、8、16 にし、各 3 回を測った(表は decision log 0068 の「戻し方の根拠」)。4 が `limited` で最も速く(27.55 秒、429 は 15〜16 件)、`spike` では最も速い 1 と 2(13.7〜13.8 秒)との差が 1 秒以内(14.66 秒)だったため、既定値を 4 のままにした。#275 の lane(`parallel-275`)は `limited` で 21.35 秒と速いが、429 が 69〜73 件と 4〜5 倍になる。`traced` はどの設定も #275 の lane と同じ(1.03〜1.04 秒)。

### test の作り

- `internal/lane`(`ratelimit_test.go`): job を 1 手ずつ動かす puppet で、429 による lane の待ち、1 回の半減、成功による回復、`TooLongError`(jitter を除いた待ちで判定すること)、枠の返却、`Detach`、cancel を確かめる。
- `internal/output`(`ratelimit_test.go`): Issue の完了条件の「`testing/synctest` の仮想時間と fake server」。synctest の bubble の中で、`net.Pipe` の接続に `http.Server` を立て、Slack client と `net/http` の transport を通して取得する(socket は bubble の中で durably blocking にならないため)。429 は他の応答(1 秒)より先に(0.5 秒で)返す。同時に返すと、429 を処理する前に他の download が終わって次の request を出す、同じ時刻の順序が決まらなくなる(最初の版の test で起きた)。
- `internal/output` には、第三者 host の 60 秒(待つ)と 61 秒(失敗する)の境界の test もある(P4)。
- `internal/slack`(`ratelimit_test.go`): 第三者 host の 60 秒の上限(60 秒は待ち、61 秒は失敗する)と、Slack の file と Web API が上限なく待つこと、trace が lane の待ちを数えること。
- 変異の確認(P4 の後): `withRetry` の `lane.RateLimited` を外すと `internal/output` の 3 件が落ちる。429 の 60 秒の判定を外すと `TestRetryAfterOverTheLimit`、`TestAssetsFetchFailsLongRateLimitOfThirdParty`、`TestAssetsFetchWaitsUpToTheLimitOfThirdParty` が落ちる。429 の判定を jitter を足した待ちに戻すと `TestRetryAfterOverTheLimit` と `TestAssetsFetchWaitsUpToTheLimitOfThirdParty` が、lane の判定を jitter を足した再開の時刻に戻すと `TestWaitTooLongHoldsWhatWasAsked` と `TestAssetsFetchWaitsUpToTheLimitOfThirdParty` が落ちる。最後の変異では、`TestAssetsFetchFailsLongRateLimitOfThirdParty` も落ち(lane が断る download の待ちが `2m1s` と示される)、`TestAssetsFetchWaitsUpToTheLimitOfThirdParty` が落ちるかは jitter しだいである(再確認の reviewer の実行で 20 回中 16 回。`TestWaitTooLongHoldsWhatWasAsked` は jitter を固定しているため毎回落ちる)。

### review の指摘への対応(P4)

review cycle `claude-code-e64f479-20260929075701` の指摘 2 件を、どちらも採用して直した(`cdd2b6f`)。

- `[must]`(`internal/slack/client.go`): 60 秒の上限を、`Retry-After` の秒数に jitter(1 秒未満)を足した待ちで判定していたため、`Retry-After: 60` が必ず失敗していた。lane の判定(`tooLong`)も jitter を含む再開の時刻で測っていた。直す前に、足した境界の test で再現した(`Retry-After: 60` の download が `the server asks to wait 1m0s, over the 1m0s limit` で失敗し、取得の test では 8 件中 1 件を保存できなかった)。
  - 直し方: `retryAfter` は `Retry-After` の秒数だけを返し、jitter は呼び出し側が足す(`jitter()`)。`withRetry` は秒数(`asked`)で上限を判定する。lane は、再開の時刻(`until`、jitter を含む)とは別に、429 が求めた待ちの終わり(`asked`)を持ち、`Wait` の `maxWait` はその残りで判定する(`RateLimited(ctx, asked, wait)`)。error の待ちは秒に切り上げて示す(lane の残りが 60.5 秒なら `1m1s`。`1m0s, over the 1m0s limit` にならない)。
  - test: `TestRetryAfterOverTheLimit` に 60 秒(待つ)と 61 秒(`1m1s` で失敗する)を足し、120 秒の error を `2m0s` ちょうどにした。`TestWaitTooLongHoldsWhatWasAsked`(lane、jitter 0.9 秒)と `TestAssetsFetchWaitsUpToTheLimitOfThirdParty`(取得。60 秒では 8 件すべてを保存し、0.5 秒から 60.5 秒の間に request が無い。61 秒では 429 を受けた 1 件と、lane の待ちの間に始まった 2 件が `1m1s` で失敗し、5 件を保存する)を足した。
  - 文書: decision log 0068(検討内容、決定、影響)と `slack-api-usage.md`(「rate limit とリトライ」「file / asset の取得」)に、判定が jitter を含めないこと、60 秒は待つこと、lane の判定が `Retry-After` の残りによることを書いた。
- `[fyi]`(decision log 0068): `limited` の `parallel-275` は、記録した 3 回のうち 2 回でも asset を 1 件保存できていなかった(`Not saved` が 1、1、0。request の件数は 160 件と 429 の件数から 1 を引いた retry の和)。0068 の「戻し方の根拠」の `limited` の段落と、PR description の benchmark に 1 文足した。本 PR の lane は、どの設定でも全件を保存した。
- スコープ外とした指摘と follow-up 候補は無い。出力生成系 3 skill の判断は変わらない(変更は 429 の待ちの判定で、出力に関わらない。固定サンプルを作り直し、差分が無いことを確かめた)。

## 次にやること

- PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。(完了)
- review(P2)と指摘への対応(P4)。(完了)
- 再確認(P5)を P2 と同じ subagent に委譲する。(完了。未対応 0 件で review cycle を完了した)
- review cycle を終えたら PR を Ready for review にする(P6)。(完了)
- 人間: resolve 可とした 2 thread(Claude の cycle)を GitHub の UI で resolve する。
- Codex: Ready for review の PR のクロスレビュー(ユーザーの Codex が行う)。
- 人間: PR を merge する。

## 検証

cloud session の dev container(`docker compose run --rm dev ...`、go1.26.8)で実行した。

- `gofmt -l .`(出力なし)、`go vet ./...`、`go build ./...`: 成功。
- `go test -count=1 ./...`、`CGO_ENABLED=1 go test -race -count=1 ./...`: すべて成功。
- 反復: `internal/lane`、`internal/slack`、`internal/output` の `-count=20 -shuffle=on`、同じ 3 つと `internal/export` の `-race -count=5 -shuffle=on`、同じ 3 つの `GOMAXPROCS=1 -count=5`、`tools/assetbench` の `-count=5` と `-race -count=3`、`cmd/slapex` と `internal/export` の `-count=3`: すべて成功。
- PR #271 の characterization test(`internal/slack/client_retry_test.go`)は変えずに通る。
- cross compile(`CGO_ENABLED=0`、darwin / linux × amd64 / arm64 の `go build ./cmd/slapex`): 成功。
- 固定サンプル(`docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample -time 2026-07-04T16:32:41+09:00`)を作り直し、`doc/samples/` との差分なし。
- benchmark(Issue の「PF-01 の benchmark に 429 を返す origin を足し、挙動を確かめる」): 上記「戻し方の benchmark」。
- `git diff --check`: 問題なし。
- 実 token は使っていない。test と benchmark の通信先は、in-process の fake(`net.Pipe` の server、fake transport、httptest の server)だけ。
- P4 の修正(`cdd2b6f`)の後に、`gofmt -l .`(出力なし)、`go vet ./...`、`go build ./...`、`go test -count=1 ./...`、`CGO_ENABLED=1 go test -race -count=1 ./...`、`internal/lane`、`internal/slack`、`internal/output` の `-count=20 -shuffle=on` と `GOMAXPROCS=1 -count=5`、`internal/export` を加えた `-race -count=5 -shuffle=on`、cross compile、固定サンプルの作り直し(差分なし)、`git diff --check` をやり直し、すべて成功した。PR #271 の characterization test は変わっていない。変異の確認は「test の作り」のとおり。

### 出力生成系 skill の適用判断

- `update-sample-exports`: 使わない。asset の path、保存仕様、表示変換、fixture は変えていない。変更は download の 429 の扱いで、サンプルの fake server は 429 を返さない。固定条件で作り直し、差分が無いことを確かめた。
- `update-readme-preview-screenshots`: 使わない。HTML、CSS、asset は変わらない。
- `update-readme-demo-gif`: 使わない。CLI の操作と表示、fixture のシナリオは変わらない(demo の fake server は 429 を返さない)。PF-03 の後の再録画(PR #291 からの持ち越し)は本 PR と関係しない。

## リスク・ブロッカー

- なし。第三者 host の `Retry-After` の上限は、ユーザーが「60 秒まで」を選んで決まった(2026-09-29)。

## セッションログ

- 2026-09-29: 着手。card でユーザーに上限を確認し、推奨の案で実装、test、benchmark、設計文書、decision log 0068 を書いた。
- 2026-09-29: PR #292 を draft で作成し(07:50Z)、note を採番した(`015192e`)。`progress.md` の PF-04 の PR 欄を `#292` にした(P1)。検証は「検証」のとおり。出力生成系 3 skill は、3 つとも「いつ使うか」に当たらないため適用しなかった(「出力生成系 skill の適用判断」)。
  - `run-issue-task` の報告から引き上げた項目。`number-working-branch-note` の報告の「書き換えた行の一覧」: note の `PR:` 欄(`未作成` → `#292`)、PR description の note のファイル名参照 1 行(`draft_` → `292_`)。title は書き換えていない。「触らずに残した行の一覧」: note の「次にやること」の「PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。」(複合行。`progress.md` の反映は同 skill で完了しない)。採番の後、`progress.md` の反映と合わせて完了にした。PR description と title に触らずに残した行は無い。情報統制チェックで直した箇所は無い。出力生成系 3 skill は呼ばなかった(3 つとも「いつ使うか」に当たらない)。
  - PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。GitHub の write 操作はすべて組み込みの GitHub MCP tool で行った。`gh` は、#272 のコメントの読み取り(`gh api`。trace の集計から status の行を絞るため)にだけ使った。
- 2026-09-29: ユーザーが card で推奨の「60 秒まで」を選んだ(07:51Z)。実装は変えず、decision log 0068 の検討内容、note、PR description に選択を記録した。
- 2026-09-29: review(P2)を subagent に委譲した。review cycle `claude-code-e64f479-20260929075701`、`Reviewed head` `e64f479f93c127fdb127002fdadb50b387f936d4`。指摘 2 件(inline 2、top-level 0)。prefix ごとに `[must]` 1、`[ask]` 0、`[imo]` 0、`[nits]` 0、`[fyi]` 1。`gh` への fallback は無し。完了要約の `Model` は、上位の指示で記載を控えたため `unknown`。指摘が 1 件以上のため P4 に進んだ(P3)。
- 2026-09-29: 指摘 2 件を採用して修正した(P4、`cdd2b6f`。「review の指摘への対応(P4)」)。処置の内訳は、採用し修正した 2 件。スコープ外とした指摘と follow-up 候補は無い。出力生成系 3 skill の判断は変わらない。検証は「検証」の末尾のとおり。
- 2026-09-29: 再確認(P5)を P2 と同じ subagent に委譲した。修正確認済み 2 件(resolve 可)、スコープ外として確認済み 0、対応不要として確認済み 0、未対応 0 件。`gh` への fallback は無し。訂正できなかった metadata の誤りは無し。reviewer は、note の変異の確認の記載に食い違いは無いとしたうえで、lane の判定の変異で落ちる test が記載より 1 件多いこと、取得の境界の test が jitter しだいで落ちることを示した。「test の作り」に書き足した。review cycle を完了し、P6 に進んだ。
- 2026-09-29: 終了時の状態(P6): head `8c6846a` の check runs は 5 件すべて success。note だけの commit を push した後に、PR を Ready for review にする。
