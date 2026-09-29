# 作業ブランチメモ

- ブランチ: `claude/issue-275-tu25zj`(cloud session が指定。Issue の推奨ブランチ名は `perf-parallel-asset-lanes`)
- PR: #291
- 最終更新: 2026-09-29

## 目的

Issue #275(PF-03)。所要時間の最小化(#272)に向けて、asset の download の pacing を外し、PF-02(#274)の取得の計画を origin(scheme + host + port)ごとの lane で並列に取得する。出力(HTML、assets、manifest、cache、stderr)は直列のときと変えない。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 25 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。ユーザーが 2026-09-29 に実 workspace の trace の集計を #272 にコメントした(PF-01 の申し送り)後に着手した。

## 現在の状況

- 依存の #273(PF-01、PR #281)と #274(PF-02、PR #286)は merge 済み。推奨の前提の #245(PR #287)も merge 済み。main `7b22e92`(PR #290 の merge)から作業した。
- 実装、test、benchmark、設計文書、decision log 0067 を済ませ、Issue の「検証」を実行した(「検証」)。反復実行で見つかった test の不安定さ 5 件の原因を直した(コード 2 件、test の前提 3 件。「反復実行で見つかった不安定さ」)。
- PR #291 を draft で作り、note を採番した。`progress.md` の PR 欄を反映した(P1)。
- review(P2)の指摘 3 件(`[ask]` 1、`[imo]` 2)にすべて対応した(P4。「review の指摘への対応」)。`[ask]` はユーザーに判断を仰ぎ、回答を待つ間は推奨の案で進めた。
- 次は CI を確かめてから、再確認を subagent に委譲する(P5)。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `0a83bf7` 時点の記載で、main `7b22e92` でも同じだった。`slack.Client.Download` は `c.pace(ctx, "download")` で開始間隔 1 秒を待ち、Web API と同じ `http.Client`(`Timeout` 120 秒)で送っていた。`output.Assets.Fetch` は計画を計画の順に直列で取得していた(0064)。

### #272 の trace(2026-09-29、ユーザーの実 workspace)

81 request、71.785 秒。download 56 件(redirect 3 段を含めて 59 request)が 27 origin(HTTP/2 24、HTTP/1.1 3)に分かれ、1 origin あたり最大 10 request。download の pacing の待ちは 40.0 秒(全体 51.406 秒のうち Web API 11.359 秒を除いた分)で最大の項。download の request 自体は直列で計 15.8 秒。files.slack.com の最初の byte は 1 件あたり約 0.9 秒(thumbnail 約 0.71 秒、原本 約 1.07 秒)で、転送(3.9 MB で 0.08 秒)よりずっと長い。

### 設計(decision log 0067)

