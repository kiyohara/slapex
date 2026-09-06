# 作業ブランチメモ

- ブランチ: `add-pr-handling-to-register-progress-issue`
- PR: 未作成
- 最終更新: 2026-09-06

## 目的

Issue #217 の対応。`register-progress-issue` skill に、`progress.md` 編集後の commit / PR の扱いを追記する。

skill は step 5(`progress.md` の編集)までしか定めておらず、`main` へ直接 push しない運用である以上必ず PR が要るのに、独立 PR にするのか・起点 Issue を作るのか・`Closes` を付けるのかが読み取れなかった。

類似プロジェクト bizdate の kiyohara/bizdate#5 / kiyohara/bizdate#6 で同じ抜けが解消され、その追記内容が slapex の PR #212 の実運用を参照して書かれていたため、slapex 側へ戻して明文化する。

## 現在の状況

skill 本体と `doc/guidelines/development-loop.md` の編集を終え、Issue #217 の検証 5 項目をすべて実施した。PR 未作成。

## 決定事項

- 追記内容は bizdate PR kiyohara/bizdate#6 を基にするが、機械的な移植はしない。slapex の実態に合わせて次を変える。
  - **「参照する正本」への追加は 2 本**(`git-operation-guidelines.md` / `pull-request-guidelines.md`)。bizdate は 4 本追加したが、slapex 版には working branch notes の 2 本が既にある。
  - **`development-loop.md` の例外記述を「索引登録のみ」にしない。** slapex では索引登録(PR #212 / #167)に加え、進捗整理(`maintain-progress`、PR #178)とリリース(`release` skill)も起点 Issue を持たない。bizdate の「恒常的な例外はこの 1 件に限り」をそのまま持ち込むと既存運用と矛盾する。
- 手順の末尾に step を足すのではなく、**step 6 として PR 化を挿入し、既存の step 6・7 を 7・8 へ繰り下げる**。「終了時に確認する」は PR 作成後に行うのが自然なため。
- 既存 step 7(旧 6、Issue へ確認コメント)を `progress.md` 編集より前へ動かす判断もあり得るが、本 Issue のスコープは commit / PR の扱いの追記であり、既存手順の並べ替えは含まない。

## 次にやること

- PR を作成し、note を採番する。
- ユーザーの review と merge 判断を待つ。

## 検証

Issue #217 の検証 5 項目をすべて実施した。

| 項目 | 結果 |
|---|---|
| `pull-request-guidelines.md` との整合 | 矛盾なし。slapex の同正本には merge の記述が無く、merge ルールの正本は `development-loop.md` の基本方針と `issue-driven-task-execution.md` の前提である。skill の step 6 と「やらないこと」はこの 2 本と同趣旨を書いている |
| `number-working-branch-note` の commit / push step との表現整合 | 一致。両 skill とも「参照する正本」に `git-operation-guidelines.md` と `pull-request-guidelines.md` を挙げ、「変更が無ければ commit を作らず終了する」ガードを持つ |
| 既存 step 番号と「やらないこと」「終了報告」の整合 | step は 1〜8 で連続。既存の 6・7 を 7・8 へ繰り下げた。「やらないこと」に 2 項目、「終了報告」に PR URL を追加し、既存記述との矛盾は無い |
| 参照する正本の存在 | 9 件すべて存在(`AGENTS.md` / `github-mcp-guidelines.md` / `development-command-guidelines.md` / `git-operation-guidelines.md` / `pull-request-guidelines.md` / `working-branch-notes-handling.md` / `working-branch-notes-security.md` / `issue-driven-task-execution.md` / `progress.md`) |
| `development-loop.md` の追記と既存運用の整合 | 矛盾なし。skill の「やらないこと」を「索引登録による `progress.md` の変更」に限定したため、`issue-driven-task-execution.md` step 7(索引表の行更新)を抑止しない。起点 Issue を持たない運用作業を 3 種としたのは実態と一致する(PR #178 note に進捗整理の扱いが明記、リリース PR #179 note に `Closes` の記載なし) |

`progress.md` は更新していない。Issue #217 は索引に載らない単発 Issue であり、`doc/guidelines/issue-driven-task-execution.md` step 7 の「表に載らない単発 Issue は `progress.md` を更新しなくてよい」に従った。

テストは実行していない。ドキュメントのみの変更で、実行可能なコードを含まないため。

`doc/guidelines/working-branch-notes-security.md` の情報統制チェックを通した。秘密情報、個人情報、ローカル絶対 path の混入は無い。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-06: Issue #217 に着手。bizdate の Issue / PR を確認し、slapex との差分(参照する正本の既存分、起点 Issue を持たない運用作業が 3 種ある点)を洗い出した。main から分岐し、本 note を作成した。
- 2026-09-06: skill に step 6(PR 化)を挿入し、既存 6・7 を繰り下げ。「参照する正本」に 2 本、「やらないこと」に 2 項目、「終了報告」に PR URL を追加した。`development-loop.md` の基本方針・標準フロー・資材表・参照順の 4 箇所へ、起点 Issue を作らない運用作業の扱いを反映した。検証 5 項目を実施し、すべて通過した。
