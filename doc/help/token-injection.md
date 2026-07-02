# Token の渡し方

`slapex` は Slack OAuth token を保存しない。token は CLI 引数ではなく、実行時に環境変数 `SLACK_TOKEN` として渡す。

token の実値を `.env`、shell history、作業メモ、PR、Issue、ログに残さない。値の例示が必要な場合は `op://<vault>/<item>/<field>` や `xoxp-...` のような placeholder だけを使う。

## 1Password CLI を使う

ローカル実行では、1Password CLI などの secret manager から実行時に注入する方法を推奨する。1Password CLI の secret reference を `SLACK_TOKEN` に入れ、`op run` 経由で `slapex` を実行する。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering
```

この形では、Slack OAuth token の実値を shell history や `.env` に書かない。

出力先を固定する場合も同じように渡す。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering --output ./exports
```

channel を省略した場合や複数候補がある場合、操作可能な terminal がある環境では `op run` 経由でも interactive selection を表示する。slapex は prompt を controlling terminal (`/dev/tty`) に直接出すため、`op run` の既定の secret masking(`--no-masking` なし)のままでも対話選択を使える。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex
```

controlling terminal が無い環境(CI や pipe 実行など)では interactive selection は使えず、候補一覧と再実行例が表示される。表示された channel ID またはより具体的な channel 名を指定して再実行する。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex C0123456789
```

1Password CLI の詳しい使い方は、1Password 公式 docs の [Load secrets into the environment](https://www.1password.dev/cli/secrets-environment-variables) と [`op run` reference](https://www.1password.dev/cli/reference/commands/run) を参照する。

## CI secrets を使う

CI では、CI の secret store から `SLACK_TOKEN` を job に渡す。GitHub Actions では repository secret や environment secret に token を保存し、workflow では secret 名だけを参照する。

```yaml
jobs:
  export-slack:
    runs-on: ubuntu-latest
    steps:
      - name: Export Slack posts
        env:
          SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
        run: |
          slapex engineering --output ./exports
```

CI、定期実行、チーム共通 automation では bot token(`xoxb-`)を基本候補にする。対象 channel の履歴を取得するには、必要な scope に加えて bot / app が対象 channel に参加している必要がある。

CI log に token を出力しない。`set -x` など、コマンドや環境変数を log に出す設定を有効にしたまま token を扱わない。

## Secret manager を使わず一時的に渡す

個人評価や PoC で secret manager をまだ用意していない場合は、`SLACK_TOKEN` を設定せずに、操作可能な端末で `slapex` を実行する。`SLACK_TOKEN` が未設定で、controlling terminal (`/dev/tty`) を開けて `--no-interactive` を指定していないときは、slapex が token 入力プロンプトを表示する。

```sh
slapex engineering
# SLACK_TOKEN is not set.
# Paste a Slack OAuth token to use for this run only.
# It is kept in memory only: not echoed, and not written to files, cache, logs or HTML.
# For repeated use, provide it from a secret manager (e.g. 1Password CLI) or CI secrets.
# Enter SLACK_TOKEN (input hidden): 
```

入力は echo されず、貼り付けた token はその 1 回の実行の中だけで使われ、設定ファイル・cache・log・HTML 出力には保存されない。実値をコマンド行や環境変数へ書かないため、shell history にも残らない。CI や pipe 実行など操作可能な端末が無い環境、または `--no-interactive` 指定時はプロンプトを表示せず、`SLACK_TOKEN` 未設定エラーで終了する。

同じ token を複数コマンドで使い回したい場合は、実値をコマンド行に書かず、対話入力で一時的に shell 変数へ入れる方法もある。

```sh
trap 'stty echo' EXIT
printf "SLACK_TOKEN: "
stty -echo
IFS= read -r SLACK_TOKEN
stty echo
trap - EXIT
printf "\n"
export SLACK_TOKEN

slapex engineering

unset SLACK_TOKEN
```

この方法では、token は実行中の shell 環境に一時的に入る。使い終わったら `unset SLACK_TOKEN` で消す。いずれの方法も一時利用向けであり、長期利用では secret manager または CI secrets に移す。

次のように token の実値をコマンド行へ直接書かない。

```sh
# 非推奨: token の実値が shell history に残る。
export SLACK_TOKEN="xoxp-..."
```

## token を更新したとき

Slack App の scope を追加または変更した場合、App を workspace に再 install / 再 authorize し、更新された token を保存先へ反映する。

- 1Password CLI を使う場合: 1Password item の token field を更新する。
- CI secrets を使う場合: repository secret または environment secret を更新する。
- 一時注入だけで使う場合: 次回実行時に新しい token を入力する。

## 関連

- Slack App と token の発行手順: [`slack-app-setup.md`](slack-app-setup.md)
- CLI が読む環境変数の仕様: [`../design/cli-interface.md`](../design/cli-interface.md)
