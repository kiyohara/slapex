# 作業ブランチメモ

- ブランチ: `add-release-skill`
- PR: 未作成
- 最終更新: 2026-06-26

## 目的

slapex のリリース作業を標準化する agent skill `release` を追加する。バージョン未指定時は直前 tag からの差分を提示して推奨版を確認し、tag 前に必要な doc 調整(README の version 参照・`progress.md`・decision log)を 1 本のリリース PR に集約、merge 後に署名付き tag を作成・push して GoReleaser を起動し、公開物を検証する一連の手順を skill 化する。

## 現在の状況

- `.agents/skills/release/SKILL.md` を作成済み(正本)。
- `.claude/skills/release` を正本への symlink として作成済み。Cursor / Codex は `.agents/skills/` を直読するため symlink は作らない。
- skill は既存 `number-working-branch-note` の構成(SKILL.md 単体・日本語)を踏襲。
- symlink 経由で `SKILL.md` が読めること、Claude Code の skill 一覧に `release` が出ることを確認済み。

## 決定事項

- リリースフローは **専用ブランチ + 1 PR** 方式。doc 更新を 1 PR に集約し、ユーザー merge 後に `main` HEAD へ署名付き tag を打つ。
- バージョン未指定時は差分を提示し、既定 patch bump を控えめに推奨。最終判断はユーザー(本リポジトリは Conventional Commits 非使用のため機械確定しない)。
- README の install 例の version 参照はリリースごとに最新へ bump する。
- GitHub 操作の MCP 優先ポリシーは常時ロードの rule にあるため skill では再掲しない。skill 固有の運用差分のみ記載: (1) ツール不可視時は discovery で探してから `gh` fallback、(2) Release / workflow / run 系は MCP 非対応のため `gh`(`op plugin run -- gh`)。
- 配置は `doc/guidelines/agent-configuration-management.md` の checklist に従う。

## 次にやること

1. skill 追加分を commit。
2. push(ユーザー承認後)。
3. PR を作成し、`number-working-branch-note` skill で本 note を採番。
4. PR の merge はユーザーが行う。

## 検証

- 2026-06-26: 正本 `.agents/skills/release/SKILL.md` と symlink `.claude/skills/release` の両方から `SKILL.md` が読めることを確認。
- 2026-06-26: Claude Code の skill 一覧に `release` が表示されることを確認。

## リスク・ブロッカー

- なし。skill 本体の追加のみで、実コードや配布設定には変更を加えない。

## セッションログ

- 2026-06-26: リリース skill の追加方針を検討。配置・リリースフロー・バージョン推奨ロジック・README bump・GitHub 操作の記載粒度をユーザーと確定し、`release` skill を作成した。
