# review モード手順

PR 自体をレビューし、指摘と review 完了コメントを投稿する。`SKILL.md` の共通 workflow(対象 PR の特定、Agent 識別と review cycle、コメント言語、capability 再利用、event / resolve の制約)を前提とする。

1. `pull_request_read(get)` で PR metadata と base / head SHA を、`pull_request_read(get_diff / get_files)` で変更内容を、`pull_request_read(get_review_comments / get_reviews / get_comments)` で既存 review を、`pull_request_read(get_check_runs)` で check runs を取得する。
2. 関連 Issue、project の正本(`doc/guidelines/` / `doc/design/`)、既存実装、working branch note、PR description と照合して review scope を確定する。
3. correctness、regression、security、test、document / process 整合性を、変更内容に応じて確認する。分析には `SKILL.md`「組み込み / 汎用 review capability の再利用」に従い、利用可能な review capability を活用してよい。
4. 指摘がある場合は、`pull_request_review_write(create)` で pending review を作り、`add_comment_to_pending_review` で可能な限り inline comment としてまとめ、`pull_request_review_write(submit_pending)` の `COMMENT` event で投稿する。`APPROVE` / `REQUEST_CHANGES` は使わない。
5. 指摘の有無にかかわらず、`add_issue_comment` で review 完了コメントを必ず残す。完了コメントには canonical metadata(`Agent` / `Model` / `Review cycle` / `Reviewed head` / `Mode`)に加えて、指摘件数、実施した検証、未実施事項を含める。指摘が無い場合も「指摘なし」「問題なし」と明記し、review 完了を可視化する。対象が draft PR の場合はその旨も記録する。
6. `pull_request_read(get_reviews / get_review_comments / get_comments)` で投稿結果を read-back し、部分反映・二重投稿が無いことを確認する。あわせて `pull_request_read(get)` で head SHA が review 時点から更新されていないことを確認する。更新されていた場合は `SKILL.md`「対象 PR の特定と review source」に従い、context と検証を取り直す。

完了条件: 指摘(あれば)と review 完了コメントを投稿し、read-back で反映を確認した時点。
