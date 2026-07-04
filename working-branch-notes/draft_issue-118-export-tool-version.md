# 作業ブランチメモ

- ブランチ: issue-118-export-tool-version
- PR:
- 最終更新: 2026-07-04

## 目的

Issue #118: 出力 HTML の footer `Export information` に、生成に使った slapex のバージョンを控えめに追記する。

## 現在の状況

依存なし。`progress.md` 索引外の単発 Issue。作業開始。

## 決定事項

- `PageData.ToolLine` に `slapex <version>` 形式(`--version` 出力と揃える)で渡す。
- Export information の `<dl>` に `Tool` 行を追加する。

## 次にやること

- render / export / テストを更新し、検証する。

## 検証

(未実施)

## リスク・ブロッカー

- なし

## セッションログ

- 2026-07-04: Issue #118 の依存確認を実施し、作業ブランチを作成した。
