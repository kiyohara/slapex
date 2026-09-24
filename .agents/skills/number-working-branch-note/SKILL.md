---
name: number-working-branch-note
description: PR 採番直後に working branch note のファイル名へ PR 番号を割り当てる。`working-branch-notes/draft_<branch>*.md` が存在し、対応する PR が OPEN な場合に、note のリネーム・関連参照(note 本文・PR description)の置換・commit・push までを一連の手順で安全に実施する。note の "確定" を意味するものではなく、採番後も note は通常通り更新される前提。
---

# number-working-branch-note

PR を作成した直後に `working-branch-notes/` 配下の `draft_...md` を `<PR-number>_...md` へ採番(リネーム)し、note 本文と PR description に残る旧ファイル名参照を新名へ揃え、commit から push、PR 反映までを安全に完了させるための skill。

この skill は note の "仕上げ" や "確定" を意味するものではない。採番後も note は作業の進行に応じて更新され続ける前提とする。skill の責務は **ファイル名規約の切り替え時点での整合取り** に限られる。

## 適用範囲

このリポジトリで PR を作成した後、ブランチ内に `working-branch-notes/draft_<escaped-branch>*.md` が残っているときの採番処理に限る。次のいずれかに該当する場合は、この skill の対象外として停止し、ユーザーに状況を報告する。

- 現在ブランチが default branch(`main` または `master`)である。
- 対応する PR を `github-op-integrated` MCP tool で取得できない。
- PR の `state` が `OPEN` ではない(`MERGED` / `CLOSED` は cleanup PR の領分)。
- PR の `headRefName` が現在ブランチと一致しない。
- `working-branch-notes/draft_<escaped-branch>*.md` が 1 件も存在しない(採番処理不要)。
- escape 後のブランチ名と一致しない無関係な `draft_*.md` が混ざっている(自動判断しない)。

## 参照する正本

実行前および疑問が出た時点で、以下の正本を必ず参照する。

- `doc/guidelines/working-branch-notes-handling.md` — note のファイル名規約、escape ルール、番号付き note と draft の優先関係。
- `doc/guidelines/working-branch-notes-security.md` — push 前の情報統制チェック観点。
- `doc/guidelines/github-mcp-guidelines.md` — GitHub 操作で `github-op-integrated` MCP tool を優先する方針。
- `doc/guidelines/github-cli-guidelines.md` — MCP fallback として `gh` を実行する場合の `op plugin run -- gh ...` 形式と実行環境制約。
- `doc/guidelines/git-operation-guidelines.md` — `git commit` / `git push` (SSH remote) の 1Password SSH agent 連携と実行環境制約。
- `doc/guidelines/pull-request-guidelines.md` — PR title / description の書式(日本語、title に tool 名と model の識別子なし、既存表現の置換に留める)。

## GitHub 操作形式

- PR の read / update は、必ず最初に `github-op-integrated` MCP tool を試す。
- 必要な MCP tool が現在の tools に見えていない場合は、`gh` へ進む前に利用中 agent の tool discovery 機構(利用可能なら `tool_search`)で `github-op-integrated` を検索する。
- `gh pr view`、`gh pr edit`、`gh auth status` などの `gh` preflight を、MCP tool の試行より先に実行しない。
- `gh` は、MCP tool が利用できない、または MCP tool で対象操作を完結できない場合の fallback としてのみ使う。fallback 時は `doc/guidelines/github-cli-guidelines.md` に従い、`.op/` と `op` コマンドが利用できる場合は `op plugin run -- gh ...` を使う。
- write 系(`update_pull_request` など)を `gh` に fallback する場合は、`doc/guidelines/github-mcp-guidelines.md` の write fallback 注意に従い、再実行前に read 系 tool で対象の現状(未反映かどうか)を確認してから実行する。
- `git commit` / `git push` は commit signing と SSH agent を伴うため、socket 通信・承認プロンプトが阻害された場合は git-operation-guidelines.md に従い、制約のない実行環境で同じコマンドを再実行する。

## 手順

以下の順で実行する。各ステップで失敗・矛盾を検出したら停止し、ユーザーに報告する。安全側に倒し、判断に迷うときは進めずに確認する。ただし、「stale 表現の定型置換」の対象(状況を説明する stale 表現、本 skill の実行で完了するタスク行、title の stale な記述)で判断に迷う行は例外とし、処理を止めず、合意も求めず、触らずに「終了時の報告」へ回す。この例外は定型置換の対象の行に限り、前提チェック、対象 note の特定、番号付き note との衝突、Step 5 / Step 10 のファイル名参照の判別、情報統制チェック、commit 対象の限定で止まる扱いには及ばない。

