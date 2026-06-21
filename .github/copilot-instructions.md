# Copilot Review レビューガイドライン

I want to review in Japanese.

## レビューの目的

- 目的は「マージ前に修正すべき問題を見つけること」であり、改善案を網羅的に列挙することではない。
- 重要な問題が無ければ、追加コメントを生成しないことを正しい結果とする。
- 現段階の本リポジトリは設計段階であり、実装固有の技術スタックはまだ確定していない。

## 原則

- `Pull Request Overview` を含め、すべて日本語で出力する。
- すべてのコメントに、内容に応じて次のいずれかの prefix を必ず付ける。

| prefix | 使う場面 | マージへの影響 |
| --- | --- | --- |
| `[must]` | correctness / security / reliability / data integrity / 秘密情報混入に影響する問題 | マージ前に修正が必要 |
| `[ask]` | 意図や前提の確認が必要 | 回答次第で要修正 |
| `[imo]` | 保守性向上の提案 | 影響なし(任意) |
| `[nits]` | 軽微な改善提案 | 影響なし(任意) |
| `[fyi]` | 情報共有のみ | 影響なし |

## 指摘の優先順位

重要度の高い順に提示する。

1. correctness(正しく機能するか、設計意図と矛盾しないか)
2. security / privacy
3. reliability
4. data integrity
5. maintainability
6. guideline 違反

## 原則として指摘しない事項

マージを妨げない限り、次は指摘しない。指摘する場合も `[imo]` / `[nits]` / `[fyi]` に留め、`[must]` にしない。

- 個人の好みに依存する命名
- コードスタイル / フォーマット
- 任意のリファクタリング
- 将来的な改善提案
- 設計段階のひな形ドキュメントにおける未記入項目

## 設計ドキュメントのレビュー観点

- `doc/design/usage-flow.md` は利用者の操作の流れ(利用体験と CLI 挙動)の設計文書である。利用者が実行できない手順、前提の欠落、実装アーキテクチャ判断に影響する矛盾があれば指摘する。
- `doc/design/output-format.md` は出力ディレクトリ構造、保存 assets、取得範囲、サイズ制限の仕様である。出力構造・保存対象・制限値の矛盾や、`usage-flow.md` との不整合があれば指摘する。
- `doc/design/html-rendering.md` は生成する `index.html` の表示仕様(見た目)である。JavaScript 不使用や CSS 分離など `0012` の方針との矛盾があれば指摘する。
- `doc/design/cache.md` は中間ファイル `.cache/` の扱いである。成果物と中間ファイルの分離、cache 再利用・削除方針の矛盾があれば指摘する。
- `doc/help/*.md` は利用者が GitHub 上で直接読む help である。CLI から案内される URL、実行手順、scope、token、secret の扱いに矛盾があれば指摘する。
- `progress.md` は作業状況の把握を目的とする。進捗、TODO、検証状況、ブロッカーが混ざって読めなくなる変更は指摘する。
- `doc/design/decision-log/index.md` は方針決定ログの入口である。詳細議論を詰め込みすぎる変更、個別ログへの参照漏れ、現在有効な方針と未決事項の混同を指摘する。
- `doc/design/decision-log/*.md` は 1 テーマ 1 ファイルの詳細ログである。背景、候補、検討内容、決定、理由、影響、見直し条件のいずれかが判断に必要なのに欠けている場合は指摘する。

## Working Branch Notes のレビュー観点

- `working-branch-notes/**/*.md` は作業メモであり最終仕様書ではない。
- note 内の細かな整合性(stale 表現、最終実装との 1:1 整合、note 内記述同士の整合)は指摘しないか `[fyi]` に留める。
- 秘密情報・個人情報・顧客固有情報の混入は必ず `[must]` で指摘する。

## 情報統制

Slack token、個人情報、顧客固有情報、出力 HTML に含まれる機密情報、実 Slack ワークスペースの秘匿すべき識別情報が混入している場合は `[must]` で指摘する。

## 認証情報の送信先スコープ

- `Authorization` / `Cookie` / `X-API-Key` などの header 追加・変更は security-sensitive として扱う。
- 認証情報は明示 allowlist の host にだけ送る。host 判定なしに共通 HTTP client / downloader へ認証情報を持たせる変更は `[must]` で指摘する。
- URL preview 画像、URL preview service icon、avatar、emoji などの public asset URL へ Slack bot token を送る変更は `[must]` で指摘する。
- 認証情報付与条件を広げる変更では、allowlist 外へ送られない negative test と、必要な host へ送られる positive test があるか確認する。

## 実装 PR が始まった後

実装言語、フレームワーク、テスト方針は Go / stdlib-first / Docker Compose 経由の Go test として確定している。対象 path 別の詳細レビュー観点は `.github/instructions/*.instructions.md` に置く。

`.github/instructions/*.instructions.md` を追加する場合、各 instruction file は先頭から約 4,000 文字のみ反映される前提で、高シグナルな要点に絞る。

## 最終指示

レビューの目的は改善案を大量に列挙することではなく、マージ前に修正すべき重要な問題を見つけることである。重要な問題が無ければ、追加コメントを生成しなくてよい。
