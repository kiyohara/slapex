---
name: run-issue-task
description: slapex の GitHub Issue 駆動タスクを開始・実行する。Issue 番号を入力として受け取り、Issue 本文の依存確認から、作業ブランチ作成、working branch note 作成、実装、検証、progress.md 更新、PR 作成までを `doc/guidelines/issue-driven-task-execution.md` に準じて進める。Issue 番号が無い場合は `progress.md` の進行中タスク索引から次に進める候補をサジェストする。
---

# run-issue-task

GitHub Issue を 1 件選び、Issue 駆動タスクとして実行するための skill。

この skill は `doc/guidelines/issue-driven-task-execution.md` を正本として扱う。ここに書いた手順と guideline が食い違う場合は、guideline を優先する。

## 入力

- 入力は GitHub Issue 番号、またはこのリポジトリの Issue URL とする。
- Issue 番号が明示された場合は、対象 Issue を読み、依存確認から開始する。
- Issue 番号が明示されていない場合は、`progress.md` の進行中タスク索引から推奨 Issue を探し、ユーザーにサジェストする。
- 複数 Issue を同時に開始しない。1 Issue = 1 ブランチ = 1 PR として扱う。
- 入力が Issue か PR か曖昧、別リポジトリの URL、または複数 Issue の同時実行指示である場合は、作業を始めずユーザーに確認する。

## 参照する正本

作業前に必要な範囲で次を読む。

- `AGENTS.md` — agent 向け入口。
- `doc/guidelines/issue-driven-task-execution.md` — Issue 駆動タスク実行の正本。
- `doc/guidelines/github-mcp-guidelines.md` — GitHub 操作は MCP 優先。
- `doc/guidelines/git-operation-guidelines.md` — commit / tag / GitHub SSH remote 通信を伴う git 操作。
- `doc/guidelines/development-command-guidelines.md` — test / build などの開発コマンドは Docker Compose 優先。
- `doc/guidelines/working-branch-notes-handling.md` — working branch note の作成・rename・扱い。
- `doc/guidelines/working-branch-notes-security.md` — working branch note に書いてはいけない情報。
- `doc/guidelines/pull-request-guidelines.md` — PR title / description / 検証記載。
- `progress.md` — 進行中タスク索引と依存・状態確認。

## Issue 番号がある場合

1. GitHub MCP tool で対象 Issue を読む。
   - Issue 本文、state、labels、comments、sub-issues、関連 PR の有無を確認する。
   - MCP tool が使えない場合のみ、`doc/guidelines/github-mcp-guidelines.md` の fallback ルールに従う。
2. Issue の依存を確認する。
   - 依存が `progress.md` の索引表にあれば、その行の状態と PR 欄で確認する。
   - 索引表に無ければ、依存先 Issue / PR が完了または merge 済みかを GitHub MCP で確認する。
   - 未完了の依存があれば作業を始めず、未完了依存を報告して終了する。
3. `main` を最新化し、Issue 記載の推奨ブランチ名があればそれを使って作業ブランチを作る。
   - 推奨ブランチ名が無い場合は、Issue title から短く意味が分かる kebab-case のブランチ名を決める。
   - git 操作は `doc/guidelines/git-operation-guidelines.md` に従う。
4. `working-branch-notes/draft_<escaped-branch-name>.md` を作る。
   - `working-branch-notes/_template.md` を基本形にする。
   - note には秘密情報・個人情報・顧客固有情報を書かない。
5. Issue の「作業内容」を実施する。
   - 「スコープ外」に挙がる事項へ手を広げない。
   - 開発コマンドは Docker Compose 経由を優先する。
6. Issue の「検証」をすべて実行し、結果を note の検証セクションに記録する。
7. 対象 Issue が `progress.md` の進行中タスク索引にある場合は、該当行を更新する。
   - 状態は PR merge 後に正になる内容として `done` へ進めてよい。
   - PR 作成前は PR 欄を `-` のままにし、PR 採番後に `#<PR 番号>` へ更新する。
   - 単発 Issue が索引に無い場合は、無理に `progress.md` へ登録しない。
8. PR を作る。
   - description は日本語で、`Closes #<Issue 番号>` を含める。
   - merge はしない。
9. PR 採番後、working branch note を `<PR 番号>_<escaped-branch-name>.md` へ rename する。
   - `number-working-branch-note` skill が使える場合は、その skill を使う。
   - rename 後に note 本文や PR description 内の note 参照があれば更新する。
10. PR の URL、検証結果、未解決事項をユーザーへ報告する。

## Issue 番号がない場合

1. `progress.md` の進行中タスク索引を読む。
2. 候補を次の条件で絞る。
   - 状態が `todo` または `doing`。
   - 依存欄にある Issue / PR が完了済み。
   - ブロッカーや review / merge 待ちが無い。
3. 候補の順序は、索引表に載っている順を優先する。
4. 一意に決まる場合は、その Issue 番号と理由をユーザーにサジェストする。
5. 複数候補が同等、依存状態が読み切れない、または候補が無い場合は、勝手に開始せず Issue 番号を指示するよう誘導する。

## 終了条件

- 依存未完了の場合: 未完了依存を報告して終了する。
- 実装を進めた場合: PR 作成、note rename、検証結果記録、`progress.md` 更新まで終えて報告する。
- merge は常にユーザーが行う。

## やらないこと

- 複数 Issue を並行実行しない。
- PR を merge しない。
- `progress.md` に載らない単発 Issue を自動的に横断プランへ登録しない。
- Issue 本文や guideline と食い違う仕様判断を勝手に補完しない。
