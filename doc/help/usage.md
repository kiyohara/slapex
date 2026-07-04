# 使い方

このページは、`slapex` の使い方(実行方法、option、cache、出力)の詳細をまとめたものです。初めて使う場合は、先に [クイックスタート](quickstart.md) を完走することをおすすめします。

各項目は利用時の要点を載せます。仕様の正本(開発者向けドキュメント)への参照が必要な項目には、文末の脚注へのリンクを付けています。

## token なしで試す(`--demo`)

Slack App や token を用意する前にまず試したい場合は、`--demo` で同梱の架空サンプルから export を生成できます(token 不要、実 Slack への通信なし)。ロケール(`LANG` など)が `ja` で始まる環境では日本語サンプル、それ以外では英語サンプルを使います。

```sh
# token なしで同梱サンプルを export(出力先を指定する例):
slapex --demo --output ./slapex-demo
```

## 実行の基本形

実際の workspace を export するときは token が必要です。token を CLI 引数では渡せません(プロセス一覧や shell history への漏えいを避けるため)。user token は通常 `xoxp-`、bot token は通常 `xoxb-` で始まります。

基本は `SLACK_TOKEN` を設定せずに実行し、表示された token 入力プロンプトにコピーした token を貼り付けます。入力は画面に表示(echo)されず、その 1 回の実行の中だけで使われます。

```sh
# channel keyword は channel 名・ID・名前の一部を指定する。
slapex engineering
```

channel を省略すると、操作可能な terminal がある環境では channel を対話選択できます。詳細は次の節を参照してください。

## 補足: 継続利用(secret manager など)

繰り返し使う場合は、1Password CLI などの secret manager から実行時に注入する方法を推奨します。token を都度コピーする必要がなくなり、実値を手元で扱う機会自体を減らせます。手順は [Token の渡し方](token-injection.md) を参照してください。

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering --output ./exports
```

CI や定期実行では CI secrets から `SLACK_TOKEN` を渡します。こちらも [Token の渡し方](token-injection.md) を参照してください。

## channel の対話選択

channel を指定せずに実行した場合、操作可能な terminal がある環境では channel を対話選択できます。slapex は対話選択の prompt を controlling terminal (`/dev/tty`) に直接出すため、`op run` 経由でも既定の secret masking のまま対話選択を使えます。CI など terminal が無い環境では候補と usage を表示し、非 0(exit 2)で終了します。[^spec-cli]

## 主要な option

主要な option は次のとおりです。全量と default・制約・exit code は脚注を参照してください。[^spec-cli]

| option | default | 用途 |
|---|---|---|
| `--output <path>` | 実行時刻から生成 | 出力 root を指定する |
| `--max-posts <count>` | `1000` | timeline 上の親投稿の最大取得件数(1〜10000) |
| `--days <days>` | `30` | 現在時刻から何日前までを取得するか(1〜90) |
| `--max-attachment-size <size>` | `10MB` | 添付ファイル / original 画像 1 件あたりの保存上限 |
| `--keep-cache` | off | 中間ファイル `.cache/` を成否に関係なく残す。`--reuse-cache` で再利用する cache を作るときに使う |
| `--reuse-cache <path>` | なし | 以前の出力ディレクトリまたは `.cache/` を再利用する |
| `--no-interactive` | off | TTY があっても対話選択を開始しない |
| `--no-color` | off | 進捗表示を色・アイコン・アニメーションなしの plain output にする |
| `--demo` | off | token なしで同梱の架空サンプルを export する(実 Slack に接続しない) |
| `--version` | | version を表示して終了する |
| `--help` | | usage を表示して終了する |

## cache の再利用

`.cache/` は通常実行の最後に削除されます。`--reuse-cache` で再利用するには、前回実行時に `--keep-cache` を付けて `.cache/` を残しておく必要があります。指定先は前回の出力ディレクトリでも、その直下の `.cache/` でも構いません。[^spec-cache]

## stdout / stderr と進捗表示

stdout には成功時に出力先ディレクトリ(`<workspace-label>/<channel-label>/` まで)の絶対 path を 1 行だけ出力し、進捗・診断・候補表示は stderr に出します。`out=$(slapex ...)` の形で出力先を後続処理へ渡せます。

進捗表示は、ターミナルでは色・状態アイコン・進行中フェーズの spinner 付きで表示され、CI / pipe / redirect、`CI` / `NO_COLOR` / `TERM=dumb` 環境変数、`--no-color` 指定時には ANSI escape のないプレーンな行出力に自動で切り替わります。[^spec-cli]

## 出力の構造

`--output` を省略すると、カレントディレクトリに `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成します(この日時はコマンド実行時刻で、投稿の日時ではありません)。

```text
slapex-20260602-1530/
└── <workspace-label>/
    └── <channel-label>/
        ├── index.html      # ブラウザで開く入口
        ├── style.css
        └── assets/         # 画像・絵文字・添付ファイルなど
```

生成された `index.html` をブラウザで開くと、取得した投稿・スレッド・assets をローカルだけで閲覧できます。出力ディレクトリ構造、保存される assets、取得範囲、サイズ制限の詳細は脚注を参照してください。[^spec-output]

取得範囲の default、再現されない表示、対応環境などの制限や、初回 export でつまずきやすい点は [よくある質問・制限事項](faq.md) にまとめています。

[^spec-cli]: 仕様の正本(開発者向け): [`doc/design/cli-interface.md`](../design/cli-interface.md)(option 一覧、exit code、出力制御)。
[^spec-cache]: 仕様の正本(開発者向け): [`doc/design/cache.md`](../design/cache.md)。
[^spec-output]: 仕様の正本(開発者向け): [`doc/design/output-format.md`](../design/output-format.md)。
