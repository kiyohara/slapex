---
name: drive-issue-to-reviewed-pr
description: slapex の GitHub Issue を 1 件受け取り、実装と PR 作成から review、review comment 対応、対応結果の再確認までを 1 つのフローとして回す orchestrator。Issue 番号または PR 番号を入力に、`run-issue-task` と `review-pull-request` の 3 モード(`review` / `address-comments` / `verify-comments`)を決まった順で呼び、review と再確認は context を分離するため subagent へ委譲して、review 済みの PR で止まる。merge と review thread の resolve はしない。Issue 着手から review cycle の完了まで一続きで進めたいとき、または既存 PR の review 以降を回したいときに使う。
argument-hint: "<Issue番号 | PR番号 | URL>"
---

# drive-issue-to-reviewed-pr

slapex の Issue 1 件を、実装から「review と再確認を済ませた PR」まで 1 つのフローで運ぶ orchestrator skill。

## 位置づけ

本 skill は新しいレビュー手法を持たない。既存 skill の呼び出し順、委譲境界、判断基準、停止条件を固定することだけを責務とし、被委譲 skill の責務と手順を変えない。

- 実装と PR 作成の正本: `.agents/skills/run-issue-task/SKILL.md`、`doc/guidelines/issue-driven-task-execution.md`
- review / 対応 / 再確認の正本: `.agents/skills/review-pull-request/SKILL.md` とその `references/`
- 本 skill の記述が被委譲 skill や guideline と食い違う場合は、被委譲 skill と guideline を優先する。

review(P2)と再確認(P5)を subagent へ分けるのは、context を分離するためである。実装 context を持つ agent は、自分の変更を妥当と見做しやすい。fresh context の reviewer を立てることは効率化ではなく、指摘の質を保つための設計前提である。

同じ理由から、orchestrator は subagent の出力をそのまま採用しない。各指摘の採否は、orchestrator が対象 Issue、project の正本、実装に照らして確かめてから決める。

## 入力と入口

| 入力 | 開始フェーズ |
| --- | --- |
| Issue 番号 / Issue URL | P1 |
| PR 番号 / PR URL | 下記「PR から始める場合」で決める |

- URL は path(`/issues/` か `/pull/`)で種別を決める。番号だけの入力は、その番号が GitHub 上で Issue か PR かを確かめて決める。
- 次の場合は始めず、ユーザーに確認する(`.agents/skills/run-issue-task/SKILL.md` の「入力」と同じ扱い)。
  - 入力が Issue か PR か曖昧である。指示の文言と GitHub 上の種別が食い違う場合を含む。
  - 別リポジトリの URL である。
  - 複数の Issue または PR を同時に指定されている。
  - 対象 PR が closed または merged である。
- Issue を入力された場合でも、その Issue を `Closes` する open PR が既にある場合は、PR 番号を入力されたものとして「PR から始める場合」に進む。前の session が PR の作成後に途切れた場合などに、ブランチと PR を作り直さないためである。該当する PR が一意に決まらない場合は、始めずにユーザーに確認する。

### PR から始める場合

PR の state と head SHA、既存の review と comment を取得し、canonical metadata(`.agents/skills/review-pull-request/SKILL.md` の「可視 metadata の canonical フォーマット」)から再開位置を決める。

- metadata は、同節の規定どおり、5 つのキーがこの順で連続する 5 行を探して読む。投稿内の位置には依存しない。
- 対象 review cycle は、`Agent` が現在の Agent 種別と一致する review cycle のうち最も新しいものとする。それ以外の cycle と人間の review は「他の review cycle の扱い」に従う。
- 関連 Issue は PR description の `Closes #<番号>` から取る。起点 Issue を持たない PR(索引登録、進捗整理、リリース)では `なし` とする。
- 現在のブランチが PR の head branch と異なる場合は、head branch に切り替えてから進める。切り替えられない場合(cloud session で、PR を作った session と別の session から再開した場合など)は、push を伴う手順(下記の P1 や P4 の残りの手順、P4 の修正、P6 の note の更新など)の前で止まる(「停止とエスカレーション」)。

最新の完了要約とは、対象 review cycle の `review` と `verify-comments` の完了要約のうち最も新しいものを指す。

