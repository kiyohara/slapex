# verify-comments モード手順

元の Review 担当 Agent が対応結果を再検証し、確認結果を返信して、妥当な inline thread に resolve 可マーカーを残す。resolve 自体は人間が GitHub UI で行う。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get_review_comments / get_reviews / get_comments)` で元の review comment と実装担当 Agent の返信を、可視 metadata から元 review 時点の head SHA(`Reviewed head`)を、`pull_request_read(get)` で現在の head SHA を、`pull_request_read(get_diff / get_files)` で修正差分を、`pull_request_read(get_check_runs)` で check runs を取得する。
2. 現在の Agent が元の Review 担当 Agent / review cycle と一致することを可視 metadata で確認する。GitHub username の一致だけを根拠にしない。一致しない場合は `SKILL.md`「verify-comments の担当一致」に従い、ユーザーの明示指示が無ければ処理を停止する。
3. 各指摘について、返信内容だけで判断せず、現在の PR diff、実装、関連 test / document、実行した検証を確認して対応結果の妥当性を再評価する。確かめることと結果の区分は、返信が示す処置に応じて下記「処置ごとの確認」に従う。再検証には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 妥当と判断した inline thread には、`add_reply_to_pull_request_comment` で確認済み返信を残す。返信本文の先頭行は、「処置ごとの確認」の区分に対応する `SKILL.md`「Review event と resolve の制約」の resolve 可マーカー(修正確認済みなら `**修正確認済み(resolve 可)**`)とし、その thread 固有の確認結果を短く続ける。スコープ外として確認済みの返信には、確かめた follow-up の記録先を書く。共通の head SHA、check runs、件数、canonical metadata は返信ごとに繰り返さず、Step 7 の完了要約へ集約する。マーカーを付けてよいのは、現在の Agent が Review 担当として作成した review cycle の inline thread に限る。
5. 対応が不十分、未 push、検証不足、処置の根拠が成り立たない、または新たな問題がある場合は、その thread 固有の理由と必要な追加対応だけを簡潔に返信し、thread は unresolved のまま残す。この返信には resolve 可マーカーを付けない。スコープ外とした指摘を本 PR で直すべきと判断した場合と、follow-up の記録先が見つからない場合は、処置の根拠が成り立たないものとして扱う。
6. top-level comment の指摘など inline thread が無い指摘も、「処置ごとの確認」に従って区分を決め、確認結果を Step 7 の完了要約へ含める。マーカーは付けない。同じ結果を個別の PR conversation comment と完了要約へ重複投稿しない。
7. 全対象 thread の確認後、`add_issue_comment` でその verify cycle の完了要約を 1 本だけ投稿する。区分ごとの確認済み件数(inline thread は resolve 可とした thread の URL 一覧を、スコープ外として確認済みの指摘は follow-up の記録先を添える)、未対応件数(top-level の指摘を含む)、top-level comment の確認結果、現在の head SHA、check runs、次の action、canonical metadata(`Review cycle` は元 review の値、`Reviewed head` は確認した head SHA)を含める。thread 返信本文は全文コピーせず、一覧は結果を識別できる短い表現に留める。人間はこの要約を起点に手動 resolve する。
8. `pull_request_read(get_review_comments / get_comments)` で thread 返信と完了要約を read-back し、未反映や同趣旨の重複投稿が無いことを確認する。inline thread の resolve は自動実行しない。
9. 未対応が残る場合は `address-comments` へ戻し、修正・返信・再確認を反復する。反復は `SKILL.md`「反復の上限」に従い、同一 review cycle につき 2 周を上限とし、収束しない場合はユーザーへエスカレーションする。
10. すべての指摘が修正確認済み、スコープ外として確認済み、対応不要として確認済みのいずれかの区分に当たる(未対応が 0 件である)場合だけ、review cycle を完了扱いにする。GitHub 上の thread resolve 操作の完了は人間の作業に委ね、cycle 完了の判定は resolve 可マーカーと完了要約を基準とする。

完了条件: 全対象 thread への簡潔な返信と完了要約 1 本を投稿し、read-back で反映を確認した時点。resolve は自動実行せず、人間が GitHub UI で行う。

## 処置ごとの確認

各指摘の処置は、その指摘への `address-comments` の返信のうち、最新のものが示す処置(`references/address-comments.md` で返信に明示する処置)とする。処置ごとに確かめることと、確かめたことが成り立つ場合の区分を次のとおりとする。

| 処置 | 確かめること | 成り立つ場合の区分 |
| --- | --- | --- |
| 採用し修正した | 修正が対象 PR の head に push 済みで、現在の差分、実装、関連 test / document、実行した検証から、指摘が解消していること | 修正確認済み |
| 妥当だが今回はスコープ外である | スコープ外とする判断が、PR の目的と関連 Issue の完了条件に照らして妥当であること。follow-up の記録先(起票済みの Issue、または working branch note か PR description の follow-up 候補)があること | スコープ外として確認済み |
| 既存実装ですでに満たしている | 返信が示す実装が現在の head にあり、指摘の求めることを満たしていること | 対応不要として確認済み |
| 再現しない、または前提が異なる | 返信が示す再現の条件と結果、または前提の違いが、現在の head で成り立つこと | 対応不要として確認済み |
| project guideline と競合するため採用しない | 返信が示す guideline の規定があり、指摘に従うとその規定に反すること | 対応不要として確認済み |
| outdated / duplicate である | outdated では、指摘の対象が現在の head で無くなったか変わり、指摘が当たらないこと。duplicate では、同じ内容の指摘を扱う thread があること | 対応不要として確認済み |
| 判断に追加情報が必要である | 確かめない。処置が改められるまで未対応とする | 未対応 |

- 確かめたことが成り立たない指摘と、処置の返信が無い指摘は、未対応とする(Step 5)。
- 未対応以外の 3 区分は、inline thread では resolve 可とし、区分に対応するマーカーを付ける。未対応件数には数えない。
- 対応不要として確認済みは、修正を伴わない処置のうち、スコープ外と「判断に追加情報が必要である」を除く処置について、根拠が成り立つと確かめた指摘を指す。スコープ外として確認済みも本 PR で修正しない点は同じだが、resolve の前に follow-up の記録先を確かめる thread だと読めるよう、区分を分ける。
