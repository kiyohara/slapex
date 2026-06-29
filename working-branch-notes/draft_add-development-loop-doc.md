# 作業ブランチメモ

- ブランチ: add-development-loop-doc
- PR:
- 最終更新: 2026-06-29

## 目的

Issue #90 に従い、slapex の開発ループを説明する入口ドキュメントを追加する。GitHub Issue から始める前提、`progress.md`、各 skill、PR、release、maintain-progress の役割分担を短く辿れる状態にする。

## 現在の状況

- Issue #88 / #89 と PR #92 / #93 が完了済みであることを確認した。
- `doc/guidelines/development-loop.md` を追加した。
- `AGENTS.md` と `doc/README.md` から新規 guideline に到達できるようにした。
- 新規 guideline として扱うため、Cursor / Claude Code の薄い入口 rule も追加した。

## 決定事項

- 詳細手順は既存 guideline / skill を正本とし、入口ドキュメントには全体の流れと参照先だけを置く。
- `progress.md` は詳細経緯ではなく、開発ループ整備プランの状態更新に留める。

## 次にやること

- PR 作成後に note を PR 番号付きへ rename する。

## 検証

- `git diff --check`
- `rg -n "development-loop.md|開発ループ入口|register-progress-issue|run-issue-task|maintain-progress|release skill|progress.md|working-branch-notes|decision log" AGENTS.md doc/README.md doc/guidelines/development-loop.md .cursor/rules/development-loop.mdc .claude/rules/development-loop.md progress.md`
- working branch note の情報統制キーワード確認(実値該当なし)
- 新規ドキュメントを読み直し、`issue-driven-task-execution.md` / `maintain-progress` skill / `release` skill の詳細手順を複製しすぎていないことを確認した。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-06-29: Issue #90 を読み、依存 #88 / #89 が closed、PR #92 / #93 が merged であることを GitHub MCP で確認した。
