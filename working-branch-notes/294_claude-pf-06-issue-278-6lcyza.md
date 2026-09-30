# 作業ブランチメモ

- ブランチ: `claude/pf-06-issue-278-6lcyza`(cloud session が指定。Issue の推奨ブランチ名は `perf-api-method-lanes`)
- PR: #294
- 最終更新: 2026-09-30

## 目的

Issue #278(PF-06)。PF-05(#277、decision log 0069)で決めた export 全体の並行化のうち、Web API の部分を実装する。`slack.Client` の Web API を method ごとの lane で呼び、`export.Run` の工程は今の順に進めたまま(driver)、後の工程が必ず出す `conversations.replies`、`users.info`、`bots.info`、`emoji.list` を、確定した時点で先に出す(先行取得)。工程の順序、進捗表示、stderr、exit code、出力は直列の場合と同じに保つ。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 28 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #293(PF-05)の merge の後に着手した。

## 現在の状況

- 依存の #277(PF-05、PR #293)は merge 済み。main `30f8192`(PR #293 の merge)から作業している。
- 実装、test、`tools/tracereport` の区間、文書の更新、Docker Compose での全体の検証まで済んだ。次は draft PR の作成。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `30f8192` でも同じだった。`slack.Client` の `pace` は並行に安全でない `lastCall` の map で間隔を管理する。`History` は page ごとに件数だけを `progress` へ渡し、`--max-posts` に達した page では `progress` を呼ばずに返る。`export.Run` は `fetchMessages`(`History` の batch、batch ごとの `fetchThread`、`dropExcludedThreads`、補充)、`resolveUsers`(`lookupUsers` は ID の順、`lookupBots`)、`resolveCustomEmoji` の順に Web API を直列に呼ぶ。

### 実装の要点

- method の lane(`internal/slack/methodlane.go`): 待つ呼び出しごとの channel を来た順に並べ、`leave` で先頭の呼び出しへ lane を渡す(lane は busy のまま)。context が終わった呼び出しは lane を取らない(待ちの列から抜けるか、渡された lane を次へ渡す)。間隔の待ちは lane を取った後に client の sleeper で待つため、HTTP trace の pacing の待ちに入り、lane の待ちは入らない。
- `History` の page の口(引数 `kept`): page ごとに、残した message(`--max-posts` の内)を渡す。`--max-posts` に達した page も渡し、その後の `progress` は今どおり呼ばない。
- 先行取得(`internal/export/prefetch.go`): request を method と引数の key で表に 1 回だけ登録する。先に出す request は、`Run` の context の子(`stop` で cancel)と、通知を保つ `slack.WithNotices` で走る。driver が受け取る(`take`)ときに保った通知を `client.Logf` へ出し、その後の通知はそのまま出す。`Run` の defer の `stop` で cancel し、`WaitGroup` で待つ。
- 確定の規則: page の口で、thread は残った親投稿のうち除外されていないものを先に出す(filter ありの broadcast の thread は driver に任せる)。ID は、filter なしでは page の全 message、filter ありでは thread に属さない message から集める。filter なしでは、先に出した thread の replies が届いた時点で、replies の ID も先に出す(すべて表示されるため)。当初は driver が thread を受け取った時点で出していたが、replies にしか現れない user の `users.info` が driver の進みを待つため、届いた時点に改めた。Users 工程の開始時に残りの ID をまとめて先に出し、`bots.info` を `users.info` と並行させる。
- test だけの切り替え: context の `prefetchOffKey`(先行取得なし)と `requestTakenKey`(driver が結果を受け取った request と、それが先に出ていたか)。利用者向けの option は足していない。
- fake server: API の障害を `requestName`(path と、user / bot / ts / latest の値)の key でも注入できるようにし、`BeforeAPI` hook と `HistoryPageSize`(cursor による page 分け)を足した。harness は `Run` が返った後の request を失敗にする。
- `tools/tracereport`: class ごとに request が続いた区間の表(run の開始から、最初の request の開始と最後の request の終わり)を足した。class ごとの行に加えて、download の全 class を合わせた `All downloads` と、全 request の `All` の行を置く。`All downloads` は、0069 の「検証の方法」の比べる値(download の最初の request が始まる時点と最後の request が終わる時点)をそのまま出すために足した(Issue の本文にない追加)。
- export 全体の所要時間(synctest、各 request 100ms、thread 3 件、各 thread に reply 1 件): user 5 人が page に現れる場面は、直列が Web API 6.7 秒 / run 6.8 秒、先行取得ありが 4.5 秒 / 4.6 秒。user 1 人が page に、3 人が reply にだけ現れる場面は、直列が 5.7 秒 / 5.8 秒、先行取得ありが 3.5 秒 / 3.6 秒。decision log 0069 に追記した。

