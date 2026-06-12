# Issue 駆動タスク実行ルール

この文書は、GitHub Issue として登録されたタスクを AI agent が消化するときの共通正本である。直近の主対象は v1.0 リリース実装プラン(`progress.md` のタスク表)だが、同方式で運用するタスク全般に適用する。

## 前提

- 1 Issue = 1 ブランチ = 1 PR とする。Issue 本文がそのタスクの指示書であり、本ルールは共通の進め方だけを定める。
- タスクは直列に消化する。複数 Issue の並行作業はしない。
- **PR の merge は agent が行わない**。レビューと merge 判断はユーザーが行う。

## 進め方

1. 対象 Issue を読む。GitHub 操作は `doc/guidelines/github-mcp-guidelines.md` に従い MCP を優先する。
2. `progress.md` のタスク表で、Issue の「依存」に挙がるタスクがすべて done であることを確認する。未完了の依存があれば作業を始めず、その旨をユーザーに報告して終了する。
3. `main` を最新化し、Issue 記載のブランチ名で作業ブランチを作る(`doc/guidelines/git-operation-guidelines.md`)。
4. `working-branch-notes/draft_<escaped-branch-name>.md` を作る(`doc/guidelines/working-branch-notes-handling.md` と `doc/guidelines/working-branch-notes-security.md` に従う)。
5. Issue の「作業内容」を実施する。「スコープ外」の事項には手を付けない。開発コマンドは `doc/guidelines/development-command-guidelines.md` に従い Docker Compose 経由で実行する。
6. Issue の「検証」をすべて実行し、結果を note の検証セクションに記録する。
7. `progress.md` タスク表の該当行を更新する(状態を done にし、PR 列に番号を記入する。この PR が merge された時点で正になる内容でよい)。
8. PR を作る(`doc/guidelines/pull-request-guidelines.md`)。description に `Closes #<Issue 番号>` を含める。
9. PR 採番後、note を `<PR 番号>_<escaped-branch-name>.md` へ rename する(`number-working-branch-note` skill が使える場合は skill で行う)。
10. PR の URL、検証結果の要約、未解決事項をユーザーに報告して終了する。merge はしない。

## 判断に迷ったとき

- Issue の指示と `doc/design/` の仕様が食い違う場合は、実装で勝手に解釈せず、作業を止めて Issue にコメントを残し、ユーザーに報告する。
- テスト作成中に対象コードのバグを見つけた場合: 仕様文書から正しい挙動が一意に読み取れるなら同じ PR で修正し、note と PR description に明記する。仕様の解釈・変更が必要なら止めて報告する。
- スコープ外のバグ・改善点を見つけた場合は、修正せず Issue コメントまたは終了報告に記載する(変更行に隣接する typo 修正程度は同 PR でよい)。
- 方針決定が必要になった場合は `doc/guidelines/decision-log-guidelines.md` に従う。Issue に decision log の作成・追記が含まれる場合は、その指示を優先する。

## kickoff prompt

ユーザーが各タスクを開始するときの prompt 例:

```text
GitHub Issue #<番号> のタスクを、doc/guidelines/issue-driven-task-execution.md のルールに従って実施してください。
```
