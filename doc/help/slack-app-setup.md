# Slack App Setup Help

このページは、`slapex` を使うために利用者自身が Slack App を作成し、Slack OAuth token を発行する手順をまとめる。

`slapex` は token を保存しない。発行した Slack OAuth token は secret manager または CI secrets に保存し、実行時に `SLACK_TOKEN` として渡す。

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

スクリーンショットは撮影時点の Slack UI であり、実際の画面と細部が異なる場合がある。その場合は本文の手順を正として読み替える。スクリーンショット内の workspace 名(`myworkspace`)はダミーで、token はマスク済みである。実際の画面では自分の workspace 名と token 実値が表示される。

### 1. App を新規作成する

1. <https://api.slack.com/apps?new_app=1> を開く。
2. `Create an app` ダイアログで `From a manifest` をクリックする。

![Create an app ダイアログで From a manifest を選ぶ](../../assets/screenshots/slack-app-setup/01-create-new-app.png)

3. workspace 選択画面で取得対象の workspace を選び、`Next` をクリックする。

![取得対象の workspace を選ぶ](../../assets/screenshots/slack-app-setup/02-pick-workspace.png)

### 2. manifest を貼り付ける

1. manifest editor 上部の tab が `JSON` になっていることを確認する。
2. editor の内容をすべて消し、下の「Manifest の例」を貼り付けて `Next` をクリックする。

![JSON tab を選び manifest を貼り付ける](../../assets/screenshots/slack-app-setup/03-paste-manifest-user.png)

3. Review summary に `User Scopes` の一覧が表示される。内容を確認して `Create` をクリックすると App が作成される。

![Review summary を確認して Create をクリックする](../../assets/screenshots/slack-app-setup/04-review-summary-user.png)

### 3. App を workspace に install する

1. App 管理画面の左メニューから `OAuth & Permissions` を開く。
2. ページ上部の `OAuth Tokens` にある `Install to <workspace 名>` ボタンをクリックする。

![OAuth & Permissions の Install to <workspace 名> をクリックする](../../assets/screenshots/slack-app-setup/05-install-to-workspace-user.png)

3. 認可画面で要求される権限を確認し、`Allow` をクリックする。権限設定によって承認 request が表示される場合は、workspace 管理者の承認を待つ。

![認可画面で Allow をクリックする](../../assets/screenshots/slack-app-setup/06-authorize-user.png)

### 4. User OAuth Token を取得する

1. install / authorize 後、`OAuth & Permissions` の `OAuth Tokens` に `User OAuth Token` が表示される。token は通常 `xoxp-` で始まる。
2. `Copy` をクリックして token をコピーする。

![OAuth Tokens に表示された User OAuth Token をコピーする](../../assets/screenshots/slack-app-setup/07-oauth-token-user.png)

3. user token を secret manager または CI secrets に保存する。token をファイルや chat に貼らない。
4. `SLACK_TOKEN` として `slapex` 実行時に渡す。

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
        "team:read",
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

App の作成から install までの画面操作は user token の場合と同じ流れである(App 新規作成と workspace 選択の画面は「推奨手順」のスクリーンショットを参照)。manifest だけ bot token 用のものを使う。

1. <https://api.slack.com/apps?new_app=1> を開く。
2. `Create an app` ダイアログで `From a manifest` を選ぶ。
3. 取得対象の workspace を選ぶ。
4. manifest editor の tab が `JSON` になっていることを確認し、下の「Manifest の例」を貼り付けて `Next` をクリックする。

![JSON tab を選び bot token 用 manifest を貼り付ける](../../assets/screenshots/slack-app-setup/03-paste-manifest-bot.png)

5. Review summary に `Bot Scopes` の一覧が表示される。内容を確認して `Create` をクリックすると App が作成される。

![Review summary に Bot Scopes が表示される](../../assets/screenshots/slack-app-setup/04-review-summary-bot.png)

6. 左メニューの `OAuth & Permissions` を開き、`OAuth Tokens` にある `Install to <workspace 名>` ボタンをクリックする。

![OAuth & Permissions の Install to <workspace 名> をクリックする](../../assets/screenshots/slack-app-setup/05-install-to-workspace-bot.png)

7. 認可画面で要求される権限を確認し、`Allow` をクリックする。権限設定によって承認 request が表示される場合は、workspace 管理者の承認を待つ。

