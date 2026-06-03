# 作業ブランチメモ

- ブランチ: workspace-target-labels
- PR:
- 最終更新: 2026-06-03

## 目的

`SLACK_BOT_TOKEN` から解決した workspace が利用者の意図と異なる場合に気づきやすくするため、実行中と生成物に workspace / channel の人間向け表示を出す方針を文書化する。

## 現在の状況

- `usage-flow.md` に処理対象 workspace / channel の表示方針を追加済み。
- `doc/design/decision-log/0020-target-label-display.md` を追加し、index に追記済み。

## 決定事項

- workspace を必須入力に戻すのではなく、token から解決した対象情報を表示して利用者が確認できるようにする。
- directory 用 label と画面表示用 label は役割を分ける。

## 次にやること

- 差分を最終確認する。

## 検証

- `git diff` で文書差分を確認。
- working branch note に secret 実値が入っていないことを目視確認。
- `team_id` は初期対象外にした `--team-id` option / org-wide install 対応とは別に、Slack API が返す workspace ID として表示・metadata・fallback に使いうる情報であることを確認。ただし、表示に含めるかは利用者向け label の冗長性との trade-off として再検討余地がある。

## リスク・ブロッカー

- 表示は誤認を減らす施策であり、CI などで mismatch を強制停止する guard ではない。

## セッションログ

- 2026-06-03: `workspace-target-labels` ブランチを作成し、作業メモを追加。
- 2026-06-03: `usage-flow.md`、decision log、decision-log index を更新。
- 2026-06-03: `team_id` の扱いについて、`--team-id` option を初期対象外にした判断と、workspace ID として表示・記録しうる判断を切り分けて確認。
