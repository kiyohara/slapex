# 作業ブランチメモ

- ブランチ: issue-97-interactive-token-prompt
- PR: #99
- 最終更新: 2026-07-02

## 目的

Issue #97: `SLACK_TOKEN` が未設定のとき、操作可能な端末があるときに限り token を安全に対話入力できるようにする。secret manager をまだ用意していない個人評価・PoC 利用者が、`export SLACK_TOKEN=...` のように実値を shell history へ残さずに token を渡せる導線を作る。

## 現在の状況

- 実装・ドキュメント・decision log・テストを一通り追加し、Docker 経由の build / vet / test まで完了。
- 手元の実ターミナルで手動 E2E を実施し、全シナリオ pass(詳細は「検証」)。
- PR #99 を作成済み。merge はユーザーが行う。

## 決定事項

- interactive 可否は controlling terminal (`/dev/tty`) を開けるかどうかで判定する。Issue 本文の「検討する条件」は stdin / stderr の TTY 判定を挙げていたが、既存の decision log 0043・`cli-interface.md`・`usage-flow.md` は `/dev/tty` 判定を確定済み。channel selection と同じ機構に揃え、`op run` の既定 secret masking(stdout / stderr が pipe 化)下でも動くようにする。経緯は decision log 0044 に記録。
- 入力は `golang.org/x/term` の `term.ReadPassword` で echo せずに読む(sudo / ssh / git と同じ作法)。値はプロセス内でのみ使い、設定ファイル・cache・log・HTML には書かない。
- 対象は `SLACK_TOKEN` のみ。他の環境変数への一般化・ファイル保存・secret manager 連携・CI 向け fallback はスコープ外(Issue のスコープ外に合わせる)。
- `--no-interactive` 指定時、および `/dev/tty` を開けない環境(CI / pipe)では従来どおり未設定エラー(exit 3)と案内を表示して終了する。
- decision log index の 0043 要約が確定内容(`/dev/tty`)と食い違って stdin/stderr のままだったため、同じ index 表に 0044 を足すのに合わせて 0043 要約も実装・0043 本文に整合させた。

## 次にやること

- この note は PR 採番に合わせて `99_issue-97-interactive-token-prompt.md` へ rename 済み。
- merge はユーザーが行う。

## 検証

### 自動テスト(Docker 経由)

- `docker compose run --rm --no-deps dev go build ./...` / `go vet ./...` / `go test ./...` — すべて pass。
- `gofmt -l cmd/slapex/` — 差分なし。
- 追加した単体テスト: `resolveToken` の分岐(env 優先 / tty なし / `--no-interactive` / 対話入力あり / 空入力)、`writeTokenPrompt` の文面。

### 手動 E2E(macOS の実ターミナル)

Docker で cross-compile した darwin/arm64 バイナリを、controlling terminal のある実ターミナルで実行して確認した。以下はすべて期待どおり。ワークスペース名・ID・token・出力先などの実値は本 note に残さない。

- 実 TTY で `SLACK_TOKEN` 未設定・`--no-interactive` なし: prompt を表示し、入力は echo されない(hidden)。有効な token を入力すると export が正常完了(exit 0)。進捗・診断・prompt は stdout に出ず、stdout の最終行は出力ディレクトリ path のみ。
- 同条件で無効な token を入力: prompt は入力を受理し、後続の Slack 認証で失敗(exit 3)、setup help URL を表示。prompt が token 解決経路に入っていることを確認。
- `--no-interactive` 指定・`SLACK_TOKEN` 未設定: prompt を出さず未設定エラー(exit 3)。
- controlling terminal を持たない session(`/dev/tty` を開けない状態)で実行: prompt を出さず未設定エラー(exit 3)。CI / pipe 相当で deterministic に非対話へ倒れることを確認。
- `SLACK_TOKEN` を環境変数で設定した既存経路(回帰): prompt を出さず、そのまま認証処理へ進む。
- 無漏えい確認: 有効な token で `--keep-cache` 付き実行後、生成物と保持した cache を token 形式で grep して一致なし。入力した token が HTML / assets / cache / log / stdout / stderr に残らないことを確認。

## リスク・ブロッカー

- 実 TTY 上の対話 UX(prompt 表示・hidden 入力・貼り付け)は CI / 単体テストでは再現できないが、macOS の実ターミナルで手動 E2E 済み(「検証」参照)。未解決のブロッカーはなし。

## セッションログ

- 2026-07-02: Issue #97 着手。context 読み込み、`/dev/tty` 方針で実装方針確定。ブランチ作成、note 作成。
- 2026-07-02: 実装・テスト・docs・decision log 0044 を commit、build / vet / test 通過。PR #99 作成。note を採番(`99_...`)。
- 2026-07-02: darwin/arm64 ローカルビルドで手動 E2E を実施し、prompt 表示 / hidden 入力 / 非対話 fallback(`--no-interactive` と /dev/tty 不在)/ env 経路の回帰 / 無漏えいを確認。結果概要を「検証」に記録。
