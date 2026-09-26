---
name: review-pull-request
description: slapex の Pull Request を対象に、PR review、review comment 対応、対応結果の再確認を github-op-integrated MCP-first の workflow として実行する。slapex リポジトリで PR をレビューするとき、review comment へ対応・返信するとき、対応結果を再検証して resolve 可否を返信するときに使う。汎用の gh-address-comments 系 skill / plugin と同時に該当する場合も、本 skill と project guideline の tool routing を優先する。
argument-hint: "<review | address-comments | verify-comments> [PR番号 | PR URL]"
---

# review-pull-request

slapex の PR review を、project guideline に沿った github-op-integrated MCP-first workflow として実行する skill。PR 自体の review、既存 review comment への対応、対応結果の再確認と resolve 可否の判定までを 3 モードで扱う。inline thread の resolve 自体は人間が GitHub UI で行う(「Review event と resolve の制約」を参照)。

本 skill は「レビューエンジン」ではなく、slapex 固有の orchestrator / adapter である。各 Agent の組み込み review capability を置き換えない(「組み込み / 汎用 review capability の再利用」を参照)。

`drive-issue-to-reviewed-pr` skill から呼ばれる場合、`review` と `verify-comments` は subagent が、`address-comments` は呼び出し元の orchestrator が実行する。本 skill の手順は変わらず、単独での利用も従来どおり続ける。

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
| `review` | PR 自体をレビューし、指摘があれば inline comment と review body、無ければ PR conversation comment で完了を可視化する | 完了要約 1 本と、指摘がある場合は inline comment を投稿し、read-back で反映を確認した時点 | `references/review.md` |
| `address-comments` | 既存 review comment の妥当性を検証し、必要な修正・検証・返信を行う。inline thread は resolve しない | 確認した各 inline comment へ処置を返信し、read-back で反映を確認した時点 | `references/address-comments.md` |
| `verify-comments` | 元の Review 担当 Agent が対応結果を再検証し、妥当な inline thread に簡潔な resolve 可マーカー付き返信を残す。共通の検証結果は完了要約 1 本へ集約し、resolve は自動実行しない | 全対象 thread への返信と完了要約 1 本を投稿し、read-back で反映を確認した時点 | `references/verify-comments.md` |

## 対象 PR の特定と review source

- PR 番号または PR URL が指定されている場合は、それを対象とする。
- 指定が無い場合は、現在の local branch から `list_pull_requests` / `pull_request_read(get)` で対応する open PR を特定する。
- 次の場合は、未 push の local 変更へ対象を切り替えず、処理を停止してユーザーへ報告する。
  - 対応する open PR が無い。
  - 複数候補があり一意に決まらない。
  - local branch と PR head branch の対応を確認できない。
- PR 番号 / URL を明示指定された場合も `pull_request_read(get)` で state を確認し、closed または merged の場合は処理を停止してユーザーへ報告する。draft PR は review 対象としてよいが、draft であることを完了要約へ記録する。
- review 対象の正本は GitHub 上の PR head SHA と PR diff とする。未 push commit、staged change、working tree change を review 対象へ混ぜない。
- local checkout は実装確認や test に利用してよいが、GitHub 上の head SHA との対応を確認する。
- review または再確認の途中で PR head SHA が変わった場合は、古い diff を前提に投稿せず、必要な context と検証を取り直す。

## Agent 識別と review cycle

- `review` / `verify-comments` モードでは review cycle の完了要約を載せる 1 本に、処理した Agent 自身を識別できる表示と review cycle の可視 metadata を含める。`address-comments` では投稿する各返信・コメントに同じ metadata を含める。GitHub 上の操作 account は単一であり、username では処理主体を区別できないためである。
- Agent 種別は実行環境(system prompt、実行 harness の情報)から確認する。判定できない場合は推測せず、ユーザーに確認する。
- `review` モードの開始時に review cycle ID を作る。session ID、token、local path などの内部情報を review cycle ID や metadata に含めない。

### 可視 metadata の canonical フォーマット

可視 metadata を含める投稿の末尾に次の 5 行を置く。実行環境が投稿の末尾に署名行(footer)を付ける場合、または agent に付けるよう求める場合は、footer の直前に置く。これは複数の agent 実装が書くだけでなく parse して review cycle を突合する前提の正規フォーマットであり、キー名・順序・区切りを次のとおり固定する。

