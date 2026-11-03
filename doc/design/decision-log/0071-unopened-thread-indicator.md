# 0071 一度も開いていない thread の chip に赤い丸印を出す

- 状態: decided
- 作成日: 2026-11-03
- 最終更新日: 2026-11-03
- 関連: `../html-rendering.md`、[0012-html-rendering-style.md](0012-html-rendering-style.md)、Issue #305

## 背景

export の HTML では、thread replies を親投稿の下の chip(`<details class="thread-group">` の `<summary class="thread-label">`)にまとめて折りたたんでいる。chip は背景と枠を透明にした控えめな表示で、thread があること自体が見逃されやすかった(Issue #305)。

最新の Slack は、thread の中に未読があることを赤い丸印で示す。このとき `>` のような印は出さず、hover すると最終返信日時と右端の `>` を出す。Issue は、このうち赤い丸印を取り入れ、一度も開いていない thread の chip に表示することを求めた。一度開いた thread では以降は表示しなくてよく、開いたことを恒久的に保持する必要は無い(再読み込みで再び表示してよい)。Slack と同じ見た目にすることは求めておらず、thread を開くサイドバーを持たない slapex の archive で見やすい表示を探ることが目的である。

出力の HTML は JavaScript を一切使わない(0012、`../html-rendering.md` の表示方針 10)。CSS で表現できる interaction は使ってよい(表示方針 12)。

## 候補

開いたことを覚える方法:

- A. JavaScript で開いた thread を記録し、丸印を消す(`sessionStorage` などで再読み込み後も保持することもできる)。
- B. `.thread-group:not([open])` のときだけ丸印を出す。閉じると丸印が戻る。
- C. 丸印を閉じた状態の値へ戻す変化に、極端に長い `transition-delay` を付ける。開くと遅延 0 で消え、閉じても遅延の間は消えたままになる。再読み込みで初期状態に戻る。

丸印の位置:

- P1. 件数(`N messages`)の右。
- P2. 参加者 avatar の左(chip の先頭)。
- P3. 右端の caret の位置。caret は hover 時だけ出す(Slack と同じ)。

あわせて検討したこと:

- Q1. Slack の hover 時の最終返信日時を chip に出す。
- Q2. caret を hover 時だけ出す。

## 検討内容

A は表示方針 10 に反する。0012 が JavaScript を使わない理由(静的 HTML としての保存性、ブラウザや security policy による実行制限、ローカル閲覧時の安全性)は今回も変わらない。

B は単純だが、閉じると丸印が戻る。Issue が求める「一度開いたら以降は表示しない」に当たらない。

C は CSS だけで「一度開いたら、再読み込みまで表示しない」を実現できる。CSS の transition は変化後の値に付いた transition の指定に従うため、開いたとき(`[open]` の規則、遅延 0)は即座に消え、閉じたとき(既定の規則、遅延 1000000 秒、約 11.6 日)は消えたままになる。ページを開いた時点の値には transition が掛からないため、初期表示では閉じた thread に丸印が出て、HTML で `open` を付けた thread には出ない。`visibility` を使い、`display: none` は使わない(`display` は既定では transition しない)。Chromium で、開く、閉じる、開き直す、再読み込みの各場合に期待どおりに動くことを確かめた。Firefox と Safari は確かめていない。いずれも `visibility` の transition と遅延に対応している。遅延が過ぎれば丸印は戻るが、1 つのページを 11 日以上開いたままにする閲覧は想定しない。

P1 は丸印を件数の後ろに置くため、丸印の有無で avatar と件数の位置が変わらない。`visibility: hidden` の丸印は場所を占めたまま見えなくなり、caret は右端に寄せてあるため、開いた状態の chip の見た目は丸印を足す前と同じになる。P2 は丸印の幅だけ avatar と件数がずれ、chip の先頭を親投稿の本文の左端に揃えている今の配置が崩れる。P3 は caret を hover 時だけにする変更(Q2)を伴う。

Q1 は Issue が取り入れる点として挙げていない。slapex は返信の時刻を各返信に絶対時刻で表示しており、chip に足す必要は薄い。Q2 は採らない。slapex の thread はサイドバーではなくその場で開閉する disclosure であり、常に見える caret が開閉できることを示している(`../html-rendering.md`)。hover できない環境(touch 端末など)でも開閉の手がかりが残る。

色は Slack の通知の赤に近い `#e01e5a` とし、`:root` の `--thread-unopened-dot` に置く。thread の他の色と同じく token にして、theme の差し替えで変えられるようにする。

## 決定

- 閲覧中のページで一度も開いていない thread の chip に、件数の右に直径 8px の赤い丸印を出す(P1)。
- 開いたことは C の方法で覚える。開くと丸印を消し、閉じた後も再読み込みまで出さない。開いたかどうかは保存せず、再読み込みで再び出す。
- JavaScript は使わない。HTML の構造は変えず、`style.css` の `.thread-label-count::after` と `.thread-group[open] .thread-label-count::after` で実装する。
- caret は常に表示したままにする。最終返信日時は出さない。

## 理由

thread があることに気づきやすくするという Issue の目的に対し、丸印は既存の chip の配置を変えずに注意を引ける。C は表示方針 10 を保ったまま、Issue が許した「再読み込みで再び表示する」という範囲で「一度開いたら表示しない」を満たす。P1 なら、開いた状態の見た目と chip の左端の揃えが今と変わらない。

## 影響

- 実装: `internal/render/templates/style.css` に token と 2 つの規則を足す。template と export の処理は変えない。
- test: `internal/render/html_test.go` で token と規則を確かめる。
- spec: `../html-rendering.md` の thread の段落に丸印の表示を書く。
- sample export: `doc/samples/{ja,en}/style.css` が変わる。sample の `index.html` は変わらない。
- README の preview screenshot: thread の screenshot は thread を開いた状態で撮るため丸印は出ず、timeline の screenshot は thread を含まない範囲を切り出すため、見た目は変わらない見込みである。再生成で確かめる。
- 印刷すると、閉じた thread の丸印も印刷される。

## 後から見直す条件

- 主要なブラウザで transition の latch が期待どおりに動かないことが分かった場合。B へ戻すか、表示の方法を見直す。
- 開いたことを再読み込み後も保持したいという要望が出た場合。JavaScript を使わない方針(表示方針 10)の見直しを伴う。
- 未読のような、export の時点の Slack の状態を表す情報を出したくなった場合。丸印は「このページで開いたか」だけを表しており、Slack の既読状態とは関係しない。
