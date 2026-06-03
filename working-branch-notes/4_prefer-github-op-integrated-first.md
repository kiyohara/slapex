# 作業ブランチメモ

- ブランチ: prefer-github-op-integrated-first
- PR: #4
- 最終更新: 2026-06-03

## 目的

GitHub 操作では `gh` コマンドより先に `github-op-integrated` MCP tool を試すことを、正本・rule shim・repo skill・README でより強く表現する。`AGENTS.md` は index として薄い記述を維持する。

## 現在の状況

- `number-working-branch-note` skill、`github-op-integrated` README を更新済み。`AGENTS.md` は index としての薄い記述(MCP 優先)を維持し、強い表現は正本・rule shim 側へ集約。
- 正本 `doc/guidelines/github-mcp-guidelines.md` に「MCP を最初に試す / `gh` preflight を先に走らせない」点を最小追記済み。`.cursor` / `.claude` の rule shim も同じ強調に揃え済み。
- `gh` の廃止には踏み込んでいない。

## 決定事項

- `gh` コマンドの廃止は未確定であり、各種ガイドラインへ「廃止」は書かない。
- ただし「MCP を最初に試す(`gh` preflight を先に走らせない)」は `gh` 廃止とは別軸として正本に明記する。元から正本にある「まず MCP を試してよい」方針の明確化に留める。
- GitHub 操作では、まず `github-op-integrated` MCP tool の利用可否を確認し、read / write ともに MCP tool を最初に試すことを、入口・shim・skill・正本で揃えて強める。

## 次にやること

- レビュー指摘 (#4) 反映分を commit / push し、PR description の補足を更新する。

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
- 2026-06-03: PR #4 レビュー指摘を反映。正本・rule shim へ MCP-first を明記し、skill に write fallback の安全弁を追記。
- 2026-06-03: AGENTS.md の強い表現を破棄し、index としての薄いポインタに戻す(強い表現は正本・rule shim に集約)。
