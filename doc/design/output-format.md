# 出力形式

このファイルには、`slapex` が生成する出力の構造、保存対象 assets、取得範囲、サイズ制限の仕様をまとめる。

想定読者は、このツールを利用する人間と、出力を実装・検証する担当者である。

本ファイルの出力ディレクトリ構造、option 名、制限値は確定仕様として扱う。実装アーキテクチャは未確定である。

利用者の操作の流れは `usage-flow.md`、CLI option と exit code の一覧は `cli-interface.md`、生成する `index.html` の表示仕様(見た目)は `html-rendering.md`、中間ファイル `.cache/` の扱いは `cache.md`、Slack API の取得方法は `slack-api-usage.md` を参照する。

## 取得範囲

初期出力は channel 単位の `index.html` とする。日付単位や thread 単位の HTML 分割は初期対象外とする。

歴史の長い channel を指定した場合に無制限取得にならないよう、取得範囲は post 件数と日付で制限する。2 つの制限は AND で結合し、両方を満たす投稿だけを取得対象にする。

option:

| option | default | max | 目的 |
|---|---:|---:|---|
| `--max-posts <count>` | `1000` | `10000` | channel timeline 上の親投稿の最大取得件数 |
| `--days <days>` | `30` | `90` | 現在時刻から何日前までの投稿を取得するか |

`--max-posts` は親投稿数だけを数え、thread replies は含めない。対象になった親投稿に thread replies がある場合、replies は一緒に取得する。ここでの「親投稿」は channel timeline 上に現れるメッセージを指し、thread への返信のうち channel にも送信されたもの(thread_broadcast)は timeline 上に現れるため数える。取得 API と pagination の詳細は `slack-api-usage.md` を参照する。

ただし、1 thread の replies が `1000` 件を超える場合は、それ以上の取得を取りやめ、HTML 上では残りの replies を次のようなメッセージに置き換える。

```text
取り扱える件数の上限に達しました。
```

親投稿数とは別に、thread replies を含めた全体取得量が大きくなり得る。取得前の見込み表示や、thread replies を含めた全体上限を設けるかどうかは未決事項として扱う。

## 保存する assets

ローカル HTML から外部 URL へ依存せず閲覧できるように、次の assets を保存対象とする。

| 種別 | 取得元 | 保存時の扱い |
|---|---|---|
| 標準絵文字 | Slack message text / Unicode emoji mapping | 原則として Unicode に戻して HTML に直接表示する。Unicode fallback できない場合だけ画像 asset として扱う |
| カスタム絵文字 | Slack API `emoji.list` | workspace 固有の絵文字画像として保存する |
| URL preview 画像 | Slack message の unfurl / attachment 情報 | Slack 上で preview として表示されていた画像を保存する。ツール自身による Open Graph fetch は行わない |
| ユーザーがアップロードした画像 | Slack message の `files` 情報、`files.info`、画像 thumbnail / original URL | thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開けるようにする |
| 画像以外の添付ファイル | Slack message の `files` 情報、`files.info`、download URL | サイズ上限以下の添付ファイルを保存し、HTML から相対リンクで参照する |

標準絵文字とカスタム絵文字については、関連評価実装である `slack_posts_dumper` に PoC 実装がある。PoC では `AssetManager` が `output/assets/<url-md5>.<ext>` のような URL hash ベースのファイル名を生成し、`output/assets_manifest.json` に元 URL、ローカルパス、metadata を記録する。`EmojiResolver` は `emoji.list` でカスタム絵文字を解決し、標準絵文字は Slack の標準絵文字 URL を組み立てている。

本リポジトリでも、asset ファイル名は PoC と同じく URL hash ベースにする。元 URL が同じ asset は同じファイル名へ解決されるため、重複 download と重複保存を避けやすい。asset 種別、元 URL、Slack file ID、emoji 名、元の表示ファイル名、content type、取得成否などの人間が読むための情報は `.cache/assets_manifest.json` と HTML 側の表示に保持する。

標準絵文字は原則として Unicode に戻して HTML に直接表示する。カスタム絵文字や Unicode fallback できない絵文字は画像 asset として保存するが、利用者にとって custom かどうかは重要な分類ではないため、保存先は `assets/emoji/` に集約する。

利用者が出力内容を把握しやすいように、ファイル名は URL hash ベースとしつつ、保存先は asset 種別ごとの分類ディレクトリに分ける。

