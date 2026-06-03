# 0011 Channel HTML And Fetch Limits

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../output-format.md`

## 背景

出力 HTML の粒度を channel 単位にするか、日付単位や thread 単位に分割するかを決める必要があった。

また、歴史の長い channel を指定した場合に、過去の投稿を無制限に取得し続けることは避ける必要がある。

## 候補

- channel 単位の HTML を生成し、取得範囲は制限しない。
- channel 単位の HTML を生成し、取得範囲を post 件数と日付で制限する。
- 日付単位または thread 単位に HTML を分割する。

## 検討内容

channel 単位の HTML は、利用者が「この channel の export」として理解しやすく、初期出力として単純である。

日付単位や thread 単位に HTML を分割すると、大規模 channel では扱いやすくなる可能性がある。一方、初期仕様では navigation、index 生成、分割単位、相互リンクなどの設計が増える。

無制限取得は、実行時間、Slack API call、出力 HTML サイズ、CI artifact サイズの面で危険である。post 件数と日付の両方で制限すれば、初期実行の安全性を確保しつつ、必要に応じて利用者が範囲を広げられる。

## 決定

初期出力 HTML は channel 単位にする。

取得範囲は次の 2 つの条件で制限し、両方を AND で満たす投稿だけを取得対象にする。

- post 件数: default `1000`、option `--max-posts <count>` で指定可能、指定可能な上限は `10000`。
- 日付: default `30` 日以内、option `--days <days>` で日単位指定可能、指定可能な上限は `90` 日。

`--max-posts` は channel timeline 上の親投稿数として扱い、thread replies は件数に含めない。

対象になった親投稿に thread replies がある場合、thread replies は一緒に取得する。ただし、1 thread の replies が `1000` 件を超える場合は、それ以上の取得を取りやめ、HTML 上では残りの replies を次のようなメッセージに置き換える。

```text
取り扱える件数の上限に達しました。
```

## 理由

channel 単位の HTML は、初期の How to Use と CLI 体験に合っている。出力分割を後回しにすることで、まず export の主導線を単純に保てる。

一方で、取得範囲を無制限にすると長期間運用されている channel で実行が重くなる。`--max-posts` と `--days` の AND 条件にすれば、「直近 N 日以内、かつ最大 M posts まで」という利用者に説明しやすい制限になる。

`--max-posts` を親投稿数として扱うと、利用者が channel timeline 上の投稿数として理解しやすい。thread replies は投稿文脈に必要なため取得対象に含めるが、巨大 thread による暴走を避けるため per-thread の replies 上限を設ける。

## 影響

- `usage-flow.md` に channel 単位 HTML と取得範囲制限を記載する。
- 実装では `--max-posts` の default / max validation が必要になる。
- 実装では `--days` の default / max validation が必要になる。
- Slack API 取得では日付条件と post 件数条件を AND で適用する。
- `--max-posts` は親投稿数だけを数え、thread replies は含めない。
- thread replies が 1000 件を超えた場合は、その thread の追加取得を止め、HTML に上限到達メッセージを出す。

## 後から見直す条件

- channel 単位 HTML が大きくなりすぎ、読み込みや閲覧が遅くなる。
- 日付単位、月単位、thread 単位の分割出力が必要になる。
- thread replies の 1000 件上限が厳しすぎる、または緩すぎる。
- `--max-posts` と `--days` だけでは取得範囲指定が不足する。