```text
Agent: <Agent 種別>
Model: <model 識別子>
Review cycle: <agent-slug>-<short-head>-<YYYYMMDDHHMMSS>
Reviewed head: <head SHA>
Mode: <review | address-comments | verify-comments>
```

- キーは `Agent`、`Model`、`Review cycle`、`Reviewed head`、`Mode` の 5 つとし、この順序で必須とする。1 行 1 キーとし、5 行を空行を挟まず連続させる。各行はキーで始め、前に記号や空白を付けない。
- 区切りは半角コロン + 半角スペース(`: `)とする。
- 1 つの投稿に置く metadata は 1 組とする。本文で他の投稿の metadata を行ごと引用しない。参照が要る場合は review cycle ID などの値だけを書く。
- parse は投稿内の位置ではなく、5 つのキーがこの順で連続する 5 行を探して行う。後ろに footer が続くかどうかと、footer の内容には依存しない。
- footer を自分で書く場合は、5 行と footer の間に空行を 1 行置く。footer が `---` で始まる場合、直前の行が空行でないと Markdown では `Mode` の行が見出しとして表示されるためである。
- footer を自分で書くよう求められていない場合(cloud session の subagent など)は、5 行で投稿を終え、footer を書かない。実行環境が後から footer を付けた場合は、read-back で 5 行と footer の間に空行があることを確かめ、無ければ下記「投稿前の確認と誤りの訂正」に従って直す。
- `Agent` は処理した Agent 種別の表示名とする(例: `Codex`、`Claude Code`、`Cursor`)。
- `Model` は、その処理で利用した model の識別子とする。実行環境から確認できる値を使い(例: `claude-fable-5`)、確認できない場合は `unknown` とし、推測しない。実行環境の指示が model の識別子の記載を禁じていても、PR と review のコメントがその対象外と確認できる場合は、それを理由に `unknown` にしない。コメントへの記載まで禁じている場合、または対象外と確認できない場合は `unknown` とし、上位の指示で記載を控えた旨を本文に 1 行残す(`doc/guidelines/pull-request-guidelines.md` の「Tool 名と model の識別子の扱い」)。`Model` は記録目的の参考情報であり、review cycle の突合や `verify-comments` の担当一致判定には使わない。同一 Agent 種別でも session により model が変わり得るためである。確認の手段は下記「`Model` の確認手段」に従う。
- `Review cycle` の値は `<agent-slug>-<short-head>-<YYYYMMDDHHMMSS>` とする。`<agent-slug>` は Review 担当 Agent 種別の小文字 kebab-case(例: `codex`、`claude-code`、`cursor`)、`<short-head>` は review 開始時点の head SHA 先頭 7 文字、`<YYYYMMDDHHMMSS>` は review 開始時刻(UTC、秒まで)とする(例: `codex-1a2b3c4-20260711103045`)。同一 Agent・同一 head の再レビューや再試行で cycle ID が衝突しないよう、秒までを含める。
- `Reviewed head` は、そのコメントの投稿時点で当該 Agent が確認した full head SHA とする。
- `Mode` は投稿を行ったモード名とする。
- `address-comments` の返信・コメントと `verify-comments` の完了要約では、`Review cycle` に対象 review cycle の ID(元 review の値)をそのまま使い、新しい ID を作らない。同じ cycle を追跡可能に保つためである。
- HTML comment などの非表示 marker は取得経路によって欠落し得るため、併用してよいが、非表示 marker だけに依存しない。

### `Model` の確認手段

`Model` の値は、投稿する Agent 自身が実行環境から確かめる。他の Agent(orchestrator など)から伝えられた値を確かめずに写さない。委譲する側も `Model` の値を指示しない。

| 投稿する Agent | 確認手段 | 書く値 |
| --- | --- | --- |
| cloud session(Claude Code on the web)の session 本体 | 投稿の直前に claude-code-remote MCP の `get_session` を `session_id` を省略して呼ぶ | `external_metadata.last_served_model`。無い場合は `session_context.model` |
| subagent(cloud session を含む) | 自身の system prompt が示す model の識別子 | exact model ID |
| 上記以外(ローカルの Claude Code、Codex、Cursor など) | system prompt など実行環境が示す model の識別子 | exact model ID |

