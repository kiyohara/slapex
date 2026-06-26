# Slack App Setup Help

このページは、`slapex` を使うために利用者自身が Slack App を作成し、Slack OAuth token を発行する手順をまとめる。

`slapex` は token を保存しない。発行した Slack OAuth token は secret manager または CI secrets に保存し、実行時に `SLACK_TOKEN` として渡す。`SLACK_BOT_TOKEN` は使わない。

## 前提

- 利用者自身が Slack App を作成する。
- App は取得対象の workspace に install する。
- Enterprise org-wide install は初期対象外とする。
- デフォルトの利用方法は user token(`xoxp-`)とする。
- CI、定期実行、チーム共通 automation、個人ユーザーに紐付けたくない運用では bot token(`xoxb-`)も正式サポートする。
- user token では、認可したユーザー本人が参照できる範囲の conversation が対象になる。
- bot token では、対応 scope に加えて bot / app が対象 conversation の member である必要がある。

## アクセスするページ

Slack App の作成と設定では、次の Slack API pages を使う。

| 目的 | URL |
|---|---|
| Slack App を新規作成する | <https://api.slack.com/apps?new_app=1> |
| 作成済み Slack App を管理する | <https://api.slack.com/apps> |
| App manifest の公式説明 | <https://docs.slack.dev/tools/app-manifests/> |
| token types の公式説明 | <https://docs.slack.dev/authentication/tokens/> |
| OAuth install の公式説明 | <https://docs.slack.dev/authentication/installing-with-oauth/> |

## どちらの token を使うか

| token type | 主な用途 | channel アクセス要件 |
|---|---|---|
| user token(`xoxp-`) | 個人が自分の参照できる Slack channel 履歴を手元に保存する | 認可したユーザー本人が見える範囲に従う |
| bot token(`xoxb-`) | CI、定期実行、チーム共通 automation | 対応 scope に加えて bot / app が対象 channel に参加している必要がある |

まずは user token を使う。CI やチーム共有の実行では bot token を使う。

## 推奨手順: user token 用 App を manifest から作成する

Slack App は、個別に scope を追加するだけでなく、manifest を貼り付けて必要設定をまとめて作成できる。

1. <https://api.slack.com/apps?new_app=1> を開く。
2. `Create New App` で `From an app manifest` を選ぶ。
3. 取得対象の workspace を選ぶ。
4. manifest editor の format が `JSON` になっていることを確認し、次の manifest を貼り付ける。
5. 表示された設定内容を確認して App を作成する。
6. 作成後、左メニューの `OAuth & Permissions` または `Install App` を開き、App を workspace に install / authorize する。権限設定によって承認 request が表示される場合は、workspace 管理者の承認を待つ。
7. install / authorize 後、`OAuth & Permissions` に表示される `User OAuth Token` を取得する。token は通常 `xoxp-` で始まる。
8. user token を secret manager または CI secrets に保存する。
9. `SLACK_TOKEN` として `slapex` 実行時に渡す。

Manifest の例:

```json
{
  "display_information": {
    "name": "slapex"
  },
  "oauth_config": {
    "scopes": {
      "user": [
        "channels:read",
        "channels:history",
        "groups:read",
        "groups:history",
        "files:read",
        "emoji:read",
        "users:read"
      ]
    }
  },
  "settings": {
    "org_deploy_enabled": false,
    "socket_mode_enabled": false,
    "token_rotation_enabled": false
  }
}
```

`display_information.name` は、workspace 上で分かりやすい名前に変更してよい。

## bot token を使う場合

bot token は CI、定期実行、チーム共通 automation、個人ユーザーに紐付けたくない運用で使う。

1. <https://api.slack.com/apps?new_app=1> を開く。
2. `Create New App` で `From an app manifest` を選ぶ。
3. 取得対象の workspace を選ぶ。
4. manifest editor の format が `JSON` になっていることを確認し、次の manifest を貼り付ける。
5. 表示された設定内容を確認して App を作成する。
6. 作成後、左メニューの `Install App` を開き、`Install to Workspace` を実行する。権限設定によって承認 request が表示される場合は、workspace 管理者の承認を待つ。
7. install / authorize 後、`Install App` または `OAuth & Permissions` に表示される `Bot User OAuth Token` を取得する。token は通常 `xoxb-` で始まる。
8. bot / app を取得対象 channel に参加させる。
9. bot token を secret manager または CI secrets に保存する。
10. `SLACK_TOKEN` として `slapex` 実行時に渡す。

Manifest の例:

```json
{
  "display_information": {
    "name": "slapex"
  },
  "features": {
    "bot_user": {
      "display_name": "slapex",
      "always_online": false
    }
  },
  "oauth_config": {
    "scopes": {
      "bot": [
        "channels:read",
        "channels:history",
        "groups:read",
        "groups:history",
        "files:read",
        "emoji:read",
        "users:read"
      ]
    }
  },
  "settings": {
    "org_deploy_enabled": false,
    "socket_mode_enabled": false,
    "token_rotation_enabled": false
  }
}
```

