# Slack App Setup Help

このページは、`slapex` を使うために利用者自身が Slack App を作成し、bot token を発行する手順をまとめる。

`slapex` は token を保存しない。発行した bot token は secret manager または CI secrets に保存し、実行時に `SLACK_BOT_TOKEN` として渡す。

## 前提

- 利用者自身が Slack App を作成する。
- App は取得対象の workspace に install する。
- Enterprise org-wide install は初期対象外とする。
- public channel と private channel の scope は同じ設定手順でまとめて設定する。
- private channel を取得する場合は、scope だけでなく bot / app がその channel に参加している必要がある。

## アクセスするページ

Slack App の作成と設定では、次の Slack API pages を使う。

| 目的 | URL |
|---|---|
| Slack App を新規作成する | <https://api.slack.com/apps?new_app=1> |
| 作成済み Slack App を管理する | <https://api.slack.com/apps> |
| App manifest の公式説明 | <https://api.slack.com/reference/manifests> |
| Bot token / token types の公式説明 | <https://api.slack.com/concepts/token-types> |

## 推奨手順: manifest から App を作成する

Slack App は、個別に scope を追加するだけでなく、manifest を貼り付けて必要設定をまとめて作成できる。

1. <https://api.slack.com/apps?new_app=1> を開く。
2. `Create New App` で `From an app manifest` を選ぶ。
3. 取得対象の workspace を選ぶ。
4. 次の manifest を貼り付ける。
5. 表示された設定内容を確認して App を作成する。
6. 作成後、`OAuth & Permissions` で `Install to Workspace` を実行する。
7. install 後に表示される `Bot User OAuth Token` を取得する。
8. bot / app を取得対象 channel に参加させる。
9. bot token を secret manager または CI secrets に保存する。
10. `SLACK_BOT_TOKEN` として `slapex` 実行時に渡す。

Manifest の例:

```yaml
display_information:
  name: slapex
features:
  bot_user:
    display_name: slapex
    always_online: false
oauth_config:
  scopes:
    bot:
      - channels:read
      - channels:history
      - groups:read
      - groups:history
      - files:read
      - emoji:read
      - users:read
settings:
  org_deploy_enabled: false
  socket_mode_enabled: false
  token_rotation_enabled: false
```

`display_information.name` と `features.bot_user.display_name` は、workspace 上で分かりやすい名前に変更してよい。

## 手動設定する場合

manifest を使わずに設定する場合は、次の手順で作成する。

1. <https://api.slack.com/apps?new_app=1> を開き、Slack API の App 管理画面で App を作成する。
2. App を取得対象の workspace に紐付ける。
3. App の OAuth & Permissions で Bot Token Scopes を設定する。
4. App を workspace に install する。
5. install 後に発行される bot token を取得する。
6. bot / app を取得対象 channel に参加させる。
7. bot token を secret manager または CI secrets に保存する。
8. `SLACK_BOT_TOKEN` として `slapex` 実行時に渡す。

## Bot Token Scopes

初期手順では、public channel と private channel の両方を扱えるように次の scope をまとめて設定する。manifest を使う場合は、これらの scope は `oauth_config.scopes.bot` に記載済みである。

| 目的 | scope |
|---|---|
| public channel の一覧・解決 | `channels:read` |
| public channel の投稿取得 | `channels:history` |
| private channel の一覧・解決 | `groups:read` |
| private channel の投稿取得 | `groups:history` |
| 画像・添付ファイルの情報取得と download | `files:read` |
| カスタム絵文字の一覧取得 | `emoji:read` |
| 投稿者名や表示名の解決 | `users:read` |

scope を追加または変更した場合は、App を workspace に再 install し、更新された token を secret manager または CI secrets に反映する。

## Private Channel

private channel の投稿を取得するには、`groups:read` と `groups:history` だけでは不十分な場合がある。

取得対象の private channel に bot / app が参加している必要がある。参加していない場合は、その private channel の参加者が Slack 上で bot / app を招待する。

## Token の渡し方

ローカルの `.env` などに token の実値を保存することは推奨しない。

Slack の Bot User OAuth Token は通常 `xoxb-` で始まる。`slapex` には、この bot token を `SLACK_BOT_TOKEN` として渡す。

1Password CLI を使う例:

```sh
SLACK_BOT_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering
```

CI では secret store から `SLACK_BOT_TOKEN` を job に渡す。

```yaml
steps:
  - name: Export Slack posts
    env:
      SLACK_BOT_TOKEN: ${{ secrets.SLACK_BOT_TOKEN }}
    run: |
      slapex engineering --output ./exports
```

## よくあるエラー

### `SLACK_BOT_TOKEN` が未設定

`SLACK_BOT_TOKEN` を環境変数として渡してから再実行する。

### token が無効

secret manager または CI secrets に保存した token が正しいか確認する。App を uninstall した場合や scope を変更した場合は、再 install して token を更新する。

### scope が不足している

OAuth & Permissions で不足 scope を追加し、App を workspace に再 install する。更新後の token を secret manager または CI secrets に反映する。

### channel が見えない

channel 名または channel ID を確認する。private channel の場合は、bot / app がその channel に参加しているか確認する。

## 参考

- Slack Developer Docs: [Creating Slack apps](https://docs.slack.dev/tools/app-manifests/)
- Slack Developer Docs: [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth)
- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test)
