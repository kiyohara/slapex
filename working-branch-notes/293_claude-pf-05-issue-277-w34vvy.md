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
- method ごとの lane: 同時 1 件、来た順、前の呼び出しの開始から 1 秒以上(今の `pace` と同じ)。429 の `Retry-After` は同じ method だけを待たせる。
- 失敗: 先行取得の成否と通知は driver が結果を受け取るときに直列の順で出す。先行取得の失敗で早く止めない。`export.Run` が返るときに先行取得を止めて待つ。
- 出力: 先に download した asset は一時ファイルに置き、`Fetch` が計画の順に扱うときに `assets/` へ移す。残りは消す。
- 進捗表示: 変えない。別 Issue に分けるものは無い。demo GIF は時間だけが変わり、PF-07 の後にユーザーが手元で再録画する。
- 仕事の分け方: PF-06 は Web API、PF-07 は asset。PF-07 は PF-06 に依存する(`progress.md` の依存に #278 を足した)。

### 出力生成系 skill の判断

3 skill とも使わない。変更は設計文書、decision log、`progress.md`、note だけで、CLI の出力、HTML、fixture、README の media に関わらない。

## 次にやること

- `git diff --check` と文体を確かめ、commit と push をする。
- PR を draft で作り、note を採番し、`progress.md` の PR 欄を反映する。
- #278 と #279 の本文を、0069 に沿って具体化する。
- review(P2)と指摘への対応(P4)、再確認(P5)。
- review cycle を終えたら PR を Ready for review にする(P6)。

## 検証

## リスク・ブロッカー

- 見込みは trace の request の時間を並べ替えた推定である。PF-06 と PF-07 の test(synctest の export 全体)と、PF-07 の後のユーザーの trace で確かめる。

## セッションログ

- 2026-09-29: 着手。Issue、#272、PF-01〜PF-04 の PR と note、取得工程の code を読み、#272 の PF-03 の後の trace で効果を見積もって、decision log 0069 と設計文書を書いた。
