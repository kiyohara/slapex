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

v1.0.0 / v1.0.1 を GitHub Releases で公開済み。v1.1.0 はリリース準備中。配布経路は単一バイナリ(GitHub Releases)、install script(`scripts/install.sh`)、Homebrew cask(`kiyohara/homebrew-tap`)の 3 つ。

現在は post-v1 の CLI 出力 UX と利用者向け help / サンプル整備を横断タスクとして追跡中。状態・依存・Issue / PR 参照だけを下の索引表で管理する。

## 利用者向け help / サンプル整備

| ID | Issue | 状態 | 依存 / 順序 | 次にやること | PR |
|---|---|---|---|---|---|
| help-00 | #100 | done | help-01 / help-03 / help-04 前が望ましい | (完了)CLI 出力を styled / plain の 2 モード化し `--no-color` を追加。方針は decision log 0045 | #102 |
| help-01 | #51 | todo | help-00 後が望ましい | 出力プレビューと匿名化サンプル成果物を README / help から辿れるようにする | - |
| help-02 | #48 | todo | - | Slack App セットアップ help にスクリーンショット付き UI 操作手順を追加する | - |
| help-03 | #52 | todo | help-00 後が望ましい | 制限事項・FAQ help を作り README / design doc から要点だけ辿れるようにする | - |
| help-04 | #49 | todo | help-00 / help-01 / help-02 / help-03 後が望ましい | 初回利用クイックスタートをチェックリスト形式で追加する | - |

## リリース履歴

| バージョン | 状態 | メモ |
|---|---|---|
| v1.0.0 | released | 初版。スコープは decision log 0036、リリース実施は PR #76 |
| v1.0.1 | released | user token default への認証方針転換(Issue #81 / PR #84) |
| v1.1.0 | preparing | token prompt / interactive selection / CLI output UX / 開発ループ整備を含む minor release 準備 |

## 完了済みフェーズ(参考)

詳細な経緯は decision log と各 Issue / PR を正本とする。ここでは到達点だけ残す。

- 設計基盤・AI agent 入口・ガイドライン整備 — done(PR #1 / #5 ほか)。
- 詳細仕様確定 / アーキテクチャ選定(Go + stdlib-first)/ PoC 実装による機能充足性確認 — done(decision log 0024〜0034)。
- v1.0 リリース実装プラン(v1-01〜v1-17: CI・テスト整備・`--reuse-cache`・コンテナ TZ・goreleaser・README/LICENSE・総合 E2E・リリース実施)— 全 done(Issue #15〜#31 / PR #33〜#76)。運用方式は decision log 0036 / 0037。
- post-v1 改善(配布・導入)— install script(#77 / PR #78)、Homebrew cask(#50 / PR #80)、cask 自動更新の release 検証(#79 / PR #85)、user token default 転換(#81 / PR #84)すべて done。経緯は decision log 0041。
- 1Password integration 整備 — token 注入 help と `op run` 経由の interactive selection を整備済み(Issue #53〜#54 / PR #96・#98)。
- 開発ループ整備 — 既存 Issue 登録 skill、Issue 駆動タスク実行 skill、開発ループ入口ドキュメントを追加済み(Issue #88〜#90 / PR #92〜#94)。
