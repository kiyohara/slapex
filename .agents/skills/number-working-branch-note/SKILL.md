---
name: number-working-branch-note
description: PR 採番直後に working branch note のファイル名へ PR 番号を割り当てる。`working-branch-notes/draft_<branch>*.md` が存在し、対応する PR が OPEN な場合に、note のリネーム・関連参照(note 本文・PR description)の置換・commit・push 確認までを一連の手順で安全に実施する。note の "確定" を意味するものではなく、採番後も note は通常通り更新される前提。
---

# number-working-branch-note

PR を作成した直後に `working-branch-notes/` 配下の `draft_...md` を `<PR-number>_...md` へ採番(リネーム)し、note 本文と PR description に残る旧ファイル名参照を新名へ揃え、commit から push、PR 反映までを安全に完了させるための skill。

この skill は note の "仕上げ" や "確定" を意味するものではない。採番後も note は作業の進行に応じて更新され続ける前提とする。skill の責務は **ファイル名規約の切り替え時点での整合取り** に限られる。

## 適用範囲

このリポジトリで PR を作成した後、ブランチ内に `working-branch-notes/draft_<escaped-branch>*.md` が残っているときの採番処理に限る。次のいずれかに該当する場合は、この skill の対象外として停止し、ユーザーに状況を報告する。

- 現在ブランチが default branch(`main` または `master`)である。
- 対応する PR を `gh pr view` で取得できない。
- PR の `state` が `OPEN` ではない(`MERGED` / `CLOSED` は cleanup PR の領分)。
- PR の `headRefName` が現在ブランチと一致しない。
- `working-branch-notes/draft_<escaped-branch>*.md` が 1 件も存在しない(採番処理不要)。
- escape 後のブランチ名と一致しない無関係な `draft_*.md` が混ざっている(自動判断しない)。

## 参照する正本

実行前および疑問が出た時点で、以下の正本を必ず参照する。

- `doc/guidelines/working-branch-notes-handling.md` — note のファイル名規約、escape ルール、番号付き note と draft の優先関係。
- `doc/guidelines/working-branch-notes-security.md` — push 前の情報統制チェック観点。
- `doc/guidelines/github-cli-guidelines.md` — `gh` 実行時の `op plugin run -- gh ...` 形式と実行環境制約。
- `doc/guidelines/git-operation-guidelines.md` — `git commit` / `git push` (SSH remote) の 1Password SSH agent 連携と実行環境制約。
- `doc/guidelines/pull-request-guidelines.md` — PR title / description の書式(日本語、tool 名なし、既存表現の置換に留める)。

## コマンド形式

- `gh` は原則 `op plugin run -- gh ...` で実行する。`.op/` と `op` コマンドが利用できない場合だけ通常の `gh` にフォールバックする(github-cli-guidelines.md)。
- `git commit` / `git push` は commit signing と SSH agent を伴うため、socket 通信・承認プロンプトが阻害された場合は git-operation-guidelines.md に従い、制約のない実行環境で同じコマンドを再実行する。

## 手順

以下の順で実行する。各ステップで失敗・矛盾を検出したら停止し、ユーザーに報告する。安全側に倒し、判断に迷うときは進めずに確認する。

### 1. 前提チェック

```sh
git branch --show-current
ls working-branch-notes/draft_*.md 2>/dev/null
op plugin run -- gh pr view --json number,title,body,headRefName,state,url
```

- 現在ブランチが `main` または `master` の場合は停止。
- `draft_*.md` が 0 件なら停止(後処理不要であることをユーザーに伝える)。
- `gh pr view` が PR を返さない、`state` が `OPEN` でない、`headRefName` が現在ブランチと一致しない場合は停止。

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
- 「PR 未作成」「PR 作成後に更新」「working branch note が未確定」など、PR 採番前提の stale 表現があれば置換候補を提示し、ユーザー合意を得てから書き換える(機械的に書き換えない)。

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

### 9. push(ユーザー確認後)

push は SSH remote 経由で 1Password SSH agent を使うため、必ずユーザー確認を取ってから実行する。確認のために、これから走らせる正確なコマンドをユーザーに提示する。

```sh
git push
```

SSH 認証失敗・socket 通信エラーなどが出た場合は `git-operation-guidelines.md` に従い、制約のない実行環境で再実行する。

### 10. PR title / description 更新

`op plugin run -- gh pr view --json title,body` の結果に対して次を行う。

- description 内の `draft_<escaped-branch>` 表記(`__suffix` 付き含む)を `<PR-number>_<escaped-branch>` に置換する。これは機械的に置換してよい。Step 5 と同じく、**具体的な escape branch 名を含む参照のみ** を機械的置換の対象とする。汎用 placeholder は触らない。
- 「PR 未作成」「PR 作成後に更新」「working branch note が未確定」などの stale 表現を検出し、置換候補をユーザーに提示する。ユーザー合意後に書き換える。
- title は通常触らない。明確に stale な記述が含まれる場合のみ、置換候補を提示してユーザー合意後に書き換える。
- `doc/guidelines/pull-request-guidelines.md` に従い、日本語維持・tool 名なし・既存表現の置換に留める(新規セクションの追加はしない)。

合意が取れたら、`op plugin run -- gh pr edit <number> --body-file <tmp>` などで反映する。body は必ずファイル経由で渡し、shell エスケープの取りこぼしを避ける。

## 終了時の報告

ユーザーへの最終報告には次を含める。

- rename した note ファイルの一覧(旧名 → 新名)。
- 情報統制チェックで除外・修正した箇所があればその概要。
- 作成した commit のメッセージと SHA(分かれば)。
- push の有無(ユーザー承認の結果)。
- PR description / title への変更点(置換した stale 表現があれば箇条書きで)。

## やらないこと

- note の "確定" や "完成版へのまとめ直し"。採番後も note は更新され続ける前提で、本 skill の責務はファイル名規約切り替え時点の整合取りに限る。
- note 本文の網羅的レビュー(`PR:` 欄の更新、自ファイル名への直接参照の置換、Step 5 / Step 10 で挙げた stale 表現キーワードの検出だけが対象)。
- note の `最終更新:` 欄の自動更新。
- `<PR-number>_...md` が既に存在するときの自動上書き・自動削除。
- 関連しない作業ツリー変更の commit への巻き込み。
- 機械的な stale 表現の自動書き換え(必ずユーザー合意を取る)。
- PR description への新規セクション追加(既存表現の置換のみ)。
- title の積極的な書き換え。
