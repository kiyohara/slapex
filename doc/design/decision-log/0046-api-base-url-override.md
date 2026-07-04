# 0046 Slack API 接続先の内部 override(SLAPEX_API_BASE_URL)

- 状態: decided
- 作成日: 2026-07-04
- 最終更新日: 2026-07-04
- 関連: `../cli-interface.md`、`../../guidelines/credential-scope-guidelines.md`、[0040-credential-scope-for-asset-downloads.md](0040-credential-scope-for-asset-downloads.md)、Issue #115、Issue #113

## 背景

README 用のターミナルデモ GIF(Issue #115)は、実 workspace / 実 token を使わず、`tools/gensample` の架空 fixture を配信する fake Slack API server に対して実際の slapex バイナリを実行して録画する。しかし CLI には Slack Web API の接続先を差し替える手段がなく、`cmd/slapex/main.go` は `slack.New(token)` を直接呼んでいた(`slack.WithBaseURL` は統合テストと `tools/gensample` だけが使用)。同様の差し替えは、token 不要の demo / fixture 実行(Issue #113)でも必要になる可能性が高い。

## 候補

1. 隠し環境変数(`SLAPEX_API_BASE_URL`)で base URL を上書きする。ユーザー向けドキュメント・`--help` には載せない。
2. 正式な CLI option(例: `--api-base-url`)として公開する。
3. 録画専用のビルド(build tag や別 main)を用意する。

## 検討内容

- 候補 1 は、公開 CLI 仕様(`cli-interface.md` の option 一覧・環境変数)を変えずに済み、通常利用の挙動に影響しない。demo / 録画という内部用途に対して露出が最小。
- 候補 2 は公開仕様の拡張になるが、利用者が誤って第三者 host へ token を送る導線を正式仕様として増やすことになり、credential-scope 方針(default deny)と相性が悪い。現時点で利用者向けの需要もない。
- 候補 3 は配布物と録画対象が別物になり、「実際の CLI の操作イメージ」を録るという目的に反する。
- 接続先の上書きは Slack OAuth token の送信先が変わることを意味するため、`credential-scope-guidelines.md` の checklist に従い、override 未指定時に default(`https://slack.com/api/`)のままである negative test と、override 指定時のみ差し替わる positive test を追加する。

## 決定

候補 1 を採用する。内部環境変数 `SLAPEX_API_BASE_URL` が非空のときだけ `slack.WithBaseURL` を適用する(実装は `cmd/slapex/main.go` の `newSlackClient`)。ユーザー向けドキュメント(`README.md` / `doc/help/`)と `--help` には載せず、仕様上の扱いは `cli-interface.md` の内部用途注記にとどめる。

## 理由

- 公開 CLI 仕様を変えずに、録画・fixture 実行という内部用途を満たせる。
- 環境変数のみの経路は、既存の `SLACK_TOKEN` と同じ受け渡し方針(CLI option / 引数を増やさない)と整合する。
- 未設定時は従来どおり default 接続先のみで、既存利用者への影響がない。

## 影響

- 実装: `cmd/slapex/main.go` に `apiBaseURLEnv` / `apiBaseURLFromEnv` / `newSlackClient` を追加。
- テスト: `cmd/slapex/main_test.go` に negative(`TestAPIBaseURLFromEnv`: 未設定・空・空白のみでは override しない)と positive(`TestNewSlackClientBaseURLOverride`: 指定時のみ override 先 host に Bearer token 付きで届く)を追加。default base URL 自体は `internal/slack` の既存 `TestNewDefaults` が担保する。
- 利用: `tools/gensample -serve` と VHS 録画(`tools/demo/`)がこの override を使う。
- Issue #113(token 不要 demo 実行)は 0047 で公開 option `--demo` として別経路で確定した。`--demo` は接続先を CLI 内部で直接指定し、この環境変数は経由しない。`SLAPEX_API_BASE_URL` は token 入力プロンプトを見せるデモ録画(`tools/demo/`)用として引き続き残す。

## 後から見直す条件

- 利用者向けに接続先差し替えを公開する需要が出た場合(正式 option 化や allowlist の再設計を含めて再検討する)。
- #113 の demo 実行仕様は 0047 で確定した(内部用途のまま録画に残し、利用者向けは `--demo` で別経路)。今後この棲み分けを再整理する必要が生じた場合は 0047 と併せて見直す。