- cloud session で `last_served_model` を使うのは、実際に応答した model を表すためである。設定された model(`session_context.model`、`configured_model`)とは、fallback などで異なり得る。
- subagent は `get_session` の値を使わない。`get_session` が示すのは session の model であり、subagent が別の model で起動されている場合がある。
- 書くのは API で使う形の識別子(exact model ID。上記 `Model` の項の例と同じ形)とし、表示名は書かない。投稿者によって表記が分かれると、記録として揃わないためである。実行環境が表示名しか示さない場合は、確認できないものとして扱う。
- 同じ理由から、context window を示す接尾辞(`[1m]` など角括弧の部分)は除いて書く。`session_context.model` や system prompt の識別子には付くことがあり、`last_served_model` には付かない。
- どの手段でも値を確認できない場合は `unknown` とする。session ID など `get_session` の他の値は書かない。
- 委譲元(orchestrator など)が従う上位の指示の内容が、委譲の brief で事実として渡された場合は、自身の実行環境の指示と同じく上記 `Model` の項に当てはめる。渡されるのは指示の内容だけであり、値は自分で確かめる。
- 上記 `Model` の項により上位の指示で記載を控える場合は、確かめた値を書かず `unknown` とする。

### 投稿前の確認と誤りの訂正

metadata を含む投稿の前に、次を確かめる。

- 5 行がキーの順に連続し、末尾(footer が付く場合はその直前)にあること。
- `Agent` と `Model` が、それぞれ上記の手段で確かめた値であること。`get_session` を使う場合は、投稿の直前に呼んだ値であること。
- `Review cycle` が、`review` では新しく作った ID、それ以外では元 review の ID と一致すること。
- `Reviewed head` が、投稿の直前に `pull_request_read(get)` で取り直した head SHA と一致すること。
- `Mode` が、実行中のモードであること。

投稿後の read-back では、5 行が崩れずに残っていることも確かめる。

誤りに気付いた場合は、訂正のための新しい投稿をせず、その投稿を編集する。1 投稿 1 組と、完了要約 1 本を保つためである。編集してよいのは、同じ Agent 種別が投稿したと `Agent` 行で確認できる投稿に限る。編集は本文全体の置き換えになるため、読み戻した本文の metadata の部分(5 行の値と並び、5 行と footer の間の空行)だけを直し、footer を含む他の部分は変えない。編集後は read-back で反映を確かめる。

| 対象 | 編集の手段 |
| --- | --- |
| PR conversation comment(完了要約など) | `gh api -X PATCH repos/{owner}/{repo}/issues/comments/{id}` |
| inline comment とその返信 | `gh api -X PATCH repos/{owner}/{repo}/pulls/comments/{id}` |
| 提出済み review の本文 | `gh api -X PUT repos/{owner}/{repo}/pulls/{number}/reviews/{id}` |

- 編集の tool は `github-op-integrated` の allowlist に無いため、`gh` を使う。cloud session の組み込み tool には編集の tool(`update_issue_comment`)も見えるが、allowlist 外のため使わない(`doc/guidelines/github-mcp-guidelines.md` の「cloud session(Claude Code on the web)」と「操作別の第一選択」)。`gh` の実行形式は `doc/guidelines/github-cli-guidelines.md` に従う。
- 次の場合は編集せず、削除と再投稿もしない。対象の URL、誤っている箇所、正しい値をユーザーへ報告し、GitHub の UI での編集を依頼する。
  - `gh` が使えない(cloud session で導入されていない、など)。agent は `gh` の導入を試みない。
  - 誤りが `Agent` 行そのものにある、または 5 行が崩れていて、同じ Agent 種別の投稿だと `Agent` 行で確認できない。
- 報告は working branch note にも残す。note を書くのは PR branch に push できる Agent(`drive-issue-to-reviewed-pr` では orchestrator)である。subagent は note を書かず push もせず、報告を呼び出し元へ返す。

### verify-comments の担当一致

- `verify-comments` は、原則として現在の Agent と可視 metadata 上の Review 担当 Agent(review cycle ID の `<agent-slug>` と元 review コメントの `Agent`)が一致する review cycle だけを対象とする。GitHub username の一致だけを根拠にしない。
- 異なる Agent が代理確認する場合は、ユーザーの明示指示を必要とし、代理確認である事実を返信へ記録する。

## コメント言語と文体

