# 作業ブランチメモ

- ブランチ: issue-144-split-token-e2e-help
- PR: #146
- 最終更新: 2026-07-05

## 目的

`doc/help/slack-app-setup.md` から開発者・メンテナ向けの実 token E2E 確認計画を外し、必要な内容を開発者向け文書へ移す。

## 現在の状況

- Issue #144 を確認済み。コメント、sub-issues はなし。
- PR #143 は merge 済みで、`main` を `origin/main` に fast-forward 済み。
- 配置方針として `doc/README.md` と `doc/help/README.md` を確認済み。
- `doc/help/slack-app-setup.md` から実 token E2E の節を削除し、`doc/guidelines/credential-scope-guidelines.md` へ常体で移設済み。

## 決定事項

- 実 token E2E の確認項目は利用者向け help ではなく、認証情報の扱いに近い開発者向けルールとして `doc/guidelines/credential-scope-guidelines.md` に移す。
- `progress.md` は Issue #144 のスコープ外なので更新しない。

## 次にやること

- PR description の note 参照を更新し、採番 commit を push する。

## 検証

- `sed -n '280,340p' doc/help/slack-app-setup.md`: `## 実 token E2E の確認計画` 節が消え、利用者向け help が参考リンクへ直接つながることを確認。
- `sed -n '1,160p' doc/guidelines/credential-scope-guidelines.md`: 移設先が開発者向け常体であることを確認。
- `rg -n "## 実 token E2E の確認計画|リリース前に、実 token|PR や working branch note" doc/help/slack-app-setup.md`: 該当なし。
- `git diff --check`: 成功。
- `rg -n "password|secret|token|cookie|session|PRIVATE KEY|xox[abp]-|https?://[^ )]+[?][^ )]+" working-branch-notes/146_issue-144-split-token-e2e-help.md`: 用語としての `token` のみ検出。実値や署名付き URL はなし。

## リスク・ブロッカー

現時点ではなし。

## セッションログ

- 2026-07-05: Issue #144 を開始。`main` を PR #143 merge 後の `origin/main` に合わせ、作業ブランチを作成した。
