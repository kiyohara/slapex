# 0067 asset の download の pacing を外し、origin ごとの lane で並列に取得する

- 状態: decided
- 作成日: 2026-09-29
- 最終更新日: 2026-09-29
- 関連: `../slack-api-usage.md`、`../architecture.md`、`../cli-interface.md`、`../../guidelines/credential-scope-guidelines.md`、[0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md)、[0033-go-dependency-policy.md](0033-go-dependency-policy.md)、[0040-credential-scope-for-asset-downloads.md](0040-credential-scope-for-asset-downloads.md)、[0063-http-trace-and-asset-benchmark.md](0063-http-trace-and-asset-benchmark.md)、[0064-two-pass-asset-planning.md](0064-two-pass-asset-planning.md)、Issue #272、Issue #275、Issue #276

## 背景

export の所要時間の最小化(#272)の PF-03(#275)である。PF-01(#273、0063)の trace で、ユーザーが実 workspace の export を測った(2026-09-29、main `7b22e92`、channel 1 つの 3 日分)。81 request、71.785 秒のうち、asset の download の pacing の待ちが 40.0 秒(pacing の待ちは全体で 51.406 秒、うち Web API が 11.359 秒)で、最大の項だった。download は 56 件(redirect 3 段を含めて 59 request)で、27 origin に分かれ、1 origin あたり最大 10 request だった。download の request 自体は直列で計 15.8 秒かかり、files.slack.com では最初の byte までが 1 件あたり約 0.9 秒と、転送(3.9 MB で 0.08 秒)よりずっと長かった。

download の pacing(host を区別しない key `download` で、開始間隔 1 秒)は、Web API の method ごとの平準化(0025)を download にも当てたもので、PoC(PR #11)で files.slack.com の file を bot token で取得する前提で入った。Slack の rate limit は method × workspace × app の単位で、file の download と CDN の制限は公表されていない(#275 で 2026-09-27 に確認)。第三者 host には、Slack の制限は関係ない。

PF-02(#274、0064)で、download の前に取得リストが確定するようになった。測った Slack の host と gravatar は、すべて HTTP/2 で、`MAX_CONCURRENT_STREAMS` は 128 だった(#275、2026-09-27)。

## 候補

- download の pacing: (A) 残す。(B) 外し、Web API の method ごとの平準化は残す。
- 並列の単位: (A) origin を区別しない全体の同時数だけで抑える。(B) origin(scheme + host + port)ごとの lane の同時数と、全体の同時数で抑える。
- lane の最初の接続: (A) 最初から lane の上限まで同時に出す。(B) 最初の 1 件(先導)が接続を得てから残りを出す。
- HTTP/2 の接続で stream の上限に達したとき: (A) `http.HTTP2Config.StrictMaxConcurrentRequests` で stream の空きを待たせる(#275 の作業内容)。(B) Go の既定のまま、追加の接続を開かせる。
- download の timeout: (A) 今の `http.Client` の `Timeout`(120 秒、body の読み取りを含む)。(B) 応答 header までの待ちと、body が進まない時間の打ち切り。
- 少しずつ返し続けて終わらない download((B) の場合): (a) 打ち切らない(#275 の作業内容のまま)。(b) Slack の file 以外は、1 試行の上限で打ち切る。(c) すべての download を、1 試行の上限で打ち切る。
- 並列にした download の retry の通知と警告: (A) 起きた順に出す。(B) 計画の順に並べ直して出す。
- 実行中の SIGINT(Ctrl-C)と SIGTERM: (A) Go の既定のまま、その場で process を終わらせる。(B) 受けて export を止め、download 中の一時ファイルを消してから、同じ signal で終わる。

## 検討内容

- pacing: (A) では、並列にしても 1 秒に 1 件しか始められず、56 件で約 55 秒かかる。Slack は download の制限を公表しておらず、第三者 host には関係ないため、1 秒の間隔に根拠が無い。429 を受けた場合は、今の retry(429 + `Retry-After` の遵守)がそのまま働く。429 を受けた lane 全体を止めるかは PF-04(#276)で決める。
- 並列の単位: (A) は、1 つの origin に件数が偏ると、その origin への同時数が全体の上限まで膨らむ。HTTP/1.1 の origin では接続数がそのまま増え、第三者 host の負荷になる。(B) は、偏っても 1 origin への同時数を lane の上限で抑えられる。origin は HTTP client が接続を持つ単位で、host 名だけで分けると、scheme や port が違って接続を共有できないものを 1 つの lane に数えてしまう。
- 最初の接続: Go の HTTP/2 client は、接続がまだない host へ同時に出た request に、それぞれ TCP + TLS を張らせ、余った接続を捨てる(#272)。(A) では、HTTP/2 の origin でも lane の同時数だけ接続を張る。(B) は、最初の接続が確立した時点(`net/http/httptrace` の `GotConn`)で残りを流し、残りはその接続を共有する。待つのは接続の確立(trace で 1 接続あたり 33〜100 ms)だけで、最初の download の応答は待たない。確立した接続が HTTP/2 か HTTP/1.1 かも、このとき分かる。
- stream の上限に達したとき: (A) を実装したところ、fake server の test が止まった。Go の HTTP/2 client には、strict のときに `ClientConn.ReserveNewRequest` の予約が数えられたままになり、server の `MAX_CONCURRENT_STREAMS` を超える数の request が同じ接続を待つと、stream が空いても進まなくなる不具合がある(golang/go#70809、2026-09-29 の時点で open)。Go 1.26.8 の dev container で、server の `MAX_CONCURRENT_STREAMS` を 4、16、128 にし、それぞれ 2 倍の数の request を 1 本の接続へ同時に出すと、3 回とも 5 秒たっても終わらなかった(上限より 1 件多いだけのときは終わった)。strict にしなければ、どの場合も、上限を超えた分に追加の接続を開いて終わった。(B) の Go の既定は、stream の上限に達したときだけ追加の接続を開く。lane の上限(16)は Slack の host の上限(128)より小さいため、先導と合わせて接続は 1 本のままで、(A) で得たかったことは (B) でも保たれる。上限の小さい server では接続が増えるが、止まりはしない。
- timeout: (A) は body の読み取りを含む。1 本の接続を多くの stream で分け合うと、大きいファイルがそろって 120 秒を超え、そろって retry する失敗が起こり得る(#275)。(B) は、応答しない server と、途中で止まった download を打ち切る。`http.Transport` の `ResponseHeaderTimeout` は transport を差し替える test(`slack.WithTransport`)で効かず、body が止まったことも測れないため、client 側の watchdog で測る。応答 header の待ちは、最終の応答 header が揃うまで続ける。最初の byte(`httptrace` の `GotFirstResponseByte`)で待ちを外すと、1xx の中間応答(103 Early Hints など)や header の途中で止まった server を打ち切れない(PR #291 の review の指摘。test で再現した)。`Got1xxResponse` で待ちを張り直す方法もあるが、設定すると 1xx の header の合計の大きさを抑える `net/http` の制限が外れるため、使わない。redirect の次の request は、接続を得る時点(`GetConn`)で待ちを止め、送り終えてから測り直す。接続の確立は transport の timeout が抑える。
- 少しずつ返し続けて終わらない download: (B) の body の待ちは 1 byte ごとに測り直すため、30 秒以内ごとに 1 byte でも届く download は打ち切られない(PR #291 の review の指摘)。(a) では、そうした host が 1 つあるだけで export が終わらない。URL preview の画像と service icon は、link 先の site が指す第三者 host から取り、量は 5 MiB の上限(`publicPreviewAssetLimit`)で抑えているが、時間は抑えていない。(c) は、上限を長くしても、#275 が全体の timeout をやめた理由(大きい Slack の file が接続を分け合って上限を超える)を消せない。(b) は、上限を Slack の file 以外に限り、大きい Slack の file には置かない。
- 通知と警告の順: (A) では、stderr の行の並びが download の終わる順で変わり、実行ごとに異なる。(B) は、各 download の通知と警告を保持し、計画でそれより前の download がすべて出し終えてから出す。行と並びは、計画の順に直列で取得した場合と同じになる。代わりに、後ろの download の通知は、前の download が終わるまで表示が遅れる。
- 中断: これまでは Ctrl-C で process がその場で終わり、download 中の一時ファイル(出力 directory の `asset-*`)が残り得た。(B) は #275 の「cancel(Ctrl-C)で全 lane が止まり、一時ファイルが残らない」を満たす。shell から見た終わり方は (A) と同じ(signal で終わる。Ctrl-C なら終了状態 130)に保つ。

## 決定

- asset の download は pacing しない。Web API の method ごとの平準化(1 req/sec 目安)は変えない。
- `output.Assets.Fetch` は、計画のうち `--reuse-cache` の copy で済むものを計画の順に先に済ませ、残りの download を `internal/lane` で並列に取得する。manifest は従来どおり 2 回目の描画が要求の順に記録する(0064)。
  - lane は origin(scheme と host を小文字にし、port が無ければ scheme の既定の port を補ったもの)ごとに作る。
  - 各 lane は、最初の 1 件(先導)だけを先に出す。先導が接続を得た時点(`GotConn`)で、その接続が HTTP/2 を交渉していれば 16 件、そうでなければ 6 件まで同時に流す。先導が接続を得ずに終わった lane は 6 件とする。
  - 全 lane を合わせた同時数は 64 件までとし、空いた枠は lane が順に取る。
  - lane の中は、サイズが分かるもの(Slack の file の原本と添付)を大きい順に始め、サイズの分からないもの(thumbnail など)をその後に計画の順に始める。4 MiB 以上のものは、lane あたり同時 4 件までとし、空いた枠をそれより小さいものが埋める。
  - 上限値は `lane.Defaults`(HTTP/2 16、HTTP/1.1 6、全体 64、大きいファイル 4 件、閾値 4 MiB)に置く。根拠は下の benchmark である。
  - lane は計画の URL の origin で決まり、redirect 先の origin は数えない。redirect 先への同時の request は、redirect 元の lane の上限までになる。
- download は Web API と別の `http.Client` と `http.Transport` で送る。transport は `http.DefaultTransport` の複製で、`MaxIdleConnsPerHost` を lane の上限(16)にする(`slack.NewDownloadTransport`)。`slack.WithTransport` は、Web API と download の両方に同じ transport を渡す。`StrictMaxConcurrentRequests` は有効にしない(#275 の作業内容から変えた点)。
- download の client に全体の timeout は置かない。試行ごとに、request を送り終えてから最終の応答 header まで 30 秒、body が 1 byte も進まない 30 秒で打ち切る。応答 header の待ちは、1xx の中間応答や header の途中では止めない。redirect の次の request は、接続を得るまでの時間を数えず、送り終えてから 30 秒を測り直す。接続の確立は transport の dial と TLS handshake の timeout(30 秒と 10 秒)に任せる。
- Slack の file(files.slack.com。認証 header を付ける送信先と同じ判定)以外の download は、1 試行を、接続と応答 header の待ちを含めて 5 分で打ち切る。Slack の file には上限を置かない。
- 打ち切りは timeout の失敗として扱い、応答 header の前なら network error と同じく retry し、body の途中なら今の body の失敗と同じく retry しない。retry の扱いの見直しは PF-04(#276)で行う。
- 認証 header の送信先は変えない。files.slack.com への request にだけ付ける(0040)。
- 並列に取得する download の retry の通知(`slack.WithNotices` で受ける)と警告は、download ごとに保持し、計画の順に出す。export を止めた後は、保持している通知と警告を出さずに捨てる。
- export の実行中(token の対話入力の後から `export.Run` が返るまで)に SIGINT または SIGTERM を受けると、stderr の出力を止め、export の context を cancel する。`Fetch` は新しい copy と download を始めず、進行中の download は止まって一時ファイルを消す。`export.Run` が返った後、受けた signal で process を終わらせる。止まるまでに 5 秒を超えた場合と、2 回目の signal を受けた場合は、その時点で終わらせる。起動時から無視されている signal は無視したままにする。
- `tools/assetbench` の strategy を `paced`(pacing ありの直列。client は pacing しなくなったため、benchmark の側で開始間隔 1 秒を待つ)、`unpaced`(pacing なしの直列)、`parallel`(本決定の lane)にした。workload には、上記の trace に合わせた `traced`(既定)と、files.slack.com に多数の file が並ぶ `heavy` を足した。
- 標準ライブラリだけで書く(0033)。

## 上限値の根拠(benchmark)

`docker compose run --rm dev go run ./tools/assetbench`(go1.26.8、linux/amd64、4 CPUs の cloud の dev container、2026-09-29)。origin は in-process のモデルで、実測ではない。

`traced` は上記の trace に合わせた workload で、56 asset、59 request、27 origin(HTTP/2 24、HTTP/1.1 3)。接続と最初の byte までの遅延は trace の class ごとの平均に合わせ、帯域は仮定である。`heavy` は、files.slack.com に thumbnail 96 件、原本 24 件(0.5〜8 MiB)、添付 8 件(1〜6 MiB)が並び、files.slack.com の帯域を 10 MiB/s に絞った仮定の workload である(219 asset、16 origin)。

| workload | 方式 | 時間 | pacing の待ち | 新しい接続 | 同時の最大 | 最も長い request |
|---|---|---:|---:|---:|---:|---:|
| `traced` | `paced`(変更前) | 55.38 秒 | 39.50 秒 | 27 | 1 | 0.97 秒 |
| `traced` | `unpaced` | 15.86 秒 | 0 | 27 | 1 | 0.98 秒 |
| `traced` | `parallel` | 1.07 秒 | 0 | 27 | 45〜49 | 1.06 秒 |
| `heavy` | `paced`(変更前) | 228.61 秒 | 84.62 秒 | 19 | 1 | 1.75 秒 |
| `heavy` | `unpaced` | 143.35 秒 | 0 | 17 | 1 | 1.74 秒 |
| `heavy` | `parallel` | 15.47 秒 | 0 | 17 | 63〜64 | 7.66 秒 |

`traced` は各方式 3 回、`heavy` は `paced` と `unpaced` が 1 回、`parallel` が 3 回の中央値である。`traced` の `paced` の pacing の待ち(39.5 秒)と `unpaced` の時間(15.9 秒)は、trace の download の pacing の待ち(40.0 秒)と request の時間(15.8 秒)に近い。`traced` の `parallel` は、測るたびに 1.03〜1.09 秒の幅がある。`heavy` の `paced` の接続が 2 本多いのは、request の間隔が transport の idle の上限(90 秒)を超えた origin があったためと見られる(推定)。

`parallel` の上限を 1 つずつ変えた結果(3 回の中央値。他の上限は既定値で、太字が既定値):

| 変えた上限 | 値 | `traced` | `heavy` |
|---|---:|---:|---:|
| HTTP/2 | 4 | 1.05 秒 | 39.37 秒 |
| | 8 | 1.05 秒 | 23.21 秒 |
| | **16** | 1.05 秒 | 15.47 秒 |
| | 32 | 1.05 秒 | 15.56 秒 |
| | 64 | 1.04 秒 | 15.65 秒 |
| HTTP/1.1 | 1 | 1.04 秒 | 15.62 秒 |
| | 2 | 1.03 秒 | 15.47 秒 |
| | 4 | 1.04 秒 | 15.73 秒 |
| | **6** | 1.05 秒 | 15.47 秒 |
| 全体 | 8 | 2.32 秒 | 24.00 秒 |
| | 16 | 1.50 秒 | 15.31 秒 |
| | 32 | 1.04 秒 | 15.44 秒 |
| | **64** | 1.05 秒 | 15.47 秒 |
| | 128 | 1.03 秒 | 15.47 秒 |

大きいファイルの件数(lane あたり)と閾値を変えた `heavy` の結果(3 回の中央値):

| 閾値 | 1 件 | 2 件 | 4 件 | 8 件 | 16 件 |
|---|---:|---:|---:|---:|---:|
| 256 KiB | – | – | 18.46 秒 | – | – |
| 1 MiB | 39.63 秒 | 26.12 秒 | 17.45 秒 | 14.69 秒 | 18.80 秒 |
| 2 MiB | – | – | 16.74 秒 | – | – |
| **4 MiB** | – | 19.82 秒 | **15.47 秒** | 16.89 秒 | – |
| 8 MiB | – | 18.79 秒 | 18.78 秒 | – | – |

閾値を置かない(大きいファイルの上限を掛けない)場合は 18.79 秒だった。`heavy` の原本と添付は 8 MiB 未満のため、閾値 8 MiB はこれと同じになる。

files.slack.com の帯域だけを変えた `heavy`(大きいファイル 4 件。`-print-workload` で書き出した JSON の `bytes_per_sec` を変えて `-workload` に渡した。2.5 MiB/s と 40 MiB/s は各 1 回):

| 閾値 | 2.5 MiB/s | 10 MiB/s(`heavy`) | 40 MiB/s |
|---|---:|---:|---:|
| 1 MiB | 53.34 秒 | 17.45 秒 | 10.24 秒 |
| **4 MiB** | **52.47 秒** | **15.47 秒** | **9.08 秒** |
| 8 MiB | 55.75 秒 | 18.78 秒 | 9.67 秒 |
| なし | 55.73 秒 | 18.79 秒 | 9.74 秒 |

- HTTP/2 16: `traced` は 1 origin あたり最大 10 request で、4〜64 で差が無い。`heavy` の files.slack.com(128 request)は、4 で 39.37 秒、8 で 23.21 秒、16 で 15.47 秒で、32 と 64 は 16 より速くならない。16 は差が出なくなる最小の値で、Slack の host の stream の上限(128)の 1/8 である。
- HTTP/1.1 6: `traced` と `heavy` の HTTP/1.1 の origin は 1 origin あたり 2 request 以下で、1〜6 で差が無い。6 は browser が 1 host あたりに張る接続数で、第三者 host への負荷を browser の範囲に収める。
- 全体 64: `traced` は同時に 45 request まで重なり、8 で 2.32 秒、16 で 1.50 秒、32 以上で 1.03〜1.05 秒だった。`heavy` は files.slack.com の lane(16 件)が律速で、全体 16 以上では差が無く(15.31〜15.47 秒)、8 では 24.00 秒だった。64 は、`traced` の倍の origin がある export にも余裕を残す。
- 大きいファイル 4 件、閾値 4 MiB: 大きいファイルを同時に多く流すと、帯域を分け合って 1 件ずつが長くなるうえ、大きい順に始めるため小さいファイルが後ろに残り、最後に最初の byte を待つだけの区間ができる(閾値 1 MiB で 16 件は 18.80 秒。その trace では、原本と添付が 13.4 秒で終わった後に thumbnail 96 件のうち 81 件が始まり、終わりまで 5.4 秒かかった。既定値の trace では、thumbnail は原本と添付と並んで走り、最後の原本より前に終わった)。絞りすぎると、大きいファイルの最初の byte の待ちが隠れない(閾値 1 MiB で 1 件は 39.63 秒、2 件は 26.12 秒)。件数は、#275 の目安(origin あたり 2〜4)で最も速い 4 とした。閾値は、4 件のときに `heavy` で最も速い 4 MiB とし、files.slack.com の帯域を 1/4 と 4 倍にした場合も 4 MiB が最も速かった。閾値 1 MiB で 8 件は `heavy` で 14.69 秒とさらに速いが、目安の範囲を超え、帯域は仮定であり、`traced` では差が出ないため採らない。`traced` は 1 MiB 以上のファイルが 2 件だけで、1 MiB で 1 件にしたときだけ遅い(1.99 秒)。閾値を上げると、1 件の最も長い download は延びる(`heavy` で 1 MiB の 4.31 秒から 4 MiB の 7.66 秒、2.5 MiB/s では 15.3 秒から 29.2 秒)が、全体の timeout は置かず、body の待ちは途切れた時間で測るため、打ち切りには関わらない。
- 応答 header と body の待ち 30 秒: trace の class ごとの平均で、最初の byte までが最も長いのは files.slack.com の原本の約 1.1 秒で、30 秒はその 28 倍ほどである。body の待ちは、応答 header の後に 1 byte も来ない時間で測り、download 全体の時間には関わらない。HTTP/2 の DATA frame(既定の最大 16 KiB)が 16 stream に順に回るとすると、30 秒の間に 1 frame も来ないのは、接続全体の速度が約 9 KiB/s を下回る場合に当たる。
- Slack の file 以外の 1 試行 5 分: 5 分は、接続、応答 header の待ち、body を含む 1 試行全体の上限である。main の 1 試行の上限(120 秒)より長くし、並列で帯域を分け合っても、main で取れていた asset を打ち切りにくくした。URL preview 画像、service icon、workspace icon の上限 5 MiB を 5 分で運ぶのは、約 17 KiB/s にあたる。応答 header の前の打ち切りは retry するため、1 件が lane の枠を持つ時間は、429 の `Retry-After` の待ちを除いて、6 試行(各 5 分まで)と backoff(計約 31〜36 秒)までになる。応答 header の後に少しずつ返し続ける host では、body の途中の打ち切りを retry しないため、その試行で終わる。その前に応答 header の待ちの失敗が 5 回続いた場合で、約 8 分(接続の時間は別)である。

## 理由

- download の pacing の待ち(40.0 秒)は、trace で最大の項で、根拠の無い待ちだった。`traced` では、並列化で download の時間が 55.4 秒(pacing ありの直列)から 1.1 秒になり(pacing を外しただけの直列は 15.9 秒)、ほぼ最も遅い 1 件(files.slack.com の最初の byte)の時間まで縮む。
- origin ごとの lane と先導 1 本で、1 origin への同時数と接続数を抑えたまま、origin をまたいだ並列が効く。
- stream の空きを待たせないのは、Go の既知の不具合で取得が止まるのを避けるためで、lane の上限が stream の上限より小さい限り、接続数は変わらない。
- 通知と警告を計画の順に出せば、stderr を含む出力が直列の場合と同じになり、比較の test で保てる。

## 影響

- 出力(HTML、assets、manifest、cache、stderr の行と並び)は、直列で取得した場合と同じである。`internal/export` の結合 test が、全種類の経路を含む場面を、download をばらばらの遅延で終わらせて 5 回取得し、1 件ずつ取得した場合と比べる。固定 sample(`tools/gensample -time 2026-07-04T16:32:41+09:00`)も `doc/samples/` と一致する。
- stderr の retry の通知は、download の終わりを待ってから出るため、表示の時点が遅れることがある。
- HTTP trace では、download の pacing の待ちは 0 になり、request が時間的に重なる。`tools/tracereport` の class ごとの合計は、実行全体の時間を超え得る。
- 認証 header の送信先は変わらない。`internal/output` の test が、lane で並列に取得したときも files.slack.com にだけ送り、他の host と redirect 先には送らないことを確かめる。
- Slack の file 以外の asset を少しずつ返し続ける host や、1xx の中間応答だけを返す server があっても、その asset の失敗で済み、export は終わる。
- 実行中の SIGINT と SIGTERM で、一時ファイルが残らなくなる。channel の対話選択の画面で押す Ctrl-C は、signal ではなく選択の取り消しとして、従来どおり exit code 2 で終わる。一方、選択の画面に `kill` などで SIGINT や SIGTERM が送られた場合は、これまでの exit code 2 ではなく、その signal で終わる。
- `slack-api-usage.md` の「rate limit とリトライ」と「file / asset の取得」、`architecture.md`、`cli-interface.md` を更新した。0025、0063、0064 に追記した。

## 後から見直す条件

- ユーザーが merge 後に実 workspace で取り直す trace(#272)で、429、応答の遅れ、接続の失敗が増えた場合(上限値を下げるか、PF-04 で lane を止める)。
- golang/go#70809 が直り、stream の空きを待たせる利点が生じた場合(上限の小さい server で接続を増やさずに済む)。
- `MAX_CONCURRENT_STREAMS` が lane の上限より小さい server で、接続の増加が問題になった場合。
- 大きいファイルの多い export で、帯域の分け合いや 30 秒の打ち切りが問題になった場合。
- Slack の file 以外の asset が 5 分の打ち切りで失敗する場合(上限を延ばすか、最低の転送速度を下回ったら打ち切る方式にする)。Slack の file で、少しずつ返し続けて終わらない download が見られた場合(Slack の file にも上限を置く)。
