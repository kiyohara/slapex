# 作業ブランチメモ

- ブランチ: add-pr-review-skill
- PR: #164
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
- 可視 metadata の canonical フォーマットは `Agent` / `Model` / `Review cycle` / `Reviewed head` / `Mode` の 5 キー・この順序・`: ` 区切り・1 行 1 キーで SKILL.md に確定する。
- review cycle ID の時刻部は秒まで(`YYYYMMDDHHMMSS`、UTC)とする。分単位では同一 Agent・同一 head の再試行で ID が衝突し得るという Codex review の指摘(PR #164)を採用した。
- canonical metadata に `Model` キーを追加し 5 キーとする(ユーザー要望)。利用 model の記録が目的の参考情報であり、cycle 突合や担当一致判定には使わない。`Agent` 値へ埋め込まず独立キーにしたのは、同一 Agent 種別でも session により model が変わり得て、`Agent` の等値比較を壊さないため。確認できない場合は `unknown` とし推測しない。
- `address-comments` / `verify-comments` の返信では、元 review の review cycle ID をそのまま使い、新しい ID を作らない(cycle 突合のため)。
- review 完了コメント・再確認結果は `add_issue_comment`(PR conversation comment)へ一本化する。
- inline thread の resolve は skill から自動実行せず、人間が GitHub UI で行う(ユーザー決定)。理由: resolve の実体である GraphQL mutation `resolveReviewThread` は REST に対応 endpoint が無く、fine-grained PAT では Pull requests に加えて `Contents: Read and Write` を要求する(公式 doc に記載なし、community Discussion #44650 で確認)。本プロジェクトは Contents: write を付与せず、`gh` も fine-grained PAT を使うため fallback も不可。
- `verify-comments` の確認済み返信は先頭行 `**修正確認済み(resolve 可)**` を canonical な resolve 可マーカーとし、再確認結果コメントに resolve 可 thread の URL 一覧を含める。人間はこれを起点に手動 resolve する。
- AGENTS.md への skill 名追記は行わない(自動 discover のため必須でないと Issue に明記)。
- `progress.md` の索引に #162 の行は無いため、更新しない(単発 Issue は無理に登録しない)。

## 次にやること

- PR #164 のレビュー対応と merge 待ち(merge はユーザー)。

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

- review 投稿・inline 返信・conversation comment の write 経路は、本 PR 自体のレビュー運用で実地確認済み。thread resolve は fine-grained PAT の権限不足で不可と判明し、人間の手動操作へ変更した(決定事項参照)。
- 将来、自動 resolve を再導入する場合は、`resolveReviewThread` が fine-grained PAT で `Contents: Read and Write` を要求する点と、`pull_request_read(get_review_comments)` response に thread node ID(`PRRT_...`)が含まれない点の両方を再確認する必要がある。
- `doc/guidelines/github-mcp-guidelines.md` の操作表は「Review thread の解決 = `pull_request_review_write(resolve_thread)`」のままであり、本 skill の手動 resolve 方針との差分がある。guideline 側の更新は本 Issue のスコープ外のため、別タスクとして扱う。

## セッションログ

- 2026-07-11: 依存 #161(PR #163)の merge を確認し、ブランチ作成。skill 実装を開始。
- 2026-07-11: skill 実装と検証を完了し、PR #164 を作成。note を採番 rename。
- 2026-07-11: Codex review の指摘(review cycle ID の分単位衝突)を採用し、時刻部を `YYYYMMDDHHMMSS` へ変更。Cursor review は指摘なし。
- 2026-07-12: ユーザー要望により canonical metadata へ `Model` キーを追加(記録目的、突合には不使用)。
- 2026-07-12: Codex verify-comments の指摘(`Model` 追加時の「次の 4 行」導入文と本 note 旧決定事項の 5 キー不整合)を採用し修正。同一 cycle の address-comments 2 周目(上限)。
- 2026-07-12: 別 Agent の verify-comments で thread resolve が fine-grained PAT の権限エラーとなった。調査の結果、`resolveReviewThread` は fine-grained PAT で `Contents: Read and Write` を要求すると判明(公式 doc 記載なし、community Discussion #44650)。`gh` も fine-grained PAT のため fallback 不可。resolve を人間の手動操作へ変更し、resolve 可マーカーを canonical 化した。
