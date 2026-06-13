# 作業ブランチメモ

- ブランチ: `v1/08-integration-rendering-cases`
- PR: (採番前)
- 最終更新: 2026-06-13

## 目的

v1.0 リリース実装プランのタスク 08/17(Issue #22)。v1-07 で整えた fake Slack server
統合テストハーネスに、PoC の実 token E2E で未確認だった表示系経路を中心としたシナリオ
を追加し、`doc/design/html-rendering.md` / `output-format.md` の表示仕様を fixture で
網羅する。

参照:

- `doc/design/html-rendering.md`(subtype 表示 / 画像・添付の表示 / 時刻表示)
- `doc/design/output-format.md`(添付サイズ制限 / 1 thread 1000 件打ち切り)
- `doc/design/decision-log/0027-message-subtypes-rendering.md`
- `working-branch-notes/11_poc-implementation.md`(「E2E 未確認」一覧)

## 現在の状況

- 既存ハーネス(`internal/export/integration_test.go`)と rendering 実装
  (`internal/export/export.go`、`internal/render/*`、`internal/output/output.go`)を読了。
- Issue #22 の 13 ケースを、1 ケース 1 test function として追加する方針。
  各ケースは最小 fixture を `baseScenario()` から組み立てる。

## 決定事項

- ケースごとに独立した test function を作り、共有の最小 fixture builder と
  小さな assert ヘルパーで構成する(Issue の「1 ケースずつ独立」に従う)。
- date divider など timezone 依存の assert は、ハードコードせず `tsTime(...)` から
  期待値を算出する(直近の TZ flaky 修正と同じ方針)。
- 1000 件打ち切りの replies は fixture をコードでループ生成する(Issue の許可どおり)。
- 実装の挙動はすべて確定仕様(html-rendering.md / output-format.md)に一致しており、
  仕様と実装の食い違いは見つかっていない。表示仕様の変更はしない(スコープ外)。

## 次にやること

- 13 ケースのテスト追加 → 検証コマンド実行 → progress.md 更新 → PR 作成。

## 検証

すべて Docker Compose(`dev` service)経由で実行。2026-06-13 時点で全 pass。

- `go test ./internal/export/... -v`: 既存 + 追加 13 ケース(`TestRunIntegration*`)が pass。
- `go test ./...`: 全パッケージ ok。
- `gofmt -l .`: 出力なし(整形差分なし)。
- `go vet ./...`: 出力なし(指摘なし)。

追加ケースと対応 test function:

1. fenced code block(URL・複数行)→ `TestRunIntegrationFencedCodeBlock`
2. system 行(`channel_join` / `channel_topic`)→ `TestRunIntegrationSystemRows`
3. tombstone 親 + 通常 replies → `TestRunIntegrationTombstoneParent`
4. 未知 subtype(text 有/無)→ `TestRunIntegrationUnknownSubtype`
5. `me_message` / `bot_message` 表示名 → `TestRunIntegrationMeAndBotMessage`
6. 編集済み `(edited)` → `TestRunIntegrationEditedMessage`
7. `thread_broadcast` の二重表示 → `TestRunIntegrationThreadBroadcast`
8. 複数日 date divider → `TestRunIntegrationDateDividers`
9. `<!date^…>` fallback → `TestRunIntegrationDateTokenFallback`
10a. 添付サイズ超過置換 + manifest `skipped_size` → `TestRunIntegrationOversizeAttachment`
10b. 画像 original 超過(thumbnail 残存)→ `TestRunIntegrationOversizeImageOriginal`
11. download 404(部分失敗・export 成功)→ `TestRunIntegrationAssetDownloadFailure`
12. 1 thread 1000 件超打ち切り → `TestRunIntegrationRepliesTruncated`
13. 標準絵文字 Unicode / 不明 shortcode literal → `TestRunIntegrationEmojiRendering`

実装メモ: case 7 は thread を親の下にインライン展開する仕様上、broadcast の timeline
コピー(ts は親より後)が thread block の後ろに描画される。そのため「2 回出現 + thread
内に 1 回」を assert する形にした(出現順は thread→timeline)。

## リスク・ブロッカー

- 特になし。コンテナ内 local timezone は UTC になり得る点は date divider assert で
  考慮済み(0028、v1-11 で TZ forward を別途対応予定)。

## セッションログ

- 2026-06-13: main を最新化(PR #39 merge 済みを確認)し作業ブランチを作成。
  ハーネスと rendering 実装を読了。13 ケースの追加方針を確定。
