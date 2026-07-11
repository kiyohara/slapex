# address-comments モード手順

既存の review comment を検証し、必要な修正・検証・返信を行う。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get_review_comments)`、`pull_request_read(get_reviews)`、`pull_request_read(get_comments)` と、必要に応じて `pull_request_read(get_files / get_diff)` で review context を取得し、可視 metadata から対象の review cycle を特定する。
2. 指摘を分類する: unresolved / resolved / outdated / informational / actionable / conflicting / duplicate。
3. Issue、project の正本、既存実装、関連 PR と照合して各指摘の妥当性を判断する。reviewer 間で見解が異なる場合は、依存関係と project 正本を根拠に判断理由を残す。判断には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 採用する指摘だけを実装し、必要な検証を行う。説明で対応する指摘に無理なコード変更を加えない。
5. local branch が対象 PR の head branch に対応することを確認する。dirty working tree や未 push commit がある場合は、既存のユーザー変更を混ぜず、状態と扱いを明示する。
6. 修正を「対応済み」と返信する前に、その修正が対象 PR の head branch へ push 済みで、GitHub 上の head SHA から確認できることを確かめる。local のみの修正を対応済みとして扱わない。commit / push は local git / SSH で行い、`doc/guidelines/git-operation-guidelines.md` に従う。
7. 確認した各 inline comment に `add_reply_to_pull_request_comment` で必ず処置を返信する。返信には canonical metadata(`Review cycle` は元 review の値)と、採否、判断理由、修正 commit / head SHA、検証結果のうち該当する情報を含める。処置は次のいずれかを明示する。
   - 採用し修正した。
   - 妥当だが今回はスコープ外である。
   - 既存実装ですでに満たしている。
   - 再現しない、または前提が異なる。
   - project guideline と競合するため採用しない。
   - outdated / duplicate である。
   - 判断に追加情報が必要である。
8. 修正担当 Agent 自身は inline thread を resolve しない。元の Review 担当 Agent による `verify-comments` を待つ。
9. 必要に応じて `add_issue_comment` で PR 全体の対応結果と未対応件数を残す。
10. `pull_request_read(get_review_comments / get)` と `pull_request_read(get_check_runs)` で返信、PR state、head SHA、check runs を再取得し、二重投稿や未反映が無いことを確認する。

完了条件: 確認した各 inline comment へ処置を返信し、read-back で反映を確認した時点。resolve は行わない。
