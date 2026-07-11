# 作業ブランチメモ

- ブランチ: add-pr-review-skill
- PR: 未作成
- 最終更新: 2026-07-11

## 目的

Issue #162 に従い、slapex 専用の PR review / review comment 対応 skill `review-pull-request` を整備する。`review` / `address-comments` / `verify-comments` の 3 モードを持ち、github-op-integrated MCP-first の tool routing、review cycle 管理、Agent 識別と可視 metadata、コメント形式、resolve 制約を skill として定義する。

## 現在の状況

- 依存 #161 は PR #163 として merge 済みであることを確認した(main HEAD `ee9403e` が merge commit)。
- 更新後の `doc/guidelines/github-mcp-guidelines.md`(優先規則・操作別の第一選択表)が main に含まれることを確認した。
- Issue #162 のコメントは 0 件で、追加の調査結果・capability 変更は無い。
- `.config/github-op-integrated.conf.example` の現行 allowlist に、必要 tool(`list_pull_requests` / `pull_request_read` / `add_comment_to_pending_review` / `pull_request_review_write` / `add_reply_to_pull_request_comment` / `add_issue_comment`)がすべて含まれることを静的確認した。tool allowlist の追加は不要。

## 決定事項

- SKILL.md には共通 workflow(対象 PR の特定、Agent 識別と canonical metadata、capability 再利用条件、tool routing、event / resolve 制約、write failure 手順、反復上限)とモード選択を置き、モード固有手順は `references/review.md` / `references/address-comments.md` / `references/verify-comments.md` に分割する(Issue の指示)。
- 可視 metadata の canonical フォーマットは `Agent` / `Review cycle` / `Reviewed head` / `Mode` の 4 キー・この順序・`: ` 区切り・1 行 1 キーで SKILL.md に確定する。
- `address-comments` / `verify-comments` の返信では、元 review の review cycle ID をそのまま使い、新しい ID を作らない(cycle 突合のため)。
- review 完了コメント・再確認結果は `add_issue_comment`(PR conversation comment)へ一本化する。
- AGENTS.md への skill 名追記は行わない(自動 discover のため必須でないと Issue に明記)。
- `progress.md` の索引に #162 の行は無いため、更新しない(単発 Issue は無理に登録しない)。

## 次にやること

- PR 作成と note の採番 rename。

## 検証

Issue #162 記載の検証項目の実施結果(2026-07-11)。

- `test -f .agents/skills/review-pull-request/SKILL.md` — pass。
- `test -f .claude/skills/review-pull-request/SKILL.md` — pass(symlink 経由で読める)。
- `readlink .claude/skills/review-pull-request` — `../../.agents/skills/review-pull-request` を返すことを確認。
- frontmatter `name` が directory / symlink 名(`review-pull-request`)と一致。`description` に slapex / PR review / review comment 対応 / 対応結果の再確認 / github-op-integrated MCP-first をすべて含むことを確認。
- Claude Code の skill 一覧に `review-pull-request` が discover されることを確認。
- SKILL.md から `doc/guidelines/github-mcp-guidelines.md` の優先規則・操作別の第一選択表への参照を確認。
- 3 モードの判定・責務・完了条件の区別、PR 特定と停止条件、closed / merged 停止と draft の扱い、review 完了コメント必須、canonical metadata(キー名・順序・区切り固定)、日本語(常体)、capability 再利用条件、address-comments の非 resolve、verify-comments の read-back 後 resolve と対象限定、反復 2 周上限、`COMMENT` 限定と `APPROVE` / `REQUEST_CHANGES` 禁止、第一選択 tool 名の明記、write fallback 前の read-back を SKILL.md / references に記載したことを本文確認。
- `.config/github-op-integrated.conf.example` の現行 allowlist に必要 tool がすべて含まれ、追加不要であることを静的確認。
- gh preflight(`gh auth status` / `gh pr view`)が MCP 試行より先に置かれていないことを本文確認(明示的に禁止)。
- `find -L .claude/skills -maxdepth 1 -type l -print` — 出力なし(broken symlink なし)。
- `git diff --check` — pass。
- 実行時 MCP read 検証: `list_pull_requests(open)` は成功(現在 open PR は 0 件)。`pull_request_read(get_review_comments)` を merge 済み PR #160 に対して read-only で実行し、thread(`is_resolved` 等の metadata と comments)は取得できるが、現行 response に `PRRT_...` 形式の thread node ID が含まれないことを確認した(#161 の観測と同じ)。test comment の投稿や resolve は行っていない。
- skill / guideline 文書のみの変更のため package test は実施していない。

## リスク・ブロッカー

- 実 PR での review / reply / resolve の write 動作は、本 PR では read 検証のみ(無関係な既存 PR へ test comment を投稿しない)。write 経路の実地確認は今後の実運用で行う。
- 現行 GitHub MCP Server の `pull_request_read(get_review_comments)` response に thread node ID(`PRRT_...`)が含まれないため、`resolve_thread` 実行時は skill 記載のとおり別の MCP read method の確認 → guideline の fallback 規則の順で扱う必要がある。MCP server の version 更新で response に node ID が含まれるようになる可能性がある。

## セッションログ

- 2026-07-11: 依存 #161(PR #163)の merge を確認し、ブランチ作成。skill 実装を開始。
