# 作業ブランチメモ

- ブランチ: prefer-github-op-integrated-first
- PR: #4
- 最終更新: 2026-06-03

## 目的

GitHub 操作では `gh` コマンドより先に `github-op-integrated` MCP tool を試すことを、AI agent の入口と repo skill 側でより強く表現する。

## 現在の状況

- `AGENTS.md`、`number-working-branch-note` skill、`github-op-integrated` README を更新済み。
- ガイドライン本文には反映していない。

## 決定事項

- `gh` コマンドの廃止は未確定であり、各種ガイドラインへは書かない。
- GitHub 操作では、まず `github-op-integrated` MCP tool の利用可否を確認し、read / write ともに MCP tool を最初に試すことを強める。

## 次にやること

- 差分を確認し、commit / push / PR 作成を行う。

## 検証

- `git diff` で変更範囲を確認。
- `doc/guidelines/` 配下に差分がないことを確認。
- `number-working-branch-note` skill 内の `gh pr view` / `gh pr edit` は fallback 文脈に限られることを確認。

## リスク・ブロッカー

- 外部 plugin skill の `yeet` など、repo 外の指示はこの PR では変更できない。
- MCP tool が実行環境に露出していない場合は fallback 判断が必要になる。

## セッションログ

- 2026-06-03: `main` から `prefer-github-op-integrated-first` ブランチを作成。
- 2026-06-03: GitHub 操作の最初の試行先を `github-op-integrated` MCP tool に寄せる表現を、入口と skill に反映。
