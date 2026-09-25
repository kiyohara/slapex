# 作業ブランチメモ

- ブランチ: `claude/project-thread-ffymtu`(cloud session が指定。Issue の推奨ブランチ名は `verify-out-of-scope-dispositions`)
- PR: #256
- 最終更新: 2026-09-25

## 目的

Issue #253(RV-02)。`review-pull-request` の `verify-comments` に、`address-comments` で「妥当だが今回はスコープ外である」など修正を伴わない処置を受けた指摘の確かめ方、resolve 可マーカー、cycle の完了の扱いを定める。スコープ外の指摘を verifier が未対応に数え、`drive-issue-to-reviewed-pr` が P4 へ戻って反復の上限で止まる経路を塞ぐ。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 1 件目として選んだ。この後に処理する Issue もすべて同じ review cycle を通るため、先に行う。

## 現在の状況

- 作業内容 1〜5 を実施し、Issue の「検証」を実行した(P1)。

## 決定事項

- 未決事項 1(マーカー)と 2(他の処置への適用)は、Issue の仮決定のまま進めた。
- `verify-comments.md` に「処置ごとの確認」の節を足し、`address-comments` の 7 つの処置ごとに、確かめることと区分を表にした。区分は修正確認済み、スコープ外として確認済み、対応不要として確認済み、未対応の 4 つとした。
  - スコープ外: スコープ外とする判断が PR の目的と関連 Issue の完了条件に照らして妥当か、follow-up の記録先(起票済みの Issue、または note / PR description の候補)があるかを確かめる。本 PR で直すべきと判断した場合と、記録先が見つからない場合は未対応(Step 5)。
  - 修正を伴わない他の 4 処置(既存実装ですでに満たしている、再現しない、guideline と競合する、outdated / duplicate)は、返信が示す根拠が現在の head で成り立つかを確かめ、成り立てば対応不要として確認済みとする(未決事項 2)。確かめる事実が処置ごとに違うため、表の行を分けた。
  - 「判断に追加情報が必要である」は確かめず、未対応とする(未決事項 2 のとおり)。
  - 処置の返信が無い指摘も未対応とする。処置は最新の返信が示すものとする。2 周目で処置が改められた場合にも区分を一意にするためである。
