# 0064 描画を 2 回走らせて asset の取得を計画する

- 状態: decided
- 作成日: 2026-09-28
- 最終更新日: 2026-09-28
- 関連: `../architecture.md`、`../cache.md`、[0030-cache-schema-and-reuse-validation.md](0030-cache-schema-and-reuse-validation.md)、[0052-content-hash-asset-filenames.md](0052-content-hash-asset-filenames.md)、[0063-http-trace-and-asset-benchmark.md](0063-http-trace-and-asset-benchmark.md)、Issue #272、Issue #274、Issue #275

## 背景

export の所要時間の最小化(#272)は、asset の並列取得(PF-03、#275)を予定している。asset の download は描画の途中で同期的に行われ、`output.Assets.Save` がその場で取得し、返した path が HTML に埋め込まれる。並列に取得するには、download の前に取得リストを確定する必要がある(PF-02、#274)。この段階では出力(HTML、assets、manifest、cache、stderr)をバイト単位で変えない。

また、user の avatar は `resolved.users` の map の順に保存していたため、manifest の entry と avatar の警告の順が、同じ入力でも実行ごとに変わりえた(PR #281 からの申し送り、#274 のコメント)。

## 候補

- 取得リストの求め方: (A) 描画とは別の収集関数で求める。(B) 描画を 2 回走らせ、1 回目の要求を記録する。
- 計画の記録先: (A) `output.Assets` に計画用の動作(planner)を持たせる。(B) 描画側に記録用の interface を差し込む。
- 取得結果の受け渡し: (A) 取得済みの結果を返す `output.Downloader` の実装を、2 回目の `Assets` に渡す。(B) `Assets` が取得結果の表を持ち、2 回目の `Save` がそれを記録する。
- 警告を出す時点: (A) 各取得の終わり。(B) 2 回目の描画で、`Save` が結果を記録するとき。
- `--reuse-cache` の一致の判定: (A) 計画のときに判定し、計画に含める。(B) 取得のときに判定する。
- user の avatar の順: (A) map の順のまま。(B) bot の icon と同じく ID 順。

## 検討内容

- 求め方: (A) は描画と収集がずれる危険がある。user の収集では実際にずれた(#205、#209、#251)。(B) は描画そのものが要求を出すため、ずれない。どの URL を要求するかは各 `Save` の成否に左右されない。サムネイルと原本、URL preview の画像と service icon は常に両方要求され、`Status` は表示の分岐にしか使われない。本文の emoji の HTML は mrkdwn 変換が placeholder に退避し、後の解析に影響しない。このため 1 回目で正確なリストが取れる。代償は描画 1 回分の CPU 時間で、download の時間に比べて小さい。
- 記録先: (A) は `Save`、`SkipTooLarge`、`Status` の判断(空の URL の無視、元 URL ごとの重複排除、最初の要求の kind による上限)を 1 か所に保てる。(B) は描画側の呼び出しを書き換えることになり、判断が二重になる。
- 受け渡し: (A) は download 先の一時ファイルを 2 回目にもう一度 copy することになり、大きい添付を二重に書く。(B) は取得のときに最終の path へ置き(一時ファイルから rename)、2 回目は記録するだけで済む。
- 警告の時点: 取得中の retry の通知(`INFO: assets: retrying download ...`)は取得のときに出る。(B) では通知がすべて先に出て、警告が後にまとまるため、stderr の並びが変わる。(A) なら通知と警告の並びが現行と同じになる。計画の順は 2 回目の要求の順と同じなので、警告の順も 2 回目の呼び出し順と一致する。
- reuse の判定: 判定は再利用元のファイルの状態(存在、サイズと上限、コピー先が同じファイルか。#202)を見る。現行はこれを各 `Save` の時点、つまり直前までの copy と download の後に行っていた。(A) は判定と copy の間に時間差ができ、同じ directory へ再出力する場合(#202)の順序も変わりうる。(B) は取得を計画の順に行うため、判定の時点が現行と同じになる。
- avatar の順: (A) では manifest と avatar の警告の順が実行ごとに変わり、2 回の描画の間でも変わりうる。計画の順と 2 回目の要求の順がずれると、比較の test も書けない。(B) は 1 行の変更で順が決まり、HTML、assets、cache の内容は変わらない(保存名は内容 hash、0052)。

## 決定

- Assets 工程は timeline を 2 回描画する(`export.renderWithAssets`)。1 回目は `output.Assets.Planner` が返す planner に描画し、要求された asset を最初に要求された順に計画(`output.PlannedAsset`: kind、元 URL、kind の上限、meta、事前のサイズ判定で除いたか)として記録する。planner は出力 directory に書かず、`--reuse-cache` の copy をせず、警告を出さない。
- `Assets.Fetch` が計画を計画の順に取得する。取得は直列で、pacing も現行と同じ(slack client の download)。事前のサイズ判定で除いたものは取得しない。reuse の一致は取得のときに判定し、一致すれば copy、しなければ download する。取得の結果は `Assets` の表に持ち、manifest には記録しない。
- 2 回目の描画の `Save` と `SkipTooLarge` が、要求された順に manifest へ記録する。計画に無い URL が要求された場合は、その場で取得する(安全側の fallback)。fallback が保つのは HTML、assets、manifest で、stderr の並びは保たない(その asset の retry 通知と警告が、`Fetch` の出力の後に出る)。逆向きのずれ(計画にあって 2 回目に要求されない asset)は受け止めない。`Fetch` が取得したファイルが manifest にも HTML にも無いまま出力に残り、取得の警告も出る。どちらのずれも、計画と 2 回目の要求の一致を比べる test(`internal/export/integration_plan_test.go`)が検出する。
- 警告は取得の終わりに出す。計画の順は 2 回目の要求の順と同じなので、manifest と警告の順は揃う。
- user の avatar は user ID 順、`bots.info` で解決した bot の icon は bot ID 順に保存する。manifest の entry の順は、描画が asset を求めた順になる(`cache.md`)。
- 計画は test のために context の値で観察できる(`export` の `assetPlanObserverKey`)。`export.Options` には足さない。`Options` は CLI の option と呼び出し側の設定を写す型で、`cmd/slapex` の test が field ごとに、設定するのが CLI か呼び出し側かを検査している。test 専用の field は、そのどちらにも当たらない。
- 新しい依存は足さない(0033)。

## 理由

- 描画そのものを計画に使えば、取得リストと描画の要求がずれない。ずれないことは、計画と 2 回目の要求の一致を全種類の経路で比べる test で保つ。
- 取得を計画の順に、現行と同じ判断と時点で行えば、download の順、警告と retry 通知の並び、reuse の判定、manifest、counts が現行と同じになる。
- 取得結果を `Assets` の表で渡せば、大きい添付を二重に copy しない。

## 影響

- `architecture.md` の `internal/export` と `internal/output` の責務、Assets 工程の説明を更新した。`cache.md` に manifest の entry の順を書いた。
- 出力は変更前と同じである。変更前のコードに avatar の ID 順だけを当てたものと比べ、export の結合 test が走らせる export すべてと、全種類の経路を含む場面(警告、retry、reuse、同じ directory への再出力を含む)で、HTML、assets、cache、stderr が一致した(fake server の URL、一時 directory、表示される待ち時間は揃えて比べた。実時刻を使う test と Messages 工程の待ち時間の揺れによる差は、変更と関係しない)。固定 sample(`tools/gensample -time 2026-07-04T16:32:41+09:00`)も `doc/samples/` と一致した。変更前のコードとの違いは、user の avatar が 2 人以上いるときの manifest の entry と avatar の警告の順だけで、これは変更前も実行ごとに変わっていた。
- PF-03 は `Fetch` を並列にすればよい。manifest の順は 2 回目の描画が決めるため、取得の順に依存しない。警告と retry 通知の並びは、並列にすると変わりうる。
- 描画を 2 回行うため、Assets 工程の CPU 時間が描画 1 回分増える。

## 後から見直す条件

- 描画が `Save` の成否によって要求する asset を変えるようになった場合(計画と 2 回目の要求がずれる。一致の test が落ちる)。
- PF-03 で、取得の前に reuse の copy と download を分ける必要が生じた場合(reuse の判定を計画に含めるかを見直す)。
- 描画 1 回分の時間が、所要時間の中で無視できなくなった場合。
