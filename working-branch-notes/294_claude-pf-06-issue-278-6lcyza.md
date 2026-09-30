# 作業ブランチメモ

- ブランチ: `claude/pf-06-issue-278-6lcyza`(cloud session が指定。Issue の推奨ブランチ名は `perf-api-method-lanes`)
- PR: #294
- 最終更新: 2026-09-30

## 目的

Issue #278(PF-06)。PF-05(#277、decision log 0069)で決めた export 全体の並行化のうち、Web API の部分を実装する。`slack.Client` の Web API を method ごとの lane で呼び、`export.Run` の工程は今の順に進めたまま(driver)、後の工程が必ず出す `conversations.replies`、`users.info`、`bots.info`、`emoji.list` を、確定した時点で先に出す(先行取得)。工程の順序、進捗表示、stderr、exit code、出力は直列の場合と同じに保つ。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 28 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #293(PF-05)の merge の後に着手した。

## 現在の状況

- 依存の #277(PF-05、PR #293)は merge 済み。main `30f8192`(PR #293 の merge)から作業している。
- 実装、test、`tools/tracereport` の区間、文書の更新、Docker Compose での全体の検証まで済み、PR #294 を作った。review cycle `claude-code-560e493-20260930004425` の指摘 1 件に対応し、再確認で修正確認済みになった。PR を Ready for review にした。残りは人間の手番である。

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

- 人間: 指摘の thread(1 件、`**修正確認済み(resolve 可)**`)の resolve。
- 人間: Codex のクロスレビューと PR の merge。

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

- 追加の反復(2026-09-30): `internal/slack`、`internal/export`、`tools/tracereport` の `-count=20 -shuffle=on` と `GOMAXPROCS=1 -count=5` は成功した。cross compile(`CGO_ENABLED=0`、darwin / linux × amd64 / arm64)も成功した。
- `-race -count=15`(同じ 3 package)で、既存の `TestTraceTimeoutClass/http2=false` が 1 回落ちた(trace が 13 行で、18 行を期待)。この PR の test は落ちていない。main `30f8192` の worktree で、`internal/export` の `-race -count=15` を並行に走らせた負荷の下、`-race -count=300 -run TestTraceTimeoutClass` を回すと、main でも 1 回落ちた(この branch でも 300 回中 1 回)。落ちた回の記録では、`auth.test` のある試行が status 200 を受け取り、その後の試行が無かった。fake server の handler が server 側の request の context の終わりで普通に return して 200 の空の応答を返し、client がそれを timeout の間際に受け取ると、decode に失敗して retry せずに終わる、とみている(推定)。handler で context の終わりに `panic(http.ErrAbortHandler)` とする修正を main の worktree で試すと、同じ条件で 300 回落ちなかった(1/300 との比較のため確証は弱い)。この PR の範囲外のため直さず、follow-up 候補として PR description に書いた。
- 変異 test: 次の 14 の変異を 1 つずつ入れ、それぞれ対応する test が落ちることを確かめた(変異は戻した)。先行取得: filter ありでも thread の message の ID を先に出す、broadcast の thread も先に出す、通知を保たずに出す、`stop` で待たない、reply の ID を届いた時点で出さない、保った通知を捨てる、filter ありでも reply の ID を先に出す。method の lane: 同時数を限らない、待つ順を逆にする、1 秒の間隔を空けない、cancel された呼び出しが渡された lane を次へ渡さない、cancel された呼び出しが lane を取る。tracereport: 区間の終わりを延ばさない、download の区間に request の終わりを使わない。
- P4(2026-09-30): 比較の表に行を足した後、`gofmt -l .`、`go vet ./...`、`go build ./...`、`go test ./...`、`-race`(同じ 3 package)、`TestRunIntegrationPrefetch` の `-race -count=20 -shuffle=on` と `GOMAXPROCS=1 -count=10`、`git diff --check` が成功した。broadcast の thread も先に出す変異を入れると、足した行と既存の `--max-posts` の外の行がともに落ちることを確かめた(変異は戻した)。

## P1 の記録(drive-issue-to-reviewed-pr)

