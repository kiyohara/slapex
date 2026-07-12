# 作業ブランチメモ

- ブランチ: `update-mcp-guidelines-resolve-row`
- PR: #166
- 最終更新: 2026-07-12

## 目的

Issue #165 に従い、GitHub MCP ガイドラインの Review thread 解決行を、fine-grained PAT の権限要件を踏まえた手動 resolve 運用へ合わせる。

## 現在の状況

- 依存する Issue #162 / PR #164 の merge を GitHub MCP で確認済み。
- `doc/guidelines/github-mcp-guidelines.md` の操作表を更新した。
- Issue 指定の静的検証を完了した。
- PR #166 を作成し、working branch note を採番した。
- 2 件の review comment は同じ stale 記述を指摘しており、妥当と判断して最小修正した。
- review comment 対応後の静的検証を完了した。
- 両 inline comment へ返信し、対応 commit `cb08318` の push と CI 成功を確認した。
- 手動 resolve 方針の理由を全関連文書で PAT permission 方針へ統一した。

## 決定事項

- Review thread の resolve は自動実行せず、人間が GitHub UI で行う。
- fine-grained PAT に `Contents: write` は追加しない。
- MCP tool の `resolve_thread` と `gh api graphql` は機能上利用できる。手動運用の理由は tool / command の機能制約ではなく、本プロジェクトの PAT permission 方針である。
- resolve 可マーカーなどの詳細運用は `.agents/skills/review-pull-request/SKILL.md` を参照し、ガイドラインには複製しない。
- 単発 Issue のため `progress.md` は更新しない。
- skill の resolve 運用は変更せず、ガイドライン更新後に事実と食い違う移行説明 1 文だけを削除する。

## 次にやること

- 整合性修正を commit / push し、PR description を更新する。
- PR #166 の再確認と merge を待つ。

## 検証

- 操作表の「Review thread の解決」行が手動運用、fine-grained PAT の権限要件、MCP tool / `gh` の機能制約ではないこと、詳細運用の skill 参照を示すことを目視確認した。
- `git ls-files | xargs rg -n 'resolve_thread'`: 正本・入口に自動 resolve 前提の記述が残っていないことを確認した。skill の自動実行禁止記述と過去の working branch note の経緯記録だけが該当した。
- `AGENTS.md`、`.claude/rules/github-mcp-guidelines.md`、`.cursor/rules/github-mcp-guidelines.mdc` が正本を参照し、操作表を複製していないことを確認した。
- review skill の stale な移行説明が削除され、手動 resolve 方針と tool 提供範囲の記述だけが残ることを確認した。
- `.claude/skills/review-pull-request` が正本への symlink のままで、symlink 経由の内容が正本と一致することを確認した。
- broken symlink が無いことを確認した。
- tracked Markdown / rule 全体を検索し、MCP tool / `gh` の機能不足を手動 resolve の理由とする記述が残っていないことを確認した。
- ガイドライン、review skill、verify-comments reference、関連 working branch note が、人間による手動 resolve で一致することを確認した。
- `git diff --check`: 成功。
- 文書のみの変更のため package test は実施していない。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-07-12: PR #164 の merge を確認し、ブランチを作成。ガイドラインの操作表を手動 resolve 運用へ更新した。
- 2026-07-12: Issue 指定の静的検証を完了した。
- 2026-07-12: PR #166 を作成し、working branch note を採番した。
- 2026-07-12: Cursor / Claude Code review の同一指摘を採用し、skill の stale な移行説明 1 文を削除した。
- 2026-07-12: 手動 resolve の理由を本プロジェクトの PAT permission 方針へ統一し、MCP tool / `gh` の機能制約ではないことを明記した。
