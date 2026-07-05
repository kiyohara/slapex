# 作業ブランチメモ

- ブランチ: readme-preview-image-borders
- PR:
- 最終更新: 2026-07-05

## 目的

Issue #136: README の「出力プレビュー」から table レイアウト由来の枠線を除去し、HTML マークアップで preview 画像の周囲に枠線を付ける。PNG 自体は変更しない。

## 現在の状況

- `README.md` の 2 枚横並び `<table>` を、他セクションと同様の縦並び `<p align="center">` に変更。
- 枠線は GitHub sanitizer を通る `<img border="1">` で付与(`style` 属性は除去されるため未使用)。
- `doc/samples/README.md` に枠線が README HTML 側にある旨を追記。

## 決定事項

- 横並び維持より table 枠線除去と sanitizer 通過を優先し、縦並びレイアウトを採用。
- 枠線色の細かい指定は sanitizer 上困難なため、HTML `border` 属性のデフォルト表示を使う。

## 次にやること

- PR 作成、note rename、progress.md 更新。

## 検証

- [ ] GitHub README 表示で table 由来のセル枠線が無いこと
- [ ] preview 画像周囲に HTML 由来の枠線が表示されること
- [ ] screenshot PNG に変更が無いこと
- [ ] caption が読みにくくなっていないこと

## リスク・ブロッカー

- `border="1"` の見た目は GitHub テーマ依存。merge 前に GitHub 上で目視確認する。

## セッションログ

- 2026-07-05: Issue #136 を run-issue-task で開始。HTML `border` 属性方針で README / doc/samples/README.md を更新。
