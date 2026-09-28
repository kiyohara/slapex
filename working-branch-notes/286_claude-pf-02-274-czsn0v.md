# 作業ブランチメモ

- ブランチ: `claude/pf-02-274-czsn0v`(cloud session が指定。Issue の推奨ブランチ名は `perf-plan-asset-downloads`)
- PR: #286
- 最終更新: 2026-09-28

## 目的

Issue #274(PF-02)。所要時間の最小化(#272)に向けて、Assets 工程の描画を 2 回走らせ、asset の取得リストを download の前に確定する。取得は直列のままとし、出力(HTML、assets、manifest、cache、stderr)は変えない。PF-03(#275)の並列取得の前提になる。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 21 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 16 件目である。前のスレッド(#211 / PR #285)の報告の後、ユーザーの判断で PF-02 を FU-12(#245)と FU-09(#210)より前に進めた。

## 現在の状況

- 依存は無い(#274 の「依存・順序」)。推奨の前提だった #211(PR #285)と #192(PR #271)は merge 済み。main `a430231`(PR #285 の merge)から作業した。
- 実装、test、設計文書、decision log 0064 を済ませ、Issue の「検証」を実行した(「検証」)。
- PR #286 作成済み(draft)。採番と `progress.md` の PR 欄の反映を済ませた。
- 次は CI を確かめてから、review を subagent に委譲する(P2)。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `0a83bf7` 時点の記載で、main `a430231` でも同じだった。`export.Run` の Assets 工程は `saveWorkspaceIcon`、`newMessageViewBuilder`(avatar と bot の icon)、`buildTimeline` の順に描画し、`messageView` の `authorAvatar`(bot_profile の icon)、`EmojiHTML`(本文と reaction)、`addImage`、`addAttachmentFile`、`addUnfurls` が `output.Assets.Save` / `SkipTooLarge` を呼ぶ。`Save` はその場で download(または `--reuse-cache` の copy)をし、manifest に記録し、失敗とサイズ上限の警告を出していた。

どの URL を要求するかは `Save` の成否に左右されないことを確かめた。`Save` の結果は path と表示の分岐(`Status`)にだけ使われる。本文の emoji の HTML は mrkdwn 変換が placeholder に退避する(`internal/render/mrkdwn.go` の `stash`)ため、後の解析に影響しない。

### 設計(decision log 0064)

- `output.Assets.Planner` が返す planner は、`Save`、`SkipTooLarge`、`Status` の判断(空の URL の無視、元 URL ごとの重複排除、最初の要求の kind による上限)を通し、要求を最初に要求された順に `PlannedAsset`(kind、元 URL、上限、meta、事前のサイズ判定で除いたか)として記録する。planner は書き込み、reuse の copy、警告をしない。planner の `Save` は常に「保存なし」を返す。
- `Assets.Fetch` が計画を計画の順に直列で取得する。reuse の一致は取得のときに判定する(判定の時点を現行と揃えるため。#202 の同じ directory への再出力も同じ順になる)。結果は `Assets` の表に持ち、manifest には記録しない。download は一時ファイルから最終の path へ rename するため、2 回目に copy し直さない(大きい添付の二重 copy を避ける)。
- 2 回目の描画の `Save` と `SkipTooLarge` が、要求の順に manifest へ記録する。計画に無い URL はその場で取得する。
- 警告は取得の終わりに出す。Issue の「作業内容」は「manifest と警告は 2 回目の呼び出し順で記録し、現行と同じ順序にする」とする。計画の順は 2 回目の呼び出し順と同じなので、警告の順は 2 回目の呼び出し順と一致する。2 回目の描画で出すと、取得中の retry 通知がすべて先に出て stderr の並びが変わり、完了条件(stderr がバイト単位で一致)を満たさないため、出す時点は取得の終わりにした。
- `export.renderWithAssets` が planner への描画、`Fetch`、2 回目の描画を行い、`renderTimeline` が 1 回分の描画(workspace icon、avatar、timeline)を行う。
- 計画は test のために context の値(`assetPlanObserverKey`)で観察する。`export.Options` に test 専用の field を足すと、`cmd/slapex` の `TestExportOptions`(field ごとに設定元が CLI か呼び出し側かを検査する)に例外が増えるため。

### avatar の保存順(PR #281 からの申し送り)

- user の avatar を bot の icon と同じく ID 順に保存する(`newMessageViewBuilder`)。manifest の entry と avatar の警告の順が実行ごとに変わらなくなり、計画と 2 回目の順も揃う。HTML、assets、cache の内容は変わらない(保存名は内容 hash)。
- この PR に含めるかを thread のカードでユーザーに尋ね、推奨の「この PR に含める」で進めた。ユーザーは 2026-09-28 に「この PR に含める」を選んだ。
- ID 順にしたため、`TestRunIntegrationHTTPTrace` の `sortedManifest`(manifest の entry を並べ替えて比べていた)を外した。

### test

- `internal/output`: `TestAssetsPlannerRecordsFirstRequests`(planner の記録、重複、kind ごとの上限、空の URL、`Status`、書き込み・download・警告が無いこと)、`TestAssetsFetchThenSaveMatchesSave`(計画・取得・2 回目の描画が、直接の `Save` と同じ応答、manifest、ファイル、警告、counts、download の順になること。保存、内容 hash の共有、失敗、download の上限、事前のサイズ判定、reuse の copy、上限を超えた reuse の download を含む)、`TestAssetsSaveOutsidePlanAcquiresWhenAsked`(計画外の URL をその場で取得し、そのとき警告すること。`Fetch` の再実行と記録済みの URL)。`fakeDownloader` に呼び出しの記録を足した。
- `internal/export`: `integration_plan_test.go` の `TestRunIntegrationAssetPlanMatchesRender` と `TestRunIntegrationAssetPlanWithReuseCache`。Issue の完了条件に挙がる経路(画像のサムネイルと原本、添付、URL preview の画像と service icon、本文と reaction の emoji、avatar、bot の icon(`bots.info` と bot_profile)、workspace icon、`--reuse-cache` の一致、事前のサイズ判定)に、download の上限、download の失敗、重複を加えた場面で、計画が 2 回目の要求(manifest)と順序まで一致すること、計画の確定時に出力 root に何も無いこと、download が計画の順に走ること、警告が retry 通知との並びを保つことを確かめる。fake server に asset の request の順の記録(`AssetRequests`)、harness に context を渡す口(`runExportScenarioContext`、`runReuseScenarioContext`)を足した。
- `internal/export`: `page_test.go` の `TestNewMessageViewBuilderSavesAvatarsInIDOrder`(user 20 人と bot 5 件で、avatar の要求が ID 順になること)。
- 変異の検出(使い捨ての書き換え。いずれも戻した): `Fetch` を逆順にする、`Save` が取得結果を使わない、planner が重複を記録する、警告を 2 回目の描画で出す、user の avatar を map の順に戻す。5 種すべて、いずれかの test が落ちることを確かめた。avatar の順は `TestNewMessageViewBuilderSavesAvatarsInIDOrder` が 20 回中 20 回落ちた。

### 設計文書・decision log・progress.md

- decision log 0064(`0064-two-pass-asset-planning.md`)を作り、`index.md` に行を足した。
- `architecture.md`: `internal/export` と `internal/output` の責務、Assets 工程の説明を更新した。
- `cache.md`: `assets_manifest.json` の entry の順(描画が asset を求めた順。avatar は ID 順)を書いた。
- `progress.md` の PF-02 の行を `done(PR merge後)`、次にやることを「merge後は対応なし」にした。PR 欄は採番後に記入する。

### 出力生成系 3 skill の適用判断

- `update-sample-exports`: 適用しない。`internal/export/**` と `internal/output/**` を変えたが、サンプル出力に反映される変更ではない(表示変換と保存 path は変えていない)。Issue の「検証」どおり `TZ=Asia/Tokyo`、`-time 2026-07-04T16:32:41+09:00` で別ディレクトリへ再生成し、commit 済みの `doc/samples/ja` / `en` と `diff -r` で無差分だった。sample は commit しない。
- `update-readme-preview-screenshots`: 適用しない。sample export に差分が無い。
- `update-readme-demo-gif`: 適用しない。CLI の出力(phase 名、summary、進捗表示、警告の並び)を変えていない。

## 次にやること

- PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。(完了)
- CI を確かめてから review を subagent に委譲する(P2)。
- avatar の保存順のカードへのユーザーの回答を確かめる。(完了。「この PR に含める」)

## 検証

2026-09-28、cloud session(project の thread)で実行。すべて Docker Compose(`docker compose run --rm dev ...`)経由。実 token は使わず架空 fixture だけを使った。

| 項目 | 結果 |
|---|---|
| `gofmt -l .` | 出力なし |
| `go vet ./...` | pass |
| `go build ./...` | pass |
| `go test ./...` | pass |
| `go test -race ./...`(`CGO_ENABLED=1`) | pass |
| `go test ./internal/export ./internal/output -count=5 -shuffle=on` | pass |
| cross-compile(`CGO_ENABLED=0`、darwin / linux × amd64 / arm64 の `go build ./cmd/slapex`) | 4 通りとも成功 |
| 固定 sample の互換検証(`TZ=Asia/Tokyo`、`go run ./tools/gensample -time 2026-07-04T16:32:41+09:00 -out /tmp/pf02-samples`、`diff -r`) | `doc/samples/ja` / `en` と無差分。footer の時刻は commit 済み sample の Exported 行(`2026-07-04T07:32:41Z`)で確かめた |
| 変更前との出力の比較(使い捨ての test と hook。commit しない) | main `a430231` に user の avatar の ID 順だけを当てたものと本変更で、export の結合 test が harness 経由で走らせる export 100 件と、全経路の場面(新規、reuse、上限を下げた reuse、同じ directory への reuse。reuse は 2 回の実行)の 7 件を比べた。fake server の URL、一時 directory、log の待ち時間の表示を揃えた上で、HTML、assets、cache、stderr が 105 件で一致した。残る 2 件は、実時刻を使う test の時刻と、Messages 工程の Retry-After の jitter(待ち時間の表示が 1s と 2s)による差で、Assets 工程と関係しない |
| `git diff --check` | 出力なし |

## リスク・ブロッカー

- 無し。実 workspace での実行はしていない(出力を変えない変更で、実 token が要るため)。所要時間の変化は、描画 1 回分の CPU 時間が増えるだけで、download の順と pacing は変わらない。

## セッションログ

- 2026-09-28: Issue #274、#274 のコメント(PR #281 からの avatar の保存順の申し送り)、`progress.md`、関連コードを読んだ。依存は無く、推奨の前提の #211 / #192 は merge 済み。
- 2026-09-28: avatar の保存順をこの PR に含めるかを thread のカードで尋ね、推奨(含める)で進めた。
- 2026-09-28: 描画 2 回の計画と取得を実装し、test、設計文書、decision log 0064 を足した。Issue の「検証」と変更前との比較を実行した(「検証」)。
- 2026-09-28: PR #286 を draft で作成し、note を採番した(`bfb43a0`)。`progress.md` の PF-02 の PR 欄を #286 にした(P1)。検証はすべて pass(「検証」)。出力生成系 3 skill は適用しない(「出力生成系 3 skill の適用判断」)。
  - `run-issue-task` の報告から引き上げた項目。`number-working-branch-note` の報告の「書き換えた行の一覧」: note の `PR:` 欄(`未作成` → `#286`)と、PR description の note のファイル名参照 1 行(`draft_` → `286_`)。title は書き換えていない。「触らずに残した行の一覧」: note の「現在の状況」の「次は PR の作成(draft)と採番。」(定型に当てはまらない)と、「次にやること」の「PR を draft で作成し、note を採番する。`progress.md` の PR 欄を反映する。」(`progress.md` の反映を含む複合行)。この 2 行は、`progress.md` の反映の後に書き換えた。PR description と title に触らずに残した行は無い。情報統制チェックで直した箇所は無い。出力生成系 3 skill は呼ばなかった(「いつ使うか」に当たらない)。
  - PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。
