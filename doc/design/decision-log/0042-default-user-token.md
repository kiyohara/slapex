# 0042 Default User Token

- 状態: decided
- 作成日: 2026-06-24
- 最終更新日: 2026-06-26
- 関連: `../usage-flow.md`, `../cli-interface.md`, `../slack-api-usage.md`

## 背景

これまでの `slapex` は、利用者自身が Slack App を作成し、bot token(`xoxb-`)を発行して `SLACK_BOT_TOKEN` として渡す前提で設計していた。

この前提は、当初の Slack token 種別に関する理解不足に由来する。Slack には bot token だけでなく user token(`xoxp-`)があり、user token は認可したユーザー本人の権限で Slack Web API を呼び出せる。

`slapex` の主用途は、多くの利用者にとって「自分が参照できる Slack channel の履歴を手元に保存する」ことである。この用途では、bot / app を対象 channel に参加させる運用よりも、ユーザー本人の可視範囲をそのまま使う user token の方が自然である。一方で、CI 上で定期実行する場合や共有された automation では、bot token の方が責任範囲と運用が明確になる。

## 候補

- bot token のみを正式サポートし続ける。
- user token をデフォルト利用方法にし、bot token も正式サポートする。
- user token のみを正式サポートし、bot token を非推奨または対象外にする。
- ツール提供側が配布用 Slack App と OAuth flow を用意し、利用者別 token を発行・管理する。

## 検討内容

bot token のみの方式は CI やチーム共通 automation では扱いやすいが、Slack の `conversations.history` において bot が member である conversation しか読めない。public channel であっても bot / app の参加が必要になるため、個人が普段見ている channel を手元に保存する用途では導入手順が重くなる。

user token はユーザー本人に紐付くため、ユーザーが見える public channel や参加済み private channel を扱う用途に合う。Slack 公式ドキュメントでは、user token はユーザーの代わりに操作する token と説明され、`conversations.history` の accessible conversations でも bot token とは異なる範囲が示されている。

user token には、個人の権限を帯びるため secret としての重みが増す、CI やチーム共有の運用には向かない、組織ポリシーによって user scope の承認が必要になり得る、という制約がある。そのため bot token の正式サポートは残す必要がある。

配布用 Slack App と OAuth flow を `slapex` 側が提供する方式は、導入体験をさらに改善し得るが、redirect URL、state 管理、token exchange、token storage、配布設定、審査や組織導入の責任が増える。現時点では CLI ツールとして過剰であり、利用者自身が token を発行し secret manager / CI secrets から注入する方式を維持する。

## 決定

`slapex` のデフォルト利用方法は user token をベースにする。

bot token は CI 実行、定期実行、チーム共通 automation、個人ユーザーに紐付けたくない運用のために正式サポートを継続する。bot token 利用時は、scope に加えて bot / app が対象 channel に参加している必要があることを明示する。

ツール提供側は、現時点では配布用 Slack App、OAuth callback、OAuth token exchange、token storage を提供しない。利用者が自分の管理下で Slack App / token を用意し、実行時に環境変数から渡す方式を維持する。

今後の CLI とドキュメントでは、汎用の Slack OAuth token を受け取る前提へ改める。環境変数名は `SLACK_TOKEN` とし、旧 bot token 前提設計で使っていた `SLACK_BOT_TOKEN` との互換性は維持しない。bot token を使う場合も `SLACK_TOKEN` に渡す。

2026-06-26 に Issue #81 で、user token / bot token ごとの scope、channel access、エラー案内、E2E 計画を整理し、次の修正リリースを v1.0.1 として進める方針を確定した。

## 理由

主用途を「利用者本人が見ている Slack 履歴の保存」と置くと、user token の方が Slack の権限モデルに沿っている。bot token をデフォルトにすると、利用者は本来閲覧済みの public channel でも app 招待を追加で行う必要があり、導入の失敗点が増える。

一方で、CI 実行やチーム共通の運用では user token は個人依存が強く、退職・権限変更・個人 secret 管理の問題を招きやすい。bot token はこの用途では妥当であり、廃止すべきではない。

このため、デフォルトを user token に転換しつつ、bot token を同等に検証されたサポート対象として残す方針が、個人利用と automation の両方に対して最も整合的である。

## 影響

- `README.md` と `doc/help/slack-app-setup.md` は、user token を基本手順、bot token を CI / automation 向け手順として再構成する必要がある。
- `usage-flow.md`、`cli-interface.md`、`slack-api-usage.md` は、bot token 固定の記述を Slack OAuth token / token type 別の記述に更新する必要がある。
- 環境変数名は `SLACK_TOKEN` とする。`SLACK_BOT_TOKEN` は参照しないため、旧環境変数を使っていた利用者は `SLACK_TOKEN` へ移行する必要がある。
- token type ごとの scope、channel membership、エラー診断を分ける必要がある。
- bot token では public / private のどちらも bot / app の channel 参加が必要であることを明記する必要がある。
- user token と bot token の両方で実 token E2E を行い、`conversations.list`、`conversations.history`、`conversations.replies`、file download、emoji / user 解決の差分を確認する必要がある。
- 既存の bot token 前提の decision log は、この方針で上書きされた履歴として残す。

Tracking Issue: [#81](https://github.com/kiyohara/slapex/issues/81)

## 後から見直す条件

- Slack 側の user token / bot token の権限モデル、scope、rate limit、App 管理 UI が大きく変わる。
- 多くの利用者環境で user scope の承認が困難で、bot token の方が導入しやすいことが分かる。
- CI / automation 利用が主用途になり、個人ローカル実行よりも bot token default の方が実態に合う。
- ツール提供側が配布用 Slack App と OAuth flow を持つ価値が、運用コストを上回る。

## 参考

- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth/)
- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history/)
- Slack Developer Docs: [`channels:history`](https://docs.slack.dev/reference/scopes/channels.history/)
- Slack Developer Docs: [`groups:history`](https://docs.slack.dev/reference/scopes/groups.history/)
