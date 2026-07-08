#!/usr/bin/env bash
# worktree-setup: git worktree に gitignored な local config を配置する。
#
# 背景:
#   tracked な MCP 起動定義(.mcp.json / .cursor/mcp.json / .codex/config.toml)は
#   worktree に入るが、gitignored な local config(例: .config/github-op-integrated.conf)は
#   worktree へ自動配置されない。fresh worktree ではこの local config が無いため、
#   MCP wrapper が config file を見つけられず fail-loud で停止する。
#   この script は `.worktreeinclude` に allowlist した local config だけを、
#   main worktree から現在の worktree へコピーしてその状態を補う。
#
# 安全策:
#   - `.worktreeinclude` に明示したファイルだけを扱う。`.gitignore` 全体はコピーしない。
#   - allowlist には raw secret を含むファイルを載せない前提(1Password secret
#     reference を書いた config だけを想定)。
#   - ファイルの内容は出力・log しない。file path と動作だけを表示する。
#
# 使い方:
#   .agents/scripts/worktree-setup.sh [--source <dir>] [--force]
#
# 詳細: doc/guidelines/agent-configuration-management.md の
#       「worktree での ignored local config」セクション。

set -euo pipefail

self="worktree-setup"

usage() {
  cat <<'EOF'
usage: worktree-setup.sh [--source <dir>] [--force]

`.worktreeinclude` に列挙した gitignored な local config を、main worktree から
現在の git worktree へコピーする。tracked な MCP 起動定義は worktree に入るが、
gitignored な local config は入らないため、それを補う。

options:
  --source <dir>  コピー元 worktree(既定: 現在の worktree に対応する main worktree)
  --force         destination に既存ファイルがあっても上書きする(既定: 上書きしない)
  -h, --help      この help を表示する
EOF
}

force=0
source_dir=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --source)
      [ "$#" -ge 2 ] || { echo "$self: --source にはパスが必要" >&2; exit 2; }
      source_dir="$2"; shift 2 ;;
    --source=*) source_dir="${1#--source=}"; shift ;;
    --force) force=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "$self: unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# destination = 現在の worktree の root。
if ! dest_dir="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  echo "$self: git worktree の中で実行する必要がある" >&2
  exit 2
fi

# source 未指定なら main worktree を自動判定する。
# linked worktree では --git-common-dir が main の .git を指す。その親が main worktree。
if [ -z "$source_dir" ]; then
  common_git_dir="$(git -C "$dest_dir" rev-parse --git-common-dir)"
  case "$common_git_dir" in
    /*) ;;
    *) common_git_dir="$dest_dir/$common_git_dir" ;;
  esac
  source_dir="$(cd "$(dirname "$common_git_dir")" && pwd)"
fi

include_file="$dest_dir/.worktreeinclude"
if [ ! -f "$include_file" ]; then
  echo "$self: allowlist file が無い: $include_file" >&2
  exit 2
fi

copied=0
skipped=0
missing=0

while IFS= read -r raw || [ -n "$raw" ]; do
  line="${raw%%#*}"
  # 前後の空白を除去する。
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  [ -n "$line" ] || continue

  # allowlist は repo root からの相対 path だけを許可する安全境界。絶対 path や
  # `..` segment を含む entry は repo root 外(worktree 外)を指し得るため拒否する。
  case "$line" in
    /*|..|../*|*/../*|*/..)
      echo "$self: allowlist entry が repo root 外を指し得るため拒否: $line" >&2
      echo "  .worktreeinclude は repo root からの相対 path のみ許可する('/' 始まりや '..' segment は不可)。" >&2
      exit 2 ;;
  esac

  src="$source_dir/$line"
  dst="$dest_dir/$line"

  if [ ! -e "$src" ]; then
    echo "$self: source に無いためスキップ: $line" >&2
    if [ "$line" = ".config/github-op-integrated.conf" ]; then
      echo "  main worktree ($source_dir) で次を実行して用意する:" >&2
      echo "    cp .config/github-op-integrated.conf.example .config/github-op-integrated.conf" >&2
      echo "    \$EDITOR .config/github-op-integrated.conf   # 1Password secret reference を記入" >&2
      echo "  詳細: .agents/mcp/github-op-integrated/README.md" >&2
    fi
    missing=$((missing + 1))
    continue
  fi

  if [ "$src" -ef "$dst" ]; then
    # source と destination が同一ファイル(main worktree 上での実行など)。
    echo "$self: 既に配置済み: $line"
    skipped=$((skipped + 1))
    continue
  fi

  if [ -e "$dst" ] && [ "$force" -ne 1 ]; then
    echo "$self: 既に存在するためスキップ: $line (上書きするには --force)"
    skipped=$((skipped + 1))
    continue
  fi

  mkdir -p "$(dirname "$dst")"
  cp -p "$src" "$dst"
  echo "$self: コピーした: $line"
  copied=$((copied + 1))
done < "$include_file"

echo "$self: 完了 (copied=$copied skipped=$skipped missing=$missing)"
echo "$self:   source=$source_dir"
echo "$self:   dest=$dest_dir"

if [ "$missing" -gt 0 ]; then
  echo "$self: 未配置の local config がある。上記の手順で用意してから再実行する。" >&2
fi
