# 作業ブランチメモ

- ブランチ: `claude/project-thread-94h2xr`(cloud session が指定。Issue の推奨ブランチ名は `fix-flaky-429-retry-test`)
- PR: 未作成
- 最終更新: 2026-09-26

## 目的

Issue #254(FU-18)。`TestCall429RetryAfterWaitsBeforeGivingUp` が CI でまれに失敗し、429 の 1 回が Retry-After でなく指数 backoff の待機として記録される原因を調べて直す。同じ構造の `TestDownloadRetryAfterWaitsBeforeGivingUp` も対象とする。RF-04(#192、retry 制御の共通化)がこれらの test を検証の土台にするため、RF-04 より前に行う。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 8 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 3 件目である。`progress.md` の推奨順(FU-13 → FU-14 → FU-18)とは前後するが、FU-18 の順序の制約は RF-04 より前であることだけで、同じ指示の選定基準(早めにやることで後続の処理に効く作業を先に行う)で先に選んだ。

## 現在の状況

- 依存は無い。main `3ff83c0` から作業した。
- 作業内容 1(診断)を `cb5c145`、作業内容 3(修正)を `1d3e434` で commit した。原因は特定できた(「決定事項」の「原因」)。
- 作業内容 2 の再現の試行のうち、増幅した条件は済んだ(「試した条件と結果」)。増幅しない条件の試行と、Issue の「検証」を実行している。
- PR 未作成。

## 決定事項

### 原因

test の client が `http.DefaultTransport` を共有し、並行して走る別の test の `httptest.Server.Close` が、その idle 接続を閉じることで起きる。net/http 側の競合で、client の retry 制御の誤りではない。

1. `slack.New` の client は `&http.Client{Timeout: 120 * time.Second}` で、Transport を指定しないため `http.DefaultTransport` を使う。package の test は 24 件とも `t.Parallel()` で、test ごとに `httptest.Server` を立て、終わりに `Close` する。
2. `httptest.Server.Close` は、自分の server の接続に加えて `http.DefaultTransport.CloseIdleConnections()` を呼ぶ(Go 1.26.4 の `net/http/httptest/server.go` L268)。
3. net/http の client は、body の無い応答(`Content-Length: 0`。`statusSequenceServer` の 429 と 5xx が当たる)を読むと、接続を idle pool に戻してから、応答を呼び出し側へ渡す(`transport.go` の `persistConn.readLoop`、L2379〜L2395。L2388 で `tryPutIdleConn`、L2395 で応答の送信)。
4. この間に別の test の `Close` が `CloseIdleConnections` を呼ぶと、応答を読み終えた接続が閉じられる(L907、`errCloseIdleConns`)。応答を待つ `persistConn.roundTrip` は接続の close を先に受け取り、応答がまだ送られていないため error を返す(L2928〜L2940)。request はすでに書き込み済みのため、error は `net/http: HTTP/1.x transport connection broken: http: CloseIdleConnections called` になる(L2281)。POST は再送の対象外で、GET も「connection broken」は再送の条件に当たらない(`shouldRetryRequest`)。
5. server はその request を受けて 429 を返しているので、server の request 数は減らない。client の `withRetry` / `downloadRetry` は transport の error として扱い、次の attempt の前に指数 backoff の待機に入る。

CI の失敗(run 36007902809、Go 1.26.4)はこの形に合う。request 数 6 と待機 6 回の assertion が通り、待機 1 回だけが `8.33s`(`backoffWait(4)`)だった。`withRetry` の attempt 3(0 始まりで、4 回目の request)の応答が error になり、attempt 4 の前に backoff が入ったことになる。request 数が減らないことは、server が attempt 3 の request を受けていたことを示す。行番号は Go 1.26.4 の source で確かめた。dev container の Go 1.26.8 でも該当する関数(`readLoop`、`persistConn.roundTrip`、`CloseIdleConnections`)は同一である。

