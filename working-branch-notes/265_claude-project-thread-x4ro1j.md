# 作業ブランチメモ

- ブランチ: `claude/project-thread-x4ro1j`(cloud session が指定。Issue の推奨ブランチ名は `fix-stale-tool-behavior-in-guidelines`)
- PR: #265
- 最終更新: 2026-09-26

## 目的

Issue #215。guideline の記述のうち、外部仕様の変化で実態と合わなくなった 4 件(Cursor の rule の拡張子、Copilot code review と AGENTS.md、instruction file の文字数の上限、commit 署名の実行経路)を、Issue に記載された一次資料に合わせて直す。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 9 件目である。引き継ぎの第一候補だった #246 は、前提となる Slack の payload を確かめる Slack API の文書(`docs.slack.dev`、`api.slack.com`)に environment の network 設定で接続できず、cloud session で確かめられなかったため後に回した。本 Issue は agent 設定の正本の誤りを直すため後の作業の判断にも効き(選定基準 1)、文書だけの修正で cloud session で完結する(選定基準 2)。

## 現在の状況

- 作業内容 1〜4 を実施し、Issue の「検証」を実行した。PR #265 作成済み。
- PR #265 を draft で作成し、note を採番した(P1)。`progress.md` は索引に無い単発 Issue のため更新しない。
- review cycle `claude-code-bd5f05b-20260926135509` の review(P2)で指摘 2 件(`[imo]` 1、`[nits]` 1)を受け、2 件とも採用して直した(P4)。再確認(P5)で修正確認済み 2 件、未対応 0 件になり、review cycle は完了した。人間の手番(thread の resolve、Codex のクロスレビュー、Ready for review、merge)だけが残る。

## 決定事項

