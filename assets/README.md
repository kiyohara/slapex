# assets

このディレクトリには、リポジトリで使う静的アセットを置く。

## ロゴ

- `slapex-logo-shadow.svg`: README や紹介文書など、大きめに表示する場合の推奨ロゴ。影により白背景上でもアプリアイコンとしてのまとまりが出やすい。
- `slapex-logo.svg`: 小さめに表示する場合の推奨ロゴ。影を省き、外周をシングルラインにしているため、favicon、UI 部品、一覧表示などで潰れにくい。

どちらも SVG のまま保持する。PNG などの raster 画像は、必要な利用先が出た時点で生成する。

## スクリーンショット

`screenshots/` には README で使う出力プレビュー画像(`sample-*.png`)を置く。`doc/samples/` のサンプル export を撮影したもので、日本語版(`-ja`)と英語ページ準備用の英語版(`-en`)がある。再生成手順は `doc/samples/README.md` を参照。
