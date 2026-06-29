---
name: register-progress-issue
description: slapex の既存 GitHub Issue を `progress.md` の進行中タスク索引へ登録する。Issue 番号を入力として受け取り、GitHub MCP 優先で対象 Issue の本文・状態・ラベル・関連 PR を確認し、既に登録済みの Issue との依存関係・順序・ブロッカー・並行可否を整理したうえで、`progress.md` には詳細経緯を複製せず状態・依存・Issue / PR 参照が分かる最小の行だけを追加または更新する。
---

# register-progress-issue

既存 GitHub Issue を `progress.md` の進行中タスク索引へ登録するための skill。

`progress.md` はリリース台帳と進行中タスクの索引を兼ねる status board であり、詳細な検討経緯やブランチ単位の作業メモを置く場所ではない。Issue 本文・PR・decision log・working branch note を正本として扱い、`progress.md` には現在の追跡に必要な最小情報だけを置く。

## 入力

- 入力は GitHub Issue 番号、またはこのリポジトリの Issue URL とする。
- 複数 Issue を登録する場合も、各 Issue の依存と順序が判断できるよう 1 件ずつ確認する。
- Issue 番号が曖昧、別リポジトリの URL、または Issue と PR の区別がつかない入力の場合は、作業を始めずユーザーに確認する。

## 参照する正本

作業前に必要な範囲で次を読む。

- `AGENTS.md` — agent 向け入口。
- `doc/guidelines/github-mcp-guidelines.md` — GitHub 操作は MCP 優先。
- `doc/guidelines/development-command-guidelines.md` — 開発コマンド実行方針。
- `doc/guidelines/working-branch-notes-handling.md` / `doc/guidelines/working-branch-notes-security.md` — 作業 note を作る場合の扱い。
- `doc/guidelines/issue-driven-task-execution.md` — 登録後に issue-driven task として実行する場合の依存確認と状態更新。
- `progress.md` — 既存の進行中タスク索引とリリース台帳。

## 手順

1. GitHub MCP tool で対象 Issue を読む。
   - Issue 本文、state、labels、assignee、milestone、comments、関連 PR の有無を確認する。
   - MCP tool が使えない場合のみ、`doc/guidelines/github-mcp-guidelines.md` の fallback ルールに従う。
2. Issue が登録対象か確認する。
   - open の作業 Issue を基本対象にする。
   - closed / duplicate / won't fix / 既に完了済みの Issue は、登録せず理由を報告する。
   - Issue 本文に「スコープ外」や依存条件がある場合は、`progress.md` に複製せず依存欄や次にやることへ短く反映する。
3. `progress.md` の既存索引を読む。
   - 既に同じ Issue が登録済みなら、新規行を作らず状態・依存・PR 参照だけ必要最小限で更新する。
   - 関連する横断プランの表がある場合はその表へ追加する。無い場合は、横断的に追跡したい Issue 群かを確認してから新しい小さな表を作る。
   - 単発 Issue を無理に登録しない。索引は「横断的に見渡したい単位」に絞る。
4. 登録済み Issue との関係を整理する。
   - 依存: 先に完了していないと着手できない Issue / PR。
   - 順序: 依存ではないが、レビューや運用上先に進めるとよい Issue。
   - ブロッカー: 外部待ち、設計判断待ち、未 merge PR など。
   - 並行可否: 同時に進めても競合しにくいか、同じファイルや同じ方針に触れるため直列がよいか。
5. `progress.md` を最小限に編集する。
   - 行には ID、Issue、状態、依存、次にやること、PR 参照など、既存表の列に合わせた情報だけを書く。
   - Issue 本文の詳細、検討経緯、長い背景説明、作業ログを複製しない。
   - 状態は `todo` / `doing` / `blocked` / `done` など既存表に合わせる。
   - PR が無い場合は `-`、関連 PR がある場合は `#<番号>` とする。
6. 必要なら Issue へ確認コメントを残す。
   - 依存やスコープが曖昧で登録判断ができない場合は、勝手に解釈せずユーザーに報告する。
   - Issue コメントを残す場合も MCP 優先で、write fallback の二重投稿防止ルールに従う。
7. 終了時に確認する。
   - `progress.md` を冒頭から読んで、リリース履歴と完了済みフェーズの役割が壊れていない。
   - 登録行から Issue / PR へ辿れる。
   - 依存欄が既存登録 Issue との順序・ブロッカーを表している。
   - 詳細経緯が `progress.md` に複製されていない。

## やらないこと

- GitHub Issue の自動選定や優先順位付けをこの skill に持たせない。
- `progress.md` を詳細な作業メモや仕様書にしない。
- issue-driven task の実行そのものは行わない。登録後に実行する場合は `doc/guidelines/issue-driven-task-execution.md` に従う。
- closed Issue を過去履歴として大量に登録しない。

## 終了報告

ユーザーには次を短く報告する。

- 登録または更新した Issue 番号と `progress.md` 上の ID。
- 依存・順序・ブロッカー・並行可否の判断。
- 変更した `progress.md` のセクション。
- 登録しなかった Issue があれば、その理由。