`display_information.name` と `features.bot_user.display_name` は、workspace 上で分かりやすい名前に変更してよい。

## 手動設定する場合

manifest を使わずに設定する場合は、次の手順で作成する。

1. <https://api.slack.com/apps?new_app=1> を開き、Slack API の App 管理画面で App を作成する。
2. App を取得対象の workspace に紐付ける。
3. user token を使う場合は、App の OAuth & Permissions で User Token Scopes を設定する。
4. bot token を使う場合は、App の OAuth & Permissions で Bot Token Scopes を設定し、必要に応じて bot user を有効化する。
5. App を workspace に install / authorize する。権限設定によって承認 request が表示される場合は、workspace 管理者の承認を待つ。
6. user token では `User OAuth Token`、bot token では `Bot User OAuth Token` を取得する。
7. bot token を使う場合は、bot / app を取得対象 channel に参加させる。
8. token を secret manager または CI secrets に保存する。
9. `SLACK_TOKEN` として `slapex` 実行時に渡す。

## Scopes

public channel と private channel の両方を扱えるように、user token / bot token のどちらでも次の scope を設定する。manifest を使う場合、user token では `oauth_config.scopes.user`、bot token では `oauth_config.scopes.bot` に記載済みである。

| 目的 | scope |
|---|---|
| public channel の一覧・解決 | `channels:read` |
| public channel の投稿取得 | `channels:history` |
| private channel の一覧・解決 | `groups:read` |
| private channel の投稿取得 | `groups:history` |
| スレッド返信の取得 | 対象 conversation 種別に対応する `*:history` |
| 画像・添付ファイルの情報取得と download | `files:read` |
| カスタム絵文字の一覧取得 | `emoji:read` |
| 投稿者名や表示名の解決 | `users:read` |

scope を追加または変更した場合は、App を workspace に再 install / 再 authorize し、更新された token を secret manager または CI secrets に反映する。

## Channel access

user token を使う場合、認可したユーザー本人が参照できる範囲がアクセス範囲になる。対象 channel が見えない場合は、そのユーザーが対象 channel を参照できるか、必要 scope が付いているかを確認する。

bot token を使う場合、public channel / private channel の投稿を取得するには、対応する scope だけでは不十分である。取得対象 channel に bot / app が参加している必要がある。参加していない場合は、Slack 上で bot / app を対象 channel に追加する。private channel の場合は、その private channel の参加者が bot / app を招待する。

## Token の渡し方

ローカルの `.env` などに token の実値を保存することは推奨しない。

Slack OAuth token は `SLACK_TOKEN` として渡す。`SLACK_BOT_TOKEN` は使わない。

1Password CLI を使う例:

```sh
SLACK_TOKEN="op://<vault>/<item>/<field>" \
  op run -- slapex engineering
```

CI では secret store から `SLACK_TOKEN` を job に渡す。CI / automation では bot token を `SLACK_TOKEN` に入れる運用を基本候補とする。

```yaml
steps:
  - name: Export Slack posts
    env:
      SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
    run: |
      slapex engineering --output ./exports
```

## よくあるエラー

### `SLACK_TOKEN` が未設定

`SLACK_TOKEN` を環境変数として渡してから再実行する。`SLACK_BOT_TOKEN` は参照されない。

### token が無効

secret manager または CI secrets に保存した token が正しいか確認する。App を uninstall した場合や scope を変更した場合は、再 install / 再 authorize して token を更新する。

### scope が不足している

OAuth & Permissions で不足 scope を追加し、App を workspace に再 install / 再 authorize する。更新後の token を secret manager または CI secrets に反映する。

### channel が見えない

channel 名または channel ID を確認する。user token の場合は、認可したユーザーが対象 channel を参照できるか確認する。bot token の場合は、bot / app がその channel に参加しているか確認する。

### bot token 利用時に bot が channel に参加していない

public channel の場合は Slack 上で bot / app を channel に追加する。private channel の場合は、その private channel の参加者が bot / app を招待する。

## 実 token E2E の確認計画

リリース前に、実 token の値を記録しない形で次を確認する。

| token type | 確認内容 |
|---|---|
| user token | public channel、参加済み private channel、thread replies、file download、emoji、user 解決 |
| bot token | bot / app 参加済み public channel、bot / app 参加済み private channel、thread replies、file download、emoji、user 解決 |

確認結果を PR や working branch note に残す場合は、token 実値、workspace 固有の非公開情報、channel 固有の非公開情報を書かない。

## 参考

- Slack Developer Docs: [Creating apps with manifests](https://docs.slack.dev/tools/app-manifests/)
- Slack Developer Docs: [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth/)
- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test/)
- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history/)
- Slack Developer Docs: [`conversations.replies`](https://docs.slack.dev/reference/methods/conversations.replies/)
- Slack Developer Docs: [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list/)
