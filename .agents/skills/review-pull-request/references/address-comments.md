# address-comments モード手順

既存の review comment を検証し、必要な修正・検証・返信を行う。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get_review_comments)`、`pull_request_read(get_reviews)`、`pull_request_read(get_comments)` と、必要に応じて `pull_request_read(get_files / get_diff)` で review context を取得し、可視 metadata から対象の review cycle を特定する。
2. 指摘を分類する: unresolved / resolved / outdated / informational / actionable / conflicting / duplicate。
3. Issue、project の正本、既存実装、関連 PR と照合して各指摘の妥当性を判断する。reviewer 間で見解が異なる場合は、依存関係と project 正本を根拠に判断理由を残す。判断には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 指摘が事実として正しいかを、推論ではなく確認して判断する。設定値や環境の挙動に関する指摘は、実際にコマンドを実行するか一次資料を参照して裏を取る。裏を取らずに採否を決めない。
5. 採用する指摘だけを実装し、必要な検証を行う。説明で対応する指摘に無理なコード変更を加えない。
6. local branch が対象 PR の head branch に対応することを確認する。dirty working tree や未 push commit がある場合は、既存のユーザー変更を混ぜず、状態と扱いを明示する。
7. 修正を「対応済み」と返信する前に、その修正が GitHub 上に反映済みであることを確かめる。修正は次の 2 種に分け、1 つの指摘への修正が両方を含む場合(例: note の commit と PR description の編集)は両方を確かめる。
   - push を伴う修正(commit で repository のファイルを変える修正): 対象 PR の head branch へ push 済みで、GitHub 上の head SHA から確認できること。local のみの修正を対応済みとして扱わない。commit / push は local git / SSH で行い、`doc/guidelines/git-operation-guidelines.md` に従う。
   - push を伴わない修正(PR description、PR の title、Issue の本文など、GitHub 上の編集で直す修正): 編集の後に、編集した対象を read 系 tool(PR description と title は `pull_request_read(get)`、それ以外は `doc/guidelines/github-mcp-guidelines.md` の「操作別の第一選択」の tool)で取り直し、直した内容が GitHub 上の現在の内容にあること。反映を確かめられない編集(write の失敗など。`SKILL.md`「MCP write failure の安全手順」)を対応済みとして扱わない。
8. 確認した各 inline comment に `add_reply_to_pull_request_comment` で必ず処置を返信する。返信には canonical metadata(`Review cycle` は元 review の値)と、採否、判断理由、修正の所在、検証結果のうち該当する情報を含める。修正の所在は、push を伴う修正では修正 commit / head SHA、push を伴わない修正では編集した対象(PR description など)と箇所(節名など)とし、両方を含む修正では両方を書く。`verify-comments` が確かめる対象を返信から特定できるようにするためである。処置は次のいずれかを明示する。
   - 採用し修正した。
   - 妥当だが今回はスコープ外である。
   - 既存実装ですでに満たしている。
   - 再現しない、または前提が異なる。
   - project guideline と競合するため採用しない。
   - outdated / duplicate である。
   - 判断に追加情報が必要である。
9. 修正担当 Agent 自身は inline thread を resolve しない。元の Review 担当 Agent による `verify-comments` を待つ。
10. 必要に応じて `add_issue_comment` で PR 全体の対応結果と未対応件数を残す。top-level の指摘など inline thread の無い指摘への処置をこのコメントで返す場合も、手順 8 の返信と同じ内容(修正の所在を含む)を書く。
11. `pull_request_read(get_review_comments / get)` と `pull_request_read(get_check_runs)` で返信、PR state、head SHA、check runs を再取得し、二重投稿や未反映が無いことを確認する。

完了条件: 確認した各 inline comment へ処置を返信し、read-back で反映を確認した時点。resolve は行わない。
