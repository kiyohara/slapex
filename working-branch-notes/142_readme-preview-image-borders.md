# 作業ブランチメモ

- ブランチ: readme-preview-image-borders
- PR: #142
- 最終更新: 2026-07-05

## 目的

Issue #136: README の「出力プレビュー」から table レイアウト由来の枠線を除去し、HTML マークアップで preview 画像の周囲に枠線を付ける。PNG 自体は変更しない。

## 現在の状況

- `README.md` の 2 枚横並び `<table>` を縦並び `<p align="center">` + `<kbd><img ...></kbd>` へ変更済み。
- `doc/samples/README.md` に枠線が README HTML 側にある旨を追記済み。
- 実装・検証・PR 作成(#142)・note rename・progress.md 更新まで完了。merge 待ち。

## 決定事項

- 横並び維持より table 枠線除去と sanitizer 通過を優先し、縦並びレイアウトを採用。
- 枠線色の細かい指定は sanitizer 上困難なため、`<kbd>` 要素の GitHub 既定スタイルを使う。

## 次にやること

- 特になし(merge 待ち)。

## 検証

- [x] GitHub README 表示で table 由来のセル枠線が無いこと — `<table>` を除去し、preview 領域に table 要素が無いことを DOM で確認。
- [x] preview 画像周囲に HTML 由来の枠線が表示されること — `<kbd>` ラッパーが `border: 1px solid rgba(209, 217, 224, 0.7)` で残ることを GitHub preview で確認。初版の `img border="1"` は sanitizer で除去されたため `<kbd>` に変更。
- [x] screenshot PNG に変更が無いこと — `git diff main...HEAD` に PNG 変更なし。
- [x] caption が読みにくくなっていないこと — `<sub>` caption は従来どおり画像直下に配置。

## リスク・ブロッカー

- `<kbd>` 枠線の見た目は GitHub テーマ依存。GitHub preview 上で目視確認済み。未解決のブロッカーは無い。

## セッションログ

- 2026-07-05: sanitizer 検証で `img border="1"` が除去されることを確認。`<kbd>` ラッパーへ変更し GitHub preview で枠線表示を確認。
- 2026-07-05: review 指摘に従い note の「次にやること」「リスク・ブロッカー」を最終状態へ更新。
