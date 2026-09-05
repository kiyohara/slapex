# 作業ブランチメモ

- ブランチ: `align-work-record-guidelines`
- PR: #198
- 最終更新: 2026-09-05

## 目的

Issue #196 の既存の記録配置ルールの矛盾を解消する。

## 現在の状況

- 調査 PR #197 の merge と、Issue #196 にコメント・sub-issue・着手済みの open PR が無いことを確認した。
- 手元の main を pull し、`b979b9e` から作業ブランチを作成した。
- decision log の作業メモ配置指示、note の長期情報の移転先、maintain-progress skill の参照説明を修正した。

## 決定事項

- 作業経緯は note、実行指示は Issue、恒久運用規則は guideline、仕様は spec、設計判断の理由は decision log、状態と依存は progress という既存の分担に揃える。
- 新たな方針は導入せず、矛盾する本文と参照説明のみを修正する。過去 note の一括移転や長期保守は要求しない。

## 次にやること

- PR の review / merge。

## 検証

- `git diff --check`: 成功。
- 関連 guideline / skill の配置キーワード検索と本文の突合: 作業メモ、実行指示、仕様、運用規則、採否理由、状態・依存の分担が一致することを確認した。
- AGENTS.md、Cursor / Claude Code の入口から修正した両 guideline への参照と参照先の存在を確認した。入口への本文複製は不要。
- maintain-progress の Claude Code 用 symlink が共通 skill を指すことを確認した。
- note の情報統制チェック: 禁止情報なし。
- Go tests は文書のみの変更のため未実施(Issue の検証条件に従う)。

## リスク・ブロッカー

- 現時点でなし。文書のみのため Go tests は不要。

## セッションログ

- 2026-09-05: Issue と依存条件を確認し、最新 main から着手した。