### stale 表現の定型置換

Step 5 と Step 10 で列挙済みの stale 表現は、次の定型で置換する。検出対象は 2 種類ある。

#### 1. 状況を説明する stale 表現

| 置換前 | 置換後 |
| --- | --- |
| `PR 未作成` | `PR #<PR-number> 作成済み` |
| `PR 作成後に更新` | `PR #<PR-number> に更新済み` |
| `working branch note が未確定` | `working branch note を採番済み` |

採番は note の確定を意味しないため、`working branch note が確定済み` など、確定を含意する表現へ置換してはならない。

#### 2. 本 skill の実行で完了するタスク行

`## 次にやること` など未完タスクを列挙するセクションにある、本 skill の実行で完了する行を対象とする。例:「PR 作成」「採番後に note rename」「PR 作成後に note を `<PR-number>_...md` へ rename する」。

- `draft_` を含まず `<PR-number>_...` や `<PR 番号>_...` のような placeholder で書かれた行も対象とする。Step 5 / Step 10 のファイル名置換は `draft_<escaped-branch>` にしか反応しないため、この形式の行は置換だけでは処理されない。placeholder が本 note 自身の採番を指すのか、汎用 placeholder(skill 仕様や handling.md からの引用・例示)なのかを判別できない行は、完了したかどうかを判断できない行として扱い(「共通の扱い」)、「終了時の報告」の「触らずに残した行の一覧」には理由「汎用 placeholder か判別できない」で列挙する。
- Step 5 でファイル名だけを新名へ機械置換すると、「これから採番する」という文意の行が新名のまま残り、自己矛盾する。置換した行が本項の対象に該当しないかを必ず確認する。
- 書き換え形式は note の記法に合わせる。

| note の記法 | 書き換え |
| --- | --- |
| checkbox(`- [ ]`) | `- [x]` にする |
| checkbox でない箇条書き・文 | 行末に `(完了)` を付ける |

行は削除しない。note は作業ログであり、どのタスクが採番時点で完了済みだったかを残すためである。

本 skill の実行では完了しないタスク(review 対応、merge 待ち、後続 Issue への着手など)は対象外とする。`progress.md` の PR 番号反映もこれに含まれる。Step 7 が commit 対象を `working-branch-notes/` 配下に限定しているとおり、本 skill は `progress.md` を更新しない(索引表の更新は `doc/guidelines/issue-driven-task-execution.md` の手順で行う)。

完了として扱うのは、本 skill の前提として実行前に満たされている要素(PR 作成)と、本 skill の実行そのもので達成される要素(note の rename、`PR:` 欄の記入、note 本文と PR description の note 参照の更新、それらの commit と push)に限る。その達成を確認・記録する作業(例:「採番で人間の手番が入らないことを確認する」「採番結果を検証欄に記録する」)は、本 skill の外側が終了後に行うため対象外とする。

1 行に本 skill で完了する要素と完了しない要素が混在する行(例:「PR を作成し、採番後に note rename と `progress.md` の PR 番号反映を行う」)は、行全体を完了扱いにしない。未完了の部分が隠れるためである。この種の行は触らず、終了時に報告する。

#### 共通の扱い

定型置換後に文法や文脈が不自然になる場合、および本 skill の実行で完了したかどうかを判断できない行は、曖昧な表現として触らず、終了時に報告する。

本 skill で「終了時に報告する」は、「終了時の報告」節の項目に列挙することを指す。「stale 表現の定型置換」と Step 5 / Step 10 で触らずに残した行は「触らずに残した行の一覧」に理由とともに列挙し、定型に従って書き換えた行は「書き換えた行の一覧」に列挙する。報告先はこの 2 項目に集約する。

### 1. 前提チェック

```sh
git branch --show-current
ls working-branch-notes/draft_*.md 2>/dev/null
```

そのうえで `github-op-integrated` MCP tool の `list_pull_requests` を使い、現在 branch を `head` に持つ open PR を取得する。現在の tool 一覧に `github-op-integrated` が無い場合は、先に利用中 agent の tool discovery 機構(利用可能なら `tool_search`)で MCP tool を探す。

