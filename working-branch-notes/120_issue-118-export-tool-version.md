# 作業ブランチメモ

- ブランチ: issue-118-export-tool-version
- PR: #120
- 最終更新: 2026-07-04

## 目的

Issue #118: 出力 HTML の footer `Export information` に、生成に使った slapex のバージョンを控えめに追記する。

## 現在の状況

実装完了。PR #120 作成済み。

## 決定事項

- `PageData.ToolLine` に `slapex <version>` 形式(`--version` 出力と揃える)で渡す。
- Export information の `<dl>` に `Tool` 行を追加する。
- レビュー対応: `tools/gensample` の `ToolVersion` を `"gensample"` → `"dev"` に変更(footer が `slapex dev` になり `slapex <version>` の意図に合う)。
- レビュー対応: テンプレート変更に合わせて `doc/samples/{ja,en}` を `gensample` で再生成し PR に含める。本文の投稿日時・テキスト・アバター見た目は不変で、変わるのは footer の `Exported` 時刻・`Tool` 行・アセットハッシュ名のみ。スクリーンショットは折りたたみ footer を含まない timeline / thread 部分の撮影のため撮り直し不要。

## 次にやること

- merge 待ち。

## 検証

- `docker compose run --rm --no-deps dev go test ./...` 成功
- `docker compose run --rm --no-deps dev go vet ./...` 成功
- `docker compose run --rm --no-deps dev go build ./...` 成功

## リスク・ブロッカー

- なし

## セッションログ

- 2026-07-04: Issue #118 の依存確認を実施し、作業ブランチを作成した。
- 2026-07-04: footer Export information に Tool 行を追加し、PR #120 を作成した。