- マーカーは区分ごとに 3 つとし、`SKILL.md` の「Review event と resolve の制約」の canonical 形式を直した。
  - `**修正確認済み(resolve 可)**`(従来どおり)、`**スコープ外として確認済み(resolve 可)**`(Issue の仮決定の例のとおり)、`**対応不要として確認済み(resolve 可)**`。
  - スコープ外を対応不要と分けたのは、resolve の前に follow-up の記録先を確かめる thread だと人間が読めるようにするためである(未決事項 1 の理由)。修正を伴わない処置に「修正確認済み」を付けないことも明記した(PR #242 の再確認の例)。
- 「対応不要」は、修正を伴わない処置のうちスコープ外と「判断に追加情報が必要である」を除く処置について、根拠を確かめた指摘と定義した。cycle の完了条件(Step 10)は、すべての指摘が 3 区分のいずれかに当たる(未対応 0 件)場合とした。
- 完了要約(Step 7)は区分ごとの件数に分け、スコープ外には follow-up の記録先を添える。未対応件数に top-level の指摘を含むことを明記した。`drive-issue-to-reviewed-pr` の P5 後の分岐が未対応件数で決まるためである。top-level の指摘にも同じ区分を当て、マーカーは付けない。
- `drive-issue-to-reviewed-pr`:
  - 「判断基準」の P5 後の項に、妥当と確かめた修正を伴わない処置を未対応に数えないことを、`verify-comments.md` の「処置ごとの確認」への参照で書いた(作業内容 5)。規定は複製しない。
  - スコープ外の follow-up 候補を P4 で note に残すことを、「判断基準」と「working branch note」の P4 の行に足した。orchestrator の流れでは起票しないため、P5 の再確認が確かめる記録先は note になる。従来は「終了時の報告に挙げる」とだけあり、P5 の時点で記録先が無いことがあり得た。
  - 「返させる出力」の再確認の結果、「working branch note」の P5 の行、「終了時の報告」の resolve 可とした thread を、区分ごとの表記に揃えた。
- decision log は作らない。Issue に指示が無く、同じ skill の文言を変えた #216(PR #235)と #222(PR #236)も作っていない。判断の理由は Issue の未決事項、本 note、PR description に残す。
- `address-comments.md` は変えない(処置の分類はスコープ外)。スコープ外の返信に記録先を書く規定も足さない。記録先は verifier が note、PR description、Issue で確かめる。
- 出力生成系 3 skill は、ドキュメントだけの変更で各 skill の「いつ使うか」に当たらないため適用しない。
- `progress.md` は RV-02 の行を done(PR merge後)にした。

## 次にやること

- PR を作成し、note を採番する(P1)。(完了)

## 検証

2026-09-25、cloud session(project の thread)で実行。

### 再確認の結果と P5 後の分岐

変更後の `verify-comments.md` と `drive-issue-to-reviewed-pr` を読み、各状態で再確認の結果と P5 後の分岐が一意に決まることを確かめた。

| 状態 | 再確認の結果 | P5 後の分岐 | 根拠 |
|---|---|---|---|
| スコープ外の処置で、follow-up を Issue として起票済み | 判断が妥当で Issue があれば、スコープ外として確認済み(返信の先頭行は `**スコープ外として確認済み(resolve 可)**`、記録先の Issue を書く)。未対応に数えない | 他に未対応が無ければ P6 | 「処置ごとの確認」の表の 2 行目、Step 4、Step 10。drive の「判断基準」 |
| スコープ外の処置で、follow-up を note と PR description の候補にだけ記録した(orchestrator の流れ) | 同上。記録先は P4 で残した note と PR description | 同上 | 同上。drive の「working branch note」の P4 の行 |
| スコープ外の処置だが、verifier は本 PR で直すべきと判断する | 処置の根拠が成り立たないとして未対応。理由と必要な対応を返信し、マーカーは付けない | 未対応が 1 件以上なので P4 へ戻る。2 周で収束しなければエスカレーション | Step 5、Step 9、表の後の 1 つ目の箇条。drive の「判断基準」 |
| 修正を伴う処置とスコープ外の処置が、同じ cycle に混在する | thread ごとに区分を決め、修正確認済みとスコープ外として確認済みのマーカーを付け分ける。完了要約は区分ごとの件数に分ける | どちらも成り立てば未対応 0 件で P6 | Step 4、Step 7、Step 10 |
| (追加)スコープ外の処置で、記録先が Issue、note、PR description のどこにも無い | 記録先が見つからないとして未対応 | P4 へ戻る。P4 で note に残せば、次の P5 で確認済みになり得る | Step 5。drive の「判断基準」 |
| (追加)既存実装で満たす、再現しない、guideline と競合、outdated / duplicate の処置 | 根拠が成り立てば対応不要として確認済み(`**対応不要として確認済み(resolve 可)**`)。成り立たなければ未対応 | 未対応でなければ P6 | 表の 3〜6 行目 |
| (追加)「判断に追加情報が必要である」の処置 | 確かめず未対応 | drive はこの処置が出た P4 で止まる(「停止とエスカレーション」)ため、通常は P5 に届かない | 表の最終行。drive の「判断基準」 |
| (追加)top-level の指摘をスコープ外とした | 同じ区分で確かめて完了要約に含める。マーカーは付けない。未対応なら未対応件数に含める | 未対応件数で分岐する | Step 6、Step 7 |
| (参考)PR #233 の Codex の cycle(#234 を起票、PR の差分は未修正) | スコープ外の判断と #234 を確かめれば、スコープ外として確認済み。変更前は未対応 1 件で未収束とされた | 未対応 0 件 | 表の 2 行目 |
| (参考)PR #242 の `[fyi]`(note と PR description に候補、code の変更なし) | スコープ外として確認済み。変更前は `**修正確認済み(resolve 可)**` が付いた | 同上 | 表の 2 行目、`SKILL.md` の「Review event と resolve の制約」 |

### その他

| 項目 | 結果 |
|---|---|
| repo 相対 path の存在(`.agents/skills/review-pull-request/` の `SKILL.md`、`references/verify-comments.md`、`references/address-comments.md`) | 切れなし |
| 参照先の節名(「処置ごとの確認」「Review event と resolve の制約」「verify-comments の担当一致」「組み込み / 汎用 review capability の再利用」「反復の上限」、drive の「判断基準」「working branch note」「返させる出力」「終了時の報告」「停止とエスカレーション」) | すべて存在する |
| 文体(追加行の `(です\|ます)。`) | 該当なし |
| `git diff --check` | 問題なし |
| `.claude/skills/` の symlink | 変えていない。`review-pull-request` と `drive-issue-to-reviewed-pr` の参照先は正しい |
| Go の test | Go のコードを変えないため実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。両 skill とも frontmatter は変えていない。新しい session を開始して確かめる |

## リスク・ブロッカー

- 本 PR の P5 は、subagent が本 PR の `verify-comments.md` を読んで行う(自己適用)。変更後の規定を review の指摘への処置に当てはめる機会になる一方、merge 前の規定で再確認することになる。
- 未検証: description による発火。

## セッションログ

- 2026-09-25: Issue #253 と、PR #233 / PR #242 の再確認の例を読んだ。依存なし。
- 2026-09-25: 作業内容 1〜5 を実施し、Issue の「検証」を実行した。
