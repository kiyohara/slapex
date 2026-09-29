# 0069 Web API を method ごとの lane で呼び、確定した取得を先に始める

- 状態: decided
- 作成日: 2026-09-29
- 最終更新日: 2026-09-29
- 関連: `../slack-api-usage.md`、`../usage-flow.md`、`../cli-interface.md`、`../architecture.md`、[0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md)、[0033-go-dependency-policy.md](0033-go-dependency-policy.md)、[0045-cli-output-style.md](0045-cli-output-style.md)、[0052-content-hash-asset-filenames.md](0052-content-hash-asset-filenames.md)、[0063-http-trace-and-asset-benchmark.md](0063-http-trace-and-asset-benchmark.md)、[0064-two-pass-asset-planning.md](0064-two-pass-asset-planning.md)、[0067-parallel-asset-lanes.md](0067-parallel-asset-lanes.md)、[0068-lane-wide-rate-limit-wait.md](0068-lane-wide-rate-limit-wait.md)、Issue #272、Issue #277、Issue #278、Issue #279

## 背景

export の所要時間の最小化(#272)の PF-05(#277)である。段階 3(export 全体の並行化)の設計を決め、実装は PF-06(#278)と PF-07(#279)で行う。本 Issue では code を変えない。

PF-03(#275、0067)と PF-04(#276、0068)で、asset の download は origin ごとの lane で並列になった。ユーザーが手元で取った trace の集計(#272 のコメント、2026-09-29。同じ channel の 3 日分、`--reuse-cache` なし)では、run は main `7b22e92` の 71.785 秒から main `fbd7cae` の 19.064 秒になった。後者の内訳は次のとおりである。

- Web API の 22 回の呼び出しが、run の開始から 15.693 秒の時点まで 1 件ずつ続く。request の時間の合計は 15.670 秒で、うち pacing の待ちが 11.136 秒(`users.info` 12 回で 8.759 秒、`conversations.replies` 4 回で 2.378 秒)である。
- asset の download(56 件、27 origin)は、その後の 3.349 秒に集まる。新しい接続 27 本の DNS の解決に合計 22.952 秒かかり、download の区間の最初の 3 ms に 25 件の解決が始まって、うち 14 件に 1.05〜2.40 秒かかった。

Web API は、method ごとの 1 秒 1 件の平準化(0025)の下で、method が違っても 1 件ずつ直列に呼ばれる。`slack.Client` の `pace` は並行に安全でない map で間隔を管理し、`export.Run` は工程(Workspace、Channel、Messages、Users、Emoji、Assets)を順に進める。asset の download は、Assets 工程で描画から取得の計画(0064)を作るまで始まらない。

#277 は、export 全体を「頻度で絞る method の lane」と「同時数で絞る origin の lane」を持つ 1 つの scheduler で回すにあたり、工程の順序と、工程ごとに 1 行ずつ進む進捗表示(0045)が変わり得るため、実装の前に次を決めるよう求めている: 効果の見込み、task の依存関係、並行する呼び出しの間での同じ method の pacing、`--max-posts`・emoji による除外・truncated の判定を今と同じにする方法、先読みの方針、失敗時の扱い、進捗表示、決定性、`--reuse-cache` と demo mode、検証の方法、PF-06 と PF-07 の本文。

## 候補

- 並行の形:
  - (A) 工程を同時に走らせる。Messages、Users、Emoji、Assets をそれぞれ task にし、ID や URL が見つかるたびに後の工程の task へ渡す。進捗表示は、同時に進む工程を複数の行で示す。
  - (B) 工程は今の順に 1 つずつ進め(以下、driver)、後の工程が必ず出す request を、それが確定した時点で先に出しておく(以下、先行取得)。driver は、その工程に来たときに先行取得の結果を受け取る。
  - (C) 今のまま直列にする。
- 先行取得の範囲((B) の場合):
  - (a) 確定した request だけ: 今の直列の処理がこの後必ず同じ request を出すと、その時点で分かるもの。
  - (b) 見込みで出す: 後で除外されたり `--max-posts` の外になったりし得る投稿の分も出し、使わない結果は捨てる。
- 同じ method の pacing:
  - (a) method ごとの lane: 同時に 1 件まで、来た順、開始の間隔 1 秒以上。
  - (b) method ごとに開始の間隔だけを守り、同時の数は限らない。
  - (c) 外部の rate limiter(`golang.org/x/time/rate` など)を使う。
- 先行取得の失敗の報告:
  - (a) driver がその結果を受け取るまで保ち、直列と同じ位置で扱う。
  - (b) 致命的な失敗を受けた時点で export を止める。
- 仕事の分け方: (a) PF-06(Web API)と PF-07(asset)の 2 つのまま。(b) 進捗表示の変更などを別 Issue に分ける。

## 検討内容

### 効果の見込み

上の trace の request の時間は変えずに、並べ替えたときの所要時間を見積もる(推定)。

| 段階 | 所要時間 | 見積もり方 |
|---|---:|---|
| 今(main `fbd7cae`) | 19.1 秒(実測) | Web API 15.7 秒の後に download 3.3 秒 |
| method の lane と Web API の先行取得(PF-06) | 約 15.5 秒 | `auth.test`、`team.info`、`conversations.list`、`conversations.history` の 0.9 秒の後、`users.info` 12 回(11.2 秒)が `conversations.replies`(3.2 秒)、`bots.info`、`emoji.list` と並行に進み、約 12.1 秒で Web API が終わる。その後に download 3.3 秒 |
| asset の先行取得(PF-07) | 約 12.5 秒 | message の asset は `conversations.history` と `conversations.replies` の結果が届いた時点(1〜4 秒)から取得し始め、`users.info` の待ちの間に終わる。残るのは、最後の `users.info` の結果の avatar(既にある接続を使う)と描画 |

- 測った export では、PF-06 で約 3.6 秒(19%)、PF-07 と合わせて約 6.6 秒(35%)縮む見込みである。上の DNS の解決の遅れも、先行取得では URL が分かった順にばらけて始まり、`users.info` の待ちの裏に隠れる。
- method の lane で縮む時間は、おおむね、`conversations.history` の後に呼ぶ method のうち、一番長い method 以外の時間の合計である。thread が多い export ほど大きく、#277 の例(thread 30 件、user 15 人)では、pacing だけで約 45 秒が約 30 秒になる。`conversations.history` の page が複数になる export では、`conversations.replies` と `users.info` を page ごとに始められる分も縮む。
- PF-07 の後の下限は、`conversations.history` までの呼び出し(約 1 秒)と、`users.info` の回数 × 1 秒でほぼ決まる。これは 0025 の後から見直す条件「多人数 channel で `users.info` の呼び出し回数が実行時間の支配項になった場合」に当たる状態で、`index.md` の未決事項「user 解決の最適化」で扱う。method ごとの 1 秒 1 件は #272 の「変えないもの」のため、本決定では扱わない。
- 見込みは小さくないと判断し、PF-06 と PF-07 を進める。PF-07 の merge の後に、ユーザーが手元の trace で確かめる(#272 の「ユーザーが行うこと」の 2)。

### 並行の形

- (A) と (B) の所要時間は、先行取得の範囲が同じなら、ほぼ同じになる。どちらも、method の lane と origin の lane が request を出せる速さで決まり、driver が待つのは、その時点で lane が出し終えていない結果だけだからである。例えば (B) の driver は、Messages 工程で `conversations.replies` の結果を順に受け取る間、`users.info` を裏で進め、Users 工程でその lane が出し終えるのを待つ。(A) で Users の task が同じ lane を待つのと同じ時点に終わる。
- (A) は進捗表示を変える。styled では同時に進む工程を複数行で描き直し、plain では工程の INFO 行が交互に並ぶ。0045 のフェーズ行の型(1 つの live な phase 行と、完了した行の追記)が変わり、plain の行の順は実行ごとに変わって、CI の log を安定して読めること(`cli-interface.md` の「出力制御」)を損なう。error と警告の順、最初に報告する致命的な失敗も、request の終わる順で変わる。直列の場合と stderr を比べる test も書けない。
- (B) は、工程の順序、フェーズ行、stderr の行の内容と順序、最初に報告する失敗と exit code を、作りの上で直列の場合と同じに保てる。工程の処理(`fetchThread`、`dropExcludedThreads`、`lookupUsers`、`renderWithAssets` など)は同じ関数のまま、request の結果が早く返るだけになる。足すものは、`slack.Client` の method ごとの lane、先行取得の表、何が確定したかを決める規則である。
- (B) の欠点は、裏で進む取得が表示に出ないことである。先に取得が済んだ工程は、待たずに完了の行を出す。測った export では、Users 工程が `users.info` の lane を待つ間、「resolving 12 users ...」の行が残る。今も、この行は `users.info` を順に呼ぶ間(測った export で約 11 秒)残っており、見え方は変わらない(残る時間は短くなる)。
- (C) は #272 の目的に合わない。

### 先行取得の範囲

- (b) は、rate limit のかかる呼び出しを無駄にする。`users.info` は所要時間の下限を決める lane で、除外される投稿や `--max-posts` の外の投稿の分を 1 件出すごとに、約 1 秒延びる。
- (b) は、利用者が emoji による除外で export から外した投稿の asset も取得する。出力から消しても、files.slack.com の認証付きの download は行われ、除外の option の意図に反する。
- (b) では、使わなかった結果を出力、manifest、cache から外し、その失敗を無視する仕組みも要る。
- (a) は、何が確定したかを決める規則が要る。規則は今の Messages 工程の処理から決まり、emoji filter を指定しない場合は単純である。filter を指定した場合、thread の親と `thread_broadcast` は、後の page や `conversations.replies` で見た親で除外が決まることがある(`fetchThread`、`dropExcludedThreads`)ため、Messages 工程の終わりまで確定させない。
- (a) では、成功した export が出す request は直列の場合と同じになる(method ごと、asset ごとの件数が同じ)。例外は、同じ URL の asset を、kind や file object のサイズが食い違う複数の要求が求める場合で、1 件増えることがある(「決定性」)。trace と test で直列と比べられる。

### 確定の規則と、`--max-posts`、除外、truncated

- driver は Messages 工程の処理(`conversations.history` の page、emoji filter、`--max-posts`、thread の取得、除外した thread の親と broadcast の削除、補充の page、replies の選別)を今のまま行う。先行取得は request を出して結果を保つだけで、filter の状態と件数は driver だけが今の順に変える。そのため、timeline、replies、truncated、除外件数は直列の場合と同じになる。
- `slack.Client.History` は、page ごとに、残した message(filter を通り、`--max-posts` の内のもの)を知らせる口を持つ(PF-06)。残した message は、その後の page の結果によらず batch に入る。
- 確定した message:
  - emoji filter を指定しない場合: `conversations.history` が残した message と、取得した thread の replies。filter が無いと除外が起きず、除外で減った分を補う page(補充)も無い。
  - filter を指定した場合: thread に属さない message は、`conversations.history` が残した時点で確定する。後で timeline から除かれるのは、除外された thread の message(親と broadcast)だけだからである。thread に属する message(親、`thread_broadcast`、replies)は、Messages 工程の終わりに確定する。
- `conversations.replies` は、`conversations.history` が残した親投稿の thread なら、その page を受け取った時点で確定する(driver は、除外されていない親の thread を必ず取得する)。filter を指定した場合の `thread_broadcast` の thread は、後の page に来る親の除外で取得しなくなることがあるため、先に出さない。同じ thread は 1 回だけ取得する(先行取得と driver の取得を区別せず、1 つの表で重複を除く)。
- 先行取得は Messages 工程の開始から(対象の確定、`--reuse-cache` の検証、出力先の作成の後)行う。`auth.test`、`team.info`、`conversations.list`、`conversations.history` は先に出さない。前の 3 つは対象を確定する工程(channel の対話選択を含む)の中で呼び、合わせて 0.6 秒ほどと小さい。`conversations.history` は cursor で page を順に辿るためである。`emoji.list` は、`--reuse-cache` の custom emoji を使わない限り driver が必ず呼ぶため、Messages 工程の開始時に確定する。

### 同じ method の pacing

- (a) は、同じ method の呼び出しを今の直列と同じ間隔で出す: 同時に 1 件まで、前の呼び出し(retry を含む)が終わってから、かつ前の呼び出しの開始から 1 秒以上空けて始める(今の `pace` は、呼び出しを始める直前に時刻を記録する)。429 の `Retry-After` を待つ間は、その method の次の呼び出しも待つ。0068 の download の lane が 429 で origin 全体を待たせるのと同じ形で、Slack の rate limit の単位(method × workspace × app)に合う。異なる method は止めない。
- (b) は、前の呼び出しが 1 秒を超えると、同じ method の request が 2 件以上同時に出る。429 を受けても、既に出た同じ method の request は止まらない。
- (c) は依存を足す。1 秒の間隔と 1 件の同時数は、標準ライブラリの mutex と待ち行列で書ける(0033)。
- 来た順(FIFO)にするのは、driver の呼び出しと先行取得を区別しないためである。lane に並ぶのは確定した request だけで、driver はどれも待つことになるため、順を入れ替えても所要時間は縮まない。
- HTTP trace(0063)の Web API の pacing の待ちは、今と同じく、前の呼び出しの開始から 1 秒の間隔を守るために待った時間とする。呼び出しが lane の先頭に来た時点(lane に並んだ時点と、同じ method の前の呼び出しが終わった時点の遅い方)から始まるまでの時間で、今どおり client の sleeper で待つ。呼び出しが、同じ method の前の呼び出しが終わるのを lane で待つ時間は記録しない。download の trace も、始まった download が `lane.Wait` で待った時間(429 で lane が止まった間と、retry の前に枠を取り直す待ち)だけを記録し、`lane.Run` が download を始めるまでの待ち(先導の 1 件が接続を得るまで、lane と全体の上限)は記録しない(0068、`internal/slack/trace.go` の説明)ため、それと揃う。lane に並んだ時点からの待ちを全部数えると、例えば `users.info` 12 件が同時に並んだとき、待ちは 0、1、…、11 秒で合計約 66 秒になり、今の trace(8.759 秒)と比べられない。

### 先行取得の失敗と cancel

- (b) は、報告する error と、その前に出る stderr の行を、request の終わる順で変える。driver は、失敗した request より前の結果の多くを既に先行取得で得ていて、すぐその位置に来るため、止めるのを早めても縮む時間は小さい。
- (a) の扱い: 先行取得の成否は、driver がその結果を受け取るまで報告しない。driver は直列の場合と同じ位置で同じように扱う。致命的な失敗(`conversations.replies` の retry の上限など)は export を終え、exit code は今どおり(`cli-interface.md` の「exit code」)である。`users.info` と `bots.info` の失敗は警告して続け、asset の失敗は置換表示にする。exit code `4` の意味は変えない。
- `export.Run` が返るとき(成功、失敗、SIGINT / SIGTERM)は、終わっていない先行取得を止め、止まるのを待ってから返る。driver が受け取らなかった結果、通知、警告は捨てる。SIGINT / SIGTERM の扱い(0067)は変えず、先行取得も `export.Run` の context で止まる。

### 進捗表示

- フェーズ行の型(0045)と工程の順は変えない。裏で進む取得は別の行に出さない。
- 先行取得した request の retry と rate limit 待機の通知は、driver がその結果を受け取るときにまとめて出し、request がまだ終わっていなければ、その後の通知はそのまま出す(0067 が download の通知を計画の順に保つのと同じ考え方)。plain の行の内容と順序は直列の場合と同じになる。裏で既に終わった待ちの通知は、driver がそこに来たときに出るため、示す待ちの秒数は過ぎた待ちのことがある。
- `usage-flow.md` の「処理対象の表示」の表示例は変わらない。
- demo GIF(`tools/demo/demo-ja.tape`)は実際の pacing で録画するため、PF-06 と PF-07 で工程の終わる時間が早まる。表示の文言は変わらない。再録画は cloud session で行えないため、PF-07 の後にまとめてユーザーが手元で行う(#272 の「ユーザーが行うこと」の 4)。

### 決定性

- 成功した export の HTML、`assets/`、manifest、cache は、直列の場合と同じになる。driver が結果を直列と同じ順に受け取り、描画と cache の組立は今のままだからである。
- 先に download した asset は、内容を一時ファイルに置き、download の結果(内容 hash、Content-Type、内容から判別した形式、サイズ、または失敗とその error)を保つ。Assets 工程の取得(`Assets.Fetch`)が計画の順に同じ URL を扱うときに、保った結果と計画の kind・meta から、直列の download と同じ名前(kind の directory、内容 hash、`extensionFor` の拡張子。0052)と manifest の entry を作って `assets/` へ移す。失敗の警告も計画の kind で組み立てる。
- 計画は URL ごとに、描画の順で最初の要求の kind と meta を保つ(`Assets.addToPlan`)。先行取得は `conversations.history` の page を新しい順に受け取りながら始まるため、先に download したときの要求と食い違うことがある。例えば、custom emoji の alias は同じ画像の URL に解決されるが、meta の `EmojiName` は書かれた名前になる。download の request は URL だけで決まり(認証の header、timeout、retry)、kind が効くのはサイズの上限と、警告の文と trace の label だけなので、meta の違いでは取り直さない。
- サイズの上限は計画の kind のもので判定する。先の download が最後まで済み、計画の上限を超えていれば、直列の download が上限で止めた場合と同じ結果(manifest の `skipped_size` と同じ警告)にする。先の download が計画の kind より小さい上限で止まった場合は、先の結果を使わずに今どおり取得する。計画が download しない URL(最初の要求が file object の `size` によるサイズの skip のもの)の先の結果も使わない。このため、同じ URL を、kind や file object のサイズが食い違う複数の要求が求める場合に限り、成功した export の request が直列より 1 件増えることがある。
- 移さなかった一時ファイルは、使わないと決まった時点か `export.Run` の終わりに消す。失敗した export の出力先も、直列の場合と同じに保つためである。
- stderr は、時間の表示(Done の所要時間、待ちの秒数)を除き、行の内容と順序が直列の場合と同じになる。
- HTTP trace は、成功した export では request の集合が直列の場合と同じで(上の例外を除く)、時刻と順序が変わる。先に download した asset の record の kind は、先に download したときの要求の kind である。

### `--reuse-cache` と demo mode

- 先行取得は、driver と同じ判定で cache を使う。cache にある user と bot は `users.info` / `bots.info` を呼ばない。cache の custom emoji を使う場合は `emoji.list` を呼ばない。`--reuse-cache` の copy で済む asset は download せず、copy は今どおり `Fetch` が行う。cache の検証は Messages 工程の前に済んでいる。
- demo mode は同じ `export.Run` を通り、pacing を省く(`NoPacing`)。method の lane は同時 1 件を保ち、間隔を待たない。demo の出力、stderr、同梱 sample は変わらない。

### 検証の方法

- `testing/synctest` の仮想時間: method の lane(同じ method は同時 1 件、来た順、1 秒以上の間隔、異なる method は並行、429 の `Retry-After` は同じ method だけを待たせる、cancel で待ちがほどける)を、`internal/output/ratelimit_test.go` と同じ `net.Pipe` の fake server で確かめる。export 全体も、同じ bubble の中で固定の遅延を持つ fake server に対して走らせ、所要時間が見込みの形(`conversations.history` まで + `users.info` の回数 × 1 秒 + 残り)に合うことを確かめる。
- fake server の結合 test(`internal/export`): 各 scenario を、先行取得なし(test だけの切り替え)と先行取得ありで走らせて比べる。
  - 成功する scenario では、出力のファイル、正規化した stderr(`assertSameExport`)、method ごと・asset ごとの request の件数が一致することを確かめる。scenario は、基本、`--max-posts` の打ち切り、emoji filter(親の除外で replies も除く、親が範囲外や `--max-posts` の外の broadcast、`conversations.replies` で見た親で除外される thread)、replies の上限、`--reuse-cache`、`users.info` / `bots.info` の失敗、`users.info` の 429 と `Retry-After`(通知の順)とする。
  - 失敗と cancel の scenario(`conversations.history` / `conversations.replies` の致命的な失敗、先行取得中の cancel)では、request の件数は一致するとは限らない。先行取得は、直列なら失敗の後に出る request(Messages 工程の開始時の `emoji.list`、thread が retry の上限まで backoff する間の `users.info` / `bots.info` など)を、失敗より前に出していることがあり、cancel では各 lane の進み具合で件数が変わるためである。error、exit code、正規化した stderr、出力先のファイル(直列に無いファイルと一時ファイルが残らないこと)が一致すること、先行取得ありの request が、同じ scenario で失敗も cancel もしない場合に出る request に含まれること、`export.Run` が返った後に request が出ないことを確かめる。cancel は時刻ではなく driver の位置(driver がある request の結果を受け取る時点など)で起こし、両方で同じ位置にする。
- PF-07 の merge の後に、ユーザーが手元で trace を取り、PF-03 の後(`fbd7cae`、#272 のコメント)と比べる(#272 の「ユーザーが行うこと」の 2)。比べる値は、run の時間、Web API の最後の request が終わる時点と、download の最初の request が始まる時点と最後の request が終わる時点(run の開始から)、method ごと・asset の kind ごとの request の件数、method ごとの pacing の待ちとする。request が重なるため、class ごとの Total の合計と `Outside requests` は、run の中の時間の配分を表さない。PF-06 で `tools/tracereport` に、class ごとに request が続いた区間(run の開始から、最初の request が始まる時点と最後の request が終わる時点)を足す。PF-03 の後の比較では、この区間を trace の record から計算した。

### 仕事の分け方

- 進捗表示は変えないため、別 Issue に分けるものは無い。
- PF-06 は Web API(method の lane、`conversations.history` の page の口、先行取得の表と確定の規則、`emoji.list`・`conversations.replies`・`users.info`・`bots.info` の先行取得)、PF-07 は asset(後から job を足せる lane の scheduler、一時ファイルと `Fetch` での移動、確定した message と user / bot の結果からの asset の計画)を扱う。PF-07 は PF-06 の確定の規則と表の上に作るため、PF-06 を依存にする。

## 決定

- 並行の形は (B) とする。`export.Run` の工程は今の順に 1 つずつ進め、後の工程が必ず出す request を、確定した時点で先に出す。
- 先行取得の範囲は (a) とする。規則の正本は `slack-api-usage.md` の「取得の並行化」に置く。要点は次のとおりである。

| request | 先に出す時点 |
|---|---|
| `auth.test`、`team.info`、`conversations.list`、`conversations.history` | 先に出さない |
| `emoji.list` | Messages 工程の開始時(cache の custom emoji を使う場合は呼ばない) |
| `conversations.replies` | `conversations.history` が残した親投稿の thread を、その page を受け取った時点 |
| `users.info`、`bots.info` | 確定した message に、解決する ID が現れた時点(cache にある ID は呼ばない) |
| asset の download | 確定した message の asset は URL が分かった時点(custom emoji の画像は custom emoji の一覧の後)、avatar は `users.info` / `bots.info` の結果の後、workspace icon は Messages 工程の開始時 |

- Web API は method ごとの lane((a))で呼ぶ。HTTP trace の pacing の待ちは、呼び出しが lane の先頭に来た後に、前の呼び出しの開始から 1 秒の間隔のために待った時間だけとする。
- 先行取得の失敗は (a) で扱い、`export.Run` が返るときに先行取得を止めて待つ。
- 進捗表示のフェーズ行と工程の順は変えない。先行取得の通知と警告は、driver が結果を受け取るときに直列の順で出す。
- 先に download した asset は一時ファイルに置き、`Fetch` で計画の kind と meta から名前と manifest の entry を作って `assets/` へ移し、残りは消す。取り直すのは、先の download が計画の kind より小さいサイズの上限で止まった場合だけとする。
- 仕事は (a) の 2 つに分け、PF-07 は PF-06 に依存させる。効果の見込み(測った export で約 35%)から、両方を進める。

## 理由

- (B) は、(A) とほぼ同じ所要時間で、stderr、error、exit code、出力を直列の場合と同じに保てる。0045 の表示の型と、CI で安定して読める plain の log を変えずに済む。
- (a) は、所要時間の下限を決める `users.info` の lane と Slack の rate limit を無駄に使わず、除外した投稿の asset を取得しない。成功した export の request の集合が直列と同じになり(例外は「決定性」)、trace と test で確かめやすい。
- method ごとの lane は、Slack の rate limit の単位と今の平準化(0025)に合い、標準ライブラリで書ける。
- 失敗を driver の位置で扱うと、報告は直列の場合と同じになる。止めるのを早める利得は小さい。

## 影響

- `slack-api-usage.md` に「取得の並行化」を足す(PF-06 と PF-07 で実装するまでは今の直列のままであることを、節の冒頭に書く)。`usage-flow.md` の「処理対象の表示」に、並行に取得してもフェーズ行と通知の順を変えないことを書く。`cli-interface.md` の「exit code」に、exit code と error・警告の表示を直列の場合と同じに保つことを書く。`architecture.md` には、実装の PR で内部構成の表を同期する旨と本ログへの参照を足す。
- 0025 と 0067 に追記する。`index.md` に本ログを足し、未決事項「user 解決の最適化」に本ログの見込みを添える。
- PF-06(#278)と PF-07(#279)の本文を、本決定に沿って具体化する。`progress.md` の PF-07 の依存に #278 を足す。
- 実装で変わるもの:
  - PF-06: `internal/slack`(method の lane、`Client` の説明の「Web API は 1 件ずつ」、`History` の page の口、HTTP trace の pacing の待ち)、`internal/export`(先行取得の表と確定の規則、`Run` の終わりの cancel と待ち)、`tools/tracereport`(class ごとの区間)、`cli-interface.md` の HTTP trace の説明(Web API の pacing の待ちの範囲と、download の lane の待ちが始まった download の待ちだけであること)、`architecture.md` の表。
  - PF-07: `internal/lane`(後から job を足せる scheduler。上限と 429 の扱いは 0067 / 0068 のまま)、`internal/output`(一時ファイルと download の結果の保持、`Fetch` で計画の kind と meta から名前と entry を作る移動、通知と警告の計画の順)、`internal/export`(確定した message の asset の計画)、`cli-interface.md` の中断の扱い(一時ファイル)、`architecture.md` の表。
- 固定 sample と README の preview は変わらない(出力が同じため)。demo GIF は PF-07 の後に、ユーザーが手元で再録画する。

## 後から見直す条件

- PF-07 の後の trace で、`users.info` の回数が所要時間の大半を占める場合(未決事項「user 解決の最適化」。#272 の「変えないもの」と 0025 の見直しを伴う)。
- 進捗表示に裏の取得の進み具合を出す必要が出た場合(Users 工程で長く待つ export が問題になった場合など)。
- emoji による除外のような option が増え、確定の規則が保てなくなった場合。
- `conversations.history` の page ごとの先行取得に、測って利得が見られない場合(規則を簡単にする)。
