# 作業ブランチメモ

- ブランチ: `claude/project-thread-jlentp`(cloud session が指定。Issue の推奨ブランチ名は `align-review-comment-prefix-and-verification`)
- PR: #235
- 最終更新: 2026-09-24

## 目的

Issue #216。bizdate の PR #4 で先に入れたレビュー品質の改善 3 点を、`review-pull-request` skill に取り込む。

1. `review` モードの指摘に `.github/copilot-instructions.md` と同じ prefix を付ける。定義は複製せず参照する。
2. `address-comments` に「推論ではなく確認して判断する」手順を足す。
3. CI の確認を review の手順に組み込む。「操作別の第一選択 tool」の表に 1 行、`review` の手順 1 に失敗 job の log の取得を足す。

`drive-issue-to-reviewed-pr` の手順で処理する(P1 は `run-issue-task`)。本 Issue は #222 / #228 / #234 と並行で処理する。`doc/guidelines/issue-driven-task-execution.md` の「前提」(直列に消化する)と decision log 0037 に対する例外であり、ユーザーの明示の指示(2026-09-24、cloud 環境での並列実行のトライアル)による。

## 現在の状況

- P1〜P4 を終えた。P2 の review(cycle `claude-code-f0588a9-20260924131945`)の指摘 1 件を P4 で採用し、`ece8726` で直した。次は P5(再確認)。
- 変更: `.agents/skills/review-pull-request/SKILL.md`(「コメント言語と文体」と「操作別の第一選択 tool」に 1 行ずつ)、`references/address-comments.md`(手順 4 を挿入し、以降を繰り下げ)、`references/review.md`(手順 1 に失敗 job の log の取得)、`doc/guidelines/github-mcp-guidelines.md`(「失敗 job の log 取得」の行。P4)。

## 決定事項