| 対象 review cycle の状態 | 開始フェーズ |
| --- | --- |
| 無い | P2 |
| 最新の完了要約で指摘または未対応が 1 件以上あり、その後の `address-comments` の返信がまだ付いていない指摘がある | P4(「反復の上限」に従う) |
| 最新の完了要約で指摘または未対応が 1 件以上あり、その後に、対応が要る指摘のすべてへ `address-comments` の返信が付いている | P5 |
| 最新の完了要約で指摘または未対応が 0 件 | 完了要約の `Reviewed head` から現在の head までの差分が `working-branch-notes/` だけなら P6。それ以外の差分があれば、新しい review cycle を P2 から始める |

- top-level の指摘は、同じ cycle の `address-comments` の PR conversation comment があれば返信済みとみなす。
- 処置が「判断に追加情報が必要である」の返信は、返信済みに数えない。その指摘が残る場合は P4 から再開し、「停止とエスカレーション」に従って確認事項を報告する。
- metadata が崩れている、または状態を一意に決められない場合は、始めずにユーザーに確認する。

開始フェーズが P2 または P5 の場合は、前のフェーズの完了条件(「フェーズ」の表)を確かめてから始める。前の session がフェーズの途中(PR 作成の後で採番の前、返信の後で note の push の前など)で途切れた場合に、残りの手順を飛ばさないためである。満たしていない条件は、そのフェーズの残りの手順で満たす。note の無い PR では、note の手順を行わない。

- P2 から始める場合は、P1 の完了条件を確かめる。
  - note が採番前(`doc/guidelines/working-branch-notes-handling.md` の「note の探し方」で `draft_` の note しか見つからない)なら、`number-working-branch-note` を実行する。
  - 関連 Issue が `progress.md` の索引にあり、対応行の PR 欄が反映されていなければ、`run-issue-task` の手順どおりに更新して push する。
  - note の P1 の記録(「working branch note」の表)に、`run-issue-task` の報告から引き上げた項目が無ければ、追記して push する。採番の報告が手元に無い場合(別の session で採番した場合など)は追記せず、終了時の報告でその旨と採番の commit を示す。
- P5 から始める場合は、P4 の完了条件を確かめる。
  - その周の P4 の記録(「working branch note」の表)が note に無ければ、追記して push する。
  - 返信は再開位置の判定で確かめている。修正の push は、`address-comments` が対応済みの返信の前に確かめている。
- 最新 head の check runs は、どちらの場合も「CI の確認点」で、P2 / P5 の前に確かめる。

## 起動前提

- 1 Issue = 1 ブランチ = 1 PR とし、Issue は直列に消化する。PR の merge はユーザーが行う(`doc/guidelines/issue-driven-task-execution.md` の「前提」)。
- Issue の依存確認は P1 の中で `run-issue-task` が行う。本 skill で重複して行わない。

## 参照する正本

作業前に必要な範囲で次を読む。GitHub の tool 名、`gh` の実行形式、開発コマンドの形は本 skill に書かず、これらの正本に委ねる。

- `AGENTS.md`
- `doc/guidelines/issue-driven-task-execution.md`
- `doc/guidelines/github-mcp-guidelines.md`
- `doc/guidelines/git-operation-guidelines.md`
- `doc/guidelines/development-command-guidelines.md`
- `doc/guidelines/working-branch-notes-handling.md`
- `doc/guidelines/working-branch-notes-security.md`
- `doc/guidelines/pull-request-guidelines.md`
- `doc/guidelines/cloud-session-guidelines.md` — cloud session で実行する場合。
- `.agents/skills/run-issue-task/SKILL.md`
- `.agents/skills/review-pull-request/SKILL.md`

## フェーズ

| フェーズ | 担当 | 委譲先 | 完了条件 |
| --- | --- | --- | --- |
| P1 実装と PR 作成 | orchestrator | `run-issue-task`(依存確認から note 採番まで) | PR が open で note を採番済み。索引にある Issue なら `progress.md` の PR 欄を反映して push 済み。`run-issue-task` の報告から引き上げた項目を note に残して push 済み。最新 head の check runs がすべて完了し success |
| P2 review | subagent | `review-pull-request` の `review` | 完了要約 1 本が投稿され、subagent が「返させる出力」を返した |
| P3 判断 | orchestrator | なし | 指摘件数が 0 なら P6、1 件以上なら P4 へ進む |
| P4 対応 | orchestrator | `review-pull-request` の `address-comments` | 各指摘へ処置を返信し、修正と note を push し、最新 head の check runs が完了した |
| P5 再確認 | subagent | `review-pull-request` の `verify-comments` | 完了要約 1 本が投稿され、subagent が「返させる出力」を返した |
| P5 後の判断 | orchestrator | なし | 未対応が 0 件なら P6、残れば P4 へ戻る(「反復の上限」に従う) |
| P6 終了 | orchestrator | なし | 「終了時の報告」を返した |

