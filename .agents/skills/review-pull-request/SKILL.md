---
name: review-pull-request
description: slapex の Pull Request を対象に、PR review、review comment 対応、対応結果の再確認を github-op-integrated MCP-first の workflow として実行する。slapex リポジトリで PR をレビューするとき、review comment へ対応・返信するとき、対応結果を再検証して inline thread を resolve するときに使う。汎用の gh-address-comments 系 skill / plugin と同時に該当する場合も、本 skill と project guideline の tool routing を優先する。
---

# review-pull-request

slapex の PR review を、project guideline に沿った github-op-integrated MCP-first workflow として実行する skill。PR 自体の review、既存 review comment への対応、対応結果の再確認と inline thread の resolve までを 3 モードで扱う。

本 skill は「レビューエンジン」ではなく、slapex 固有の orchestrator / adapter である。各 Agent の組み込み review capability を置き換えない(「組み込み / 汎用 review capability の再利用」を参照)。

tool routing の正本は `doc/guidelines/github-mcp-guidelines.md` の「MCP 優先・`gh` fallback」「汎用 skill / plugin と競合する場合」「操作別の第一選択」とする。汎用 skill、plugin、user-level skill(例: `gh-address-comments`)が GitHub app や `gh` を第一選択としていても、slapex では本 skill と project guideline の tool routing を優先する。

## 入力

- モード: `review` / `address-comments` / `verify-comments` のいずれか。
- 対象 PR: PR 番号またはこのリポジトリの PR URL(省略可)。

処理開始時に必ずモードを 1 つに確定する。ユーザー指示・引数からモードを確定できない場合は、処理を始めずユーザーに確認する。モード確定後、該当モードの reference だけを読む。

## 事前確認

処理を始める前に、必要な範囲で次を読む。

- `AGENTS.md`
- `doc/guidelines/github-mcp-guidelines.md` — 優先規則と操作別の第一選択の正本。
- `doc/guidelines/github-cli-guidelines.md` — `gh` fallback のルール。
- `doc/guidelines/git-operation-guidelines.md` — 修正の commit / push を伴う場合。
- `doc/guidelines/pull-request-guidelines.md`
- `doc/guidelines/working-branch-notes-handling.md`
- `doc/guidelines/working-branch-notes-security.md`

`gh auth status` / `gh pr view` などの `gh` preflight を MCP 試行より先に実行しない。GitHub 操作の最初の試行先は常に github-op-integrated MCP tool とする。

## モード

| モード | 責務 | 完了条件 | 手順 |
| --- | --- | --- | --- |
| `review` | PR 自体をレビューし、指摘(あれば inline comment)と review 完了コメントを投稿する | 指摘の有無にかかわらず review 完了コメントを投稿し、read-back で反映を確認した時点 | `references/review.md` |
| `address-comments` | 既存 review comment の妥当性を検証し、必要な修正・検証・返信を行う。inline thread は resolve しない | 確認した各 inline comment へ処置を返信し、read-back で反映を確認した時点 | `references/address-comments.md` |
| `verify-comments` | 元の Review 担当 Agent が対応結果を再検証し、確認返信と妥当な inline thread の resolve を行う | 全対象 thread を確認し、再確認結果コメントを投稿した時点 | `references/verify-comments.md` |

## 対象 PR の特定と review source

- PR 番号または PR URL が指定されている場合は、それを対象とする。
- 指定が無い場合は、現在の local branch から `list_pull_requests` / `pull_request_read(get)` で対応する open PR を特定する。
- 次の場合は、未 push の local 変更へ対象を切り替えず、処理を停止してユーザーへ報告する。
  - 対応する open PR が無い。
  - 複数候補があり一意に決まらない。
  - local branch と PR head branch の対応を確認できない。
- PR 番号 / URL を明示指定された場合も `pull_request_read(get)` で state を確認し、closed または merged の場合は処理を停止してユーザーへ報告する。draft PR は review 対象としてよいが、draft であることを review 完了コメントへ記録する。
- review 対象の正本は GitHub 上の PR head SHA と PR diff とする。未 push commit、staged change、working tree change を review 対象へ混ぜない。
- local checkout は実装確認や test に利用してよいが、GitHub 上の head SHA との対応を確認する。
- review または再確認の途中で PR head SHA が変わった場合は、古い diff を前提に投稿せず、必要な context と検証を取り直す。

## Agent 識別と review cycle

