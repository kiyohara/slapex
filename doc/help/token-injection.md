# Token の渡し方

`slapex` は Slack OAuth token を保存しません。token は CLI 引数ではなく、環境変数 `SLACK_TOKEN`、または実行時の対話入力で渡します。[^spec-env]

password manager などの追加ツールは必須ではありません。実行のたびに token を貼り付ける方法(このページの最初の方法)だけで利用できます。

token の実値を `.env`、shell history、作業メモ、PR、Issue、ログに残さないでください。値の例示が必要な場合は `op://<vault>/<item>/<field>` や `xoxp-...` のような placeholder だけを使います。

## 実行時に貼り付ける(基本)

`SLACK_TOKEN` を設定せずに、操作可能な端末で `slapex` を実行します。token 入力プロンプトが表示されるので、コピーしておいた token を貼り付けます。

```sh
slapex engineering
# SLACK_TOKEN is not set.
# Paste a Slack OAuth token to use for this run only.
# It is kept in memory only: not echoed, and not written to files, cache, logs or HTML.
# For repeated use, provide it from a secret manager (e.g. 1Password CLI) or CI secrets.
# Enter SLACK_TOKEN (input hidden):
```

入力は画面に表示(echo)されず、貼り付けた token はその 1 回の実行の中だけで使われます。設定ファイル・cache・log・HTML 出力には保存されず、コマンド行に書かないため shell history にも残りません。

CI や pipe 実行など操作可能な端末が無い環境、または `--no-interactive` 指定時は、プロンプトを表示せず `SLACK_TOKEN` 未設定エラーで終了します。

## 補足: shell 環境変数に一時的に設定する

同じ token を複数コマンドで使い回したい場合は、実値をコマンド行に書かず、対話入力で一時的に shell 変数へ入れる方法もあります。

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

この方法では、token は実行中の shell 環境に一時的に入ります。使い終わったら `unset SLACK_TOKEN` で消してください。

次のように token の実値をコマンド行へ直接書かないでください。

```sh
# 非推奨: token の実値が shell history に残る。
export SLACK_TOKEN="xoxp-..."
```

## 補足: secret manager(1Password CLI)を使う(継続利用の推奨)

繰り返し使う場合は、1Password CLI などの secret manager から実行時に注入する方法を推奨します。token を都度コピーする必要がなくなり、実値を手元で扱う機会自体を減らせます。1Password CLI では、secret reference を `SLACK_TOKEN` に入れ、`op run` 経由で `slapex` を実行します。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering
```

出力先を固定する場合も同じように渡します。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering --output ./exports
```

channel を省略した場合や複数候補がある場合、操作可能な terminal がある環境では `op run` 経由でも interactive selection を表示します。slapex は prompt を controlling terminal (`/dev/tty`) に直接出すため、`op run` の既定の secret masking(`--no-masking` なし)のままでも対話選択を使えます。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex
```

controlling terminal が無い環境(CI や pipe 実行など)では interactive selection は使えず、候補一覧と再実行例が表示されます。表示された channel ID またはより具体的な channel 名を指定して再実行してください。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex C0123456789
```

1Password CLI の詳しい使い方は、1Password 公式 docs の [Load secrets into the environment](https://www.1password.dev/cli/secrets-environment-variables) と [`op run` reference](https://www.1password.dev/cli/reference/commands/run) を参照してください。

## CI secrets を使う

CI では、CI の secret store から `SLACK_TOKEN` を job に渡します。GitHub Actions では repository secret や environment secret に token を保存し、workflow では secret 名だけを参照します。

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

CI、定期実行、チーム共通 automation では bot token(`xoxb-`)を基本候補にします。対象 channel の履歴を取得するには、必要な scope に加えて bot / app が対象 channel に参加している必要があります。

CI log に token を出力しないでください。`set -x` など、コマンドや環境変数を log に出す設定を有効にしたまま token を扱わないでください。

## token を更新したとき

Slack App の scope を追加または変更した場合、App を workspace に再 install / 再 authorize し、更新された token を利用中の方法に反映します。

- 実行時に貼り付ける場合・shell 変数へ一時設定する場合: 次回実行時に新しい token を入力します。
- 1Password CLI を使う場合: 1Password item の token field を更新します。
- CI secrets を使う場合: repository secret または environment secret を更新します。

## 関連

- Slack App と token の発行手順: [`slack-app-setup.md`](slack-app-setup.md)

[^spec-env]: 仕様の正本(開発者向け): [`doc/design/cli-interface.md` の環境変数](../design/cli-interface.md#環境変数)。
