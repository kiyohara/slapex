# 作業ブランチメモ

- ブランチ: `review-pull-request-single-completion-comment`
- PR: #174
- 最終更新: 2026-07-13

## 目的

Issue #173 に従い、`review-pull-request` の `review` モードで完了要約が review body と PR conversation comment に重複しないよう、投稿先を指摘の有無で一意にする。

## 現在の状況

正本 skill の `SKILL.md` と `references/review.md` を単一チャネル方針へ更新し、指定検証を完了した。PR 作成を進める。

## 決定事項

- 指摘ありでは、inline comment と `submit_pending` の review body を使い、完了要約の `add_issue_comment` は投稿しない。
- 指摘なしでは、pending review を作らず、`add_issue_comment` だけで完了を可視化する。
- 完了要約は 1 review cycle につき 1 本とし、inline の詳細は複製しない。

## 次にやること

- commit、push、PR 作成後に note を採番する。

## 検証

- `rg -n 'add_issue_comment|完了コメント|submit_pending|指摘の有無にかかわらず|完了要約|重複投稿|全文コピー' .agents/skills/review-pull-request/`: pass。旧「指摘の有無にかかわらず必ず add_issue_comment」は残っておらず、単一チャネル、重複禁止、全文コピー禁止を確認した。
- `rg -n '^\\| `review`|^完了条件:' .agents/skills/review-pull-request/SKILL.md .agents/skills/review-pull-request/references/review.md`: pass。完了条件が一致することを確認した。
- `ls -la .claude/skills/review-pull-request` と `test -f .claude/skills/review-pull-request/SKILL.md`: pass。symlink が正本を指し、`SKILL.md` を読めることを確認した。
- `git diff --check`: pass。
- package test: 未実施。文書 / skill のみの変更であり、Issue の検証指示に従い不要と判断した。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-07-13: Issue #173 の state、依存、コメント、sub-issue、関連 PR を確認。依存なし、関連 PR なし。
- 2026-07-13: review 完了要約を指摘ありでは review body、指摘なしでは PR conversation comment の 1 本に揃え、指定検証を完了。
