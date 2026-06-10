# 利用手順

このファイルには、利用者が Slack の書き込みを HTML として保存するまでの操作の流れを記載していく。

想定読者は、このツールを利用する人間である。

本ファイルの操作の流れ、option 名、出力ディレクトリ構造は確定仕様として扱う。実装アーキテクチャは未確定である。

仕様はトピックごとに次の文書へ分割している。本ファイルは利用者の操作の流れを扱う。

- CLI のコマンド形式、option、環境変数、exit code: `cli-interface.md`
- 出力ディレクトリ構造、保存される assets、取得範囲、サイズ制限: `output-format.md`
- 生成する `index.html` の表示仕様(見た目): `html-rendering.md`
- 中間ファイル `.cache/` の扱い: `cache.md`
- Slack API の利用方針(method、pagination、rate limit、user / emoji / file 解決): `slack-api-usage.md`

## 想定する利用体験

利用者は、channel を示すキーワードを指定する。ツールは `SLACK_BOT_TOKEN` から対象 workspace を解決し、その workspace 内の対象 channel から投稿、スレッド、添付画像、添付ファイルなどを取得して、ローカルに閲覧可能な HTML と assets 一式を保存する。channel を指定しない場合は、TTY で操作可能な環境では channel を選択できる。

```sh
slapex <channel-keyword>
```

`<channel-keyword>` は channel 名、channel ID、または名前の一部を想定する。曖昧な指定で複数候補が見つかった場合、ツールは候補を表示し、利用者により具体的な指定を促す。

通常の単一 workspace install で発行された bot token は install 先 workspace に紐付くため、利用者が workspace を指定する必要はない。ツールは `auth.test` などで token の workspace 名、workspace URL、`team_id` を確認し、出力ディレクトリ名や metadata に反映する。出力ディレクトリ名では ID そのものではなく、人間が読みやすい workspace / channel label を優先する。

Enterprise Grid の org-wide install など、1 つの token が複数 workspace を表し得るケースは初期対象外とする。初期の How to Use 素案では、単一 workspace install の bot token を基本利用として扱う。

## フェーズ 1: Slack App を準備する

Slack App の作成、scope 設定、workspace install、bot token 発行は手順が長いため、CLI のエラー出力には詳細なステップを表示しない。詳細手順は本リポジトリ内の help ページに分離し、GitHub 上で参照できるようにする。

Help URL:

```text
https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md
```

token が未設定、無効、または必要な権限を持たない場合、ツールは短い原因説明と上記 help URL を表示する。

初期利用手順では、利用者自身が自分用の Slack App を作成し、対象 workspace に install する前提にする。配布用 Slack App や OAuth flow は初期対象外とする。

基本の準備は、help ページで次の内容として案内する。Slack App は <https://api.slack.com/apps?new_app=1> から作成し、個別に scope を設定する方法だけでなく、manifest を貼り付けて必要 scope をまとめて設定する方法を推奨する。

1. Slack API の App 管理画面で Slack App を作成する。
2. App を対象 workspace に紐付ける。
3. manifest または OAuth & Permissions で bot token scopes を設定する。
4. App を workspace に install し、bot token を発行する。
5. bot を取得対象 channel に参加させる。
6. 発行された bot token を secret manager または CI secrets に保存する。
7. `SLACK_BOT_TOKEN` として実行時に渡す。

想定する scope:

| 目的 | scope |
|---|---|
| public channel の一覧・解決 | `channels:read` |
| public channel の投稿取得 | `channels:history` |
| private channel の一覧・解決 | `groups:read` |
| private channel の投稿取得 | `groups:history` |
| スレッド返信の取得 | 対象 conversation 種別に対応する `*:history` |
| 画像・添付ファイルの情報取得と download | `files:read` |
| カスタム絵文字の一覧取得 | `emoji:read` |
| 投稿者名や表示名の解決 | `users:read` |

初期利用手順では、public channel と private channel の scope を同じ設定手順で扱い、上記 scope をまとめて設定する。private channel を扱う場合は、scope の付与だけでなく bot がその private channel に参加している必要がある。