- 本 skill が投稿するすべての GitHub コメント(review body、inline comment、返信、PR conversation comment)は日本語で書く。
- 文体は `doc/guidelines/document-style-guidelines.md` の開発者向け文体(常体)に合わせる。
- `review` モードの指摘には、`.github/copilot-instructions.md` の「原則」にある prefix(`[must]` / `[ask]` / `[imo]` / `[nits]` / `[fyi]`)を付ける。使い分けは同ファイルを正とし、定義をここに複製しない。同じ PR に並ぶ Copilot と AI agent の指摘で語彙を揃え、merge 前に直すべき指摘かを投稿者によらず同じ基準で読めるようにするためである。

## 組み込み / 汎用 review capability の再利用

- 本 skill は、各 Agent の native / built-in review capability(例: Claude Code の `code-review` / `review` skill、Codex の review mode)や、利用可能な汎用 review skill / plugin を置き換えない。
- 各モードで、利用可能な review capability を diff 分析、指摘の分類、妥当性判断、修正内容の再検証に利用してよい。
- 本 skill が専有する責務は次のとおり: 対象 PR と head SHA の確定、slapex の正本との照合、github-op-integrated MCP-first の tool routing、review cycle 管理、Agent 識別とコメント形式、コメント投稿、read-back、resolve 可否の判定、完了条件。
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
| Review thread の resolve | 自動実行しない。`verify-comments` は resolve 可マーカー付き返信までを行い、resolve は人間が GitHub UI で行う |
| PR conversation comment | `add_issue_comment`(PR 番号を `issue_number` として渡す) |
| Check runs の確認 | `pull_request_read(get_check_runs)` |
| CI の workflow run / job / log の確認 | `actions_list` / `actions_get` / `get_job_logs` |

- 上記 tool はすべて現行 `.config/github-op-integrated.conf.example` の allowlist に含まれる。本 skill のための tool allowlist 追加は不要である。

## Review event と resolve の制約

- GitHub 上の操作 account は単一であるため、本 skill は `APPROVE` と `REQUEST_CHANGES` を自動実行しない。GitHub 側が self-review を拒否するかどうかに依存しない、本 skill の禁止事項とする。
- PR review の投稿は `COMMENT` event に限定する。
- 本 skill は inline thread の resolve を自動実行しない。MCP tool は `resolve_thread` を提供し、`gh api graphql` でも GraphQL mutation `resolveReviewThread` を送信できるため、これは MCP tool や `gh` command の機能制約ではない。fine-grained PAT では `Pull requests` permission に加えて `Contents: Read and Write` を要求するが(公式 documentation に記載は無く、community で確認されている挙動)、本プロジェクトは `Contents: write` を付与しない方針であるため、resolve は人間が GitHub UI で行う。
- `verify-comments` で対応結果を妥当と確認した inline thread への返信は、本文の先頭行を、再確認の区分(`references/verify-comments.md` の「処置ごとの確認」)に対応する次の resolve 可マーカーとする。これらを resolve 可マーカーの canonical 形式とし、人間はマーカーの付いた thread を目視確認して手動で resolve する。
  - 修正確認済み: `**修正確認済み(resolve 可)**`
  - スコープ外として確認済み: `**スコープ外として確認済み(resolve 可)**`。人間は resolve の前に、返信にある follow-up の記録先を確かめる。
  - 対応不要として確認済み: `**対応不要として確認済み(resolve 可)**`
- マーカーは resolve 相当と確認できた返信だけに付け、未解決・対応不十分の返信には付けない。修正を伴わない処置を確かめた返信に `**修正確認済み(resolve 可)**` を付けない。
- resolve 可マーカーを付けてよいのは、現在の Agent が Review 担当として作成した review cycle に属する inline thread に限る。人間、他の Agent、または他の review cycle が作成した thread には付けない。
- `unresolve_thread`、review の dismiss、PR の merge、reviewer request の変更は本 skill から自動実行しない。
- `pull_request_review_write` は tool 単位では `APPROVE` / `REQUEST_CHANGES` / pending review / `resolve_thread` 操作も提供する。allowlist に含まれることを実行の許可根拠とせず、本節の method / event 制約に従う。

## permission

- fine-grained PAT は repository を slapex に限定し、少なくとも Pull requests の read / write を許可する。review 作成、inline reply、conversation comment はこの範囲で実行できる。
- thread resolve(`resolveReviewThread`)は Pull requests permission だけでは実行できず、fine-grained PAT では `Contents: Read and Write` が必要である。本プロジェクトはこれを付与しないため、MCP tool / `gh` の機能の有無にかかわらず resolve は自動実行の対象外とする(「Review event と resolve の制約」)。
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
