# 作業ブランチメモ

- ブランチ: issue-145-simplify-slack-setup-help
- PR: 未作成
- 最終更新: 2026-07-05

## 目的

`doc/help/slack-app-setup.md` に集まっている token 渡し方とエラー対処の詳細を整理し、token の渡し方は `doc/help/token-injection.md`、入口表は `doc/help/faq.md` を正本として参照する形にする。

## 現在の状況

- Issue #145 を確認済み。コメント、sub-issues はなし。
- `main` は PR #146 merge 後の `origin/main` に fast-forward 済み。
- `doc/README.md`、`doc/help/README.md`、文体 guideline、対象 help 3 ファイルを確認済み。
- `slack-app-setup.md` の token 渡し方とエラー対処を短縮し、`faq.md` の入口表を調整済み。`token-injection.md` は既存内容で正本として不足なしと判断した。

## 決定事項

- `slack-app-setup.md` の `Token の渡し方` は短い誘導にする。
- `slack-app-setup.md` の `よくあるエラー` は setup 固有の scope / channel / bot 参加へ寄せる。
- `faq.md` の「うまくいかないとき」を入口表として、token 未設定・無効は `token-injection.md`、scope / channel は `slack-app-setup.md` へ誘導する。
- Issue のスコープ外指定に従い、`progress.md` は更新しない。

## 次にやること

- PR を作成し、採番後に note を rename する。

## 検証

- `sed -n '250,340p' doc/help/slack-app-setup.md`: `Token の渡し方` が短い誘導になり、`よくあるエラー` が scope / channel / bot 参加へ絞られていることを確認。
- `sed -n '60,78p' doc/help/faq.md`: 「うまくいかないとき」の入口表が token 未設定 / token 無効・権限不足 / channel access を詳細正本へ誘導していることを確認。
- `rg -n 'SLACK_TOKEN=|op run -- slapex|secrets\\.SLACK_TOKEN|Paste a Slack OAuth token|Enter SLACK_TOKEN|```yaml|```sh' doc/help/slack-app-setup.md`: 該当なし。prompt 例、`op run` 例、CI secrets 例が Slack App setup 側から消えていることを確認。
- `ls doc/help/token-injection.md doc/help/faq.md doc/help/slack-app-setup.md`: リンク先ファイルの存在を確認。
- `rg -n '^(##|###) ' doc/help/slack-app-setup.md doc/help/token-injection.md doc/help/faq.md`: 関連 anchor の見出しが存在することを確認。
- `git diff --check`: 成功。

## リスク・ブロッカー

現時点ではなし。

## セッションログ

- 2026-07-05: Issue #145 を開始。`main` を最新化し、作業ブランチを作成した。
