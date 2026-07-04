# 作業ブランチメモ

- ブランチ: slack-app-setup-screenshots
- PR: -
- 最終更新: 2026-07-04

## 目的

Issue #48 の対応。`doc/help/slack-app-setup.md` に Slack 管理画面 / Slack client のスクリーンショット付き UI 操作手順を追加し、bot / app の channel 招待節を独立させ、スクリーンショットのメンテ方針を `doc/help/README.md` に追記する。

## 現在の状況

- 文書再構成(番号付き手順・画像参照・channel 招待節の独立・メンテ方針追記)完了。
- 01〜08 の画像を受領・調整・組み込み済み(下の「受領済み画像と調整内容」参照)。06 の workspace 名・icon も追加でダミー化済み。撮影に使った token はユーザーが revoke 済み。
- channel 招待手順を `/invite` に一本化(決定事項参照)。撮影残は 09(`/invite` 実行)と 10(再 install banner)の 2 枚のみで、この 2 つの画像リンクのみ意図的に broken のまま。
- PR は 09〜10 組み込み・検証完了後に作成する。

## 決定事項

- **分担(案 A)**: Slack へのログイン・実 workspace 操作を伴う撮影はユーザーが実施し、agent は文書構成・撮影指示リスト・画像の組み込み(必要ならトリミング / マスク / 番号マーカー焼き込み)・PR 作成を行う。agent 単独では実 Slack アカウントが必要なため完遂不可(#114 / #116 の自己生成アセットとは性質が異なる)と判断した。
- **画像配置**: `assets/screenshots/slack-app-setup/` に置く。README 用プレビュー画像(`sample-*.png`)と同じ `assets/screenshots/` 配下でサブディレクトリを分ける。`assets/README.md` に説明を追記。
- **メンテ方針**: スクリーンショットは補助でありテキスト手順を正とする。Slack UI 変更時は手順テキストの正しさを優先確認し、画像は利用者が迷う乖離が出た時点で同名ファイル差し替え。`doc/help/README.md` に 1 段落で記載。
- **番号付きクリック明示**: 本文の番号付き手順と画像を対応させる。必要に応じて受領画像へ番号マーカーを焼き込む(受領後に判断)。
- **ファイル命名規約**: `<連番>-<画面内容>[-user|-bot].png`。番号 = 手順の登場順、user token / bot token で画面が異なるものは `-user` / `-bot` suffix で区別、共通画面(01 / 02、09〜11 予定)は suffix なし。当初の `07-user-oauth-token.png` / `08-bot-oauth-token.png` は suffix 規約に合わせ `07-oauth-token-user.png` / `08-oauth-token-bot.png` へ改名。
- **bot token 節の画像**: 撮影で user / bot の画面差(manifest 内容・Bot Scopes・認可権限一覧)が判明したため、「同じ流れ」参照方式をやめ、bot 節にも 03〜06 の bot 版と 08 を番号付き手順で埋め込む構成に変更。
- **マスク処理の方式**: workspace 名は install / reinstall ボタンごとダミー(`Install to myworkspace` / `Reinstall to myworkspace`)を同色・同サイズで再描画して置換。token 実値は入力欄内を背景色で塗りつぶし、`xoxp-***...` / `xoxb-***...` のダミー文字列を描画。ImageMagick(host 常設)で実施。加工前の原本は repo 外(`/tmp/slack-app-setup-originals/`)に退避。
- **channel 招待手順の一本化**: Issue 提案では public(`/invite`)/ private(Integrations tab)の分岐記載だったが、ユーザーの実機確認で private channel でも `/invite @slapex` が有効に動作することを確認。Slack 公式 help(Guide to apps in Slack)や Slack 自身の app help でも、channel への app 追加手段として mention 経由・`/invite`・Integrations tab が並列に案内されており、private 限定の手段ではない(private の制約は「その channel の member が実行する」ことのみ)。利用者に最短手順を示すため `/invite` に一本化し、mention / Integrations tab は代替手段として箇条書きで残す構成に変更した。これに伴い撮影枚数が 1 枚減り、09 = `/invite` 実行、10 = 再 install banner(旧 11)へ再採番。

## 次にやること

1. ユーザーから 09〜10 の画像を受領し、調整(必要ならマスク・高さカット)して配置。
2. 画像込みで help の表示確認(パス・alt・GitHub 上での見え方)。
3. Issue の検証観点を実施し、本 note の検証セクションへ記録。
4. `progress.md` の help-02 行を更新し、PR 作成(`Closes #48`)、note rename。

## 受領済み画像と調整内容(01〜08)

- `01-create-new-app.png` / `02-pick-workspace.png`: 受領そのまま採用(機微情報なし)。
- `03-paste-manifest-user/-bot.png`、`04-review-summary-user/-bot.png`: 受領そのまま採用。user / bot で manifest 内容と scope 見出し(`User Scopes` / `Bot Scopes`)が異なるため両方使用。
- `06-authorize-user/-bot.png`: 本文中とドロップダウンの実 workspace 名をダミー(`myworkspace.` / `myworkspace`)に置換し、workspace icon を無地の丸 + `m` に置換。
- `05-install-to-workspace-user/-bot.png`: 全画面キャプチャ(高さ 6312px)を `OAuth Tokens` セクションまで(高さ 1840px)にカット。`Install to <実 workspace 名>` ボタンをダミー(`Install to myworkspace`)に置換。
- `07-oauth-token-user.png` / `08-oauth-token-bot.png`(旧 `07-user-oauth-token.png` / `08-bot-oauth-token.png`): 高さ 6468px → 1985px にカット。token 実値をダミー(`xoxp-***...` / `xoxb-***...`)に置換。`Reinstall to <実 workspace 名>` ボタンをダミーに置換。
- 未加工の原本は `/tmp/slack-app-setup-originals/` に退避(repo には入れない)。

## 残りの撮影指示リスト(09〜10)

共通条件: ブラウザ幅 1280px 程度、ライトモード、PNG。09 は bot token 用 App(bot user 有効)を install 済みの workspace が必要。

| # | ファイル名 | 画面 / URL | 撮りたい状態 |
|---|---|---|---|
| 09 | `09-invite-app-to-channel.png` | Slack client の channel | 入力欄に `/invite @slapex` を入力した状態(送信直前または招待完了のシステムメッセージ)。channel 名が映る場合はダミー名の channel を推奨 |
| 10 | `10-reinstall-banner.png` | `OAuth & Permissions` | scope 変更後に画面上部へ出る再 install を促す banner が見える状態 |

## 検証

- 2026-07-04: help 内の画像参照 15 件のうち 01〜08 の 12 件が実ファイルに解決することを確認(09〜11 の 3 件は撮影待ちで意図的に未解決)。
- 2026-07-04: 07 / 08 の加工後画像を目視確認し、token 実値が残っていないこと(マスク文字列のみ)、workspace 実名がボタンから消えていることを確認。
- 2026-07-04: 06 の加工後画像を目視確認し、workspace 実名と icon が本文・ドロップダウンから消えていることを確認。撮影に使った token はユーザーが revoke 済み。
- 2026-07-04: private channel でも `/invite @slapex` で App を追加できることをユーザーが実機確認(手順一本化の根拠)。

## リスク・ブロッカー

- 09〜10 のスクリーンショット受領待ち(ユーザー撮影)。受領までこの 2 件の画像リンクは broken。
- Slack UI は変更されうるため、画像と本文ラベルの食い違いがあれば本文側を実画面に合わせて微修正する。

## セッションログ

- 2026-07-04: agent 単独での完遂可否を判断(不可)。ユーザーと分担案 A で合意。ブランチ作成、help 再構成、メンテ方針追記、撮影指示リスト作成。
- 2026-07-04: 01〜08 受領。user / bot の画面差に合わせ命名規約を `-user` / `-bot` suffix に統一し、bot 節へ bot 版画像を埋め込み。05 / 07 / 08 に高さカット・workspace 名ダミー化・token マスクを実施して配置。
- 2026-07-04: 06 の workspace 名・icon を追加でダミー化。private channel でも `/invite` が有効というユーザー実機確認を受けて事実関係を調査し、channel 招待手順を `/invite` に一本化(Issue 提案の public / private 分岐から変更)。撮影リストを 09〜10 の 2 枚に再編。
