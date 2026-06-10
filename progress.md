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
| PoC 実装(機能充足性の確認) | pending | 選定スタックで happy path を実装し、実 workspace への E2E で検証する。テストは対象外 |
