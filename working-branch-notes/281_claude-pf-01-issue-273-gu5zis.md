# 作業ブランチメモ

- ブランチ: `claude/pf-01-issue-273-gu5zis`(cloud session が指定。Issue の推奨ブランチ名は `perf-http-trace-and-bench`)
- PR: #281
- 最終更新: 2026-09-27

## 目的

Issue #273(所要時間の最小化 #272 の PF-01)。asset の download と Slack Web API の呼び出しの時間の内訳を記録する trace と、trace を集計する開発用 tool と、取得方式を比べる benchmark を足す。並列化(PF-03)の前に、改善の効果を数字で確かめる土台にする。

- trace は既定で無効にし、内部用途の環境変数 `SLAPEX_HTTP_TRACE` に path を指定したときだけ JSON Lines で書く。URL、header、token、workspace 名、channel 名は記録しない。
- 集計 tool は host 名を出さず、Issue に貼れる集計だけを出す。
- benchmark は in-process の fake origin(TLS、HTTP/2 と HTTP/1.1)で、現行(直列、pacing あり)と pacing なしの直列を比べる。並列方式は PF-03(#275)が足す。
- 実 workspace での計測はスコープ外で、merge 後にユーザーが手元で行う(#272)。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 16 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 11 件目である。次の Issue をユーザーが選んだ(2026-09-27 13:48Z、RF-05 #193 より先)。

## 現在の状況

- 依存(#192 / PR #271)は merge 済み。main `2666ff1` から作業した。
- 実装と Issue の「検証」を終え、draft PR #281 を作成した。review cycle の P4(指摘への対応)を進めている。

## 決定事項

方針の全体は decision log 0063(`doc/design/decision-log/0063-http-trace-and-asset-benchmark.md`)に記録した。実装上の要点は次のとおり。

- 記録の位置: `internal/slack` の `WithTrace(io.Writer)` が client の transport と sleeper を包む。`call` と `Download` は呼び出しごとの記録器(`requestTrace`)を context に載せ、transport が受け取る request ごとに 1 行を作る。retry の各試行と redirect の各段(`Request.Response` で判定)が 1 行ずつになる。`withRetry` と `pace` は変えていない。
- 時刻: 各段(DNS、接続、TLS、接続の取得と再利用、最初の byte、完了)は、その行の開始からの経過時間(µs)で記録する。完了は、body を最後まで読んだ時点と閉じた時点の早いほう(失敗した場合はその時点)とする。`httptrace` の hook は別 goroutine から、request の後にも呼ばれ得るため、行ごとに mutex と書き込み済みの印を持つ。
- 待機の帰属: 最初の request の前の待機を pacing、失敗の後の待機(backoff、Retry-After)をその行の retry 待機とする。待機は sleeper の実測で記録する(sleeper を差し替えた実行では 0 に近くなる)。行は次の request の開始時か、呼び出しの終了時に書くため、retry 待機を含められる。
- URL の識別: client ごとの乱数(32 bytes)を鍵にした HMAC-SHA256 の先頭 8 bytes(16 桁の hex)。error は `canceled` / `timeout` / `dns` / `network` の種類だけを記録し、本文は記録しない。
- asset の kind: `output.Assets.Save` が `slack.WithAssetKind` で context に載せる。`internal/output` の変更はこの 1 か所だけである。
- CLI: `cmd/slapex/httptrace.go` が `SLAPEX_HTTP_TRACE` を読む。空白以外なら 0600 で作成(既存は上書き)し、作れなければ接続の前に exit 4(`reportRunError`)。書き込みの失敗は最初の 1 件を保持し、終了時に警告だけを出す(exit code は変えない)。`--demo` は trace しない(demo は `newSlackClient` を通らない)。
- 集計: `tools/tracereport` は origin を Web API、files.slack.com、Slack CDN(`slack-edge.com` と `slack-imgs.com` の配下)、gravatar、その他に分け、件数、bytes、時間の内訳と実行全体に対する割合を出す。host 名は出さず、origin の数と、origin あたりの件数の分布(`n ×k`)と、HTTP version の分布だけを出す。Web API は method ごと、download は asset の kind ごとの内訳も出す。
- benchmark: `tools/assetbench` は workload(asset の件数・サイズ・origin、origin ごとの handshake と最初の byte の遅延、帯域、`MAX_CONCURRENT_STREAMS`)を preset(`recent`、`small`)か JSON で受け取る。origin は `httptest` の TLS server で、HTTP/2 と HTTP/1.1 を origin ごとに選べる。取得は slapex の client(`slack.WithTransport`、`slack.WithSleeper`)と `output.Assets` で行う。
- 統合 test の比較: trace の有無で export の出力を比べる test は、`.cache/assets_manifest.json` の entry を並べ替えてから比べる。user の avatar は map の順に保存され(`newMessageViewBuilder`)、manifest の entry の順が実行ごとに変わるためである(既存の挙動。下記「follow-up 候補」)。
- timeout の分類(P2 の `[must]`): trace を有効にすると、`http.Client` は知らない transport(`traceTransport`)に送る request を、timeout で context に加えて `Request.Cancel` でも止める。transport は先に気付いたほうの error を返すため、`net/http: request canceled` が混ざる。行に request の deadline を持たせ、deadline を過ぎた後の失敗は error の型によらず `timeout` とした。timeout の error の文(retry の通知、asset の警告、manifest の `error`)が trace の有無で変わり得ることは受け入れ、`cli-interface.md` と decision log 0063 に書いた。止める時点と retry は変わらない。transport を包まない案(hook を context に載せ、redirect の段を `CheckRedirect` で区切る)は、redirect の段の記録が粗くなり、既定の redirect の方針を複製することになるため採らなかった。
- trace ファイルの権限(P2 の `[nits]`): 既存のファイルは中身を置き換え、権限は変えない(`os.WriteFile` と同じ)。`Chmod` で `0600` に揃える案は、device(`/dev/stderr` など)を指定されたときに端末の権限まで変え得るため採らなかった。

### follow-up 候補

- `newMessageViewBuilder`(`internal/export/page.go`)は user の avatar を `resolved.users` の map の順に保存する(bot は ID 順)。このため `.cache/assets_manifest.json` の entry の順と、avatar の取得に失敗したときの警告の順が、実行ごとに変わり得る。本 PR では統合 test の比較だけを順に依存しない形にし、挙動は変えていない。起票の可否はユーザーが判断する。

## 次にやること

- draft PR を作成し、note を採番する。`progress.md` の PR 欄を反映する。(完了)
- review cycle(P2 以降)を進める。
- merge 後: ユーザーが手元の実 workspace で trace を取り、`tools/tracereport` の出力を #272 にコメントする。

## 検証

Docker Compose(cloud session の `compose.yaml:compose.cloud.yaml`、go1.26.8 linux/amd64)で実行した。

- `gofmt -l .`: 出力なし。
- `go vet ./...`: 成功。
- `go build ./...`: 成功。
- `go test ./...`: 全 package 成功。
- `go test -race ./...`(`-e CGO_ENABLED=1`): 全 package 成功。
- 反復: trace の統合 test を 200 回、`internal/slack`、`cmd/slapex`、`tools/tracereport` を各 100 回、`tools/assetbench` の `TestMaxStreams` と `TestUnpacedRun` を 100 回。いずれも成功。trace 関連の test は `-race` でも 50 回成功。
- cross-compile(`GOOS`=darwin / linux、`GOARCH`=amd64 / arm64、`CGO_ENABLED=0`、`go build ./cmd/slapex`): 成功。
- `git diff --check`: 問題なし。
- negative test: token、`Bearer`、URL の path と query、workspace 名、channel 名、channel ID が trace に出ないことを、unit test(`TestTraceLeavesOutSecrets`)と統合 test(`TestRunIntegrationHTTPTrace`)で確かめる。
- trace が既定で無効で、無効時の出力が変わらないこと: `TestTraceIsOffByDefault`、`TestOpenHTTPTraceOff`、統合 test(trace の有無で export のファイルとログが同じ)で確かめる。
- E2E: `tools/gensample -serve` の fake server に対し、fake token と `SLAPEX_API_BASE_URL` で slapex を実行し、`SLAPEX_HTTP_TRACE` を指定した。exit 0、trace は 26 行(Web API 11、download 15)、ファイルは 0600、token と path は含まれなかった。`tools/tracereport` の集計も想定どおりだった。
- 出力生成系 skill:
  - `update-sample-exports`: 適用しない。trace が無効のときの出力は変わらない。`tools/gensample` で生成し直した sample が既存と一致することを確かめた(`-time 2026-07-04T16:32:41+09:00`、`TZ=Asia/Tokyo`)。
  - `update-readme-preview-screenshots`、`update-readme-demo-gif`: 適用しない。利用者に見える出力(HTML、CLI の表示)を変えていない。

### benchmark の結果

`docker compose run --rm dev go run ./tools/assetbench -runs 3`(workload `recent`、cloud の dev container、4 CPUs)。origin は in-process のモデルで、実測ではない。第三者 origin のサイズ、遅延、帯域、HTTP version は仮定である。

Workload "recent": 56 assets (8.5 MB) from 25 origins (files.slack.com 4 on 1, Slack CDN 14 on 4, gravatar 5 on 1, other 33 on 19); HTTP/2 origins 19, HTTP/1.1 origins 6.

| Strategy | Run | Wall | Pacing wait | Retry wait | Connect | First byte | Transfer | Other | Requests | New conns | Not saved | Speedup |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| current (serial, 1 s pacing) | 1 | 55.223 s | 44.029 s | 0.000 s | 1.862 s | 7.856 s | 1.459 s | 0.017 s | 56 | 25 | 0 | 1.0× |
| current (serial, 1 s pacing) | 2 | 55.227 s | 44.040 s | 0.000 s | 1.855 s | 7.856 s | 1.459 s | 0.016 s | 56 | 25 | 0 | 1.0× |
| current (serial, 1 s pacing) | 3 | 55.225 s | 44.020 s | 0.000 s | 1.862 s | 7.857 s | 1.468 s | 0.017 s | 56 | 25 | 0 | 1.0× |
| unpaced (serial, no pacing) | 1 | 11.176 s | 0.000 s | 0.000 s | 1.851 s | 7.852 s | 1.457 s | 0.016 s | 56 | 25 | 0 | 4.9× |
| unpaced (serial, no pacing) | 2 | 11.196 s | 0.000 s | 0.000 s | 1.862 s | 7.852 s | 1.464 s | 0.017 s | 56 | 25 | 0 | 4.9× |
| unpaced (serial, no pacing) | 3 | 11.179 s | 0.000 s | 0.000 s | 1.853 s | 7.853 s | 1.455 s | 0.017 s | 56 | 25 | 0 | 4.9× |

- 現行方式の約 8 割(44 s / 55 s)が pacing の待機である。pacing を外した直列では、最初の byte までの待ち(7.9 s)が最大で、接続の確立(1.9 s、新しい接続 25)と転送(1.5 s)が続く。直列のままでは、origin の遅延がそのまま積み上がる。
- PF-03 は、この workload に並列方式を足して比べる。同時数の上限は、この表と実 workspace の trace から決める。

## リスク・ブロッカー

- PF-01 は `cmd/slapex/main.go` を触るため、RF-05(#193)とは直列にする(#273)。RF-05 は本 PR の merge の後に着手する予定である。
- benchmark の絶対値は workload のモデルに依存する。実 workspace の trace(merge 後にユーザーが取得)で、モデルの妥当性を確かめる必要がある。

## セッションログ

- 2026-09-27: 着手。Issue #273、#272、#275、#277、PR #271 の引き継ぎ(`/mnt/project-files` の報告)を読んだ。
- 2026-09-27: trace、`SLAPEX_HTTP_TRACE`、`tools/tracereport`、`tools/assetbench`、文書(decision log 0063、`cli-interface.md`、`architecture.md`、`progress.md`)を実装した。Issue の「検証」を Compose で実行し、benchmark を計測した。
- 2026-09-27 P1: draft PR #281 を作成し、note を採番した(`dd8c98c`)。`progress.md` の PF-01 の PR 欄を反映した。検証は「検証」のとおりすべて ok。出力生成系 3 skill は呼ばなかった(「検証」の「出力生成系 skill」)。`number-working-branch-note` の報告から引き上げた項目: 書き換えた行は、PR description の note のファイル名参照 1 行(`draft_` → `281_`)だけで、note の stale 表現と完了タスク行、title は書き換えていない。触らずに残した行は、note の「次にやること」の「draft PR を作成し、note を採番する。`progress.md` の PR 欄を反映する。」(複合行。`progress.md` の反映は同 skill の範囲外)と、「現在の状況」の「実装と Issue の「検証」を終え、draft PR を作る段階である。」(定型に当てはまらない)の 2 行。PR description と title には無い。どちらの行も、この P1 の記録で orchestrator が更新した。ほかに note の `PR:` 欄に `#281` を記入した。情報統制チェックで直した箇所は無い。PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。
- 2026-09-27 P2 / P3: review cycle `claude-code-37d0fdd-20260927145527`、Reviewed head `37d0fdd`。指摘 2 件(inline 2 件、top-level 0 件。`[must]` 1 件、`[nits]` 1 件で、`[ask]`、`[imo]`、`[fyi]`、prefix 無しは 0)。merge 前に直す指摘(`[must]`)が 1 件あった(trace を有効にすると timeout の約半数が `network` と記録され、timeout の error の文も変わる)。`[nits]` は既存の trace ファイルの権限。1 件以上のため P4 に進んだ。依頼した 8 観点は、この 2 件を除いて妥当とされた。subagent は PR head の複製(作業ツリーの外)で、timeout、redirect と Authorization、最後の試行の Retry-After、cancel の実験 test、`gensample -serve` への E2E、固定時刻の sample の再生成、`assetbench -runs 1`(PR の表と一致)を実行した。完了要約の `Model` は、上位の指示が review のコメントを対象から外すと確認できなかったため `unknown` とされた。`gh` への fallback、停止、訂正できなかった誤りはなし。
