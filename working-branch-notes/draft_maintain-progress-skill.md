# 作業ブランチメモ

- ブランチ: `maintain-progress-skill`
- PR: (採番後に記入)
- 最終更新: 2026-06-26

## 目的

v1.0.0 / v1.0.1 公開で v1 リリースプランが完了し、`progress.md` のタスク表が役目を終えて肥大化していた。これを契機に、`progress.md` をリリース台帳 + 進行中タスクの索引として薄く保つ運用を整える。具体的には次の 2 つを行う。

1. `progress.md` をスリム化する(完了済みタスク表を、decision log / Issue / PR 参照を残した要約へ圧縮)。
2. 今後も定期的に発生する `progress.md` 整理作業を標準化する agent skill `maintain-progress` を追加し、`progress.md` 側にはその skill を定期実行する旨の薄いポインタを残す。

## 現在の状況

- `progress.md` をスリム化済み(71 行 → 約 35 行)。構成は「運用メモ / 現況 / リリース履歴 / 完了済みフェーズ(参考)」。v1 の 17 行タスク表と post-v1 表は、decision log・Issue・PR を正本とする要約数行へ圧縮した。
- `.agents/skills/maintain-progress/SKILL.md` を作成済み(正本)。整理の観点を 6 つ(現況 / リリース履歴 / 完了タスクの圧縮 / 進行中タスクの索引 / 参照整合性の維持 / 境界の維持)で構造化。
- `.claude/skills/maintain-progress` を正本への symlink として作成済み(target: `../../.agents/skills/maintain-progress`)。Cursor / Codex は `.agents/skills/` を直読するため symlink は作らない。
- `progress.md` の運用メモに、区切りで `maintain-progress` skill を使って定期整理する旨の薄いポインタを 1 行追加。観点の正本は skill 側。
- 既存 skill(`release` / `number-working-branch-note`)の SKILL.md 単体・日本語構成を踏襲。

## 決定事項

- `progress.md` は **廃止せず存続**。`release` skill と `doc/guidelines/issue-driven-task-execution.md` がこのファイルの存在・役割を前提にしているため、単純削除は参照を宙吊りにする。スリム化(役割は維持しつつ薄くする)を選択した。
- 整理手順は **skill 化**(`maintain-progress`)。整理対象の status board に観点チェックリストを同居させると board が再び太るため、別ファイル(skill)へ分離。
- `progress.md` には恒久ルールを書かず **薄いポインタのみ**(`agent-configuration-management.md` の「入口に恒久ルールを複製しない」方針に沿う)。
- skill 配置は `doc/guidelines/agent-configuration-management.md` の作成 checklist に従う。AGENTS.md への skill 登録は checklist 上「任意」かつ「ルールをシンプルに保つ」方針に沿って省略。
- `progress.md` の note は自動実行ではなく運用上の合図。真の自動化(`/schedule` 等)はこのプロジェクトの手動・熟慮型運用には不要と判断。

## 次にやること

1. 変更分を commit(progress.md スリム化 + ポインタ、maintain-progress skill 正本 + symlink、本 note)。
2. push(ユーザー承認後)。
3. PR を作成し、`number-working-branch-note` skill で本 note を採番。
4. サブエージェントに `/code-review medium` 相当のレビューを実施させ、PR にコメントを残す。
5. PR の merge はユーザーが行う。

## 検証

- 2026-06-26: 正本 `.agents/skills/maintain-progress/SKILL.md` と symlink `.claude/skills/maintain-progress` の両方から `SKILL.md` が読めることを確認。`find -L .claude/skills` で broken symlink が無いことを確認。
- 2026-06-26: Claude Code の skill 一覧に `maintain-progress` が表示されることを確認。
- 2026-06-26: `progress.md` から参照する側(AGENTS.md / doc README / 各 guideline / release skill / rule frontmatter / copilot)の前提を壊していないことを確認(ファイルは存続し役割も維持)。

## リスク・ブロッカー

- なし。ドキュメントと agent skill の追加・整理のみで、実コード・配布設定・CI には変更を加えない。

## セッションログ

- 2026-06-26: `progress.md` 削除の可否を調査。現役の規範参照(release skill / issue-driven-task-execution / 配置ルール / rule frontmatter / copilot)と歴史記録(working-branch-notes / decision log)を切り分け、単純削除は不可と判断。スリム化を選択し実施。
- 2026-06-26: 定期メンテの標準化として `maintain-progress` skill を追加する方針をユーザーと合意。観点を 6 つに構造化して作成し、`progress.md` に薄いポインタを追加。ブランチ + PR 化に着手。
