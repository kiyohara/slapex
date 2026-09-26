# 作業ブランチメモ

- ブランチ: `claude/project-thread-ffymtu`(cloud session が指定。Issue の推奨ブランチ名は `cloud-subagent-tool-routing`)
- PR: #258
- 最終更新: 2026-09-26

## 目的

Issue #255(RV-03)。cloud session で `review-pull-request` を実行する subagent が、組み込みの GitHub tool で足りる read に `gh api` を使う経路を塞ぐ。`review-pull-request` の tool routing の正本に `doc/guidelines/github-mcp-guidelines.md` の「cloud session(Claude Code on the web)」を加え、`drive-issue-to-reviewed-pr` の brief に、cloud session で実行していることと同節を事実として添える。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 2 件目である。「進行中タスク: review cycle の skill 改善」の表で #253 の次の優先度にあり、cloud session で処理する以後の Issue の P2 / P5 すべてに効くため選んだ。

## 現在の状況

- 作業内容 1・2 を実施し、Issue の「検証」のうち、SKILL.md を読んで確かめる項目と、path・節名・文体・`git diff --check` を実行した。PR #258 を draft で作成し、note を採番して、`progress.md` の RV-03 の PR 欄を反映した(P1)。
- P2 / P5 の subagent が使った GitHub tool の記録は、P2 / P5 の後に行う。

## 決定事項

- 作業内容 1: `review-pull-request` の SKILL.md の冒頭の tool routing の正本に「cloud session(Claude Code on the web)」を加えた。あわせて、cloud session では `github-op-integrated` が起動しないことを理由に `gh` へ fallback せず、本 skill の `github-op-integrated` の記載を組み込みの GitHub tool に読み替えることを 1 文で書いた。挙げられた節だけを頼りにすると「MCP が使えないので `gh` へ fallback する」と読める(Issue の背景)ため、節名を足すだけでなく、その読み方をしないことを明示した。組み込み tool に無い操作だけを `gh` で補うことなど、規定の中身は複製せず、同節への参照に委ねた。
- 「事前確認」の `doc/guidelines/github-mcp-guidelines.md` の項に、cloud session では同節も読むことを足した(作業内容 1 の L29)。
- 「事前確認」の後の「GitHub 操作の最初の試行先は常に github-op-integrated MCP tool とする」の文は変えない。冒頭の読み替えが、skill 内の `github-op-integrated` の記載すべてに当たるためである。
- 作業内容 2(未決事項 1): 仮決定のとおり、brief に足す。`drive-issue-to-reviewed-pr` の「渡す入力」の箇条に、cloud session で実行している場合は、その事実と、GitHub 操作の tool routing に同節が当たることを brief に事実として添える項を足し、brief の形に `実行環境:` の行を足した。
  - 「実行環境の指示」の行と同じく、条件付きの行とした(cloud session でなければ省く)。「渡す入力」の表は、すべての brief に含める項目の表であるため、入れない。
  - 使う tool の指示としてではなく、subagent が同節を当てはめる実行環境の情報として渡す。`review-pull-request` の参照先と同じ節を指すため、brief の項目と参照先は食い違わない(Issue の完了条件)。
  - cloud session の判定は `doc/guidelines/cloud-session-guidelines.md`(`CLAUDE_CODE_REMOTE=true`)に委ねた。
- decision log は作らない。#255 には作成・追記の指示が無い。未決事項 1 の採否と理由は、Issue、本 note、PR description に残る。同じ表の #253(PR #256)も作っていない。
- 出力生成系 3 skill(`update-sample-exports`、`update-readme-preview-screenshots`、`update-readme-demo-gif`)は適用しない。変更は `.agents/skills/`、`progress.md`、本 note だけで、各 skill の「いつ使うか」に当たらない。
- `progress.md` は RV-03 の行を done(PR merge後)にし、PR 欄に #258 を記入した。

## 次にやること

- PR を作成し、note を採番して、`progress.md` の RV-03 の PR 欄を反映する(P1)。(完了)
- P2 の subagent に review を委譲する。P2 / P5 の subagent に使った GitHub tool を報告させ、「検証」に記録する(Issue の「検証」の 2 つ目)。

## 検証

2026-09-26、cloud session(project の thread)で実行。

### cloud session の subagent の立場で読んだ第一選択

変更後の `review-pull-request` の SKILL.md を、cloud session の subagent の立場で読み、read と投稿のそれぞれで第一選択の tool が決まることを確かめた。

| 操作 | 第一選択 | 根拠 |
|---|---|---|
| PR の特定・取得、diff / files、review thread / review / comment、check runs の取得(read) | 組み込みの `list_pull_requests` / `pull_request_read` | 冒頭の読み替えにより、「操作別の第一選択 tool」の表の tool 名を組み込み tool として読む(guideline の「cloud session」の「tool 名はそのまま読み替え」) |
| CI の workflow run / job / log の取得(read) | 組み込みの `actions_list` / `actions_get` / `get_job_logs` | 同上 |
| review の投稿、inline comment への返信、PR conversation comment(投稿) | 組み込みの `pull_request_review_write` / `add_comment_to_pending_review` / `add_reply_to_pull_request_comment` / `add_issue_comment` | 同上 |
| 投稿の metadata の訂正(本文の編集) | `gh api` | 「投稿前の確認と誤りの訂正」。guideline の「cloud session」の例外(`update_issue_comment` は allowlist 外)と一致する |
| `github-op-integrated` の起動失敗を見たとき | `gh` へ fallback しない | 冒頭の読み替え。guideline の「cloud session」の起動失敗の扱い(想定どおりで、診断や再接続の依頼をしない)と一致する |

### その他

| 項目 | 結果 |
|---|---|
| repo 相対 path(`doc/guidelines/github-mcp-guidelines.md`、`doc/guidelines/cloud-session-guidelines.md`) | 存在する |
| 参照先の節名(「MCP 優先・`gh` fallback」「cloud session(Claude Code on the web)」「汎用 skill / plugin と競合する場合」「操作別の第一選択」、`review-pull-request` の「`Model` の確認手段」など既存の参照) | すべて存在する |
| 文体(追加行の `(です\|ます)。`) | 該当なし |
| `git diff --check` | 問題なし |
| Go の test | Go のコードを変えないため、ローカルでは実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。両 skill とも frontmatter は変えていない。新しい session を開始して確かめる |

### P2 / P5 の subagent が使った GitHub tool

P2 / P5 の後に記録する。

## リスク・ブロッカー

- 未検証: description による発火。
- 本 PR の P2 / P5 は、変更後の brief の形(`実行環境:` の行を含む)で委譲する。subagent が組み込みの GitHub tool だけを使っても、それが SKILL.md の記述によるのか brief の行によるのかは切り分けられない。

## セッションログ

- 2026-09-26: #253(PR #256)の merge を確かめ、次の Issue に #255 を選んだ。作業内容 1・2 を実施し、Issue の「検証」を実行した。
- 2026-09-26: PR #258 を作成し、note を採番して、`progress.md` の PR 欄を反映した(P1)。検証は上記のとおりで、出力生成系 3 skill は不適用。
