# design

このディレクトリには、`slapex` の仕様設計と設計判断を置く。

## 置くもの

- 利用者体験や CLI 挙動の設計文書
- 出力形式、保存形式、取得範囲、制限値などの仕様設計
- 設計判断の履歴である decision log
- 実装前に合意したい仕様の素案

## 置かないもの

- 利用者がそのまま手順として読む help: `doc/help/` に置く
- AI agent / Git / GitHub / PR などの作業ルール: `doc/guidelines/` に置く
- 作業状況の一覧: `progress.md` に置く
- ブランチ単位の引き継ぎメモ: `working-branch-notes/` に置く

## 主な文書

- `usage-flow.md`: 利用者の操作の流れ(利用体験と CLI 挙動)
- `cli-interface.md`: CLI のコマンド形式、option、環境変数、exit code、対象プラットフォーム
- `architecture.md`: 実装言語、依存方針、主要ライブラリ、内部構成、開発環境、配布方式
- `output-format.md`: 出力ディレクトリ構造、保存 assets、取得範囲、サイズ制限
- `html-rendering.md`: 生成する `index.html` の表示仕様(見た目、本文変換、subtype、時刻表示)
- `slack-api-usage.md`: Slack API の利用方針(method、pagination、rate limit、解決系)
- `cache.md`: 中間ファイル `.cache/` の扱いと schema
- `decision-log/`: 方針決定ログ