![認可画面で Allow をクリックする](../../assets/screenshots/slack-app-setup/06-authorize-bot.png)

8. install / authorize 後、`OAuth & Permissions` の `OAuth Tokens` に表示される `Bot User OAuth Token` を取得する。token は通常 `xoxb-` で始まる。

![OAuth Tokens に表示された Bot User OAuth Token をコピーする](../../assets/screenshots/slack-app-setup/08-oauth-token-bot.png)

9. bot / app を取得対象 channel に参加させる(手順は「bot / app を channel に参加させる」を参照)。
10. bot token を secret manager または CI secrets に保存する。
11. `SLACK_TOKEN` として `slapex` 実行時に渡す。

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
        "team:read",
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
7. bot token を使う場合は、bot / app を取得対象 channel に参加させる(手順は「bot / app を channel に参加させる」を参照)。
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
| workspace icon の取得(任意。無い場合は icon なしで出力) | `team:read` |
| 投稿者名や表示名の解決 | `users:read` |

### scope 変更後の再 install

scope を追加または変更すると、`OAuth & Permissions` の画面上部に再 install を促す banner が表示される。banner 内のリンク(または `Install to Workspace` / `Reinstall to Workspace`)から App を workspace に再 install / 再 authorize する。

![scope 変更後に表示される再 install banner](../../assets/screenshots/slack-app-setup/11-reinstall-banner.png)

再 install / 再 authorize すると token が更新される場合がある。`OAuth & Permissions` に表示されている現在の token を、secret manager または CI secrets に反映する。

## bot / app を channel に参加させる

bot token で channel の投稿を取得するには、scope に加えて bot / app が対象 channel の member である必要がある。参加のさせ方は public channel と private channel で異なる。

user token を使う場合、この操作は不要である(認可したユーザー本人が参照できる範囲が対象になる)。

### public channel の場合

対象 channel のメッセージ入力欄で `/invite @<App 名>` を実行する。この help の manifest どおりに作成した場合は `/invite @slapex` になる。

![public channel で /invite @slapex を実行する](../../assets/screenshots/slack-app-setup/09-invite-public-channel.png)

メッセージ入力欄に `@<App 名>` をメンションとして投稿し、表示される案内から channel に追加する方法でもよい。

### private channel の場合

private channel には、その channel の参加者が App を追加する。

1. 対象 private channel を開き、channel 名をクリックして設定を開く。
2. `Integrations` tab を開く。
3. `Add apps` から対象 App(この help の例では `slapex`)を追加する。

![private channel の Integrations tab から App を追加する](../../assets/screenshots/slack-app-setup/10-invite-private-channel.png)

## Channel access

user token を使う場合、認可したユーザー本人が参照できる範囲がアクセス範囲になる。対象 channel が見えない場合は、そのユーザーが対象 channel を参照できるか、必要 scope が付いているかを確認する。

bot token を使う場合、public channel / private channel の投稿を取得するには、対応する scope だけでは不十分である。取得対象 channel に bot / app が参加している必要がある。参加していない場合は「bot / app を channel に参加させる」の手順で追加する。

## Token の渡し方

ローカルの `.env` や shell history に token の実値を残さない。

Slack OAuth token は `SLACK_TOKEN` として渡す。

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

1Password CLI、CI secrets、secret manager 未利用時の一時注入など、用途別の詳しい手順は [`token-injection.md`](token-injection.md) を参照する。

## よくあるエラー

### `SLACK_TOKEN` が未設定

`SLACK_TOKEN` を環境変数として渡してから再実行する。

### token が無効

secret manager または CI secrets に保存した token が正しいか確認する。App を uninstall した場合や scope を変更した場合は、再 install / 再 authorize して token を更新する。

### scope が不足している

OAuth & Permissions で不足 scope を追加し、App を workspace に再 install / 再 authorize する(手順は「scope 変更後の再 install」を参照)。更新後の token を secret manager または CI secrets に反映する。

### channel が見えない

channel 名または channel ID を確認する。user token の場合は、認可したユーザーが対象 channel を参照できるか確認する。bot token の場合は、bot / app がその channel に参加しているか確認する。

### bot token 利用時に bot が channel に参加していない

「bot / app を channel に参加させる」の手順で追加する。public channel は `/invite @<App 名>`、private channel はその channel の参加者が `Integrations` tab から追加する。

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
