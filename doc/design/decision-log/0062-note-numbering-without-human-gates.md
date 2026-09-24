# 0062 採番 skill の定型フローに人間の手番を置かない

- 状態: decided
- 作成日: 2026-09-24
- 最終更新日: 2026-09-24
- 関連: `.agents/skills/number-working-branch-note/SKILL.md`, `doc/guidelines/working-branch-notes-handling.md`, `doc/guidelines/git-operation-guidelines.md`, `.agents/skills/run-issue-task/SKILL.md`, `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md`, [0059-issue-review-cycle-orchestration.md](0059-issue-review-cycle-orchestration.md)

## 背景

`number-working-branch-note` は、PR 採番の直後に必ず通る skill である。`run-issue-task`(step 9)、`release`(Step 5)、`maintain-progress`、`register-progress-issue` から呼ばれ、`drive-issue-to-reviewed-pr`(0059)では P1 の中で実行される。agent に長時間の自律的な作業を任せるには、この経路で人間の手番が入らないことと、その代わりに何を書き換えたかがユーザーへ確実に届くことの両方が要る。

この skill に人間の手番を置くかは、次の 3 回にわたって扱われた。前の 2 回は decision log を作っていない。

- Issue #171 / PR #172(2026-07-13): push 前のユーザー確認と、列挙済みの stale 表現を書き換える前の合意を削除し、定型置換表による機械置換へ移した。定型フローが運用でこなれたこと、`doc/guidelines/git-operation-guidelines.md` が `git push` にチャットでの承認を求めていないことが理由である。例外・矛盾時の停止条件(前提チェック、無関係な draft の混在、番号付き note との衝突、ファイル名参照か汎用 placeholder かの判別、commit 対象への混入、情報統制)と、手順の前置きの「判断に迷うときは進めずに確認する」は、安全停止として意図的に残した。判断理由は Issue #171 と `working-branch-notes/172_number-working-branch-note-remove-user-gates.md` にだけあった。
- Issue #224 / PR #225(2026-09-12): 検出対象に「本 skill の実行で完了するタスク行」を加えた。取り込み元(asahimaru/asahimaru#725)のユーザー確認ゲートは取り込まず、#171 の方針に合わせて機械置換とした。review の指摘を受け、`progress.md` の PR 番号反映を対象外とし、複合行は触らず報告する扱いを足した。検出範囲の調整であり方針の変更ではないとして、decision log は作らなかった(`working-branch-notes/225_adopt-note-numbering-and-pr-tool-name-updates.md`)。
- Issue #228(本ログ): kiyohara/bizdate は Issue #52 / PR #54(bizdate の decision log 0020)で、完了タスク行に対する合意ゲートを撤回し、報告項目を足した。slapex はゲートを外す部分をすでに通過していたが、bizdate の結果と照合すると次の 2 点が閉じていなかった。
  - 本文は 6 箇所で「終了時に報告する」と定めていたが、「終了時の報告」節の項目は PR description / title の変更点だけだった。note 本文で書き換えた行と、触らなかった行を受ける項目が無く、合意ゲートの代わりになる確認経路が規定として閉じていなかった。
  - 手順の前置きの「判断に迷うときは進めずに確認する」が、完了タスク行を「触らず、終了時に報告する」扱いと衝突して読めた。

## 候補

### 定型フローの人間の手番

