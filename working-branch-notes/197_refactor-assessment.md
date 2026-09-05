# 作業ブランチメモ

- ブランチ: `refactor-assessment`
- Issue: [#188](https://github.com/kiyohara/slapex/issues/188)
- PR: [#197](https://github.com/kiyohara/slapex/pull/197)
- 最終更新: 2026-09-05

## 目的

コード規模、責務分担、命名、ドキュメントを調査し、施策の採否と根拠を記録して採用施策を Issue 化する。

## 現在の状況

調査・評価を完了し、17候補のうち8件を採用して Issue #189〜#196 に登録した。基準コミットは `d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55`。開始時の main は origin/main と一致し、作業ツリーは clean、open Issue / PR は 0 件であった。

## 決定事項

- 手書きコードの重複・不要処理削減を主目的とし、可読性・テストを犠牲にしない。
- CLI・出力内容・cache 形式の互換性を維持する。工数上限を設けず候補を評価して段階化する。
- 本体・テスト・開発ツール・ドキュメントを対象とする。個別施策の実装は後続 Issue で行う。
- 採用・保留・不採用を記録する。採用 Issue にも根拠を残し、note cleanup 後に判断が失われないようにする。
- 段階的な整理方針と比較判断を [decision log 0056](../doc/design/decision-log/0056-incremental-refactoring-plan.md) に記録する。個々の詳細設計は後続 Issue で検証する。

## 調査方法・範囲

- `git ls-files` の tracked file を分類し、改行区切りの物理行数(空行・コメント込み)を集計した。LOC は複雑度や不要コード量そのものではない。生成物、テスト、手書き本体を分けた。
- 本体の package、主要関数、呼び出し元、JSON/template の参照、既存テスト、Go module と CI/Compose を確認した。特に export / CLI / retry / cache を詳細に読んだ。
- README、help の入口と構成、現行設計 spec、decision log index と関連する依存/demo/記録方針を照合した。全ての過去 note を精読したわけではない。
- 閉鎖済み Issue を `refactor` / `リファクタリング` / `整理` で検索した。前二者に既存改善 Issue はなく、`整理` の33件から既存の README/help 再編、demo共通化、進捗索引運用を確認した。検索語による確認は全履歴の意味的な重複検出を保証しない。
- 履歴上の #88/#90、#113、#123/#145、#154/#170、#182 と既存設計を尊重した。既に完了した利用者文書再編や demo 共通ドライバを新規施策として重複登録しない。

## 規模の基準値

| 区分 | ファイル数 | 物理行数 | bytes |
|---|---:|---:|---:|
| Go 本体(cmd/internal、test除外、架空demo fixture含む) | 19 | 5,453 | 179,225 |
| Go 開発ツール | 3 | 799 | 25,789 |
| Go test | 18 | 7,201 | 234,229 |
| 既存 working branch note(README/template含む) | 107 | 5,883 | 405,622 |
| decision log(README/index/template含む) | 58 | 3,335 | 279,261 |
| その他 Markdown | 63 | 4,756 | 361,781 |

Go は合計40ファイル、13,453行。test が約54%を占める。本体のうち `internal/demo` は1,105行で、ja/en の長い scenario 関数は主に架空データ literal である。これを業務ロジックの巨大関数と同じ基準で削減しない。

別集計: `doc/samples/` は37ファイル/82,171 bytes、組込み emoji JSON は48,881 bytes。sample には画像もあるため行数は評価指標にしない。render template は120行、CSSは458行。バイナリサイズ、実行時間、循環的複雑度、将来の削減率は測定していない。

| package | 非test Go行数 | test行数 |
|---|---:|---:|
| cmd/slapex | 537 | 846 |
| internal/export | 1,697 | 3,843 |
| internal/slack | 721 | 894 |
| internal/output | 536 | 654 |
| internal/render | 362 | 469 |
| internal/ui | 320 | 180 |
| internal/demo | 1,105 | 225 |
| internal/emoji | 135 | 41 |
| internal/datetime | 40 | 49 |

`export.go` は1,501行、`Run` は346行(72–417)、`parseCLIArgs` は164行(335–498)。行数を理由に一律分割せず、独立する責務と変更時の重複を評価した。

## 候補評価

工数は相対見積もり(S:局所、M:複数箇所、L:横断する状態処理)。実装日数の約束ではない。削減量は実装後の差分で測り、抽出による追加型やテストも含めて報告する。

### 採用

| ID / Issue | 根拠と施策 | 効果 / 工数 / リスク | 採用理由・検証上の注意 |
|---|---|---|---|
| RF-01 / [#189](https://github.com/kiyohara/slapex/issues/189) | `integration_test.go` の fixture/server と検証を分離。`renderingOptions` / `reuseOptions` と反復する Options 初期化・HTML読込を集約 | 総量削減 中、可読性 高 / M / 中 | 現在も helper はあるため局所的な統合で済む。ケース固有の時刻・上限・期待値を隠さず、既存シナリオの対応表で消失を防ぐ |
| RF-02 / [#190](https://github.com/kiyohara/slapex/issues/190) | `export.go` の range/channel/view/cache/filter を同一 package の責務別ファイルへ分割。`builder`/`limit` を具体化 | 総量ほぼ不変、探索性 高 / M / 低〜中 | 機械的移動を先に行い、後続の論理変更と混ぜない。公開 package を増やさない |
| RF-03 / [#191](https://github.com/kiyohara/slapex/issues/191) | `Run` の fetch/resolve/view/output を工程化。`threadFetches` の bool値を廃し取得済み集合にする | 状態重複削減 小、理解容易性 高 / L / 高 | `newThreadIDs` はキー存在だけを参照する。除外は `messageFilter` が既に保持。親除外後の補充・broadcast・件数・phaseを保持する |
| RF-04 / [#192](https://github.com/kiyohara/slapex/issues/192) | `slack.Client.withRetry` / `downloadRetry` の429/5xx/待機/回数管理を共通化 | 総量削減 中、変更点集約 高 / M / 中〜高 | 明確な制御の重複。API body read errorの再試行とdownload streamingは異なるため保持。認証処理は経路別のまま |
| RF-05 / [#193](https://github.com/kiyohara/slapex/issues/193) | CLI通常/demo/export間の取得・出力option転記を集約、mainを責務別に配置 | 総量削減 小〜中、転記漏れ防止 高 / M / 中 | 追加optionの同期先を減らせる。demo固有のfake client/非対話/接続先制約は `demo.Export` に残す |
| RF-06 / [#194](https://github.com/kiyohara/slapex/issues/194) | `writeCaches` の19引数、特に連続する7個のintを責務別の入力型にまとめる。cache変換を同じ場所に集める | 総量増加も許容、取り違え防止 高 / M / 中 | 行数より呼出し契約の明瞭化に価値がある。JSONのnull/省略/空配列、legacy fields、旧bots無しcacheを維持 |
| RF-07 / [#195](https://github.com/kiyohara/slapex/issues/195) | architecture.md のPoC/Compose/release将来形、7 API表記、datetime/demo/ui欠落を更新。cache記述差も確認 | 情報重複削減 小、現行理解 高 / S〜M / 低 | 実装とspec双方で裏付け可能。`local_path` の未保存null記述とomitemptyは差異として扱い、黙って挙動を変更しない |
| RF-08 / [#196](https://github.com/kiyohara/slapex/issues/196) | decision-log-guidelines.md:46 の「試行錯誤をprogressへ」と、薄い索引方針の不整合を訂正。note長期情報の移転先を明確化 | 行数ほぼ不変、判断の一貫性 高 / S / 低 | 作業記録を肥大化させる矛盾した指示を解消する。rule本文をshimや入口へ増殖させない |

主なコード根拠は基準コミットの次の箇所。後続の移動で行が変わった場合は関数名を優先する。

- [export orchestration](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/export/export.go#L72)、[cache引数](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/export/export.go#L1091)、[thread集合参照](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/export/export.go#L1268)
- [API retry](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/slack/client.go#L147)、[download retry](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/slack/client.go#L256)
- [CLI通常転記](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/cmd/slapex/main.go#L128)、[demo転記](https://github.com/kiyohara/slapex/blob/d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55/internal/demo/export.go#L50)

### 保留(今回 Issue 化しない)

| ID | 候補 / 根拠 | 評価・見送り理由 | 再検討条件 |
|---|---|---|---|
| H-01 | demoと結合testのfake Slack server統合。両方にhandleAPI/writeSlackOKがある | 削減 中 / L / 高。external test package化で循環は回避可能だが、現状はtsTime/hostOf/testPrinter等の非公開helperに依存し移行作業を要する。主な保留理由はtest側の故障注入・request count・range未加工応答とdemo側range filteringが異なる | 共通serverの変更漏れが繰り返し発生し、機能差の吸収とexternal test化または下位fixture package化の費用を上回る場合 |
| H-02 | cache payloadを全面struct化、新cache package化 | 型安全性 中 / L / 中。動的range optionやnull/省略を維持する型が増える。RF-06の小さな入力型で主要問題を解消可能 | schema変更が頻発し、read/write不一致が実害を生んだ場合 |
| H-03 | `tools/genscreenshot/main.go`(514行)を複数moduleに分割、純粋crop/image helperのunit test追加 | 効果 中 / M / 中。独立関数とborder validationは既に存在。変更頻度・障害履歴を定量確認しておらず、今回の主要重複ではない | crop規則の追加や回帰発生時。純粋計算のtestを先に足す候補とする |
| H-04 | CLI/exportの日付検証とSlack timestamp変換を一律統合 | 削減 小 / M / 高。CLIの早期診断と直接export呼び出しの検証は別契約。`datetime.Parse`は既に共有。float timestampの精度改善は挙動変更を伴い得る | precision/境界の具体的な不具合、または新range mode導入時に別Issueで検討 |
| H-05 | 過去107 noteを一括削除・再編 | ファイル量削減 大 / L / 中。履歴の分類・移転なしでは判断を失う。プログラム保守対象の縮小とは別で、今回の記録目的とも衝突する | cleanup専用作業で、Issue/PR/decision logへの移転済み情報を識別できた時 |

### 不採用

| ID | 候補 | 評価と判断 |
|---|---|---|
| N-01 | 全面的な多層package化、汎用DI/interfaceの導入 | 工数 L、回帰リスク 高。現行はcmd→demo→export→slack/output/render/emoji/ui、小さなdatetimeの構成。明確な再利用需要なく境界を増やすと配線が増える。同一package内抽出から始める |
| N-02 | Slack SDK / CLI framework / rendererの置換による自前コード削除 | 工数 L、リスク 高。現行stdlib-first方針0033を変える根拠がない。外部へ移した行数を削減効果に数えず、現在のretry/flag/HTML契約を維持する。外部製品の最新優劣は今回評価していない |
| N-03 | テスト・コメント削除、コードの詰込み、生成sample/emoji削除で行数を減らす | 数値削減 大、理解・検証への悪影響 大。commentは境界条件を説明し、sample/emojiは成果物と実行時データである。目的に反する |
| N-04 | 全変数・関数名を一律長文化、utils/commonへ集約 | 工数 M〜L、効果 低。`ctx`/`err`/局所receiverまで変更しても責務は明確にならない。問題のあるbuilder/limitやthread状態はRF-02/RF-03の変更範囲で扱う |

## 依存・実施順序

- 全8施策は調査PR(#188対応)のmergeを着手条件とする。計画のレビュー・合意待ちであり、実装を開始していない。
- 必須依存: RF-03はRF-01/RF-02完了後、RF-06は#188のみを必須依存とし、RF-03の後は推奨順に留める。現行Runにもcache入力型を導入でき、先行時はRF-03がその型を再利用する。
- 推奨順: RF-08 → RF-07 → RF-01 → RF-02 → RF-03 → RF-06 → RF-04 → RF-05。最初に現行文書を整え、testの足場と機械移動を先にする。
- RF-07/RF-08、retry、CLIは技術的には分離可能だが、プロジェクト方針に従い全Issueを直列実行する。必須依存に単なる推奨順を混ぜない。
- RF-02/RF-03/RF-06は同じexportを触るので競合回避のため直列実行する。RF-06の技術的な着手条件にRF-03は含めない。各PRが自分の構成変更に必要な文書だけ同期する。
- `progress.md` はこの依存と状態の索引のみとし、詳細は各Issueを正本とする。

## 次にやること

調査PRをレビュー・mergeする。その後、progress.md の推奨順に個別Issueへ着手する。

## 検証

- `docker compose run --rm --no-deps -e TZ=Asia/Tokyo dev go test ./...`: 成功。9 packageが成功(うち8 packageはGo test cache)、3 toolsはtestなし。
- GitHub MCPから#189〜#196を1件ずつread-backし、作成本文との一致、open状態、comments 0、labels/assignees/milestone未設定を確認した。登録時点で関連open PRなし。
- 本体コード、既存spec、sample、画像はこの調査で変更していない。成果物はnote・計画decision log・索引である。
- `git diff --check`、ローカルMarkdownリンク存在、decision log indexの表構造、Issueと索引の依存、情報統制を確認した。秘密情報・実データの記録はない。

## リスク・ブロッカー

推定削減行数は実装前に保証できない。ファイル分割による局所的な縮小と総量削減を区別する。

この調査は全欠陥の検出や性能評価ではない。実Slack E2E、対話terminalの目視検証、browserによる見た目確認、バイナリサイズ測定は未実施。互換性の必要条件をIssueへ記載したが、実装時の回帰検証で確認する必要がある。

## セッションログ

- 2026-09-05: ユーザーと調査前提、Issue 起点、専用ブランチ、note と最小限の progress 更新を合意した。
- 2026-09-05: Issue #188 を作成し、最新 main からブランチを作成した。
- 2026-09-05: 規模集計とコード/spec/既存Issue確認により17候補を比較し、8採用・5保留・4不採用とした。
- 2026-09-05: #189〜#196を作成・再取得した。register-progress-issueの手順で索引化し、構造変更と論理変更の分離を記録した。

## PR review対応(1周目)

対象cycle: `claude-code-71c307b-20260905053435`。10件を検証した。

- 引数は19(対象等8 + counts7 + 解決情報等4)に訂正した。RF-06の採用理由は不変。
- RF-06のRF-03依存を解除した。現行Runでcounts型を導入でき、結果型をRF-03だけで設計する必然性はない。推奨順は維持する。
- RF-01は結合testのfixture/server/実行helper、RF-02はproduction関数に対応するunit test配置を担当する。RF-02でRF-01の共通fixtureを再移動しない。
- RF-01に通常成功時のphase完了順(Workspace → Channel → Messages → Users → Emoji → Assets → Done)のcharacterization testを追加する。既存の個別文字列assertだけでは順序を証明できない。test追加は後続#189で行う。
- RF-03のthread集合化は工程抽出と別commitにする。局所変更だけの新Issueは作らず、同じPR内でレビューを分離する。
- RF-02は機械移動と命名変更を別commitにし、移動を `git diff --color-moved=dimmed-zebra` で確認する。
- RF-05はexport.Options/demo.Optionsのfield形状を維持し、変換関数で集約する。埋め込み型への変更と既存literalの一括移行は対象外。転記総数を実測し、単なる移動を削減と扱わない。
- retry closure案は#192の設計候補として記録し、具体的な型設計は着手時に比較する。
- H-01は機能差を主要理由にした。external test化は可能だが、現行testがexported APIのみを使うという前提はtsTime/hostOf参照から成立しない。
- RF-02/RF-03/RF-05に固定時刻sample比較を追加する。cacheはsampleに含まれないため#194のJSON比較で補う。

### 集計の再現

repo rootで次を実行する。基準commitは上記SHA、変更後比較は `rev = "HEAD"` に置き換える。tracked blobのみを使い、空行・コメントを含む物理行数を同じ方法で数える。

```python
import subprocess
from collections import defaultdict
rev = "d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55"
totals = defaultdict(lambda: [0, 0, 0])
for path in subprocess.check_output(["git", "ls-tree", "-r", "--name-only", rev], text=True).splitlines():
    if not path.endswith(".go"):
        continue
    group = "test" if path.endswith("_test.go") else "tools" if path.startswith("tools/") else "production"
    data = subprocess.check_output(["git", "show", rev + ":" + path])
    totals[group][0] += 1
    totals[group][1] += len(data.splitlines())
    totals[group][2] += len(data)
print(dict(totals))  # files, physical lines, bytes
```

検証結果: Docker Composeで `-time 2026-07-04T16:32:41+09:00`、`TZ=Asia/Tokyo`、`-out /tmp/refactor-review-samples` を指定し、ja/enの `diff -r` は無差分。Z表記ではExportedのoffsetがUTCになる差を確認し、検証手順を訂正した。文書のみの修正のためGo testは再実行していない。
