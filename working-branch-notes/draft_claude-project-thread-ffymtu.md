# 作業ブランチメモ

- ブランチ: `claude/project-thread-ffymtu`(cloud session が指定。Issue の推奨ブランチ名は `verify-non-push-fixes`)
- PR: 未作成
- 最終更新: 2026-09-26

## 目的

Issue #257。`review-pull-request` の `address-comments` と `verify-comments` に、push を伴わない修正(PR description、PR の title、Issue の本文など、GitHub 上の編集で直す修正)の確かめ方を定める。push を伴わない修正を含む指摘について、`address-comments` が返信してよい条件と、`verify-comments` の再確認の結果(修正確認済みか未対応か)を、verifier によらず一意に決まるようにする。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 4 件目である。PR description への指摘は review のたびに出得るため、以後の Issue の再確認(P5)の揺れを減らせる。cloud session で完結する。

## 現在の状況

- 作業内容 1〜4 を実施し、Issue の「検証」を実行した(P1 の途中)。

## 決定事項

- 用語: 修正を「push を伴う修正」(commit で repository のファイルを変える修正)と「push を伴わない修正」(GitHub 上の編集で直す修正)の 2 種に分けた。Issue の作業内容 3 の呼び方に合わせた。
- 作業内容 1: `address-comments.md` の手順 7 を、修正が GitHub 上に反映済みであることを確かめる規定にし、2 種ごとの確かめ方を箇条にした。push を伴う修正は従来の文言(head branch へ push 済みで、head SHA から確認できる)を保った。push を伴わない修正は、編集の後に read 系 tool で対象を取り直し、直した内容が GitHub 上の現在の内容にあることを確かめる。
  - read 系 tool は、PR description と title を `pull_request_read(get)` と明記し、それ以外(Issue の本文など)は `doc/guidelines/github-mcp-guidelines.md` の「操作別の第一選択」を参照した。tool の対応を skill に複製しないためである。
  - 反映を確かめられない編集(write の失敗など)は対応済みとして扱わないとし、失敗時の扱いは `SKILL.md`「MCP write failure の安全手順」を参照した。
