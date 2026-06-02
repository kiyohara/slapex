# 0006 No Subcommands Initially

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

利用手順の素案では `slack-posts-exporter export ...` のように `export` subcommand を置いていた。

しかし、初期の主要な利用目的は Slack 投稿を HTML と assets として保存することだけであり、利用者の主導線としては subcommand がなくても十分に表現できる。

## 候補

- `export` subcommand を置く。
- 初期 CLI は subcommand なしにする。
- `doctor`、`channels`、`cache` などの補助 subcommand を最初から用意する。

## 検討内容

`export` subcommand は、将来的に複数の独立した操作が増える場合には自然な構造である。

一方、現時点では export が唯一の主操作であり、root command に channel 引数や `--output ...` を直接指定する方が短く、利用者の意図も明確である。

`doctor`、`channels`、`cache` などは候補として考えられるが、初期時点では export 実行時の事前確認、channel selection、cache option と重複する部分が大きい。

## 決定

初期 CLI では subcommand を採用しない。

基本形は次のようにする。

```sh
slapex <channel-keyword> --output ./exports
```

将来、独立した操作が必要になった場合に subcommand 導入を再検討する。

## 理由

初期のユーザー体験では、単一の主操作を最短の CLI で実行できる方が分かりやすい。

まだ必要性が固まっていない補助操作のために subcommand 構造を先取りすると、初期利用手順が重くなる。

## 影響

- `usage-flow.md` のコマンド例から `export` subcommand を外す。
- CLI option は root command に直接ぶら下げる。
- 将来の `doctor`、`channels`、`cache` などは、必要性が明確になった時点で追加検討する。

## 後から見直す条件

- export 以外の独立した操作が増える。
- root command の option が増えすぎて分かりにくくなる。
- package manager や CLI framework の制約上、subcommand の方が自然になる。
