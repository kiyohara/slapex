# 作業ブランチメモ

- ブランチ: `add-ci-read-tools-to-mcp-allowlist`
- PR: #214
- 最終更新: 2026-09-06

## 目的

Issue #213。`github-op-integrated` の tool allowlist に CI の read 3 tool(`actions_get` / `actions_list` / `get_job_logs`)を追加し、CI の状態確認と失敗 log 取得を `gh` 依存から MCP へ移す。あわせて allowlist 変更に伴って事実と食い違うドキュメント(guideline / MCP README / `release` skill)を同期する。

`actions_run_trigger`(workflow の実行・再実行・cancel・log 削除)は意図的に allowlist へ入れない。

## 現在の状況

5 ファイルの編集と Issue 記載の検証が完了。PR #214 作成済み。

- `.config/github-op-integrated.conf.example` — `GITHUB_TOOLS` に read 3 tool を追加(13 → 16)。除外理由を英語コメントで追記。
- `doc/guidelines/github-mcp-guidelines.md` — 適用範囲に CI read を追加、対象外の `workflow dispatch` を具体化、第一選択表に CI read 3 行を追加、「CI 操作の境界」と「PAT に付与する権限」を新設、allowlist 運用の記述を現状へ更新。
- `doc/guidelines/github-cli-guidelines.md` — allowlist 対象外の列挙を具体化し、CI read は MCP 側である旨を明記。
- `.agents/mcp/github-op-integrated/README.md` — PAT 権限に Actions / Checks / Commit statuses の read を追加、`Actions: Write` 非付与を明記、除外リストを `actions_run_trigger` へ具体化。
- `.agents/skills/release/SKILL.md` — L48 の記述を workflow run(MCP)と Release / asset(`gh`)に分割、CI success 確認の第一選択を `actions_list(list_workflow_runs)` へ、`gh run watch` は維持理由を明記。

## 決定事項

- allowlist に足すのは read 3 tool のみ。`actions_run_trigger` は高リスク write として除外し、承認を経ない実行経路を仕組みで塞ぐ。
- check run / commit status の確認は既存 `pull_request_read(get_check_runs / get_status)` で足りるため、tool 追加はしない。
- PAT の権限追加は不要(Actions: read / Checks: read は既に付与済みであることを Issue で確認済み)。
- `release` skill の `gh run watch` は MCP に等価 tool が無く、完了までブロックして待つ挙動をポーリングで置き換えると挙動が変わるため、意図的に `gh` のまま残す。
- Issue の「permission セクション」への追記先は、`doc/guidelines/github-mcp-guidelines.md` に該当セクションが無いため「安全性の担保」配下に「PAT に付与する権限」subsection を新設して対応した。
- Issue が明示していない箇所でも、allowlist 拡張により事実と食い違う「初期 allowlist」「初期 MCP 化スコープ」系の表現は同 PR 内で現状へ揃えた。allowlist の構成を説明する文が矛盾したまま残るのを避けるため。
- `.agents/skills/review-pull-request/SKILL.md` は Issue のスコープ外指定に従い変更しない。同ファイルに残る「workflow dispatch」は、付与しない permission の列挙であり、`Actions: Write` を付与しない本 PR の方針と矛盾しないため据え置く。

## 次にやること

1. ユーザーによる review / merge。

## 検証

Issue 記載の検証をすべて実施した。

### allowlist の反映

wrapper に JSON-RPC を流して `tools/list` を取得した。事前に local の `.config/github-op-integrated.conf`(gitignored、commit 対象外)にも 3 tool を追記済み。

- tool 総数: 16(期待値どおり)
- CI tool: `actions_get` / `actions_list` / `get_job_logs`
- `actions_run_trigger`: 含まれない(False)

### CI tool の実呼び出し

`tools/call` を kiyohara/slapex に対して実行し、いずれも 403 にならず応答した。

- `actions_list(list_workflow_runs)`: `total_count: 440`、3 件取得。
- `actions_get(get_workflow_run)`: run の詳細を取得(`conclusion: success`)。
- `get_job_logs(run_id=..., failed_only=true)`: `total_jobs: 5`、`failed_jobs: 0`。

### ドキュメントの整合

- `git ls-files | xargs grep -n "workflow dispatch"` の残存は `.agents/skills/review-pull-request/SKILL.md` の 1 件のみ。Issue のスコープ外指定に該当し、内容も矛盾しないため据え置いた。
- `release` skill の 3 箇所が更新後の allowlist と矛盾しないことを確認した。

### 未実施

- Go の test / build は実行していない。本 PR は agent 設定とドキュメントのみの変更で、製品コードに触れていないため。

## リスク・ブロッカー

- 各利用者の `.config/github-op-integrated.conf` は gitignored のため、この PR では更新されない。利用者側で `GITHUB_TOOLS` に 3 tool を追記し MCP host を再起動する必要がある旨を PR description で案内する。

## セッションログ

- 2026-09-06: Issue #213 を読み、依存なしを確認。ブランチ作成と note 作成。
- 2026-09-06: 5 ファイルを編集。allowlist 反映 / CI tool 実呼び出し / ドキュメント整合の検証をすべて実施し、期待値どおりであることを確認。
