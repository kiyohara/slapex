# verify-comments モード手順

元の Review 担当 Agent が対応結果を再検証し、確認結果を返信して、妥当な inline thread に resolve 可マーカーを残す。resolve 自体は人間が GitHub UI で行う。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get_review_comments / get_reviews / get_comments)` で元の review comment と実装担当 Agent の返信を、可視 metadata から元 review 時点の head SHA(`Reviewed head`)を、`pull_request_read(get)` で現在の head SHA を、`pull_request_read(get_diff / get_files)` で修正差分を、`pull_request_read(get_check_runs)` で check runs を取得する。
2. 現在の Agent が元の Review 担当 Agent / review cycle と一致することを可視 metadata で確認する。GitHub username の一致だけを根拠にしない。一致しない場合は `SKILL.md`「verify-comments の担当一致」に従い、ユーザーの明示指示が無ければ処理を停止する。
3. 各指摘について、返信内容だけで判断せず、現在の PR diff、実装、関連 test / document、実行した検証を確認して対応結果の妥当性を再評価する。再検証には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 妥当と判断した inline thread には、`add_reply_to_pull_request_comment` で確認済み返信を残す。返信本文の先頭行は `SKILL.md`「Review event と resolve の制約」の resolve 可マーカー `**修正確認済み(resolve 可)**` とし、その thread 固有の確認結果を短く続ける。共通の head SHA、check runs、件数、canonical metadata は返信ごとに繰り返さず、Step 7 の完了要約へ集約する。マーカーを付けてよいのは、現在の Agent が Review 担当として作成した review cycle の inline thread に限る。
5. 対応が不十分、未 push、検証不足、または新たな問題がある場合は、その thread 固有の理由と必要な追加対応だけを簡潔に返信し、thread は unresolved のまま残す。この返信には resolve 可マーカーを付けない。
6. top-level comment の指摘など inline thread が無い指摘は、確認結果を Step 7 の完了要約へ含める。同じ結果を個別の PR conversation comment と完了要約へ重複投稿しない。
7. 全対象 thread の確認後、`add_issue_comment` でその verify cycle の完了要約を 1 本だけ投稿する。確認済み件数(resolve 可とした thread の URL 一覧を含む)、未対応件数、top-level comment の確認結果、現在の head SHA、check runs、次の action、canonical metadata(`Review cycle` は元 review の値、`Reviewed head` は確認した head SHA)を含める。thread 返信本文は全文コピーせず、一覧は結果を識別できる短い表現に留める。人間はこの要約を起点に手動 resolve する。
8. `pull_request_read(get_review_comments / get_comments)` で thread 返信と完了要約を read-back し、未反映や同趣旨の重複投稿が無いことを確認する。inline thread の resolve は自動実行しない。
9. 未対応が残る場合は `address-comments` へ戻し、修正・返信・再確認を反復する。反復は `SKILL.md`「反復の上限」に従い、同一 review cycle につき 2 周を上限とし、収束しない場合はユーザーへエスカレーションする。
10. すべての指摘が resolve 可または対応不要と確認できた場合だけ、review cycle を完了扱いにする。GitHub 上の thread resolve 操作の完了は人間の作業に委ね、cycle 完了の判定は resolve 可マーカーと完了要約を基準とする。

完了条件: 全対象 thread への簡潔な返信と完了要約 1 本を投稿し、read-back で反映を確認した時点。resolve は自動実行せず、人間が GitHub UI で行う。
