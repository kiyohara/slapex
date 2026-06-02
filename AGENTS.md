# AI Agent ガイド

このファイルは、このリポジトリにおける AI agent 向け共通入口です。

index として利用し、これ自体を唯一の正本として扱わないこと。

## 共通正本

各 rule の本文はこの一覧の正本に置く。Codex は `AGENTS.md` 経由で正本に到達するため、新規 rule を作るときは必ずここに追加する。

- Agent 設定管理ルール(skill / rule の作成・削除・rename・配置): `doc/guidelines/agent-configuration-management.md`
- Decision log 記録ルール(方針決定ログの作成・更新・index 管理): `doc/guidelines/decision-log-guidelines.md`
- Working branch notes 取り扱いルール(性質・整合性スコープ・ライフサイクル・メンテコスト判断): `doc/guidelines/working-branch-notes-handling.md`
- Working branch notes 情報統制ルール(`working-branch-notes/**/*.md` のセキュリティ禁則): `doc/guidelines/working-branch-notes-security.md`

## プロダクト設計ドキュメント

- 利用手順: `doc/product/usage-flow.md`
- 進捗管理表: `doc/product/progress.md`
- 方針決定ログ index: `doc/product/decision-log/index.md`
- 方針決定ログ template: `doc/product/decision-log/_template.md`
- 作業ブランチメモ: `working-branch-notes/README.md`

## Agent 固有の入口

### 共通 index

- AI 向け共通入口は **`AGENTS.md`**(本ファイル)。
- **Claude Code** 用の agent 固有 shim は **`CLAUDE.md`**(`@AGENTS.md` で本ファイルを取り込む)。

### ルール(Cursor / Claude Code)

- **Cursor**: `.cursor/rules/*.{md,mdc}` を frontmatter に従ってロードする。
- **Claude Code**: `.claude/rules/*.md` を `paths:` frontmatter に従ってロードする。
- **Codex app**: `AGENTS.md` から `doc/guidelines/` の正本へ移動して読む。
- **GitHub Copilot Review**: `.github/copilot-instructions.md` をレビュー時に読む。Copilot は `AGENTS.md` やリンク先正本を辿らないため、効かせたい要点は Copilot 用ファイル内に直接書く。

## AI Agent 向けルール

- AI agent 用の設定ファイル、rule、skill、agent 固有入口を作成・削除・rename するときは `doc/guidelines/agent-configuration-management.md` に従う。
- 設計判断、方針変更、重要な検討経緯を記録するときは `doc/guidelines/decision-log-guidelines.md` に従う。
- Decision log を記録するときは、まず `doc/product/decision-log/index.md` を読み、必要に応じて個別ログを作成または更新する。
- `working-branch-notes/**/*.md` を作成・編集・レビューするときは `doc/guidelines/working-branch-notes-handling.md` と `doc/guidelines/working-branch-notes-security.md` の両方に従う。
- 恒久的なプロジェクト方針を agent 固有 shim にだけ書いてはならない。
- AI と人間で別ドキュメントを持たない。共通正本は `doc/guidelines/` または `doc/product/` に置く。
