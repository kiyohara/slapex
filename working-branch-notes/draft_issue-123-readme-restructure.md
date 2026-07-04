# 作業ブランチメモ

- ブランチ: issue-123-readme-restructure
- PR: -
- 最終更新: 2026-07-04

## 目的

Issue #123。README をキャッチーな入口 + 誘導リンク集へ再構成し、インストール・使い方の詳細を `doc/help/` へ移設する。利用者向け情報と開発者向け情報の層を分離する。

## 現在の状況

- 実装・検証完了。PR 作成待ち。
- `doc/help/installation.md`(Homebrew / install script / 手動 + checksum 検証)と `doc/help/usage.md`(`--demo` / 実行基本形 / 対話選択 / option 表 / cache / stdout・stderr / 出力構造)を新設し、README の詳細を移設。
- README はキャッチー部(ロゴ・一言説明・特徴・出力プレビュー)→ クイックスタート誘導 → 利用方法の詳細(リンク集)→ 開発者向けリンク(2 行)→ ライセンスの構成へ再構成。「開発」節のコマンド例は削除(開発者は `doc/README.md` / `AGENTS.md` から到達)。
- README 本文の開発者向けリンク(`doc/design/usage-flow.md` / `cli-interface.md` / `cache.md` / `output-format.md`)を除去し、移設先 help の文末脚注(開発者向け明示)へ置換。`doc/help/token-injection.md` の「関連」に残っていた design 直リンクも脚注化(#129 方針の棚卸し対象)。
- `doc/README.md` / `doc/help/README.md` の配置説明に「利用者向け詳細は doc/help、README は入口」の分担を追記。quickstart の README アンカー参照(#インストール / #使い方)を新設 help へ差し替え。

## 決定事項

- 移設先ファイル名は Issue の例示どおり `doc/help/installation.md` / `doc/help/usage.md` とする。
- README 本文から開発者向けドキュメント(`doc/design/` spec、decision log)への直接リンクを除去する。移設先の help で仕様正本への参照が必要な箇所は、faq.md と同じ「文末脚注 + 開発者向け明示」方式にする(#129 / decision log 0049 の方針)。
- 文体は document-style-guidelines に従い、利用者向け(README / doc/help/)はですます調・簡潔・中立とする(#125 / decision log 0048)。
- quickstart 本体の内容は変更しない(#49 スコープ)。ただし README アンカーへのリンク(#インストール / #使い方)は移設先へ差し替える(Issue 本文で明示されたリンク整合の範囲)。

## 次にやること

- (PR 作成後)レビュー対応。

## 検証

- 変更対象文書(README / doc/help 配下 / doc/README.md)の相対リンクの解決を script で全数確認: ALL LINKS OK。
- fragment 付きリンク(見出しアンカー)の存在確認を script で実施: ALL ANCHORS OK(`usage.md#主要な-option`、`usage.md#token-なしで試す--demo`、`README.md#出力プレビュー` などを含む)。
- 旧 README アンカー(#インストール / #使い方 など)への参照が working-branch-notes 以外に残っていないことを rg で確認。
- README 本文に `doc/design` / decision log への直リンクが残っていないことを rg で確認。
- 旧 README の「事前準備」節にあった token 種別・scope・アクセス範囲の説明が `doc/help/slack-app-setup.md` に正本として存在することを確認(情報ロスなし)。
- コード変更なしのため go test / build は未実施。

## リスク・ブロッカー

- README のアンカー(#インストール / #使い方 / #出力)への外部リンクが GitHub 上の古い参照(過去 PR 等)に残るが、リポジトリ内の現行文書の整合のみをスコープとする。

## セッションログ

- 2026-07-04: Issue #123 / コメント 2 件(文体ガイドライン #125、リンク方針 #129)を確認。依存 #49 / #52 / #125 / #129 の完了を確認し、作業開始。