- 現在ブランチが `main` または `master` の場合は停止。
- `draft_*.md` が 0 件なら停止(後処理不要であることをユーザーに伝える)。
- MCP tool が PR を返さない、`state` が `OPEN` でない、`headRefName` が現在ブランチと一致しない場合は停止。

### 2. 対象 note の特定

- 現在ブランチ名を `working-branch-notes-handling.md` の escape ルールで変換する(`/`, `:`, `\`, `*`, `?`, `"`, `<`, `>`, `|` と空白・制御文字を `-` に置換し、連続 `-` をまとめ、先頭末尾 `-` を削る。空なら `branch`)。
- 対象は `working-branch-notes/draft_<escaped-branch>.md` と `working-branch-notes/draft_<escaped-branch>__*.md` の全件。
- escape branch 名と一致しない `draft_*.md` が混ざっていれば停止し、ユーザーに切り分けを求める。

### 3. 既存番号付き note との衝突確認

- 対応する `working-branch-notes/<PR-number>_<escaped-branch>.md` または `working-branch-notes/<PR-number>_<escaped-branch>__*.md` が既に存在する場合は停止する。
- `working-branch-notes-handling.md` の方針(番号付きを正、draft は移行漏れ)に従い、上書きや自動削除はしない。ユーザーに統合方針を確認する。

### 4. rename

draft note を `<PR-number>_` 接頭辞に揃えて rename する。主 note と suffix 付きを全件処理する。

```sh
git mv working-branch-notes/draft_<escaped-branch>.md \
       working-branch-notes/<PR-number>_<escaped-branch>.md
# suffix 付きがあれば同様に
git mv working-branch-notes/draft_<escaped-branch>__<suffix>.md \
       working-branch-notes/<PR-number>_<escaped-branch>__<suffix>.md
```

### 5. note 本文更新

各 note について次を更新する。`最終更新:` 欄は触らない(handling.md「最終仕様書ではない / 1:1 整合は要求しない」と整合させ、運用コストを増やさないため)。

- 先頭メタ行の `- PR:` 欄が空、または `#` 番号や URL が無い場合に、`#<PR-number>`(または PR URL)を入れる。既に正しい値があれば変更しない。
- 本文中の `draft_<escaped-branch>` 表記(自ファイル名含む参照)を `<PR-number>_<escaped-branch>` に置換する。`__suffix` 付きも同様に置換する。置換対象は **具体的な escape branch 名を含む参照** に限る。`draft_...md` / `<PR-number>_...md` のような汎用 placeholder(skill 仕様や handling.md からの引用・例示)は対象外。判別が難しい場合はユーザーに確認する。
- 「stale 表現の定型置換」の 2 種類(状況を説明する stale 表現 / 本 skill の実行で完了するタスク行)を検出し、同節の定型に従って機械的に書き換える。列挙パターンに明確に当てはまらない曖昧な表現は触らず、終了時に報告する。
- 書き換えた行は「終了時の報告」の「書き換えた行の一覧」に、触らなかった行は「触らずに残した行の一覧」に列挙する(「stale 表現の定型置換」の「共通の扱い」)。

編集が終わったら、対象 note を `git add <path>` で再 stage する。Step 4 の `git mv` は rename 時点の内容しか index に載せないため、本文編集分を改めて stage しないと Step 8 の commit に含まれない。

### 6. 情報統制チェック

`working-branch-notes-security.md` の項目に沿って、rename 後の note 全体を確認する。

- `password`、`secret`、`token`、`cookie`、`session`、`PRIVATE KEY` に続く実値が無いか。
- 長いランダム文字列や署名値に見える文字列が無いか。
- URL に認証情報、署名、token、個人情報、顧客固有情報が含まれていないか。
- ログや問い合わせ文を必要以上に貼っていないか。

該当が見つかった場合は実値を placeholder に置き換えてからでないと commit に進まない。

### 7. commit 対象の限定

```sh
git status
git diff --cached --name-status
```

- staged 変更が `working-branch-notes/` 配下の rename と note 本文更新だけであることを確認する。
- 対象 note に unstaged の変更が残っていないことを確認する(Step 5 の `git add` 漏れ検出)。残っている場合は再 stage するか、意図的に外す対象であればユーザーに確認する。
- 他の作業ツリー変更が混入している場合は、`git restore --staged` で外すか、ユーザーに切り分けを求める。
- 巻き込んで commit しない。

### 8. commit

- 変更が無ければ commit を作らない。ユーザーに「rename / 本文更新ともに不要だった」旨を伝えて終了。
- 変更がある場合は次のメッセージで commit する。

```sh
git commit -m "Number working branch note for PR #<number>"
```

`git commit` は commit signing を伴うため、署名失敗・1Password 承認プロンプト不達などが起きた場合は `git-operation-guidelines.md` に従い、制約のない実行環境で同じコマンドを再実行する。

### 9. push

commit 成功後、`doc/guidelines/git-operation-guidelines.md` に従って push を実行する。

```sh
git push
```

SSH 認証失敗・socket 通信エラーなどが出た場合は `git-operation-guidelines.md` に従い、制約のない実行環境で再実行する。

### 10. PR title / description 更新

前提チェックで取得した PR title / description に対して次を行う。PR title / description を再取得する必要がある場合も、最初に `github-op-integrated` MCP tool を使う。

- description 内の `draft_<escaped-branch>` 表記(`__suffix` 付き含む)を `<PR-number>_<escaped-branch>` に置換する。これは機械的に置換してよい。Step 5 と同じく、**具体的な escape branch 名を含む参照のみ** を機械的置換の対象とする。汎用 placeholder は触らない。
- 「stale 表現の定型置換」の 2 種類を Step 5 と同じ定型で機械的に書き換える。列挙パターンに明確に当てはまらない曖昧な表現は触らず、終了時に報告する。
- title は通常触らない。列挙済みのパターンに明確に当てはまる stale な記述だけを機械的に書き換え、曖昧な場合は触らず終了時に報告する。
- 書き換えた行(ファイル名参照の置換を含む)と触らなかった行は、title を含めて Step 5 と同じく「終了時の報告」の「書き換えた行の一覧」と「触らずに残した行の一覧」に列挙する。
- `doc/guidelines/pull-request-guidelines.md` に従い、日本語維持・既存表現の置換に留める(新規セクションの追加はしない)。

定型置換が終わったら、`github-op-integrated` MCP tool の `update_pull_request` で反映する。MCP tool が利用できず `gh` に fallback する場合だけ、`op plugin run -- gh pr edit <number> --body-file <tmp>` などを使う。body は必ずファイル経由で渡し、shell エスケープの取りこぼしを避ける。

## 終了時の報告

ユーザーへの最終報告には次を含める。

- rename した note ファイルの一覧(旧名 → 新名)。
- 情報統制チェックで除外・修正した箇所があればその概要。
- 作成した commit のメッセージと SHA(分かれば)。
- push の成否。
- 書き換えた行の一覧。「stale 表現の定型置換」に従って書き換えた行を、対象(note のファイル名 / PR description / title)ごとに書き換え前 → 書き換え後の形で示し、状況を説明する stale 表現と完了タスク行を区別する。PR description と title の変更は PR diff に現れないため、Step 10 のファイル名参照の置換もここに含める。
- 触らずに残した行の一覧。「stale 表現の定型置換」の対象のうち触らずに残した行を、対象(note のファイル名 / PR description / title)ごとに示し、理由(定型に当てはまらない / 複合行 / 完了したか判断できない / 置換後の文脈が不自然 / 汎用 placeholder か判別できない)を添える。

最後の 2 項目は、該当が無い場合も省かず「なし」と書く。合意を求めずに書き換える代わりに置く確認経路であり、報告を受け取る側(ユーザー、本 skill を呼ぶ上位 skill)が書き換えの有無を報告だけで判別できるようにするためである。

## やらないこと

- note の "確定" や "完成版へのまとめ直し"。採番後も note は更新され続ける前提で、本 skill の責務はファイル名規約切り替え時点の整合取りに限る。
- note 本文の網羅的レビュー(`PR:` 欄の更新、自ファイル名への直接参照の置換、Step 5 / Step 10 で挙げた stale 表現と完了タスク行の検出だけが対象)。
- note の `最終更新:` 欄の自動更新。
- `<PR-number>_...md` が既に存在するときの自動上書き・自動削除。
- 関連しない作業ツリー変更の commit への巻き込み。
- 列挙パターンに明確に当てはまらない stale 表現や title の推測による書き換え(触らず終了時に報告する)。
- 機械判定できる stale 表現と完了タスク行の書き換えで、合意を求めて処理を止めること。
- 「stale 表現の定型置換」の対象に判断できない行があることを理由に、採番全体を止めること(触らず、終了時に報告する)。
- PR description への新規セクション追加(既存表現の置換のみ)。
- title の積極的な書き換え。
