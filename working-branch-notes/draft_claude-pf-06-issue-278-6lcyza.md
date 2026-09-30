# 作業ブランチメモ

- ブランチ: `claude/pf-06-issue-278-6lcyza`(cloud session が指定。Issue の推奨ブランチ名は `perf-api-method-lanes`)
- PR: 未作成
- 最終更新: 2026-09-30

## 目的

Issue #278(PF-06)。PF-05(#277、decision log 0069)で決めた export 全体の並行化のうち、Web API の部分を実装する。`slack.Client` の Web API を method ごとの lane で呼び、`export.Run` の工程は今の順に進めたまま(driver)、後の工程が必ず出す `conversations.replies`、`users.info`、`bots.info`、`emoji.list` を、確定した時点で先に出す(先行取得)。工程の順序、進捗表示、stderr、exit code、出力は直列の場合と同じに保つ。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 28 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #293(PF-05)の merge の後に着手した。

## 現在の状況

- 依存の #277(PF-05、PR #293)は merge 済み。main `30f8192`(PR #293 の merge)から作業している。
- method の lane、`History` の page の口、先行取得と、その test(synctest の lane の test、先行取得の単体 test、先行取得あり・なしを比べる結合 test、export 全体の所要時間の test)まで実装した。残りは `tools/tracereport` の区間、文書、全体の検証、draft PR。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `30f8192` でも同じだった。`slack.Client` の `pace` は並行に安全でない `lastCall` の map で間隔を管理する。`History` は page ごとに件数だけを `progress` へ渡し、`--max-posts` に達した page では `progress` を呼ばずに返る。`export.Run` は `fetchMessages`(`History` の batch、batch ごとの `fetchThread`、`dropExcludedThreads`、補充)、`resolveUsers`(`lookupUsers` は ID の順、`lookupBots`)、`resolveCustomEmoji` の順に Web API を直列に呼ぶ。

### 実装の要点

- method の lane(`internal/slack/methodlane.go`): 待つ呼び出しごとの channel を来た順に並べ、`leave` で先頭の呼び出しへ lane を渡す(lane は busy のまま)。context が終わった呼び出しは lane を取らない(待ちの列から抜けるか、渡された lane を次へ渡す)。間隔の待ちは lane を取った後に client の sleeper で待つため、HTTP trace の pacing の待ちに入り、lane の待ちは入らない。
- `History` の page の口(引数 `kept`): page ごとに、残した message(`--max-posts` の内)を渡す。`--max-posts` に達した page も渡し、その後の `progress` は今どおり呼ばない。
- 先行取得(`internal/export/prefetch.go`): request を method と引数の key で表に 1 回だけ登録する。先に出す request は、`Run` の context の子(`stop` で cancel)と、通知を保つ `slack.WithNotices` で走る。driver が受け取る(`take`)ときに保った通知を `client.Logf` へ出し、その後の通知はそのまま出す。`Run` の defer の `stop` で cancel し、`WaitGroup` で待つ。
- 確定の規則: page の口で、thread は残った親投稿のうち除外されていないものを先に出す(filter ありの broadcast の thread は driver に任せる)。ID は、filter なしでは page の全 message、filter ありでは thread に属さない message から集める。filter なしでは、driver が受け取った thread の replies の ID も先に出す(すべて表示されるため)。Users 工程の開始時に残りの ID をまとめて先に出し、`bots.info` を `users.info` と並行させる。
- test だけの切り替え: context の `prefetchOffKey`(先行取得なし)と `requestTakenKey`(driver が結果を受け取った request と、それが先に出ていたか)。利用者向けの option は足していない。
- fake server: API の障害を `requestName`(path と、user / bot / ts / latest の値)の key でも注入できるようにし、`BeforeAPI` hook と `HistoryPageSize`(cursor による page 分け)を足した。harness は `Run` が返った後の request を失敗にする。
- export 全体の所要時間(synctest、各 request 100ms、thread 3 件、user 5 人、avatar 5 件): 直列は Web API 6.7 秒、run 6.8 秒。先行取得ありは Web API 4.5 秒、run 4.6 秒。

## 次にやること

- `tools/tracereport` の区間、文書の更新、全体の検証、draft PR の作成。

## 検証

- 途中(2026-09-30): Docker Compose で `go vet ./...`、`go test ./...`、`internal/slack` と `internal/export` の `go test -race` が通った。全体の検証は未実施。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-29: 着手。Issue #278、handoff(`issue-277-pr-293-report.md`)、decision log 0069、`slack-api-usage.md` の「取得の並行化」を確認した。
- 2026-09-30: method の lane、`History` の page の口、先行取得とその test を実装し、途中の状態を push した。
