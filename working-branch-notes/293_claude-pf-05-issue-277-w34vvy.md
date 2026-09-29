# 作業ブランチメモ

- ブランチ: `claude/pf-05-issue-277-w34vvy`(cloud session が指定。Issue の推奨ブランチ名は `perf-export-scheduler-design`)
- PR: #293
- 最終更新: 2026-09-29

## 目的

Issue #277(PF-05)。export 全体の並行化(#272 の段階 3)の設計を決め、decision log と設計文書に書く。Web API の method ごとの lane と、API の待ち時間の裏での asset の取得の実装(PF-06 #278、PF-07 #279)の前に、工程の順序、進捗表示、失敗時の扱い、先読みの方針、決定性を決め、PF-06 と PF-07 の本文を具体化する。code は変えない。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 27 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の thread である。PR #292(PF-04)の merge の後に着手した。

## 現在の状況

- 依存の #273(PF-01、PR #281)と #275(PF-03、PR #291)は merge 済み。main `fbd7cae`(PR #292 の merge)から作業した。
- ユーザーが手元で取った PF-03 の後の trace の集計(#272 のコメント、2026-09-29 13:33Z、main `fbd7cae`)を効果の見込みの材料にした。
- decision log 0069 と設計文書を書いた(「決定事項」)。
- PR #293 を draft で作り、note を採番した。Issue #278(PF-06)と #279(PF-07)の本文を、0069 に沿って具体化した。
- review(cycle `claude-code-1fcabdc-20260929212208`)の指摘 3 件([must] 3)を受け、0069 と `slack-api-usage.md` の「取得の並行化」、#278 と #279 の本文を直した(P4)。
- 再確認(P5)で、指摘 3 件とも修正確認済み(resolve 可)になり、未対応は 0 件だった。review cycle `claude-code-1fcabdc-20260929212208` は完了した。
- 終了時の状態(P6): head `181cfea` の check runs は 5 件すべて success。note だけの commit の push の後に、PR を Ready for review にする。

## 決定事項

### 着手時の現行コード

Issue の関数名は main `0a83bf7` 時点の記載で、main `fbd7cae` でも同じだった。`export.Run` は Workspace(`auth.test`、`team.info`)、Channel(`conversations.list`)、`--reuse-cache` の検証と出力先の作成、Messages(`fetchMessages`: `History` の batch、batch ごとの `fetchThread`、`dropExcludedThreads`、補充)、Users(`lookupUsers` は ID の順、`lookupBots`)、Emoji(`emoji.list`)、Assets(`renderWithAssets`: 描画 2 回と `Fetch`)の順に進む。`slack.Client` の `pace` は並行に安全でない `lastCall` の map で、呼び出し(page ごと)の開始の前に時刻を記録する。`History` は page ごとに `call` を呼び、filter を通った message を `--max-posts` の内で残し、page ごとに件数だけを `progress` へ渡す。

### 効果の見込み(#272 の trace から)

main `fbd7cae` の run 19.064 秒の内訳: Web API 22 回が 15.693 秒の時点まで直列(`auth.test`、`team.info`、`conversations.list`、`conversations.history` で 0.891 秒、`conversations.replies` 4 回で 3.222 秒、`users.info` 12 回で 11.192 秒、`bots.info` と `emoji.list` で 0.365 秒)、その後に download 56 件の 3.349 秒(DNS の解決の遅れを含む)。

- PF-06(method の lane と Web API の先行取得): 0.891 + 11.192 ≒ 12.1 秒で Web API が終わり、download を足して約 15.5 秒(約 3.6 秒、19% 減)。
- PF-07(asset の先行取得): download が `users.info` の待ちの裏に隠れ、最後の avatar と描画が残って約 12.5 秒(合わせて約 6.6 秒、35% 減)。
- 下限は `users.info` の回数 × 1 秒。0025 の見直し条件(`users.info` が支配項)に当たる状態で、index の未決事項「user 解決の最適化」で扱う(#272 の「変えないもの」のため本 Issue では扱わない)。
- 効果は小さくないと判断し、PF-06 と PF-07 を進める前提で書いた(Issue の完了条件の「効果が小さい場合はユーザーに確認」には当たらない)。

### 設計(decision log 0069)

- 並行の形: 工程は今の順に 1 つずつ進め(driver)、後の工程が必ず出す request を確定した時点で先に出す(先行取得)。工程を同時に走らせる案は、所要時間が同じで、フェーズ行の型(0045)、plain の log の順、error の順を変えるため採らない。
- 先行取得の範囲: 確定した request だけ。確定の規則は `slack-api-usage.md` の「取得の並行化」の表。filter を指定した場合、thread に属する message は Messages 工程の終わりに確定する。見込みで出す案は、`users.info` の lane を無駄に使い、除外した投稿の asset を取得するため採らない。
- method ごとの lane: 同時 1 件、来た順、前の呼び出しの開始から 1 秒以上(今の `pace` と同じ)。429 の `Retry-After` は同じ method だけを待たせる。HTTP trace の pacing の待ちは、lane の先頭に来た後の間隔の待ちだけとし、lane で前の呼び出しを待つ時間は記録しない(download の `lane.Run` が始めるまでの待ちと同じ扱い)。
- 失敗: 先行取得の成否と通知は driver が結果を受け取るときに直列の順で出す。先行取得の失敗で早く止めない。`export.Run` が返るときに先行取得を止めて待つ。
- 出力: 先に download した asset は一時ファイルに置き、`Fetch` が計画の順に扱うときに、保った結果と計画の kind・meta から名前と manifest の entry を作って `assets/` へ移す。meta の違い(custom emoji の alias など)では取り直さない。取り直すのは、先の download が計画の kind より小さい上限で止まった場合だけで、同じ URL を kind や file object のサイズが食い違う要求が求める場合に限り、request が直列より 1 件増えることがある。残りは消す。
- 検証: 結合 test の request の件数の一致は成功する scenario に限る。失敗と cancel では、error、exit code、stderr、出力先のファイルと、request が失敗しない場合に出るものに含まれることを比べる。PF-07 の後の trace は、run の時間、Web API が終わる時点、download の区間、request の件数、method ごとの pacing の待ちで比べ、PF-06 で `tools/tracereport` に class ごとの区間を足す。
- 進捗表示: 変えない。別 Issue に分けるものは無い。demo GIF は時間だけが変わり、PF-07 の後にユーザーが手元で再録画する。
- 仕事の分け方: PF-06 は Web API、PF-07 は asset。PF-07 は PF-06 に依存する(`progress.md` の依存に #278 を足した)。

### 出力生成系 skill の判断

3 skill とも使わない。変更は設計文書、decision log、`progress.md`、note だけで、CLI の出力、HTML、fixture、README の media に関わらない。

## 次にやること

- `git diff --check` と文体を確かめ、commit と push をする。(完了)
- PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。(完了)
- #278 と #279 の本文を、0069 に沿って具体化する。(完了)
- review(P2)。(完了)
- 指摘への対応(P4): 修正の push、#278 と #279 の本文の修正、各指摘への処置の返信。(完了)
- 再確認(P5)。(完了。未対応 0 件で review cycle を完了した)
- review cycle を終えたら PR を Ready for review にする(P6)。(完了)
- 人間: resolve 可とした 3 thread(Claude の cycle)を GitHub の UI で resolve する。
- Codex: Ready for review の PR のクロスレビュー(ユーザーの Codex が行う)。
- 人間: PR を merge する(PF-06 と PF-07 を進めるかの判断を含む。#272 の「ユーザーが行うこと」の 3)。

## 検証

- `git diff --check`: 問題なし。
- 変更が設計文書、decision log、`progress.md`、note だけであることを `git diff --name-only` で確かめた。
- 文体: 追加した行に「です」「ます」が無いことを確かめた(開発者向けの常体)。
- 追加した decision log と設計文書のリンク先が存在することを確かめた。
- code を変えないため、test と build は実行していない。
- P4 の修正(`32d3058`): `git diff --check` は問題なし。変更は 0069 と `slack-api-usage.md` だけで、追加した行に「です」「ます」は無い。指摘の前提を code で確かめた(`Assets.addToPlan`、`download`、`extensionFor`、`limitFor`、`EmojiHTML` と `emoji.Resolver.Resolve`、`ErrTooLarge`、`lane.Run` と `lane.Wait` の説明、`internal/slack/trace.go` の説明、`tools/tracereport` の出力)。

## リスク・ブロッカー

- 見込みは trace の request の時間を並べ替えた推定である。PF-06 と PF-07 の test(synctest の export 全体)と、PF-07 の後のユーザーの trace で確かめる。

## セッションログ

- 2026-09-29: 着手。Issue、#272、PF-01〜PF-04 の PR と note、取得工程の code を読み、#272 の PF-03 の後の trace で効果を見積もって、decision log 0069 と設計文書を書いた。
- 2026-09-29: P1。PR #293 を draft で作った(`Closes #277`)。reviewer に kiyohara を指定したが、PR の作成者のため GitHub が受け付けなかった(assignee は設定済み)。`number-working-branch-note` で note を採番し(`3caa0fa`)、`progress.md` の PF-05 の PR 欄を #293 にした。#278 と #279 の本文を具体化した。出力生成系 3 skill は使わない(「決定事項」)。
  - 採番の報告(確認経路): 書き換えた行は、note の `PR:` 欄(`未作成` → `#293`)と、PR description の note 参照(`draft_` → `293_`)の 2 行。状況を説明する stale 表現と完了タスク行の書き換えは 0 件。title は変えていない。
  - 採番の報告(残された事項): 触らずに残した行は、note の「次にやること」の「PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。」の 1 行(複合行。`progress.md` の反映は採番では完了しない)。PR description と title は 0 件。その後、`progress.md` を反映したため、この行は orchestrator が完了にした。
- 2026-09-29: P2 / P3。review cycle `claude-code-1fcabdc-20260929212208`、Reviewed head `1fcabdc`。指摘 3 件(inline 3、top-level 0。[must] 3、[ask] 0、[imo] 0、[nits] 0、[fyi] 0)。review の途中で、25 件目の thread から届いた trace の読み解き(download の trace の lane の待ちは `lane.Wait` の分だけ)を、確かめる点として subagent に足した(3 件目の指摘になった)。1 件以上のため P4 へ進んだ。
- 2026-09-29: P4。3 件とも「採用し修正した」(`32d3058`)。
  - L106(決定性): 名前と manifest の entry を計画の kind・meta から作り、meta の違いでは取り直さない。取り直すのは、計画の kind より小さい上限で止まった場合だけ。request が直列より増える例外を 0069 と `slack-api-usage.md` に書き、#279 の作業内容 2 と完了条件を直した。
  - L118(結合 test): request の件数の一致を成功する scenario に限り、失敗と cancel の比べ方を足した。#278 と #279 の完了条件を直した。
  - L88(trace): pacing の待ちの範囲を決め、L119 に比べる値と `tools/tracereport` の区間を書いた。#278 の作業内容 1 と文書、完了条件を直した。`cli-interface.md` の `SLAPEX_HTTP_TRACE` の download の lane の待ちの記述(main の既存の文)は、#278 の作業内容で直す。
  - follow-up 候補: なし。出力生成系 3 skill: 変更は decision log と設計文書だけで、使わない判断は変わらない。
- 2026-09-29: P5。再確認を P2 と同じ subagent に委譲した(head `181cfea`)。修正確認済み 3 件(resolve 可)、スコープ外として確認済み 0、対応不要として確認済み 0、未対応 0 件。`gh` への fallback は無し。訂正できなかった metadata の誤りは無し。review cycle を完了し、P6 に進んだ。
  - reviewer の補足(指摘ではない): `--reuse-cache` の copy も、先に download したときの kind の上限では copy できず、計画の kind では copy できる場合に request が 1 件増えるが、kind が食い違う場合の例外に収まる(PF-07 の実装で扱う)。0069 の「決定性」の上限の行は、計画の上限以上の上限で止まった先の download を直接は書いていないが、保った失敗から entry を作る前の行と `slack-api-usage.md` の規則で扱われる。
- 2026-09-29: 終了時の状態(P6): head `181cfea` の check runs は 5 件すべて success。note だけの commit を push した後に、PR を Ready for review にする。follow-up 候補は無し。
