# 作業ブランチメモ

- ブランチ: poc-additional-e2e
- PR: (未採番)
- 最終更新: 2026-06-11

## 目的

PoC(PR #11)の追加 E2E 検証。テスト投稿ベースだった E2E に対し、実運用コンテンツを持つ channel(`#meetup165` / `#meetup166`)でバリエーションを押さえる(ユーザー依頼)。

## 現在の状況

- 両 channel の export を完了(exit 0)。コード変更は不要だった(発見されたバグなし)。
- 本ブランチの変更は検証記録(本 note、progress、decision log index の未決事項 1 行)のみ。

## 決定事項

- 検証のみのためコード・仕様の変更なし。軽微な忠実度の所見 1 件を未決事項として記録(下記)。

## 検証

実 token E2E(検証用 workspace、token は 1Password secret reference 経由で実行時のみ注入):

- `#meetup165`(`--days 90`): timeline 25 件、ユーザー解決 8 名、assets 15 件保存・失敗 0(avatar 8 / og-image 5 / カスタム絵文字 2)。
- `#meetup166`(`--days 90`): timeline 4 件、ユーザー解決 2 名、assets 2 件保存・失敗 0。

今回新たに実データで確認できた経路(PR #11 時点で未確認だったもの):

- **bot 未参加 channel**: Target 表示が `not member` となり、`conversations.history` の `not_in_channel` で **exit 3 + help URL 案内**(招待後の再実行で成功)。
- **system 行**: `channel_join` / `channel_topic` が avatar なしの 1 行表示で出力され、join の mention は表示名へ解決される(bot ユーザーの join も解決)。
- **複数ユーザー**: 8 名の `users.info` 解決と avatar 保存・表示。
- **複数日にまたがる timeline**: date divider が日付ごとに挿入される(165 で 3 日分)。
- **実運用の unfurl**: 9 件の unfurl 表示と og-image 5 件のローカル保存。
- **ローカル参照の整合**: 両 export の `index.html` 内 `src` / `href` のローカル参照が全件実ファイルに解決することを機械確認(リンク切れなし)。
- keyword 完全一致での channel 解決(`meetup165` / `meetup166`)。

所見:

- `channel_topic` の system 行は、Slack API の `text` に actor の mention が含まれない場合があり、その場合「誰が topic を変更したか」が表示されない。Slack UI 同等にするには `user` field から author prefix を補完する必要がある。軽微なため将来検討として decision log index の未決事項に記録(関連: 0027)。

本ラウンド後も未確認のまま残る項目(実装あり・検証データに含まれず): fenced code block、`--max-attachment-size` 超過の置換表示、tombstone、429 待機表示、TTY interactive 選択、`--reuse-cache`(PoC 未実装)。

## 次にやること

- PR 作成、採番後に note rename、自己マージ。

## リスク・ブロッカー

- なし(検証のみ)。

## セッションログ

- 2026-06-11: ユーザー依頼で `poc-additional-e2e` ブランチを作成。`#meetup165` 初回実行で bot 未参加 → exit 3 を確認。ユーザーが両 channel へ bot を招待後、両 export 成功。構造カウント・ローカル参照整合・system 行レンダリングを点検し、所見 1 件(channel_topic の actor 表示)を記録。