- P1 では、`run-issue-task` の最後の報告(PR の URL、検証結果、未解決事項、被委譲 skill から引き上げた報告項目)をユーザーへ返さず、本 skill の「終了時の報告」へ回す。引き上げた項目は、P2 の委譲の前に note に残して push する(「working branch note」)。
- `number-working-branch-note` は P1 の中で実行する。採番は commit と push を伴い head を動かすため、review を採番より後に始めることで、review 中に head が動かない。
- `progress.md` の対応行は P1 で更新する。状態は PR 作成前に進め、PR 欄は採番後に `#<PR 番号>` へ更新する(`run-issue-task` の手順)。`number-working-branch-note` は `progress.md` を更新しないため、採番後の PR 欄の反映を P1 の完了条件に含める。索引に無い単発 Issue は更新しない。
- 出力生成系 3 skill(`update-sample-exports`、`update-readme-preview-screenshots`、`update-readme-demo-gif`)の適用判断は、コードを変える担当(P1 と P4 の orchestrator)が行う。判断は各 skill の「いつ使うか」を正本とし、判断の結果と根拠を note と PR description に残す。実行順は `update-readme-preview-screenshots` の「事前確認」と `update-readme-demo-gif` の「README 用 media との境界」を正本とし、本 skill で再定義しない。cloud session で実行できない skill の扱いは `doc/guidelines/cloud-session-guidelines.md` に従う。
- P2 / P5 の subagent は、出力生成系 skill の適用判断の妥当性も review の観点として確かめる。
- `review-pull-request` の「verify-comments の担当一致」は Agent 種別で判定されるため、orchestrator が P5 を自分で実行しても metadata 上は成立してしまう。review と再確認の分担を担保しているのはこの表だけである。P2 と P5 を orchestrator 自身で実行しない。

## subagent への委譲

### 渡す入力

委譲の brief に次をすべて含める。省略した項目は subagent が推測するため、空欄を残さない。P5 で P2 の subagent を再利用する場合も省略しない。

| 項目 | 内容 |
| --- | --- |
| モード | `review` または `verify-comments` |
| 対象 PR | リポジトリと PR 番号 |
| 読むべき正本 | `.agents/skills/review-pull-request/SKILL.md` と、モードに応じた `.agents/skills/review-pull-request/references/review.md` または `.agents/skills/review-pull-request/references/verify-comments.md` |
| 関連 Issue | Issue 番号。起点 Issue を持たない PR では `なし` |
| working branch note | note の repo 相対 path。note の無い PR では `なし` |
| 期待する head SHA | 委譲の直前に orchestrator が確かめた full head SHA(「head SHA の受け渡し」) |
| review cycle ID | P2 では「新しく作る」。P5 では対象 review cycle の ID |
| 追加の観点 | その回で特に確かめてほしい点。出力生成系 skill の適用判断や、「skill を変更する Issue を処理するとき」の観点を含める |