- download は pacing しない。Web API の method ごとの平準化は変えない(`internal/slack` の test が fake sleeper の記録で確かめる)。
- `internal/lane`(新規): origin ごとの lane。先導 1 本が `GotConn` で接続を得てから残りを流し、接続が HTTP/2 なら 16、そうでなければ 6 まで。全体 64。サイズの分かるものは大きい順、4 MiB 以上は lane あたり 4 まで。値は `lane.Defaults`。
- `output.Assets.Fetch`: reuse の copy を計画の順に先に済ませ、残りの download を lane で並列に走らせる。各 download の retry の通知(`slack.WithNotices`)と警告を保持し、計画の順に出す(`heldNotices`)。context が終わったら新しい download を始めず、保持した通知と警告は捨てる。
- `internal/slack`: download 専用の `http.Client` と `http.Transport`(`NewDownloadTransport`、`MaxIdleConnsPerHost` を 16)。`WithTransport` は両方に渡す。全体の timeout をやめ、試行ごとの watchdog で、request を送り終えてから最終の応答 header まで 30 秒、body が進まない 30 秒で打ち切る(timeout の `net.Error`)。Slack の file(files.slack.com)以外の download は、1 試行を 5 分で打ち切る(「review の指摘への対応」)。watchdog が打ち切った後に届いた応答は使わず、打ち切りとして扱う(「反復実行で見つかった不安定さ」)。HTTP trace は download の client も包む。
- `cmd/slapex`: export の実行中(token の入力の後から `export.Run` が返るまで。`--demo` も同じ)の SIGINT / SIGTERM で、printer を止め(`ui.Printer.Mute`)、context を cancel し、`Run` が返った後に同じ signal で終わる。猶予 5 秒、2 回目の signal で即座に終わる(1 回目の処理の途中に届いた 2 回目も)。起動時から無視されている signal は無視したまま。
- `tools/assetbench`: strategy を `paced`(benchmark の側で開始間隔 1 秒を待つ直列)、`unpaced`、`parallel` にし、lane の上限の flag(`-h2`、`-h1`、`-total`、`-large`、`-large-size`)、報告の Peak と Longest、fake origin の redirect、workload の `traced`(既定)と `heavy` を足した。Issue のコメント(PR #288 からの申し送り)に従い、`TestUnpacedRun` の pacing の検査を `>= time.Millisecond` で落ちるように緩め、`TestParallelRun` も同じ検査にした。

### Issue の作業内容から変えた点

- `http.HTTP2Config.StrictMaxConcurrentRequests` を有効にしない。有効にすると、fake server の test(`TestAssetsFetchOverHTTP`)が止まった。Go の HTTP/2 client の不具合(golang/go#70809、open)で、server の `MAX_CONCURRENT_STREAMS` を超える数の request が 1 本の接続を待つと進まなくなる。使い捨ての再現(`MAX_CONCURRENT_STREAMS` 4、16、128 にそれぞれ 2 倍の request)で、3 回とも止まった。既定のまま(stream の上限で追加の接続を開く)でも、lane の上限(16)が Slack の host の上限(128)より小さいため、先導と合わせて接続は 1 本のまま。`TestAssetsFetchOverHTTP` に stream を 1 に絞った server の場合を足し、strict に戻すとこの test が止まる(timeout で落ちる)ことを確かめた。

### 反復実行で見つかった不安定さ

Issue の「検証」の反復実行(`-count=20`、`-race -count=5` など)で、新しい test が稀に落ちた。次の 5 件の原因を直した。前の 2 件はコードの不具合、後の 3 件は test の前提の誤りである。

- 応答 header の待ちの打ち切りが効かない(`TestDownloadHeaderWait` と `TestTraceTimeoutClass` の HTTP/1.1 の場合)。watchdog が試行を cancel した後に届いた応答を、download がそのまま使っていた。net/http は cancel と競合して届いた応答を返すことがあり、test の server は request の cancel を受けて handler を終えるため、空の 200 を返しうる。`sendDownload` は、応答を受け取った時点で watchdog が打ち切っていれば、応答を閉じて打ち切りの error にする。HTTP trace も、打ち切りで cancel された request の応答を `timeout` と記録する。cancel の後に応答を返す transport で決定的に再現する test(`TestDownloadHeaderWaitBeatsLateResponse`)を足し、どちらの修正を外しても落ちることを確かめた。
- body の停止の打ち切りが応答 header の待ち(30 秒)に置き換わる(`TestDownloadStallWait` の HTTP/2 の場合)。HTTP/2 の transport は、request の header を送ってから送信完了(`WroteRequest` の trace)を知らせるまでの間に応答を受け取りうる(Go の `h2_bundle.go` の `writeRequest`)。その通知が body の待ちを掛けた後に来ると、応答 header の待ちを掛け直していた。watchdog は、body の待ちを掛けた後は `arm` と `disarm` を受け付けない(`armBody`)。応答の後に送信完了を知らせる transport で決定的に再現する test(`TestDownloadStallWaitAfterLateWroteRequest`)と、`TestWatchdog` の規則を足し、修正を外すと落ちることを確かめた。既定値では両方の待ちが 30 秒のため、実際の影響は error の文言だけである。
- HTTP/1.1 の origin の接続数が上限を超える(`TestAssetsFetchOverHTTP`、`TestRunOverHTTP`)。net/http は、空いた接続に追い越された dial も最後まで張り、idle の接続として残す。同時数(lane の上限)は守られており、接続数に上限を置いた test の前提が誤りだった。HTTP/1.1 の接続数の検査を外し、同時数の検査(上限以下かつ 2 以上)は残した。HTTP/2 の接続が 1 本であることの検査は変えていない。
- `TestDownloadHeaderWait` の試行の数え方: server の側で数えると、最後の試行を server が受け取る前に client が諦めて返りうる。client の側(transport)で数えるようにした。
- `TestTraceTimeoutClass` の trace の行数: body が止まる download にも応答 header の待ち 20 ms を掛けていたため、負荷の高いときに header の前の打ち切りと retry が起き、行が増えた。body が止まる download は、応答 header の待ちを 10 秒にした別の client で送るようにした。応答 header の待ちの打ち切りは、同じ test の header が来ない download で確かめている。

### review の指摘への対応(P4)

review cycle `claude-code-a85f4c7-20260929031415` の指摘 3 件(すべて inline)に対応した。3 件とも採用して修正した(修正 commit `ffb9337`)。

- `[ask]` 全体の timeout が無くなり、少しずつ返し続ける download(特に第三者 host の URL preview)が止まらない: ユーザーに判断を仰ぐ card を出した(2026-09-29 03:45Z。選択肢は「5 分で打ち切る」(推奨)、「記録だけ」、「全部に上限」)。回答を待つ間は推奨の案で進め、Slack の file(`downloadNeedsAuth`。認証 header を付ける判定と同じ)以外の download は、1 試行を 5 分(`downloadPublicTimeout`)で打ち切るようにした。watchdog に試行の上限(`limit`)を足した。5 分は main の 1 試行の上限(120 秒)より長く、5 MiB(URL preview 画像などの上限)を 5 分で運ぶのは約 17 KiB/s にあたる。ユーザーが別の案を選んだ場合は、その案に合わせて直す。
- `[imo]` 1xx の中間応答で応答 header の待ちが外れる: 提案の `Got1xxResponse` で張り直す方法ではなく、最初の byte(`GotFirstResponseByte`)で待ちを外すのをやめた。応答 header の待ちは、`Do` が返る(最終の応答 header が揃う)まで続く。1xx の header の合計の大きさを抑える `net/http` の制限は、`Got1xxResponse` を設定しないため外れない。header の途中で止まる server も同じく打ち切れる。redirect の次の request が接続を得るまでの時間を数えないよう、`GetConn` で待ちを止め、送り終えた時点(`WroteRequest`)で測り直す。
- `[imo]` 1 回目の signal から `reset()` までに届いた 2 回目の signal が読まれない: 提案どおり、`reset()` を `mute()` より先に呼び、内側の select で 2 回目の signal を読んで終わるようにした。signal の channel の容量も 2 にし、watch が 1 回目を読む前に 2 回目が届いても落とさない。`TestInterruptEndsProcessBySignal` の 2 回の SIGINT の間の sleep は残した。Go の runtime は、届いて未処理の同じ signal に次の同じ signal をまとめるため(`runtime/sigqueue.go` の `sigsend`)、1 回目が届く前に 2 回目を送ると 1 回に数えられうる。これは watch の側では直せない。
- 足した test: `TestInterruptWatchSecondSignal`(`mute()` が止まっていても `reset()` が先に済むこと、`reset()` の前に届いた 2 回目の signal で終わること)、`TestDownloadHeaderWaitPastFirstByte`(103 の後に何も返さない server を HTTP/1.1 と HTTP/2 で、header の途中で止まる server を HTTP/1.1 で打ち切る)、`TestDownloadHeaderWaitSkipsRedirectConnect`(redirect 先への接続に header の待ちより長くかかっても retry しない)、`TestDownloadPublicTimeout`(Slack の file 以外は少しずつ返し続けても 5 分の上限(test では 50 ms)で打ち切り、retry しない。Slack の file は打ち切らない)、`TestWatchdog` の試行の上限の規則。既定値の検査(`TestWithTransportKeepsTimeout`)に `publicTimeout` を足した。
- 変異の確認: `reset()` を `mute()` の後に戻す、内側の select から 2 回目の signal を外す(どちらも `TestInterruptWatchSecondSignal` が落ちる)、最初の byte で待ちを外す(`TestDownloadHeaderWaitPastFirstByte` の 3 件が落ちる)、`GetConn` で待ちを止めない(`TestDownloadHeaderWaitSkipsRedirectConnect` が retry で落ちる)、試行の上限を外す、Slack の file にも上限を掛ける(`TestDownloadPublicTimeout` のそれぞれの場合が落ちる)。以前の変異(printer を止める、cancel、reset、止まった後の signal での終了のそれぞれを外す)も、順を変えた後のコードで落ちることを確かめ直した。
- 設計文書: decision log 0067(候補、検討内容、決定、上限値の根拠、影響、後から見直す条件)、`index.md` の 0067 の行、`slack-api-usage.md` の「file / asset の取得」、`architecture.md` の `internal/slack` の行、`cli-interface.md` の `SLAPEX_HTTP_TRACE` の説明を更新した。
- スコープ外とした指摘は無く、follow-up Issue の候補も無い。
- 出力生成系 3 skill の再判断: 変わらない。変更は download の打ち切りと中断の処理だけで、出力、サンプル、画面は変わらない。`update-readme-demo-gif` は P1 の判断(該当するが「ローカルで要再生成」)のまま。

### 上限値の決定(benchmark)

`docker compose run --rm dev go run ./tools/assetbench`(cloud session の dev container、go1.26.8、linux/amd64、4 CPUs、2026-09-29)。origin は in-process のモデルで、実測ではない。上限を 1 つずつ変えた結果、大きいファイルの件数と閾値、帯域を変えた場合の表は、decision log 0067 の「上限値の根拠」にある。

| workload | 方式 | 時間 | pacing の待ち | 新しい接続 | 同時の最大 |
|---|---|---:|---:|---:|---:|
| `traced` | `paced`(変更前) | 55.38 秒 | 39.50 秒 | 27 | 1 |
| `traced` | `unpaced` | 15.86 秒 | 0 | 27 | 1 |
| `traced` | `parallel` | 1.07 秒 | 0 | 27 | 45〜49 |
| `heavy` | `paced`(変更前) | 228.61 秒 | 84.62 秒 | 19 | 1 |
| `heavy` | `unpaced` | 143.35 秒 | 0 | 17 | 1 |
| `heavy` | `parallel` | 15.47 秒 | 0 | 17 | 63〜64 |

- `traced` は #272 の trace(2026-09-29)に合わせた workload、`heavy` は files.slack.com に原本と添付が多い仮定の workload。`traced` は各方式 3 回、`heavy` は `paced` と `unpaced` が 1 回、`parallel` が 3 回の中央値。`traced` の `paced` と `unpaced` は、trace の download の pacing の待ち(40.0 秒)と request の時間(15.8 秒)に近い。
- HTTP/2 16、HTTP/1.1 6、全体 64 は Issue の目安の値で、`traced` では変えても差が無く、`heavy` では HTTP/2 16 と全体 16 で頭打ちになった。
- 大きいファイルの件数は Issue の目安(2〜4)で最も速い 4 とし、閾値は当初の 1 MiB から 4 MiB に変えた(`heavy` で 17.45 秒から 15.47 秒。files.slack.com の帯域を 1/4 と 4 倍にした場合も 4 MiB が最も速い)。大きいファイルの上限を実質なくすと(1 MiB で 16 件)、原本と添付が先に帯域を使い、thumbnail が後ろに残って遅くなることを trace で確かめた。
- 再現: `-workload traced` / `heavy`、`-strategies`、`-runs`、上限の flag(`-h2`、`-h1`、`-total`、`-large`、`-large-size`)。帯域の変種は、`-print-workload` で書き出した `heavy` の JSON の files.slack.com の `bytes_per_sec` を変えて `-workload` に渡した。

### test

- `internal/lane`(新規): origin の正規化(`TestOrigin`)、先導が接続を得るまで 1 件で、得た後は HTTP/2 なら 16、それ以外は 6 まで(`TestRunLeadsEachOrigin`)、先導が接続を得ずに終わった lane(`TestRunLeaderWithoutConnection`)、全体の上限と lane の順番(`TestRunTotalLimit`)、大きいファイルの上限(`TestRunLargeJobs`)、開始順(`TestRunStartOrder`)、cancel(`TestRunCanceled`)。`TestRunOverHTTP` は httptest の server で、HTTP/2 の origin が接続 1 本を上限まで共有し、HTTP/1.1 の origin が上限まで同時に走ることを確かめる。
- `internal/slack`: `download_test.go`(新規)で、download が pacing で待たず、Web API の同じ method の呼び出しは待つこと(fake sleeper の記録。`TestDownloadsAreNotPaced`)、`NewDownloadTransport` の設定、応答 header と body の待ちの打ち切りと、遅くても進む body は打ち切らないこと、打ち切りの後に届いた応答と、応答の後に届いた送信完了の通知の扱い(`TestDownloadHeaderWaitBeatsLateResponse`、`TestDownloadStallWaitAfterLateWroteRequest`)、watchdog の規則、`WithNotices` を確かめる。`TestWithTransportKeepsTimeout` は、両方の client が渡した transport を使い、Web API の timeout と download の待ちを保つことを、`TestTraceTimeoutClass` は、打ち切りが HTTP/1.1 と HTTP/2 の両方で trace の `timeout` になることを確かめる。既存の認証 header の test は、download の client を差し替えるようにした。
- `internal/output/fetch_test.go`(新規): `TestAssetsFetchOverHTTP` は、`slack.Client` と `NewDownloadTransport` を実際の server に向け、HTTP/2 の origin の接続が 1 本で、同時数が上限以下かつ 2 以上であること、stream を 1 に絞った HTTP/2 の server でも止まらないこと、HTTP/1.1 の origin の同時数が上限以下であることを確かめる。`TestAssetsFetchSendsTokenOnlyToSlackFiles` は、並列の取得で token を files.slack.com にだけ送り、Slack の公開 asset の host、第三者の host、名前の前方だけが似た host、Slack の file の redirect 先には送らないことを確かめる(positive と negative)。ほかに、通知と警告を計画の順に出すこと(`TestAssetsFetchPassesNoticesInPlanOrder`、`TestHeldNoticesPassInPlanOrder`)と、cancel で新しい download を始めず一時ファイルが残らないこと(`TestAssetsFetchStopsWhenCanceled`)。
- `internal/export/integration_lanes_test.go`(新規): `TestRunIntegrationParallelAssetsMatchSerial` は、全種類の経路を含む場面を、download をばらばらの遅延(seed ごとに 20 ms 未満)で終わらせて 5 回取得し、HTML、assets、manifest、cache、stderr が 1 件ずつ取得した場合と一致することを確かめる。`TestRunIntegrationCanceledDuringAssets` は、download 中の cancel で export が止まり、一時ファイルが残らないことを確かめる。既存の結合 test と `internal/output` の test は、download の順を問わず、警告を計画の順で確かめるようにした。
- `internal/ui`: `TestMuteDropsLaterOutput`。
- `cmd/slapex/interrupt_test.go`(新規): 中断の watch の単体 test 4 件と、子 process に SIGINT / SIGTERM を送る `TestInterruptEndsProcessBySignal`(一時ファイルを消し、以後は何も出さず、signal で終わる。止まらない export は 2 回目の signal で終わる)。
- `tools/assetbench`: `TestCurrentRunPaces` を `TestPacedRun` にし、`TestParallelRun`(先導で接続 1 本、redirect、Peak が 2 以上)と `TestTracedWorkload`(trace の件数と origin の内訳)を足した。
- review の指摘への対応で足した test と変異の確認は「review の指摘への対応(P4)」にある。
- 変異を入れて test が落ちることを確かめた: strict に戻す(`TestAssetsFetchOverHTTP` の stream を 1 に絞った場合が止まる)、通知の保持を外す(並列と直列の比較が落ちる)、中断の hook を 1 つずつ外す(printer を止める、cancel、signal の既定の扱いへの戻し、止まった後の signal での終了。どれも `TestInterruptEndsProcessBySignal` などが落ちる)、打ち切りの後の応答を使う(`TestDownloadHeaderWaitBeatsLateResponse` が `err = nil` で落ちる)、trace の分類を外す(同じ test が trace の行で落ちる)、body の待ちの後の `arm` または `disarm` を受け付ける(`TestDownloadStallWaitAfterLateWroteRequest` または `TestWatchdog` が落ちる)。

### 設計文書・decision log・progress.md

- decision log 0067(`0067-parallel-asset-lanes.md`)を作り、`index.md` に行を足した。0025(download の平準化をやめた)、0063(benchmark の strategy と workload)、0064(`Fetch` の並列化と reuse の判定の時点)に追記し、`index.md` の 0025 と 0064 の行を更新した。
- `slack-api-usage.md`: 「rate limit とリトライ」に download を平準化しないこと、「file / asset の取得」に lane、上限、待ち時間の打ち切り、通知の順、中断を書いた。
- `architecture.md`: `internal/lane` の行、`internal/slack`・`internal/output`・`internal/export`・`internal/ui`・`cmd/slapex` の責務と依存、Assets 工程と中断の説明、`tools/assetbench` の説明を更新した。
- `cli-interface.md`: 「exit code」に中断の扱いを書いた。`SLAPEX_HTTP_TRACE` の説明の、120 秒の timeout の error の文を Web API に限り、中断した場合の `run` の行を書いた。
- `progress.md` の PF-03 の行を `done(PR merge後)`、次にやることを「merge 後にユーザーが手元で trace を取り直し、集計を #272 にコメント」にした。PR 欄は採番後に記入する。

### 出力生成系 3 skill の適用判断

- `update-sample-exports`: 使わない。`internal/output` と `internal/export` を変えたが、asset の path、保存仕様、表示変換は変えていない。固定条件(`TZ=Asia/Tokyo`、`-time 2026-07-04T16:32:41+09:00`)でサンプルを作り直し、`doc/samples/ja` と `doc/samples/en` との差分が無いことを確かめた(「検証」)。
- `update-readme-preview-screenshots`: 使わない。HTML、CSS、asset、サンプルは変わらない。
- `update-readme-demo-gif`: 該当する。録画(`tools/demo/demo-ja.tape`)は、実バイナリを `gensample -serve -asset-delay 250ms` の fake server に向けるため、これまでは download の pacing(開始間隔 1 秒)で asset 15 件の取得に 14 秒以上かかっていた。この PR で 1 origin に 6 本の並列になり、同じ条件の E2E では export 全体が 17.3 秒から 3.8 秒になった(「検証」)。録画の長さ、進捗表示が映る時間、完了表示の所要秒数が変わる。cloud session では録画できないため再生成せず、「ローカルで要再生成」として未検証事項に残す(`doc/guidelines/cloud-session-guidelines.md`)。進捗表示を映すための `-asset-delay` の見直しも、再録画のときに判断する。

## 次にやること

- PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。(完了)
- review(P2)と指摘への対応(P4)。(完了)
- CI を確かめてから、再確認を subagent に委譲する(P5)。未対応が残れば P4 に戻る。
- `[ask]` への回答(Slack 以外の download の時間の上限)を受けたら、その案に合わせる。
- merge 後: ユーザーが手元で PF-01 の trace を有効にして実 workspace を export し、集計を #272 にコメントする(手順は project の共有フォルダの `perf-trace/local-trace-prompt.md`)。

## 検証

すべて cloud session の dev container(`docker compose run --rm dev ...`、go1.26.8、linux/amd64、4 CPUs)で実行した。host の `go` は使っていない。

- `gofmt -l .`: 出力なし。
- `go vet ./...`、`go build ./...`: 成功。
- `go test -count=1 ./...`: 成功。
- `CGO_ENABLED=1 go test -race -count=1 ./...`: 成功。
- 反復: `-count=5 -shuffle=on`(lane、output、slack、export、ui、cmd/slapex、assetbench)、`GOMAXPROCS=1 -count=3`、並列と直列の比較と中断の結合 test の `-count=20`、`-race -count=5`(lane、output、slack、export、cmd/slapex)。さらに負荷をかけた反復(`-count=20` から `-count=30`、`-race -count=10` から `-race -count=15` を、検証の実行と同時に)を、不安定さを直すたびに繰り返し、最後の 2 回はすべて成功した。途中で見つかった不安定さと直し方は「反復実行で見つかった不安定さ」にある。
- cross compile(`CGO_ENABLED=0`、darwin / linux × amd64 / arm64): 成功。
- 認証 header の test: `TestAssetsFetchSendsTokenOnlyToSlackFiles`(並列の取得での positive と negative)と、`internal/slack` の既存の `TestDownloadSendsAuthForSlackFiles`、`TestDownloadNeedsAuthOnlyForSlackFiles`、`TestCallSendsFormEncodedRequestWithAuth` が成功。
- 固定サンプル: `TZ=Asia/Tokyo go run ./tools/gensample -time 2026-07-04T16:32:41+09:00` で作り直し、`doc/samples/ja` と `doc/samples/en`(各 18 ファイル)と差分なし。`update-sample-exports` は使わない(「出力生成系 3 skill の適用判断」)。
- `git diff --check`(新規ファイルを含む): 問題なし。
- 実 token は使っていない。test の token は架空値で、通信先は httptest の server、in-process の fake、`roundTripFunc` だけで、実際の Slack と第三者の host には接続しない。
- E2E: main と本ブランチの実バイナリを、`gensample -serve -lang ja -asset-delay 250ms`(架空の fixture)に向けて export した。所要時間は 17.3 秒から 3.8 秒になった。出力は 21 ファイルで一覧が同じ、内容の差分は実行時刻(`generated_at` など)だけ、stderr の差分は完了表示の所要秒数だけだった。asset の取得中(1 件 3 秒の遅延)に SIGINT を送ると終了状態 130、SIGTERM では 143 で即座に終わり、一時ファイルは 6 件から 0 件になり、以後の出力は無かった。
- benchmark: 「上限値の決定(benchmark)」と decision log 0067。
- 実施していないこと: 実 workspace での実行(実 token が要る。merge 後にユーザーが trace を取り直す)、demo GIF の再生成(「ローカルで要再生成」)。
- review の指摘への対応(P4)の後に、上記の gofmt、vet、build、`go test -count=1 ./...`、`-race -count=1 ./...`、`-count=5 -shuffle=on`、`GOMAXPROCS=1 -count=3`、並列と直列の比較の `-count=20`、`-race -count=5`、cross compile、固定サンプルの比較をやり直し、すべて成功した。負荷をかけた反復(`internal/slack` と `cmd/slapex` の `-count=30` と、`internal/output` を加えた `-race -count=15` を同時に)も成功した。変異の確認は「review の指摘への対応(P4)」にある。E2E は、変更が打ち切りと中断の処理だけで出力の経路を変えないため、やり直していない。

## リスク・ブロッカー

- 実 workspace での実行はしていない(実 token が要るため)。上限値は in-process のモデルでの benchmark と Issue の目安から決めた。実際の Slack の host と第三者 host が 16 本(HTTP/2)や 6 本(HTTP/1.1)の同時の request にどう応じるかは、merge 後のユーザーの trace で確かめる。
- 429 を受けた lane 全体の停止は PF-04(#276)のスコープ。今は 429 を受けた download だけが `Retry-After` を待つ。

## セッションログ

- 2026-09-29: Issue #275 と #275 のコメント(PR #288 からの申し送り)、#272 の trace の集計、`progress.md`、関連コードを読んだ。依存は merge 済み。
- 2026-09-29: pacing の撤去、`internal/lane`、並列の `Fetch` と通知の保持、download の client と watchdog、中断の処理、`tools/assetbench` の方式と workload を実装し、test、benchmark、設計文書、decision log 0067 を足した。Issue の「検証」を実行した(「検証」)。
- 2026-09-29: 検証の反復実行で見つかった不安定さ 5 件(コード 2 件、test の前提 3 件)の原因を直し、決定的な test と変異の確認を足した。検証、E2E、中断の確認をやり直した。
- 2026-09-29: PR #291 を draft で作成し(03:10Z)、note を採番した(`d09ae90`)。`progress.md` の PF-03 の PR 欄を `#291` にした(P1)。検証は「検証」のとおり。出力生成系 3 skill は、`update-sample-exports` と `update-readme-preview-screenshots` を適用せず、`update-readme-demo-gif` は該当するが cloud session では実行できないため「ローカルで要再生成」とした(「出力生成系 3 skill の適用判断」)。
  - `run-issue-task` の報告から引き上げた項目。`number-working-branch-note` の報告の「書き換えた行の一覧」: note の `PR:` 欄(`未作成` → `#291`)、PR description の note のファイル名参照 1 行(`draft_` → `291_`)。title は書き換えていない。「触らずに残した行の一覧」: note の「現在の状況」の「次は PR の作成(draft)と採番。」(定型に当てはまらない)と、「次にやること」の「PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。」(複合行。`progress.md` の反映は同 skill で完了しない)。どちらも採番の後、`progress.md` の反映と合わせて書き換えた。PR description と title に触らずに残した行は無い。情報統制チェックで直した箇所は無い。出力生成系 3 skill は呼ばなかった(`update-sample-exports` と `update-readme-preview-screenshots` は「いつ使うか」に当たらない。`update-readme-demo-gif` は当たるが、cloud session では実行しない)。
  - PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。GitHub の操作はすべて組み込みの GitHub MCP tool で行い、`gh` は使っていない。
- 2026-09-29: review(P2)を subagent に委譲した。review cycle `claude-code-a85f4c7-20260929031415`、`Reviewed head` `a85f4c76b715927a7077e055fad200192754d16e`。指摘 3 件(inline 3、top-level 0)。prefix ごとに `[must]` 0、`[ask]` 1、`[imo]` 2、`[nits]` 0、`[fyi]` 0。`gh` への fallback は無し。完了要約の `Model` は、上位の指示で記載を控えたため `unknown`。指摘が 1 件以上のため P4 に進んだ(P3)。
- 2026-09-29: 指摘 3 件を採用して修正した(P4、`ffb9337`。「review の指摘への対応(P4)」)。処置の内訳は、採用し修正した 3 件(`[ask]` はユーザーに確認中で、推奨の案で進めた。1xx は提案と別の方法で直した)。スコープ外とした指摘と follow-up 候補は無い。出力生成系 3 skill の判断は変わらない。検証は「検証」の末尾のとおり。
