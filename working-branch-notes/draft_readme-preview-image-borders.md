# 作業ブランチメモ

- ブランチ: readme-preview-image-borders
- PR:
- 最終更新: 2026-07-05

## 目的

Issue #136: README の「出力プレビュー」から table レイアウト由来の枠線を除去し、HTML マークアップで preview 画像の周囲に枠線を付ける。PNG 自体は変更しない。

## 現在の状況

- `README.md` の 2 枚横並び `<table>` を、他セクションと同様の縦並び `<p align="center">` に変更。
- 枠線は GitHub sanitizer を通る `<kbd>` で画像を囲んで付与(`img border` と `style` は除去される)。
- `doc/samples/README.md` に枠線が README HTML 側にある旨を追記。

## 決定事項

- 横並び維持より table 枠線除去と sanitizer 通過を優先し、縦並びレイアウトを採用。
- 枠線色の細かい指定は sanitizer 上困難なため、`<kbd>` 要素の GitHub 既定スタイルを使う。

## 次にやること

- PR 作成、note rename、progress.md 更新。

## 検証

- [x] GitHub README 表示で table 由来のセル枠線が無いこと — `<table>` を除去し、preview 領域に table 要素が無いことを DOM で確認。
- [x] preview 画像周囲に HTML 由来の枠線が表示されること — `<kbd>` ラッパーが `border: 1px solid rgba(209, 217, 224, 0.7)` で残ることを GitHub preview で確認。初版の `img border="1"` は sanitizer で除去されたため `<kbd>` に変更。
- [x] screenshot PNG に変更が無いこと — `git diff main...HEAD` に PNG 変更なし。
- [x] caption が読みにくくなっていないこと — `<sub>` caption は従来どおり画像直下に配置。

## リスク・ブロッカー

- `border="1"` の見た目は GitHub テーマ依存。merge 前に GitHub 上で目視確認する。

## セッションログ

- 2026-07-05: sanitizer 検証で `img border="1"` が除去されることを確認。`<kbd>` ラッパーへ変更し GitHub preview で枠線表示を確認。
