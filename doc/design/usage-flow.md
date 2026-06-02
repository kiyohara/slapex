# 利用手順

このファイルには、利用者が Slack の書き込みを HTML として保存するまでの手順を記載していく。

想定読者は、このツールを利用する人間である。

この内容は議論用の素案であり、実装アーキテクチャ、オプション名、出力ディレクトリ構造は未確定である。利用者が自然に完了できる流れを先に整理し、その後に実装方式を決める。

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
https://github.com/kiyohara/slack_posts_exporter/blob/main/doc/help/slack-app-setup.md
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
4. channel 一覧から channel 引数の指定に合う候補を探す。
5. channel が一意に決まったら、取得範囲制限に従って投稿履歴を取得する。
6. スレッドがある投稿では返信を取得する。
7. 投稿内で参照される assets を取得し、ローカル assets として保存する。
8. 投稿本文、投稿者情報、日時、スレッド、assets への相対リンクを HTML に変換する。
9. 出力先に HTML と assets 一式を書き込む。

## 取得範囲

初期出力は channel 単位の `index.html` とする。日付単位や thread 単位の HTML 分割は初期対象外とする。

歴史の長い channel を指定した場合に無制限取得にならないよう、取得範囲は post 件数と日付で制限する。2 つの制限は AND で結合し、両方を満たす投稿だけを取得対象にする。

option:

| option | default | max | 目的 |
|---|---:|---:|---|
| `--max-posts <count>` | `1000` | `10000` | channel timeline 上の親投稿の最大取得件数 |
| `--days <days>` | `30` | `90` | 現在時刻から何日前までの投稿を取得するか |

`--max-posts` は親投稿数だけを数え、thread replies は含めない。対象になった親投稿に thread replies がある場合、replies は一緒に取得する。

ただし、1 thread の replies が `1000` 件を超える場合は、それ以上の取得を取りやめ、HTML 上では残りの replies を次のようなメッセージに置き換える。

```text
取り扱える件数の上限に達しました。
```

## 保存する assets

ローカル HTML から外部 URL へ依存せず閲覧できるように、次の assets を保存対象とする。

| 種別 | 取得元 | 保存時の扱い |
|---|---|---|
| 標準絵文字 | Slack の標準絵文字 URL | 本文中の `:emoji_name:` を画像または Unicode fallback として解決する |
| カスタム絵文字 | Slack API `emoji.list` | workspace 固有の絵文字画像として保存する |
| URL preview 画像 | Slack message の unfurl / attachment 情報 | Slack 上で preview として表示されていた画像を保存する。ツール自身による Open Graph fetch は行わない |
| ユーザーがアップロードした画像 | Slack message の `files` 情報、`files.info`、画像 thumbnail / original URL | thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開けるようにする |
| 画像以外の添付ファイル | Slack message の `files` 情報、`files.info`、download URL | サイズ上限以下の添付ファイルを保存し、HTML から相対リンクで参照する |

標準絵文字とカスタム絵文字については、関連評価実装である `slack_posts_dumper` に PoC 実装がある。PoC では `AssetManager` が `output/assets/<url-md5>.<ext>` のような URL hash ベースのファイル名を生成し、`output/assets_manifest.json` に元 URL、ローカルパス、metadata を記録する。`EmojiResolver` は `emoji.list` でカスタム絵文字を解決し、標準絵文字は Slack の標準絵文字 URL を組み立てている。

本リポジトリでも、asset ファイル名は PoC と同じく URL hash ベースにする。元 URL が同じ asset は同じファイル名へ解決されるため、重複 download と重複保存を避けやすい。asset 種別、元 URL、Slack file ID、emoji 名、元の表示ファイル名、content type、取得成否などの人間が読むための情報は `.cache/assets_manifest.json` と HTML 側の表示に保持する。

標準絵文字は原則として Unicode に戻して HTML に直接表示する。Unicode fallback できないカスタム絵文字は画像 asset として保存するが、利用者にとって custom かどうかは重要な分類ではないため、保存先は `assets/emoji/` に集約する。

利用者が出力内容を把握しやすいように、ファイル名は URL hash ベースとしつつ、保存先は asset 種別ごとの分類ディレクトリに分ける。

## 出力イメージ

出力ディレクトリ構造の素案:

```text
slapex-<yyyymmdd>-<hhmm>/
└── <workspace-label>/
    └── <channel-label>/
        ├── index.html
        ├── style.css
        ├── assets/
        │   ├── emoji/
        │   │   └── <url-hash>.gif
        │   ├── og-images/
        │   │   └── <url-hash>.jpg
        │   ├── uploads/
        │   │   ├── thumbs/
        │   │   │   └── <url-hash>.jpg
        │   │   └── originals/
        │   │       └── <url-hash>.<ext>
        │   └── attachments/
        │       └── <url-hash>.<ext>
        └── .cache/
            ├── assets_manifest.json
            ├── metadata.json
            └── slack_api_cache.json
```

`index.html` はローカルブラウザで開ける HTML とする。画像や添付ファイルへの参照は、可能な限り出力ディレクトリ内の相対パスにする。

`style.css` は `index.html` から相対 path で参照する。style は HTML 内に固定的に埋め込まず、将来的に theme 切り替えや style 差し替えをしやすいように分離する。

`--output` が指定された場合、その値を出力 root とする。`--output` が指定されていない場合は、カレントディレクトリ配下に `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する。この日時はコマンド実行時刻を表し、取得対象となる投稿の日時ではない。`<workspace-label>/<channel-label>/` は token と channel 解決結果からツールが作成する。

`<workspace-label>` と `<channel-label>` は、Slack API 上の ID そのものではなく、人間が読みやすい workspace 名、workspace domain、channel 名などを filesystem-safe に正規化した label とする。label が取得できない場合や、正規化後に衝突する場合は、短い `team_id` / channel ID などを suffix または fallback として使う。元の ID、表示名、実際に使った label は metadata / cache に記録する。

同じ分に複数回実行され、出力 root が既に存在する場合は、`slapex-<yyyymmdd>-<hhmm>-2` のように suffix を付けて衝突を避ける。

`.cache/` は、HTML と assets を生成するための中間ファイル置き場として扱う。通常は export 終了時に削除され、利用者が成果物として扱うのは `index.html` と `assets/` だけにする。

`.cache/assets_manifest.json` は、元 URL、ローカルパス、asset 種別、Slack file ID、emoji 名、取得成否などを保持する候補として扱う。`.cache/metadata.json` は、取得対象 workspace、channel、取得時刻、Slack API 上の ID、取得件数などを保持する候補として扱う。`.cache/slack_api_cache.json` は、同じ export 中に何度も参照する Slack API response や解決済み user / emoji / channel 情報を保持する候補として扱う。

いずれの `.cache/` ファイルにも Slack token や secret は保存しない。

## HTML の表示仕様

最終成果物の `index.html` は、Slack default の投稿表示を模倣した見た目にする。

表示方針:

1. 投稿は channel timeline と同じく、上から oldest、下へ latest の順に表示する。
2. 日付と時刻は相対表現ではなく、絶対時刻として表示する。
3. thread replies は親投稿の下に、親投稿よりインデントを下げて表示する。
4. thread replies は初期表示で展開済みにする。
5. reaction は、絵文字 icon と件数を可能な限り Slack default 風に表示する。
6. reaction した user の一覧や名前は表示しない。
7. JavaScript は一切使わない。
8. style は `style.css` に分離し、HTML 内に固定的に inline style として埋め込まない。
9. CSS で表現可能な interaction は活用してよい。
10. thread の開閉を入れる場合は、JavaScript ではなく HTML native の `<details open>` / `<summary>` など、JavaScript なしで動作する仕組みを使う。

Slack default 風の avatar、投稿者名、絶対時刻、本文、reactions、attachments を CSS で整え、HTML 自体は静的 file として閲覧できるようにする。

ユーザーがアップロードした画像は、Slack file object の available な thumbnail のうち表示に適したものを保存し、HTML 上の inline image として使う。あわせて original 画像も保存し、inline image をクリックすると original を開けるようにする。

original 画像の保存には `--max-attachment-size` を適用する。original がサイズ上限を超える場合、original は download せず、HTML では thumbnail 表示を残したうえで original がサイズ上限超過により保存されなかったことを示す。thumbnail も取得できない場合は、通常の添付ファイル表示または置換メッセージとして扱う。

## 添付ファイルのサイズ制限

画像以外の添付ファイルも可能な限り保存対象に含める。また、ユーザーがアップロードした画像の original も保存対象に含める。ただし、巨大な添付ファイルや original 画像による実行時間、出力サイズ、CI artifact サイズの肥大化を避けるため、保存にはサイズ上限を設ける。

option:

| option | default | 目的 |
|---|---:|---|
| `--max-attachment-size <size>` | `10MB` | 添付ファイルまたは original 画像 1 件あたりの保存上限を指定する |

サイズ上限を超える添付ファイルまたは original 画像は download しない。画像以外の添付ファイルは、HTML 上では添付表示を次のようなメッセージに置き換える。original 画像が上限を超えた場合は、thumbnail 表示を残しつつ original が保存されなかったことを表示する。

```text
サイズオーバーのため保存されませんでした。
```

置換表示には、可能であればファイル名、Slack file ID、元の file size、設定された size limit を含める。`.cache/assets_manifest.json` には、保存した添付ファイルだけでなく、サイズ上限超過で保存しなかった添付ファイルの状態も記録する。

## cache の扱い

`.cache/` は最終成果物ではなく、HTML と assets を作成するための中間状態として扱う。

採用理由:

- 良い点: 最終成果物と中間ファイルを分離できるため、利用者が保存・共有すべきファイルが明確になる。
- 良い点: Slack API response、emoji list、asset download manifest、user 解決結果などを再帰的に参照しやすくなり、同じ実行内での重複 API call や重複 download を減らせる。
- 良い点: `--keep-cache` を指定すれば、どこまで取得できたか、どの asset が失敗したかを調査しやすい。
- 注意点: `.cache/` には channel 名、user ID、message ID、file ID、元 URL などが入り得るため、成果物として不用意に共有しない前提にする。
- 注意点: 古い `.cache/` を再利用すると、Slack 側の更新や権限変更を反映しない stale data になる可能性がある。

option:

| option | 目的 |
|---|---|
| `--keep-cache` | export の成否に関係なく `.cache/` を削除せず残す |
| `--reuse-cache <path>` | 以前に保存した `.cache/` を読み込み、取得済み情報や asset manifest を再利用する |

通常動作では、export の成否に関係なく `.cache/` を削除する。原因調査や cache 再利用のために残したい場合は `--keep-cache` を指定する。process kill や OS 側の異常終了では、cleanup が実行されず `.cache/` が残る可能性がある。

`--reuse-cache` を使う場合は、cache が同じ workspace、channel、token の見える権限範囲、取得条件に対応しているかを検証する必要がある。検証できない cache は使わず、再取得する。

`--no-cache` は初期 option としては採用しない。cache の影響を排除したい場合は `--reuse-cache` を指定せず通常実行すればよく、`--keep-cache` を指定しない限り `.cache/` は削除されるためである。

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
https://github.com/kiyohara/slack_posts_exporter/blob/main/doc/help/slack-app-setup.md
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
See: https://github.com/kiyohara/slack_posts_exporter/blob/main/doc/help/slack-app-setup.md
```

### token が無効

表示する内容:

1. secret manager または CI secrets に保存した token が正しいか確認する。
2. Slack App が workspace から uninstall されていないか確認する。
3. token を再発行または再 install する。
4. 詳細手順として help URL を表示する。

### token の workspace が想定と違う

表示する内容:

1. `auth.test` で確認した workspace 名、workspace URL、`team_id` を表示する。
2. 複数 workspace を扱う場合は、それぞれの workspace に対応する bot token を使う。
3. CI では job ごとに渡している `SLACK_BOT_TOKEN` が正しいか確認する。
4. Enterprise org-wide install の token を使っている場合は、初期対象外であることを表示し、単一 workspace install の bot token を使うよう案内する。

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

- `.cache/` 再利用時の整合性検証方法。
- 差分取得、再実行、既存出力への上書き方針。
- CI artifact としての保存方法。

## 参考

- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history)
- Slack Developer Docs: [`conversations.replies`](https://docs.slack.dev/reference/methods/conversations.replies/)
- Slack Developer Docs: [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list)
- Slack Developer Docs: [`files.info`](https://docs.slack.dev/reference/methods/files.info)
- Slack Developer Docs: [`files:read`](https://docs.slack.dev/reference/scopes/files.read)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test)
- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth)
