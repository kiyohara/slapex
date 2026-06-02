# 作業ブランチメモ

- ブランチ: setup_discussion_base
- PR: #1
- 最終更新: 2026-06-02

## 目的

Slack の書き込みを閲覧しやすい HTML 形式で保存するツールについて、実装前の設計作業を進めるためのドキュメント基盤と AI agent 共同作業の入口を整備する。

## 現在の状況

- ルート直下の既存 `.git` はユーザーが `.git-backup` に退避済み。
- 新しい Git リポジトリを `main` で初期化した。
- `.gitignore` のみを initial commit として作成済み。
- 作業ブランチ `setup_discussion_base` を作成済み。
- ここまでに作成していた設計ドキュメント、AI agent 向け入口、Copilot Review 設定、working branch notes 関連ファイルを本ブランチの作業成果として整理する。

## 決定事項

- プロダクト設計ドキュメントは `doc/product/` 配下に置く。
- 利用手順は `doc/product/usage-flow.md` に置く。
- 進捗管理表は `doc/product/progress.md` に置く。
- 方針決定ログは `doc/product/decision-log/` 配下に置き、`index.md` と個別ログに分ける。
- AI agent 向けの長い正本は `doc/guidelines/` に置く。
- `AGENTS.md` は Codex app を含む AI agent 共通入口とする。
- `CLAUDE.md` は `@AGENTS.md` を取り込む Claude Code 用 shim とする。
- Cursor / Claude Code にはそれぞれ `.cursor/rules/` と `.claude/rules/` の薄い入口を用意する。
- GitHub Copilot Review には `.github/copilot-instructions.md` を用意する。ただし実装技術が未確定のため、path 別 instructions はまだ作成しない。
- working branch notes はレビュー専用ではなく、ブランチ単位の汎用作業メモとして取り込む。
- Git / GitHub / PR / MCP / 開発コマンドの運用ルールは、参加メンバーが同じ関連リポジトリと揃えるため、本リポジトリにも取り込む。
- `number-working-branch-note` skill を取り込み、`.agents/skills/` と `.claude/skills/` の最低限の skill 管理構成を作る。
- 既存の試作プロジェクトは参考資料として扱うが、本リポジトリでの方針は個別に議論して決める。

## 次にやること

- Claude Code のレビューサブノートで指摘された残項目を反映する。
- 修正後に staged file を確認し、本ブランチの追加作業として commit する。

## 検証

- `git init -b main` を実行し、新しいリポジトリを作成した。
- `.gitignore` の initial commit を作成した。
- `git switch -c setup_discussion_base` で作業ブランチを作成した。
- `working-branch-notes/1_setup_discussion_base__agent-review.md` に Claude Code のレビュー結果が保存されたことを確認した。

## リスク・ブロッカー

- `.git-backup/` は作業ツリー内に残っているが、`.gitignore` で除外している。
- 現時点では実装技術が未確定のため、Copilot Review の path 別 instructions は未作成。
- 設計ドキュメントはひな形段階であり、内容は今後の検討で埋める。
- GitHub MCP 共通資材は secret を含まないテンプレートと wrapper のみを取り込む。実 env file は `.gitignore` で除外する。

## セッションログ

### 2026-06-02

- ユーザーから、利用手順、進捗管理表、方針決定ログのファイル名と配置場所について相談を受けた。
- 類似の別プロジェクトや、[kiyohara/slack_posts_dumper](https://github.com/kiyohara/slack_posts_dumper) を確認し、`doc/product/` 配下に設計ドキュメントを置く案を採用した。
- `doc/product/usage-flow.md`、`doc/product/progress.md`、`doc/product/decision-log/index.md`、`doc/product/decision-log/_template.md` を作成した。
- decision log は単一ファイルではなく、`index.md` と個別ログファイルに分割する方針にした。
- AI agent が decision log を記録できるよう、`doc/guidelines/decision-log-guidelines.md` と agent 向け入口を作成した。
- `AGENTS.md`、`CLAUDE.md`、`.cursor/rules/`、`.claude/rules/` を整備した。
- AI agent 用ファイルの管理ルールとして `doc/guidelines/agent-configuration-management.md` を作成した。
- 将来の GitHub Copilot Review 利用を見据えて、`.github/copilot-instructions.md` を作成した。
- working branch notes を汎用作業メモとして取り込み、`working-branch-notes/README.md`、`working-branch-notes/_template.md`、`doc/guidelines/working-branch-notes-handling.md`、`doc/guidelines/working-branch-notes-security.md` と agent 入口を作成した。
- ユーザーが既存 `.git` を `.git-backup` に退避した後、新しい Git 履歴を作成した。
- Claude Code のレビュー結果を `working-branch-notes/1_setup_discussion_base__agent-review.md` で確認した。
- Cursor の working branch note rule に `globs` を追加し、decision log rule にも `doc/product/decision-log/**/*.md` の `globs` を追加した。
- `number-working-branch-note` skill を `.agents/skills/number-working-branch-note/` に取り込み、`.claude/skills/number-working-branch-note` から symlink した。
- `doc/guidelines/agent-configuration-management.md` を関連リポジトリの詳細版に寄せ、skill / MCP 共通資材の管理方法を含めた。
- `doc/guidelines/git-operation-guidelines.md`、`github-cli-guidelines.md`、`github-mcp-guidelines.md`、`pull-request-guidelines.md`、`development-command-guidelines.md` と各 tool 入口を取り込んだ。
- GitHub MCP 共通資材を `.agents/mcp/github-op-integrated/` に取り込んだ。`github.env` 実体はコピーせず、`.gitignore` に除外を追加した。
- 試作プロジェクトとの関係を `doc/product/decision-log/0001-relationship-to-prototype.md` に記録した。
- `doc/product/progress.md` に progress と working branch note の違いを薄く記載し、初期進捗を seed した。
