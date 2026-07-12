# 進捗管理表

このファイルには、設計および実装作業の進捗を記録していく。

想定読者は、実装作業を行う人間、および AI agent である。

横断的な作業状況(作業項目、状態、次にやること、検証状況、未解決のリスクやブロッカー)を一覧で把握するための管理表として使う。詳細な検討経緯は `doc/design/decision-log/` に置き、このファイルでは現在の作業状況を把握しやすく保つ。

## 運用メモ

- このファイルは、プロダクト全体の進捗を見渡すための一覧として使う。仕様設計や decision log ではなく、横断的な作業状況の管理表として扱う。
- リリース台帳と、進行中タスクの索引を兼ねる。`release` skill はリリースごとに「リリース履歴」へ行を追加し、Issue 駆動タスクは依存確認と状態更新にこの表を使う(`doc/guidelines/issue-driven-task-execution.md`)。
- `working-branch-notes/` はブランチ単位の作業目的・状況・判断・引き継ぎメモを扱う。このファイルの 1 アイテムが必ずしも 1 ブランチに対応するとは限らない。
- 完了タスクが溜まる、リリース後、まとまった Issue 群を消化し終えた区切りなどで、`maintain-progress` skill を使って定期的に整理する。整理の観点(完了表の圧縮・現況/リリース履歴の更新・参照整合性と境界の維持)はこの skill が正本。
- この運用は暫定であり、実際の作業に合わせて軽く更新していく。

## 現況

v1.0.0 / v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 を GitHub Releases で公開済み。配布経路は単一バイナリ(GitHub Releases)、install script(`scripts/install.sh`)、Homebrew cask(`kiyohara/homebrew-tap`)の 3 つ。

現在は取得範囲指定(`--date` / `--from` `--to`)と、emoji 除外(privacy signal)を横断タスクとして追跡中。状態・依存・Issue / PR 参照だけを下の索引表で管理する。

## 取得範囲指定

| ID | Issue | 状態 | 依存 / 順序 | 次にやること | PR |
|---|---|---|---|---|---|
| range-01 | #153 | todo | - | `--date YYYY-MM-DD` で特定日の timeline 取得範囲を指定できるようにする | - |
| range-02 | #154 | todo | #153 | `--from` / `--to` で任意期間の timeline 取得範囲を指定できるようにする | - |

## emoji 除外(privacy signal)

| ID | Issue | 状態 | 依存 / 順序 | 次にやること | PR |
|---|---|---|---|---|---|
| exclude-01 | #155 | todo | - | `--exclude-body-emoji` で本文 shortcode 一致投稿を export から除外する | - |
| exclude-02 | #156 | todo | #155 | `--exclude-reaction-emoji` で指定 reaction 付き投稿を export から除外する | - |

## リリース履歴

| バージョン | 状態 | メモ |
|---|---|---|
| v1.0.0 | released | 初版。スコープは decision log 0036、リリース実施は PR #76 |
| v1.0.1 | released | user token default への認証方針転換(Issue #81 / PR #84) |
| v1.1.0 | released | token prompt / interactive selection / CLI output UX / 開発ループ整備を含む minor release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.1 | released | `--reuse-cache` の出力ディレクトリ検出改善、exported HTML header / footer 調整、logo asset 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.2 | released | 出力プレビュー・匿名化サンプル、token 不要 `--demo`、Slack App セットアップ help スクリーンショット、export footer への tool version 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |

## 完了済みフェーズ(参考)

詳細な経緯は decision log と各 Issue / PR を正本とする。ここでは到達点だけ残す。

- 設計基盤・AI agent 入口・ガイドライン整備 — done(PR #1 / #5 ほか)。
- 詳細仕様確定 / アーキテクチャ選定(Go + stdlib-first)/ PoC 実装による機能充足性確認 — done(decision log 0024〜0034)。
- v1.0 リリース実装プラン(v1-01〜v1-17: CI・テスト整備・`--reuse-cache`・コンテナ TZ・goreleaser・README/LICENSE・総合 E2E・リリース実施)— 全 done(Issue #15〜#31 / PR #33〜#76)。運用方式は decision log 0036 / 0037。
- post-v1 改善(配布・導入)— install script(#77 / PR #78)、Homebrew cask(#50 / PR #80)、cask 自動更新の release 検証(#79 / PR #85)、user token default 転換(#81 / PR #84)すべて done。経緯は decision log 0041。
- 1Password integration 整備 — token 注入 help と `op run` 経由の interactive selection を整備済み(Issue #53〜#54 / PR #96・#98)。
- 開発ループ整備 — 既存 Issue 登録 skill、Issue 駆動タスク実行 skill、開発ループ入口ドキュメントを追加済み(Issue #88〜#90 / PR #92〜#94)。
- CLI 出力 UX / 利用者向け help 基盤 / サンプル表示の初期整備 — styled/plain 出力(#100 / PR #102)、README 出力プレビューと同梱サンプル(#51 / PR #114)、Slack App セットアップ画像 help(#48 / PR #119)、FAQ(#52 / PR #128)、quickstart(#49 / PR #124)、README 再構成(#123 / PR #131)、文体・リンク方針(#125 / PR #127、#129 / PR #130)を整備済み。方針は decision log 0045 / 0048 / 0049。
- Agent / 開発環境整備 — worktree の project MCP local config セットアップ(#8 / PR #149、decision log 0051)。関連 follow-up として Cursor MCP command の相対 path 化(#151 / PR #152、decision log 0053)、GitHub MCP 優先規則・PR review skill・手動 resolve 運用(#161 / #162 / #165、PR #163 / #164 / #166)も完了。
- 利用者向け help / サンプル更新運用 — 文体一括修正(#126 / PR #143)、asset 内容 hash(#135 / PR #150、decision log 0052)、preview 枠線(#136 / PR #142)、sample / preview / demo 更新 skill(#137〜#139 / PR #158〜#160)、quickstart 全体フロー SVG(#140 / PR #157)を整備済み。
