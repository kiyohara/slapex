# 0044 SLACK_TOKEN 未設定時の対話入力プロンプト

- 状態: decided
- 作成日: 2026-07-02
- 最終更新日: 2026-07-02
- 関連: `../cli-interface.md`, `../usage-flow.md`, `../../help/token-injection.md`, [0042-default-user-token.md](0042-default-user-token.md), [0043-interactive-selection-streams.md](0043-interactive-selection-streams.md)

## 背景

Issue #97。PR #96 で token 注入パターンの help を整理する過程で、secret manager をまだ用意していない個人評価・PoC 利用者向けに、`SLACK_TOKEN` を shell history に残さず渡す導線が欲しいことが分かった。

現状は `SLACK_TOKEN` 未設定時に短い未設定エラー(exit code 3)と案内を出して終了する。利用者は shell 側で環境変数を設定してから再実行する必要があり、慣れていない利用者は `export SLACK_TOKEN="<実値>"` の形で token を shell history に残しやすい。

## 候補

- 変更しない。`SLACK_TOKEN` は環境変数からのみ受け取り、未設定なら従来どおりエラーで終了する。
- stdin / stderr が TTY のときだけ対話入力プロンプトを出す。
- controlling terminal (`/dev/tty`) を開けるときだけ、`/dev/tty` に no-echo の対話入力プロンプトを出す。判定・入出力とも `/dev/tty` を使う。
- token をローカルファイル / OS keyring に保存する仕組みを入れる。

## 検討内容

Issue 本文の「検討する条件」は stdin / stderr の TTY 判定を挙げていた。しかし channel の interactive selection について 0043 で確認したとおり、1Password CLI の `op run` は既定の secret masking で stdout だけでなく stderr も pipe 化するため、stdin / stderr 判定では `op run -- slapex` の主要経路で対話に入れない。判定を stdio から独立させる必要がある点は token 入力でも同じである。

`/dev/tty` を直接開く案は、stdio の redirect / wrap から独立して「操作可能な端末があるか」を判定でき、`op run` の既定 masking のままでも prompt を出せる。CI / pipe 実行では controlling terminal が無く `/dev/tty` を開けないため、deterministic に非対話へ倒せる。channel selection と同じ機構に揃うため実装・仕様の一貫性も保てる。

入力は `golang.org/x/term` の `term.ReadPassword` で echo せずに読む。これは sudo / ssh / git の credential prompt と同じ作法で、画面にも stdout / stderr にも実値を残さない。

token をファイルや keyring に保存する案は、漏えい面を増やし、Issue のスコープ外(ファイル保存・secret manager 連携そのもの)でもあるため採らない。

## 決定

`SLACK_TOKEN` が未設定で、controlling terminal (`/dev/tty`) を read/write で開けて、かつ `--no-interactive` が指定されていないときに限り、`/dev/tty` に token 入力プロンプトを表示する。

- 入力は `term.ReadPassword` で echo しない。
- 入力した token はそのプロセス内でだけ使い、設定ファイル・cache・log・HTML 出力には保存しない。
- プロンプト文面では、継続利用向けに secret manager(例: 1Password CLI)や CI secrets があることも短く示す。
- `/dev/tty` を開けない環境(CI・pipe 実行など)や `--no-interactive` 指定時は、対話入力を行わず、従来どおり未設定エラー(exit code 3)と案内を表示して終了する。

初期対象は `SLACK_TOKEN` のみとする。他の環境変数への一般化は必要になった時点で検討する。

## 理由

interactive 可否を `/dev/tty` で判定する方針は 0043 で確定済みで、token 入力もこれに揃えることで `op run` の既定 masking 下でも動き、CI では deterministic に失敗できる。no-echo 入力は Unix の credential prompt の一般的な作法であり、stdout の機械可読契約(`out=$(slapex ...)`)とも両立する。token を保存しない方針(0042 / `cli-interface.md`)も維持できる。

## 影響

- `cli-interface.md` の環境変数節に対話入力の条件・非保存を明記し、入出力ストリーム表の `/dev/tty` 行に token 入力(no-echo)を追記する。
- `usage-flow.md` の「Slack token が未設定」に対話入力プロンプトの挙動を追記する。
- `doc/help/token-injection.md` の「Secret manager を使わず一時的に渡す」を、まず slapex 内蔵プロンプトを案内し、shell の手動入力手順は代替として残す形に更新する。
- 実装は `cmd/slapex/main.go` の `resolveToken` / `promptForToken` / `writeTokenPrompt`。controlling terminal は 1 度だけ開き、token 入力と channel selection で共用する。`term.ReadPassword` を利用する。
- token を CLI option / 引数で受け取る経路や、環境変数以外の保存経路は追加しない。

## 後から見直す条件

- `SLACK_TOKEN` 以外の必須環境変数にも対話入力を広げたい要望が出た場合。
- token をファイルや OS keyring に保存したい要望が出た場合(漏えい面と併せて再検討)。
- Windows など `/dev/tty` を持たない OS を対象に含める場合(`CONIN$` / `CONOUT$` などの抽象化が必要)。
- `golang.org/x/term` の no-echo API が変わった場合。
