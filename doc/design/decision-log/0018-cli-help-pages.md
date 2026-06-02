# 0018 CLI help pages

- 状態: decided
- 作成日: 2026-06-03
- 最終更新日: 2026-06-03
- 関連: `doc/design/usage-flow.md`, `doc/help/slack-app-setup.md`, `0009-user-managed-slack-app.md`

## 背景

`slapex` は利用者自身が Slack App を作成し、workspace に install して bot token を発行する前提で始める。

一方で、Slack App の作成、scope 設定、install、bot token 発行、secret manager / CI secrets への登録は手順が長い。これらを CLI のエラー出力として毎回表示すると、重要な診断情報が埋もれ、CI log も読みづらくなる。

## 候補

- CLI のエラー出力に詳細なセットアップ手順をステップごとに表示する。
- CLI のエラー出力は短い診断と次の最小アクションに絞り、詳細手順は GitHub 上で読める help ページへ誘導する。
- CLI に setup wizard や interactive guide を実装する。

## 検討内容

詳細な手順を CLI に直接表示すると、初回利用者には便利に見えるが、エラー出力が長くなり、再実行や CI ではノイズになりやすい。

help ページへ誘導する方式であれば、CLI は短い診断に集中できる。GitHub 上の Markdown として公開すれば、Slack App の画面変更や scope 変更に合わせて手順を更新しやすい。

setup wizard は導入支援として有効な可能性があるが、Slack App 作成や token 発行は Slack の管理画面操作を含むため、初期実装では過剰である。

## 決定

Slack App の作成、scope 設定、workspace install、bot token 発行、secret manager / CI secrets への登録手順は、リポジトリ内の help ページに分離する。

初期 help ページは `doc/help/slack-app-setup.md` とする。GitHub 上では次の URL で参照できる前提にする。

```text
https://github.com/kiyohara/slack_posts_exporter/blob/main/doc/help/slack-app-setup.md
```

CLI のエラー出力は、短い原因説明、次に確認すべき最小限の内容、help URL に絞る。詳細な Slack App セットアップ手順は CLI に展開しない。

## 理由

CLI の出力を短く保つことで、ローカル実行でも CI 実行でも診断情報を読み取りやすくなる。

セットアップ手順を Markdown として分離すれば、GitHub 上で共有しやすく、Slack 側の UI や必要 scope が変わった場合にも更新しやすい。

## 影響

`usage-flow.md` の「情報が足りない場合の案内」は、長いステップ表示ではなく help URL を案内する方針に更新する。

`doc/help/slack-app-setup.md` を追加し、Slack App 作成、scope 設定、install、token 注入、よくあるエラーをまとめる。

help ページには Slack API の App 作成 URL を明記し、個別 scope 設定だけでなく manifest を貼り付けて App を作成する手順を載せる。

実装時は、token 未設定、token 無効、scope 不足、bot が channel に参加していない場合などのエラーに help URL を含める。

## 後から見直す条件

利用者が help ページを見てもセットアップできない場合は、スクリーンショット付き手順、FAQ、または interactive setup wizard を検討する。

配布用 Slack App や OAuth flow を採用する方針に変わった場合は、help ページと CLI エラー案内を見直す。
