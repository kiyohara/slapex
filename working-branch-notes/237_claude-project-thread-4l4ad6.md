# 作業ブランチメモ

- ブランチ: `claude/project-thread-4l4ad6`(cloud session が指定。Issue の推奨ブランチ名は `check-phase-completion-on-pr-entry`)
- PR: #237
- 最終更新: 2026-09-24

## 目的

Issue #234。`drive-issue-to-reviewed-pr` を PR 番号の入口から再開するとき、開始フェーズより前のフェーズの完了条件を確かめる規定を足す。P1 の途中で途切れた場合の note の採番と `progress.md` の反映、P1 と P4 の後の check runs の確認を、入口から飛ばさないようにする。

本 Issue は、#216、#222、#228 と並行して進める。`doc/guidelines/issue-driven-task-execution.md` の「前提」(タスクは直列に消化する。decision log 0037)の例外で、cloud での並列実行を試すユーザーの指示(2026-09-24)による。本 Issue の処理は、PR #233 で入った `drive-issue-to-reviewed-pr` の手順で行う。

## 現在の状況

- 作業内容 1〜4 を実施し、Issue の「検証」を実行した(P1)。
- 変更: `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md` の「入力と入口」(理由の 1 文)、「PR から始める場合」(push できない場合の手順と、P2 / P5 から始める前の前のフェーズの確認)、「CI の確認点」(確認点を P2 と P5 の前へ)。decision log 0059 に追記した。
- review cycle `claude-code-b2f5d14-20260924132259` の指摘 2 件を P4 で採用し、`b4bf5f0` で直した。

## 決定事項

- check runs の確認は、Issue のコメントの案(bizdate と同じく、P2 と P5 の前に確かめる形)を採った。通常の流れでは現行の確認点(P1 の完了時、P4 の push 後)と同じ時点になり、入口ごとに書き分けずに済む。PR の入口に足すのは、前のフェーズの完了条件のうち check runs 以外の確認だけにした。
- 確認点は、委譲する直前だけでなく、subagent が使えない環境で P2 / P5 の前で止まる直前も含める(review の指摘 1)。bizdate は subagent が使えない環境でも同じ agent が P2 / P5 を実行し得るが、slapex は止まる(0059 の F1)。
- P5 から始める場合も P4 の完了条件を確かめる(未決事項 1 の仮決定)。check runs は「CI の確認点」で確かめ、その周の P4 の記録が note に無ければ追記して push する(review の指摘 2)。返信は再開位置の判定で、修正の push は `address-comments` の手順で満たされている。
- 未決事項 2(decision log は 0059 への追記)は仮決定のまま進めた。
- check runs が失敗した場合の扱い(差分に起因すれば直して再 push、説明できなければ止まる)は変えない。bizdate は PR の外に原因がある失敗を brief に明記して委譲するが、本 Issue の範囲外とした。
- PR の head branch へ push できない場合に止まる手順に、P1 や P4 の残りの手順を足した。採番、`progress.md` の反映、note の P4 の記録は push を伴う。
- 「入力と入口」の理由の文を「P2 以降で途切れた場合など」から「PR の作成後に途切れた場合など」に改めた。P1 の途中で途切れた場合も PR の入口へ入るようになったためである。
- 「フェーズ」の表、「head SHA の受け渡し」、「working branch note」、「subagent が使えない環境」の節は変えていない。P4 の完了条件が success を含まない点は、確認点を止まる直前にも置くことで補った。
- `doc/design/decision-log/index.md` の 0059 の行は変えない。行は CI の確認点と PR の入口に触れておらず、今回の追記で古くならない。並行する #228 / #216 / #222 が同じ表に行を足すため、衝突も避けられる。
- `address-comments` などの手順番号は書かない。並行する #216 が `address-comments.md` の手順番号をずらすためである(#216 の thread からも同じ共有を受けた)。
- `progress.md` は索引外の単発 Issue のため更新しない。
- 出力生成系 3 skill は、ドキュメントだけの変更で各 skill の「いつ使うか」に当たらないため適用しない。P4 の修正後も同じ。

## 次にやること

- PR を作成し、note を採番する(P1)。(完了)
- 最新 head の check runs を確かめ、P5 を P2 の subagent に委譲する。

## 検証

2026-09-24、cloud session(project の thread)で実行。

### 再開の状態ごとの開始フェーズと、その前に行う手順

変更後の SKILL.md を読み、各状態で開始フェーズと、その前に行う手順が一意に決まることを確かめた。P4 の修正後の SKILL.md で読み直した。

