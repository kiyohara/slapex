# 0009 User Managed Slack App

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

Slack の bot token を得るには Slack App の作成、scope 設定、workspace install が必要になる。

このツールを利用する際に、利用者自身が Slack App を作成する前提にするか、ツール提供側が配布用 Slack App や OAuth flow を用意するかを決める必要があった。

## 候補

- 利用者自身が、自分用の Slack App を作成して token を発行する。
- ツール提供側が配布用 Slack App を用意し、OAuth flow で workspace install してもらう。

## 検討内容

利用者自身が Slack App を作成する方式は、導入時の手順は増えるが、個人利用の CLI ツールとしては運用が単純である。token の発行元、scope、install 先 workspace が利用者自身の管理下に置かれる。

配布用 Slack App と OAuth flow を用意する方式は、一般配布や SaaS 的な利用では便利だが、Slack App の配布設定、redirect URL、OAuth state 管理、token 保管、利用者ごとの install 管理が必要になる。今回の初期用途は、各個人がローカルまたは CI で個別に使う CLI ツールであり、そこまでの仕組みは過剰である。

## 決定

初期利用手順では、利用者自身が Slack App を作成し、自分が扱う workspace に install して bot token を発行する前提にする。

ツール提供側は、初期段階では配布用 Slack App、OAuth callback、OAuth token exchange、token storage を提供しない。

## 理由

このツールは、各個人が Slack 投稿をローカル HTML と assets として保存するための CLI ツールとして始める。個人利用では、利用者自身が Slack App と scope を管理する方が、責任範囲と secret の扱いが明確になる。

CI 実行でも、利用者が発行した bot token を CI secret store に登録し、`SLACK_BOT_TOKEN` として注入すればよい。

## 影響

- `usage-flow.md` の Slack App 準備手順は、利用者自身が App を作成する流れとして記載する。
- 初期実装では OAuth redirect endpoint や token storage を作らない。
- 配布用 Slack App の審査、配布設定、利用者別 install 管理は初期対象外とする。
- token はツールに保存せず、secret manager または CI secrets から `SLACK_BOT_TOKEN` として実行時に渡す。

## 後から見直す条件

- 複数利用者へ広く配布し、Slack App 作成手順を省略したい要件が出る。
- 組織内で統一 Slack App と centralized install を求められる。
- OAuth flow がないと利用者が導入できない運用になる。
