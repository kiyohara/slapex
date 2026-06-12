# 0037 Issue 駆動タスク消化の運用方式

- 状態: decided
- 作成日: 2026-06-12
- 最終更新日: 2026-06-12
- 関連: `doc/guidelines/issue-driven-task-execution.md`, `progress.md`

## 背景

v1.0 実装プラン(0036)のタスクは、コンテキストサイズの小さい model(sonnet 等)を使う AI agent が 1 タスクずつ消化する。高い推論力を前提にできないため、タスクの粒度、指示の置き場所、終了条件を事前に固定する必要がある。

## 候補

- 指示の置き場所: (a) 各 Issue 本文に自己完結した指示を書く / (b) repo 内ドキュメントに全タスクの詳細を置き、Issue は参照だけにする。
- 粒度: (a) 1 Issue = 1 PR とし、1 パッケージまたは 1 関心事に限定する / (b) 機能単位の大きな Issue にする。
- merge: (a) ユーザーが merge する / (b) agent が self-merge する。

## 検討内容

- 小コンテキストの agent には「Issue を読めば目的・参照文書・作業内容・受け入れ条件・検証コマンドがすべて分かる」自己完結性が効く。repo 側に全タスク詳細を置くと Issue との二重管理になり drift する。
- 共通の進め方(ブランチ、note、検証、PR、merge しない等)は毎 Issue に複製せず、ガイドラインに 1 回だけ書いて Issue から参照する。
- 粒度の基準は「1 PR で 1 パッケージまたは 1 関心事、検証コマンドが明確、推測が必要な仕様判断を含まない」とする。仕様判断を含むタスクは、推奨方針を Issue に明記し、decision log の更新を作業内容に含める。
- これまでの 3 ステップ作業は agent の self-merge で進めたが、本フェーズは sonnet 利用のため品質ゲートを人間側に置く(2026-06-12 にユーザー確認)。
- タスクは直列消化とし、依存関係は `progress.md` のタスク表と各 Issue の「依存」欄で明示する。並列実行は想定しない。

## 決定

- 1 Issue = 1 ブランチ = 1 PR。Issue 本文を指示書として自己完結させ、共通手順は `doc/guidelines/issue-driven-task-execution.md` に置く。
- タスク一覧・順序・依存・状態は `progress.md` のタスク表を索引とする(別のプラン文書は作らない)。
- Issue title は `v1-<2 桁連番>: <内容>`、ブランチ名は `v1/<2 桁連番>-<slug>` とする。label / milestone は使わない(直列消化のため title の連番と progress 表で足りる)。
- 実行環境は local の Claude Code(model は sonnet 等)を想定する。repo の既存ルール(Docker Compose / GitHub MCP / 1Password / working branch note)がそのまま適用される前提で Issue を書く。
- PR の merge はユーザーが行う。agent は PR 作成と報告までを担当する。

## 理由

- 自己完結した Issue + 共通ルール参照の構成が、小コンテキスト agent の読み込み量を最小にしつつ、二重管理による drift を避けられるため。
- 人間 merge は、推論力の低い model の成果物に対する品質ゲートとして最も単純な仕組みのため。

## 影響

- `doc/guidelines/issue-driven-task-execution.md` を新設し、agent-configuration-management の checklist に従って入口(`.cursor/rules/` / `.claude/rules/` / `AGENTS.md`)を整備した。
- v1 タスク 17 件を GitHub Issue として登録し、`progress.md` のタスク表から参照する。

## 後から見直す条件

- タスク消化の品質・速度に問題が出た場合(粒度の再調整、低リスクタスクへの self-merge 導入、並列化の検討)。
- v1.0 完了後、post-v1 のタスクでも同方式を継続するかの判断。