- prefix を付ける対象は `review` モードの指摘に限った。`address-comments` の処置の返信と `verify-comments` の確認結果は指摘ではない。`verify-comments` の resolve 可の返信は、先頭行を resolve 可マーカーにする規定もある。
- prefix の名前 5 つは括弧内に並べ、使い分けと merge への影響は `.github/copilot-instructions.md` を正とした。名前を並べる形は bizdate の同じ行に揃えた。
- 追加した手順は、Issue の文面どおり手順 3 と 4 の間に置いた(新しい手順 4)。以降の番号は 1 つずつ繰り下がる。repo 内に address-comments の手順番号での参照は無い。#234 の Issue 本文が参照する「手順 6」は手順 7 になるため、#234 の thread に共有した。
- `drive-issue-to-reviewed-pr` の「判断基準」にある「指摘の正しさは推論で決めず、実物で確かめて判断する(同 reference の手順)」には、これまで address-comments 側に対応する手順が無かった。追加した手順 4 がその参照先になる。
- `review` の手順 1 では、Issue の例に `return_content=true` を足した。実際に呼び、省くと log の本文ではなく download URL だけが返ることを確かめたためである(「検証」)。run ID は check run の `details_url` から取る。`tail_lines` は既定値のままとし、手順には書かない。
- decision log は作らない。既存の定義(Copilot 用の prefix、allowlist 済みの CI の read tool)を skill に適用するだけで、方針の変更や撤回ではない(PR #225 の判断と同じ)。並行実行で #216 用に空けた 0063 は使わない。
- 0059 は、判断に重要度の表記を使わない理由の 1 つに「review ごとに表記が揃わない(prefix の統一は Issue #216 の範囲)」を挙げている。本 PR で P3 の入力(P2 の出力)の表記は揃い、この理由は弱まる。件数で判断する結論は、`address-comments` が全件に処置を返信する点で変わらない。0059 と `drive-issue-to-reviewed-pr` の文言は変えず、follow-up の候補とする。0059 には #234 が並行して追記している。
- P4 で review の指摘 1 件([imo])を採用し、`doc/guidelines/github-mcp-guidelines.md` の「失敗 job の log 取得」の行を本 PR の記述に揃えた。この行は `return_content` を取得量を絞る option と書いており、本 PR の手順 1 と食い違っていた。`drive-issue-to-reviewed-pr` の「CI の確認点」は log の取得をこの行に委ねるため、スコープ外とせず本 PR で直した。指摘の前提(download URL だけが返ること、cloud session で配信元に届かないこと、`tail_lines` の挙動)は実物で確かめた(「検証」)。
- `progress.md` は索引外の単発 Issue のため更新しない。
- 出力生成系 3 skill は、ドキュメントだけの変更で各 skill の「いつ使うか」に当たらないため適用しない。
- Issue の「スコープ外」(prefix の定義の複製、`review` / `verify-comments` への確認の手順、`actions_run_trigger` の記述、bizdate の他のカスタマイズ)には手を付けていない。

## 次にやること

- PR を作成し、note を採番する。(完了)
- P5〜P6 を `drive-issue-to-reviewed-pr` の手順で進める。

## 検証

2026-09-24、cloud session(project の thread)で実行。

| 項目 | 結果 |
|---|---|
| prefix の名前と順序(`SKILL.md` と `.github/copilot-instructions.md` の表) | 一致(`[must]` / `[ask]` / `[imo]` / `[nits]` / `[fyi]`) |
| prefix の定義を複製していないこと | `SKILL.md` に使い分けや merge への影響の記述は無く、参照と名前だけ(`grep` で 0 件) |
| address-comments の手順番号 | 1〜11 の連番。repo 内に手順番号での参照なし(`git grep`) |
| 追記した tool 名と allowlist | `actions_list` / `actions_get` / `get_job_logs` がすべて `.config/github-op-integrated.conf.example` の `GITHUB_TOOLS` にある |
| `get_job_logs` の実際の挙動(組み込みの GitHub tool、過去の失敗 run 1 件) | `run_id` と `failed_only=true` で失敗 job 1 件を返す。`return_content` 無しでは署名付きの download URL だけ、`return_content=true` で本文が返る。`tail_lines` を小さくすると、末尾の後処理の行だけになる |
| log の配信元の host(cloud session) | proxy が CONNECT を 403 で拒否する(`curl` と proxy の status で確認)。`doc/guidelines/github-cli-guidelines.md` の「cloud session(Claude Code on the web)」の記載と一致 |
| check run の `details_url` | `.../actions/runs/<run_id>/job/<job_id>` の形で、check run の ID は job ID と一致する(PR #233 の check runs) |
| 正本と入口の整理(`doc/guidelines/agent-configuration-management.md`) | prefix の定義を複製せず、`.github/copilot-instructions.md` を参照する形 |
| 文体 | 追記箇所は常体(`(です\|ます)。` で 0 件) |
| `git diff --check` | 問題なし(P4 の修正後も) |
| Go の test | Go のコードを変えないため実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。description は変えていない。新しい session で確かめる |

## リスク・ブロッカー

- 変更した `review-pull-request` を、この PR の review(P2)、対応(P4)、再確認(P5)で使う(自己適用)。subagent には SKILL.md を repo 相対 path で渡し、自己完結性を review の観点に含める。
- 並行実行: #234 は `drive-issue-to-reviewed-pr` と 0059 を変える。本 PR はどちらも触らないため、影響は address-comments の手順番号の参照に限られる見込み。

## セッションログ

- 2026-09-24: Issue #216 を確認した。依存の #213 は完了済み(PR #214)。bizdate PR #4 の実装を参照した。
- 2026-09-24: 作業内容 1〜3 を実施し、Issue の「検証」を実行した。
- 2026-09-24: PR #235 を作成し、note を採番した(P1)。head `f0588a9` の check runs 5 件が success。出力生成系 3 skill は不適用。
- 2026-09-24: P2 の subagent が review した。review cycle `claude-code-f0588a9-20260924131945`、`Reviewed head` `f0588a9`、指摘 1 件(inline 1、top-level 0。`[imo]`)。P3 で P4 へ進んだ。
- 2026-09-24: P4 で指摘 1 件を採用し、`ece8726` で guideline の行を直した。出力生成系 3 skill はドキュメントだけの変更のまま不適用。