## フェーズ 2: Slack API token を実行時に渡す

Slack の投稿を取得するには、Slack App を workspace に install した後に発行される bot token が必要である。ツールは token 自体を保存せず、実行時に環境変数から受け取る。

想定する環境変数名:

```sh
SLACK_BOT_TOKEN
```

token をローカルの `.env` などに実値で保存することは推奨しない。ローカル実行では 1Password CLI などの secret manager から実行時に注入する。

```sh
SLACK_BOT_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex <channel-keyword>
```

CI で実行する場合は、CI の secret store に `SLACK_BOT_TOKEN` を登録し、job の環境変数として渡す。ツールは interactive なブラウザ操作やローカル専用 credential store に依存しない。

## フェーズ 3: 取得を実行する

利用者は channel を指定または選択して取得を開始する。workspace は `SLACK_BOT_TOKEN` から解決される。

```sh
op run -- slapex <channel-keyword>
```

ツールの想定動作:

1. `SLACK_BOT_TOKEN` が存在するか確認する。
2. Slack の認証確認 API で token が有効か確認する。
3. token が紐付く workspace を取得し、workspace 名、workspace URL、`team_id` を記録する。
4. 確定した workspace 情報を表示する。
5. channel 一覧から channel 引数の指定に合う候補を探す。
6. channel が一意に決まったら、確定した channel 情報を表示する。
7. 取得範囲制限に従って投稿履歴を取得する。
8. スレッドがある投稿では返信を取得する。
9. 投稿内で参照される assets を取得し、ローカル assets として保存する。
10. 投稿本文、投稿者情報、日時、スレッド、assets への相対リンクを HTML に変換する。
11. 出力先に HTML と assets 一式を書き込む。
12. 完了時に出力先と取得対象 workspace / channel を表示する。

取得範囲制限・出力先のディレクトリ構造・保存対象は `output-format.md`、生成する HTML の見た目は `html-rendering.md` を参照する。

### 処理対象の表示

通常実行では、ツール側に期待する workspace を示す入力がないため、workspace mismatch を自動検出するエラーにはしない。その代わり、処理の進行中に token から解決した workspace と、確定した channel を繰り返し表示し、利用者が対象を意識できるようにする。

画面表示用の workspace label は、Slack 上の表示名だけに依存しない。可能な限り workspace 名、workspace URL または domain、短い `team_id` を組み合わせる。

例:

```text
Workspace: Example Workspace (example.slack.com, T012345...)
```

`team_id` は `auth.test` の戻り値や workspace URL にも現れる非機密情報である。進捗の繰り返し表示では上記のように短縮してよいが、workspace 確定直後、完了 summary、生成 HTML の冒頭では full の `team_id` を表示し、利用者が照合や将来の guard option のために正確な値をコピーできるようにする。

画面表示用の channel label は、channel 名に加えて channel ID、public/private、archived 状態、bot membership を含める。

例:

```text
Target: Example Workspace (example.slack.com, T012345...) / #engineering (C012345..., public, active, member)
```

表示タイミング:

1. `auth.test` などで workspace が確定した直後に workspace label を表示する。
2. channel 候補を表示する場合は、候補 list の前に workspace label を表示する。
3. channel が確定した直後、履歴取得を開始する前に workspace / channel label を表示する。
4. 投稿、thread replies、assets などの進捗表示では、必要に応じて workspace / channel label を含める。
5. 完了時の summary では、出力先 path とあわせて workspace / channel label を表示する。
6. 生成した `index.html` の冒頭にも、取得対象 workspace / channel と export 実行時刻を表示する。

画面表示用 label と directory 用 label は役割を分ける。画面表示用 label は利用者が Slack 上の対象を確認するための情報であり、Slack の表示名、domain、ID などを読みやすく含める。directory 用の `<workspace-label>` / `<channel-label>` は filesystem-safe な slug であり、出力 path の安全性と衝突回避を優先する(directory 用 label の詳細は `output-format.md`)。