- **skill は名前ではなく repo 相対 path で渡す。** subagent の skill 一覧に載っている保証は無く、同じ session で追加・変更した skill では特に保証が無い。brief には「この path を読んでから始めること」と明示する。
- **可視 metadata の値と書き方は brief で指示しない。** subagent は `review-pull-request` の「可視 metadata の canonical フォーマット」と「`Model` の確認手段」に従い、`Agent` と `Model` を自分で確かめて書く。brief に `Model` の値を書いたり、`unknown` にする、置き場所を変えるなど skill と異なる書き方を指示したりしない。review cycle ID と head SHA は上表のとおり渡すが、subagent は投稿前に同 skill の「投稿前の確認と誤りの訂正」で確かめ直す。
- orchestrator が従う上位の指示(harness の system prompt など)が model の識別子の記載範囲を定めている場合は、その内容を brief に事実として添える。subagent の投稿は orchestrator の session の出力でもあり、subagent の実行環境に同じ指示が載っているとは限らないためである。値や書き方の指示としてではなく、subagent が同 skill の `Model` の項に当てはめる実行環境の情報として渡す。
- orchestrator が cloud session(Claude Code on the web。判定は `doc/guidelines/cloud-session-guidelines.md`)で実行している場合は、その事実と、GitHub 操作の tool routing に `doc/guidelines/github-mcp-guidelines.md` の「cloud session(Claude Code on the web)」が当たることを、brief に事実として添える。`review-pull-request` も同節を tool routing の正本に挙げるが、subagent が参照先の節まで読むとは限らないためである。上記の実行環境の指示と同じく、使う tool の指示としてではなく、subagent が同節を当てはめる実行環境の情報として渡す。
- subagent は受け取った head SHA を前提にせず、自身で PR head を取り直す(`review-pull-request` の「対象 PR の特定と review source」)。期待する head SHA と異なる場合は投稿せず、その旨を停止理由として返す。orchestrator がそれを想定外の変化として扱うためである(「head SHA の受け渡し」)。
- subagent はユーザーへ直接問えない。`review-pull-request` が「ユーザーに確認する」「処理を停止してユーザーへ報告する」とする場面に当たった場合は、処理を止めて確認事項を停止理由として返す。brief にこの扱いを明記する。

brief の形:

```text
slapex リポジトリ(kiyohara/slapex)の PR #<番号> について、review-pull-request の <モード> を実行してほしい。

まず次を読んでから始めること。
- .agents/skills/review-pull-request/SKILL.md
- .agents/skills/review-pull-request/references/<モード>.md

モード: <review | verify-comments>
期待する head SHA: <full SHA>(自分で PR head を取り直し、異なれば投稿せずに報告する)
review cycle ID: <新しく作る | 対象 cycle の ID>
関連 Issue: <#番号 | なし>
working branch note: <repo 相対 path | なし>
特に確かめてほしい点: <観点>

実行環境の指示: <orchestrator が従う上位の指示のうち、model の識別子の記載範囲を定めるもの。無ければこの行を省く>
実行環境: <cloud session で実行している場合は、その事実と、GitHub 操作の tool routing に doc/guidelines/github-mcp-guidelines.md の「cloud session(Claude Code on the web)」が当たること。cloud session でなければこの行を省く>

可視 metadata(Agent / Model を含む)は SKILL.md の規定どおり自分で確かめて書くこと。
commit、push、作業ツリーのファイル変更、address-comments、thread の resolve、APPROVE / REQUEST_CHANGES、Issue の起票はしないこと。
ユーザーへの確認が要る場面では処理を止め、確認事項を停止理由として報告すること。

完了後、次を報告すること。
<「返させる出力」の各項目>
```

### 返させる出力

| 項目 | 内容 |
| --- | --- |
| モード | 実行したモード |
| review cycle ID | P2 は新しく作った ID、P5 は対象 cycle の ID |
| Reviewed head | 投稿の時点で確かめた full head SHA |
| 完了要約の URL | 投稿した review body または PR conversation comment の URL |
| 指摘件数 | 総数と、inline と top-level の内訳。P2 では prefix ごとの内訳(prefix の無い指摘があればその件数も)を加え、指摘ごとに prefix、対象 path と thread の URL を 1 行で添える |
| 再確認の結果 | P5 だけ。区分(`.agents/skills/review-pull-request/references/verify-comments.md` の「処置ごとの確認」)ごとの確認済み件数と resolve 可とした thread の URL の一覧、未対応件数 |
| check runs | 確かめた check runs の状態 |
| 未実施事項 | 実施しなかった検証とその理由 |
| `gh` への fallback | `review-pull-request` の「MCP write failure の安全手順」で `gh` へ fallback した場合の、試した MCP tool、失敗内容、未反映確認の結果、実行した command。無ければ「なし」 |
| 停止理由 | 途中で止まった場合の理由と、ユーザーへ中継すべき確認事項 |
| 訂正できなかった誤り | 投稿の metadata の誤りを編集で直せなかった場合の対象 URL、誤っている箇所、正しい値(`review-pull-request` の「投稿前の確認と誤りの訂正」) |

subagent の最終報告はユーザーへ表示されない。orchestrator が内容を確かめ、必要な部分を要約してユーザーへ伝える。

### subagent にさせないこと

