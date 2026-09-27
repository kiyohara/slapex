# 作業ブランチメモ

- ブランチ: `claude/serial-issue-15-oikfi1`(cloud session が指定。Issue の推奨ブランチ名は `refactor-slack-retry-policy`)
- PR: #271
- 最終更新: 2026-09-27

## 目的

Issue #192(RF-04)。Web API の呼び出し(`withRetry`)と asset の download(`downloadRetry`)は、retry の回数、backoff と Retry-After の待機、status の判定、進捗表示の文言を別々の loop に重複して持っていた。retry の方針を 1 か所で変えられるよう、小さな非公開の共通処理にまとめる。API の body の読み取り(失敗は retry する)と download の stream(失敗は retry しない)は各呼び出し側の責務に残し、認証 header の決定は統合しない。

#192 には、先行 PR からの申し送りが 2 件ある。本 PR で扱う。

1. PR #264(#254)から: export の結合 test `TestRunIntegrationRateLimitRetryThenSuccess` にも `http.DefaultTransport` の共有による競合が残る。共通化の検証に使う test のため、先に直す。
2. PR #267(#250)から: download が接続の失敗で止まると、retry の通知と取得失敗の警告行に download の URL(upload では Slack private file URL)が出る。ログ文言の維持の例外として、共通化とは別の commit で直し、URL が出ないことを test で確かめる。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 15 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 10 件目である。`progress.md` の推奨順で RF-06(#194)の次に当たる。

## 現在の状況