| 状態 | 開始フェーズ | 開始前に行う手順 | 根拠 |
|---|---|---|---|
| PR は open、note が採番前(`draft_` のまま)、review cycle が無い | P2 | `number-working-branch-note` で採番して push する。関連 Issue が索引にあれば `progress.md` の PR 欄も確かめる。採番後の head の check runs を、P2 の前に確かめる | 「PR から始める場合」の表の 1 行目と、表の後の前のフェーズの確認。「CI の確認点」 |
| 採番済み、索引にある Issue の `progress.md` の PR 欄が未反映、review cycle が無い | P2 | `run-issue-task` の手順どおりに PR 欄を更新して push する。その head の check runs を、P2 の前に確かめる | 同上 |
| 採番済み、最新 head の check runs が未完了または失敗、review cycle が無い | P2 | 未完了なら完了を待つ。失敗なら、差分に起因すれば直して再 push し、完了を待ち直す。差分から説明できなければ止まる | 「CI の確認点」、「停止とエスカレーション」 |
| P1 の完了条件をすべて満たし、review cycle が無い | P2 | 追加の手順は無い(確認だけ)。従来どおり | 同上 |
| 対応が要る指摘のすべてに `address-comments` の返信があり、最新 head の check runs が未完了 | P5 | その周の P4 の記録が note にあることを確かめる。check runs は完了を待ち、success を確かめてから P5 を委譲する。失敗した場合は 3 行目と同じ | 表の 3 行目、表の後の前のフェーズの確認、「CI の確認点」 |
| (追加)対応が要る指摘のすべてに返信があり、その周の P4 の記録が note に無い | P5 | 記録を追記して push し、その head の check runs を P5 の前に確かめる | 同上 |
| (追加)subagent が使えない環境で、P1 または P4 を終えた | P2 / P5 の前で止まる | 止まる直前に check runs を確かめる。失敗なら 3 行目と同じ。success なら再開手順を返す | 「CI の確認点」、「subagent が使えない環境」 |
| (追加)Issue を入力し、それを `Closes` する open PR があり、P1 の途中で途切れている | P2 | PR の入口として 1〜3 行目と同じ手順を通る | 「入力と入口」 |
| (追加)cloud session で別の session から再開し、PR の head branch に切り替えられない。note が採番前 | P2 | 採番の push の前で止まり、head branch へ push できる環境での再開を依頼する | 「PR から始める場合」の branch の項、「停止とエスカレーション」 |
| (追加)起点 Issue も note も無い PR、review cycle が無い | P2 | 採番と `progress.md` は対象外。check runs だけを P2 の前に確かめる | 表の後の前のフェーズの確認 |

### その他

| 項目 | 結果 |
|---|---|
| repo 相対 path の存在(追加行の path) | 切れなし |
| 参照先の節名(「フェーズ」「CI の確認点」「停止とエスカレーション」「PR から始める場合」「入力と入口」「working branch note」「note の探し方」「操作別の第一選択」、0059 の「決定」、bizdate の同 skill の「head SHA と CI」) | すべて存在する |
| 文体(追加行の `(です\|ます)。`) | 該当なし |
| `git diff --check` | 問題なし |
| Go の test | Go のコードを変えないため、ローカルでは実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。frontmatter は変えていない。新しい session を開始して確かめる |
| P2 の subagent が報告した実行環境 | brief で渡した上位の指示(model の識別子はチャットの返答だけに書く)が PR と review のコメントを対象外とすると確認できず、`Model` は `unknown` と理由の 1 行になった。server が付けた footer と 5 行の間に空行が 1 行あることを read-back で確かめた |

## リスク・ブロッカー

- 並行実行: #230 は本 Issue と同じく、orchestrator の P1 の完了条件と PR の入口に触れる。後から着手する #230 の側で、PR の入口の P1 の確認に引き上げた項目を加える(0059 の追記の「影響」)。#216 / #222 / #228 とは触るファイルが重ならない(並行可否の評価と、P2 の subagent の確認による)。
- 未検証: description による発火。
- P2 の subagent は、並行する PR の変更ファイル一覧を `gh api`(REST の read)で取得した。組み込みの `pull_request_read` で足りたため、`doc/guidelines/github-mcp-guidelines.md` の「cloud session(Claude Code on the web)」の規定から外れる。read だけで、投稿への影響は無い。

## セッションログ

- 2026-09-24: 作業内容 1〜4 を実施し、Issue の「検証」を実行した。bizdate main(`201bda4`)の同 skill の「head SHA と CI」を参照した。
- 2026-09-24: PR #237 を作成し、note を採番した(P1)。head `b2f5d14` の check runs 5 件が success。出力生成系 3 skill は不適用。
- 2026-09-24: P2 の subagent が review した。review cycle `claude-code-b2f5d14-20260924132259`、`Reviewed head` `b2f5d14`、指摘 2 件(inline 2、top-level 0)。P3 で P4 へ進んだ。
- 2026-09-24: P4 で 2 件とも採用し、`b4bf5f0` で直した(確認点に止まる直前を含める、P5 から始める場合の P4 の記録の確認)。出力生成系 3 skill はドキュメントだけの変更のまま不適用。