- PR: #294(draft)。検証結果は「検証」、出力生成系 skill の判断は「出力生成系 skill の判断」のとおり。
- `run-issue-task` の手順で使った skill の報告から引き上げた項目:
  - `number-working-branch-note`(commit `270f70e`、`draft_claude-pf-06-issue-278-6lcyza.md` → `294_claude-pf-06-issue-278-6lcyza.md`、push 成功)。
    - 書き換えた行の一覧: note の `- PR: 未作成` → `- PR: #294`(`PR:` 欄の記入)。PR description の `working-branch-notes/draft_claude-pf-06-issue-278-6lcyza.md` → `working-branch-notes/294_claude-pf-06-issue-278-6lcyza.md`(ファイル名参照の置換)。title は無し。
    - 触らずに残した行の一覧: note の「現在の状況」の「次は draft PR の作成。」(定型に当てはまらない)、note の「次にやること」の「draft PR を作り、note を採番し、`progress.md` の PR 欄を反映する。」(複合行。`progress.md` は同 skill の対象外)、PR description の note 参照の後の「(採番後に rename する)」(置換後の文脈が不自然)、PR description の「(PR 欄は採番後に反映する)」(定型に当てはまらない)。これらは skill の後に、`progress.md` の PR 欄の反映と合わせて orchestrator が更新した。
  - 出力生成系 3 skill: 呼ばなかった(`update-sample-exports` と `update-readme-preview-screenshots` は「いつ使うか」に当たらない。`update-readme-demo-gif` は cloud session で実行できないため、ローカルで要再生成として未検証事項に残した)。

## P2 / P3 の記録

- review cycle: `claude-code-560e493-20260930004425`。Reviewed head: `560e49338e5f180def903a8054752656eacc205a`。
- 指摘 1 件(inline 1、top-level 0)。prefix ごとでは `[must]` 0、`[ask]` 0、`[imo]` 1、`[nits]` 0、`[fyi]` 0。
  - `[imo]` `internal/export/integration_prefetch_test.go`: 比較の表に、親が範囲外の broadcast の置き方が無い(完了条件と 0069 の追記の文との対応)。
- subagent の報告: `gh` への fallback なし、停止理由なし、訂正できなかった誤りなし。check runs は 5 件とも success。
- P3: 指摘が 1 件のため P4 へ進んだ。

## P4 の記録

- 処置: 採用し修正した 1 件(上の `[imo]`)。スコープ外とした指摘は無く、follow-up 候補は増えていない。
- 修正 commit: `7e8e4d3`。比較の表に、親が取得の範囲より古い置き方の行を足し、2 つの broadcast の行で scenario と option の組立を共有した。PR description の「検証」の scenario の列挙にも足した。
- 出力生成系 skill の再判断: test だけの変更のため、P1 の判断(sample と screenshot は使わない、demo GIF はローカルで要再生成)を変えない。

## P5 の記録

- 再確認(review cycle `claude-code-560e493-20260930004425`、Reviewed head `b3cc2179a2087a2ef53d4898d0016011c3bc5a7b`): 修正確認済み 1 件、スコープ外として確認済み 0 件、対応不要として確認済み 0 件、未対応 0 件。resolve 可とした thread は 1 件(`internal/export/integration_prefetch_test.go` の `[imo]`)。
- subagent の報告: `gh` への fallback なし、停止理由なし、訂正できなかった誤りなし。check runs は 5 件とも success。
- P5 後の判断: 未対応が 0 件のため P6 へ進んだ。

## P6 の記録

- 終了時の状態: PR #294 を Ready for review にした。P5 が確かめた head `b3cc217` の check runs は 5 件とも success。この記録を足した note だけの commit が最新 head になる。
- 人間に残る作業: 指摘の thread の resolve、Codex のクロスレビュー、PR の merge。metadata の誤りを訂正できなかった投稿は無い。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-29: 着手。Issue #278、handoff(`issue-277-pr-293-report.md`)、decision log 0069、`slack-api-usage.md` の「取得の並行化」を確認した。
- 2026-09-30: method の lane、`History` の page の口、先行取得とその test を実装し、途中の状態を push した。
- 2026-09-30: reply にだけ現れる user の `users.info` を replies が届いた時点で先に出すよう改め、`tools/tracereport` の区間、文書、decision log 0069 の追記を足した。Docker Compose で全体を検証した。
- 2026-09-30: P1 を終えた。draft PR #294 を作り、note を採番し(`270f70e`)、`progress.md` の PR 欄を反映した。既存の `TestTraceTimeoutClass` の不安定さを main でも再現し、follow-up 候補にした。
- 2026-09-30: P2 の review を subagent に委譲した(指摘 1 件、`[imo]`)。P3 で P4 へ進んだ。
- 2026-09-30: P4 で、比較の表に範囲外の除外された親の broadcast の行を足した(`7e8e4d3`)。
- 2026-09-30: P5 の再確認で修正確認済みになった(未対応 0 件)。P6 で PR を Ready for review にした。
