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

- interactive 判定は stdin + stderr の TTY とし、stdout の機械可読契約を維持する。
- `huh` form は input を stdin、output を stderr に明示する。
- 仕様経緯は decision log 0043 として記録する。
- `script` 経由の workaround は主導線にせず、interactive selection が使えない環境では候補一覧に表示された channel ID / より具体的な channel 名で再実行する導線を help に置く。

## 次にやること

- PR #98 の review / merge 判断を待つ。

## 検証

- `docker compose run --rm dev go test ./...` 成功。
- `docker compose run --rm dev go build ./...` 成功。
- `docker compose run --rm dev go vet ./...` 成功。

## リスク・ブロッカー

- 実 token / 実 `op run` の E2E は token 実値を扱うため、この PR では自動検証しない。

## セッションログ

- 2026-07-02: Issue #54、依存 PR #96、関連仕様、1Password 公式 docs を確認して作業開始。
- 2026-07-02: stdin + stderr TTY 判定へ実装変更し、README / help / design spec / decision log / progress を更新。Docker Compose 経由の test / build / vet 成功。
