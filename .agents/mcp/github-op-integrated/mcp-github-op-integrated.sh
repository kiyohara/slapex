#!/usr/bin/env bash
# GitHub MCP Server を 1Password CLI による secret 解決付きで起動する wrapper。
#
# セットアップ手順は同ディレクトリの README.md と config-examples.md を参照。
# 各ユーザーの MCP host (Cursor / Claude Code / Codex) から呼ばれる前提で、
# 直接実行する用途は想定しない。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CONFIG_FILE="$REPO_ROOT/.config/github-op-integrated.conf"
IMAGE="${GITHUB_OP_INTEGRATED_IMAGE:-ghcr.io/github/github-mcp-server}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --config)
      if [ "$#" -lt 2 ]; then
        echo "mcp-github-op-integrated: --config requires a path" >&2
        exit 1
      fi
      CONFIG_FILE="$2"
      shift 2
      ;;
    --config=*)
      CONFIG_FILE="${1#--config=}"
      shift
      ;;
    *)
      echo "mcp-github-op-integrated: unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if ! command -v op >/dev/null 2>&1; then
  echo "mcp-github-op-integrated: '1Password CLI (op)' が PATH に見つからない" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "mcp-github-op-integrated: 'docker' が PATH に見つからない" >&2
  exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
  echo "mcp-github-op-integrated: config file が見つからない: $CONFIG_FILE" >&2
  echo "project root の .config/github-op-integrated.conf.example を .config/github-op-integrated.conf にコピーし、1Password secret reference を記入する。" >&2
  exit 1
fi

# サポート対象の環境変数だけを明示的に container へ転送する。
# `-e VAR` (値を指定しない形) は、`op run --env-file` が解決した親プロセスの環境変数を
# docker へ pass-through する。
DOCKER_ENV_ARGS=(-e GITHUB_PERSONAL_ACCESS_TOKEN)
for var in GITHUB_TOOLS GITHUB_TOOLSETS GITHUB_READ_ONLY GITHUB_DYNAMIC_TOOLSETS GITHUB_HOST; do
  DOCKER_ENV_ARGS+=(-e "$var")
done

# --no-masking の理由: MCP server は stdio 上の JSON-RPC で通信する。
# op のデフォルト masking は stdout 上の bytes を書き換えうるが、PAT 自体は
# MCP の応答に現れない。masking を切ることで stdio transport を阻害する
# リスクを排除する。
exec op run --no-masking --env-file="$CONFIG_FILE" -- \
  docker run -i --rm "${DOCKER_ENV_ARGS[@]}" "$IMAGE"
