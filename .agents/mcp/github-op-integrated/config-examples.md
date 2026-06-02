# github-op-integrated MCP host 設定例

`github-op-integrated` server を各 MCP host に登録するための、コピペ前提の設定例集。各 host 用の設定ファイル(`.cursor/mcp.json`、repo root の `.mcp.json`、`.codex/config.toml`)は各ユーザー環境の入口とみなし、本 repo では commit しない。必要な部分を自分の設定ファイルにコピーして使う。

利用前に [`README.md`](./README.md) のセットアップ手順(`github.env.example` を `github.env` にコピーして 1Password reference を記入する)を完了させておく。

wrapper には引数が不要で、設定はすべて `github.env` から読まれる。`GITHUB_OP_INTEGRATED_ENV_FILE` や `GITHUB_OP_INTEGRATED_IMAGE` などの任意 override を使う場合は、各 host の `env` field 経由で渡す。

## Cursor — `.cursor/mcp.json`

project scope(本 repo での推奨)。自分の作業 copy の repo root に `.cursor/mcp.json` として置く。Cursor は `${workspaceFolder}` を project root に展開する。

```json
{
  "mcpServers": {
    "github-op-integrated": {
      "type": "stdio",
      "command": "${workspaceFolder}/.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh"
    }
  }
}
```

すべての Cursor project で共通に有効化したい場合は global config(`~/.cursor/mcp.json`)を使う。global config には project context が無いため、`${workspaceFolder}` ではなく本 repo の絶対 path に置き換える。

## Claude Code — repo root の `.mcp.json`

project scope。Claude Code は repo root の `.mcp.json` を project scope として読む。自分の作業 copy の repo root に置く。本 repo では commit しないため、各 contributor が有効化のタイミングを自分で決められる。

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

絶対 path を使いたい場合や、任意の cwd から有効化したい場合は `mcp-github-op-integrated.sh` の絶対 path を指定する。

user scope での登録は `claude mcp add` でも可能。詳細は Claude Code の MCP ドキュメントを参照する。

## Codex — `~/.codex/config.toml` または `.codex/config.toml`

Codex の MCP 設定は TOML 形式。user 階層の `~/.codex/config.toml` はどこからでも有効になる。trusted project では repo root の `.codex/config.toml` が project scope として読まれる。

```toml
[mcp_servers.github-op-integrated]
command = "/absolute/path/to/slack_posts_exporter/.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh"
startup_timeout_sec = 30
tool_timeout_sec = 60
```

`command` は絶対 path で指定する(Codex は `${workspaceFolder}` を展開しない)。初回の `docker run` が遅い環境(Docker Desktop の cold start が長いケースなど)では timeout を調整する。

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

## 任意: read-only profile

デフォルトの write-capable server と並べて(あるいは置き換えて)read-only な server を動かす場合は、別の env file(例: `github.read-only.env`)を用意する。

```env
GITHUB_PERSONAL_ACCESS_TOKEN=op://<VAULT>/<ITEM>/<FIELD>
GITHUB_READ_ONLY=1
GITHUB_TOOLSETS=default
```

そのうえで、別名(例: `github-op-integrated-read-only`)の MCP server entry を登録し、`GITHUB_OP_INTEGRATED_ENV_FILE` でその env file の絶対 path を渡す。

```json
{
  "mcpServers": {
    "github-op-integrated-read-only": {
      "type": "stdio",
      "command": "./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh",
      "env": {
        "GITHUB_OP_INTEGRATED_ENV_FILE": "/absolute/path/to/slack_posts_exporter/.agents/mcp/github-op-integrated/github.read-only.env"
      }
    }
  }
}
```

read-only 用の env file も `github.env` と同様に gitignore 済み(`.agents/mcp/github-op-integrated/*.env` パターンでカバー)。
