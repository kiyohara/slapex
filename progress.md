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

v1.0.0 / v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 / v1.2.0 / v1.2.1 を GitHub Releases で公開済み。配布経路は単一バイナリ(GitHub Releases)、install script(`scripts/install.sh`)、Homebrew cask(`kiyohara/homebrew-tap`)の 3 つ。

リファクタリングの調査・評価(#188)で採用した8施策を追跡する。実施方針は [decision log 0056](doc/design/decision-log/0056-incremental-refactoring-plan.md)、詳細な作業条件は各Issueを参照する。

## 進行中タスク: 段階的リファクタリング

調査PRのmerge後、表の順を推奨順として直列実行する。依存欄は必須条件のみとし、単なる推奨順は含めない。RF-03/RF-06は同じexportを触るため推奨順で直列実行するが、RF-06はRF-03なしでも着手可能。他の施策は技術的に分離可能だが、運用上は並行実行しない。全施策の現在の着手条件は調査PRのmergeである。

| ID | Issue | 状態 | 依存 | 次にやること | PR |
|---|---|---|---|---|---|
| RF-00 | [#188](https://github.com/kiyohara/slapex/issues/188) 調査・評価 | done(PR merge後) | - | merge後にRF-08へ | [#197](https://github.com/kiyohara/slapex/pull/197) |
| RF-08 | [#196](https://github.com/kiyohara/slapex/issues/196) 記録配置の整合 | done(PR merge後) | #188 | merge後にRF-07へ | - |
| RF-07 | [#195](https://github.com/kiyohara/slapex/issues/195) 現行設計文書 | todo | #188 | 現行構成とspecを突合 | - |
| RF-01 | [#189](https://github.com/kiyohara/slapex/issues/189) test準備集約 | todo | #188 | fixture/helperを整理 | - |
| RF-02 | [#190](https://github.com/kiyohara/slapex/issues/190) export分割・命名 | todo | #188 | 同一package内で機械分割 | - |
| RF-03 | [#191](https://github.com/kiyohara/slapex/issues/191) Run工程・状態整理 | todo | #188, #189, #190 | 工程の入出力を整理 | - |
| RF-06 | [#194](https://github.com/kiyohara/slapex/issues/194) cache入力整理 | todo | #188 | 同型位置引数を集約 | - |
| RF-04 | [#192](https://github.com/kiyohara/slapex/issues/192) retry共通化 | todo | #188 | streamingとの差を保って共通化 | - |
| RF-05 | [#193](https://github.com/kiyohara/slapex/issues/193) CLI option集約 | todo | #188 | 通常/demoの転記を整理 | - |

## リリース履歴

| バージョン | 状態 | メモ |
|---|---|---|
| v1.0.0 | released | 初版。スコープは decision log 0036、リリース実施は PR #76 |
| v1.0.1 | released | user token default への認証方針転換(Issue #81 / PR #84) |
| v1.1.0 | released | token prompt / interactive selection / CLI output UX / 開発ループ整備を含む minor release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.1 | released | `--reuse-cache` の出力ディレクトリ検出改善、exported HTML header / footer 調整、logo asset 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.2 | released | 出力プレビュー・匿名化サンプル、token 不要 `--demo`、Slack App セットアップ help スクリーンショット、export footer への tool version 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.2.0 | released | 取得範囲指定(`--date` / `--from` / `--to`)、emoji 除外(`--exclude-body-emoji` / `--exclude-reaction-emoji`)、footer timezone 改善、README / help 再構成を含む minor release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.2.1 | released | bot 投稿の投稿者名 / avatar を `bots.info` で解決し `APP` 表示を追加、asset の保存拡張子を download 内容から決定、PNG logo asset 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |

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
- 取得範囲指定 — 特定日の `--date`(#153 / PR #168)、任意期間の `--from` / `--to`(#154 / PR #169)、footer の timezone 表示改善(#170 / PR #177)を実装済み。
- emoji 除外(privacy signal) — 本文 shortcode の `--exclude-body-emoji`(#155 / PR #175)と、reaction の `--exclude-reaction-emoji`(#156 / PR #176)を実装済み。
