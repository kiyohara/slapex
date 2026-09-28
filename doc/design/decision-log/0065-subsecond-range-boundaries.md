# 0065 取得範囲の境界の秒未満

- 状態: decided
- 作成日: 2026-09-28
- 最終更新日: 2026-09-28
- 関連: `doc/design/cli-interface.md`, `doc/design/output-format.md`, `doc/design/cache.md`, `doc/design/slack-api-usage.md`, [0011-channel-html-and-fetch-limits.md](0011-channel-html-and-fetch-limits.md)

## 背景

`--from` / `--to` は RFC3339 / RFC3339Nano と、秒の後に小数部を続けた local datetime を受け付け、開始が終了より前かを秒未満まで含めて判定していた。一方、Slack API へ渡す境界(`oldest` / `latest`)は秒に切り捨てていた(`%d.000000`)。client 側の範囲の判定もその値で行うため、`--from 2026-07-03T09:30:00.2Z --to 2026-07-03T09:30:00.8Z` は判定を通るが、`oldest` と `latest` が同じ値になり、取得結果は 0 件でエラーにならなかった。footer の `Range` と `.cache/metadata.json` の境界も RFC3339(秒)で、入力の秒未満を示さなかった。Issue #210(PR #201 の review 補助分析で見つかった)。

`--days` の範囲は、内部では実行時刻(秒未満を持つ)を終了境界としていた。Slack へ渡すときと footer に表示するときに、それぞれ秒へ切り捨てていたため、両者は同じ時刻を指していたが、範囲そのものとは最大 1 秒ずれていた。

## 候補

- A: 境界を秒未満まで使う。Slack API へは Slack の ts 形式(小数 6 桁)で渡し、footer と metadata も秒未満まで示す。
- B: 秒未満を含む `--from` / `--to` を usage error として拒否する。境界と表示は秒単位のままとする。

## 検討内容

- 仕様(`cli-interface.md`)は RFC3339Nano を受け付けると定めている。B は `--from` / `--to` についてこれを狭める。ログや JavaScript の `toISOString()` が出す `.123Z` 付きの時刻、`date +%N` を使った出力をそのまま指定すると失敗するようになり、今は受け付けている指定も usage error に変わる。
- A は受け付ける入力を変えない。`conversations.history` は `oldest` / `latest` に小数付きの ts を受け付け、slapex の取得も 2 ページ目以降は `latest` に投稿の ts(小数 6 桁)を渡している。client 側の範囲の判定(`timestampInRange`)は float64 で比べるが、epoch 秒が 2^33(2242 年ごろ)に達するまでは、小数 6 桁で異なる 2 つの ts の大小を取り違えない。今の epoch 秒で境界の前後 1 マイクロ秒の ts を判定できることは、test で確かめた。
- 秒単位の入力、`--date`、`--days` の出力(Slack へ渡す値、進捗表示、footer、metadata)は、A でも B でも変わらない。A で表示を RFC3339Nano にしても、秒ちょうどの境界は RFC3339 と同じ文字列になる。
- Slack の ts はマイクロ秒単位だが、RFC3339Nano はナノ秒まで書ける。マイクロ秒より細かい部分を切り捨てると、開始境界の直前の ts を含み得る。切り上げれば、開始(含む)と終了(含まない)のどちらでも、範囲に入る ts が入力どおりになる。拒否する案もあるが、`date +%N` のような 9 桁の出力を指定すると失敗する。切り上げで開始と終了が同じ値になる範囲(同じマイクロ秒の中の範囲)には ts が入り得ないため、判定は切り上げた後の境界で行い、usage error とする。
- `--days` は入力が日数で、秒未満の精度を持たない。終了境界に実行時刻の秒未満まで使うと、既定の mode の footer と metadata に秒未満が出るようになる。実行時刻の秒未満を切り捨てた時刻を範囲そのものにすれば、Slack へ渡す値、footer、metadata が範囲と同じ時刻を指し、出力も今と変わらない。

## 決定

A を採用する。

- `--from` / `--to` の境界は秒未満を含めて使い、マイクロ秒より細かい部分は切り上げる。開始が終了より前かは、切り上げた後の境界で判定する。
- Slack API へは、境界を小数 6 桁の ts で渡す。
- 進捗表示、footer の `Range`、`.cache/metadata.json` の `fetch.target_range.start` / `end` は RFC3339Nano で書く。秒ちょうどの境界は従来と同じ文字列になる。
- `--days` は、実行時刻の秒未満を切り捨てた時刻を終了境界とし、その `--days` × 24 時間前を開始境界とする。
- `--date` は変えない。範囲は対象日の 00:00 からの 1 日で、秒未満を持たない。

## 理由

- 受け付ける入力を変えずに、入力の精度、Slack へ渡す境界、表示を揃えられる。
- 秒単位の入力と `--date` / `--days` の出力が変わらない(Issue #210 の完了条件)。
- 変更は境界の値と書式に収まり、取得の経路(API の呼び出し方と client 側の範囲の判定)は変えない。

## 影響

- 実装: `internal/export/fetch_range.go`(`--days` の終了境界、`--from` / `--to` の切り上げ、表示)、`internal/export/cache.go`(`target_range`)、`internal/slack/api.go`(境界を ts にする関数)。
- test: 秒未満の `--from` / `--to`、切り上げ、秒単位の入力の出力、`--days` の unit test と、境界の前後の ts を持つ fake server の結合 test。
- 文書: `cli-interface.md`、`output-format.md`、`cache.md`、`slack-api-usage.md`、`doc/help/usage.md`。
- `.cache/metadata.json` の `schema_version` は変えない。`oldest_ts` / `latest_ts` / `start_slack_ts` / `end_slack_ts` は従来も小数 6 桁の文字列で、`start` / `end` の ISO 8601 が秒未満を含み得るようになるだけである。

## 後から見直す条件

- Slack の ts の小数部の桁数、または `oldest` / `latest` の扱いが変わった場合。
- `--days` などの相対範囲に秒未満の精度が要る要件が出た場合。
