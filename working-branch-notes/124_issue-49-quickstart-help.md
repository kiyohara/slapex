# 作業ブランチメモ

- ブランチ: issue-49-quickstart-help
- PR: #124
- 最終更新: 2026-07-04

## 目的

Issue #49 の初回利用クイックスタートガイド(チェックリスト形式)を `doc/help/quickstart.md` として新設し、README の「使い始めるまでの流れ」から誘導する。初回利用者が README と design doc を行き来せず 1 ページで完走できるようにする。

## 現在の状況

- `doc/help/quickstart.md` を新設し、README の「使い始めるまでの流れ」直前に quickstart への誘導を追加済み。
- README 再構成の新 Issue #123 を起票し、`progress.md` に help-05 として登録済み(依存: #49 / #52)。
- #52 に「FAQ 完成時に quickstart のトラブル分岐リンクを差し替える」引き継ぎコメントを追加済み。

## 決定事項

- quickstart は「前提 → インストール → Slack App / token 準備 → 初回 export → 出力確認」のチェックリスト形式とし、各ステップに所要時間目安と完了確認方法を付ける。
- 詳細手順は複製せず、既存の正本(`doc/help/slack-app-setup.md` / `doc/help/token-injection.md` / README のインストール節)へリンクする。
- 典型トラブルの分岐リンクは、現時点で存在する正本(`slack-app-setup.md` の「よくあるエラー」、`cli-interface.md` の exit code)へ張る。制限事項・FAQ help(#52)が完成したら、#52 側の作業でリンクを差し替える。
- README の変更は「使い始めるまでの流れ」から quickstart への誘導追加に留める(README 全面再構成は新 Issue)。
- README 再構成の新 Issue を起票し、`progress.md` に help-05 として登録する。依存は #49 / #52。
- progress.md の順序メモでは help-04 は help-03(#52)後が望ましいとされていたが、順序理由はトラブルリンク先(FAQ)の有無だけであり、リンク差し替えを #52 側に含める前提で help-04 を先行する(ユーザー了承済み)。
- ユーザー要望により、有名 CLI ツール(gh / Stripe CLI / Slack CLI / 1Password CLI / Terraform / flyctl / minikube)の quickstart 事例を調査し、共通パターンを quickstart に反映した。調査の詳細は補助 note `124_issue-49-quickstart-help__cli-quickstart-research.md` を参照。反映点: (1) 完了確認への期待出力例の追加(`--version`、初回 export の完了 summary)、(2) `--demo` を独立した任意ステップへ昇格、(3) 末尾に「次のステップ」セクションを追加。

## 次にやること

- PR レビュー対応。merge はユーザーが行う。
- #52 実施時: quickstart「つまずいたら」のリンク先を FAQ へ差し替える(#52 コメントで引き継ぎ済み)。

## 検証

- quickstart 内の相対リンク先ファイルの存在を確認: `../../README.md` / `../design/cli-interface.md` / `slack-app-setup.md` / `token-injection.md` すべて存在。
- リンクの anchor(README の `#出力プレビュー` / `#インストール` / `#使い方`、slack-app-setup.md の `#推奨手順-user-token-用-app-を-manifest-から作成する` / `#bot-token-を使う場合` / `#bot--app-を-channel-に参加させる` / `#channel-access` / `#よくあるエラー`)を見出し一覧と突き合わせて確認。
- 記載内容(exit code 2 / 3 の意味、候補 11 件以上の挙動、token 対話入力、`--demo`、`/invite @slapex`)を `doc/design/cli-interface.md` / `doc/design/usage-flow.md` / `doc/help/slack-app-setup.md` の正本と突き合わせて確認。
- ドキュメントのみの変更のため、Go の build / test は実施していない(コード変更なし)。
- 事例調査反映後: `--version` の期待出力例を実装(`main.go` の `slapex <version>`、goreleaser の `{{ .Version }}` は tag の `v` なし)と、完了 summary の例を `doc/design/usage-flow.md` の確定仕様の表示例と突き合わせて確認。追加リンク(`#使い方` anchor 含む)の存在も確認。

## リスク・ブロッカー

- #52(制限事項・FAQ)完了時に quickstart 内のトラブル分岐リンクを FAQ へ差し替える必要がある。#52 へのコメントで引き継ぐ。

## セッションログ

- 2026-07-04: Issue #49 読解、方針総評をユーザーへ提示。README 再構成は別 Issue 切り出しで進める方針を確定。ブランチと note を作成。
- 2026-07-04: quickstart.md 作成、README へ誘導追加、リンク検証。Issue #123(README 再構成)起票、#52 へ引き継ぎコメント、progress.md 更新(help-04 done / help-05 追加)。
- 2026-07-04: 有名 CLI の quickstart 事例調査(補助 note 参照)を実施し、期待出力例・`--demo` 任意ステップ・「次のステップ」セクションを quickstart に反映。
