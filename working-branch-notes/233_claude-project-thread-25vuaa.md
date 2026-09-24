# 作業ブランチメモ

- ブランチ: `claude/project-thread-25vuaa`(cloud session が指定。Issue の推奨ブランチ名は `add-drive-issue-to-reviewed-pr-skill`)
- PR: #233
- 最終更新: 2026-09-24

## 目的

Issue #227。Issue 着手から review と再確認まで済んだ PR までを 1 本で回す orchestrator skill `drive-issue-to-reviewed-pr` を追加する。bizdate の PR #51(decision log 0019)を下敷きにし、slapex 固有の差分(note の採番、`progress.md`、出力生成系 3 skill、Docker Compose での検証、subagent が使えない環境で止まる扱い)を加える。

ユーザーの判断(2026-09-24)で、bizdate PR #78(review metadata の位置と `Model` の確認手段)と、その前提の PR #72(model の識別子の記載範囲)に相当する変更も本 Issue に含める。

本 Issue は、追加する skill の手順そのもので処理する(完了条件の自己適用)。P1 は `run-issue-task` と本 Issue の作業内容に従い、PR 作成後は追加した SKILL.md を repo 相対 path で読み、PR 番号の入口から P2〜P6 を実行する。

## 現在の状況

- 作業内容 0〜4 と、bizdate PR #72 / #78 相当の取り込みを済ませ、Issue の「検証」を実行した(P1)。
- 追加: `.agents/skills/drive-issue-to-reviewed-pr/SKILL.md`、`.claude/skills/drive-issue-to-reviewed-pr`(symlink)、decision log 0059 / 0060 / 0061。
- 追記: `doc/guidelines/development-loop.md`(「使う skill」表に 1 行)、`.agents/skills/run-issue-task/SKILL.md` と `.agents/skills/review-pull-request/SKILL.md`(呼ばれ得ることの参照を 1 段落ずつ)、decision log の `index.md`。
- #72 相当: `doc/guidelines/pull-request-guidelines.md`(「Tool 名と model の識別子の扱い」)、`review-pull-request` の `Model` の項、正本の要約を持つ `release` と `number-working-branch-note` の skill。
- #78 相当: `review-pull-request` の「可視 metadata の canonical フォーマット」と、新しい 2 節(「`Model` の確認手段」「投稿前の確認と誤りの訂正」)。`drive-issue-to-reviewed-pr` の委譲の節、`doc/guidelines/cloud-session-guidelines.md` と `doc/guidelines/github-mcp-guidelines.md` に 1 行ずつ。

## 決定事項

- 下敷きは bizdate PR #51 の時点(`7bc69e0`)の SKILL.md と decision log 0019。bizdate PR #58 / #59 の差分は先取りしない(未決事項 2)。
- 未決事項 1〜6 は仮決めのまま進めた。
- bizdate PR #78 と前提の PR #72 に相当する変更を本 Issue に含めた(2026-09-24 ユーザー判断)。decision log は bizdate と同じくテーマごとに分け、0060(bizdate の 0023 相当)と 0061(0024 相当)にした。
- bizdate との差分は 3 点。
  - conversation comment の編集は `gh api` で行う。bizdate は cloud session で組み込みの `update_issue_comment` を使うが、slapex の cloud session は allowlist 外の組み込み tool を使わない(0058)。allowlist は広げない。
  - `Model` の値から context window の接尾辞(`[1m]` など)を除く。この session の `get_session` で、`session_context.model` と `configured_model` には付き、`last_served_model` には付かないことを確かめた。
  - orchestrator が従う上位の指示が model の識別子の記載範囲を定めている場合は、その内容を brief に事実として添える。値や書き方は指示しない。
