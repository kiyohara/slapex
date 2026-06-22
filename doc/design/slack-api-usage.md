# Slack API 利用方針

このファイルには、`slapex` が使う Slack API method、pagination、rate limit 対応、user / emoji / file の解決方針をまとめる。

想定読者は、取得処理を実装・検証する担当者である。

本ファイルの方針は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

利用者の操作の流れは `usage-flow.md`、取得範囲と保存対象は `output-format.md`、cache の扱いは `cache.md` を参照する。決定経緯は `decision-log/0025-slack-api-usage-policy.md` と `decision-log/0040-credential-scope-for-asset-downloads.md` を参照する。

## 前提とする token と App

- 単一 workspace install の bot token(`xoxb-`)を前提とする(`decision-log/0003-workspace-selection.md`)。
- App は利用者自身が作成・install する internal App とする(`decision-log/0009-user-managed-slack-app.md`)。
- 2025-05 に発表された非 Marketplace「配布」アプリ向けの rate limit 強化(`conversations.history` / `conversations.replies` が 1 req/min、最大 15 件/req)は、internal App は対象外であることが公式に明言されている。slapex の「利用者自身が App を作成する」前提は、この点でも妥当である。
- ただし将来の方針変更に備え、実装は「低い rate limit・小さい page size しか許されない環境でも、時間をかければ完走できる」ことを設計条件とする。具体的には、page size をサーバー側が縮小しても cursor 継続で正しく動き、429 応答には待機で追従する。

出典:

- [Rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/)
- [Clarifying rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/06/03/rate-limits-clarity/)
- [Rate limits](https://docs.slack.dev/apis/web-api/rate-limits/)

## 使用する API

| method / 経路 | 用途 | 呼び出しタイミング |
|---|---|---|
| `auth.test` | token 検証、workspace(`team_id`、名前、URL)の解決 | 起動時に 1 回 |
| `conversations.list` | channel 一覧の取得と keyword 解決 | channel 確定まで(pagination) |
| `conversations.history` | timeline 上の親投稿の取得 | 取得範囲制限に達するまで(pagination) |
| `conversations.replies` | thread replies の取得 | thread を持つ親投稿ごと(pagination) |
| `users.info` | 投稿者・mention の表示名解決 | unique な user ID ごとに 1 回 |
| `emoji.list` | カスタム絵文字 URL の取得 | 1 回 |
| HTTP GET(`url_private_download`) | 添付ファイル・画像の download | 保存対象 asset ごと |

- `files.info` は原則呼ばない。message 内の file object に必要な metadata(size、mimetype、thumbnail / original URL)が含まれるためである。file object に必要情報が欠けている場合だけ補完として呼ぶ。
- `users.list` による一括解決は採用しない。大規模 workspace で過剰取得になるためである。多人数 channel で `users.info` の呼び出し回数が問題になる場合の最適化(閾値での `users.list` 切り替えなど)は将来検討とする。

## pagination

- cursor ベースの pagination を使い、`response_metadata.next_cursor` が空になるまで辿る。
- `conversations.history` は `oldest` に「実行開始時刻 − `--days` × 24 時間」を指定し、1 ページの要求件数は 200 件を上限とする。サーバー側がより小さいページ(例: 15 件)しか返さなくても、cursor 継続により動作が変わらない設計とする。
- timeline 上の親投稿件数が `--max-posts` に達した時点で打ち切る。`--max-posts` が数える対象は `conversations.history` が返す timeline メッセージであり、`conversations.replies` だけに現れる thread replies は数えない(thread_broadcast は timeline に現れるため数える)。
- `conversations.replies` も同様に pagination し、1 thread あたり合計 1000 件で打ち切る(`output-format.md`)。

## rate limit とリトライ

- 一次情報は HTTP 429 と `Retry-After` ヘッダとする。`Retry-After` の指定秒数に小さな jitter を加えて待機し、再試行する。
- `Retry-After` の無い 429、一時的な 5xx、ネットワークエラーは指数バックオフ(初回 1 秒、上限 60 秒、jitter 付き)で再試行する。
- 同一リクエストの再試行は最大 5 回とする。超過した場合、メッセージ取得系は exit code `4` で失敗し、個別 asset は失敗として記録して継続する(`cli-interface.md`)。
- 通常時も同一 method の呼び出しは 1 req/sec を目安に自主的に平準化する(公式推奨に従う)。
- rate limit 待機中は、待機理由とおおよその待機時間を進捗表示する(`usage-flow.md` の「処理対象の表示」と同じく stderr)。

## user 解決

- 取得済みメッセージの投稿者と本文中の mention に現れる unique な user ID を集め、`users.info` で表示名を解決する。
- 解決結果は `.cache/slack_api_cache.json` に蓄積し、同一実行内で再問い合わせしない。
- 表示名は display name を優先し、無ければ real name、それも無ければ user ID へ fallback する。
- bot 投稿(`bot_message` subtype や `bot_id` 付き)は `bot_profile` / `username` を優先して表示名にする。
- 解決失敗(退会ユーザーなど)は user ID をそのまま表示する。

## emoji 解決

- カスタム絵文字は `emoji.list` を 1 回取得する。`alias:<name>` 形式の alias は再帰的に解決し、循環を検出したら打ち切る。
- 標準絵文字の shortcode は、実装に組み込む標準絵文字データセットで Unicode 文字へ変換し、HTML に直接出力する(`output-format.md`)。
- データセットにもカスタム絵文字にも無い shortcode は、`:shortcode:` の文字列のまま表示する。
- reaction の絵文字名も同じ経路で解決する。skin tone variation は base 絵文字に寄せ、合成表示は将来検討とする。

## file / asset の取得

- message の `files` 配列を情報源とし、`url_private_download`(無ければ `url_private`)へ `Authorization: Bearer` ヘッダ付きの HTTP GET で取得する。
- URL preview 画像、URL preview service icon、avatar、emoji など、Slack private file ではない public asset URL へは `Authorization: Bearer` ヘッダを送らない。
- 画像は表示用 thumbnail と original の両方を保存する(`decision-log/0017-uploaded-image-assets.md`)。
- file object の `size` が `--max-attachment-size` を超えるものは download しない(`output-format.md`)。
- 外部サービス連携ファイル(external file)など download URL を持たないものは、リンクのみの添付として扱い、manifest に記録する。
- asset の download にも上記のリトライ方針を適用する。失敗した asset は HTML 上で置換表示にし、export 全体は継続する。

## 取得の整合性

- export は単発実行であり、実行中に Slack 側で起きた更新との完全な整合は保証しない。取得開始時点のスナップショットとして扱う。
- API の返却順には依存せず、レンダリング時に `ts` で昇順(oldest → latest)に整列する。
- timeline と thread の両方に現れるメッセージ(thread_broadcast)は、Slack の表示と同様に両方へ表示する(`html-rendering.md`)。

## 参考

- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test)
- Slack Developer Docs: [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list)
- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history)
- Slack Developer Docs: [`conversations.replies`](https://docs.slack.dev/reference/methods/conversations.replies/)
- Slack Developer Docs: [`users.info`](https://docs.slack.dev/reference/methods/users.info)
- Slack Developer Docs: [`emoji.list`](https://docs.slack.dev/reference/methods/emoji.list)
- Slack Developer Docs: [Rate limits](https://docs.slack.dev/apis/web-api/rate-limits/)
