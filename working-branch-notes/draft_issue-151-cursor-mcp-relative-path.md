# 作業ブランチメモ

- ブランチ: issue-151-cursor-mcp-relative-path
- PR: -
- 最終更新: 2026-07-08

## 目的

Issue #151 の対応。`.cursor/mcp.json` の `github-op-integrated` 起動 command が
`${workspaceFolder}` を使っており、cursor-agent (CLI / Agent chat) はこの変数を
展開しないため spawn が `ENOENT` で失敗する。project root 基準の相対 path に
統一し、Cursor IDE / cursor-agent の両方で MCP が起動するようにする。

## 現在の状況

- Issue #151 全文を確認済み(gh REST 経由で本文取得)。
- 現状の再現を確認済み:
  - この Agent セッションの MCP tool には `github-op-integrated` が出ていない。
  - `cursor agent mcp list` → `github-op-integrated: Error: Connection failed`。
  - `cursor agent mcp list-tools github-op-integrated` →
    `Failed to load MCP ...: spawn ${workspaceFolder}/.agents/.../mcp-github-op-integrated.sh ENOENT`。
  - wrapper 単体(手動起動)と MCP initialize/tools/list は正常。
- 一次原因は Issue 記載どおり `${workspaceFolder}` 未展開の path 差。

## 決定事項

- `.cursor/mcp.json` の command を `./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh`
  へ変更(`.mcp.json` / `.codex/config.toml` と揃える)。
- `config-examples.md` の Cursor 節から `${workspaceFolder}` 推奨を外し、
  IDE / agent 両対応の相対 path 記述へ更新する。
- README トラブルシューティングに cursor-agent の ENOENT / 変数未展開項目を追加する。
- host 差分と相対 path 統一を decision log に追記する(0023 の path 推奨を補正する新規ログ)。

## 次にやること

- 上記の実装。
- 検証(受け入れ条件)を実行して note に記録。
- PR 作成(Closes #151)。

## 検証

- `.cursor/mcp.json` を相対 path に修正後、repo root で `cursor agent mcp list`:
  修正前 `Error: Connection failed` → 修正後 `not loaded (needs approval)` に変化。
  `${workspaceFolder}` 未展開による `ENOENT` が解消したことを確認。
- `cursor agent mcp enable github-op-integrated` で承認後、
  `cursor agent mcp list-tools github-op-integrated` が 13 tool を返した
  (allowlist と一致: list_pull_requests / pull_request_read / issue_write ほか)。
  MCP server が起動し MCP protocol に応答している。
- 手動 wrapper 起動(`... </dev/null` + initialize / tools/list)は本セッション冒頭で成功済み。
  GitHub MCP Server v1.5.0 が起動し tool を返すことを確認。
- 未実施(ユーザー確認事項): Cursor IDE を再起動しての MCP パネル connected 表示、
  Agent chat セッションからの MCP read tool 実呼び出し。IDE 側は相対 path でも
  project root 基準で解決される前提。
- `cursor agent mcp enable` は local approval 状態(`~/.cursor` 側)を変えるだけで
  repo には含めない。

## リスク・ブロッカー

- 本作業中は `github-op-integrated` MCP が壊れている前提。GitHub read/write は
  `gh` fallback。ただしこのシェルでは `gh` = `op plugin run -- gh` が対話 IO 不可で
  失敗するため、`.config/github-op-integrated.conf` の PAT を `op run` で解決して
  REST API を叩く方式を使う。
- path 修正後に cursor-agent 側の PATH 狭小(`op` が見つからない等)や `op` の
  非対話 unlock といった二次問題が出る可能性がある。これは Issue #151 のスコープ外
  (別 issue / follow-up)。

## セッションログ

- 2026-07-08: `main` から `issue-151-cursor-mcp-relative-path` を作成。Issue #151 の
  現象を再現確認し、原因が `${workspaceFolder}` 未展開であることを特定。相対 path 統一
  方針で実装に着手。
