# 0059 Issue 起点 review cycle の orchestration

- 状態: decided
- 作成日: 2026-09-24
- 最終更新日: 2026-09-26
- 関連: `doc/guidelines/development-loop.md`, `doc/guidelines/issue-driven-task-execution.md`, `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md`, `.agents/skills/run-issue-task/SKILL.md`, `.agents/skills/review-pull-request/SKILL.md`, [0037-issue-driven-task-execution.md](0037-issue-driven-task-execution.md), [0058-cloud-session-environment.md](0058-cloud-session-environment.md), [0061-review-metadata-in-cloud-session.md](0061-review-metadata-in-cloud-session.md)

## 背景

slapex には `run-issue-task`(Issue 着手から PR 作成まで)と `review-pull-request`(`review` / `address-comments` / `verify-comments` の 3 モード)が個別 skill として揃っていた。一方、Issue 着手から「review と対応の再確認まで済んだ PR」に至る流れは、呼び出し順、委譲境界、判断基準を定めた正本が無かった(Issue #227)。目的は、agent に長時間の自律的な作業を任せられる状態を作ることであり、その作業単位を 1 本の手順に固定する。

過去の PR と note を照合すると、次のことが分かった。

- 流れの順序は安定している。PR #221 と PR #201 は、実装、note の採番と `progress.md` の更新、review、address、verify の順で進んだ。
- review の担当は subagent ではなく、別 session か別の Agent 種別だった。PR #221 には Codex の review と Claude Code の review cycle が並び、PR #201 の verify は元 review と同じ Agent 種別の別 session が行った。
- `verify-comments` の担当一致は Agent 種別で判定され、同じ Agent 種別の別 session による verify が受け入れられている。
- address の途中で、スコープ外の follow-up を起票するかがユーザーの判断になっていた(PR #218、PR #221)。
- 指摘の重要度の表記は review ごとに異なっていた(`[should]` / `[nit]` / `[fyi]`、`[medium]` / `[low]`、優先度のラベル)。
- 出力生成系 3 skill の適用判断は実装担当が行い、reviewer が確かめていた。CI は各段で確かめていた。

各段の受け渡し(どの head を見たか、どの review cycle を対象にするか、CI をいつ見るか、ユーザー判断をどこで挟むか)は session ごとの判断に委ねられていた。

同じ目的の対応は kiyohara/bizdate の Issue #50 と PR #51(bizdate の decision log 0019)で先に行われている。本ログはその判断を下敷きにし、slapex 固有の差分(note の採番、`progress.md`、出力生成系 3 skill、Docker Compose での検証、subagent が使えない環境の扱い)を加えたものである。

## 候補

### orchestration の置き方

- A: orchestrator skill を 1 つ追加し、既存 2 skill へ委譲する。
- B: `run-issue-task` を拡張し、PR 作成後の review cycle まで同じ skill で扱う。
- C: `doc/guidelines/development-loop.md` に流れを書き、skill は増やさない。

### review と再確認の実行主体

- R1: fresh context の subagent へ委譲し、実装と対応を担う orchestrator から分離する。
- R2: 同じ agent が実装に続けて review する。
- R3: review は常にユーザーか別 session に委ねる。

### subagent が使えない環境

- F1: P2 / P5 の前で止まり、別 session での実行をユーザーへ依頼する。
- F2: 同じ agent が、入力を GitHub 上の PR diff と metadata に限って review する(bizdate の fallback の 1 つ)。

### 判断基準

- J1: 指摘件数(0 件か 1 件以上か)で分岐し、個々の採否は `address-comments` の処置の分類に委ねる。
- J2: 重要度の表記で分岐する。

### 反復上限の置き場

- L1: `review-pull-request` の既存の上限に委ね、orchestrator では定義しない。
- L2: orchestrator 側に独自の上限を持つ。

### skill 名

- N1: bizdate と同じ `drive-issue-to-reviewed-pr` とする。
- N2: slapex の表記(`review-pull-request`)に揃えた別名にする。

## 検討内容

- **置き方**: B は `run-issue-task` の責務を「Issue を 1 件実行する」から「review cycle まで回す」へ広げ、PR 作成で止めたい場合に分割し直す必要が出る。C は guideline に委譲 interface や brief の形のような実行手順を置くことになり、guideline と skill の役割分担(`doc/guidelines/development-loop.md` の「各資材の役割」)に反する。A は既存 2 skill の責務を変えずに呼び出し順と境界だけを固定でき、個別 skill の単独利用も残る。
- **実行主体**: R2 は実装 context を持つ agent が自分の変更を review することになり、自分の判断を妥当と見做しやすい。PR #221 の review は、実装側が通していた test が判断を守れていないことを probe で示しており、fresh context の reviewer でなければ拾えない種類の指摘だった。R3 は質は高いが、ユーザーの手番を毎回必須にする。R1 は `review-pull-request` が前提とする役割分担(修正担当は resolve せず、元の Review 担当が再確認する)にそのまま乗る。
- **subagent が使えない環境**: F2 は入力を絞っても、同じ agent が実装時の判断を持ったまま review することに変わりなく、R1 を採る理由(context の分離)を崩す。PR #201 の verify は別 session で行われた前例があり、PR 番号の入口から再開できれば、F1 でも運用上の支障は小さい。
- **判断基準**: J2 は review ごとに表記が揃わないため成り立たない(prefix の統一は Issue #216 の範囲)。`address-comments` は確認した各 inline comment へ処置を必ず返信するため、「対応すべきものがあるか」は指摘件数が 1 件以上かで決まり、個々の採否は処置の分類で扱える。
- **反復上限**: L2 は上限が 2 箇所に現れ、値が食い違ったときにどちらが正かを決められない。L1 は正本を 1 つに保てる。
- **skill 名**: N1 は bizdate の同じ path の更新を `git log --first-parent -- <path>` でそのまま追える(後続の bizdate PR #58 / #59 もこの path を触る)。動詞始まりの kebab-case は slapex の既存 skill と揃い、「reviewed PR」で止まることが名前から読める。`pr` の略記が `review-pull-request` と揃わない難点は、追従の容易さを優先して受け入れる。
- **下敷きにする版**: bizdate PR #51 の時点の版を下敷きにする。bizdate PR #58(1Password 連携操作が失敗した場合の停止条件)と PR #59(被委譲 skill の報告項目の引き上げ)は、slapex では Issue #229 / #230 の担当であり、先取りすると両 Issue の結論を先に固定してしまう。本 skill は停止条件と終了時の報告を、両 Issue が追記できる形の節として用意するだけにする。
- **同じ PR に並ぶ他の review**: 他の Agent 種別や人間の review は実運用で同じ PR に並ぶ。対応(P4)から除外すると同じ PR で対応が 2 回に分かれるため、対象に含めてよい。再確認は担当一致を変えられないため、人間に返す。
- **follow-up Issue の起票**: 起票の可否をユーザーに仰ぐために cycle の途中で止めると、自律区間が短くなる。処置を「妥当だが今回はスコープ外である」とし、候補を終了時の報告にまとめる。

## 決定

- orchestrator skill `drive-issue-to-reviewed-pr` を追加する(A、N1)。既存 skill の呼び出し順、委譲境界、判断基準、head SHA と CI の確認点、停止条件、終了時の報告だけを固定し、被委譲 skill の責務と手順は変えない。
- フェーズは P1(実装と PR 作成)、P2(review)、P3(判断)、P4(対応)、P5(再確認)、P5 後の判断、P6(終了)とする。note の採番と `progress.md` の PR 欄の反映は P1 に含め、review を採番の後に始める。
- review と再確認は fresh context の subagent へ委譲する(R1)。subagent が使えない環境では P2 / P5 の前で止まり、PR 番号を含む再開手順をユーザーへ返す。自身の context で review しない(F1)。
- 再確認を行う subagent は、review を実行した subagent の再利用を第一選択とし、失われている場合は GitHub 上の可視 metadata から context を再構築する新規 subagent へ fallback する(bizdate の decision log 0019 と同じ)。
- 判断は指摘件数と、`verify-comments` の完了要約の未対応件数で行う(J1)。重要度の表記は使わない。処置の分類は `address-comments` に委ね、複製しない。
- 反復上限は `review-pull-request` に委ね、orchestrator では定義しない(L1)。
- 途中再開は PR 番号の入口から行う。canonical metadata は `review-pull-request` の規定(0061)どおり、投稿内の位置ではなく 5 つのキーの並びで読む。
- subagent への brief では、可視 metadata の値と書き方を指示しない(0061)。
- CI は P1 の完了時と P4 の push 後の 2 箇所で、最新 head の check runs の完了を待つ。Docker Compose での検証と CI は互いの代わりにならない。
- 出力生成系 3 skill の適用判断は、コードを変える担当(P1 と P4 の orchestrator)の責務とし、subagent は判断の妥当性を review の観点として確かめる。
- 同じ PR の他の review cycle の thread は P4 の対象に含めてよい。P5 は対象の review cycle だけを再確認し、それ以外は人間に返す。
- follow-up Issue は起票せず、候補として終了時の報告に挙げる。
- PR の merge、thread の resolve、`APPROVE` / `REQUEST_CHANGES` は自動化しない。
- skill を追加・変更する Issue をこのフローで処理する場合、同じ session では SKILL.md を repo 相対 path で直接読み、description による発火は未検証事項として記録する。SKILL.md の自己完結性を review の観点として subagent へ渡す(bizdate の decision log 0019 と同じ)。

## 理由

- 既存 skill の責務境界を保ったまま、session ごとの判断に委ねていた受け渡しを正本にできる。
- context の分離は、この体制で指摘の質を保つ実効的な手段である。分離できない環境で自己 review に落とすと、skill の前提そのものが崩れる。
- 上限と処置の分類の正本を 1 つに保てば、orchestrator と被委譲 skill の二重定義を避けられる(`doc/guidelines/agent-configuration-management.md` の「ルールをシンプルに保つ」)。
- 判断を件数に置けば、review ごとに揃わない重要度の表記に左右されない。
- merge と resolve を人間に残す前提は、0037 と `review-pull-request` の制約に由来し、orchestrator を挟んでも変える理由が無い。

## 影響

- `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md` と、`.claude/skills/drive-issue-to-reviewed-pr` の symlink を追加した。
- `doc/guidelines/development-loop.md` の「使う skill」表に本 skill の行を足した。標準フローの図と参照順は変えていない。
- `.agents/skills/run-issue-task/SKILL.md` と `.agents/skills/review-pull-request/SKILL.md` に、本 skill から呼ばれ得ることを示す参照を足した。本ログの範囲では手順を変えていない。同じ PR で行った `review-pull-request` の metadata の規定の変更は、0060 と 0061 による。
- subagent を 1 review cycle あたり 1〜2 本起動するため、token の消費は個別 skill の運用より増える。指摘の質と再現性を取る判断とする。
- Codex や Cursor など subagent 機構が Claude Code と同じでない環境では、P2 / P5 の前で止まり、別 session での実行を挟む。
- Issue #229 と #230 は、本 skill の「停止とエスカレーション」と「終了時の報告」へ追記する。

## 後から見直す条件

- 被委譲 skill の責務やモード構成が変わり、委譲境界が実態と合わなくなったとき。
- subagent の再利用や識別の方法が変わり、`verify-comments` の担当一致が成り立たなくなったとき。
- 反復上限の範囲で収束しない事例が続いたとき。
- subagent を起動するコストが、得られる指摘の質に見合わないと判断されたとき。
- Codex や Cursor に同等の分離の仕組みが入り、止まる fallback が不要になったとき。

## 追記(2026-09-24): PR の入口での前のフェーズの完了条件と CI の確認点

Issue #234。PR 番号の入口は、対象 review cycle の状態だけから開始フェーズを決め、それより前のフェーズの完了条件を確かめていなかった。PR #233 の再確認の fyi と、同 PR への Codex の review で同じ点が挙がった。

- P1 の途中(PR 作成の後、採番の前など)で途切れた session を PR の入口から再開すると、対象 review cycle が無いため P2 から始まり、note の採番と `progress.md` の PR 欄の反映が飛ばされる。Issue を入力し、`Closes` する open PR があって PR の入口に入る場合も同じである。P2 以降は採番をせず、review の途中で採番すると委譲中に push しない規定とぶつかる。
- CI の確認点は P1 の完了時と P4 の push 後で、PR の入口からはどちらも通らない。`address-comments` の返信の後、check runs の完了前や note の P4 の記録の push の前に途切れると、再開位置の表により P5 から始まり、失敗した head や記録の無い head を再確認に渡し得る。

check runs の確認の置き方として、次の 2 案を比べた。

- C1: PR の入口に、開始フェーズごとの確認を足す。P2 なら P1 の完了条件のすべて(採番、`progress.md`、check runs)を、P5 なら check runs を確かめる(Issue #234 の本文の案)。
- C2: 確認点を「P2 と P5 の前」(委譲する直前。subagent が使えない環境では止まる直前)に置き直し、入口によらず確かめる。PR の入口には、P1 の完了条件のうち check runs 以外(採番、`progress.md`)の確認だけを足す(Issue #234 のコメントの案。bizdate の同 skill の「head SHA と CI」と同じ形)。

通常の流れでは P1 の次は P2、P4 の次は P5 であり、C2 の確認点は現行の 2 箇所と同じ時点になる。bizdate は subagent が使えない環境でも同じ agent が P2 / P5 を実行し得るため、委譲の直前だけで足りる。slapex は P2 / P5 の前で止まる(本ログの F1)ため、止まる直前も確認点に含める(PR #237 の review で指摘された)。C1 は check runs の確認を「CI の確認点」と PR の入口の 2 箇所に書くことになり、入口や開始フェーズが増えるたびに書き足しが要る。C2 は 1 箇所で済み、bizdate の同じ path との差分も小さい(bizdate の Issue #82 は、この形のため P1 の残りだけを対象にしている)。

決定:

- C2 を採る。「CI の確認点」は、P2 と P5 の前(委譲する直前、または subagent が使えない環境で止まる直前)に最新 head の check runs を確かめる形とし、PR の入口から始めた場合もこれを通る。本ログの「決定」にある「CI は P1 の完了時と P4 の push 後の 2 箇所で」は、この形に置き換える。通常の流れで確かめる時点は変わらない。
- PR の入口で開始フェーズが P2 の場合は、P1 の完了条件を確かめ、満たしていない条件(note の採番、索引にある Issue の `progress.md` の PR 欄)を P1 の残りの手順で満たしてから始める。手順は `number-working-branch-note` と `run-issue-task` を参照し、本 skill に複製しない。
- P5 から始める場合も、P4 の完了条件を確かめる(Issue #234 の未決事項 1 の仮決定)。check runs は「CI の確認点」で確かめ、その周の P4 の記録が note に無ければ追記して push する。返信は再開位置の判定で、修正の push は `address-comments` の手順で満たされている。P2 だけに置くと、同じ隙間が P5 に残る。
- check runs が失敗した場合の扱い(差分に起因すれば直して再 push し、説明できなければ止まる)は変えない。bizdate は PR の外に原因がある失敗を brief に明記して委譲するが、本 Issue の範囲では slapex の停止の規定を変えない。
- PR の head branch へ push できない場合に止まる手順に、P1 や P4 の残りの手順を含める。採番、`progress.md` の反映、note の P4 の記録は push を伴うためである。

影響:

- `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md` の「入力と入口」「PR から始める場合」「CI の確認点」を変えた。「フェーズ」の表の完了条件は変えていない。
- Issue #230 は P1 の完了条件に項目を足す。足す項目は、PR の入口での P1 の確認にも加える。
- bizdate への反映は bizdate の Issue #82 で扱う。

## 追記(2026-09-26): review の prefix の統一後の判断基準

Issue #252。本ログは J2(重要度の表記で分岐する)を、review ごとに表記が揃わないことを理由に退けた(背景の「指摘の重要度の表記は review ごとに異なっていた」、検討内容の「判断基準」、理由の「判断を件数に置けば、review ごとに揃わない重要度の表記に左右されない」)。その後、PR #235(Issue #216)で、`review-pull-request` の `review` モードの指摘に `.github/copilot-instructions.md` の prefix(`[must]` / `[ask]` / `[imo]` / `[nits]` / `[fyi]`)を付けることになった。P2 の subagent はこの skill で review するため、P3 の入力(P2 の出力)の表記は揃い、この理由は成り立たなくなった。PR #235 は、件数で判断する結論は変わらないとして本ログと `drive-issue-to-reviewed-pr` の文言を変えず、follow-up とした。

表記が揃った前提で、P3 の分岐を次の 2 案で比べ直した。

- J1: 指摘件数(0 件か 1 件以上か)で分岐する(現行)。
- J2': `[must]` / `[ask]` が 0 件なら P4 を飛ばし、P6 へ進む。

検討:

- J2' では、`[imo]` / `[nits]` / `[fyi]` だけの cycle が P4 を通らず、それらの指摘に処置の返信が残らない。P4 はこれらの指摘も含めて各指摘へ処置を返信し、採否を `address-comments` の処置の分類で決める。P4 を飛ばすと、採るべき指摘(正しい `[nits]` など)を拾えず、採らなかった理由も残らない。P5 で処置を確かめることもできない。
- prefix は reviewer の判断である。`drive-issue-to-reviewed-pr` は subagent の出力をそのまま採用せず、各指摘の採否を orchestrator が実物で確かめてから決める(同 skill の「位置づけ」)。J2' では、orchestrator が指摘を確かめるかどうかが reviewer の重要度の判断で決まる。
- J1 の費用は、`[must]` / `[ask]` の無い cycle でも P4 と P5 を 1 周回すことである。本ログの影響にある token の消費と同じく、指摘の質と記録を取る判断とする。

決定:

- J1 を維持する。本ログの検討内容の「判断基準」で J2 を退けた理由(表記が揃わない)と、理由の「判断を件数に置けば、review ごとに揃わない重要度の表記に左右されない」は、上の検討に置き換える。決定の「判断は指摘件数と、`verify-comments` の完了要約の未対応件数で行う(J1)。重要度の表記は使わない。」は変えない。
- P2 の「返させる出力」の指摘件数に、prefix ごとの内訳を加える(Issue #252 の未決事項 2 の仮決定)。note の P2 / P3 の記録と、終了時の報告の review cycle ID ごとの指摘件数にも添える。merge 前に直すべき指摘(`[must]`)があったかをユーザーが読めるようにするためであり、P3 と P5 後の分岐には使わない。

影響:

- `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md` の「返させる出力」「判断基準」「working branch note」「終了時の報告」を変えた。P3 と P5 後の分岐は変えていない。
- `index.md` の 0059 の行(判断は指摘件数で行い、重要度の表記に依存しない)は、決定が変わらないため変えない。
- bizdate の同じ skill には反映していない(Issue #252 のスコープ外)。
