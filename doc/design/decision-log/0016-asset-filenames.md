# 0016 asset ファイル名

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-03
- 関連: `../output-format.md`, `../slack_posts_dumper`

## 背景

Slack 投稿を静的 HTML と assets 一式として保存する際、asset のローカルファイル名をどのように決めるかを決める必要がある。

関連評価実装である `slack_posts_dumper` には、asset URL を hash 化してファイル名にする PoC 実装がある。本リポジトリでは、その実装を参考にしつつ、出力内容を利用者が理解しやすい構造にする必要がある。

## 候補

- PoC と同じ URL hash ベースのファイル名にする。
- Slack file ID、emoji 名、連番などの ID ベースのファイル名にする。
- 元の添付ファイル名や emoji 名など、人間が読みやすい名前を優先する。

## 検討内容

URL hash ベースのファイル名は、同じ元 URL を同じローカルファイルへ安定して対応付けられる。標準絵文字、カスタム絵文字、URL preview 画像、アップロード画像、添付ファイルのように取得元の種類が異なっても、URL を基準に共通の命名規則を適用できる。

ID ベースのファイル名は Slack API 上の ID と対応しやすいが、標準絵文字や URL preview 画像のように Slack file ID を持たない asset では別ルールが必要になる。

人間が読みやすい名前は出力 directory を直接見たときに理解しやすいが、重複、文字種、長さ、同名衝突、secret や個人情報に近い文字列の混入に注意が必要になる。

## 決定

asset ファイル名は PoC と同じ URL hash ベースにする。

ファイル名は `<url-hash>.<ext>` を基本形とする。元 URL が同じ asset は同じファイル名へ解決する。

保存先は利用者が把握しやすいように、asset 種別ごとの分類ディレクトリに分ける。

絵文字画像は `assets/emoji/` に集約する。標準絵文字は原則として Unicode に戻して HTML に直接表示し、Unicode fallback できないカスタム絵文字を画像 asset として `assets/emoji/` 配下に保存する。利用者にとって custom かどうかは重要な分類ではないため、`assets/custom-emoji/` は設けない。

元 URL、Slack file ID、emoji 名、元の表示ファイル名、content type、取得成否などの情報は `.cache/assets_manifest.json` と HTML 側の表示に保持する。

## 理由

PoC と同じ方式を採用することで、既存の評価実装から実装知見を引き継ぎやすい。

URL hash ベースであれば、同じ URL の重複 download と重複保存を避けやすく、asset 種別をまたいでも一貫した命名規則を適用できる。

人間向けの情報はファイル名へ詰め込まず、manifest と HTML 表示に分けて保持する方が、衝突回避と情報管理の面で扱いやすい。

## 影響

HTML 生成時は、Slack API から得た asset URL を URL hash に変換し、分類ディレクトリ配下の相対 path として参照する。

`.cache/assets_manifest.json` は、URL hash と元 URL、ローカル path、asset 種別、取得成否を対応付ける中核的な中間情報になる。

添付ファイルの元の表示ファイル名は、保存ファイル名ではなく HTML のリンク表示や manifest metadata として扱う。

カスタム絵文字か標準絵文字かの判定結果は、必要であれば manifest metadata として保持する。保存 directory は `assets/emoji/` に統一する。

## 後から見直す条件

URL に短時間で失効する署名付き query parameter が含まれ、同じ実体の asset が実行ごとに別 hash になる問題が大きい場合は、hash 対象の正規化ルールを検討する。

利用者が assets directory を直接読む用途が強くなり、人間可読なファイル名の価値が衝突リスクを上回る場合は、表示名ベースの alias や追加 index の導入を検討する。
