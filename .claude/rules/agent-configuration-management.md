---
paths:
  - "doc/guidelines/agent-configuration-management.md"
  - ".cursor/rules/**/*.md"
  - ".cursor/rules/**/*.mdc"
  - ".claude/rules/**/*.md"
  - ".agents/skills/**/*"
  - ".agents/mcp/**/*"
  - ".claude/skills/**/*"
  - ".github/copilot-instructions.md"
  - ".github/instructions/**/*.instructions.md"
  - "AGENTS.md"
  - "CLAUDE.md"
---

# Agent 設定管理

- 詳細は `doc/guidelines/agent-configuration-management.md`
- AI agent 向け skill / rule / MCP 共通資材や入口ファイルを作成・削除・rename する前に、必ず共通正本の該当 checklist を読む。
- tool 固有入口だけに恒久ルールを書かない。
- 新しい rule を作ったら `AGENTS.md` の「共通正本」セクションに必ずリンクを追加する。