- 入口は `argument-hint` の `<Issue番号 | PR番号 | URL>` とした。URL は path で、番号だけの入力は GitHub 上の種別で決める。bizdate の `--from-pr` は採らない(作業内容 1 の例に合わせた)。
- PR から始める場合の対象 review cycle は、`Agent` が現在の Agent 種別と一致する最新の cycle とした。Issue の「本 skill の review cycle」は metadata からは判別できないため、`verify-comments` の担当一致と同じく Agent 種別で読み替えた。
- 収束済みの cycle(指摘 0 件、または未対応 0 件)に PR 番号で入った場合の扱いを足した。`Reviewed head` からの差分が `working-branch-notes/` だけなら P6、それ以外なら新しい cycle を P2 から始める。Issue の入口の記述は、収束済みの場合を定めていなかった。
- P5 は対象 cycle だけを再確認する。1 回の P5 で扱う cycle を 1 つに保つためである。
- P5 で P2 の subagent を再利用する場合も「渡す入力」を省略しない。bizdate は再利用時の省略を認めていたが、P2 で読んでいない `verify-comments` の reference の path が漏れる。
- subagent にさせないことに「作業ツリーのファイル変更」を足した。subagent は orchestrator と同じ作業ツリーを使い得る。
- P2 / P5 の委譲中は push しない(bizdate PR #51 の review で足された規定)。
- 参照する正本に `doc/guidelines/git-operation-guidelines.md`(P4 / P6 の push)と `doc/guidelines/cloud-session-guidelines.md`(#226 で入った正本)を足した。
- `development-loop.md` の行は、「タイミング」列を「Issue 着手から review と再確認まで一括で進めるとき」とした。
- `progress.md` は索引外の単発 Issue のため更新しない。
- 出力生成系 3 skill は、ドキュメントだけの変更で各 skill の「いつ使うか」に当たらないため適用しない。
- decision log の番号は 0059〜0061(0058 は #226 が使った)。

## 次にやること

- PR を作成し、note を採番する。(完了)
- 追加した SKILL.md を path で読み、PR 番号の入口から P2〜P6 を実行する。
- 各フェーズの結果をセッションログに残す。
- 人間: thread の resolve と PR の merge。

## 検証

2026-09-24、cloud session(project の thread)で実行。

| 項目 | 結果 |
|---|---|
| symlink の健全性(`test -L`、`readlink`、`test -f`、broken symlink の検出) | OK。target は `../../.agents/skills/drive-issue-to-reviewed-pr`、broken symlink なし |
| frontmatter の形式 | 新 skill は `name` / `description` / `argument-hint` だけを持ち、`name` はディレクトリ名と一致。全 skill で name の不一致なし |
| repo 相対 path の切れ(新 SKILL.md と変更したファイル) | 切れなし。gitignored な個人設定(`.config/github-op-integrated.conf`)への言及だけが該当し、意図どおり |
| markdown link の切れ(decision log 0059〜0061 と `index.md`) | 切れなし |
| 文体(`grep -nE '(です\|ます)。'`) | 新規・追記箇所で該当なし |
| 既存手順を変えていないこと(`git diff -U0 ... \| grep -E '^-'`) | #227 本来の変更(作業内容 1〜4)は追記だけ。削除行は #72 / #78 相当の範囲に限られる(`review-pull-request` の 3 行、`pull-request-guidelines.md` の 5 行、`release` と `number-working-branch-note` の要約 1 行ずつ)。ユーザーの判断(2026-09-24)による |
| `git diff --check` | 問題なし |
| session 途中の skill 検出 | 検出された。symlink と SKILL.md を作った直後に、session を再起動せず skill 一覧へ載った。検出は保証されないため、SKILL.md は path 直読を既定にしている |
| description による発火 | 未検証。新しい session を開始して確かめる |
| `get_session` の field | `external_metadata.last_served_model`、`session_context.model`、`configured_model` があることを確かめた。接尾辞の有無は「決定事項」のとおり |
| 自己適用の P1 判断 | 依存なし、推奨ブランチ名あり(PR description に記録)、`progress.md` の索引外で更新不要、ドキュメントだけの変更で出力生成系 3 skill は不適用、と判断できた |
| Go の test | Go のコードを変更しないため、ローカルでは実行しない。CI の check runs 5 件で確かめる |

## リスク・ブロッカー

- 未検証: description による発火。
- 自己適用の盲点: skill を設計した orchestrator が SKILL.md を読むため、記述の不足を context で補って動けてしまう。P2 の subagent に SKILL.md の自己完結性を観点として渡す。
- 自己適用は、この PR で変えた `review-pull-request` の metadata の規定(footer の直前に置く、`Model` の確認手段)で行う。規定と実際の投稿が合っているかを、P2 / P5 の read-back で確かめる。
- この session の実行環境は、model の識別子をチャットの返答以外に書かないよう求めている。orchestrator の投稿では `Model` を `unknown` とし、その旨を 1 行残す(0060)。subagent には指示の内容を事実として渡し、値は指示しない(0061)。

## セッションログ

- 2026-09-24: bizdate main `34b33b1` を確認した(作業内容 0)。`ab7c88e` 以降の関連 merge は PR #78 の 1 件で、結果は Issue #227 のコメントに追記した。
- 2026-09-24: 作業内容 1〜4 を実施し、Issue の「検証」を実行した。
- 2026-09-24: ユーザーの判断で bizdate PR #78 と前提の PR #72 に相当する変更を取り込み、decision log 0060 / 0061 を追加した。検証をやり直した。
