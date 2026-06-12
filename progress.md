# 進捗管理表

このファイルには、設計および実装作業の進捗を記録していく。

想定読者は、実装作業を行う人間、および AI agent である。

今後は、作業項目、状態、担当、次にやること、検証状況、未解決のリスクやブロッカーを管理する。詳細な検討経緯は `doc/design/decision-log/` に分け、このファイルでは現在の作業状況を把握しやすく保つ。

## 運用メモ

- このファイルは、プロダクト全体の進捗を見渡すための一覧として使う。
- 仕様設計や decision log ではなく、横断的な作業状況の管理表として扱う。
- `working-branch-notes/` はブランチ単位の作業目的、状況、判断、引き継ぎメモを扱う。
- このファイルの 1 アイテムが、必ずしも 1 ブランチに対応するとは限らない。progress の item は小さめのマイルストーンとして扱い、そのサブセットを複数のブランチで進めることがある。
- この運用は暫定であり、実際の作業に合わせて軽く更新していく。

## 進捗

| 項目 | 状態 | メモ |
|---|---|---|
| 設計ドキュメント基盤の整備 | done | `doc/design/` の文書分割と decision log 運用が定着(PR #1, #5) |
| AI agent 向け入口とガイドラインの整備 | done | `AGENTS.md`、tool 別 rule、GitHub / Git / PR / working branch note / MCP 関連ルールを整備済み。必要に応じて随時更新 |
| 試作プロジェクトとの関係整理 | done | `doc/design/decision-log/0001-relationship-to-prototype.md` に記録 |
| 利用者向け How to Use 素案 | done | `doc/design/` の仕様 4 文書と `doc/help/slack-app-setup.md` に分割整理済み(PR #2, #5) |
| 詳細仕様の確定(後続作業に必要な範囲) | done | `finalize-detailed-specs` ブランチ。`cli-interface.md` / `slack-api-usage.md` 新設、既存 4 文書の確定、decision log 0024〜0031 |
| アーキテクチャ選定(言語・フレームワーク) | done | `select-architecture` ブランチ。Go(1.26 系)+ stdlib-first を採用し、`architecture.md` と decision log 0032〜0034 に記録 |
| PoC 実装(機能充足性の確認) | done | `poc-implementation` ブランチ。Go で happy path を実装し、実 workspace への E2E で主要経路を確認。機能充足性に問題なし。`--reuse-cache` は PoC 未実装(note 参照)。追加 E2E(`poc-additional-e2e` ブランチ、実運用 channel 2 件)で system 行・複数ユーザー・実 unfurl・bot 未参加エラー経路も確認。目視レビュー所見 2 件のうち code block 内 URL は修正済み(decision log 0026 追記)、コンテナ実行時 TZ は未決事項のまま |
| 本実装(v1.0 リリースまで) | in_progress | スコープを v1.0 リリースまでと確定(decision log 0036)。タスクは下記「v1.0 リリース実装プラン」の表と GitHub Issue #15〜#31 で管理(運用方式は decision log 0037) |

## v1.0 リリース実装プラン

到達点は「GitHub Releases から単一バイナリを取得してすぐ使える v1.0.0 の公開」(decision log 0036)。

- 運用: 1 Issue = 1 ブランチ = 1 PR。タスクは表の上から順に直列で消化する。進め方の共通ルールは `doc/guidelines/issue-driven-task-execution.md`、運用方式の経緯は decision log 0037。
- 各 Issue 本文がタスクの指示書(目的 / 参照 / 作業内容 / スコープ外 / 受け入れ条件 / 検証)。
- PR の merge はユーザーが行う。agent は PR 作成と報告まで。
- kickoff prompt 例: `GitHub Issue #<番号> のタスクを、doc/guidelines/issue-driven-task-execution.md のルールに従って実施してください。`

| # | タスク | Issue | 依存 | 状態 | PR |
|---|---|---|---|---|---|
| v1-01 | CI 整備(gofmt / vet / build / test / クロスコンパイル) | #15 | なし | done | #33 |
| v1-02 | mrkdwn 変換のユニットテスト | #16 | v1-01 | done | #34 |
| v1-03 | CLI flag parse・検証・exit code 分類のユニットテスト | #17 | v1-01 | done | #35 |
| v1-04 | output の label 正規化・出力構造・cache 書き出しテスト | #18 | v1-01 | done | #36 |
| v1-05 | slack thin client のテスト(retry / 429 / pagination) | #19 | v1-01 | pending | |
| v1-06 | channel 解決ロジックのテスト | #20 | v1-01 | pending | |
| v1-07 | fake Slack server 統合テストハーネス + happy path | #21 | v1-05 | pending | |
| v1-08 | 統合テスト: 表示系シナリオ追加 | #22 | v1-07 | pending | |
| v1-09 | 統合テスト: エラー・rate limit シナリオ追加 | #23 | v1-07 | pending | |
| v1-10 | `--reuse-cache` の実装 | #24 | v1-07 | pending | |
| v1-11 | コンテナ実行時 TZ の解決(TZ forward) | #25 | v1-01 | pending | |
| v1-12 | system メッセージの actor 表示 | #26 | v1-08 | pending | |
| v1-13 | goreleaser 設定と version 埋め込み | #27 | v1-01 | pending | |
| v1-14 | リリース workflow(tag → Releases) | #28 | v1-13 | pending | |
| v1-15 | README と LICENSE | #29 | v1-13 | pending | |
| v1-16 | 実 token 総合 E2E(ユーザー協働) | #30 | v1-10〜v1-15 | pending | |
| v1-17 | v1.0.0 リリース実施と事後整理(ユーザー協働) | #31 | v1-16 | pending | |

依存列は「その Issue の作業開始前に done であるべきタスク」の最小セット。直列運用では基本的に直前までの全タスクが done になっている想定。タスク完了時の表の更新方法は `doc/guidelines/issue-driven-task-execution.md` を参照。
