# assets

このディレクトリには、リポジトリで使う静的アセットを置く。

## ロゴ

- `slapex-logo-shadow.svg`: README や紹介文書など、大きめに表示する場合の推奨ロゴ。影により白背景上でもアプリアイコンとしてのまとまりが出やすい。
- `slapex-logo.svg`: 小さめに表示する場合の推奨ロゴ。影を省き、外周をシングルラインにしているため、favicon、UI 部品、一覧表示などで潰れにくい。

どちらも SVG のまま保持する。PNG などの raster 画像は、必要な利用先が出た時点で生成する。

## スクリーンショット

`screenshots/` には README で使う出力プレビュー画像(`sample-*.png`)を置く。`doc/samples/` のサンプル export を撮影したもので、日本語版(`-ja`)と英語ページ準備用の英語版(`-en`)がある。再生成手順は `doc/samples/README.md` を参照。

`screenshots/output-dir-finder-ja.png` は README でローカル出力ディレクトリの構造を示す Finder 風画像である。

`screenshots/slack-app-setup/` には `doc/help/slack-app-setup.md` で使う Slack 管理画面 / Slack client の操作スクリーンショットを置く。実 Slack UI の手動撮影であり、スクリプトで再生成できない。更新方針(token などの秘密情報のマスクを含む)は `doc/help/README.md` の「スクリーンショットのメンテ方針」を参照。

`screenshots/` 配下の PNG は、コミット前に lossless 最適化(例: `oxipng -o max --strip safe`)を通す。

## デモ GIF

`demo/` には README で使うターミナル操作デモ(`slapex-demo-ja.gif`)を置く。`doc/samples/` と同じ架空 fixture を配信する fake server に対して実際の slapex を実行し、VHS(`tools/demo/`)で録画したもの。再録画手順は `doc/samples/README.md` を参照。
