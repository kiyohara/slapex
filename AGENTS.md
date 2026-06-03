# AI Agent ガイド

このファイルは、このリポジトリにおける AI agent 向け共通入口です。

index として利用し、これ自体を唯一の正本として扱わないこと。

## 共通正本

各 rule の本文はこの一覧の正本に置く。Codex は `AGENTS.md` 経由で正本に到達するため、新規 rule を作るときは必ずここに追加する。

- Agent 設定管理ルール(skill / rule / MCP 共通資材の作成・削除・rename・配置): `doc/guidelines/agent-configuration-management.md`
- Git 操作ルール(1Password SSH agent / 署名(commit / tag) / GitHub SSH 通信): `doc/guidelines/git-operation-guidelines.md`
- GitHub MCP 利用ルール(MCP 優先 / `gh` fallback / tool allowlist): `doc/guidelines/github-mcp-guidelines.md`
- GitHub CLI 実行ルール(1Password op plugin 連携 / 実行環境制約 / MCP 不可時の fallback): `doc/guidelines/github-cli-guidelines.md`
- 開発コマンド実行ルール(Docker Compose 優先 / host OS 上での開発環境の直接構築・実行を抑止): `doc/guidelines/development-command-guidelines.md`
- Pull Request 作成ガイドライン: `doc/guidelines/pull-request-guidelines.md`
- Decision log 記録ルール(方針決定ログの作成・更新・index 管理): `doc/guidelines/decision-log-guidelines.md`
- Working branch notes 取り扱いルール(性質・整合性スコープ・ライフサイクル・メンテコスト判断): `doc/guidelines/working-branch-notes-handling.md`
- Working branch notes 情報統制ルール(`working-branch-notes/**/*.md` のセキュリティ禁則): `doc/guidelines/working-branch-notes-security.md`

## ドキュメント配置

- ドキュメント配置の入口: `doc/README.md`
- 設計文書: `doc/design/README.md`
- 利用者向け help: `doc/help/README.md`
- 進捗管理表: `progress.md`
- 方針決定ログ index: `doc/design/decision-log/index.md`
- 方針決定ログ template: `doc/design/decision-log/_template.md`

## 作業プロセスドキュメント

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
  - `.github/instructions/*.instructions.md` を追加した場合、各 instruction file は先頭から約 4,000 文字のみ反映される前提で要点を絞る。

## AI Agent 向けルール

- AI agent 用の設定ファイル、rule、skill、agent 固有入口を作成・削除・rename するときは `doc/guidelines/agent-configuration-management.md` に従う。
- commit 作成、署名付き tag 作成、GitHub の SSH remote を使う push / fetch / pull など、署名または GitHub との SSH remote 通信を伴う `git` 操作の前に `doc/guidelines/git-operation-guidelines.md` に従う。
- GitHub の PR / issue / レビューコメントなどを操作するときは `doc/guidelines/github-mcp-guidelines.md` に従い、MCP を優先する。
- `gh` コマンドを実行するときは `doc/guidelines/github-cli-guidelines.md` に従う。GitHub MCP 利用ルールでも `gh` fallback の経路はこの正本を参照する。
- 依存 install / アプリ起動 / test / build など、開発環境を host OS 上に構築・実行する類のコマンドを実行するときは `doc/guidelines/development-command-guidelines.md` に従い、Docker Compose 経由を優先する。
- PR を作成または更新するときは `doc/guidelines/pull-request-guidelines.md` に従う。
- ドキュメントを作成・移動・分類変更するときは、まず `doc/README.md` と該当ディレクトリの `README.md` を確認する。
- 設計判断、方針変更、重要な検討経緯を記録するときは `doc/guidelines/decision-log-guidelines.md` に従う。
- Decision log を記録するときは、まず `doc/design/decision-log/index.md` を読み、必要に応じて個別ログを作成または更新する。
- `working-branch-notes/**/*.md` を作成・編集・レビューするときは `doc/guidelines/working-branch-notes-handling.md` と `doc/guidelines/working-branch-notes-security.md` の両方に従う。
- 恒久的なプロジェクト方針を agent 固有 shim にだけ書いてはならない。
- AI と人間で別ドキュメントを持たない。配置判断は `doc/README.md` と各ディレクトリの `README.md` に従う。
