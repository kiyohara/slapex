# 0035 avatar 画像の保存対象化

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/output-format.md`, `doc/design/html-rendering.md`

## 背景

`html-rendering.md` は「Slack default 風の avatar、投稿者名、絶対時刻、本文…を CSS で整え」と avatar の表示を前提にしている。一方、`output-format.md` の保存 assets 一覧に avatar 画像は含まれておらず、PoC 実装でこのギャップが顕在化した。ローカル HTML は外部 URL に依存しない方針のため、avatar を表示するなら画像の保存が必要になる。

## 候補

- avatar 画像を保存 assets に加える(`users.info` の profile image URL から取得)。
- avatar 画像は保存せず、イニシャル文字の placeholder だけで表示する。
- リモートの profile image URL を直接参照する。

## 検討内容

- リモート直接参照は「外部 URL へ依存せず閲覧できる」方針に反するため除外。
- イニシャル placeholder のみは実装が最小だが、「Slack default の投稿表示を模倣」(0012)から見た再現度が下がる。
- avatar 画像の保存は、登場する投稿者数ぶん(チャンネル規模でも通常は数十件)の小さな画像で済み、既存の asset 保存経路(URL hash 名 + manifest 記録)をそのまま使える。取得失敗時はイニシャル表示に fallback すれば、保存失敗が export 全体を妨げない。

## 決定

- 投稿・thread replies に登場する投稿者の avatar 画像(`users.info` の profile image URL、72px 優先)を保存 assets に加え、`assets/avatars/` に置く。
- manifest の asset 種別に `avatar` を追加する。
- 取得できない場合(bot 投稿で profile が無い場合を含む)は、イニシャル文字の placeholder 表示に fallback する。
- avatar 画像にはサイズ上限(`--max-attachment-size`)を適用しない(thumbnail / 絵文字と同じ扱い)。

## 理由

- 表示仕様(0012)と保存方針(外部 URL 非依存)の両方を満たす自然な解で、追加コストが小さいため。

## 影響

- `output-format.md` の保存 assets 一覧と出力ディレクトリ構造に `avatars/` を追記した。
- `.cache/assets_manifest.json` の `kind` に `avatar` が増える(`cache.md` の schema は種別を列挙しているため、`cache.md` 側も本 PR で追従)。
- PoC 実装は本決定を反映済み。

## 追記(2026-09-03)

「取得できない場合(bot 投稿で profile が無い場合を含む)はイニシャル fallback」は、bot 投稿については `bots.info` でも icon を取れない場合に限定される。`bot_profile` が無い bot 投稿は `bot_id` から `bots.info` で app icon を解決し、同じ `assets/avatars/` に保存する(`0054-bot-author-resolution.md`)。

## 後から見直す条件

- avatar の取得が export 時間や失敗率の面で問題になった場合。
- プライバシー配慮などで avatar を含めない出力 option の需要が出た場合。
