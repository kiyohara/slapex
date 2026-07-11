# 作業ブランチメモ

- ブランチ: `prioritize-github-op-integrated-mcp`
- PR: #163
- 最終更新: 2026-07-11

## 目的

Issue #161 に従い、GitHub 操作で `github-op-integrated` MCP を優先するための切断時手順、汎用 skill / plugin との競合時ルール、操作別 tool 対応表を正本へ追加する。

## 現在の状況

- Issue 本文、コメント、関連 Issue #162、関連 PR の有無を `github-op-integrated` MCP で確認済み。
- 依存はなく、関連 PR もないため実装を開始した。
- Cursor の追加調査では、Issue の A〜D で必要範囲を満たせることを確認した。
- 正本へ切断時手順、競合時ルール、操作別 tool 対応表を追加し、静的検証を完了した。
- PR #163 の review 指摘を検証し、再接続入口の surface 別整理と PR review 作成行の追加を反映した。
- Cursor CLI の現行公式文書を再確認し、interactive command と CLI subcommand の区別を修正した。

## 決定事項

- 恒久ルールと操作別対応表は `doc/guidelines/github-mcp-guidelines.md` に集約する。
- `AGENTS.md`、Claude Code / Cursor の入口 shim は正本参照のみを維持し、恒久ルールを複製しない。
- `progress.md` に既存行がない単発 Issue のため、同ファイルは更新しない。

## 次にやること

- Cursor CLI の追加修正を commit / push し、PR への反映を確認する。

## 検証

- `github-op-integrated` の tool discovery と `.config/github-op-integrated.conf.example` の allowlist を照合し、対応表の tool がすべて存在することを確認した。
- `pull_request_read` の各 read method と `pull_request_review_write(resolve_thread)` が現行 tool discovery に存在することを確認した。
- `AGENTS.md`、Claude Code / Cursor の入口 shim が正本を参照し、操作表を複製していないことを確認した。
- GitHub MCP で Issue #162 の依存欄が Issue #161 を参照することを確認した。
- `git diff --check`: 成功。
- 文書のみの変更であるため package test は実施していない。
- Codex、Cursor、Claude Code の現行公式文書へ再接続入口を照合した。

## リスク・ブロッカー

- Cursor の利用実態は Issue コメントで確認済み。現時点の blocker はない。

## セッションログ

- 2026-07-11: Issue #161 を開始。GitHub 上の現況と依存を確認し、作業ブランチを作成した。
- 2026-07-11: 正本を更新し、tool mapping、参照整合性、文体、working branch note の情報統制を検証した。
- 2026-07-11: PR #163 の review 指摘 3 件を確認し、2 件の guideline 改善と note 採番を反映した。
- 2026-07-11: 別経路の指摘を検証し、Cursor CLI の `/mcp list` と `agent mcp enable` / `agent mcp disable` の役割を現行公式文書に合わせた。
