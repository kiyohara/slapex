# github-op-integrated MCP host 設定

`github-op-integrated` server は、次の project MCP 設定ファイルで有効化する。これらは secret を含まない project 設定として commit する。

- Cursor: `.cursor/mcp.json`
- Claude Code: repo root の `.mcp.json`
- Codex: `.codex/config.toml`

利用前に [`README.md`](./README.md) のセットアップ手順(project root の `.config/github-op-integrated.conf.example` を `.config/github-op-integrated.conf` にコピーして 1Password reference を記入する)を完了させておく。

wrapper には通常引数が不要で、設定は project root の `.config/github-op-integrated.conf` から読まれる。`--config` で任意の config file を指定できる。

## Cursor — `.cursor/mcp.json`

```json
{
  "mcpServers": {
    "github-op-integrated": {
      "type": "stdio",
      "command": "./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh"
    }
  }
}
```

`.mcp.json` / `.codex/config.toml` と揃えて project root 基準の相対 `command` を使う。相対 path は IDE / agent の両方で project root を基準に解決される。cursor-agent(CLI / Agent chat)では `${workspaceFolder}` は利用できない。経緯は `doc/design/decision-log/0053-cursor-mcp-config-path.md`。

## Claude Code — repo root の `.mcp.json`

```json
{
  "mcpServers": {
    "github-op-integrated": {
      "type": "stdio",
      "command": "./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh"
    }
  }
}
```

Claude Code は repo root の `.mcp.json` を project scope として読む。
この相対 `command` は MCP host が project root を基準に解決する前提である。wrapper 起動後は wrapper 自身の配置から project root を解決する。

## Codex — `.codex/config.toml`

```toml
[mcp_servers.github-op-integrated]
command = "./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh"
startup_timeout_sec = 30
tool_timeout_sec = 60
```

Codex は trusted project の `.codex/config.toml` を project scope として読む。project root 基準の相対 path で wrapper を指定するため、個人環境に依存する絶対 path は不要である。wrapper 起動後は wrapper 自身の配置から project root を解決する。

## 任意: Docker image を固定する

特定 tag や digest に固定したい場合は、各 host の `env` field で `GITHUB_OP_INTEGRATED_IMAGE` を設定する。Claude Code の例。

```json
{
  "mcpServers": {
    "github-op-integrated": {
      "type": "stdio",
      "command": "./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh",
      "env": {
        "GITHUB_OP_INTEGRATED_IMAGE": "ghcr.io/github/github-mcp-server:vX.Y.Z"
      }
    }
  }
}
```

Cursor と Codex にも同等の `env` field がある。具体的な書式はそれぞれの MCP ドキュメントを参照する。

## 任意: config file を明示する

通常は指定しない。wrapper が自身の配置から project root を解決し、`.config/github-op-integrated.conf` を読む。

やむを得ず別の config file を使う場合は、wrapper の `--config` option を使う。この override にも実 token は書かない。config file 側には 1Password secret reference を置く。