- 作業内容 2 / 未決事項 1: 仮決定のとおり、push を伴わない修正の返信には、修正 commit / head SHA の代わりに、編集した対象(PR description など)と箇所(節名など)を書く。手順 8 の返信の項目の「修正 commit / head SHA」を「修正の所在」にまとめ、2 種ごとに書く内容と、両方を含む修正では両方を書くことを定めた。理由(`verify-comments` が確かめる対象を返信から特定できるようにする)も書いた。
- 手順 10 に、top-level の指摘など inline thread の無い指摘への処置を PR conversation comment で返す場合も、手順 8 の返信と同じ内容(修正の所在を含む)を書くことを加えた。Issue の作業内容には無い追加である。Issue の背景の 2 例(PR #256、PR #243)はどちらも top-level の指摘を PR description の編集で直しており、手順 8 は inline comment への返信だけを定めるため、手順 8 だけを直すとこの 2 例に規定が届かない。
- 作業内容 3 / 未決事項 2: `verify-comments.md` の「処置ごとの確認」の「採用し修正した」の行を「修正が GitHub 上に反映済みで(下記)」とし、表の下に 2 種ごとの反映の確かめ方を箇条で足した。push を伴わない修正は、返信が示す対象と箇所が、再確認の時点の GitHub 上の内容で直っていることを確かめる。時点は仮決定のとおりで、編集には版を固定する SHA が無いという理由も書いた。返信が編集した対象を示さず、確かめる対象を特定できない場合は未対応とした。
  - 同じ行の、指摘の解消と新たな問題の有無を確かめる材料に「GitHub 上で編集した対象の現在の内容」を加えた。材料が差分、実装、関連 test / document、検証だけだと、PR description の編集だけで直した指摘について、解消を確かめる材料が表に無いと読めるためである。
  - Step 5 の「未 push」を「修正が未反映(「処置ごとの確認」の「採用し修正した」)」に改めた。push を伴わない修正の未反映も未対応に含まれることを、表と揃えて読めるようにした。
- 作業内容 4: 修正が両方を含む場合、`address-comments` は両方を確かめ、返信に両方の所在を書く。`verify-comments` は両方が成り立つときだけ反映済みとする。
- 変えなかったもの:
  - `review-pull-request` の `SKILL.md`。read 系 tool の選び方は参照先の guideline が定めており、skill の tool 表への追記は要らない。
  - `drive-issue-to-reviewed-pr` の「再開する場合」の「修正の push は、`address-comments` が対応済みの返信の前に確かめている」。push を伴う修正について引き続き正しい。push を伴わない修正も、変更後の手順 7 で返信の前に確かめるため、P5 から再開するときに別の確認は要らない。
  - 同 skill の「working branch note」の表の P4 の記録項目(修正 commit)。push を伴わない修正の所在は返信(手順 8 / 10)に残り、`verify-comments` はそれを読む。note の記録項目の見直しは本 Issue の範囲(`references/` の 2 ファイル)の外とした。
  - 同 skill の再開位置の判定(Issue のスコープ外)。
- decision log は作らない。修正を GitHub 上で確かめてから返信し、Review 担当が再確認するという既存の方針を、push を伴わない修正へ当てはめる規定の追加であり、方針の変更や選択肢の比較を伴う判断ではないためである。未決事項 1 / 2 は Issue の仮決定のとおりにした。
- 出力生成系 3 skill(`update-sample-exports`、`update-readme-preview-screenshots`、`update-readme-demo-gif`)は適用しない。変更は `.agents/skills/review-pull-request/references/` の 2 ファイルと本 note だけで、各 skill の「いつ使うか」に当たらない。
- `progress.md` は更新しない。#257 は索引に登録されていない。

## 次にやること

- PR を draft で作成し、note を採番する。
- P2 の前に最新 head の check runs がすべて success であることを確かめ、review を subagent に委譲する。

## 検証

2026-09-26、cloud session(project の thread)で実行。

Issue の「検証」の 4 状態と、返信が編集した対象を示さない場合について、変更後の `address-comments.md` と `verify-comments.md` を読み、返信の条件と再確認の結果が一意に決まることを確かめた。

| 状態 | `address-comments`: 「対応済み」と返信してよい条件と返信の内容 | `verify-comments`: 再確認の結果 |
|---|---|---|
| 修正が PR description の編集だけ | 編集の後に `pull_request_read(get)` で取り直し、直した内容が現在の PR description にあるとき(手順 7)。返信に編集した対象(PR description)と箇所(節名など)を書く(inline の指摘は手順 8、top-level の指摘は手順 10) | 返信が示す箇所が再確認の時点の PR description で直っており、指摘が解消し新たな問題が無ければ修正確認済み。直っていなければ未対応 |
| 修正が commit と PR description の編集の両方 | 修正 commit が head branch へ push 済みで head SHA から確認でき、かつ PR description を取り直して反映を確かめたとき(手順 7 の「両方を確かめる」)。返信に修正 commit / head SHA と、編集した対象と箇所の両方を書く | 両方が成り立つときだけ反映済みとし、指摘が解消し新たな問題が無ければ修正確認済み。どちらかが成り立たなければ未対応 |
| PR description の編集が GitHub 上に反映されていない(編集の失敗など) | 対応済みと返信しない(手順 7 の「反映を確かめられない編集を対応済みとして扱わない」)。`SKILL.md`「MCP write failure の安全手順」に従い、反映を確かめてから再試行するか、ユーザーに報告する | 誤って対応済みと返信された場合も、返信が示す箇所が直っていないため反映済みにならず、未対応(Step 5 の「修正が未反映」) |
| 修正が commit だけ(従来どおり) | 修正 commit が head branch へ push 済みで head SHA から確認できるとき(手順 7。従来の規定と同じ)。返信に修正 commit / head SHA を書く | 修正が head に push 済みで、指摘が解消し新たな問題が無ければ修正確認済み。未 push なら未対応(従来と同じ) |
| (補足)返信が編集した対象を示さない | 手順 8 に反する返信である | 確かめる対象を特定できないため未対応 |

| 項目 | 結果 |
|---|---|
| repo 相対 path(`doc/guidelines/github-mcp-guidelines.md`、`doc/guidelines/git-operation-guidelines.md`、`references/address-comments.md`) | 存在する |
| 参照先の節名(`SKILL.md`「MCP write failure の安全手順」、guideline の「操作別の第一選択」、`verify-comments.md` の「処置ごとの確認」、`address-comments.md` の手順 8) | すべて存在する |
| 文体(追加行の `(です\|ます)。` と「ください」) | 該当なし |
| `git diff --check` | 問題なし |
| push を前提にする他の記述(`git grep -n "push" -- .agents/skills/review-pull-request`、`drive-issue-to-reviewed-pr` の SKILL.md) | `review-pull-request` の `SKILL.md` は、修正の commit / push の方法と、review 対象に未 push の変更を混ぜない規定だけで、本 Issue と関係しない。`drive-issue-to-reviewed-pr` は「決定事項」の「変えなかったもの」の 2 箇所である |
| Go の test | Go のコードを変えないため、ローカルでは実行しない。CI の check runs で確かめる |
| description による発火 | 未検証。frontmatter は変えていない。新しい session を開始して確かめる |

## リスク・ブロッカー

- 未検証: description による発火。
- 変更後の規定は、push を伴わない修正を含む実際の review cycle ではまだ通していない。

## セッションログ

- 2026-09-26: #252(PR #259)の merge を確かめ、次の Issue に #257 を選んだ。作業内容 1〜4 を実施し、Issue の「検証」を実行した。
