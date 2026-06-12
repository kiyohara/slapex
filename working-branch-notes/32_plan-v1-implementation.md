# 作業ブランチメモ

- ブランチ: plan-v1-implementation
- PR: #32
- 最終更新: 2026-06-12

## 目的

本実装フェーズ(v1.0 リリースまで)のプランニング。タスクの洗い出し・分解・順序付けを行い、progress.md のタスク表と GitHub Issue(1 Issue = 1 PR 粒度、sonnet 等の小コンテキスト model で消化可能な粒度)として記録する。各 Issue は AI agent への指示書として自己完結させる。

## 現在の状況

- ユーザーへの事前確認(2026-06-12)で 3 点確定: (1) 到達点は v1.0 リリースまで、(2) 消化 agent は local の Claude Code(sonnet 等)、(3) PR の merge はユーザーが行う。
- スコープを decision log 0036、運用方式を 0037 に記録。
- `doc/guidelines/issue-driven-task-execution.md` を新設し、rule 入口(.cursor / .claude / AGENTS.md)を整備。
- v1 タスク 17 件を GitHub Issue として登録し、progress.md にタスク表を追加。

## 決定事項

- プラン文書は新設せず、progress.md のタスク表(索引)+ 自己完結 Issue(指示書)+ decision log(経緯)の 3 点構成とする(0037)。
- タスク順序: CI を最初に置き(以降の全 PR を自動検証)、ユニットテスト → 統合テストハーネス → 機能実装(reuse-cache / TZ / actor 表示) → リリース整備(goreleaser / workflow / README・LICENSE) → 総合 E2E → リリース実施、の直列とする。
- 統合テストハーネス(fake Slack server)を 1 タスクで作り、以降のシナリオ追加・機能実装のテストはハーネスへの fixture 追加で行う(小コンテキスト model に有利な「既存構造への追加」型に揃える)。
- PoC の E2E 未確認項目(サイズ超過置換、tombstone、429 待機、fenced code block 等)は統合テスト fixture で網羅し、TTY interactive selection と配布形態の実機確認は最終 E2E(ユーザー協働)で行う。

## 次にやること

- ユーザーによる PR #32 のレビューと merge(本プラン PR から人間 merge 運用に切り替え)。
- merge 後、ユーザーが Issue #15(v1-01)から 1 件ずつ指定して消化を開始する(kickoff prompt は issue-driven-task-execution.md 参照)。

## 検証

- ドキュメントのみの変更のため build / test は対象外。
- rule 入口 3 箇所(doc/guidelines / .cursor/rules / .claude/rules)の basename 一致と AGENTS.md リンク追加を確認。
- progress.md のタスク表の Issue 番号と GitHub 上の Issue の対応を作成後に確認。

## リスク・ブロッカー

- Issue 本文と仕様文書の食い違いが消化中に見つかる可能性: issue-driven-task-execution.md の「判断に迷ったとき」で停止・報告する運用にして吸収する。
- v1-07(統合テストハーネス)が 17 タスク中で最も重く、sonnet には難所になり得る。Issue に設計指示(ファイル配置・ヘルパー構成・リファクタ範囲)を厚めに書いて緩和する。

## セッションログ

- 2026-06-12: ユーザー依頼でプランニング開始。事前確認 3 問(到達点 / 実行環境 / merge 運用)に回答を得て、仕様文書・guideline・PoC note・コード構造を読み込み。タスク 17 件に分解し、decision log 0036/0037、issue-driven-task-execution.md、progress.md タスク表、GitHub Issue 17 件を作成。
