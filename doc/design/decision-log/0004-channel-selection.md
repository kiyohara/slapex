# 0004 Channel Selection

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`, `0020-target-label-display.md`

## 背景

Slack 投稿を export するには対象 channel を決める必要がある。

当初は `--channel <channel-keyword>` を指定し、曖昧な場合はより具体的な指定を促す程度の整理だった。しかし、channel は export 対象そのものであり、利用者がほぼ毎回指定する値であるため、option よりも positional argument として扱う方が CLI の主操作として自然である。

また、ローカル利用では候補から選択できる方が便利であり、CI では interactive prompt に入ると job が停止するため、TTY の有無に応じた挙動を決める必要が出た。

## 候補

- `--channel` option を必須にし、曖昧な指定は常にエラーにする。
- `--channel` option を任意にし、候補が複数ある場合は常に interactive selection を出す。
- `--channel` option を任意にし、TTY がある場合だけ interactive selection、TTY がない場合は候補と usage を表示して終了する。
- optional positional argument として `[channel]` を受け取り、TTY がある場合だけ interactive selection、TTY がない場合は候補と usage を表示して終了する。

## 検討内容

`--channel` 必須は CI では単純だが、ローカル利用で channel ID や正確な channel 名を調べる手間が大きい。

常に interactive selection を出す方式はローカルでは便利だが、CI や script 実行で stdin を待ち続けるリスクがある。

TTY の有無で分岐する方式なら、ローカルでは選びやすく、CI では deterministic に失敗できる。

channel は export の主対象であり、`slapex general` や `slapex engineering --output ./exports` の方が、`slapex --channel general` よりも利用頻度の高い操作として短く自然である。未指定時の interactive selection も維持するため、必須引数ではなく optional positional argument として定義する。

## 決定

channel は、optional positional argument による明示指定と、interactive selection による選択の両方に対応する。`--channel` option は初期 CLI では採用しない。

基本 syntax は次の通りとする。

```sh
slapex [channel] [options]
```

基本の解決順は次の通りとする。

1. `[channel]` が指定されている場合は、その値を channel keyword として使う。
2. channel keyword が channel ID に一致する場合は、その channel を確定する。
3. channel keyword が channel 名に完全一致する場合は、その channel を確定する。
4. 完全一致しない場合は、channel 名の部分一致で候補を探す。
5. 候補が 1 件なら、その channel を確定する。
6. 候補が 2 件以上 10 件以下の場合は、利用者に選択を求める。
7. 候補が 11 件以上の場合は、候補が多すぎることを表示し、より具体的な channel 引数で再実行するよう促して非 0 exit code で終了する。
8. `[channel]` が指定されていない場合も、利用者に channel 選択を求める。ただし、候補が 11 件以上になる場合は選択を開始しない。

選択は TTY の有無で分岐する。

- TTY がある場合: interactive selection を表示し、カーソル上下と Enter で選択できるようにする。
- TTY がない場合: interactive selection は開始せず、候補一覧と再実行 usage を表示し、非 0 exit code で終了する。
- `--no-interactive` が指定された場合: TTY がある場合でも interactive selection を開始せず、non-TTY と同じ挙動にする。
- 候補が 11 件以上の場合: TTY がある場合でも interactive selection を開始せず、候補が多すぎることと再実行 usage を表示し、非 0 exit code で終了する。

## 理由

ローカル実行では channel を探しながら実行できる体験が便利である。一方、CI では prompt 待ちが最も避けるべき失敗モードである。

TTY の有無で挙動を分けることで、両方の利用形態を自然に扱える。

channel を positional argument にすると、ツールの主対象が CLI syntax に表れる。`--channel` は長く、ほぼ必ず使う option としては重い。検討段階であるため、互換性維持のために `--channel` を残す必要はない。

候補が 11 件以上ある場合は、interactive selection の中で絞り込み UI を作るより、より具体的な channel keyword または channel ID を指定してもらう方が初期仕様として単純である。大量候補をそのまま表示すると、選択ミスや CI log への不要な情報露出も増える。

## 影響

- `usage-flow.md` に channel selection の挙動を採用方針として記載する。
- コマンド例は `slapex engineering` のような positional argument を基本にする。
- 実装では stdin / stdout の TTY 判定が必要になる。
- non-TTY では候補と usage を出して非 0 exit code で終了する。
- `--no-interactive` option を提供し、TTY がある script / 検証環境でも prompt を禁止できるようにする。
- channel 候補の表示内容には channel ID、channel 名、public/private、archived 状態、bot membership を含める。
- channel 候補を表示する前に workspace label を表示する(`0020-target-label-display.md`)。
- interactive selection の対象は候補 10 件以下に制限する。

## 後から見直す条件

- channel 数が多い workspace で 10 件制限が厳しすぎる。
- CI log に channel 名を出したくない運用が必要になる。
- `--no-interactive` だけでは制御できない prompt / output 要件が出る。