- 項目 1(Cursor): `agent-configuration-management.md` の表と `AGENTS.md` の `*.{md,mdc}` を `*.mdc` にした。`.md` が無視される理由(`description` / `globs` / `alwaysApply` を指定する frontmatter が無い)は、正本の表にだけ 1 文で添えた。`AGENTS.md` は index のため理由を複製しない。
- 項目 2(Copilot と AGENTS.md): 表の AGENTS.md 欄を「GitHub.com の code review は AGENTS.md も読む(VS Code などの IDE の code review は読まない)」と環境で分けた。帰結の箇条書き、「AI 向け rule 管理」の `.github/copilot-instructions.md` の項、`AGENTS.md` の Copilot の項は、「リンク先の正本まで辿る保証は無い」を理由に、要点を Copilot 用ファイルに直接書く結論を維持した(Issue の「結論は変えなくてよい」)。表現は bizdate の対応(bizdate PR #2 以降の同じ正本)に揃えた。「正本と入口の整理」の例外の項に残っていた「リンクを辿れない」も、P4 で同じ書き方に揃えた(review の `[nits]`。Issue の検証の grep は「辿らない」だけを探すため掛からなかった。bizdate のこの項は「辿れない」のまま)。
- 項目 3(文字数の上限): 上限を前提とした記述を落とし、「分量ではなくシグナルの濃さで絞る」趣旨にした。帰結の箇条書きにだけ、上限が 2026-06-12 に撤廃された事実を 1 文残した。古い知識で上限を書き戻す事故を防ぐためである。「AI 向け rule 管理」の項では、path 別の観点を `.github/instructions/*.instructions.md` に置く理由を、文字数から `applyTo:` で対象 path を絞れることに改めた。
- 項目 4(署名の実行経路): `git-operation-guidelines.md` の「1Password 連携が必要な操作」に「署名の実行経路」の小節を足し、切り分けを `git config --get gpg.ssh.program` から始める形にして、1Password の signer を使う場合と標準の ssh-keygen を使う場合の 2 経路に分けた。前者では `ssh-add -l` を根拠に判断しないことと、`SSH_AUTH_SOCK` を上書きする必要が無いことを書いた。後者には、`~/.ssh/config` の `IdentityAgent` が `ssh-keygen -Y sign` に効かないことを 1 文添えた(bizdate の対応と同じ)。3 つの経路をまとめていた段落(旧 L24)は、SSH remote と署名の経路を分けて書き直した。失敗時に制約のない実行環境で再実行する方針は変えていない(方針の見直しは #229 の担当)。
- Issue の対象行に無い一致の扱い: 検証の grep で、Issue の対象行のほかに `AGENTS.md` の 2 行(Copilot の項とその下の文字数の項)と `.github/copilot-instructions.md` の 1 行(文字数の上限を前提とする文)が見つかった。`AGENTS.md` は項目 2 と 3 と同じ誤りのため直した。文字数の子項目は、当初は「分量ではなくシグナルの濃さで絞る」に書き換えたが、上限を前提とする内容だけで、書き換えると正本の帰結と同じ文を入口に複製することになるため、P4 で削った(review の `[imo]`)。`.github/copilot-instructions.md` は、Issue がスコープ外に「内容変更(文字数制限が無くなっただけで、現在の内容を増やす必要は無い)」を挙げている。理由の書き方から、スコープ外は内容を増やすことを指すと読み、同じ誤りの訂正に当たる前提の句だけを削った(内容は増やしていない)。
- decision log 0022 の本文(「リンク先正本を辿らない」)と `working-branch-notes/1_setup_discussion_base__agent-review.md` は、当時の記録のため変えない(`doc/guidelines/decision-log-guidelines.md`、`doc/guidelines/working-branch-notes-handling.md`)。
- decision log: 方針(要点を instruction file に直接書く、署名の失敗時は制約のない実行環境で再実行する)を変えず、事実の記述を直すだけのため、記録しない。
- `progress.md`: 本 Issue は索引に無い単発 Issue のため更新しない。
- 出力生成系 3 skill(`update-sample-exports`、`update-readme-preview-screenshots`、`update-readme-demo-gif`)は適用しない。変更は `doc/guidelines/`、`AGENTS.md`、`.github/copilot-instructions.md`、本 note だけで、各 skill の「いつ使うか」に当たらない。

## 次にやること

- draft PR を作成し、note を採番する。(完了)
- review(P2)と指摘への対応(P4)。(完了)
- 再確認(P5)。(完了)
- ユーザー: resolve 可の thread 2 件の resolve、Codex のクロスレビュー、Ready for review、merge。

## 検証

2026-09-26、cloud session(project の thread)で実行。

| 項目 | 結果 |
|---|---|
| `git ls-files \| xargs rg -n "4,000 文字\|\{md,mdc\}\|辿らない"`(Issue の検証) | 本 note を除き 2 件。decision log 0022 の L44(当時の記録で、リンク先を辿らないという結論は Issue も維持する)と、`working-branch-notes/1_setup_discussion_base__agent-review.md` の L76(PR #1 の当時の記録)である。取りこぼしではない |
| 表記の揺れ(`git ls-files \| xargs rg -n "リンクを辿れ"`) | P4 の後、本 note を除いて 0 件 |
| 「各 tool の loading 機構」の時点表記 | `2026-09 時点` に更新した。Issue の一次資料の確認(2026-09-06)と同じ月である |
| 一次資料の記述との照合(4 件) | Issue に記載された一次資料の記述と一致する。項目 1 は Cursor Rules の引用、項目 2 は対応表の「Code review × GitHub.com = Yes、IDE は No」、項目 3 は 2026-06-12 の changelog の引用、項目 4 は 1Password の Git commit signing の `gpg.ssh.program` の説明(Issue の要約)と照合した |
| 一次資料そのものの取得 | 未実施。cursor.com、docs.github.com、github.blog、1Password の文書はいずれも environment の network 設定で接続できなかった。項目 1 と 3 は、Web 検索の結果の要約が Issue の引用と同じ文面を返した(補助的な確認) |
| `.cursor/rules/` の拡張子 | 14 本すべて `.mdc`(Issue の時点の 13 本に `cloud-session-guidelines.mdc` が加わった) |
| `.github/copilot-instructions.md` の文字数 | 2,783 文字 / 5,473 bytes(Python の `len()`)。変更前は 2,827 文字 |
| 文体(追加行の `(です\|ます)。`) | 該当なし |
| 参照先の節名(「署名の実行経路」) | 存在する |
| `git diff --check` | 問題なし |
| Go の test | Go のコードを変えないため、ローカルでは実行しない。CI の check runs で確かめる |

## リスク・ブロッカー

- 一次資料を cloud session から取り直せていない(「検証」)。Issue は「一次資料で裏を取ってあり、再調査は不要」としている。
- #229 は、本 Issue の項目 4 を新しい guideline(`one-password-integration-guidelines.md`)の「署名経路の確認」へ移す予定である(#229 の「依存・順序」。本 Issue が先に merge された場合は文面を移す)。本 PR の「署名の実行経路」は、そのときの移転元になる。

## セッションログ

- 2026-09-26: #254(PR #264)の merge を確かめ、次の Issue を選んだ。第一候補の #246 は、Slack API の文書に接続できず前提を確かめられないため後に回し、#215 に着手した。bizdate の対応(署名の切り分けと文字数の上限を直した commit と、現行の同じ正本)を読み取り専用の clone で参照した。作業内容 1〜4 を実施し、Issue の「検証」を実行した。
- 2026-09-26: PR #265 を draft で作成し、note を採番した(`509f2c3`)(P1)。検証は上記のとおりで、出力生成系 3 skill は不適用。`progress.md` は索引に無い単発 Issue のため更新しない。採番の報告から引き上げた項目は次のとおり。書き換えた行(確認経路の項目)は note の 3 行と PR description の 1 行である。note は `PR:` 欄の「未作成」を `#265` に、「現在の状況」の「PR 未作成。」を「PR #265 作成済み。」に直し、「次にやること」の「draft PR を作成し、note を採番する。」に「(完了)」を付けた。PR description は note の path を採番後の名前に置き換えた。title は変えていない。触らずに残した行(残された事項)は 0 件で、採番は止まっていない。
- 2026-09-26: review(P2)を別の context の subagent に委ねた。review cycle は `claude-code-bd5f05b-20260926135509`、`Reviewed head` は `bd5f05b1e7ccc53614f1d7ee4ec0b524b560f1e4`。指摘は 2 件(inline 2 件、top-level 0 件)で、prefix の内訳は `[imo]` 1 件、`[nits]` 1 件(prefix の無い指摘は 0 件)。P3 で P4 へ進んだ。
- 2026-09-26: 指摘に対応した(P4)。処置は 2 件とも「採用し修正した」。`[imo]`(`AGENTS.md` の文字数の子項目を削る): 上限の撤廃後に残る内容は正本の帰結と同じ文で、同 guideline の「ルールをシンプルに保つ」と「正本と入口の整理」に照らして削った。`.github/instructions/` を足す agent は `AGENTS.md` の AI Agent 向けルールと rule の入口(`.claude/rules/` の `paths:`、`.cursor/rules/` の `description`)から正本に届き、Copilot は `.github/copilot-instructions.md` で同じ趣旨を読むことを確かめた。`[nits]`(正本の「正本と入口の整理」の例外の項の「リンクを辿れない」): 「リンク先の正本まで辿る保証が無い」に揃えた。修正 commit は `b061c81`。PR description の「主な変更」「レビューしてほしい点」「検証」を合わせて直した。スコープ外とした指摘は無く、follow-up の候補は無い。出力生成系 3 skill は、変更が文書だけのため引き続き適用しない。
- 2026-09-26: 再確認(P5)を P2 と同じ subagent に委ねた。`Reviewed head` は `6e9aa8d13532712450a309ca18ff0da76efc4233`(check runs 5 件 success)。修正確認済み 2 件、スコープ外として確認済み 0 件、対応不要として確認済み 0 件、未対応 0 件。resolve 可の thread は 2 件(`AGENTS.md` の子項目と正本の例外の項)。完了要約は PR conversation comment 1 本。`gh` への fallback は P2、P4、P5 のいずれも無い。
- 2026-09-26: 終了(P6)。この更新は note だけの commit で、P5 が確かめた head `6e9aa8d` より後になる。merge 前に直すべき指摘(`[must]`)は無く、follow-up の候補も無い。人間に残る作業は thread 2 件の resolve と PR の merge で、metadata の誤りを訂正できなかった投稿は無い。
