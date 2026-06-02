# Agent 設定管理ルール

この文書は、slack_posts_exporter リポジトリ内で Cursor / Codex app / Claude Code / GitHub Copilot Review から共通利用する AI agent 向け rule と入口ファイルを作成・削除・rename するときの共通正本である。

AI と人間で別ドキュメントを持たず、全員がこの文書を読む。本ガイドライン自身もこのルール体系の管理対象である。

## 基本方針

- 恒久的な方針は `doc/guidelines/` または `doc/product/` の正本に置く。
- `AGENTS.md`、`CLAUDE.md`、`.cursor/rules/`、`.claude/rules/` は入口として扱い、長い本文を複製しない。
- `.github/copilot-instructions.md` は GitHub Copilot Review 用の入口として扱う。Copilot はリンクを辿らないため、レビュー時に効かせたい要点はこのファイルに直接書く。
- Cursor / Codex app / Claude Code のどれか一方だけが読める配置にしない。
- 新しい rule を作ったら `AGENTS.md` の「共通正本」に必ず追加する。Codex app が正本へ到達するために必要である。
- 削除・rename 時は、正本だけでなく tool 固有入口も同じ変更で揃える。

## 各 tool の入口

| Tool | 主な入口 | 用途 |
|---|---|---|
| Codex app | `AGENTS.md` | 共通正本への index |
| Claude Code | `CLAUDE.md`, `.claude/rules/*.md` | `@AGENTS.md` の取り込みと path scoped rule |
| Cursor | `.cursor/rules/*.{md,mdc}` | frontmatter による rule 読み込み |
| GitHub Copilot Review | `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md` | レビュー時の repo 全体方針と path 別観点 |

## AI 向け rule の配置

```text
doc/guidelines/<rule-name>.md
.cursor/rules/<rule-name>.mdc
.claude/rules/<rule-name>.md
AGENTS.md
CLAUDE.md
.github/copilot-instructions.md
.github/instructions/*.instructions.md
```

- `doc/guidelines/<rule-name>.md` は AI と人間が読む共通正本。
- `.cursor/rules/<rule-name>.mdc` は Cursor 用入口。共通正本への参照と、いつ読むかだけを書く。
- `.claude/rules/<rule-name>.md` は Claude Code 用入口。共通正本への参照と、いつ読むかだけを書く。
- `AGENTS.md` は Codex app と AI agent 共通の index。すべての rule 正本をここにリストする。
- `CLAUDE.md` は原則 `@AGENTS.md` の取り込みだけに留める。
- `.github/copilot-instructions.md` は Copilot Review の repo 全体方針。Copilot はリンクを辿らないため、本文は高シグナルな要点に絞って直接書く。
- `.github/instructions/*.instructions.md` は実装内容に応じた path 別レビュー観点。技術スタックが未確定の段階では無理に作らない。

## 作成 checklist

1. `doc/guidelines/<rule-name>.md` に共通正本を作る。
2. `.cursor/rules/<rule-name>.mdc` を作り、共通正本への参照と発火条件だけを書く。
3. `.claude/rules/<rule-name>.md` を作り、共通正本への参照と対象 path だけを書く。
4. `AGENTS.md` の「共通正本」セクションに共通正本へのリンクを追加する。
5. Copilot Review にも効かせる必要がある内容なら `.github/copilot-instructions.md` または `.github/instructions/*.instructions.md` に要点を直接書く。
6. 3 ファイルの basename(`<rule-name>`)が揃っていることを確認する。

## 削除 checklist

rule の共通正本を削除する場合は、同じ変更で Cursor / Claude Code 用入口 rule と `AGENTS.md` の参照も削除する。

削除対象:

```text
doc/guidelines/<rule-name>.md
.cursor/rules/<rule-name>.mdc
.claude/rules/<rule-name>.md
AGENTS.md 内の該当リンク
.github/copilot-instructions.md または .github/instructions/*.instructions.md 内の該当記述
```

削除後は旧名への参照が残っていないか確認する。

## rename checklist

1. 新しい `doc/guidelines/<new-rule-name>.md` を作る。
2. Cursor / Claude Code 入口 rule を新しい名前と参照先に更新する。
3. `AGENTS.md` の参照を更新する。
4. 古い共通正本、古い入口 rule、古い `AGENTS.md` 参照を削除する。
5. 旧名への残存参照がないことを確認する。

## 禁止事項

- agent 固有入口だけに恒久ルールを書く。
- `.cursor/rules/` と `.claude/rules/` に長い本文を二重に書く。
- 共通正本を削除したあと、agent 固有入口から dead link を残す。
- 新しい rule の `AGENTS.md` 登録を省略する。
- 実装技術が未確定の段階で、特定技術スタック向けの `.github/instructions/*.instructions.md` を先行作成する。
