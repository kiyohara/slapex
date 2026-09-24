# Cloud Session 実行ルール

- 詳細は `doc/guidelines/cloud-session-guidelines.md`
- cloud session(Claude Code on the web、`CLAUDE_CODE_REMOTE=true`)で作業するとき、または cloud session 向けの設定(`.agents/scripts/cloud-session-setup.sh`、`.claude/settings.json`、`compose.cloud.yaml`、environment の setup script)を変更する前に共通正本を読む。
- 開発コマンドは cloud session でも `docker compose run --rm dev go ...`。daemon は SessionStart hook が起動する。hook の出力が無ければ `bash .agents/scripts/cloud-session-setup.sh --force` を実行する。起動しなければ host の `go` で代替せず、正本に従って報告する。
- GitHub 操作は組み込みの GitHub tool を使い、`github-op-integrated` の allowlist 外の tool は見えていても使わない。
- tool 固有入口だけに恒久ルールを書かない。