以前の調査(PR #238 のコメント)が「他の httptest server を並行して作っては閉じ続ける」条件で 3,000 回再現しなかったのは、競合の窓が応答の受け渡しの直前の数命令と狭く、1 回の実行あたりの発生確率が低いためと考える。増幅した条件でも、1 回の実行あたり 2.3 万〜8 万分の 1 程度だった(「試した条件と結果」)。

### 直し方

test の問題として直す。

- `newTestClient` の client は、Transport だけを test の server が持つもの(`srv.Client().Transport`)に差し替える。他の test の `Close` はこの transport に触れず、自分の server の `Close` が idle 接続を閉じる。`http.Client` の他の設定(`Timeout`)は `New` のものを保つ。
- package 内で server と通信する test は、custom の transport を使う `TestDownloadSendsAuthForSlackFiles` を除き、すべて `newTestClient` を通るため、1 か所の変更で揃う。
- 診断(作業内容 1)として、`*WaitsBeforeGivingUp` の 2 test で `c.Logf = t.Logf` とした。修正後も残す。別の原因で backoff に落ちた場合も、失敗時の出力に `lastErr` が出る。
- test が確かめる内容(429 ごとに Retry-After の待機に入ること、request 数、待機の回数)は変えていない。skip、再実行、許容範囲の拡大もしていない。
- test の server の応答に `Connection: close` を付ける案と、package の test を並行実行しない案も考えたが、採らない。前者は製品と同じ接続の再利用の経路を test から外し、後者は package の test 全体を遅くする。

client を直さない理由:

- 製品で `CloseIdleConnections` に至るのは、`--demo` の fake server の `Close`(`internal/demo/export.go` の defer)だけで、export が終わった後に呼ばれる。idle に戻した接続を外から閉じる他の経路(`IdleConnTimeout` の timer は 90 秒後、`MaxIdleConns` の超過は最も古い接続が対象)は、戻した直後の接続に当たらない。
- transport の error に指数 backoff で retry するのは decision log 0025 の方針どおりである。
- 製品の client に専用の transport を持たせる案(`http.DefaultTransport` の Clone)も考えたが、test の都合で製品の挙動の前提を変えることになるため採らない。

### 影響の範囲

- `internal/slack` で競合が assertion に効くのは、body の無い 429 に Retry-After を付ける 4 test である: `TestCall429HonoursRetryAfter`、`TestCall429RetryAfterWaitsBeforeGivingUp`、`TestDownloadRetryAfterWaitsBeforeGivingUp`、`TestDownloadRetries`。5xx と Retry-After の無い 429 は、競合が起きても次の待機が同じ backoff になるため assertion は変わらない。4 test とも `newTestClient` を通るため、本修正で揃って直る。
- body のある応答(200 の JSON など)は、body を読み終えてから接続を idle pool に戻すため、この競合は起きない。
- `internal/export` の integration test も `slack.New` の client で `http.DefaultTransport` を使い、fake Slack server を並行して閉じる。body の無い 429 に Retry-After を付け、その待機の進捗表示を確かめる `TestRunIntegrationRateLimitRetryThenSuccess` は、同じ競合で失敗し得る(コードからの推定で、再現は試していない)。export package の test からは `Client` の Transport を差し替えられず、直すには slack package に client を注入する exported な option を足す必要がある。Issue の対象(`internal/slack` の test)から外れ、製品の API を変えるため、本 PR では扱わず follow-up 候補とする(「リスク・ブロッカー」)。

### 試した条件と結果

作業内容 2 の再現を、増幅した条件と、増幅しない条件の 2 段で試した。修正前は `cb5c145`(診断だけ)、修正後は `1d3e434`(診断と修正)の tree を scratch に copy し、Docker Compose の dev container(Go 1.26.8、4 CPU)で実行した。

**増幅した条件**。使い捨ての stress test(build tag 付きで scratch の copy にだけ置き、repository には含めない)で、`*WaitsBeforeGivingUp` の 2 test と同じ手順(`always429` の server に、`newTestClient` の client で `AuthTest` か `Download` を 1 回)を 8 並列で 1 万回ずつ、計 8 万回繰り返した。判定は test と同じ(request 6 回、待機 6 回、どの待機も [1s, 2s))。競合の相手は次のどちらかで作った。

- CloseIdle: goroutine 1 本が `http.DefaultTransport.CloseIdleConnections()` を呼び続ける。`httptest.Server.Close` が他の test に及ぼす処理だけを取り出したもの。
- churn: goroutine 4 本が別の `httptest.Server` を作っては `Close` し続ける。以前の調査と同じ条件で、並行する test の終了に当たる。

| 競合の相手 | 呼び出し | 修正前 | 修正後 |
| --- | --- | --- | --- |
| CloseIdle | `AuthTest`(POST) | 8 万回中 4 回失敗 | 8 万回中 0 回 |
| CloseIdle | `Download`(GET) | 8 万回中 3 回失敗 | 8 万回中 0 回 |
| churn | `AuthTest`(POST) | 8 万回中 1 回失敗 | 8 万回中 0 回 |
| churn | `Download`(GET) | 8 万回中 1 回失敗 | 8 万回中 0 回 |

- 修正前の失敗 9 回は、どれも `lastErr` が `net/http: HTTP/1.x transport connection broken: http: CloseIdleConnections called` だった。
- 失敗の形は、どの attempt の応答が error になったかで変わる。9 回のうち 3 回は CI と同じく `wait[3]` が 8 秒台(`backoffWait(4)`)で、ほかは `wait[1]`、`wait[2]`、`wait[4]` がそれぞれ 2 秒台、4 秒台、16 秒台になる形と、最後の応答が error になり待機が 5 回で終わる形だった。1 回目の応答が error になった場合は、次の待機が `backoffWait(1)`(1 秒 + jitter)で Retry-After の待機と区別できず、test は失敗しない。
- 発生率は、1 回の実行あたり、CloseIdle で約 2.3 万分の 1(16 万回中 7 回)、churn で約 8 万分の 1(16 万回中 2 回)だった。以前の調査の 3,000 回(churn に当たる)は、同じ発生率なら期待される失敗が 0.04 回程度で、再現しなかったことと矛盾しない。
- stress test の接続には `SO_LINGER=0` を設定した。最初の試行では設定せず、TIME_WAIT の socket が 6 万を超えて loopback の port が尽き、競合と無関係の `dial tcp ...: i/o timeout` が混ざったため、設定してやり直した(表はやり直した結果)。

**増幅しない条件**(Issue の作業内容 2 の条件)。repository の test をそのまま実行した(stress test は含めない)。

(作成中)

### その他

- 出力生成系 3 skill は呼ばなかった。変更は `internal/slack` の test だけで、出力 HTML / CSS / assets、sample fixture、CLI 出力、demo のいずれも変えていないため、各 skill の「いつ使うか」に当たらない。
- decision log は作らない。test の修正で、方針の変更は無い。
- 設計文書(`doc/design/`)に test の client の構成を書いた箇所は無く、同期するものは無い。

## 次にやること

- `progress.md` の FU-18 の行を更新し、draft PR を作成して note を採番する。
- 採番の報告から引き上げた項目を「セッションログ」の P1 に残して push する。
- CI を確かめてから review を subagent に委譲する(P2)。

## 検証

(作成中)

## リスク・ブロッカー

- follow-up 候補(起票はユーザーが判断する)
  - `internal/export` の `TestRunIntegrationRateLimitRetryThenSuccess` が同じ競合で失敗し得る(「影響の範囲」)。slack package に `http.Client` か Transport を注入する option を足し、export の test harness が fake server の transport を渡す形で直せる。
- 本修正の効果は確率的な事象に対するもので、CI 上で再発しないことは merge 後の CI の結果で確かめるほかない。修正後も診断の `c.Logf = t.Logf` が残るため、再発した場合は `lastErr` から原因を切り分けられる。

## セッションログ

- 2026-09-26: #206(PR #263)の merge 後、逐次処理の 8 件目として #254 を選び、Issue ごとに新しい thread で進める方式で始めた。依存は無い。branch は main `3ff83c0` から作った。
- 2026-09-26: 作業内容 1 の診断を入れ、増幅した条件(`http.DefaultTransport.CloseIdleConnections` を呼び続ける)で再現し、`lastErr` から原因を特定した。作業内容 3 の修正を入れ、修正前後で再現の条件を比べた。
