# 作業ブランチメモ

- ブランチ: `claude/project-thread-4l4ad6`(cloud session が指定。Issue の推奨ブランチ名は `check-phase-completion-on-pr-entry`)
- PR:
- 最終更新: 2026-09-24

## 目的

Issue #234。`drive-issue-to-reviewed-pr` を PR 番号の入口から再開するとき、開始フェーズより前のフェーズの完了条件を確かめる規定を足す。P1 の途中で途切れた場合の note の採番と `progress.md` の反映、P1 と P4 の後の check runs の確認を、入口から飛ばさないようにする。

本 Issue は、#216、#222、#228 と並行して進める。`doc/guidelines/issue-driven-task-execution.md` の「前提」(タスクは直列に消化する。decision log 0037)の例外で、cloud での並列実行を試すユーザーの指示(2026-09-24)による。本 Issue の処理は、PR #233 で入った `drive-issue-to-reviewed-pr` の手順で行う。

## 現在の状況

- 作業内容 1〜4 を実施し、Issue の「検証」を実行した(P1)。
- 変更: `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md` の「入力と入口」(理由の 1 文)、「PR から始める場合」(push できない場合の手順と、P2 から始める前の P1 の確認)、「CI の確認点」(確認点を委譲の直前へ)。decision log 0059 に追記した。

## 決定事項

- check runs の確認は、Issue のコメントの案(bizdate と同じく、P2 と P5 を委譲する直前に確かめる形)を採った。通常の流れでは現行の確認点(P1 の完了時、P4 の push 後)と同じ時点になり、入口ごとに書き分けずに済む。PR の入口に足すのは、P1 の完了条件のうち check runs 以外(note の採番、`progress.md` の PR 欄)の確認だけにした。
- 未決事項 1(P5 の開始時の check runs の確認)と 2(decision log は 0059 への追記)は、仮決定のまま進めた。
- check runs が失敗した場合の扱い(差分に起因すれば直して再 push、説明できなければ止まる)は変えない。bizdate は PR の外に原因がある失敗を brief に明記して委譲するが、本 Issue の範囲外とした。
- PR の head branch へ push できない場合に止まる手順に、P1 の残りの手順を足した。採番と `progress.md` の反映は push を伴う。
- 「入力と入口」の理由の文を「P2 以降で途切れた場合など」から「PR の作成後に途切れた場合など」に改めた。P1 の途中で途切れた場合も PR の入口へ入るようになったためである。
- 「フェーズ」の表、「head SHA の受け渡し」、「working branch note」の節は変えていない。
- `doc/design/decision-log/index.md` の 0059 の行は変えない。行は CI の確認点と PR の入口に触れておらず、今回の追記で古くならない。並行する #228 / #216 / #222 が同じ表に行を足すため、衝突も避けられる。
- `address-comments` などの手順番号は書かない。並行する #216 が `address-comments.md` の手順番号をずらすためである。
- `progress.md` は索引外の単発 Issue のため更新しない。
- 出力生成系 3 skill は、ドキュメントだけの変更で各 skill の「いつ使うか」に当たらないため適用しない。

## 次にやること

- PR を作成し、note を採番する(P1)。
- 最新 head の check runs を確かめ、P2 を subagent に委譲する。

## 検証

2026-09-24、cloud session(project の thread)で実行。

### 再開の状態ごとの開始フェーズと、その前に行う手順

変更後の SKILL.md を読み、各状態で開始フェーズと、その前に行う手順が一意に決まることを確かめた。

| 状態 | 開始フェーズ | 開始前に行う手順 | 根拠 |
|---|---|---|---|
| PR は open、note が採番前(`draft_` のまま)、review cycle が無い | P2 | `number-working-branch-note` で採番して push する。関連 Issue が索引にあれば `progress.md` の PR 欄も確かめる。採番後の head の check runs を、P2 を委譲する直前に確かめる | 「PR から始める場合」の表の 1 行目と、表の後の P1 の確認。「CI の確認点」 |
| 採番済み、索引にある Issue の `progress.md` の PR 欄が未反映、review cycle が無い | P2 | `run-issue-task` の手順どおりに PR 欄を更新して push する。その head の check runs を、P2 を委譲する直前に確かめる | 同上 |
| 採番済み、最新 head の check runs が未完了または失敗、review cycle が無い | P2 | 未完了なら完了を待つ。失敗なら、差分に起因すれば直して再 push し、完了を待ち直す。差分から説明できなければ止まる | 「CI の確認点」、「停止とエスカレーション」 |
| P1 の完了条件をすべて満たし、review cycle が無い | P2 | 追加の手順は無い(確認だけ)。従来どおり | 同上 |
| 対応が要る指摘のすべてに `address-comments` の返信があり、最新 head の check runs が未完了 | P5 | 完了を待ち、success を確かめてから P5 を委譲する。失敗した場合は上の行と同じ | 表の 3 行目、「CI の確認点」 |
| (追加)Issue を入力し、それを `Closes` する open PR があり、P1 の途中で途切れている | P2 | PR の入口として 1〜3 行目と同じ手順を通る | 「入力と入口」 |
| (追加)cloud session で別の session から再開し、PR の head branch に切り替えられない。note が採番前 | P2 | 採番の push の前で止まり、head branch へ push できる環境での再開を依頼する | 「PR から始める場合」の branch の項、「停止とエスカレーション」 |
| (追加)起点 Issue も note も無い PR、review cycle が無い | P2 | 採番と `progress.md` は対象外。check runs だけを P2 を委譲する直前に確かめる | 表の後の P1 の確認の 2 項 |

### その他

| 項目 | 結果 |
|---|---|
| repo 相対 path の存在(追加行の path) | 切れなし |
| 参照先の節名(「フェーズ」「CI の確認点」「停止とエスカレーション」「PR から始める場合」「入力と入口」「note の探し方」「操作別の第一選択」、0059 の「決定」、bizdate の同 skill の「head SHA と CI」) | すべて存在する |
| 文体(追加行の `(です\|ます)。`) | 該当なし |
| `git diff --check` | 問題なし |
| Go の test | Go のコードを変えないため、ローカルでは実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。frontmatter は変えていない。新しい session を開始して確かめる |

## リスク・ブロッカー

- 並行実行: #230 は本 Issue と同じく、orchestrator の P1 の完了条件と PR の入口に触れる。後から着手する #230 の側で、PR の入口の P1 の確認に引き上げた項目を加える(0059 の追記の「影響」)。#216 / #222 / #228 とは触るファイルが重ならない(並行可否の評価による)。
- 未検証: description による発火。

## セッションログ

- 2026-09-24: 作業内容 1〜4 を実施し、Issue の「検証」を実行した。bizdate main(`201bda4`)の同 skill の「head SHA と CI」を参照した。
