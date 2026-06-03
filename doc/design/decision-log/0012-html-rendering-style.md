# 0012 HTML Rendering Style

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`, `0020-target-label-display.md`

## 背景

Slack posts export の最終成果物である `index.html` が、どのように投稿、thread、時刻、style、interaction を表現するかを決める必要があった。

特に、ローカル HTML として保存する場合は、JavaScript なしで閲覧できること、将来 style を差し替えやすいこと、Slack 上の見え方に近く直感的に読めることが重要になる。

## 候補

- HTML 内に style と JavaScript を埋め込み、単一 file として完結させる。
- HTML と CSS を分離し、JavaScript は使わない。
- HTML、CSS、JavaScript を分離し、thread 開閉などを JavaScript で実装する。

## 検討内容

HTML 内に style を固定的に書くと、単一 file としては扱いやすいが、将来的な theme 切り替えや style 差し替えがしにくい。

JavaScript を使う方式は interaction を実装しやすいが、ローカル archive としては依存が増える。ブラウザや security policy によって実行が制限される可能性もあり、静的 HTML としての扱いやすさが落ちる。

HTML と CSS を分け、JavaScript を使わない方式なら、最終成果物を静的 file として保ちつつ、style の差し替えで theme 切り替えを実現できる。thread の開閉などは、HTML native の `<details>` / `<summary>` や CSS で表現できる範囲に留める。

## 決定

`index.html` は JavaScript を一切使わない静的 HTML とする。

style は HTML 内に固定的に埋め込まず、外部 CSS file として分離する。初期出力では `style.css` を生成し、`index.html` から相対 path で参照する。

表示内容は次の方針にする。

- 冒頭に取得対象 workspace / channel と export 実行時刻を表示する(詳細は `0020-target-label-display.md`)。
- 投稿は channel timeline と同じく、上から oldest、下へ latest の順に並べる。
- 日付と時刻は相対表現ではなく、絶対時刻として表示する。
- thread replies は元の親投稿の下に、親投稿よりインデントを下げて表示する。
- thread replies は初期表示で展開済みにする。
- 見た目は Slack default の投稿表示を模倣する。
- reaction は、絵文字 icon と件数を可能な限り Slack default 風に表示する。
- reaction した user の一覧や名前は表示しない。
- JavaScript は使わない。
- CSS で表現可能な interaction は活用してよい。
- thread の開閉を入れる場合は、JavaScript ではなく HTML native の `<details open>` / `<summary>` など、JavaScript なしで動作する仕組みを使う。

## 理由

Slack の default 表示に寄せることで、利用者は保存後の HTML を Slack の文脈に近い形で読める。

oldest から latest の順に表示すると、channel の流れを時系列で追いやすい。thread replies を親投稿の直下に展開し、インデントで区別すれば、thread の文脈を別ページへ移動せず読める。

reaction icon と件数は投稿の反応を読むために有用である。一方、誰が reaction したかという情報は archive の主目的には必須ではなく、HTML 上の情報量と privacy 面の負担を増やすため省略する。

絶対時刻は archive として後から読む際に重要であり、相対時刻よりも再現性が高い。

CSS を分離すれば、将来的に theme 切り替えや見た目の調整を、HTML 生成ロジックに触れずに進めやすくなる。JavaScript を使わないことで、静的 HTML としての保存性、CI artifact としての扱いやすさ、ローカル閲覧時の安全性を保てる。

## 影響

- 出力構造に `style.css` を含める。
- `index.html` は `style.css` を相対 path で参照する。
- 実装では style を HTML 内へ inline 固定しない。
- JavaScript file は生成しない。
- thread replies は親投稿の下に展開し、CSS class などでインデントを表現する。
- 時刻表示は絶対時刻として rendering する。
- Slack default 風の avatar、投稿者名、時刻、本文、reaction icon / count、attachments の表示を CSS で整える。
- reaction した user の一覧や名前は HTML に表示しない。

## 後から見直す条件

- JavaScript なしでは必要な interaction を満たせない。
- theme 切り替えの要件が具体化し、複数 CSS file や theme manifest が必要になる。
- Slack default 風の表示が、著作権や branding の観点で避けるべきだと判断される。
- 大きな channel HTML で、全 thread 展開表示が読みづらくなる。
