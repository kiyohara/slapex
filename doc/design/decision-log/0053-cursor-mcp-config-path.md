# 0053 Cursor MCP 設定の command path

- 状態: decided
- 作成日: 2026-07-08
- 最終更新日: 2026-07-08
- 関連: `doc/guidelines/github-mcp-guidelines.md`, `.agents/mcp/github-op-integrated/config-examples.md`, [0023-project-mcp-config.md](0023-project-mcp-config.md)

## 背景

0023 で、secret-free な MCP 起動定義を project 設定として git 管理する方針を決めた。その実装で `.cursor/mcp.json` の `github-op-integrated` 起動 command には `${workspaceFolder}` を使い、`config-examples.md` にも「Cursor は `${workspaceFolder}` を project root に展開する」と記載していた。

2026-07-08 の調査(Issue #151)で、Cursor IDE では MCP が起動する一方、cursor-agent(CLI / Agent chat)では `github-op-integrated` が接続失敗することが分かった。原因は Docker / 1Password / サンドボックスではなく、path 変数の展開差である。cursor-agent は `${workspaceFolder}` を展開せず、文字列のまま子プロセスを spawn するため `ENOENT` になる。

## 候補

- `.cursor/mcp.json` の `command` を `${workspaceFolder}` のまま維持する。
- `.cursor/mcp.json` の `command` を project root 基準の相対 path に統一する。
- 個人環境依存の絶対 path を commit する。

## 検討内容

`${workspaceFolder}` 維持は Cursor IDE では動くが、cursor-agent では展開されず起動できない。本 repo 側で cursor-agent の変数展開を制御する手段はない。

相対 path(`./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh`)は、Cursor IDE / cursor-agent の両方が project root(cwd)を基準に解決できる。すでに `.mcp.json`(Claude Code)と `.codex/config.toml`(Codex)は相対 path を使っており、Cursor だけ変数展開に依存していた。wrapper は起動後に自身の配置から project root を再解決するため、command の書き方には依存しない。

絶対 path の commit は個人環境依存になり、`agent-configuration-management` の方針で禁止している。

## 決定

`.cursor/mcp.json` の `github-op-integrated` 起動 command を、project root 基準の相対 path `./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh` に統一する。0023 で採った `${workspaceFolder}` 推奨のうち、Cursor の command path に関する部分だけを本ログで補正する。0023 の他の決定(project 設定の git 管理、secret reference の分離など)は維持する。

## 理由

相対 path は Cursor IDE と cursor-agent の両方で解決でき、`.mcp.json` / `.codex/config.toml` と表記が揃う。host ごとの変数展開仕様の差に依存しないため、同じ MCP 起動定義を全 agent で共有するという 0023 の目的にむしろ合致する。

## 影響

- `.cursor/mcp.json` の `command` が相対 path になる。
- `.agents/mcp/github-op-integrated/config-examples.md` の Cursor 節から `${workspaceFolder}` 推奨を外し、IDE / agent 両対応の相対 path 記述へ更新する。
- `.agents/mcp/github-op-integrated/README.md` のトラブルシューティングに、cursor-agent の `${workspaceFolder}` 未展開による `ENOENT` 項目を追加する。
- 修正後に cursor-agent 側の PATH 狭小(`op` / `docker` が見つからない)や `op` の非対話 unlock といった二次問題が出る可能性があるが、これは本ログのスコープ外(別 issue / follow-up)。

## 後から見直す条件

- cursor-agent が将来 `${workspaceFolder}` 等の変数展開に対応し、全 host で表記を統一し直す動機が出た場合。
- MCP 起動定義の path 解決方式(相対 / 絶対 / 変数)を横断的に変更する必要が出た場合。
