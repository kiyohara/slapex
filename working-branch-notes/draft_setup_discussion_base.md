# 作業ブランチメモ

- ブランチ: setup_discussion_base
- PR:
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

## 次にやること

- `.gitignore` 以外の作業ファイルを add する。
- 本ブランチの作業成果として commit する。
- 必要に応じて、以後の設計検討では working branch note と decision log の使い分けを運用しながら調整する。

## 検証

- `git init -b main` を実行し、新しいリポジトリを作成した。
- `.gitignore` の initial commit を作成した。
- `git switch -c setup_discussion_base` で作業ブランチを作成した。

## リスク・ブロッカー

- `.git-backup/` は作業ツリー内に残っているが、`.gitignore` で除外している。
- 現時点では実装技術が未確定のため、Copilot Review の path 別 instructions は未作成。
- 設計ドキュメントはひな形段階であり、内容は今後の検討で埋める。

## セッションログ

### 2026-06-02

- ユーザーから、利用手順、進捗管理表、方針決定ログのファイル名と配置場所について相談を受けた。
- `../seeos`、`../asahimaru`、`../slack_posts_dumper` を確認し、`doc/product/` 配下に設計ドキュメントを置く案を採用した。
- `doc/product/usage-flow.md`、`doc/product/progress.md`、`doc/product/decision-log/index.md`、`doc/product/decision-log/_template.md` を作成した。
- decision log は単一ファイルではなく、`index.md` と個別ログファイルに分割する方針にした。
- AI agent が decision log を記録できるよう、`doc/guidelines/decision-log-guidelines.md` と agent 向け入口を作成した。
- `AGENTS.md`、`CLAUDE.md`、`.cursor/rules/`、`.claude/rules/` を整備した。
- AI agent 用ファイルの管理ルールとして `doc/guidelines/agent-configuration-management.md` を作成した。
- 将来の GitHub Copilot Review 利用を見据えて、`.github/copilot-instructions.md` を作成した。
- working branch notes を汎用作業メモとして取り込み、`working-branch-notes/README.md`、`working-branch-notes/_template.md`、`doc/guidelines/working-branch-notes-handling.md`、`doc/guidelines/working-branch-notes-security.md` と agent 入口を作成した。
- ユーザーが既存 `.git` を `.git-backup` に退避した後、新しい Git 履歴を作成した。
