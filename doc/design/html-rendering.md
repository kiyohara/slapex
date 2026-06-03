# HTML 表示仕様

このファイルには、最終成果物の `index.html` の見た目と表示方針をまとめる。

想定読者は、このツールを利用する人間と、HTML / CSS を実装・検証する担当者である。

この内容は議論用の素案であり、実装アーキテクチャ、オプション名、出力ディレクトリ構造は未確定である。

出力ディレクトリ構造や保存される assets は `output-format.md`、利用者の操作の流れは `usage-flow.md` を参照する。表示仕様の決定経緯は `decision-log/0012-html-rendering-style.md`、処理対象 label の表示は `decision-log/0020-target-label-display.md` を参照する。

## 表示方針

最終成果物の `index.html` は、Slack default の投稿表示を模倣した見た目にする。

1. 冒頭に取得対象 workspace、channel、export 実行時刻を表示する。
2. workspace 表示には workspace 名、workspace URL または domain、短い `team_id` を含める。
3. channel 表示には channel 名、channel ID、public/private、archived 状態、bot membership を含める。
4. 投稿は channel timeline と同じく、上から oldest、下へ latest の順に表示する。
5. 日付と時刻は相対表現ではなく、絶対時刻として表示する。
6. thread replies は親投稿の下に、親投稿よりインデントを下げて表示する。
7. thread replies は初期表示で展開済みにする。
8. reaction は、絵文字 icon と件数を可能な限り Slack default 風に表示する。
9. reaction した user の一覧や名前は表示しない。
10. JavaScript は一切使わない。
11. style は `style.css` に分離し、HTML 内に固定的に inline style として埋め込まない。
12. CSS で表現可能な interaction は活用してよい。
13. thread の開閉を入れる場合は、JavaScript ではなく HTML native の `<details open>` / `<summary>` など、JavaScript なしで動作する仕組みを使う。

冒頭に表示する workspace / channel label の内容と、実行中の画面表示との対応は `usage-flow.md` の「処理対象の表示」を参照する。

Slack default 風の avatar、投稿者名、絶対時刻、本文、reactions、attachments を CSS で整え、HTML 自体は静的 file として閲覧できるようにする。

## 画像と添付ファイルの表示

ユーザーがアップロードした画像は、Slack file object の available な thumbnail のうち表示に適したものを保存し、HTML 上の inline image として使う。あわせて original 画像も保存し、inline image をクリックすると original を開けるようにする。

original 画像の保存には `--max-attachment-size` を適用する。original がサイズ上限を超える場合、original は download せず、HTML では thumbnail 表示を残したうえで original がサイズ上限超過により保存されなかったことを示す。thumbnail も取得できない場合は、通常の添付ファイル表示または置換メッセージとして扱う。

保存対象 asset とサイズ上限の方針は `output-format.md` を参照する。
