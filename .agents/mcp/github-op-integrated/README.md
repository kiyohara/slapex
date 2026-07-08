# github-op-integrated MCP server

このディレクトリは、**github-op-integrated** MCP server の共通資材を置く場所である。公式の [GitHub MCP Server](https://github.com/github/github-mcp-server) Docker image を、1Password CLI(`op run`)で secret を解決しながら起動する薄い wrapper を提供する。

目的は、Cursor / Claude Code / Codex から GitHub の collaboration 操作(PR / issue / レビューコメントの read & write)を行うときに、操作ごとに `gh` を呼んで 1Password 承認ダイアログが出る状況を減らすことである。

AI agent が GitHub の PR / issue / レビューコメントを操作する場合、この server の tool が最初の試行先になる。`gh` で事前確認する前に、まず MCP host 上で `github-op-integrated` の tool が利用可能か確認する。

`gh` と MCP の使い分けなどプロジェクト側の方針は [doc/guidelines/github-mcp-guidelines.md](../../../doc/guidelines/github-mcp-guidelines.md) を参照する。MCP 共通資材の配置規約は [doc/guidelines/agent-configuration-management.md](../../../doc/guidelines/agent-configuration-management.md) の「MCP server 共通資材管理」セクションを参照する。

## ファイル

| ファイル | commit | 役割 |
| --- | --- | --- |
| `mcp-github-op-integrated.sh` | yes | 各 MCP host から起動される wrapper script。`.config/github-op-integrated.conf` を `op run` で解決し、GitHub MCP Server の Docker image を exec する。 |
| `config-examples.md` | yes | Cursor / Claude Code / Codex の project MCP 設定と任意 override の説明。 |
| `README.md` | yes | このファイル。 |
| `.config/github-op-integrated.conf.example` | yes | project root に置く `github-op-integrated` 専用の設定 template。placeholder と allowlist だけを含む。 |
| `.config/github-op-integrated.conf` | **no** | project root に置く local config file。各ユーザーの 1Password secret reference を書く。commit してはならない。 |

MCP host 用の project 設定ファイル(`.cursor/mcp.json`、repo root の `.mcp.json`、`.codex/config.toml`)は secret を含まない project 設定として commit する。個人環境に依存する絶対 path、実 vault 名、実 item 名、実 token は書かない。

## 前提条件

- [Docker](https://www.docker.com/) がインストールされ、daemon が起動していること。
- [1Password CLI(`op`)](https://developer.1password.com/docs/cli/) がインストール・認証済みで、biometric unlock または session が有効であること。
- 1Password に、この repository だけに権限を絞った GitHub fine-grained Personal Access Token(PAT)が保存されていること。
  - PR / Issues / レビューコメントの read & write を許可する。

## セットアップ

1. config template をコピーし、1Password secret reference を記入する。

   ```sh
   cp .config/github-op-integrated.conf.example .config/github-op-integrated.conf
   $EDITOR .config/github-op-integrated.conf
   ```

   `op://<VAULT>/<ITEM>/<FIELD>` を、自分の 1Password 上の PAT を指す reference に置き換える。raw token は config file に書かない。`GITHUB_TOOLS` の allowlist は、レビューを経た変更でない限り初期値のまま使う。

2. wrapper が使う環境変数を確認する。token 値そのものは端末ログ・画面共有・MCP / IDE のログ収集に残るおそれがあるため、出力せず存在確認だけ行う。

   ```sh
   op run --env-file=.config/github-op-integrated.conf -- \
     sh -c '[ -n "$GITHUB_PERSONAL_ACCESS_TOKEN" ] && echo "PAT 解決済み"'
   ```

   `PAT 解決済み` と表示されれば OK。何も表示されない、または `op://...` のままになる場合は secret reference が解決できていないので、`op` の sign-in 状態と reference が指す item の存在を確認する。

3. MCP host を再起動し、tool 一覧に `github-op-integrated` の tool が現れることを確認する。

## worktree での利用

gitignored な `.config/github-op-integrated.conf` は新しい worktree へ自動配置されないため、worktree では wrapper が config file を見つけられず fail-loud で停止する。worktree を作成したら、その worktree の中で setup script を実行して local config を配置する。

```sh
.agents/scripts/worktree-setup.sh
```

main worktree に `.config/github-op-integrated.conf` があればコピーされる。無い場合は、上の「セットアップ」を先に main worktree で済ませてから再実行する。allowlist と script の詳細は `doc/guidelines/agent-configuration-management.md` の「worktree での ignored local config」を参照する。

## tool allowlist の方針

`.config/github-op-integrated.conf.example` のデフォルト `GITHUB_TOOLS` は、本プロジェクトの初期 MCP 化スコープ(利用頻度の高い collaboration write 操作と、それらを行うために必要な read)を反映している。

次の高リスク write 操作は意図的に除外している。

- `merge_pull_request`
- ファイル内容を branch に push する系の tool
- `create_release`
- workflow dispatch
- repository / branch protection / secrets / org settings 変更

新しい MCP 化対象を追加する場合は方針変更として扱い、`doc/guidelines/github-mcp-guidelines.md` に照らしてレビューしたうえで、`.config/github-op-integrated.conf.example` と該当 guideline 本文を同期する。

read-only profile は `.config/github-op-integrated.conf` で `GITHUB_READ_ONLY=1` を設定する形で運用する。

## Docker image と tag 固定

wrapper のデフォルトは `ghcr.io/github/github-mcp-server`(tag 指定なし、実質 `latest`)。tool の構成や名前はリリースごとに変わりうるため、運用方針は次のいずれかを選ぶ。

- デフォルトのまま使い、定期的に `docker pull ghcr.io/github/github-mcp-server` で更新する。更新後は `GITHUB_TOOLS` の allowlist が現行 tool 名と一致しているかを再確認する。
- 特定の tag または digest に固定する。各 MCP host 設定の `env` field で `GITHUB_OP_INTEGRATED_IMAGE` を設定する(`config-examples.md` 参照)。複数マシンで再現性が必要な場合はこちらを選ぶ。

## remote MCP server との関係

GitHub は hosted の remote MCP server も提供している。本 repo で local Docker wrapper を採用しているのは次の理由による。

- Cursor / Claude Code / Codex の 3 つで同じ起動経路を共有できる。
- この repository 専用の fine-grained PAT を 1Password から `op run` で解決でき、repo 内の他の GitHub credential 取り扱いと整合する。

remote 案を将来採らないとは決めていない。auth / capability の事情が変われば、本 wrapper を差し替えるだけで `doc/guidelines/github-mcp-guidelines.md` 側の方針は維持できる構成にしてある。

## トラブルシューティング

- **`'1Password CLI (op)' が PATH に見つからない`**: `op` をインストールし、MCP host を再起動する。
- **`'docker' が PATH に見つからない`**: Docker Desktop をインストール / 起動し、MCP host を再起動する。
- **`config file が見つからない`**: セットアップ手順 1 をスキップしている。`.config/github-op-integrated.conf.example` を `.config/github-op-integrated.conf` にコピーする。
- **tool 一覧が空 / tool 名が違う**: 起動中の image の tool 名が `GITHUB_TOOLS` とずれている。最新 image を pull し、[GitHub MCP Server README](https://github.com/github/github-mcp-server) と比較したうえで `.config/github-op-integrated.conf` / `.config/github-op-integrated.conf.example` を更新する。
- **呼び出しのたびに 1Password 承認ダイアログが出る**: 通常は `op run` の session が再利用されるため、session ごと 1 回で済むはず。毎回出る場合は session が短時間で expire している可能性が高い。`op signin` や biometric unlock の設定を確認する。
- **MCP host から起動直後にクラッシュと言われる**: wrapper を手動実行(`.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh </dev/null`)してエラーを直接確認する。
- **cursor-agent で `spawn ${workspaceFolder}/... ENOENT` になる**: cursor-agent(CLI / Agent chat)は `${workspaceFolder}` を展開しないため、この変数を含む `command` を文字列のまま spawn して失敗する。`.cursor/mcp.json` の `command` を project root 基準の相対 path(`./.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh`)にする。経緯は `doc/design/decision-log/0053-cursor-mcp-config-path.md`。
