# 0068 download の 429 で origin の lane 全体を待たせ、同時数の上限を半分にする

- 状態: decided
- 作成日: 2026-09-29
- 最終更新日: 2026-09-29
- 関連: `../slack-api-usage.md`、`../architecture.md`、`../cli-interface.md`、[0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md)、[0033-go-dependency-policy.md](0033-go-dependency-policy.md)、[0063-http-trace-and-asset-benchmark.md](0063-http-trace-and-asset-benchmark.md)、[0067-parallel-asset-lanes.md](0067-parallel-asset-lanes.md)、Issue #192、Issue #272、Issue #276

## 背景

export の所要時間の最小化(#272)の PF-04(#276)である。PF-03(#275、0067)で、asset の download は origin(scheme + host + port)ごとの lane で並列になった。429 と 5xx は、Web API と共通の request 単位の retry(0025。#192 で `withRetry` に共通化)だけで扱っていた。そのため、ある download が 429 を受けて `Retry-After` を待つ間も、同じ origin の他の download は request を出し続け、origin として `Retry-After` の指示に従えなかった。また、retry を待つ download は、待つ間も lane の枠を持ち続けていた(lane は download の関数が返るまで枠を空けない)。

Slack は file の download と CDN の rate limit を公表していない(0067)。#272 の trace(2026-09-29、PF-03 の前の直列の取得)の status は 200、301、302、403 だけで、429 は無かった。並列にした後の trace はまだ無い。

PR #291(PF-03)の再確認で、download の retry が第三者 host(URL preview の画像、service icon など、link 先の site が指す host)の 429 の `Retry-After` にも上限なく従うことが follow-up 候補になり、ユーザーの判断で本 Issue に申し送られた(#276 のコメント、2026-09-29)。

## 候補

- 429 を受けた lane: (A) 今のまま、429 を受けた download だけが `Retry-After` を待つ。(B) lane 全体が `Retry-After` の間、新しい request を出さない(#276 の作業内容)。
- lane の同時数の上限: (A) 変えない。(B) 半分にする(#276 の作業内容)。(B) の場合に、同じ時期に返った複数の 429 で、(a) 429 ごとに半分にする。(b) 前回半分にした後に枠を得た download の 429 だけが半分にする。
- 半分にした上限の戻し方: (A) 戻さない。(B) 半分にした後に枠を得た download の成功 N 件ごとに 1 枠戻す(lane が開いたときの上限まで)。(C) 時間で戻す。
- retry を待つ download の枠: (A) 持ち続ける(#275)。(B) 失敗した時点で手放し、次の request の前に取り直す。
- 5xx と network error: (A) lane の上限に反映しない。(B) 429 と同じく半分にする。
- download の body の読み取り中の失敗: (A) 今のまま retry しない。(B) retry する。
- 第三者 host の 429 の `Retry-After`: (a) 上限なく待つ(今のまま)。(b) 60 秒まで待ち、それより長い待ちを求められた asset は待たずに失敗とする。(c) 5 分まで待つ。
- `withRetry` から lane へ伝える経路: (A) `withRetry` と `Download` の引数で lane を渡す。(B) lane が download ごとに渡す context に載せ、`withRetry` は context から伝える。

## 検討内容

- 429 を受けた lane: (A) では、1 件が 429 を受けても、同じ origin の残りの download が上限まで request を出し続け、それぞれが 429 を受けて待つ。origin が求めた待ちを、origin 全体としては守れない。(B) は #276 の完了条件で、他の origin の lane は止めない。待ちの長さは 429 を受けた download が待つ時間(`Retry-After` の秒数に 1 秒未満の jitter を足したもの)と同じにし、後から来た 429 がより長い待ちを求めたら延ばす。`Retry-After` が無い(または使えない値の)429 は待ちの長さを示さないため、lane は待たず、上限を半分にするだけにする。その download は今どおり指数 backoff で待つ。
- 上限の半減の回数: (a) では、上限いっぱいに出ていた request がそろって 429 を受けると、1 回の過負荷で上限が 1 まで落ちる(16 件がそろって 429 なら 16 → 1)。(b) は、半分にする前に枠を得ていた download の 429 を同じ過負荷の知らせとみなし、半分にした後に枠を得た download の 429 だけが、もう一度半分にする。TCP の輻輳制御が、1 往復の間の複数の損失で輻輳ウィンドウを 1 回だけ縮めるのと同じ考え方である。
- 戻し方: (A) は、一時的な 429 の後もその origin を最後まで遅くする。(C) は、request の往復の時間が origin ごとに大きく違う(trace で最初の byte まで数十 ms から約 1 秒)ため、1 つの時間で決めにくい。(B) は、半分にした後に枠を得た download が N 件成功するたびに 1 枠戻し、再び 429 を受けたら半分にする(AIMD)。N が小さいほど早く戻るが、持続する制限では 429 と lane 全体の待ちを繰り返す。N は「戻し方の根拠」の benchmark で決めた。成功として数えるのは、200 の応答を受け取ったこと(body を読み終える前)とする。
- 待つ download の枠: (A) では、`Retry-After` や backoff を待つ download が lane と全体(64 件)の枠を持ったまま眠る。待つ download が多いと他の origin まで待たされるうえ、lane 全体の待ちの後に、半分にした上限を超えて request が出る。(B) は、失敗した(429、5xx、network error、応答の受け取りの失敗)時点で枠を手放し、次の request の前に lane の枠を取り直す。枠を取り直す download は、まだ始まっていない download より先に、始まった順に枠を得る。lane 全体が待っている間に始まる download は、枠を得ずに始まり、待ちが終わるのを待つ。
- 5xx と network error: Slack が rate limit の知らせとして示すのは 429 と `Retry-After` である(0025)。5xx と network error は、server や経路の障害で、送る速さを落とせという指示ではない。(B) では、壊れた 1 ファイルが 500 を返し続けるだけで、同じ origin の他の download まで遅くなる。(A) にし、失敗した download だけが今どおり指数 backoff で待つ。1 件あたりの試行の上限(6 回)は変わらない。ただし、「待つ download の枠」を (B) にするため、backoff の間はその枠を他の download が使い、障害中の origin への同時の request は、#275 の lane より多くなり得る(lane の上限までで、それを超えない)。
- body の読み取り中の失敗: download は、成功した応答の body を stream として呼び出し元(一時ファイルへの書き込み)に渡し、Web API の呼び出しと違って body を読み終えてから成功とはしない。この違いは #192 の保つ条件で、隠す抽象化はしない。(B) にするには、書き込んだ途中の内容を捨てて最初から書き直す仕組みを `Download` の呼び出し元と取り決める必要がある。また、body の途中の失敗は、PF-03 で入れた打ち切り(body が 30 秒進まない、Slack の file 以外は 1 試行 5 分)によるものが多いと見込まれ、一度止まった server は再び止まりやすい。retry すると、1 件が lane の枠を持つ時間が試行の数だけ延びる(Slack の file 以外では 6 試行 × 5 分まで)。失敗しても、その asset を置換表示にするだけで export は続く。#272 の trace には、body の読み取りを含めて、失敗した request は無かった。(A) を保つ。
- 第三者 host の `Retry-After`: (a) では、大きな `Retry-After` を返す第三者 host が 1 つあると、export はその待ちが終わるまで終わらない。本決定の lane 全体の待ちにより、同じ origin の他の asset も同じだけ待つ。(b) の 60 秒は、slapex が自分で待つ最も長い backoff(0025 の上限 60 秒)と同じで、数秒から 1 分の rate limit には従い、それより長い待ちで export を止めない。(c) の 5 分では、1 件が 6 回待つと 30 分ほどかかり得る。Slack の Web API と files.slack.com の `Retry-After` は Slack の rate limit の指示で、今どおり上限なく待つ(0025)。ユーザーに確認を求め、(b) が選ばれた(2026-09-29)。上限の判定は `Retry-After` の秒数で行い、待ちに加える jitter(1 秒未満)を含めない。含めると `Retry-After: 60` の待ちは 60 秒を必ず超え、上限の 60 秒ちょうどを待てない(PR #292 の review の指摘)。lane の待ちの判定も同じく、429 が求めた待ちの残りで行う。
- 伝える経路: `withRetry` は Web API と download で共通で(#192)、download は `output.Downloader` の `Download(ctx, url, limit, w)` を通る。(A) では、lane を知らない Web API の呼び出しと、計画の外で描画中に取得する download にも引数が要り、`Downloader` の型が変わる。(B) は、lane が download ごとに渡す context に job を載せ(0067 の先導の `GotConn` の hook と同じ経路)、`withRetry` が context から lane に伝える。lane の外の呼び出しは何もしない。
- 保つ条件(#276): 最大 5 回の retry、jitter、cancel、応答の body を閉じる責任の所在、サイズの上限、asset の失敗時に export を続けること、ログの文言、Web API の body の失敗の retry、download の stream、認証 header の送信先、download の error と警告に URL を出さないこと、新しい依存を足さないことは変えない。PR #271 の characterization test(`internal/slack/client_retry_test.go`)は変えずに通る。「最後の 429 でも `Retry-After` を待つこと」は、Web API、files.slack.com、60 秒までの第三者 host で保ち、60 秒を超える第三者 host では待たずに失敗にする(上の (b))。

## 決定

- 429 を受けた download の origin の lane は、`Retry-After` の間(429 を受けた download が待つのと同じ、秒数に 1 秒未満の jitter を足した時間)、新しい request を出さない。既に出ている request はそのまま続ける。後から来た 429 がより長い待ちを求めたら、その分延ばす。他の origin の lane は止めない。`Retry-After` の無い 429 では、lane は待たない。
- 429 を受けると、lane の同時数の上限を半分(1 未満にはしない)にする。ただし、前回半分にした後に枠を得た download の 429 に限る。
- 半分にした後に枠を得た download が 200 の応答を 4 件受け取るごとに、上限を 1 戻す(lane が開いたときの上限まで)。値は `lane.Defaults.RestoreAfter`(4)に置く。根拠は「戻し方の根拠」の benchmark である。
- download は、失敗した時点で lane の枠を手放し、次の request の前に取り直す(`lane.Wait`)。枠を取り直す download は、まだ始まっていない download より先に、始まった順に枠を得る。lane 全体が待っている間に始まる download は、枠を得ずに始まり、待ちが終わってから枠を得る。
- 5xx と network error は lane の上限に反映しない。失敗した download は今どおり指数 backoff で待ち、待つ間は枠を手放す。
- download の body の読み取り中の失敗は、今どおり retry しない。
- Slack の file(files.slack.com。認証 header を付ける送信先と同じ判定)以外の download は、429 の `Retry-After` が 60 秒(`maxBackoff` と同じ。`downloadPublicMaxWait`)を超えると、待たずにその asset の失敗にする。判定は `Retry-After` の秒数で行い、待ちに加える jitter を含めない(60 秒は待つ)。lane が待っている間に request を出そうとした同じ origin の download も、lane が待つ 429 の `Retry-After` の残り(jitter を除く)が 60 秒を超えるなら、request を出さずに失敗にする。警告は `asset failed (<kind>): rate limited (429): the server asks to wait <待ち>, over the 1m0s limit` で(`<待ち>` は秒に切り上げる)、URL を含まない。Slack の Web API と files.slack.com は、今どおり上限なく待つ。
- lane は、download ごとに渡す context に job を載せる。`withRetry` は、各 request の前に lane を待ち(`lane.Wait`)、失敗で枠を手放し(`lane.Yield`)、429 と 200 を lane に伝える(`lane.RateLimited`、`lane.Succeeded`)。lane が渡した context でなければ、どれも何もしない。Web API の retry の挙動は変えない。
- HTTP trace では、download が lane を待った時間を、最初の request の前なら pacing の待ち、失敗した request の後ならその request の retry の待ちに数える。
- `tools/assetbench` の strategy `parallel` を本決定の lane にし、#275 の lane(429 は download ごとに待ち、待つ間も枠を持つ)を `parallel-275`(`lane.Detach`)として残した。429 を返す origin(`limit_per_sec` と `limit_burst` の token bucket、`refuse_after_ms` と `refuse_for_ms` の一時的な拒否、`retry_after_s`)、workload の `limited` と `spike`、戻し方の flag `-restore-after`、報告の 429 の件数を足した。
- 標準ライブラリだけで書く(0033)。

## 戻し方の根拠(benchmark)

`docker compose run --rm dev` の中で `tools/assetbench` を build し、`-workload <workload> -strategies <strategy> -restore-after <N> -runs 3` で実行した(go1.26.8、linux/amd64、4 CPUs の cloud の dev container、2026-09-29)。origin は in-process のモデルで、実測ではない。

`limited` と `spike` は、`traced` の files.slack.com(HTTP/2、最初の byte まで 890 ms)から thumbnail 160 件(20〜76 KiB)を取る仮定の workload である。Slack は file の host の rate limit を公表していないため、制限の形と `Retry-After` は仮定である。`limited` は持続する制限で、毎秒 8 件(8 件まで溜まる token bucket)を超えた request に `Retry-After: 1` の 429 を返す。lane の上限 16 件では毎秒約 18 件を出すため、制限を超える。`spike` は一時的な制限で、開始から 1.5〜3.5 秒の request に `Retry-After: 2` の 429 を返し、それ以外は制限しない。`traced` は 0067 の workload で、429 を返さない。各設定 3 回の中央値で、429 と request の件数は 3 回の幅である。

| 方式 | 上限を戻す間隔 | `limited` の時間 | `limited` の 429 | `limited` の request | `spike` の時間 | `traced` の時間 |
|---|---:|---:|---:|---:|---:|---:|
| `parallel-275`(#275 の lane) | – | 21.35 秒 | 69〜73 | 228〜232 | 12.79 秒 | 1.03 秒 |
| `parallel` | 戻さない | 36.60 秒 | 9 | 169 | 20.03 秒 | 1.04 秒 |
| `parallel` | 1 | 40.31 秒 | 89〜91 | 249〜251 | 13.74 秒 | 1.04 秒 |
| `parallel` | 2 | 29.33 秒 | 26〜30 | 186〜190 | 13.81 秒 | 1.04 秒 |
| `parallel` | **4** | **27.55 秒** | 15〜16 | 175〜176 | 14.66 秒 | 1.03 秒 |
| `parallel` | 8 | 27.94 秒 | 14 | 174 | 14.67 秒 | 1.04 秒 |
| `parallel` | 16 | 30.22 秒 | 10 | 170 | 16.44 秒 | 1.04 秒 |

`spike` の 429 は、どの方式と設定でも 16 件(拒否の間に出た 1 回分の request)で、`traced` の 429 は 0 件だった。

- `limited`: 戻さないと、下がった上限のままで 36.60 秒かかる。成功 1 件ごとに戻すと、上限がすぐに制限を超え、429 と lane 全体の待ちを繰り返して(89〜91 件)、40.31 秒と最も遅い。4 と 8 が最も速く(27.55 秒と 27.94 秒)、429 は 14〜16 件だった。16 は 429 が 10 件と少ないが、戻りが遅く 30.22 秒になる。#275 の lane は 21.35 秒と最も速いが、429 を受けた download だけが待ち、残りの download が制限を超えた request を出し続けるため、429 は 69〜73 件と 4〜5 倍になる。また、3 回中 2 回で asset を 1 件保存できなかった。1 件が 6 回続けて 429 を受け、retry の上限で諦めたためである(その 2 回の request の件数は、最初の 160 件と、429 の件数から 1 を引いた retry の和になる)。本決定の lane は、どの設定でも全件を保存した。制限の上での下限は、最初の 8 件の後の 152 件を毎秒 8 件で約 19 秒と、最初の byte の約 0.9 秒である。
- `spike`: 一時的な制限の後は、早く戻すほど速い(1 と 2 で 13.7〜13.8 秒、4 と 8 で 14.7 秒、16 で 16.4 秒、戻さないと 20.0 秒)。#275 の lane(12.79 秒)より遅いのは、上限を半分にしたためである。#275 の lane でも 429 は同じ 16 件で、429 を受けた download が枠を持ったまま待つ間は、lane の上限に達して新しい request が出ないためと見られる(推定)。
- `traced`: 429 が無ければ、どの設定も #275 の lane と同じ時間になる。
- 4 にした: `limited` で最も速く、429 は 2 の約半分で、`spike` では最も速い設定との差が 1 秒以内である。

## 理由

- 429 を受けた origin 全体が `Retry-After` に従い、他の origin は止まらない(#276 の完了条件)。
- 1 回の過負荷で上限を 1 回だけ半分にし、成功に応じて少しずつ戻すことで、一時的な 429 の後は元の速さに戻り、持続する制限では 429 と lane 全体の待ちを繰り返しすぎない。
- 待つ download が枠を手放すことで、眠っている download が他の download と他の origin を待たせない。
- 5xx、network error、body の失敗の扱いを変えないことで、rate limit と関係の無い失敗で速さを落とさず、download の stream の扱い(#192)を保つ。
- 第三者 host の長い `Retry-After` で export が止まらない。

## 影響

- 429 を返す origin への request と 429 の数は、#275 の lane より少なくなる。代わりに、持続する制限と一時的な制限の下では、429 を受けた download だけが待つ #275 の lane より遅くなり得る(「戻し方の根拠」の `limited` と `spike`)。
- 429 を返さない origin の取得は変わらない(`traced`)。
- Slack の file 以外の asset は、60 秒を超える `Retry-After` を返す host では、待たずに失敗し、置換表示になる。その host の他の asset も、`Retry-After` の残りが 60 秒を超える間は、request を出さずに失敗する。
- lane の外の download(計画の外で、描画中にその場で取得するもの)は、lane を待たず、lane に伝えない。429 には request 単位の retry だけで従う。第三者 host の 60 秒の上限は、この download にも掛かる。
- stderr の rate limit の待ちの通知(`rate limited on download, waiting <待ち> as instructed by Slack`)の文言は変わらない。lane 全体の待ちを待つ他の download は、通知を出さない。
- HTTP trace の pacing と retry の待ちに、lane を待った時間が入る。
- `slack-api-usage.md` の「rate limit とリトライ」と「file / asset の取得」、`architecture.md`、`cli-interface.md` の `SLAPEX_HTTP_TRACE` の説明を更新した。0025、0063、0067 に追記した。

## 後から見直す条件

- merge 後に実 workspace で取る trace(#272)で、Slack の host が 429 を返した場合(戻し方と `Retry-After` の扱いを実物に合わせる)。
- 5xx や network error が同時数に応じて増える origin が見られた場合(5xx でも上限を半分にする)。
- body の途中の失敗が実際の export で目立つ場合(書き直しの仕組みを足して retry する)。
- 第三者 host の asset が 60 秒の上限で失敗する場合(上限を延ばす)。Slack が file の host の rate limit を公表した場合。
