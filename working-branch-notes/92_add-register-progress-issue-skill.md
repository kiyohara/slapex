# 作業ブランチメモ

- ブランチ: `add-register-progress-issue-skill`
- PR: #92
- 最終更新: 2026-06-29

## 目的

Issue #88 に従い、既存 GitHub Issue を `progress.md` の進行中タスク索引へ登録する手順を標準化する agent skill `register-progress-issue` を追加する。

## 現在の状況

- 依存 PR #87 が merge 済みであることを GitHub MCP で確認済み。
- `progress.md` は開発ループ整備プラン(#88〜#90)を追跡中で、#88 は `dev-loop-01` として登録済み。
- `.agents/skills/register-progress-issue/SKILL.md` を追加する。
- `.claude/skills/register-progress-issue` を正本への symlink として追加する。
- PR #92 を作成済み。note を PR 番号付きファイル名へ rename し、`progress.md` の #88 行を done / PR #92 付きへ更新する。

## 決定事項

- skill は単一 `SKILL.md` とし、追加の reference / script は作らない。Issue 登録は手順判断が中心で、決定的なスクリプト化よりも MCP での確認と最小編集の明文化が適しているため。
- `progress.md` には恒久ルールを複製しない。必要な場合でも薄いポインタに留める。
- Cursor / Codex 用 symlink は作らない。両者は `.agents/skills/` を直接 discover するため。

## 次にやること

1. note 採番と `progress.md` 更新を commit / push し、PR #92 に反映する。
2. PR #92 の review / merge を待つ。

## 検証

- 2026-06-29: `test -f .agents/skills/register-progress-issue/SKILL.md` で正本 SKILL.md が読めることを確認。
- 2026-06-29: `test -f .claude/skills/register-progress-issue/SKILL.md` で symlink 経由でも SKILL.md が読めることを確認。
- 2026-06-29: `find -L .claude/skills -maxdepth 1 -type l -print` で broken symlink が無いことを確認。
- 2026-06-29: Ruby 標準 YAML parser で SKILL.md frontmatter の `name` / `description` を確認。
- 2026-06-29: `skill-creator` 付属 `quick_validate.py` は host Python に `yaml` が無く、bundled Python も `libpython3.12.dylib` 読み込みエラーで起動できなかったため未実行。代替として上記 YAML parse と必須 field チェックを実施。
- 2026-06-29: `progress.md` の開発ループ整備プランを確認し、リリース履歴・完了済みフェーズの役割を壊していないことを確認。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-06-29: Issue #88 を GitHub MCP で取得。依存 PR #87 が merge 済みであることを確認し、推奨ブランチ `add-register-progress-issue-skill` を作成。
- 2026-06-29: `register-progress-issue` skill 正本、Claude 用 symlink、draft note を追加。symlink と frontmatter を検証。
- 2026-06-29: PR #92 を作成。note を `working-branch-notes/92_add-register-progress-issue-skill.md` へ rename し、`progress.md` の #88 行を done / PR #92 付きへ更新。