- 本 skill が投稿するすべての GitHub コメント(review、inline comment、返信、完了コメント)に、処理した Agent 自身を識別できる表示と review cycle の可視 metadata を含める。GitHub 上の操作 account は単一であり、username では処理主体を区別できないためである。
- Agent 種別は実行環境(system prompt、実行 harness の情報)から確認する。判定できない場合は推測せず、ユーザーに確認する。
- `review` モードの開始時に review cycle ID を作る。session ID、token、local path などの内部情報を review cycle ID や metadata に含めない。

### 可視 metadata の canonical フォーマット

コメント末尾に次の 4 行を置く。これは複数の agent 実装が書くだけでなく parse して review cycle を突合する前提の正規フォーマットであり、キー名・順序・区切りを次のとおり固定する。

```text
Agent: <Agent 種別>
Model: <model 識別子>
Review cycle: <agent-slug>-<short-head>-<YYYYMMDDHHMMSS>
Reviewed head: <head SHA>
Mode: <review | address-comments | verify-comments>
```

- キーは `Agent`、`Model`、`Review cycle`、`Reviewed head`、`Mode` の 5 つとし、この順序で必須とする。1 行 1 キーとする。
- 区切りは半角コロン + 半角スペース(`: `)とする。
- `Agent` は処理した Agent 種別の表示名とする(例: `Codex`、`Claude Code`、`Cursor`)。
- `Model` は、その処理で利用した model の識別子とする。実行環境から確認できる値を使い(例: `claude-fable-5`)、確認できない場合は `unknown` とし、推測しない。`Model` は記録目的の参考情報であり、review cycle の突合や `verify-comments` の担当一致判定には使わない。同一 Agent 種別でも session により model が変わり得るためである。
- `Review cycle` の値は `<agent-slug>-<short-head>-<YYYYMMDDHHMMSS>` とする。`<agent-slug>` は Review 担当 Agent 種別の小文字 kebab-case(例: `codex`、`claude-code`、`cursor`)、`<short-head>` は review 開始時点の head SHA 先頭 7 文字、`<YYYYMMDDHHMMSS>` は review 開始時刻(UTC、秒まで)とする(例: `codex-1a2b3c4-20260711103045`)。同一 Agent・同一 head の再レビューや再試行で cycle ID が衝突しないよう、秒までを含める。
- `Reviewed head` は、そのコメントの投稿時点で当該 Agent が確認した full head SHA とする。
- `Mode` は投稿を行ったモード名とする。
- `address-comments` / `verify-comments` の返信・コメントでは、`Review cycle` に対象 review cycle の ID(元 review の値)をそのまま使い、新しい ID を作らない。同じ cycle を追跡可能に保つためである。
- HTML comment などの非表示 marker は取得経路によって欠落し得るため、併用してよいが、非表示 marker だけに依存しない。

### verify-comments の担当一致

- `verify-comments` は、原則として現在の Agent と可視 metadata 上の Review 担当 Agent(review cycle ID の `<agent-slug>` と元 review コメントの `Agent`)が一致する review cycle だけを対象とする。GitHub username の一致だけを根拠にしない。
- 異なる Agent が代理確認する場合は、ユーザーの明示指示を必要とし、代理確認である事実を返信へ記録する。

## コメント言語と文体

- 本 skill が投稿するすべての GitHub コメント(review、inline comment、返信、完了コメント)は日本語で書く。
- 文体は `doc/guidelines/document-style-guidelines.md` の開発者向け文体(常体)に合わせる。

## 組み込み / 汎用 review capability の再利用

- 本 skill は、各 Agent の native / built-in review capability(例: Claude Code の `code-review` / `review` skill、Codex の review mode)や、利用可能な汎用 review skill / plugin を置き換えない。
- 各モードで、利用可能な review capability を diff 分析、指摘の分類、妥当性判断、修正内容の再検証に利用してよい。
- 本 skill が専有する責務は次のとおり: 対象 PR と head SHA の確定、slapex の正本との照合、github-op-integrated MCP-first の tool routing、review cycle 管理、Agent 識別とコメント形式、コメント投稿、read-back、resolve、完了条件。
- 汎用 capability が GitHub app / `gh`-first の取得・投稿手順、独自の approve / request changes、独自の完了条件を持つ場合、その部分は採用せず、本 skill と project guideline で置き換える。GitHub への直接投稿 option(inline comment 自動投稿など)は使わず、findings の生成までにとどめ、投稿は本 skill の github-op-integrated 経路へ一元化する。
- 課金や cloud 実行を伴う review 機能は、ユーザーの明示指示なしに起動しない。
- 汎用 capability が利用できない環境でも処理を停止せず、Agent 自身の review 能力で同じ完了条件を満たす。
- 汎用 capability の出力は補助的な分析結果として扱い、最終的な指摘、返信、resolve の妥当性は現在の Agent が slapex の Issue・正本・実装に照らして確認する。

