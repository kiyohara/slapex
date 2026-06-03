# 0017 uploaded image assets

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../output-format.md`, `../html-rendering.md`, `0010-attachment-file-downloads.md`, `0016-asset-filenames.md`

## 背景

ユーザーが Slack にアップロードした画像について、thumbnail / preview / original のどれを保存するかを決める必要があった。

当初は Slack 上の見た目を再現する目的から、original ではなく thumbnail / preview 相当だけを保存する案を検討した。一方で、画像以外の添付ファイルも可能な限り保存する方針を採用したため、画像だけ original を保存しないと、添付ファイル保存方針との一貫性が弱くなる。

## 候補

- thumbnail だけを保存し、HTML の inline 表示に使う。
- original だけを保存し、HTML の inline 表示にも使う。
- thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開けるようにする。

## 検討内容

thumbnail だけを保存する案は出力サイズを抑えやすく、Slack 上の見た目再現には十分な場合が多い。ただし、スクリーンショット、図表、文字の多い画像では、thumbnail だけでは保存資料として不十分になる可能性がある。

original だけを保存する案は情報を失いにくいが、HTML の inline 表示には大きすぎる画像をそのまま参照することになりやすく、閲覧性と出力サイズの両面で扱いづらい。

thumbnail と original の両方を保存する案は出力サイズが増えるが、Slack 風の閲覧性とアーカイブ性を両立しやすい。既に `--max-attachment-size` を採用しているため、original のサイズ肥大化は同じ option で制御できる。

Slack file object の `preview` は画像 upload の preview 画像とは限らないため、画像表示用には `thumb_1024`、`thumb_960`、`thumb_720`、`thumb_480`、`thumb_360` などの available な thumbnail URL を使う方が実態に合う。

## 決定

ユーザーがアップロードした画像は、thumbnail と original の両方を保存する。

HTML では thumbnail を inline image として表示し、クリックすると保存済み original を開けるようにする。

original 画像の保存には `--max-attachment-size` を適用する。original がサイズ上限を超える場合、original は download せず、HTML では thumbnail 表示を残したうえで original が保存されなかったことを示す。

asset ファイル名は `0016-asset-filenames.md` の方針に従い、URL hash ベースにする。保存先は `assets/uploads/thumbs/` と `assets/uploads/originals/` のように分ける。

## 理由

画像以外の添付ファイルも保存する方針と整合し、画像についても original をローカルに残せる。

thumbnail を inline 表示に使うことで、HTML の見た目と閲覧性能を保ちつつ、必要なときだけ original を開ける。

original の保存量は `--max-attachment-size` で制御できるため、CI artifact サイズや実行時間の肥大化に対する既存の制御手段を流用できる。

## 影響

実装では、Slack file object から表示用 thumbnail URL と original URL を別々に解決する。

`.cache/assets_manifest.json` には、同一 Slack file ID に対して thumbnail asset と original asset の関係、保存成否、サイズ上限超過の状態を記録する。

HTML 生成では、thumbnail が保存できた場合は `<a>` の内側に `<img>` を置く形で original へリンクする。original が保存できなかった場合は、thumbnail 表示と上限超過メッセージを組み合わせる。

## 後から見直す条件

CI artifact サイズや実行時間への影響が大きい場合は、original 画像を保存しない option や、画像 original 専用のサイズ上限 option を検討する。

Slack file object の thumbnail / original URL の提供形態が変わり、thumbnail と original の両方保存が安定しなくなった場合は再検討する。
