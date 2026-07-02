# 作業ブランチメモ

- ブランチ: issue-54-op-run-interactive-selection
- PR: #98
- 最終更新: 2026-07-02

## 目的

Issue #54 として、1Password CLI (`op run`) 経由でも channel の interactive selection を自然に使えるようにする。

## 現在の状況

- Issue #54 を確認済み。追加コメントはなし。
- 依存の #53 は `progress.md` 上で done、PR #96 は GitHub 上で merge 済み。
- 1Password 公式 docs を確認し、`op run` は secret を subprocess の環境変数として渡し、stdout/stderr の secret masking を行うことを確認した。
- 既存仕様では stdout は成功時の出力 path 専用、stderr は進捗・診断・候補 list・interactive prompt 用。
- 現行実装は interactive 判定に stdin と stdout の TTY を使っており、`op run` が stdout を TTY でない状態にする環境と相性が悪い。

## 決定事項

- interactive 判定は controlling terminal (`/dev/tty`) を read/write で開けるかどうかで行い、stdin / stdout / stderr の TTY 状態は判定に使わない。stdout は成功時 path 1 行、stderr は進捗・診断・候補 list の契約を維持する。
- `huh` form の input / output はいずれも `/dev/tty` handle に固定する。
- 仕様経緯は decision log 0043 に記録する(当初の stdin + stderr 判定案は、レビューの裏取りで既定 `op run` が stderr も pipe 化することが判明したため `/dev/tty` 案へ改訂)。
- `script` 経由の workaround は主導線にせず、interactive selection が使えない環境では候補一覧に表示された channel ID / より具体的な channel 名で再実行する導線を help に置く。

## レビュー指摘と対応(review response)

- レビュー(medium, comment)で、既定 `op run`(secret masking on)は stdout だけでなく stderr も pipe 化するため、stdin + stderr TTY 判定では Issue #54 の主目的(既定 `op run` で自然に対話選択)を満たせないと指摘。
- 対応として実装・doc を `/dev/tty`(controlling terminal)方式へ改訂。`op run` の既定 masking のままでも対話選択が動くようにした。
- 0043 は本 PR 内で新規作成した log のため、誤った版を残して 0044 で上書きするのではなく 0043 を直接改訂した。

## 次にやること

- 実 token + 実 `op run`(既定 masking)での対話選択 E2E を手動確認する(下記リスク参照)。
- レビューコメントへの返信を残し、第三者チェック → merge 判断を待つ。

## 検証

- `docker compose run --rm dev go test ./...` 成功。
- `docker compose run --rm dev go build ./...` 成功。
- `docker compose run --rm dev go vet ./...` 成功。

## リスク・ブロッカー

- 実 token / 実 `op run` の E2E は token 実値を扱うため、この PR では自動検証しない。
- `/dev/tty` が既定 masking の `op run` 配下でも実端末を指すことは 1Password 公式 docs / コミュニティと controlling terminal の一般仕様から確認済みだが、`huh` の対話 UI を実際に `/dev/tty` で描画する挙動は TTY を要するため自動テスト対象外。手動 E2E で確認する。

## セッションログ

- 2026-07-02: Issue #54、依存 PR #96、関連仕様、1Password 公式 docs を確認して作業開始。
- 2026-07-02: stdin + stderr TTY 判定へ実装変更し、README / help / design spec / decision log / progress を更新。Docker Compose 経由の test / build / vet 成功。
- 2026-07-02: PR #98 の medium レビューで、既定 `op run` が stderr も pipe 化する点を裏取り。実装・doc を `/dev/tty`(controlling terminal)方式へ改訂(判定・prompt 入出力とも `/dev/tty`)。0043 を改訂、usage-flow / cli-interface / README / help / progress を追随。`openTerminal` の unit test 追加。Docker Compose 経由の build / vet / test 成功。
