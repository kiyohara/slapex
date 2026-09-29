# 作業ブランチメモ

- ブランチ: `claude/pf-04-issue-276-6d4pux`(cloud session が指定。Issue の推奨ブランチ名は `perf-lane-failure-control`)
- PR: 未作成
- 最終更新: 2026-09-29

## 目的

Issue #276(PF-04)。PF-03(#275)の origin ごとの lane の上で、download が 429 を受けたら、その origin の lane 全体が `Retry-After` の間 request を出さず、同時数の上限を半分にする。他の origin は止めない。5xx と network error、body の読み取り中の失敗の扱いを決める。#192 で共通化した `withRetry` から lane へ 429 と待ちを伝える。Web API の retry の挙動は変えない。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 26 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #291(PF-03)の merge の後に着手した。

## 現在の状況

- 依存の #275(PF-03、PR #291)と #192(PR #271)は merge 済み。main `6f52108`(PR #291 の merge)から作業した。
- 実装、test、benchmark、設計文書、decision log 0068 を済ませ、Issue の「検証」を実行した(「検証」)。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `0a83bf7` と PR #271 の head 時点の記載で、main `6f52108` でも同じだった。`withRetry`(`internal/slack/client.go`)は Web API と download で共通で、429 は `Retry-After`(jitter 付き)を `c.sleep` で待ち、`Retry-After` の無い 429、5xx、network error は指数 backoff で待つ。download の body の途中の失敗は retry しない。`lane.Run`(`internal/lane`)は、job の関数が返るまで枠を空けないため、retry を待つ download も枠を持ち続けていた。

### 第三者 host の `Retry-After` の上限(ユーザーへの確認)

#276 のコメント(PR #291 からの申し送り)の件は、ユーザーに card で判断を仰いだ(2026-09-29 06:53Z。選択肢は「60 秒まで」(推奨)、「5 分まで」、「上限なし」)。回答を待つ間は推奨の案で進め、Slack の file(`downloadNeedsAuth`)以外の download は、429 の `Retry-After` が 60 秒(`downloadPublicMaxWait` = `maxBackoff`)を超えると待たずに失敗にする。

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

- `internal/lane`(`ratelimit_test.go`): job を 1 手ずつ動かす puppet で、429 による lane の待ち、1 回の半減、成功による回復、`TooLongError`、枠の返却、`Detach`、cancel を確かめる。
- `internal/output`(`ratelimit_test.go`): Issue の完了条件の「`testing/synctest` の仮想時間と fake server」。synctest の bubble の中で、`net.Pipe` の接続に `http.Server` を立て、Slack client と `net/http` の transport を通して取得する(socket は bubble の中で durably blocking にならないため)。429 は他の応答(1 秒)より先に(0.5 秒で)返す。同時に返すと、429 を処理する前に他の download が終わって次の request を出す、同じ時刻の順序が決まらなくなる(最初の版の test で起きた)。
- `internal/slack`(`ratelimit_test.go`): 第三者 host の 60 秒の上限と、Slack の file と Web API が上限なく待つこと、trace が lane の待ちを数えること。
- 変異の確認: `withRetry` の `lane.RateLimited` を外すと `internal/output` の 2 件が落ち、60 秒の上限の判定を外すと `TestRetryAfterOverTheLimit` と `TestAssetsFetchFailsLongRateLimitOfThirdParty` が落ちる。

## 次にやること

- PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。

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

### 出力生成系 skill の適用判断

- `update-sample-exports`: 使わない。asset の path、保存仕様、表示変換、fixture は変えていない。変更は download の 429 の扱いで、サンプルの fake server は 429 を返さない。固定条件で作り直し、差分が無いことを確かめた。
- `update-readme-preview-screenshots`: 使わない。HTML、CSS、asset は変わらない。
- `update-readme-demo-gif`: 使わない。CLI の操作と表示、fixture のシナリオは変わらない(demo の fake server は 429 を返さない)。PF-03 の後の再録画(PR #291 からの持ち越し)は本 PR と関係しない。

## リスク・ブロッカー

- 第三者 host の `Retry-After` の上限は、ユーザーの回答待ち(推奨の 60 秒で進めている)。

## セッションログ

- 2026-09-29: 着手。card でユーザーに上限を確認し、推奨の案で実装、test、benchmark、設計文書、decision log 0068 を書いた。