- 依存(#188)は merge 済み。推奨順で前に置かれた FU-18(#254 / PR #264)も merge 済み。main `0a83bf7` から作業した。
- 申し送り 1(`e5e101b`)、characterization test(`5e6e3ca`)、共通化(`9c7d022`)、申し送り 2(`3c5dc24`)を実装し、Issue の「検証」を実行した(「検証」)。
- PR #271 作成済み。

## 決定事項

### 着手時の現行コード

Issue の背景は main `d5aa977` 時点の記述である。main `0a83bf7` でも構成は同じだった。

- `withRetry(ctx, what, doReq) ([]byte, error)`: Web API 用。loop の中で request を送り、status を判定し、200 なら body を読み切って返す。読み取りの失敗は retry する。
- `downloadRetry(ctx, srcURL) (io.ReadCloser, string, error)`: download 用。`withRetry` と同じ loop を複製し、200 の body を読まずに返す。request は attempt ごとに作り直していた。
- 両方に重複していた処理: 上限超過(`giving up after %d retries`)、backoff の待機と `retrying ... in` の通知、`skipBackoff`、429(Retry-After の待機と `rate limited on ...` の通知)、5xx、想定外の status、200 の分岐。

### 申し送り 1: export の結合 test の transport(`e5e101b`)

- 原因は #254 と同じである。`httptest.Server.Close` は `http.DefaultTransport` の idle 接続を閉じる。net/http は body の無い応答(429、5xx)を返す直前に接続を idle pool に戻す。その間に並行する test が server を閉じると、429 が接続の断(`http: CloseIdleConnections called`)に変わり、Retry-After ではなく backoff で retry される。
- `slack.WithTransport(rt)` option を足し、export の結合 test の client 3 か所(`integration_harness_test.go`、`integration_reuse_test.go`、`message_fetch_test.go`)に fake server 自身の transport を渡した。slack の `newTestClient` も同じ option を使う。test ごとに transport を分ける考え方は PR #264 と同じである。
  - `WithTransport` は `WithSleeper` と同じく test のための option で、internal package の中で export する。client の timeout は保つ(`TestWithTransportKeepsTimeout`)。
- 選ばなかった案: fake server が body の無い fault(429、5xx)の応答に `Connection: close` を付ける案。接続を再利用する経路が test から外れ、fault の形を足すたびに同じ配慮が要る。transport を分ければ、どの応答も他の test の server の Close の影響を受けない。
- 確かめ方: 競合を決定的に起こす注入(`PutIdleConn` の trace hook で `CloseIdleConnections` を呼んで 20ms 待つ。scratch の copy だけで行い、commit しない)で、変更前の harness は 10 回中 10 回失敗し、変更後は 10 回とも成功した。

### 共通化の設計(`9c7d022`)

- `withRetry(ctx, what, send, accept) error` にした。
  - `send` は request を 1 回送る。その error は retry する。
  - `withRetry` は `accept` に渡さない応答をすべて閉じる。
  - `accept` は 200 の応答の body を持つ(閉じるか、呼び出し側へ渡す)。`accept` の error は network error と同じく retry する。
  - retry の回数、backoff と jitter、429 の Retry-After の待機(最終回を含む)、status の判定、進捗表示の文言、cancel の扱いは `withRetry` だけが持つ。
- API(`call`): `accept` で body を読み切って閉じる。読み取りの失敗は従来どおり retry する。request は POST の body を読み切るため attempt ごとに作る。request を作れない error も従来どおり retry する(base URL と method から作るため、通常は起きない)。
- download(`downloadRetry`): `accept` は 200 の応答をそのまま受け取り、body は `Download` が stream して閉じる。stream の失敗は従来どおり retry しない。request は loop の前に 1 回作り、各 attempt で同じ request を送る(GET で body が無く、`withRetry` は次の送信の前に応答を閉じる)。作れない URL は従来どおり retry せずに返す(従来も 1 回目の attempt で返していた)。
- 認証 header の決定は各呼び出し側に残した(API は常に、download は files.slack.com だけ)。
- 進捗表示の文言は、`what` に API は `api <method>`、download は `download` を渡し、変えていない。
- 比較した案: Issue の候補の「attempt closure が retry / abort / done を返して共通 loop を制御する」形。429 / 5xx の判定を各 closure が返すことになり、status の判定と通知の文言の一部が呼び出し側に残る。send と accept に分ける形は、status の判定、待機、文言を 1 か所に置き、呼び出し側の違い(request の作り方と body の扱い)を 2 つの関数に閉じる。API の読み取り失敗の retry、download の stream の返却、Close の所有者はどちらの形でも保てるため、重複を多く減らせる方を選んだ。

### 費用(Issue の完了条件の報告)

- retry の方針を変える箇所: 2 → 1(`withRetry`)。
- 重複していた文字列と呼び出し: `giving up after %d retries`、`rate limited on ...`、`rate limited (429)`、`server error: HTTP %d`、`unexpected HTTP %d`、`backoffWait` の呼び出し、`retryAfter` の呼び出し、`skipBackoff = true` がそれぞれ 2 → 1。retry の通知は 2(`retrying %s` と `retrying download`)→ 1。
- 行数(`internal/slack/client.go`、doc comment を除く): `call` 31 → 39、`withRetry` 51 → 49、`downloadRetry` 51 → 22。3 関数で 133 → 110(-23)。共通化の commit の前後で、doc comment を含むファイル全体は 325 → 313 行(-12)。
- 追加の型と関数: 共通化の commit では 0。申し送り 2 の commit で `withoutURL` を足した。

### 申し送り 2: 接続の失敗で URL を出さない(`3c5dc24`)

- `http.Client.Do` の error は `*url.Error` で、文字列は `Get "<URL>": <原因>` になる。download が接続の失敗(拒否、切断、DNS、TLS、timeout)で止まると、この文字列が retry の通知と取得失敗の警告行に出ていた。upload の URL は Slack private file URL で、`cli-interface.md` の出力制御に反する。変更前のコードで、fake server が接続を切ると両方の行に URL の path が出ることを確かめた。
- `withoutURL` で `*url.Error` を `Get: <原因>` の形にして URL を落とした。原因の error は wrap したまま残す(`errors.Is` / `errors.As` は従来どおり使える)。
  - URL から request を作れない場合の parse の error も同じ形(`parse: <原因>`)にした。申し送りは接続の失敗だけを挙げているが、同じ URL が同じ警告行に出るため合わせた。
  - private file URL かどうかで分けず、download の error からは常に URL を落とす。公開 asset の URL も出なくなるが、判定を誤っても漏れない安全側を取った。どの asset が失敗したかは、従来どおり HTML の表示(ファイル名の下の「取得に失敗しました。」など)と、`--keep-cache` で残した manifest の `source_url` で分かる。
  - manifest の `error` 欄の文字列も同じく変わる。`error` は失敗理由の自由記述で(`cache.md`)、`--reuse-cache` は読まない。`source_url` は従来どおり URL を持つ。
  - Web API の呼び出しの error は変えていない。URL は `https://slack.com/api/<method>` で機密でない(申し送りのとおり)。
- test:
  - slack の `TestDownloadErrorsLeaveOutTheURL`: 接続の失敗で、error の文字列、原因の wrap、6 回の request、5 行の通知の文言を確かめる。作れない URL で、parse の error に URL が無く、request を送らないことを確かめる。
  - export の `TestRunIntegrationAssetConnectionFailureHidesURL`: fake server が毎回接続を切る fault(`dropConnection`)を足した。どのログ行にも URL の path が無いこと、retry の通知 5 行、警告 1 行を確かめる。server 側の request 数は確かめない。net/http は、再利用した idle 接続が切られると GET を 1 回だけ自動で送り直すため、1 回目の attempt が server に 2 回届き得る(実測 7 回)。
  - 2 つの test は変更前のコード(`9c7d022` の `client.go`)で失敗し、変更後に通る。
  - 使い捨ての test(commit しない)で、実際の接続の失敗(閉じた port、`.invalid` の名前解決、https の閉じた port)でも URL の path が出ないことを確かめた。

### characterization test(`5e6e3ca`)

- `internal/slack/client_retry_test.go` を足した。transport を scripted にし、network error や途中で失敗する body を server なしで注入する。Web API(`auth.test`)と `Download` の両方に同じ表を当てる。
  - `TestRetryOutcomes`: network error の後の成功、Retry-After の無い 429 の後の成功、Retry-After 付きの 429 と 500 の後の成功、想定外の 404、503 の繰り返しでの断念、429 の繰り返しでの断念(最終回の Retry-After の待機を含む)。error、request 数、待機、ログの文言、閉じていない body が無いことを確かめる。
  - `TestCallRetriesBodyReadError`、`TestDownloadStreamErrorIsNotRetried`、`TestDownloadSizeLimit`、`TestRetryStopsWhenCanceled`、`TestRetriedRequestsResendTheRequest`。
- 共通化の前のコード(`5e6e3ca`)と後のコード(`9c7d022`)で、test を変えずに通る。
- 変異 14 種を入れ、いずれも test が失敗することを確かめた。
  - 新しい test だけが検出したもの(9 種): 429 の body を閉じない、cancel を「giving up」で包む、`accept` の error を retry しない、API の body を閉じない、download の body を `accept` で読み切る、download の 2 回目以降に token を送らない、download の通知を `api` と名付ける、network error を retry しない、download の body を閉じない。
  - 既存の test も検出したもの(5 種): 想定外の status を retry する、API の request を使い回す、`skipBackoff` を戻さない、backoff の待機を試行の回数でなく backoff の回数から決める、最終回の 429 で待たない。

### Issue の検証表

既存の test を土台に、不足分を足した。

| 場合 | 既存の test | 足した test(不足分) |
|---|---|---|
| 429 + Retry-After | API: `TestCall429HonoursRetryAfter`、export の `TestRunIntegrationRateLimitRetryThenSuccess`。download: `TestDownloadRetries` | 両方で待機の値、通知の文言、body の Close(`TestRetryOutcomes`) |
| Retry-After の無い 429 | API: `TestCall429WithoutRetryAfterBacksOff` | download(`TestRetryOutcomes`) |
| 最終回の 429 | `TestCall429RetryAfterWaitsBeforeGivingUp`、`TestDownloadRetryAfterWaitsBeforeGivingUp`(request 6 回、待機 6 回)、export の `TestRunIntegrationRateLimitExhausted` | 両方で error と body の Close(`TestRetryOutcomes`) |
| 5xx | API: `TestCall5xxBacksOff`、`TestCallGivesUpAfterMaxRetries`。download: `TestDownloadRetries`、export の `TestRunIntegrationTransientServerErrorRecovers`、`TestRunIntegrationAssetDownloadRetriesThenFails` | download の断念を slack 単体で(`TestRetryOutcomes`) |
| network error | なし | 両方(`TestRetryOutcomes`)。download の断念と文言(`TestDownloadErrorsLeaveOutTheURL`、export の `TestRunIntegrationAssetConnectionFailureHidesURL`) |
| 想定外の status | download: export の `TestRunIntegrationAssetDownloadFailure`(404) | 両方で retry しないこと(`TestRetryOutcomes`) |
| context の cancel | なし | 両方で backoff 中と Retry-After の待機中(`TestRetryStopsWhenCanceled`) |
| API の body の読み取り失敗 | なし | `TestCallRetriesBodyReadError` |
| download の stream の失敗 | なし | `TestDownloadStreamErrorIsNotRetried` |
| response の Close | なし | 全 case で閉じていない body が無いこと(`client_retry_test.go` の各 test) |
| サイズ上限 | export の `TestRunIntegrationOversizeAttachmentAtDownload` など | slack 単体で上限の前後(`TestDownloadSizeLimit`) |
| 公開 asset に認証を送らない / files.slack.com に送る | `TestDownloadSavesBody`、`TestDownloadSendsAuthForSlackFiles`、`TestDownloadNeedsAuthOnlyForSlackFiles`、export の service icon(`RejectAuth`) | retry の後も同じ request を送ること(`TestRetriedRequestsResendTheRequest`) |
| 失敗時の継続 | export の `TestRunIntegrationAssetDownloadRetriesThenFails`、`TestRunIntegrationAssetDownloadFailure` | 接続の失敗(`TestRunIntegrationAssetConnectionFailureHidesURL`) |
| ログ文言 | API: export の `TestRunIntegrationRateLimitRetryThenSuccess` | 両方の通知の文言(`TestRetryOutcomes`) |

### 出力生成系 skill

3 skill とも呼ばなかった。

- `update-sample-exports`: 出力 HTML / CSS / assets、DOM、asset の保存 path、demo fixture、`tools/gensample` のいずれも変えていない。gensample で生成し直した ja / en 各 18 ファイルが、commit 済みの `doc/samples/` に一致した。
- `update-readme-preview-screenshots`: sample が変わらないため当たらない。
- `update-readme-demo-gif`: CLI の出力で変わるのは、download の接続の失敗の error の文字列だけである。demo の tape(`tools/demo/demo-ja.tape`)が使う `gensample -serve` の fake server は asset を失敗させないため、録画には出ない。

### decision log

作らない。decision log 0056 の計画(RF-04)の範囲の整理である。申し送り 2 は既存の方針(`cli-interface.md` の出力制御)に実装を揃える修正で、方針は変えていない。

## 次にやること

- draft PR を作成し、note を採番する。(完了)
- `progress.md` の RF-04 の PR 欄に PR 番号を記入し、採番の報告から引き上げた項目を「セッションログ」の P1 に残して push する。
- CI を確かめてから review を subagent に委譲する(P2)。
- 指摘があれば対応し、処置を返信する(P4)。CI を確かめてから再確認を subagent に委譲する(P5)。review cycle の間は draft のまま進める(2026-09-27 06:33Z のユーザーの指示)。
- review cycle の完了後、PR を Ready for review にして、Codex のクロスレビューと merge をユーザーに依頼する。Ready for review から 2 時間経っても Codex のレビューコメントが無ければ、ユーザーに知らせる(06:36Z のユーザーの指示)。
- (人間)Codex のクロスレビュー。指摘があれば agent が `address-comments` で対応する。
- (人間)PR を merge する。

## 検証

2026-09-27、cloud session(project の thread)で、Docker Compose(`docker compose run --rm dev ...`)で実行した。実 token は使わず、架空の token と URL、scripted な transport、export の fake Slack server、demo の fake Slack server だけを使った。

- `gofmt -l .`: 出力なし。`go vet ./...`、`go build ./...`: 成功。
- `go test ./internal/slack ./internal/export ./internal/output`、`go test ./...`: ok。
- `go test -count=5 -shuffle=on ./internal/slack ./internal/export ./internal/output`、`go test -race -count=2 ./internal/slack ./internal/export ./internal/output`(`CGO_ENABLED=1`): ok。
- cross compile(darwin / linux × amd64 / arm64 の `go build ./cmd/slapex`): ok。
- `git diff --check 0a83bf7 HEAD`: 問題なし。
- 申し送り 1: 競合の注入で、変更前 10 回中 10 回失敗、変更後 10 回とも成功(「申し送り 1」)。
- characterization test: 共通化の前後のコードで、test を変えずに ok。変異 14 種をすべて検出(「characterization test」)。
- 申し送り 2: 新しい 2 つの test が変更前のコードで失敗し、変更後に通る。実際の接続の失敗でも URL の path が出ない(「申し送り 2」)。
- 固定 sample: `go run ./tools/gensample -time 2026-07-04T16:32:41+09:00`(`TZ=Asia/Tokyo`)の生成物が、commit 済みの `doc/samples/ja` / `en`(各 18 ファイル)に一致した。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-27: #192 に着手。申し送り 1(`e5e101b`)、characterization test(`5e6e3ca`)、共通化(`9c7d022`)、申し送り 2(`3c5dc24`)、検証。
