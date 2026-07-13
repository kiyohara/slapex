# review モード手順

PR 自体をレビューし、1 review cycle につき 1 本の完了要約を投稿する。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get)` で PR metadata と base / head SHA を、`pull_request_read(get_diff / get_files)` で変更内容を、`pull_request_read(get_review_comments / get_reviews / get_comments)` で既存 review を、`pull_request_read(get_check_runs)` で check runs を取得する。
2. 関連 Issue、project の正本(`doc/guidelines/` / `doc/design/`)、既存実装、working branch note、PR description と照合して review scope を確定する。
3. correctness、regression、security、test、document / process 整合性を、変更内容に応じて確認する。分析には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 指摘がある場合は、`pull_request_review_write(create)` で pending review を作り、`add_comment_to_pending_review` で可能な限り inline comment としてまとめ、`pull_request_review_write(submit_pending)` の `COMMENT` event で投稿する。review body をその review cycle の唯一の完了要約とし、指摘件数、実施した検証、未実施事項、canonical metadata(`Agent` / `Model` / `Review cycle` / `Reviewed head` / `Mode`)を含める。指摘要約は 1 行程度に留め、inline 本文を全文コピーしない。この場合、同趣旨の `add_issue_comment` は投稿しない。`APPROVE` / `REQUEST_CHANGES` は使わない。
5. 指摘が無い場合は pending review を作らず、`add_issue_comment` でその review cycle の唯一の完了要約を投稿する。「指摘なし」または「問題なし」、実施した検証、未実施事項、canonical metadata を含める。対象が draft PR の場合はその旨も記録する。
6. review body と PR conversation comment に同趣旨の完了要約を重複投稿しない。完了要約は Step 4 または Step 5 のどちらか一方だけに置く。
7. `pull_request_read(get_reviews / get_review_comments / get_comments)` で投稿結果を read-back し、完了要約が 1 本だけであることと部分反映が無いことを確認する。あわせて `pull_request_read(get)` で head SHA が review 時点から更新されていないことを確認する。更新されていた場合は `SKILL.md`「対象 PR の特定と review source」に従い、context と検証を取り直す。

完了条件: 完了要約 1 本と、指摘がある場合は inline comment を投稿し、read-back で反映を確認した時点。
