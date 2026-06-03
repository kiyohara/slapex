# 0008 Default Output Root

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../output-format.md`, `../usage-flow.md`

## 背景

利用手順では `--output ./exports` を明示指定していた。

しかし、初回利用では出力先を考えずに実行できる方が便利であり、`--output` を省略できるとコマンドが短くなる。

## 候補

- `--output` を必須にする。
- `--output` を省略可能にし、固定の `./exports` を使う。
- `--output` を省略可能にし、日時ベースの出力 root を自動作成する。

## 検討内容

`--output` 必須は出力先が明確だが、初回利用の手間が増える。

固定の `./exports` は分かりやすいが、再実行時に既存出力との衝突や上書き方針をすぐ考える必要がある。

日時ベースの出力 root は、出力先を考えずに実行でき、実行ごとの成果物を分けやすい。CI など固定 path が欲しい場合は `--output` を明示すればよい。

## 決定

`--output` は省略可能にする。

省略時は、カレントディレクトリ配下に `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する。この日時はコマンド実行時刻を表し、取得対象となる投稿の日時ではない。

```text
./slapex-20260602-1530/<workspace-label>/<channel-label>/
```

同じ分に複数回実行され、同名ディレクトリが既に存在する場合は、`slapex-<yyyymmdd>-<hhmm>-2` のように suffix を付けて衝突を避ける。

`--output` が指定された場合は、その値を出力 root として使う。

## 理由

初回利用では、出力先を考えずに短いコマンドで実行できる方が便利である。

コマンド実行時刻ベースの root にすることで、既存出力を誤って上書きしにくくなる。固定 path が必要な CI や script 実行では、`--output` を明示すればよい。

## 影響

- 基本の usage は `slapex <channel-keyword>` とする。
- `--output` は任意 option とする。
- CI 例では artifact path を固定しやすいよう、必要に応じて `--output ./exports` を明示する。
- 出力 root 配下の `<workspace-label>/<channel-label>/` は従来通り token と channel 解決結果から作成する。directory 名は人間が読みやすい label を優先し、詳細は `0013-output-directory-labels.md` に従う。

## 後から見直す条件

- 分単位の timestamp では衝突が多い。
- 利用者が出力ディレクトリ名から workspace / channel をすぐ識別したい。
- CI でデフォルト出力 root を使う場面が増え、artifact path 指定が煩雑になる。
