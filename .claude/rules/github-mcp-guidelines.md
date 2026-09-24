# GitHub MCP 利用ルール

- 詳細は `doc/guidelines/github-mcp-guidelines.md`
- GitHub 操作は MCP 優先。最初の試行先は `github-op-integrated` MCP tool とし、read / write とも `gh` より先に MCP tool の利用可否を確認する。MCP 未対応 / 未設定 / 失敗時は `doc/guidelines/github-cli-guidelines.md` に従い `gh` fallback
- cloud session(Claude Code on the web)では `github-op-integrated` は起動しない。組み込みの GitHub tool を第一選択にし、allowlist 外の tool は見えていても使わない。`gh` は組み込み tool に無い操作だけを `gh api`(REST)で補う(正本の「cloud session(Claude Code on the web)」)
- local git / SSH / commit signing は MCP に寄せず、`doc/guidelines/git-operation-guidelines.md` に従う
- tool 固有入口だけに恒久ルールを書かない
