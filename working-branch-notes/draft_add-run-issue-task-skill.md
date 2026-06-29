# 作業ブランチメモ

- ブランチ: `add-run-issue-task-skill`
- PR: 未作成
- 最終更新: 2026-06-29

## 目的

Issue #89 に従い、Issue 番号だけで issue-driven task の実行を開始できる agent skill `run-issue-task` を追加する。

## 現在の状況

- Issue #89 を GitHub MCP で取得済み。
- 依存 Issue #88 は closed / completed、`main` には PR #92 merge 済み。
- 作業ブランチ `add-run-issue-task-skill` を作成済み。

## 決定事項

- `run-issue-task` は `.agents/skills/run-issue-task/SKILL.md` を正本にする。
- Claude Code 用入口は `.claude/skills/run-issue-task` から正本への symlink にする。
- Issue 駆動タスク実行の正本は `doc/guidelines/issue-driven-task-execution.md` とし、skill 側は開始条件と参照先を明確にする。

## 次にやること

- PR 作成後に note を PR 番号付きファイル名へ rename する。
- `progress.md` の #89 行へ PR 番号を反映する。

## 検証

- `test -f .claude/skills/run-issue-task/SKILL.md` — pass
- `find -L .claude/skills -maxdepth 1 -type l -print` — pass (broken symlink なし)
- `rg -n "Issue 番号がある場合|Issue 番号がない場合|1 Issue = 1 ブランチ = 1 PR|merge はしない|未完了の依存|doc/guidelines/issue-driven-task-execution.md" .claude/skills/run-issue-task/SKILL.md` — pass

## リスク・ブロッカー

- 現時点で既知のブロッカーなし。

## セッションログ

- 2026-06-29: Issue #89 と依存 Issue #88 を GitHub MCP で確認。`main` を fetch し、`origin/main` と一致していることを確認してから作業ブランチを作成。
- 2026-06-29: `run-issue-task` skill と Claude Code 用 symlink を追加し、Issue 指定の symlink / broken symlink / skill 本文観点を検証。
