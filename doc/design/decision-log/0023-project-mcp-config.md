# 0023 project MCP config

- 状態: decided
- 作成日: 2026-06-04
- 最終更新日: 2026-06-04
- 関連: `doc/guidelines/agent-configuration-management.md`, `doc/guidelines/github-mcp-guidelines.md`, `.agents/mcp/github-op-integrated/`, [0053-cursor-mcp-config-path.md](0053-cursor-mcp-config-path.md)

## 背景

Cursor / Claude Code / Codex の各 agent で `github-op-integrated` MCP server を使う方針になっている。従来は MCP host 設定ファイルと `github.env` を個人環境入口として扱っていたが、worktree では ignored file が自動で配置されず、MCP server が利用できない状態になりやすい。

この repository では、GitHub MCP 用 PAT は repository 固有の fine-grained PAT を 1Password に保存して利用する。そのため、MCP server の起動定義は project 設定として repository に紐付ける方が運用と合う。

## 候補

- MCP host 設定を引き続き各ユーザーが手動作成する。
- `.worktreeinclude` や setup script で ignored file を worktree にコピーする。
- secret-free な MCP host 設定を project 設定として commit し、secret reference は MCP server 専用 config で管理する。

## 検討内容

手動作成は secret 管理としては安全だが、worktree ごとの設定漏れが起きやすい。ignored file のコピーは Claude Code には合うが、Codex の MCP 初期化タイミングと衝突する可能性がある。

project 設定として commit する案は、各 agent の公式仕様に沿っている。Claude Code は `.mcp.json` を project scope として扱い、Cursor は `.cursor/mcp.json` を project-specific tools 用に扱い、Codex は trusted project の `.codex/config.toml` を project-scoped config として読む。

## 決定

`.cursor/mcp.json`、repo root の `.mcp.json`、`.codex/config.toml` は secret を含まない project MCP 設定として git 管理する。

`github-op-integrated` の環境変数 template は `.config/github-op-integrated.conf.example` に置き、実際の local 設定は `.config/github-op-integrated.conf` に置く。config file には raw token を書かず、1Password secret reference を置く。

`.agents/mcp/github-op-integrated/github.env` / `github.env.example` 方式は廃止する。legacy local env file は誤追跡を避けるため ignore 対象として残す。

## 理由

MCP host 設定は project の capability を表すものであり、個人ごとの secret ではない。secret-free な起動定義を git 管理すると、worktree と複数 agent の利用時に設定漏れを減らせる。

secret reference は MCP server 専用 config file に分離することで、repository 固有 PAT という要件を保ちつつ、MCP tool に渡す環境変数を必要最小限に絞れる。

## 影響

- `.cursor/mcp.json`、`.mcp.json`、`.codex/config.toml` は git 管理対象になる。
- `.config/github-op-integrated.conf.example` は `github-op-integrated` 専用の環境変数 template になる。
- `.config/github-op-integrated.conf` は `.gitignore` で除外する。
- `github-op-integrated` wrapper は `.config/github-op-integrated.conf` を `op run --env-file` で解決して起動する。
- 旧 `GITHUB_OP_INTEGRATED_ENV_FILE` override は廃止し、任意の config file を明示する場合は wrapper の `--config` option を使う。
- setup 手順は MCP host 設定のコピーではなく、専用 config file の作成と 1Password secret reference の記入が中心になる。
- project-wide `.env` の採用は今回の PR では扱わず、将来必要になった時点で別途検討する。

## 後から見直す条件

- Codex の worktree 初期化と MCP 起動順により、tracked `.codex/config.toml` だけでは初回 MCP 起動が安定しないことが分かった場合。
- repository 固有 PAT ではなく、複数 repository を横断する GitHub MCP 運用へ変更する場合。
- GitHub hosted remote MCP server へ移行し、local Docker wrapper が不要になった場合。
