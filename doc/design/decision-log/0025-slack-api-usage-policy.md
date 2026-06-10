# 0025 Slack API 利用方針(rate limit / pagination / 解決系)

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/slack-api-usage.md`, `doc/design/output-format.md`, `doc/design/decision-log/0009-user-managed-slack-app.md`

## 背景

取得処理の実装に必要な「どの API をどう呼ぶか」(pagination、rate limit 対応、user / emoji / file の解決手順)が未確定だった。また、2025-05 に Slack が非 Marketplace アプリ向けの rate limit 強化を発表しており、slapex の前提(利用者自身が App を作成する)への影響を確認する必要があった。

## 候補

- rate limit 対応: (A) 429 + `Retry-After` 遵守と指数バックオフ。(B) method ごとの公表 Tier に合わせた事前スロットリングのみ。
- user 解決: (A) メッセージに現れた unique user ID を `users.info` で個別解決。(B) `users.list` で一括取得。
- file metadata: (A) message 内 file object を正とし `files.info` は補完のみ。(B) file ごとに `files.info` を呼ぶ。

## 検討内容

- 2025-05-29 発表の rate limit 強化(`conversations.history` / `conversations.replies` を 1 req/min、最大 15 件/req に制限)は、商用配布される非 Marketplace アプリが対象。2025-06-03 の公式 clarification で「internal customer-built apps は既存 rate limit を維持し、新制限の対象外」と明言されている。slapex の前提である利用者自作の internal App(0009)は対象外であり、0009 の方針はこの点でも裏づけられた。
  - 出典: [Rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/), [Clarifying rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/06/03/rate-limits-clarity/)
- ただし Slack はこの 1 年で制限方針を動かしており、将来 internal App の扱いが変わるリスクはゼロではない。「小さい page size・低い rate limit でも cursor 継続と待機で完走できる」実装は、この変動に対する保険になる。
- 事前スロットリングだけに頼る案 (B) は、公表 Tier が method ページごとにしか示されず、burst の正確な閾値も非公開のため、結局 429 対応が必須になる。429 + `Retry-After` を一次情報とし、自主的な平準化(1 req/sec 目安)を併用するのが公式ドキュメントの推奨とも一致する。
- `users.list` 一括取得は大規模 workspace で channel に無関係な大量のユーザーを取得する。export 対象 channel に現れる unique user は通常限られるため、`users.info` 個別解決 + 実行内キャッシュが過剰取得を避けられる。多人数 channel での呼び出し回数最適化は将来検討とする。
- message 内 file object には size / mimetype / thumbnail / original URL が含まれるため、`files.info` の常用は不要な API 消費になる。

## 決定

- 使用 API を `auth.test` / `conversations.list` / `conversations.history` / `conversations.replies` / `users.info` / `emoji.list` / `url_private_download` への HTTP GET に確定した。
- pagination は cursor ベース、history は `oldest` で `--days` 境界を適用、1 ページ要求は 200 件上限、サーバー側の page 縮小に依存しない設計とする。
- rate limit は 429 + `Retry-After` 遵守(jitter 付き)を一次対応とし、`Retry-After` 無しの 429 / 一時的 5xx / ネットワークエラーは指数バックオフ(初回 1s、上限 60s)、同一リクエスト最大 5 回で打ち切り。通常時は 1 req/sec 目安の平準化を行う。
- user 解決は `users.info` 個別解決 + 実行内キャッシュ。bot 投稿は `bot_profile` / `username` を優先。
- emoji は `emoji.list` 1 回 + alias 再帰解決。標準絵文字は組込みデータセットで Unicode 化。
- file は message 内 file object を正とし、`files.info` は欠損時の補完のみ。

## 理由

- 429 / `Retry-After` は Slack が外部に保証している唯一の機械可読な制限情報であり、これへの追従が最も頑健。
- internal App が新制限の対象外であることを確認したため、`--max-posts` default `1000` / `--days` default `30` の既存仕様(0011)は現実的な実行時間で成立する。
- 取得 API と解決手順を確定しないと、アーキテクチャ選定(HTTP client / SDK の要件)と PoC の検証項目が定義できない。

## 影響

- `slack-api-usage.md` を新設し、取得処理の仕様の正本とする。
- アーキテクチャ選定では、各言語の Slack SDK または素の HTTP client で本方針(cursor pagination、429 対応、Bearer 付き download)が素直に実装できるかを評価軸にする。
- PoC では rate limit 待機表示と pagination の打ち切り条件を確認対象にする。

## 後から見直す条件

- Slack が internal App にも rate limit 強化を適用する方針変更を行った場合(その場合は default 値の再検討も必要)。
- 多人数 channel で `users.info` の呼び出し回数が実行時間の支配項になった場合。
- `blocks` の完全レンダリング対応などで必要 API が増えた場合。