- A: 置かない(#171 と #225 の扱いを維持する)。判断できない行は止めずに触らず、終了時に報告する。
- B: 完了タスク行の書き換えに合意ゲートを置く(asahimaru/asahimaru#725 の取り込み元の扱い、bizdate の PR #54 より前の扱い)。
- C: 判断できない行があるときだけ止めて確認する(手順の前置きの一般規定をそのまま当てる)。

### 確認経路

- D: 「終了時の報告」に、書き換えた行の一覧と触らなかった行の一覧の 2 項目を足す。
- E: 書き換えた行の一覧だけを足す。
- F: 報告の項目は足さず、PR diff を確認経路とする。

## 検討内容

- B は、#171 が撤回した合意ゲートを完了タスク行について戻す。採番は PR 作成の直後に必ず通る経路であり、ゲートがあれば `drive-issue-to-reviewed-pr` の P1 の途中で人間の手番が毎回入る。完了タスク行の判定基準と、複合行・判断できない行を触らない切り分けは #225 で明文化済みであり、ゲートが守る「判断を要する行」の処置は skill 内で決まっている。書き換えは checkbox を `- [x]` にするか行末に `(完了)` を付けるだけで、行は削除しない。note は最終仕様書ではなく作業メモである(`doc/guidelines/working-branch-notes-handling.md` の「性質」「整合性のスコープ」)。低リスクで可逆な付記に、毎回の手番の費用は見合わない。
- C は、判断できない行が 1 行あるだけで採番全体が止まる。触らずに残した行は note と PR description にそのまま残り、報告で人間に届く。止めて得られるのは、人間がその場で書き換えを決めることだけであり、報告を読んで後から直す運用で足りる。
- 判断できない場面の安全弁のうち、定型置換の対象の行以外(前提チェック、無関係な draft の混在、番号付き note との衝突、ファイル名参照か汎用 placeholder かの判別、情報統制、commit 対象への混入)は、#171 が安全停止として残したものである。進めると誤った rename、上書き、秘密情報の push、無関係な変更の巻き込みにつながり得て、書き換えのような付記では済まない。A の例外は定型置換の対象の行に限る。
- F は、note の書き換えについては成り立つ。採番の commit は PR diff に現れ、merge 前の review で確かめられる。ただし PR description と title の変更は PR diff に現れない。採番の commit が本体の変更に埋もれる PR では、diff での気づきも期待しにくい。PR diff は補助の経路とする。
- E は、書き換えた行は受けられるが、本文が「触らず、終了時に報告する」と定めた行(定型に当てはまらない表現、複合行、完了を判断できない行、文脈が不自然になる表現、曖昧な title)を受けられない。約束の半分しか閉じない。
- D の 2 項目は、該当が無い場合も「なし」と書く。項目ごと省くと、書き換えが無かったのか報告が落ちたのかを区別できない。
- 旧項目の「PR description / title への変更点」は、D の「書き換えた行の一覧」へ統合する。note 本文と PR description は同じ定型で書き換えるため、書き換え前後を 1 項目に並べる方が照合しやすい。PR diff に現れない PR description のファイル名参照の置換も、同じ項目に含める。
- 完了タスク行の判定基準には、適用範囲を明記する。完了として扱うのは、本 skill の前提として満たされている要素(PR 作成)と、本 skill の実行そのもので達成される要素(note の rename、`PR:` 欄の記入、note 参照の更新、それらの commit と push)に限る。その達成を確認する作業や記録する作業(「採番で人間の手番が入らないことを確認する」「採番結果を検証欄に記録する」など)は、本 skill の終了後に外側が行う。bizdate の PR #54 と同じ整理である。

## 決定

A と D を採用する。

- 採番 skill の定型フロー(rename、note 本文の更新、情報統制チェック、commit、push、PR description の更新)に人間の手番を置かない。#171 の撤回と #225 の導入の扱いを、方針として確定する。
- 「stale 表現の定型置換」の対象(状況を説明する stale 表現、本 skill の実行で完了するタスク行、title の stale な記述)で判断に迷う行は、処理を止めず、合意も求めず、触らずに終了時に報告する。手順の前置きに、この例外を明記する。前提チェック、対象 note の特定、番号付き note との衝突、ファイル名参照の判別、情報統制チェック、commit 対象の限定で止まる扱いは変えない。
- 「終了時の報告」に「書き換えた行の一覧」と「触らずに残した行の一覧」を置き、本文の「終了時に報告する」の報告先をこの 2 項目に集約する。旧項目の「PR description / title への変更点」は前者へ統合する。該当が無い場合も「なし」と書く。
- 可視化は「終了時の報告」を正規の経路とし、PR diff を補助とする。
- 完了タスク行の判定基準に、上記の適用範囲を明記する。判定基準そのものと書き換え形式(`- [x]` / 行末 `(完了)`、行を削除しない)は変えない。
- 「やらないこと」に、機械判定できる行の書き換えで合意を求めて止めないことと、判断できない行があることを理由に採番全体を止めないことを加える。

### 他 skill の走査

同種のゲート、すなわち成果物の内容判断を理由に、判断できる場合でも書き換え前にユーザーの合意を求めるものが他 skill に無いかを確かめた。`.agents/skills/` 配下の全 `SKILL.md` と `review-pull-request/references/` を、確認・合意・承認・停止などの語で走査し、該当行を読んで分類した。行番号付きの表は Issue #228 の本文(`d0800fe` 時点)と、本ログを追加した PR #238 の description(`02f8ac8` 時点。Issue の後に追加された `drive-issue-to-reviewed-pr` を含む)にある。

| 分類 | 該当 |
| --- | --- |
| 入力の曖昧性 | `run-issue-task`(入力、候補が一意に決まらない)、`register-progress-issue`(入力、依存・スコープ)、`review-pull-request`(モード不定)、`release`(通常フロー外のリリース、バージョン未指定)、`drive-issue-to-reviewed-pr`(入力、既存 PR が一意に決まらない、再開位置の不定)、`number-working-branch-note`(無関係な draft、commit 対象への混入) |
| 前提条件の不成立 | `run-issue-task`(依存未完了)、`register-progress-issue`(closed など)、`review-pull-request`(open PR が無いなど、verify の担当不一致)、`release`(適用範囲、CI)、`drive-issue-to-reviewed-pr`(依存未完了、head の想定外の変化、push できない、subagent を起動できない、write の失敗)、`number-working-branch-note`(適用範囲、前提チェック、番号付き note との衝突) |
| 判断不能な場面の安全弁 | `maintain-progress`(判断に迷う箇所)、`run-issue-task`(仕様判断を補完しない)、`release`(判断に迷うとき、decision log の要否)、`review-pull-request`(Agent 種別を判定できない、反復上限)、`update-sample-exports`(実質差分か判断できない)、`drive-issue-to-reviewed-pr`(仕様との食い違い、CI や生成物の説明できない差分、追加情報が必要な指摘、反復上限)、`number-working-branch-note`(手順の前置き、ファイル名参照の判別) |
| 影響の大きさ・取り消しにくさ | merge、resolve、`APPROVE` / `REQUEST_CHANGES` の自動実行をしないこと(`review-pull-request`、`release`、`maintain-progress`、`register-progress-issue`)、課金や cloud 実行を伴う review 機能(`review-pull-request`)、`progress.md` の役割変更(`maintain-progress`)、実 token・外部通信(`update-sample-exports`、`update-readme-demo-gif`)、host OS での実行(`drive-issue-to-reviewed-pr`) |
| 被委譲 skill の停止の中継 | `drive-issue-to-reviewed-pr`(subagent は止めて、確認事項を停止理由として返す) |
| 1Password(Issue #229 の担当) | `number-working-branch-note` と `release` の署名・push 失敗時の再実行と `op` 固有の分岐 |
| 1Password と取り消しにくさの切り分けが要る(Issue #229 の担当) | `release` の Step 5 の push 確認と tag push の承認 |

成果物の内容判断を理由とするゲートは見つからなかった。判断不能な場面の安全弁は、判断できる場合にまで合意を求めるものではなく、本決定の対象外とする。`update-sample-exports` の「実質差分か判断できない場合はユーザーに確認する」もこれに当たり、変えない。別 Issue は切らない。

`number-working-branch-note` の手順の前置きだけは、判断不能な場面の安全弁でありながら、完了タスク行を触らず報告する扱いと衝突して読めたため、上記の決定で例外を明記した。

`release` の Step 5 は、`number-working-branch-note` で採番した直後に「push はユーザー確認後に行う」を置く。採番 skill は #171 で push 前の確認を外しているため、`release` 経由のときだけ push の扱いが食い違う。push のゲートの由来(1Password か、取り消しにくさか)の切り分けは Issue #229 が扱うため、本ログでは変えない。

## 理由

- ゲートの費用は、必ず通る経路で毎回発生する人間の手番である。守る対象は作業メモへの可逆な付記であり、判断を要する行の処置は skill 内で決まっている。費用が便益を上回る。
- 判断できない行を止めずに触らないのは、書き換えないことで誤りを作らず、報告で人間に届けられるからである。止めても、人間の判断が報告を読んだ後から前に移るだけである。
- 合意を外した代わりに、ユーザーが書き換えを知る経路を規定として閉じる必要がある。本文が約束した報告の受け皿が無ければ、ゲートを外した根拠が成り立たない。
- 同じ論点(採番 skill にゲートを置くか)が #171、#224、#228 と 3 回扱われた。判断理由を残し、後続の agent が同じ議論を繰り返さないようにする。

## 影響

- `.agents/skills/number-working-branch-note/SKILL.md`: 手順の前置き、「stale 表現の定型置換」(完了タスク行と共通の扱い)、Step 5、Step 10、「終了時の報告」、「やらないこと」を揃えた。frontmatter の `description` は変えない(skill の責務は変わらない)。
- `doc/guidelines/` と他の skill は変えない。
- 可視化は本 skill の「終了時の報告」に依存する。報告を省いた実行は、この決定の前提を欠く。本 skill を呼ぶ上位 skill(`run-issue-task`、`release`、`maintain-progress`、`register-progress-issue`、`drive-issue-to-reviewed-pr`)は、決定時点でこの 2 項目を自身の報告へ引き上げる規定を持たない。引き上げは Issue #230 で扱う。
- PR #238 の採番を変更後の skill で行い、人間の手番なしで完了した。PR #225 で未通過だった完了タスク行の書き換え経路を初めて通し、書き換えた行と触らなかった複合行の両方が「終了時の報告」に現れることを確かめた。

## 後から見直す条件

- 上位 skill への報告の引き上げ(Issue #230)を決めたとき。引き上げの規定は本ログへの追記として記録する。
- 本 skill を呼ぶ skill を新たに追加する、または「終了時の報告」から 2 項目のいずれかを外すとき。可視化の前提が規定で担保されなくなるため、ゲートまたは別の安全弁の要否を再検討する。
- 誤った書き換えが実運用で繰り返し起き、報告を読んで後から直す運用では収まらなくなったとき。
- 完了タスク行の判定基準を広げる、または書き換え形式に行の削除や文意の再構成のような不可逆な操作を含めるとき。