この表示は誤認を減らすための診断情報であり、通常実行で workspace mismatch を強制的に停止する guard ではない。CI などで誤 token を必ず止める必要が出た場合は、将来的に `--expect-team-id` や `--expect-workspace-domain` のような検証専用 option を追加するかを検討する。

## channel の指定と選択

channel は、明示指定と選択の両方に対応する。

基本方針:

1. channel 引数が指定されている場合は、その値を channel keyword として使う。
2. channel keyword が channel ID に一致する場合は、その channel を確定する。
3. channel keyword が channel 名に完全一致する場合は、その channel を確定する。
4. 完全一致しない場合は、channel 名の部分一致で候補を探す。
5. 候補が 1 件なら、その channel を確定する。
6. 候補が 2 件以上 10 件以下の場合は、利用者に選択を求める。
7. 候補が 11 件以上の場合は、候補が多すぎることを表示し、より具体的な channel 引数で再実行するよう促して非 0 exit code で終了する。
8. channel 引数が指定されていない場合も、利用者に channel 選択を求める。ただし、候補が 11 件以上になる場合は選択を開始しない。

非 0 exit code の具体値は `cli-interface.md` の exit code 仕様に従う(対象を確定できない場合は `2`)。

候補表示では、少なくとも channel ID、channel 名、public/private、archived 状態、bot が member かどうかを表示する。private channel は、bot token から見える範囲だけが候補になる。

### TTY がある場合

stdin と stdout が TTY で、利用者が操作可能な場合は、interactive selection を表示する。

`--no-interactive` が指定された場合は、TTY があっても interactive selection を開始しない。候補が複数ある場合や channel 引数が未指定の場合は、non-TTY と同じく候補と usage を表示し、非 0 exit code で終了する。

候補が 11 件以上ある場合は、TTY があっても interactive selection を開始しない。対象 channel が見つからない場合と同じく、より具体的な channel 名の一部または channel ID を指定して再実行するよう促す。

想定する操作:

1. 選択可能な channel list を表示する。
2. 利用者がカーソル上下で対象 channel を選ぶ。
3. Enter で決定する。
4. 選択後、確定した channel ID と channel 名を表示して取得を開始する。

channel 引数が未指定で候補が 10 件以下の場合は、その候補から選択できる。候補が 11 件以上の場合は、`<channel-keyword>` を指定して再実行するよう促す。

### TTY がない場合

stdin または stdout が TTY ではない場合は、interactive selection を開始しない。CI や script 実行で待ち状態に入らないことを優先する。

この場合は、選択可能な候補を表示し、次に実行すべき usage を表示して、非 0 exit code で終了する。

例:

```text
Multiple channels matched "eng".

Workspace: Example Workspace (example.slack.com, T012345...)

Candidates:
  C0123456789  #engineering          public   active
  C0234567890  #engineering-notify   public   active
  G0345678901  #engineering-private  private  active

Run again with a more specific channel:

  slapex C0123456789 --output ./exports
```

channel 引数が未指定かつ TTY がない場合も同様に、interactive selection はできないことを伝え、`slapex <channel-id-or-name>` の形で再実行する usage を表示する。

この設計は、ローカル実行では探しやすく、CI では deterministic に失敗できる点で有効である。script や検証用途では、TTY がある場合でも `--no-interactive` で prompt を禁止できる。

## 情報が足りない場合の案内

ツールは、足りない情報や権限に応じて、短い診断、次に確認すべき最小限の内容、help URL を表示する。Slack App の作成や token 発行の詳細手順は CLI の出力に展開しない。

Help URL:

```text
https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md
```

### `SLACK_BOT_TOKEN` が未設定

表示する内容:

1. `SLACK_BOT_TOKEN` が環境変数として渡されていないことを表示する。
2. token を secret manager または CI secrets から実行時に注入するよう促す。
3. Slack App の作成や token 発行が未完了の場合は help URL を参照するよう案内する。

例:

```text
SLACK_BOT_TOKEN is not set.

Set SLACK_BOT_TOKEN from your secret manager or CI secrets, then run slapex again.

Need to create a Slack App or issue a bot token?
See: https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md
```

