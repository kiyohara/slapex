# GitHub MCP 利用ルール

この文書は、slack_posts_exporter リポジトリで AI agent が GitHub 操作を実行するときに、GitHub MCP Server(公式)を優先的に使うための共通正本である。

対応する MCP server 共通資材は `.agents/mcp/github-op-integrated/` に置く。導入手順、wrapper script、各 tool 用の設定例はそちらの `README.md` と `config-examples.md` を参照する。

## 適用範囲

このルールが対象とする GitHub 操作は次のとおり。

- collaboration write 操作: PR の作成・編集、issue の作成・編集・コメント追加、レビューコメントへの返信。
- read 操作全般: PR / issue / レビューコメントの取得、検索。

このルールが対象としない GitHub 操作は次のとおり(対応する別ルールに従う)。

- `git commit`、`git tag -s`、GitHub の SSH remote を使う `git push` / `git fetch` / `git pull`: `doc/guidelines/git-operation-guidelines.md`。
- merge、file push、release 作成、workflow dispatch、repository settings 変更、protected branch / secrets / org 管理など、初期 MCP 化対象に含めない高リスク write 操作: `doc/guidelines/github-cli-guidelines.md` に従い `gh` で実行する。

## MCP 優先・`gh` fallback

`gh` 直接実行ごとに 1Password 承認ダイアログが必要になる頻度を下げるため、対象の GitHub 操作はまず GitHub MCP Server 経由で実行する。

優先順位:

1. GitHub MCP Server が設定済みで、操作対象が allowlist 内の MCP tool で完結する場合は MCP を使う。
2. 操作が MCP の allowlist に無い、MCP が未設定、MCP が起動失敗・応答失敗するなどの場合は、`doc/guidelines/github-cli-guidelines.md` に従って `gh` に fallback する。
3. local git / SSH / commit signing を伴う操作は MCP に寄せず、`doc/guidelines/git-operation-guidelines.md` に従う。

`github-op-integrated` MCP server は Docker container として起動する。MCP 起動失敗が Docker 未起動、Docker daemon 接続不可、または Docker command 利用不可に見える場合は、即座に `gh` へ fallback しない。まず `doc/guidelines/development-command-guidelines.md` の Docker 確認方針に従い、Docker を起動して MCP を再試行できるか確認する。

MCP が使える環境かどうかが事前に判断できない場合は、まず MCP を試してよい。エラー時の fallback 手順は、read 系と write 系で扱いを分ける。

- read 系 (`pull_request_read`、`list_*`、`search_*` など): 同じ操作を `gh` で再実行してよい。
- write 系 (`create_pull_request`、`update_pull_request`、`issue_write`、`add_issue_comment`、`add_reply_to_pull_request_comment` など): MCP server 側で操作が成功した後に応答だけ失敗するケースがあり、素朴に `gh` で同じ create / update を再実行すると PR / issue / コメントが二重に作成される。fallback の前に read 系 tool で対象の現状を確認し、未反映であることが確認できた場合だけ `gh` で再実行する。

## 安全性の担保

MCP 化の目的は 1Password 承認ダイアログの頻度を下げることであり、人間承認を不要にすることではない。

- MCP server には broad な `default` / `all` toolset を渡さず、`GITHUB_TOOLS` で必要 tool だけを allowlist する。初期 allowlist の候補は `.agents/mcp/github-op-integrated/github.env.example` と `.agents/mcp/github-op-integrated/config-examples.md` を参照する。
- MCP 経由で write 操作を行う場合も、AI agent はユーザー承認(各 tool の MCP 承認 UI、明示の確認応答など)を取る。
- read-only profile は `.agents/mcp/github-op-integrated/README.md` / `config-examples.md` に併記する任意の安全設定として残す。read のみを許容したい用途では `GITHUB_READ_ONLY=1` を使う。

## tool allowlist の運用

- 初期 allowlist は collaboration write を MCP 化することを目的とし、PR / issue / レビューコメント関連 tool に絞る。
- merge、file push、release、workflow dispatch、repository settings 変更などは初期 allowlist に含めない。これらを MCP 化したい強い動機が出てきたら、別途レビューしてから allowlist を広げる。
- allowlist の正確な tool 名は GitHub MCP Server の現行 README / release に従う。`.agents/mcp/github-op-integrated/` 配下の README と `<name>.env.example` を更新したときは、本ルール本文の対応関係も同期する。
- CI / Actions の調査まで MCP 経由で行う必要が明確になった場合のみ、`actions` toolset または Actions 個別 tool を追加する。

## secret と設定ファイルの扱い

- `GITHUB_PERSONAL_ACCESS_TOKEN` は 1Password に保管し、wrapper script から `op run --env-file` で解決する。
- `.agents/mcp/github-op-integrated/github.env.example` には実 vault 名・実 item 名・実 token を書かない。`op://<VAULT>/<ITEM>/<FIELD>` 形式の完全 placeholder と allowlist だけを置く。
- 各 tool の MCP 設定ファイル(`.cursor/mcp.json` / repo root `.mcp.json` / `.codex/config.toml` / `~/.codex/config.toml`)は各ユーザー環境の入口とし、本 repo では原則 commit しない。設定例は `.agents/mcp/github-op-integrated/README.md` と `config-examples.md` に集約する。
- repo に置く実 secret ファイル(例: `.agents/mcp/github-op-integrated/github.env`)は `.gitignore` で除外する。

詳細な配置ルールと禁止事項は `doc/guidelines/agent-configuration-management.md` の「MCP server 共通資材管理」を正本とする。

## 関連ルール

- `doc/guidelines/agent-configuration-management.md`: MCP 共通資材の配置ルール。
- `doc/guidelines/development-command-guidelines.md`: Docker / Docker Compose の確認と開発コマンド実行ルール。
- `doc/guidelines/github-cli-guidelines.md`: `gh` fallback のルール。
- `doc/guidelines/git-operation-guidelines.md`: `git` コマンド本体の 1Password 連携ルール。
- `.agents/mcp/github-op-integrated/`: GitHub MCP Server 共通資材の正本置き場。
