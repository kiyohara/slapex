# 作業ブランチメモ

- ブランチ: track-project-mcp-config
- PR: #7
- 最終更新: 2026-06-04

## 目的

MCP host 設定を project 設定として git 管理し、`github-op-integrated` を Cursor / Claude Code / Codex から共通利用できる状態にする。

## 現在の状況

- `.cursor/mcp.json` / `.mcp.json` / `.codex/config.toml` を project MCP 設定として追加済み。
- `github-op-integrated` は `.config/github-op-integrated.conf` を読むように変更済み。
- `.config/github-op-integrated.conf.example` に `github-op-integrated` 用の環境変数 placeholder と allowlist を移行済み。

## 決定事項

- `.cursor/mcp.json` / `.mcp.json` / `.codex/config.toml` は secret を含まない project MCP 設定として git 管理する。
- `github-op-integrated` の config template は `.config/github-op-integrated.conf.example` に置き、実設定は `.config/github-op-integrated.conf` に置く。
- raw token は repo に置かず、専用 config file には 1Password secret reference placeholder を使う。

## 次にやること

- PR #7 の review comment 対応を push する。

## 検証

- `jq . .mcp.json .cursor/mcp.json`: 成功。
- `.codex/config.toml` の TOML parse: 成功。
- `bash -n .agents/mcp/github-op-integrated/mcp-github-op-integrated.sh`: 成功。
- `github-op-integrated` wrapper と `.codex/config.toml` から git command 依存が消えていることを確認。
- config 不在時の wrapper message: `.config/github-op-integrated.conf` を探して setup 手順を案内することを確認。
- wrapper の `--config` / `--config=<path>` 経路を確認。template を直接指定した場合は placeholder secret reference が解決できず失敗することを確認。
- `git diff --check`: 成功。
- 旧 `github.env` 参照検索: 廃止理由と過去 note 補足だけに残っていることを確認。
- project-wide `.env` / `.env.example` の MCP tool 向け参照が残っていないことを確認。
- Codex app 再起動後、`github-op-integrated` が MCP tool として露出し、`list_pull_requests` が `kiyohara/slapex` に対して成功することを確認。

## リスク・ブロッカー

- Codex worktree 初期化時に `.config/github-op-integrated.conf` が存在しない場合、MCP server の初回起動が失敗する可能性がある。

## セッションログ

- 2026-06-04: `track-project-mcp-config` ブランチを作成し、project MCP 設定の git 管理化方針で作業開始。
- 2026-06-04: project MCP 設定、専用 config example、wrapper、関連 guideline / README / decision log を更新。
- 2026-06-04: review 指摘を反映。local secret config の `.gitignore` 追加を条件付き記述にし、削除 checklist から `.gitignore` 削除対象を外した。`/absolute/path/to/project` 例も削除した。
- 2026-06-04: manager feedback を反映。project-wide `.env` を MCP tool に渡す案をやめ、`.config/github-op-integrated.conf` 専用方式へ変更。
- 2026-06-04: review 指摘を反映。環境変数経由の config override と git command 依存を削除し、`.config/*.conf.example` の配置方針を明確化。
- 2026-06-04: Codex app 再起動後の実利用確認を実施。project MCP 設定から `github-op-integrated` が起動し、GitHub API read tool が成功することを確認。
- 2026-06-04: PR #7 review summary に対応。read-only 参照、相対 command path の前提、旧 env override 廃止の移行メモを文書へ反映。
