# 作業ブランチメモ

- ブランチ: issue-125-document-style-guidelines
- PR: #127
- 最終更新: 2026-07-04

## 目的

Issue #125 のドキュメント文体ガイドライン(読者層別の文末・トーン・用語表記)を `doc/guidelines/document-style-guidelines.md` として制定し、入口 rule と AGENTS.md リンク、decision log を同じ PR で整える。

## 現在の状況

- ユーザーヒアリング済み。方針: 利用者向け(README / doc/help)= ですます調、開発者・AI agent 向け = 常体、トーンは簡潔・中立、用語表記(技術用語は英語のまま)と読者層別の内容分担の要点も含める。
- 既存文書への一括適用は #126 に分離(依存: #125、PR #124 merge 後)。

## 決定事項

- ガイドラインは `doc/guidelines/agent-configuration-management.md` の「AI 向け rule 管理」checklist に従い、正本 + Cursor / Claude Code 入口 + AGENTS.md リンクを同一 PR で揃える。
- `doc/help/README.md` の既存「文体」節は内容方針(手順として書く等)を定めるものなので「内容の方針」へ改名し、語調は新ガイドラインへの参照とする。
- 方針決定の経緯は decision log 0048 に記録する。
- progress.md の索引に help-06(#125)/ help-07(#126)を登録する。help-05(#123)の行は PR #124 のブランチにあり本ブランチには無いため、#123 側の依存(文体ガイドライン準拠)は Issue #123 へのコメントで補足する。PR #124 と本 PR は progress.md の同じ表に行を追加するため、後から merge する側で軽微な conflict 解消が要る想定。

## 次にやること

- PR レビュー対応。merge はユーザーが行う。

## 検証

- rule 作成 checklist を確認: 正本(`doc/guidelines/document-style-guidelines.md`)、Cursor 入口(`.cursor/rules/document-style-guidelines.mdc`)、Claude Code 入口(`.claude/rules/document-style-guidelines.md`)の 3 ファイルで basename が一致。`AGENTS.md` に共通正本リンクと利用ルールの 2 箇所を追加。`CLAUDE.md` は変更なし。
- decision log 0048 を作成し、`index.md` の「現在有効な主要方針」に行を追加したことを確認。
- `doc/help/README.md` の「文体」節を「内容の方針」へ改名し、語調は新ガイドライン参照としたことを確認。
- ドキュメントのみの変更のため、Go の build / test は実施していない(コード変更なし)。

## リスク・ブロッカー

- 特になし。

## セッションログ

- 2026-07-04: 文体ガイドライン不在を確認、ユーザーヒアリングで方針確定。Issue #125 / #126 を起票し、本ブランチで制定作業を開始。