### token が無効

表示する内容:

1. secret manager または CI secrets に保存した token が正しいか確認する。
2. Slack App が workspace から uninstall されていないか確認する。
3. token を再発行または再 install する。
4. 詳細手順として help URL を表示する。

### token の workspace を確認したい

表示する内容:

1. `auth.test` で確認した workspace 名、workspace URL、`team_id` を表示する。
2. 複数 workspace を扱う場合は、それぞれの workspace に対応する bot token を使う。
3. CI では job ごとに渡している `SLACK_BOT_TOKEN` が正しいか確認する。
4. Enterprise org-wide install の token を使っている場合は、初期対象外であることを表示し、単一 workspace install の bot token を使うよう案内する。

通常実行では、ツール側に期待する workspace を示す入力がないため、workspace mismatch を自動検出するエラーにはしない。workspace 情報は、利用者や CI 運用者が token の向き先を確認するための診断情報として表示する。通常の export 実行でも各タイミングで workspace / channel label を表示する(表示タイミングと内容は「処理対象の表示」を参照)。`--reuse-cache` で以前の `.cache/` を再利用する場合だけ、cache に記録された workspace 情報との不一致を検出対象にできる。

### channel が見つからない

表示する内容:

1. channel 名または channel ID を確認する。
2. private channel の場合、bot が channel に参加しているか確認する。
3. private channel 用の scope が不足していないか確認する。
4. archived channel を対象にするかどうかを確認する。

### channel 候補が複数ある

表示する内容:

1. 候補が 10 件以下で TTY がある場合は、候補 list から対象 channel を選択してもらう。
2. 候補が 11 件以上の場合は、候補が多すぎることを表示する。
3. TTY がない場合、`--no-interactive` が指定された場合、または候補が 11 件以上の場合は、channel ID またはより具体的な channel keyword を指定して再実行する usage を表示し、非 0 exit code で終了する。

### scope が不足している

表示する内容:

1. Slack App の OAuth & Permissions に必要 scope を追加する。
2. App を workspace に再 install する。
3. 更新された token を secret manager または CI secrets に反映する。
4. 詳細手順として help URL を表示する。

### bot が channel に参加していない

表示する内容:

1. public channel の場合は、Slack 上で app / bot を channel に追加する。
2. private channel の場合は、その private channel の参加者が app / bot を招待する。
3. 参加後に同じコマンドを再実行する。

## ローカル実行の例

```sh
SLACK_BOT_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering
```

この例では、実 token の値は shell history や `.env` に残さず、1Password CLI が実行時だけ `SLACK_BOT_TOKEN` を解決する。

`--output` を省略した場合は、例えば次のような出力 root が作成される。この `20260602-1530` はコマンド実行時刻の例であり、取得対象投稿の日時ではない。

```text
./slapex-20260602-1530/<workspace-label>/<channel-label>/
```

## CI 実行の例

CI では secret store から `SLACK_BOT_TOKEN` を job に渡す。

```yaml
steps:
  - name: Export Slack posts
    env:
      SLACK_BOT_TOKEN: ${{ secrets.SLACK_BOT_TOKEN }}
    run: |
      slapex \
        engineering \
        --output ./exports
```

CI 上で出力ファイルを artifact として保存するか、後続 job で配布・検証するかは別途検討する。

CI では artifact path を固定しやすくするため、必要に応じて `--output ./exports` を明示する。

## 未決事項

- CI artifact としての保存方法。

出力形式・取得範囲・cache に関する未決事項は `output-format.md` と `cache.md`、全体の一覧は `decision-log/index.md` を参照する。

## 参考

- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history)
- Slack Developer Docs: [`conversations.replies`](https://docs.slack.dev/reference/methods/conversations.replies/)
- Slack Developer Docs: [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list)
- Slack Developer Docs: [`files.info`](https://docs.slack.dev/reference/methods/files.info)
- Slack Developer Docs: [`files:read`](https://docs.slack.dev/reference/scopes/files.read)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test)
- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth)