## 操作別の第一選択 tool

第一選択は `doc/guidelines/github-mcp-guidelines.md` の「操作別の第一選択」と同期する。本 skill で使う操作を抜粋する。

| 操作 | 第一選択 |
| --- | --- |
| PR の特定・取得 | `list_pull_requests` / `pull_request_read(get)` |
| PR diff / files の取得 | `pull_request_read(get_diff / get_files)` |
| Review thread / review / comment の取得 | `pull_request_read(get_review_comments / get_reviews / get_comments)` |
| PR review の投稿 | `pull_request_review_write(create / submit_pending)` / `add_comment_to_pending_review` |
| Inline review comment への返信 | `add_reply_to_pull_request_comment` |
| Review thread の resolve | `pull_request_review_write(resolve_thread)` |
| PR conversation comment | `add_issue_comment`(PR 番号を `issue_number` として渡す) |
| Check runs の確認 | `pull_request_read(get_check_runs)` |

- 上記 tool はすべて現行 `.config/github-op-integrated.conf.example` の allowlist に含まれる。本 skill のための tool allowlist 追加は不要である。
- inline thread の resolve には、`pull_request_read(get_review_comments)` response の `PRRT_...` 形式 thread node ID を使う。response に ID が無い場合は `doc/guidelines/github-mcp-guidelines.md` の操作表と fallback 規則に従い、別の MCP read method での取得可否を先に確認する。情報不足だけを理由に黙って `gh` を先行させない。

## Review event と resolve の制約

- GitHub 上の操作 account は単一であるため、本 skill は `APPROVE` と `REQUEST_CHANGES` を自動実行しない。GitHub 側が self-review を拒否するかどうかに依存しない、本 skill の禁止事項とする。
- PR review の投稿は `COMMENT` event に限定する。
- review state に対して Agent が自動実行できる完了操作は、`verify-comments` で妥当性を確認した inline thread の `pull_request_review_write(resolve_thread)` に限定する。
- resolve を自動実行してよいのは、現在の Agent が Review 担当として作成した review cycle に属する inline thread に限る。人間、他の Agent、または他の review cycle が作成した thread は resolve しない。
- `unresolve_thread`、review の dismiss、PR の merge、reviewer request の変更は本 skill から自動実行しない。
- `pull_request_review_write` は tool 単位では `APPROVE` / `REQUEST_CHANGES` / pending review 操作も提供する。allowlist に含まれることを実行の許可根拠とせず、本節の method / event 制約に従う。

## permission

- fine-grained PAT は repository を slapex に限定し、少なくとも Pull requests の read / write を許可する。review 作成、inline reply、thread resolve はこの範囲で実行する。
- `Contents: write`、merge、release、workflow dispatch、repository settings などの追加 permission は本 skill のために付与しない。
- 修正の commit / push は local git / SSH で行い、`doc/guidelines/git-operation-guidelines.md` に従う。

## MCP write failure の安全手順

write 系 MCP tool が失敗した場合は、次の順で扱う。

1. 直ちに `gh` で同じ write を再実行しない。
2. read 系 tool(`pull_request_read` など)で部分反映・重複の有無を確認する。
3. MCP 未反映で、かつ `doc/guidelines/github-mcp-guidelines.md` の fallback 条件(セッション途中の切断時手順を含む)を満たす場合だけ、`doc/guidelines/github-cli-guidelines.md` に従って `gh` へ fallback する。
4. fallback する場合は、試した MCP tool、失敗内容、未反映確認の結果、実行する command をユーザーへ明示する。

## 反復の上限

- `address-comments` と `verify-comments` の反復は、同一 review cycle につき 2 周を上限とする。
- 2 周で収束しない指摘が残る場合は自動反復を打ち切り、未収束の指摘、見解の相違点、推奨する次の対応を整理してユーザーへエスカレーションする。
