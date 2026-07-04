# 作業ブランチメモ

- ブランチ: slack-app-setup-screenshots
- PR: -
- 最終更新: 2026-07-04

## 目的

Issue #48 の対応。`doc/help/slack-app-setup.md` に Slack 管理画面 / Slack client のスクリーンショット付き UI 操作手順を追加し、bot / app の channel 招待節を独立させ、スクリーンショットのメンテ方針を `doc/help/README.md` に追記する。

## 現在の状況

- 文書再構成(番号付き手順・画像参照・channel 招待節の独立・メンテ方針追記)まで完了。
- スクリーンショット実体は未撮影。撮影はユーザーが行う分担(下の撮影指示リスト参照)。画像が揃うまで help 内の画像リンクは意図的に broken のまま。
- PR は画像組み込み・検証完了後に作成する。

## 決定事項

- **分担(案 A)**: Slack へのログイン・実 workspace 操作を伴う撮影はユーザーが実施し、agent は文書構成・撮影指示リスト・画像の組み込み(必要ならトリミング / マスク / 番号マーカー焼き込み)・PR 作成を行う。agent 単独では実 Slack アカウントが必要なため完遂不可(#114 / #116 の自己生成アセットとは性質が異なる)と判断した。
- **画像配置**: `assets/screenshots/slack-app-setup/` に置く。README 用プレビュー画像(`sample-*.png`)と同じ `assets/screenshots/` 配下でサブディレクトリを分ける。`assets/README.md` に説明を追記。
- **メンテ方針**: スクリーンショットは補助でありテキスト手順を正とする。Slack UI 変更時は手順テキストの正しさを優先確認し、画像は利用者が迷う乖離が出た時点で同名ファイル差し替え。`doc/help/README.md` に 1 段落で記載。
- **番号付きクリック明示**: 本文の番号付き手順と画像を対応させる。必要に応じて受領画像へ番号マーカーを焼き込む(受領後に判断)。

## 次にやること

1. ユーザーから撮影画像を受領し、`assets/screenshots/slack-app-setup/` に配置(必要ならトリミング・マスク・番号マーカー)。
2. 画像込みで help の表示確認(パス・alt・GitHub 上での見え方)。
3. Issue の検証観点を実施し、本 note の検証セクションへ記録。
4. `progress.md` の help-02 行を更新し、PR 作成(`Closes #48`)、note rename。

## 撮影指示リスト

共通条件:

- ブラウザ幅 1280px 程度、ライトモード、PNG。api.slack.com は英語 UI のままでよい(本文の UI ラベルは英語表記)。
- App 名は manifest どおり `slapex` を推奨。
- 撮影にはテスト用 workspace を推奨。workspace 名・個人名・アイコン等の映り込みはマスク可能(受領後に agent が対応可)。
- **token が映るショット(07 / 08)は、共有前にマスクするか、撮影後に token を revoke / regenerate する。**

| # | ファイル名 | 画面 / URL | 撮りたい状態 |
|---|---|---|---|
| 01 | `01-create-new-app.png` | <https://api.slack.com/apps?new_app=1> | `Create an app` ダイアログで `From a manifest` を選ぶ直前(両選択肢が見える状態) |
| 02 | `02-pick-workspace.png` | 同上の次ステップ | workspace 選択ドロップダウンを開いた状態 |
| 03 | `03-paste-manifest.png` | manifest editor | `JSON` tab が選択され、help 記載の user token 用 manifest を貼り付けた状態 |
| 04 | `04-review-summary.png` | Review summary | scope 一覧が表示され `Create` ボタンが見える状態 |
| 05 | `05-install-to-workspace.png` | App 管理画面 `OAuth & Permissions` | `Install to Workspace` ボタンが見える状態(install 前) |
| 06 | `06-authorize.png` | OAuth 認可画面 | 権限一覧と `Allow` ボタンが見える状態 |
| 07 | `07-user-oauth-token.png` | `OAuth & Permissions` の `OAuth Tokens` | install 後、`User OAuth Token`(`xoxp-`)と `Copy` ボタンが見える状態(token はマスク前提) |
| 08 | `08-bot-oauth-token.png` | 同上(bot token 用 App) | `Bot User OAuth Token`(`xoxb-`)と `Copy` ボタンが見える状態(token はマスク前提) |
| 09 | `09-invite-public-channel.png` | Slack client の public channel | 入力欄に `/invite @slapex` を入力した状態(送信直前または招待完了のシステムメッセージ) |
| 10 | `10-invite-private-channel.png` | Slack client の private channel 設定 | channel 設定 → `Integrations` tab → `Add apps` で App を追加する画面 |
| 11 | `11-reinstall-banner.png` | `OAuth & Permissions` | scope 変更後に画面上部へ出る再 install を促す banner が見える状態 |

08 / 09 / 10 は bot token 用 App(bot user 有効)が必要。07 のみなら user token 用 App で足りる。

## 検証

- (未実施)画像受領後に記録する。

## リスク・ブロッカー

- スクリーンショット受領待ち(ユーザー撮影)。受領まで help の画像リンクは broken。
- Slack UI は変更されうるため、受領画像と本文ラベルの食い違いがあれば本文側を実画面に合わせて微修正する。

## セッションログ

- 2026-07-04: agent 単独での完遂可否を判断(不可)。ユーザーと分担案 A で合意。ブランチ作成、help 再構成、メンテ方針追記、撮影指示リスト作成。
