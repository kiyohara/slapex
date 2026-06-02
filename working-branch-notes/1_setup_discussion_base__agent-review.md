# 作業ブランチメモ（レビュー サブノート）

- ブランチ: setup_discussion_base
- PR: #1
- 最終更新: 2026-06-02
- 主 note: `working-branch-notes/1_setup_discussion_base.md`
- 種別: AI agent によるドキュメント基盤レビュー結果

## 目的

`setup_discussion_base` ブランチで整備した設計ドキュメント基盤・AI agent 連携ファイル・ガイドライン群について、実装フェーズに入る前に大きな課題が残っていないかをレビュアー視点で点検し、指摘と改善提案を記録する。

レビュー prefix は本リポジトリの `.github/copilot-instructions.md` の分類に合わせる（`[must]` / `[ask]` / `[imo]` / `[nits]` / `[fyi]`）。

## レビュー対象

- `doc/design/`（usage-flow / decision-log）と `progress.md`
- `doc/guidelines/`（agent-configuration-management / decision-log / working-branch-notes 2 種）
- AI agent 入口（`AGENTS.md`, `CLAUDE.md`, `.cursor/rules/`, `.claude/rules/`, `.github/copilot-instructions.md`）
- `working-branch-notes/`（README / template / 主 note）

参考: 類似の別プロジェクト(最新の同型テンプレート), [kiyohara/slack_posts_dumper](https://github.com/kiyohara/slack_posts_dumper)（同目的の先行 prototype）。

## 総評

- 全体構成は良好。「共通正本は `doc/guidelines/` または `doc/design/`、各 tool 入口は薄い shim」という分離方針が一貫しており、類似の別プロジェクトの運用パターンを正しく踏襲できている。
- 入口ファイル間の相互参照・相対リンクは概ね正しく、dead link は検出されなかった。
- 一方で、(1) Cursor rule の発火条件、(2) ガイドラインが約束する範囲と実体の乖離、(3) 先行 prototype と進捗管理の活用、の 3 点に実質的な改善余地がある。実装着手前に潰しておくと後続 agent の事故と手戻りを減らせる。

## 指摘事項

### [must] working-branch-notes 系 Cursor rule に `globs:` が無く、path 自動ロードされない

- `.cursor/rules/working-branch-notes-handling.mdc` と `working-branch-notes-security.mdc` は `description` + `alwaysApply: false` のみで `globs:` が無い。
- Cursor ではこの形は「Agent Requested」扱いとなり、AI が description から関連と判断したときだけロードされる。`working-branch-notes/**/*.md` を編集しても確定的には添付されない。
- 一方 Claude 側 `.claude/rules/working-branch-notes-*.md` は `paths: ["working-branch-notes/**/*.md"]` を持ち path 自動ロードされる。**同一の情報統制ルールが tool によって発火条件が非対称**になっている。
- 参考とした類似の別プロジェクトの同 rule は両方に `globs: ["working-branch-notes/**/*.md"]` を付与しており、本リポジトリはここを取りこぼしている（テンプレートからの劣化）。
- 特に security（秘密情報・個人情報の禁則）は、AI の裁量任せではなく path で確定発火させるべき。
- 提案: 両 `.mdc` に下記を追加する。
  ```yaml
  globs:
    - "working-branch-notes/**/*.md"
  ```
  handling/security は `alwaysApply: false` のまま globs 併用で「Auto Attached」化する。`agent-configuration-management.mdc` は path 非依存なので現状（globs なし）で可。`decision-log-guidelines.mdc` は行為起点のため description のみでも可だが、`doc/design/decision-log/**` を編集中に効かせたいなら globs 追加を検討。

### [must] `agent-configuration-management` が「skill」を扱うと宣言しているが本文に skill 規定が無い

- `AGENTS.md` の共通正本一覧: 「Agent 設定管理ルール(**skill** / rule の作成・削除・rename・配置)」と skill を含めている。
- しかし `doc/guidelines/agent-configuration-management.md` の本文は「rule と入口ファイル」のみが対象で、skill の配置・作成・削除 checklist が一切無い（類似の別プロジェクトには `.agents/skills/` ↔ `.claude/skills/` symlink 運用などの詳細がある）。本リポジトリには `.agents/skills/` / `.claude/skills/` も未整備。
- このままだと「skill を作って」と指示された agent が辿り着ける checklist が存在せず、宙に浮いた約束になる。
- 提案（どちらか）:
  - (a) 当面 skill を使わないなら、`AGENTS.md` の当該ラベルから `skill /` を外し「rule の作成・削除・rename・配置」に絞る。
  - (b) 将来使う前提なら、ガイドラインに「skill は未導入。導入時は類似の別プロジェクトの `.agents/skills/` 方式に従う」の short stub だけ置き、実体を伴わせる。

### [imo] 先行 prototype [kiyohara/slack_posts_dumper](https://github.com/kiyohara/slack_posts_dumper) の知見が設計ドキュメントから参照されていない

- [kiyohara/slack_posts_dumper](https://github.com/kiyohara/slack_posts_dumper) は同目的（Slack 投稿を Slack 風 HTML で保存）の実装済み prototype で、README / PROJECT_SPEC / PROGRESS と、bot token (`xoxb-`) の必要 scope（`channels:history` / `channels:read` / `users:read` / `files:read`）、emoji 置換・URL 変換・HTML sanitize・asset ローカル化・Unicode フォールバックといった設計判断が既に存在する。
- 現状の `usage-flow.md` / `decision-log/` はこの先行資産を一切参照せず白紙から始めており、(1) 既出の判断を再導出する手戻り、(2) usage-flow を prototype の README から seed できる機会の損失、が起きうる。
- 提案: 早い段階で「prototype との関係 / どこを再利用しどこを作り直すか / 新リポジトリにした理由」を整理する。decision-log の最初の 1 件（例 `0001-relationship-to-prototype.md`）にするのが、template の end-to-end 検証も兼ねられて好適。

### [imo] progress.md と working-branch-note の境界が未定義で、status / 次にやること が重複しうる

- 現在「現在の状況」「次にやること」を持つ場所が **working-branch-note**（ブランチ単位・短命）と **progress.md**（恒久 status board）の 2 箇所あり、両者の役割分担がどのガイドラインにも明記されていない。
  - decision-log ↔ progress の境界は `decision-log-guidelines.md` で定義済み。
  - note ↔ `doc/guidelines/` 正本の境界は `working-branch-notes-handling.md` で定義済み。
  - しかし **progress.md ↔ working-branch-note** の境界だけ空白。
- 実際 `progress.md` は現時点で雛形のみ（作業項目ゼロ）で、セットアップ作業自体が反映されていない。放置すると 2 つが drift する。
- 提案: `progress.md`(横断・恒久) と note(ブランチ・短命) の使い分けを 1〜2 行どこかの正本に明記し、`progress.md` に現セットアップ項目と次アクションを最小限 seed する。

### [nits] `AGENTS.md` で working-branch-notes/README を「プロダクト設計ドキュメント」に分類

- `working-branch-notes/README.md` は作業プロセス文書であり、usage-flow / progress / decision-log と同じ「プロダクト設計ドキュメント」見出し下に並べると分類がやや不正確。別見出し（例: 作業プロセス / 作業メモ）に切り出すと読み手の期待と一致する。

### [fyi] Copilot は instruction file を冒頭一定量しか反映しない制約が未記載

- 類似の別プロジェクトの `AGENTS.md` は「Copilot は各 instruction file を先頭〜約 4,000 文字のみ反映」と明記している。現 `.github/copilot-instructions.md` は短く現状問題ないが、今後加筆する場合に備え制約を一言残すと安全。

### [fyi] `.git-backup/` が作業ツリーに残存

- `.gitignore` 済みで追跡対象外、主 note にも記録済みで現時点の問題は無い。新履歴の健全性を確認できたら、混乱や不要スキャンを避けるため作業ツリーからの削除を検討してよい（ユーザー判断）。

## 良い点（維持したい）

- 共通正本 / 薄い入口の分離、`AGENTS.md` を Codex 到達点とする設計、削除・rename checklist 整備など、運用の型が崩れていない。
- working-branch-notes の security 正本が独立しており、decision-log / copilot 側からも禁則が再掲されていて多層で効く。
- decision-log を index + 個別ログに分割し、`superseded` で履歴を残す方針は、後続 agent が経緯を辿る目的に合致。

## 推奨アクション（優先順）

1. [must] Cursor の working-branch-notes handling/security `.mdc` に `globs: ["working-branch-notes/**/*.md"]` を追加。
2. [must] `agent-configuration-management` の skill 範囲の不整合を解消（skill を外す or stub を置く）。
3. [imo] decision-log `0001` として prototype 関係 / scope を起票し、template を実運用で検証。
4. [imo] progress.md ↔ working-branch-note の境界を 1 行明記し、progress.md を最小 seed。
5. [nits]/[fyi] 分類・Copilot 制約・`.git-backup` は低コストなら同時に対応。

## メモ

- 本サブノートは `doc/guidelines/working-branch-notes-handling.md`(補助 note は主 note basename + suffix)と `working-branch-notes-security.md`(秘匿情報を書かない)に従って作成。秘密情報・個人情報・顧客固有情報は含めていない。