- commit と push、作業ツリーのファイル変更(working branch note を含む)。
- `address-comments`。対応は orchestrator が行う。
- review thread の resolve。
- `APPROVE` / `REQUEST_CHANGES` の投稿。
- Issue の起票。

### P5 の subagent

P5 は、P2 を実行した subagent の再利用を第一選択とする。review 時の判断 context が残るためである。session の再開などで失われている場合は新規 subagent へ fallback し、GitHub 上の可視 metadata(review cycle ID と完了要約)から context を再構築させる。どちらの場合も「渡す入力」は省略しない。

### subagent が使えない環境

subagent を起動できない実行環境では、P2 と P5 の前で止まる。自身の context で review や再確認を代行しない。context の分離という前提が崩れるためである。

止まるときは、PR 番号を含む再開手順をユーザーへ返す。

- orchestrator と同じ Agent 種別の別 session で、`review-pull-request` の該当モードを実行する。P5 は P2 と同じ Agent 種別で行う(`review-pull-request` の「verify-comments の担当一致」)。
- 別 session の完了後に、本 skill を PR 番号の入口から再開する。再開位置は「PR から始める場合」で決まる。

## 判断基準

- P3 は P2 の出力の指摘件数(inline と top-level の合計)で決める。0 件なら P6、1 件以上なら P4 へ進む。指摘の prefix(`review-pull-request` の「コメント言語と文体」)は判断に使わず、`[must]` / `[ask]` が 0 件の場合も P4 を通す。P4 は各指摘へ処置を返信する(「フェーズ」の表)ため、`[imo]` / `[nits]` / `[fyi]` の指摘にも採否と理由が残り、P5 で確かめられる。
- P4 での個々の採否は `.agents/skills/review-pull-request/references/address-comments.md` の処置の分類に委ね、本 skill に分類を複製しない。全件を採用しない場合も、処置の返信は要る。
- 指摘の正しさは推論で決めず、実物で確かめて判断する(同 reference の手順)。
- P5 後は `.agents/skills/review-pull-request/references/verify-comments.md` の完了要約の未対応件数で決める。0 件なら P6、1 件以上なら P4 へ戻る。スコープ外など修正を伴わない処置でも、再確認で処置が妥当と確かめられた指摘は未対応に数えない(同 reference の「処置ごとの確認」)。
- スコープ外の指摘は、処置を「妥当だが今回はスコープ外である」とし、follow-up Issue の候補として P4 で working branch note に残し(note の無い PR では PR description に残す)、終了時の報告に挙げる。候補を残すのは、P5 の再確認が follow-up の記録先を確かめるためである。起票はせず、止まらない。起票の可否はユーザーが終了後にまとめて判断する。
- 処置が「判断に追加情報が必要である」になった指摘がある場合は止まる(「停止とエスカレーション」)。

## 他の review cycle の扱い

同じ PR に、対象 review cycle 以外の review(他の Agent 種別の cycle、人間の review、同じ Agent 種別の以前の cycle)がある場合は次のとおり扱う。

- P4 では、それらの未対応 thread も `address-comments` の対象に含めてよい。
- P5 は対象 review cycle だけを再確認する。1 回の P5 で扱う cycle を 1 つに保つためである。`review-pull-request` の「verify-comments の担当一致」は変えない。
- 対象 review cycle 以外で P4 に対応した thread は、再確認が残るものとして終了時の報告で人間に返す。Agent 種別が異なる cycle は、その Agent 種別か人間が確かめる。

## head SHA の受け渡し

- P2 に渡すのは P1 の完了時の head とする。P5 に渡すのは、P4 で最後に push した head(note の更新を含む)とする。
- 各フェーズの開始時に PR head を取り直す。想定外の変化(人間の push など)があれば止まる。
- P2 / P5 の委譲中は push しない。head が動くと、subagent は古い diff を前提に投稿できず、context の取り直しを強いられる。委譲中に確定した修正と note の追記は、subagent の完了後にまとめて push する。

## CI の確認点