### 文書

- `slack-api-usage.md`「取得の並行化」の冒頭: 実装までは直列のままとする文から Web API の部分を外し、asset の部分だけを PF-07 までの扱いとして残した。
- `cli-interface.md` の HTTP trace: Web API の pacing の待ちは lane の先頭に来た後の 1 秒の間隔の待ちだけで、lane の待ちは記録しないこと、download の lane の待ちは始まった download が `lane.Wait` で待った時間だけであることを書いた。`run` の行は tracereport の区間の起点にも使うことを足した。
- `architecture.md`: 内部構成の表の `internal/export` と `internal/slack` の行、`export.Run` の説明、tracereport の説明、並行化の段落(PF-06 は実装済み、PF-07 の予定)を更新した。
- decision log 0069 に「追記(2026-09-30): PF-06 の実装」を足し、`index.md` の 0069 の行に実装の状況を書いた。`progress.md` の PF-06 を `done(PR merge後)` にし、PF-07 の次にやることを更新した。

### 出力生成系 skill の判断

- `update-sample-exports`: 使わない。`internal/export/**` を変えたが、表示変換、asset の path、保存仕様は変えていない。固定条件(`TZ=Asia/Tokyo`、`-time 2026-07-04T16:32:41+09:00`)で別ディレクトリへ作り直し、`doc/samples/ja` と `doc/samples/en`(各 18 ファイル)と `diff -r` で差分が無いことを確かめた。
- `update-readme-preview-screenshots`: 使わない。出力の HTML、CSS、assets、sample は変わらない。
- `update-readme-demo-gif`: ローカルで要再生成(PF-07 の後にまとめて)。demo GIF は実際の pacing で録画するため、工程の終わる時間が早まる(表示の文言は変わらない)。cloud session では録画できないため、0069 のとおり PF-07 の後にユーザーが手元で再録画する。

## 次にやること

- draft PR を作り、note を採番し、`progress.md` の PR 欄を反映する。
- review(P2)以降。

## 検証

2026-09-30、Docker Compose(`docker compose run --rm dev ...`)で実行した。

| 検証 | 結果 |
|---|---|
| `gofmt -l .` | 出力なし |
| `go vet ./...` | 成功 |
| `go test ./...` | 成功 |
| `go test -race ./...`(`-e CGO_ENABLED=1`) | 成功 |
| 固定 sample の互換(`TZ=Asia/Tokyo`、`go run ./tools/gensample -time 2026-07-04T16:32:41+09:00 -out <gitignore 済みの一時 directory>`) | `doc/samples/ja` / `en` と `diff -r` で無差分(各 18 ファイル) |
| `git diff --check` | 問題なし |

- 変異 test: lane の同時 1 件を外す、FIFO を崩す、間隔を lane の前で待つ、先に出した request の通知をすぐ出す、`stop` で待たない、同じ request を 2 回出す、reply の ID を先に出さない、の各変異を入れ、対応する test が失敗することを確かめた(変異は戻した)。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-29: 着手。Issue #278、handoff(`issue-277-pr-293-report.md`)、decision log 0069、`slack-api-usage.md` の「取得の並行化」を確認した。
- 2026-09-30: method の lane、`History` の page の口、先行取得とその test を実装し、途中の状態を push した。
- 2026-09-30: reply にだけ現れる user の `users.info` を replies が届いた時点で先に出すよう改め、`tools/tracereport` の区間、文書、decision log 0069 の追記を足した。Docker Compose で全体を検証した。
