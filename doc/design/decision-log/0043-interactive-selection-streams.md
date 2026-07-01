# 0043 Interactive selection の stream 判定

- 状態: decided
- 作成日: 2026-07-02
- 最終更新日: 2026-07-02
- 関連: `../usage-flow.md`, `../cli-interface.md`, `../../help/token-injection.md`

## 背景

Issue #54 で、1Password CLI (`op run`) 経由の macOS バイナリ直接実行時に channel の interactive selection に入らないケースが確認された。

既存方針 0004 では、TTY がある場合に interactive selection を出すことを決めていたが、当時の実装影響として stdin / stdout の TTY 判定を想定していた。一方、CLI 仕様では stdout を成功時の出力 path だけに使い、進捗・診断・候補 list・interactive prompt は stderr に出すことを定めている。

1Password 公式 docs では、`op run` は secret を subprocess の環境変数として渡し、stdout/stderr の secret masking を行うと説明されている。`op run` が常に stdout の TTY 性を透過するとは扱わず、slapex 側で stdout に依存しない interactive 判定へ寄せる必要がある。

## 候補

- stdin と stdout の TTY 判定を維持し、`op run` 利用時は channel ID 指定または pseudo-TTY workaround を案内する。
- interactive 判定を stdin と stderr の TTY に変更し、interactive UI の出力先も stderr に固定する。
- `--force-interactive` のような override option を追加し、利用者に明示指定させる。

## 検討内容

stdin と stdout の判定を維持すると、stdout が wrapper される実行経路では interactive selection を使えないままになる。stdout は機械処理しやすい最終結果専用 stream であり、interactive UI の表示先として扱う理由も弱い。

stdin と stderr の判定に変更すると、既存の stream 分離仕様と揃う。stderr が TTY であれば、進捗表示や候補 list と同じ stream に interactive prompt を出せる。stdout の TTY 状態を見ないため、`out=$(slapex ...)` のような stdout capture とも概念上は矛盾しない。ただし stdin または stderr が non-TTY の CI / pipe 実行では、従来どおり prompt に入らず deterministic に失敗させる必要がある。

override option は一部環境の救済にはなるが、CI で誤って prompt 待ちに入るリスクを利用者側の理解に寄せてしまう。現時点では、stdin と stderr の TTY 判定で足りる。

## 決定

interactive selection の可否は stdin と stderr が TTY であることを条件にする。stdout の TTY 状態は判定に使わない。

interactive UI の出力先は stderr に固定する。stdout は成功時の出力 directory path 1 行だけを出す stream として維持する。

stdin または stderr が TTY ではない場合、または `--no-interactive` が指定された場合は、従来どおり候補 list と再実行 usage を stderr に表示し、exit code 2 で終了する。

## 理由

既存の CLI stream 仕様に最も素直に合う。`op run` のように stdout が wrapper され得る実行経路でも、stderr に prompt を出すことで interactive selection を維持できる。

CI / script 実行で prompt 待ちを避けるという 0004 の安全性も、stderr TTY を条件にすることで維持できる。

## 影響

- `usage-flow.md` と `cli-interface.md` の TTY 判定を stdin + stderr として明記する。
- 実装では `term.IsTerminal(os.Stdin)` と `term.IsTerminal(os.Stderr)` で interactive 可否を判定する。
- `huh` form の input を stdin、output を stderr に明示する。
- 1Password CLI 利用 help では、`op run` 経由でも interactive selection が使える条件と、使えない場合は channel ID / より具体的な channel 名で再実行することを案内する。

## 後から見直す条件

- stderr が TTY ではないが安全に interactive selection を出したい具体的な利用環境が出た場合。
- `huh` の TTY / stream 制御 API が変わり、stderr 固定では期待通りに動かなくなった場合。
- stdout capture と interactive selection を同時に使う利用が増え、より明示的な出力制御 option が必要になった場合。
