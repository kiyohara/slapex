# 作業ブランチメモ

- ブランチ: `update-mcp-guidelines-resolve-row`
- PR: 未作成
- 最終更新: 2026-07-12

## 目的

Issue #165 に従い、GitHub MCP ガイドラインの Review thread 解決行を、fine-grained PAT の権限要件を踏まえた手動 resolve 運用へ合わせる。

## 現在の状況

- 依存する Issue #162 / PR #164 の merge を GitHub MCP で確認済み。
- `doc/guidelines/github-mcp-guidelines.md` の操作表を更新した。
- Issue 指定の静的検証を完了した。
- commit / push と PR 作成を進める。

## 決定事項

- Review thread の resolve は自動実行せず、人間が GitHub UI で行う。
- fine-grained PAT に `Contents: write` は追加しない。
- resolve 可マーカーなどの詳細運用は `.agents/skills/review-pull-request/SKILL.md` を参照し、ガイドラインには複製しない。
- 単発 Issue のため `progress.md` は更新しない。

## 次にやること

- 変更を commit / push して PR を作成する。
- PR 採番後に本 note を rename する。

## 検証

- 操作表の「Review thread の解決」行が手動運用、fine-grained PAT の権限要件、`gh` fallback 不可、詳細運用の skill 参照を示すことを目視確認した。
- `git ls-files | xargs rg -n 'resolve_thread'`: 正本・入口に自動 resolve 前提の記述が残っていないことを確認した。skill の自動実行禁止記述と過去の working branch note の経緯記録だけが該当した。
- `AGENTS.md`、`.claude/rules/github-mcp-guidelines.md`、`.cursor/rules/github-mcp-guidelines.mdc` が正本を参照し、操作表を複製していないことを確認した。
- `git diff --check`: 成功。
- 文書のみの変更のため package test は実施していない。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-07-12: PR #164 の merge を確認し、ブランチを作成。ガイドラインの操作表を手動 resolve 運用へ更新した。
- 2026-07-12: Issue 指定の静的検証を完了した。
