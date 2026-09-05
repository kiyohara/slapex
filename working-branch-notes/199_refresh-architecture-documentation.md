# 作業ブランチメモ

- ブランチ: `refresh-architecture-documentation`
- PR: #199
- 最終更新: 2026-09-05

## 目的

Issue #195 に従い、現行 package の責務・依存方向・開発/配布入口を architecture.md に反映する。cache の仕様差は履歴と突合し、挙動を変更せず整理する。

## 現在の状況

- 必須依存 #188 の PR #197、推奨順の先行 #196 の PR #198 は merge 済み。
- main を最新化し、Issue 指定のブランチを作成した。
- architecture.md を現行の 9 package の責務・直接依存と生成用入口へ更新した。Go version/API 件数の重複を減らし、Compose・CI・release の実装済み構成を参照可能にした。
- cache.md に仕様と実装の差を独立した節として記録した。

## 決定事項

- 過去 decision log は変更せず、現行文書と実装の差を区別する。
- `local_path` は仕様確定 commit `21a360c` で未保存時 `null`、PoC commit `eb9da63` から実装は `omitempty`。明示的な仕様変更の根拠がないため、仕様表を実装へ合わせず差異として報告する。

## 次にやること

- PR のレビュー・merge はユーザーが行う。

## 検証

- `git diff --check`: 成功。
- architecture.md / cache.md の Markdown 相対リンク先の存在を確認。追加 anchor は対象の見出しと照合した。
- production Go source の import を抽出し、cmd/slapex と internal の全 9 package の直接依存が表と一致することを確認。
- go.mod、compose.yaml、CI/release workflow、.goreleaser.yaml、slack-api-usage.md と突合。API 件数は列挙せず spec へ参照を集約した。
- 将来形の残存箇所を確認。PoC・Compose・release の古い予定は現況へ更新。段階的リファクタリング、cache の open-ended range、外部 reader を含む仕様差判断は将来の事項として維持。
- note の情報統制チェック: 秘密情報・個人情報・認証付き URL・ログ全文なし。
- 文書のみのため Issue 指定どおり Go tests は未実施。CLI / cache / HTML のコードと過去 decision log は変更していない。

## リスク・ブロッカー

- 未保存 asset の `local_path` を `null` とするか省略とするかは別途判断が必要。本 Issue では仕様・出力の統一は行わない。

## セッションログ

- 2026-09-05: Issue 本文・comments・sub-issues・labels と依存 PR を確認。追加指示・子 Issue・label はなし。