## 添付ファイルのサイズ制限

画像以外の添付ファイルも可能な限り保存対象に含める。また、ユーザーがアップロードした画像の original も保存対象に含める。ただし、巨大な添付ファイルや original 画像による実行時間、出力サイズ、CI artifact サイズの肥大化を避けるため、保存にはサイズ上限を設ける。

option:

| option | default | 目的 |
|---|---:|---|
| `--max-attachment-size <size>` | `10MB` | 添付ファイルまたは original 画像 1 件あたりの保存上限を指定する |

サイズ上限を超える添付ファイルまたは original 画像は download しない。画像以外の添付ファイルは、HTML 上では添付表示を次のようなメッセージに置き換える。original 画像が上限を超えた場合は、thumbnail 表示を残しつつ original が保存されなかったことを表示する(表示仕様は `html-rendering.md` を参照)。

```text
サイズオーバーのため保存されませんでした。
```

置換表示には、可能であればファイル名、Slack file ID、元の file size、設定された size limit を含める。`.cache/assets_manifest.json` には、保存した添付ファイルだけでなく、サイズ上限超過で保存しなかった添付ファイルの状態も記録する。

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

`index.html` はローカルブラウザで開ける HTML とする。画像や添付ファイルへの参照は、可能な限り出力ディレクトリ内の相対パスにする。表示仕様は `html-rendering.md` を参照する。

`style.css` は `index.html` から相対 path で参照する。style は HTML 内に固定的に埋め込まず、将来的に theme 切り替えや style 差し替えをしやすいように分離する(見た目の詳細は `html-rendering.md`)。

`--output` が指定された場合、その値を出力 root とする。`--output` が指定されていない場合は、カレントディレクトリ配下に `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する。この日時はコマンド実行時刻を表し、取得対象となる投稿の日時ではない。`<workspace-label>/<channel-label>/` は token と channel 解決結果からツールが作成する。

`<workspace-label>` と `<channel-label>` は、Slack API 上の ID そのものではなく、人間が読みやすい workspace 名、workspace domain、channel 名などを filesystem-safe に正規化した label とする。label が取得できない場合や、正規化後に衝突する場合は、短い `team_id` / channel ID などを suffix または fallback として使う。元の ID、表示名、実際に使った label は metadata / cache に記録する。

### directory label の正規化規則

`<workspace-label>` は次の優先順で決める。

1. workspace domain の subdomain 部(例: `example.slack.com` → `example`)。
2. domain が取得できない場合は、workspace 名を下記の正規化にかけた結果。
3. 正規化の結果が空になる場合は `team_id`。

`<channel-label>` は channel 名から次の正規化で作る。

1. Unicode を NFC 正規化する。
2. `/` `\` `:` `*` `?` `"` `<` `>` `|`、空白、制御文字を `-` に置換する。
3. 連続する `-` を 1 つにまとめ、先頭・末尾の `-` を除去する。
4. 64 文字を超える場合は 64 文字で切り詰める。
5. 結果が空になる場合は channel ID を使う。

- 日本語などの Unicode 文字はそのまま保持する。「filesystem-safe」は、対象プラットフォーム(`cli-interface.md`)で予約・禁止される文字を含まないことを指す。
- 正規化の結果、同一出力 root 内で label が衝突する場合は、`-` + ID 末尾 6 文字を suffix として付ける。
- 決定経緯は `decision-log/0029-directory-label-rules.md` を参照する。

この directory 用 label は、画面表示用の workspace / channel label と同一である必要はない。画面表示では対象確認のために Slack 上の表示名、domain、短い ID、channel 種別などを含める(画面表示の方針は `usage-flow.md` の「処理対象の表示」を参照)。一方、directory 名では filesystem-safe な slug と衝突回避を優先する。

同じ分に複数回実行され、出力 root が既に存在する場合は、`slapex-<yyyymmdd>-<hhmm>-2` のように suffix を付けて衝突を避ける。

`.cache/` は最終成果物ではなく、HTML と assets を生成するための中間ファイル置き場である。各ファイルの内容、削除、再利用の方針は `cache.md` を参照する。利用者が成果物として扱うのは `index.html` と `assets/` だけにする。

## 未決事項

- 差分取得、再実行、既存出力への上書き方針。
- thread replies を含めた全体取得量の見込み表示、または全体上限を設けるかどうか。

これらを含む全体の未決事項一覧は `decision-log/index.md` を参照する。
