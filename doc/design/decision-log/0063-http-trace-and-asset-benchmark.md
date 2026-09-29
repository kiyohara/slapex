# 0063 HTTP trace と取得方式の benchmark(SLAPEX_HTTP_TRACE)

- 状態: decided
- 作成日: 2026-09-27
- 最終更新日: 2026-09-29
- 関連: `../cli-interface.md`、`../architecture.md`、`../../guidelines/credential-scope-guidelines.md`、[0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md)、[0033-go-dependency-policy.md](0033-go-dependency-policy.md)、[0046-api-base-url-override.md](0046-api-base-url-override.md)、Issue #272、Issue #273

## 背景

export の所要時間の最小化(#272)は、asset の並列取得と Slack Web API の並行化を段階的に進める。その前に、request ごとの時間の内訳を記録する trace と、取得方式を比べる benchmark が要る(PF-01、#273)。現状は、接続確立、最初の byte まで、転送、pacing と retry の待機の内訳が分からず、改善の効果を数字で確かめられない。

trace はユーザーが実 workspace の export で取り、集計を Issue に貼る。files.slack.com の path にはファイル名が、第三者 host の URL には共有 link の識別子が入り得るため、URL、token、workspace 名、channel 名を trace に残さないことが前提になる。

## 候補

- trace の取り付け位置: (A) `withRetry` の中で試行ごとに記録する。(B) client の transport と sleeper を包み、呼び出し(`call` / `Download`)ごとの記録器を context で渡す。
- URL の識別: (A) 記録しない。(B) URL の SHA-256。(C) client ごとの乱数を鍵にした HMAC。
- trace の有効化: (A) 利用者向けの option。(B) 内部用途の環境変数。
- 実行全体の時間(集計の割合の分母): (A) 集計 tool が、最初の request から最後の request の終わりまでで代える。(B) 利用者が export の所要時間を集計 tool に渡す。(C) slapex が trace の最後に、export の開始時刻と所要時間を書く。
- benchmark の置き場: (A) `go test -bench`。(B) `tools/` の開発用 tool。

## 検討内容

- 取り付け位置: (A) は #192 で共通化した `withRetry` に計測の分岐を持ち込むうえ、`http.Client` の中で起きる redirect の各段を拾えない。(B) は `withRetry` を変えずに、transport が受け取る request ごと(試行と redirect の各段)に行を作れる。`net/http/httptrace` の hook は request の context に載せる必要があるが、transport が request ごとに載せれば、同じ request を再送する `Download` の試行も別の行になる。redirect の段は `Request.Response` で見分けられる。ただし (B) では transport が `http.Client` の知らない型になるため、`http.Client` は timeout で request を止めるのに context に加えて `Request.Cancel` も使う。止める時点は同じだが、transport が返す error は先に気付いたほうになり、timeout の error の文が trace の無い実行と変わり得る(PR #281 の review で確認)。transport を包まずに hook と redirect の判定を context と `CheckRedirect` に移せば避けられるが、redirect の段の記録が粗くなり、既定の redirect の方針を複製することになる。
- 待機の帰属: 待機は request と request の間で起きる。最初の request の前の待機を pacing、失敗した request の後の待機(backoff、Retry-After)をその request の retry 待機とし、行は次の request が始まるときか、呼び出しが終わるときに書く。Retry-After の待機は試行の上限に達したときも行う(#192)ため、最後の行に残る。待機は要求値ではなく実測で記録する。sleeper を差し替えた実行(demo、benchmark の pacing なし)では、実際に待っていないためである。
- URL の識別: (A) では同じ URL への再送と別の URL を見分けられない。(B) は既知の URL の hash と照合できてしまう。(C) は同じ trace の中の同一性だけを残し、鍵を記録しないため照合できない。
- host: 集計(origin の種別、origin ごとの件数、HTTP version)に要るため記録する。trace ファイルは第三者 host の名前を含むので、共有には host 名を出さない集計 tool の出力を使う。
- 実行全体の時間: (A) は、最初の request の前と最後の request の後の処理(最後の asset の後の HTML の組み立てと書き込み、cache の書き込みと後片付けなど)を含まないため、割合を大きく見積もる(PR #281 の Codex の review で確認)。(B) は計測の手順が増え、Done の行の所要時間は秒に丸めてある。(C) は trace だけで集計でき、export の途中で止まった trace では (A) に戻せる。
- 有効化: 計測は開発と改善の確認のためのもので、#272 は利用者向けの option を増やさない方針である。`SLAPEX_API_BASE_URL`(0046)と同じく、内部用途の環境変数にする。
- benchmark: 現行方式は asset 1 件ごとに約 1 秒の pacing を挟み、実物に近い workload(56 件)では 1 回に約 55 秒かかる。`go test -bench` の反復や `go test ./...` の実行時間には向かない。workload(asset の件数、サイズ、origin の分布、origin の遅延・帯域・stream 上限)は JSON で差し替えられるようにしたい。

## 決定

- trace は `internal/slack` の `WithTrace(io.Writer)` で有効にする。client の transport と sleeper を包み、HTTP request 1 件ごとに JSON Lines の 1 行(`slack.TraceRecord`)を書く。retry の各試行と redirect の各段が 1 行ずつになる。`withRetry` と pacing は変えない。
- asset の kind は `output.Assets.Save` が `slack.WithAssetKind` で context に載せる。
- URL(path と query)、header(Authorization を含む)、token、body、error の本文は記録しない。URL は、client ごとの乱数を鍵にした HMAC-SHA256 の先頭 8 bytes(16 桁の hex)で識別する。error は種類(`canceled` / `timeout` / `dns` / `network`)だけを記録する。request の deadline を過ぎた後の失敗は、error の型によらず `timeout` とする。
- timeout の error の文が trace の有無で変わり得ることは受け入れ、`cli-interface.md` に書く。trace は計測のための内部用途で、止める時点と retry は変わらないためである。
- trace の最後に、export の開始時刻と所要時間の 1 行(`type` が `run`)を書く。書くのは CLI で、export が失敗したときも書く。`tools/tracereport` は、これを実行全体として割合の分母と request の外の時間に使い、この行が無い trace では、最初の request から最後の request の終わりまでで代える。
- CLI は、内部用途の環境変数 `SLAPEX_HTTP_TRACE` が空白以外のときだけ trace を有効にし、その path に書く。仕様(権限、上書き、失敗時の exit code と警告、`--demo` の扱い)は `cli-interface.md` の「環境変数」を正本とする。
- 集計は `tools/tracereport`、benchmark は `tools/assetbench` に置く。benchmark は in-process の fake origin(`httptest` の TLS、HTTP/2 と HTTP/1.1。origin ごとに handshake と最初の byte の遅延、帯域、`MAX_CONCURRENT_STREAMS` を設定できる)に対し、slapex の client と `output.Assets` で取得して、現行(直列、pacing あり)と pacing なしの直列を比べる。並列方式は PF-03(#275)が足す。
- 新しい依存は足さない(0033)。

## 理由

- 計測を通信の経路の外側(transport、sleeper、context)に置けば、retry と pacing の挙動を変えずに全 request を漏れなく記録できる。trace が無効のときは、経路そのものが従来と同じになる。
- 鍵付き hash と host だけの記録で、集計に要る同一性と origin の区別を残しつつ、trace から channel の内容を辿れないようにできる。
- 所要時間が長く、workload を差し替える benchmark は、test より開発用 tool のほうが扱いやすい。tool の動作は短い workload の test で保つ。

## 影響

- `cli-interface.md` の「環境変数」に `SLAPEX_HTTP_TRACE` を追記した。`architecture.md` の `internal/slack` の責務と `tools/` の入口に追記した。
- trace が無効のときの出力(stdout、stderr、HTML、assets、cache)は変わらない。有効時と無効時の export の出力が同じであることも、統合 test で確かめる。例外は、HTTP client の timeout で失敗した request の error の文である(「決定」)。
- trace は、request ごとの行の後に `run` の行を 1 行持つ。trace を読むものは、行を `type` で見分ける。
- 実 workspace の計測は merge 後にユーザーが手元で行い、`tools/tracereport` の出力を #272 にコメントする。PF-03 の上限値と PF-05 の判断に使う。
- 並列の取得(PF-03 以降)では request の時間が重なるため、集計の割合の合計が 100% を超え得る。

## 追記(2026-09-29)

PF-03(#275)で、`tools/assetbench` に並列の方式(`parallel`)を足した(`0067-parallel-asset-lanes.md`)。slapex の client が download の pacing をしなくなったため、pacing ありの直列は、benchmark の側で開始間隔 1 秒を待つ `paced` に改め、`current` を廃した。workload には、#272 の trace(2026-09-29)に合わせた `traced`(既定)と、files.slack.com に多数の file が並ぶ `heavy` を足し、fake origin が redirect を返せるようにした。報告には、同時に走った request の最大数と、最も長い request の時間を足した。

## 後から見直す条件

- trace の項目が PF-03〜PF-07 の判断に足りないと分かった場合(接続の識別、client 全体の帯域など)。
- 利用者から計測の要望があり、利用者向けの option にする必要が生じた場合。
- `net/http` の hook の挙動(接続の取得、HTTP/2 の扱い)が変わり、記録する時点の意味が変わった場合。
