# 0015 Channel Scope Setup

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

Slack App の bot token scopes を案内する際に、public channel 用 scope と private channel 用 scope を用途別に分けて手順化するか、同じ設定手順でまとめて扱うかを決める必要があった。

## 候補

- public channel だけを扱う手順と、private channel も扱う手順を分ける。
- 初期手順では public / private channel の scope を同じ設定手順で扱う。

## 検討内容

scope 設定を用途別に分けると最小権限に近づけられるが、利用者が自分の用途に応じて手順を選ぶ必要がある。

このツールは channel export を目的とし、private channel も自然な利用対象になる。初期 How to Use では、public / private channel の両方に対応できる scope をまとめて設定する方が、手順が単純で失敗しにくい。

ただし、private channel は scope を付与するだけでは取得できない。bot が対象 private channel に参加している必要がある。

## 決定

初期利用手順では、public channel と private channel の scope を同じ設定手順で扱う。

利用者には `channels:read`、`channels:history`、`groups:read`、`groups:history` をまとめて設定するよう案内する。

private channel を扱う場合は、scope 設定に加えて、対象 private channel へ bot / app を招待する必要があることを明記する。

## 理由

利用者向け手順を分岐させないことで、Slack App 作成時の迷いを減らせる。

後から private channel を export したくなった場合でも、scope 設定をやり直す可能性を下げられる。

## 影響

- `usage-flow.md` の scope 表は public / private channel scope を同じ設定手順として案内する。
- `groups:*` scope を「private channel を扱う場合だけ任意」とは書かない。
- private channel の取得可否は scope だけでなく bot membership に依存する、と案内する。
- 最小権限をさらに絞りたい運用は、初期 How to Use の基本パスから外す。

## 後から見直す条件

- 組織ポリシー上、private channel scope を標準手順に含められない。
- public channel だけを扱う軽量手順が必要になる。
- scope を厳密に最小化する setup wizard が必要になる。
