---
name: maintain-progress
description: slapex の `progress.md` を定期的に整理し、リリース台帳と進行中タスクの索引として読みやすく保つ。役目を終えた完了タスク表を decision log / Issue / PR への参照を残した要約へ圧縮し、現況・リリース履歴・進行中タスクの索引を最新化し、AGENTS.md / 各 guideline / `release` skill / rule frontmatter が `progress.md` を指す前提(参照整合性)と、decision log・working-branch-notes との境界を維持する。観点別チェックで安全側に整理し、迷う点はユーザーに確認する。
---

# maintain-progress

slapex の `progress.md` を整理して、横断的な作業状況ボードとして読みやすく保つための skill。

`progress.md` は「リリース台帳」と「進行中タスクの索引」を兼ねる恒久ファイルであり、`release` skill と `doc/guidelines/issue-driven-task-execution.md` がこのファイルの存在と役割を前提にしている。整理の目的は **このファイルを薄く・正確に保つこと** であって、廃止や履歴の作り直しではない。

完了したタスクの詳細経緯は decision log / 各 Issue / 各 PR / working-branch-notes が正本である。`progress.md` には到達点と参照だけを残し、それらを複製しない。

## いつ実行するか

次のいずれかに当てはまり、ボードが現状を素早く把握しづらくなっているとき:

- 完了済みのタスク表・項目が溜まり、「いま何が進行中か」が一目で分からない。
- リリースを公開したのに「リリース履歴」「現況」が古いまま(通常 `release` skill 側で更新されるが、漏れの点検も兼ねる)。
- ある作業フェーズ(リリースプラン、改善バッチなど)の全項目が done になり、表として残す意味が薄れた。
- 進行中の横断タスクが始まった/終わったのに索引へ反映されていない。

定期点検として、リリースの前後や、まとまった Issue 群を消化し終えた区切りで実行するとよい。

## 整理の観点

各観点を順に確認し、必要な箇所だけ最小限に編集する。判断に迷う箇所は止めてユーザーに確認する。

### 1. 現況

- 冒頭付近に、最新の到達点(公開済みバージョン、配布経路、いま追跡している横断プランの有無)を 1〜数行で保つ。
- 進行中の横断プランが無い場合は、その旨と「新しい横断タスクが始まったら表を追加する」という運用だけを残す。事実に反する「進行中なし」を断定しない(不明なら断定せず、確認した範囲を書く)。

### 2. リリース履歴

- 公開済みバージョンを台帳として保つ。各行はバージョン・状態・1 行メモ(スコープやリリース PR への参照)に留める。
- `release` skill がここへ行を足す前提なので、表の見出しと列構成を壊さない。列を変える場合は `release` skill の手順と齟齬が出ないか確認する。

### 3. 完了タスクの圧縮

- 全項目が done になったタスク表・フェーズは、行ごとの詳細を残さず、フェーズ単位の要約数行へ圧縮する。
- 圧縮時は **追跡可能性を失わない**。decision log 番号、Issue 範囲、PR 範囲への参照を要約に残し、詳細はそれらを正本とする。
- まだ進行中・未着手の項目が混在する表は圧縮しない。done と未 done が混ざる場合は、進行中だけを表に残し、done をフェーズ要約へ送る。

### 4. 進行中タスクの索引

- いま進めている/これから着手する横断タスクは、状態・依存・参照(Issue / PR)が分かる最小の表で索引する(`doc/guidelines/issue-driven-task-execution.md` がこの索引を依存確認に使う)。
- 単発の design adjustment など、横断プランに属さない小さな Issue まで無理に索引へ載せない(過去の運用でも未掲載の Issue がある)。索引は「横断的に見渡したい単位」に絞る。

### 5. 参照整合性の維持

`progress.md` は次から参照されている。整理でこれらの前提を壊さない。

- `AGENTS.md` / `doc/README.md` / `doc/design/README.md` — 「作業状況の一覧は `progress.md`」という配置ルール。
- `doc/guidelines/issue-driven-task-execution.md` — タスク表を依存確認・状態更新の索引に使う。
- `doc/guidelines/decision-log-guidelines.md` — 小さな TODO / 作業メモの寄せ先。
- `.agents/skills/release/SKILL.md` — リリースごとに `progress.md` を更新する。
- `.claude/rules/decision-log-guidelines.md` の `paths:` frontmatter / `.github/copilot-instructions.md` — `progress.md` を対象に含む。

整理は通常これらの前提を保ったまま行える。**もしファイルの役割そのものを変える(リリース台帳/索引をやめる、ファイルを廃止する等)場合は、本 skill の範囲を超える方針変更**であり、`doc/guidelines/agent-configuration-management.md` と `doc/guidelines/decision-log-guidelines.md` に従って、上記の参照側ドキュメントの追従修正と decision log の記録を同じ変更に含める。安易に削除しない。

### 6. 境界の維持

- 検討経緯・方針の理由は書かない。それは decision log の役割(`doc/guidelines/decision-log-guidelines.md`)。`progress.md` には到達点と参照だけを置く。
- ブランチ単位の作業目的・引き継ぎは書かない。それは `working-branch-notes/`。
- `working-branch-notes/**` と既存 decision log は履歴記録なので、本 skill では遡って書き換えない。

## やらないこと

- `progress.md` の廃止やファイル削除(役割変更を伴うため本 skill の範囲外。前項 5 を参照)。
- decision log・working-branch-notes の内容を `progress.md` へ転記する/それらを書き換える。
- リリース履歴の見出し・列構成を `release` skill と無断で食い違わせる。
- 進行中・未着手を含む表の繰り上げ圧縮。

## 終了時の確認

- ボード冒頭を読むだけで「公開済みバージョン」「いま進行中の横断タスクの有無」が分かる状態になっている。
- 圧縮した完了フェーズに decision log / Issue / PR への参照が残っている。
- 参照側ドキュメント(観点 5)の前提を壊していない(役割変更をしていないこと)。
- 変更は status board として最小限で、検討経緯や引き継ぎメモを持ち込んでいない。
