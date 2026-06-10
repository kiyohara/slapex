# 作業ブランチメモ

- ブランチ: select-architecture
- PR: #10
- 最終更新: 2026-06-10

## 目的

実装アーキテクチャの確定(3 ステップ作業の Step 2)。プログラミング言語とフレームワーク・ライブラリを比較選定し、`doc/design/architecture.md` と decision log に記録する。

## 現在の状況

- `doc/design/architecture.md` を新設(言語、依存方針、主要ライブラリ、内部構成、開発環境、配布方式)。
- decision log 0032(実装言語)/ 0033(依存方針とライブラリ)/ 0034(配布方式)を追加。
- 各仕様文書の「実装アーキテクチャは未確定」の前置きを `architecture.md` への参照に更新。

## 決定事項

- 実装言語は Go(現行安定版 1.26 系)。最重要基準「単一バイナリ配布」(ユーザー確認済み)に加え、標準ライブラリの守備範囲(HTTP / JSON / HTML 自動エスケープテンプレート)による依存最小化、クロスコンパイルの容易さ、リリース自動化の実績で総合判断。
- 候補比較: Rust(性能・サイズ最良だがネットワーク律速の本ツールで利点が効かず、依存ツリーとビルド時間で不利)、Deno / Bun compile(バイナリ 60〜100MB 級で配布に不利)、Python / Ruby(単一バイナリ化とクロスコンパイルが弱い)をそれぞれ理由付きで見送り。
- 依存方針は stdlib-first。外部依存は TTY interactive selection の charmbracelet/huh v2 と golang.org/x/term に限定。Slack API client は自前 thin client(使用 method 7 種、429 / Retry-After 制御を自前 transport で実装)。
- 配布は GitHub Releases に darwin/linux × amd64/arm64 の単一バイナリ添付、リリース自動化は goreleaser 想定。Homebrew tap は将来検討。

## 次にやること

- PR 作成、採番後に note rename、自己マージ。
- Step 3(PoC 実装)へ。PoC で thin client・huh・html/template の機能充足性を実 API E2E で確認する。

## 検証

- 文書のみの変更。ライブラリの現況(huh v2.0.3 が active、Go 1.26 系が現行)は公式情報で確認済み。
- 実装上の妥当性検証は Step 3 の PoC で行う。

## リスク・ブロッカー

- thin client 方針は Slack Web API のリクエスト形式の細部(エンコーディング、エラー形式)を自前で扱う。PoC の実 E2E で齟齬がないか確認する(問題があれば 0033 を見直す)。

## セッションログ

- 2026-06-10: PR #9(詳細仕様の確定)merge 後、`select-architecture` ブランチを作成。Go 1.26 系と huh の現況を確認し、比較選定を実施。
