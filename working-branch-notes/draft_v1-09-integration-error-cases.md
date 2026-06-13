# 作業ブランチメモ

- ブランチ: `v1/09-integration-error-cases`
- PR: (採番前)
- 最終更新: 2026-06-13

## 目的

v1.0 リリース実装プランのタスク 09/17(Issue #23)。v1-07 の fake Slack server 統合テスト
ハーネスに、エラー経路と rate limit 挙動のシナリオを追加し、`export.Run` の返すエラー型と
exit code 分類(`cli-interface.md`)の対応を end-to-end で固定する。429 経路は実 E2E で安全に
再現できないため、ここが実質的な検証点になる。

参照:

- `doc/design/cli-interface.md`(exit code、部分失敗の扱い)
- `doc/design/slack-api-usage.md`(rate limit とリトライ)
- `doc/design/usage-flow.md`(情報が足りない場合の案内)

## 現在の状況

- 依存 v1-07(Issue #21 / PR #39)は progress.md で done を確認済み。v1-08(PR #40)も merge 済み。
- 既存ハーネス(`internal/export/integration_test.go`)、rendering シナリオ
  (`integration_rendering_test.go`)、retry 実装(`internal/slack/client.go`)、
  exit code 分類(`cmd/slapex/main.go` の `classify`)を読了。

## 決定事項

- ハーネス拡張は最小限とし、`exportScenario` に「fault 注入」フィールド(API endpoint / asset
  path ごとの一時的 429・5xx、持続的 slack error、持続的 429/5xx)を追加する。既存シナリオは
  zero value で従来どおり happy に動く(後方互換)。
- エラー経路の test は `Run` のエラーを fatal にしない raw ヘルパー(`runExportScenarioRaw`)を
  追加して使う。sleep の注入は fake sleeper で記録し、実時間待機しない(case 4 の待機 assert に使用)。
- **exit code の assert の置き場所**: `classify` は package main にあり `internal/export` から import
  できないため、(a) `internal/export` 側の test では `Run` の返すエラー型/コードを `errors.As` で
  厳密に assert し、(b) exit code と help URL の案内(`usage-flow.md`)は `cmd/slapex` 側で実コードに
  対して assert する。`cmd/slapex/main.go` から error 表示部を `reportRunError(w, err) int` に抽出し、
  `TestReportRunError` で「auth 系 → exit 3 + help URL」「channel_not_found → exit 2」
  「retry 上限到達の plain error → exit 4」「UsageError → exit 2」を確認する。両者で「エラー型 →
  分類(2/3/4)」の対応を end-to-end に固定する(`classify` 自体の網羅は v1-03 / Issue #17 で実施済み)。
- exit 2(対象を確定できない)は Issue の 7 ケースに含まれないが、受け入れ条件が 2/3/4/部分失敗 0 の
  assert を要求するため、`internal/export` 側に「キーワード不一致 → UsageError」シナリオを 1 件追加し、
  `cmd/slapex` 側で channel_not_found / UsageError → exit 2 を assert する。

## 次にやること

- PR 作成 → progress.md の PR 列記入 → note リネーム(`number-working-branch-note`)。

## 検証

すべて Docker Compose(`dev` service、`--no-deps`)経由で実行。2026-06-13 時点で全 pass、
実時間 sleep なし(export パッケージ全体で 0.05 秒)。

- `go test ./internal/export/... -v`: 既存 + 追加 8 本(`TestRunIntegration*` のエラー系)が pass。
- `go test ./...`: 全パッケージ ok(`cmd/slapex` の `TestReportRunError` 追加分含む)。
- `gofmt -l .`: 出力なし(整形差分なし)。
- `go vet ./...`: 出力なし(指摘なし)。

追加ケースと対応 test function(Issue #23 の番号順):

1. `auth.test` invalid_auth → `TestRunIntegrationAuthInvalid`(APIError invalid_auth → exit 3)
2. history missing_scope → `TestRunIntegrationMissingScope`(APIError missing_scope → exit 3 + help URL)
3. history not_in_channel → `TestRunIntegrationNotInChannel`(APIError not_in_channel → exit 3 + help URL)
4. 429+Retry-After 1 回 → `TestRunIntegrationRateLimitRetryThenSuccess`(成功・待機 sleep 記録・進捗表示)
5. history 429 継続 → `TestRunIntegrationRateLimitExhausted`(5 回再試行後 plain error → exit 4、history 6 回)
6. asset 5xx 継続 → `TestRunIntegrationAssetDownloadRetriesThenFails`(export 成功・manifest failed・download 6 回)
7. 一時的 5xx → `TestRunIntegrationTransientServerErrorRecovers`(バックオフ再試行で成功・retry 進捗)
   - 受け入れ条件の exit 2 補完: `TestRunIntegrationNoChannelMatch`(UsageError → exit 2)
- cmd 側 exit code + help URL: `cmd/slapex.TestReportRunError`(invalid_auth/missing_scope/not_in_channel
  → exit 3 + help URL、channel_not_found / UsageError → exit 2、retry 上限の plain error → exit 4)

## リスク・ブロッカー

- ネットワーク断など httptest で表現しにくい異常系はスコープ外(Issue 明記)。
- リトライ方針・exit code 仕様そのものは変更しない(スコープ外)。

## セッションログ

- 2026-06-13: main を最新化(PR #40 merge 済みを確認)し作業ブランチ作成。Issue #23、
  ハーネス・retry・classify を読了。fault 注入によるハーネス拡張方針と、exit code assert の
  置き場所(export 側=エラー型、cmd 側=exit code + help URL)を確定。
- 2026-06-13: ハーネスに fault 注入(`endpointFault` / `faultResponse`、`APIFaults` /
  `AssetFaults`、`nextFault` / `writeFault`)を追加し、`runExportScenario` を
  `runExportScenarioRaw`(sleep 記録・error 非 fatal)へ委譲する形に整理。エラー系 8 本を
  `integration_error_test.go` に追加。`cmd/slapex` の error 表示部を `reportRunError` に抽出し
  `TestReportRunError` を追加。全検証 pass(実時間 sleep なし)で PR 作成へ。
