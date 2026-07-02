# 0043 Interactive selection の stream 判定

- 状態: decided
- 作成日: 2026-07-02
- 最終更新日: 2026-07-02
- 関連: `../usage-flow.md`, `../cli-interface.md`, `../../help/token-injection.md`

## 背景

Issue #54 で、1Password CLI (`op run`) 経由の macOS バイナリ直接実行時に channel の interactive selection に入らないケースが確認された。

既存方針 0004 では、TTY がある場合に interactive selection を出すことを決めていたが、当時の実装影響として stdin / stdout の TTY 判定を想定していた。一方、CLI 仕様では stdout を成功時の出力 path だけに使い、進捗・診断・候補 list・interactive prompt は stderr に出すことを定めている。

`op run` の挙動を公式情報で裏取りしたところ、secret masking は既定で有効であり、**masking 有効時は stdout だけでなく stderr も pipe 化されて TTY ではなくなる**ことが分かった(1Password 公式 [`op run` reference](https://www.1password.dev/cli/reference/commands/run) および公式コミュニティ [_"op run changes stdout and stderr to not be TTYs when masking"_](https://www.1password.community/discussions/developers/op-run-changes-stdout-and-stderr-to-not-be-ttys-when-masking/26040))。`--no-masking`(env: `OP_RUN_NO_MASKING`)を付けたときだけ両者が TTY のまま残る。

したがって、判定を stdin / stdout から stdin / stderr へ替えるだけでは、既定の `op run -- slapex`(masking 有効)では stderr も非 TTY のため依然 interactive selection に入れない。stdio の redirect / wrap に影響されない経路で判定・入出力する必要がある。

## 候補

- stdin と stdout の TTY 判定を維持し、`op run` 利用時は channel ID 指定または pseudo-TTY workaround を案内する。
- interactive 判定を stdin と stderr の TTY に変更し、interactive UI の出力先も stderr に固定する。
- controlling terminal (`/dev/tty`) を直接開いて interactive UI の入出力に使い、interactive 可否も `/dev/tty` を開けるかどうかで判定する。stdin / stdout / stderr の TTY 状態は判定に使わない。
- `--force-interactive` のような override option を追加し、利用者に明示指定させる。

## 検討内容

stdin と stdout の判定を維持すると、stdout が wrapper される実行経路では interactive selection を使えないままになる。stdout は機械処理しやすい最終結果専用 stream であり、interactive UI の表示先として扱う理由も弱い。

stdin と stderr の判定に変更する案は当初有力だったが、上記の裏取りで既定の `op run` は stderr も pipe 化することが確認できたため、`op run -- slapex` の主要経路(masking 有効)を解決できない。この案で救えるのは `op run --no-masking` を明示したときだけで、Issue #54 の目的(自然に使える)には届かない。

controlling terminal (`/dev/tty`) を直接使う案は、stdio の redirect / wrap から独立して「利用者が操作可能な端末があるか」を判定できる。`op run` は subprocess の stdout / stderr を pipe に差し替えるだけで controlling terminal は継承するため、`/dev/tty` は実端末を指し、既定の masking 有効のままでも prompt を出せる。これは git / ssh / sudo などが対話プロンプトで採る一般的な作法でもある。CI や pipe 実行では controlling terminal が無く `/dev/tty` を開けないため、従来どおり deterministic に非 interactive へ倒せ、0004 の安全性も保てる。

override option は一部環境の救済にはなるが、CI で誤って prompt 待ちに入るリスクを利用者側の理解に寄せてしまう。`/dev/tty` 判定があれば不要。

## 決定

interactive selection の可否は、controlling terminal (`/dev/tty`) を read/write で開けることを条件にする。stdin / stdout / stderr の TTY 状態は判定に使わない。

interactive UI(`huh` form)の入力・出力はいずれも `/dev/tty` に固定する。stdout は成功時の出力 directory path 1 行だけを出す stream、stderr は進捗・診断・候補 list・エラー・完了 summary の stream として維持する。

`/dev/tty` を開けない場合、または `--no-interactive` が指定された場合は、従来どおり候補 list と再実行 usage を stderr に表示し、exit code 2 で終了する。

## 理由

`/dev/tty` は stdout / stderr の redirect や wrap から独立して実端末を指すため、`op run` の既定 masking(stdout / stderr を pipe 化)のままでも interactive selection を維持できる。`--no-masking` を利用者に強いる必要がない。

対話プロンプトを controlling terminal に出すのは Unix の一般的な作法であり、stdout の機械可読契約(`out=$(slapex ...)` での capture)とも両立する。

CI / script 実行では controlling terminal が無く `/dev/tty` を開けないため、prompt 待ちに入らず deterministic に失敗できる。0004 の安全性を保てる。

## 影響

- `usage-flow.md` と `cli-interface.md` の interactive 判定を、controlling terminal (`/dev/tty`) の可否として明記する(stdin / stdout / stderr の TTY 状態は判定に使わない)。
- 実装では `os.OpenFile("/dev/tty", os.O_RDWR, 0)` を試み、開けて TTY である場合だけ interactive とする。`huh` form の input / output をこの `/dev/tty` handle に固定する。
- 1Password CLI 利用 help では、`op run` の既定 masking のままでも interactive selection が使えること、controlling terminal が無い環境では channel ID / より具体的な channel 名で再実行することを案内する。
- 対象 OS は macOS / Linux(`.goreleaser.yaml`)であり、いずれも `/dev/tty` を利用できる。

## 後から見直す条件

- Windows など `/dev/tty` を持たない OS を対象に含める場合(`CONIN$` / `CONOUT$` など別経路の抽象化が必要)。
- `huh` / `bubbletea` の TTY / stream 制御 API が変わり、`/dev/tty` 固定では期待通りに動かなくなった場合。
- controlling terminal が無いが安全に interactive selection を出したい具体的な利用環境が出た場合。