- P2 と P5 の前(委譲する直前。subagent が使えない環境では、P2 / P5 の前で止まる直前)に、最新 head の check runs がすべて完了し success であることを確かめる。未完了なら完了を待つ。通常の流れでは P1 の完了時(PR 作成、採番、`progress.md` 反映、引き上げた項目の note への記録の push 後)と P4 の完了時に当たる。PR の入口から始めた場合も、この確認を通る。check runs と失敗 job の log の取得は `doc/guidelines/github-mcp-guidelines.md` の「操作別の第一選択」に従う。
- 失敗した場合、差分に起因するなら修正して再 push し、完了を待ち直す。差分から説明できなければ止まる。
- Issue の「検証」にある Docker Compose での検証は、PR 作成前の必須手順である。CI はその結果を GitHub 上の head で確かめる位置づけとする。CI は Compose での検証に無い job(`cross-compile` など)を含むため、Compose での検証の代わりにも、Compose での検証が CI の代わりにもならない。
- P6 で note だけを push した場合、その head は確認点に含めない。終了時の報告に、その時点の check runs の状態をそのまま書く。

## working branch note

P1 で `run-issue-task` が作る note に、各フェーズの終わりでセッションログを 1 行追記する。

| フェーズ | 残すこと |
| --- | --- |
| P1 | PR 番号、検証結果、出力生成系 skill の適用判断、`run-issue-task` の報告から引き上げた項目(`.agents/skills/run-issue-task/SKILL.md` の「被委譲 skill の報告の引き上げ」の確認経路の項目と残された事項。0 件、呼ばなかった、途中で停止した場合はその旨) |
| P2 / P3 | review cycle ID、`Reviewed head`、指摘件数(inline と top-level の内訳と、prefix ごとの内訳) |
| P4 | 処置の内訳、スコープ外とした指摘の follow-up 候補、修正 commit、出力生成系 skill の再判断 |
| P5 | 再確認の結果(区分ごとの確認済み件数と未対応件数) |
| P6 | 終了時の状態 |

- 追記は次の push にまとめてよい。ただし P2 / P5 の委譲中は push しない(「head SHA の受け渡し」)。
- P1 の記録は、P2 の委譲の前に push する(「フェーズ」の表の P1 の完了条件)。P1 の終了からフロー終了までに P2 以降が挟まり、`run-issue-task` から引き上げた項目が context から落ち得るためである。
- 「次にやること」は、終了時に人間の手番(thread の resolve、PR の merge、他の Agent 種別の cycle の再確認)だけが残る状態にする。
- P6 の note の更新は、P5 が確かめた head より後の commit になる。note だけの commit とし、終了時の報告でそのことを示す。

## skill を変更する Issue を処理するとき

本 skill を含む skill を追加・変更する Issue をこのフローで処理する場合は、次を守る。

1. 追加・変更した skill を同じ session で使う場合は、SKILL.md を repo 相対 path で直接読む。skill の検出は実行環境に依存し、保証されない。subagent への brief も path で指定する。
2. description による発火は未検証事項として note と PR description に記録する。確かめる手段(ローカルでは Claude Code の再起動、cloud session では新しい session の開始など)を添える。
3. SKILL.md の自己完結性(repo 相対 path だけで読めるか、記述に曖昧さが無いか)を、review の観点として P2 の subagent へ渡す。skill を設計した orchestrator は、記述の不足を context で補って読めてしまうためである。

## 反復の上限

本 skill では上限を定義しない。`review-pull-request` の「反復の上限」に従い、上限に達したら同節のエスカレーションに合流する。本 skill では P4 → P5 の 1 往復を 1 周と数える。

## 停止とエスカレーション

次の場合は止まり、状況と必要な判断をユーザーへ報告する。報告には「終了時の報告」の項目のうち該当するものを添え、note がある場合は止まった時点の状態を残す。

