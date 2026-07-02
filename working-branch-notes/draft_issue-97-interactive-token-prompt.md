# 作業ブランチメモ

- ブランチ: issue-97-interactive-token-prompt
- PR: (未採番)
- 最終更新: 2026-07-02

## 目的

Issue #97: `SLACK_TOKEN` が未設定のとき、操作可能な端末があるときに限り token を安全に対話入力できるようにする。secret manager をまだ用意していない個人評価・PoC 利用者が、`export SLACK_TOKEN=...` のように実値を shell history へ残さずに token を渡せる導線を作る。

## 現在の状況

- 実装・ドキュメント・decision log・テストを一通り追加し、Docker 経由の build / vet / test まで完了。
- PR 作成前。

## 決定事項

- interactive 可否は controlling terminal (`/dev/tty`) を開けるかどうかで判定する。Issue 本文の「検討する条件」は stdin / stderr の TTY 判定を挙げていたが、既存の decision log 0043・`cli-interface.md`・`usage-flow.md` は `/dev/tty` 判定を確定済み。channel selection と同じ機構に揃え、`op run` の既定 secret masking(stdout / stderr が pipe 化)下でも動くようにする。経緯は decision log 0044 に記録。
- 入力は `golang.org/x/term` の `term.ReadPassword` で echo せずに読む(sudo / ssh / git と同じ作法)。値はプロセス内でのみ使い、設定ファイル・cache・log・HTML には書かない。
- 対象は `SLACK_TOKEN` のみ。他の環境変数への一般化・ファイル保存・secret manager 連携・CI 向け fallback はスコープ外(Issue のスコープ外に合わせる)。
- `--no-interactive` 指定時、および `/dev/tty` を開けない環境(CI / pipe)では従来どおり未設定エラー(exit 3)と案内を表示して終了する。
- decision log index の 0043 要約が確定内容(`/dev/tty`)と食い違って stdin/stderr のままだったため、同じ index 表に 0044 を足すのに合わせて 0043 要約も実装・0043 本文に整合させた。

## 次にやること

- PR 作成後、この note を `<PR 番号>_issue-97-interactive-token-prompt.md` へ rename する。
- merge はユーザーが行う。

## 検証

- `docker compose run --rm --no-deps dev go build ./...`
- `docker compose run --rm --no-deps dev go vet ./...`
- `docker compose run --rm --no-deps dev go test ./...`
- 追加した単体テスト: `resolveToken` の分岐(env 優先 / tty なし / `--no-interactive` / 対話入力あり / 空入力)、`writeTokenPrompt` の文面。
- token が stdout / stderr / cache / HTML に出ないことを、`term.ReadPassword`(no-echo)と、token を log しない実装で担保。手元の実 TTY での対話動作は未実施(TTY 依存部は既存の `selectChannel` と同様、単体テスト対象外)。

## リスク・ブロッカー

- 実 TTY 上での対話 UX(prompt 表示・hidden 入力・貼り付け)は CI / 単体テストでは再現できない。必要ならユーザー環境で手動確認する。

## セッションログ

- 2026-07-02: Issue #97 着手。context 読み込み、`/dev/tty` 方針で実装方針確定。ブランチ作成、note 作成。
