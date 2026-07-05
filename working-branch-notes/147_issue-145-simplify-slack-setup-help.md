# 作業ブランチメモ

- ブランチ: issue-145-simplify-slack-setup-help
- PR: #147
- 最終更新: 2026-07-05

## 目的

`doc/help/slack-app-setup.md` に集まっている token 渡し方とエラー対処の詳細を整理し、token の渡し方は `doc/help/token-injection.md`、入口表は `doc/help/faq.md` を正本として参照する形にする。

## 現在の状況

- Issue #145 を確認済み。コメント、sub-issues はなし。
- `main` は PR #146 merge 後の `origin/main` に fast-forward 済み。
- `doc/README.md`、`doc/help/README.md`、文体 guideline、対象 help 3 ファイルを確認済み。
- `slack-app-setup.md` の token 渡し方とエラー対処を短縮し、`faq.md` の入口表を調整済み。`token-injection.md` は既存内容で正本として不足なしと判断した。
- Cursor Bugbot の指摘を受け、無効 token の診断・対処を `token-injection.md` に補足する方針へ変更した。

## 決定事項

- `slack-app-setup.md` の `Token の渡し方` は短い誘導にする。
- `slack-app-setup.md` の `よくあるエラー` は setup 固有の scope / channel / bot 参加へ寄せる。
- `faq.md` の「うまくいかないとき」を入口表として、token 未設定・無効は `token-injection.md`、scope / channel は `slack-app-setup.md` へ誘導する。
- Issue のスコープ外指定に従い、`progress.md` は更新しない。

## 次にやること

- PR description の note 参照を更新し、採番 commit を push する。

## 検証

- `sed -n '250,340p' doc/help/slack-app-setup.md`: `Token の渡し方` が短い誘導になり、`よくあるエラー` が scope / channel / bot 参加へ絞られていることを確認。
- `sed -n '60,78p' doc/help/faq.md`: 「うまくいかないとき」の入口表が token 未設定 / token 無効・権限不足 / channel access を詳細正本へ誘導していることを確認。
- `rg -n 'SLACK_TOKEN=|op run -- slapex|secrets\\.SLACK_TOKEN|Paste a Slack OAuth token|Enter SLACK_TOKEN|```yaml|```sh' doc/help/slack-app-setup.md`: 該当なし。prompt 例、`op run` 例、CI secrets 例が Slack App setup 側から消えていることを確認。
- `ls doc/help/token-injection.md doc/help/faq.md doc/help/slack-app-setup.md`: リンク先ファイルの存在を確認。
- `rg -n '^(##|###) ' doc/help/slack-app-setup.md doc/help/token-injection.md doc/help/faq.md`: 関連 anchor の見出しが存在することを確認。
- `git diff --check`: 成功。
- `rg -n "password|secret|token|cookie|session|PRIVATE KEY|xox[abp]-|https?://[^ )]+[?][^ )]+" working-branch-notes/147_issue-145-simplify-slack-setup-help.md`: 用語としての `token` のみ検出。実値や署名付き URL はなし。

レビュー対応後:

- `sed -n '82,135p' doc/help/token-injection.md`: `token が無効なとき` 節で保存値確認、App uninstall / token revoke / scope 変更後の再発行、利用中の方法への反映を案内していることを確認。
- `sed -n '65,72p' doc/help/faq.md`: 無効 token / 権限不足の行が `token が無効なとき` と Slack App setup の再 install 手順へ誘導していることを確認。
- `rg -n '^(##|###) (token が無効なとき|scope 変更後の再 install)' doc/help/token-injection.md doc/help/slack-app-setup.md`: 関連 anchor の見出しが存在することを確認。
- `git diff --check`: 成功。

## リスク・ブロッカー

現時点ではなし。

## セッションログ

- 2026-07-05: Issue #145 を開始。`main` を最新化し、作業ブランチを作成した。
- 2026-07-05: Cursor Bugbot の「無効 token 対処手順の欠落」指摘を確認。`token-injection.md` に `token が無効なとき` 節を追加し、FAQ の入口表から直接リンクする対応を開始した。