| 状況 | 扱い |
| --- | --- |
| Issue の依存が未完了 | 始めず、未完了の依存を報告する(`run-issue-task` の「終了条件」) |
| Issue の指示と `doc/design/` の仕様が食い違う | 実装で解釈を補わず、Issue にコメントを残して報告する(`doc/guidelines/issue-driven-task-execution.md` の「判断に迷ったとき」) |
| Docker を使えず、host OS での実行にユーザーの承認が要る | 承認を得るまで進めない(`doc/guidelines/development-command-guidelines.md` の「基本方針」と「Docker の確認」) |
| CI の失敗が差分から説明できない。check runs が完了しない | 推測で修正を重ねず、失敗内容と切り分けの結果を報告する |
| 出力生成系 skill の生成物に説明できない差分がある | commit せず、各 skill の「生成後の確認」に従って報告する |
| PR head が想定外に動いた。PR が closed / merged になった | 古い diff を前提に進めず、報告する |
| PR の head branch へ push できない(cloud session で、PR を作った session と別の session から再開した場合など) | push を伴う手順の前で止まり、PR の head branch へ push できる環境での再開を依頼する(`doc/guidelines/git-operation-guidelines.md` の「cloud session(Claude Code on the web)」) |
| `address-comments` の処置が「判断に追加情報が必要である」になった指摘がある | 確認事項を報告し、回答を得てから P4 を続ける |
| `review-pull-request` の反復上限に達した | 同 skill の「反復の上限」のエスカレーションに合流する |
| subagent を起動できない | 「subagent が使えない環境」に従い、再開手順を返す |
| subagent が停止理由を返した | 確認事項をユーザーへ中継し、回答を得てから委譲し直す |
| GitHub の write 操作が失敗し、`review-pull-request` の「MCP write failure の安全手順」で解消しない | 試した操作と反映の状況を報告する |

## 終了時の報告

フローの終了時(P6)に、次をユーザーへ報告する。

- PR の URL と state。
- Issue の検証結果(P1)。
- P1 で `run-issue-task` から引き上げた項目(`.agents/skills/run-issue-task/SKILL.md` の「被委譲 skill の報告の引き上げ」)。note の P1 の記録から報告する。PR の入口から始めて採番の報告が手元に無かった場合は、その旨と採番の commit を書く。
- review cycle ID ごとの指摘件数と、周ごとの処置の内訳。P2 の指摘件数には prefix ごとの内訳を添える。merge 前に直すべき指摘(`[must]`)があったかを読めるようにするためであり、P3 と P5 後の判断には使わない。
- resolve 可とした thread(区分ごと)。resolve は人間が行う。
- 未対応・未収束の指摘と、見解の相違点。
- 再確認を人間に返すもの(他の Agent 種別の cycle や、対象 cycle 以外で対応した thread)。
- 最新 head と check runs の状態。P6 で note だけを push した場合はその旨。
- 出力生成系 skill の適用判断。P4 で使った場合は、その記録から「被委譲 skill の報告の引き上げ」と同じ扱いで引き上げた項目を含める。
- `review-pull-request` の「MCP write failure の安全手順」による `gh` への fallback(P2 / P5 の subagent が返したものと、P4 で行ったもの)。無ければ「なし」。
- 未検証事項。skill を追加・変更した場合は description による発火を含む。
- follow-up Issue の候補。
- 人間に残る作業(thread の resolve と PR の merge。metadata の誤りを訂正できなかった投稿があれば、その編集)。

被委譲 skill の報告は、`.agents/skills/run-issue-task/SKILL.md` の「被委譲 skill の報告の引き上げ」と同じ 3 種の扱いで上記へ含める。`review-pull-request` については、確認経路の項目に当たる `gh` への fallback の明示を上記の項目で届ける。残された事項に当たるもののうち、処理の停止と反復上限のエスカレーション(未収束の指摘、見解の相違点、推奨する次の対応)は「停止とエスカレーション」で止まる時点の報告で、訂正できなかった metadata の誤りは「人間に残る作業」で届ける。被委譲 skill の報告項目を変えるときは、この節と「返させる出力」を揃える。

## やらないこと

- PR の merge。
- review thread の resolve。
- `APPROVE` / `REQUEST_CHANGES` の投稿。
- 複数 Issue の並行実行と、次の Issue の自動選定。
- ユーザーの指示が無い Issue の起票。follow-up は候補として報告する。
- 他の Agent 種別の review cycle の再確認。
- P2 / P5 を orchestrator 自身で実行すること(subagent を使えない場合の自己 review と自己再確認を含む)。
- P2 / P5 の委譲中の push。
- 被委譲 skill の手順の上書き。
- 実行していない検証を実行したものとして記録すること。

## agent 中立性

- 正本は `.agents/skills/` に置き、Codex と Cursor も直接読む。Claude Code は `.claude/skills/` の symlink を経由して読む(`doc/guidelines/agent-configuration-management.md` の「Agent skill 管理」)。
- subagent 機構の無い実行環境では、「subagent が使えない環境」のとおり P2 / P5 の前で止まり、再開手順を返す。再開後は PR 番号の入口から続けられる。
