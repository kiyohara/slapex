# 0040 Credential Scope for Asset Downloads

- 状態: decided
- 作成日: 2026-06-22
- 最終更新日: 2026-06-22
- 関連: `../slack-api-usage.md`, `../../guidelines/credential-scope-guidelines.md`

## 背景

URL preview の service icon 保存対応後の E2E で、外部 service icon URL の取得時に HTTP 400 が発生した。調査したところ、Slack private file の取得に必要な `Authorization: Bearer` header を、第三者 host の public asset URL にも送っていた。

これは取得失敗だけでなく、本来 Slack にだけ送るべき認証情報を外部 host へ送信し得る security issue である。

## 候補

- 全 asset download に従来どおり Authorization header を付ける。
- asset kind ごとに Authorization header の有無を分ける。
- download URL の host allowlist で Authorization header の有無を分ける。
- private file download と public asset download の API を完全に分ける。
- 第三者 host 由来の public display asset に固定の guard size limit を設ける。

## 検討内容

全 asset download へ Authorization header を付ける方式は、実装は単純だが認証情報の送信先スコープが広すぎる。URL preview 画像、service icon、avatar、emoji は第三者 host や Slack CDN など public asset URL になり得るため、Slack bot token を送るべきではない。

asset kind ごとの分岐は、呼び出し側の分類漏れで再発しやすい。実際に保護が必要なのは Slack private file URL であり、判定軸は kind より送信先 host の方が直接的である。

private file download と public asset download の API 分離はより強い設計だが、現時点の変更量に対して大きい。まずは download の認証付与条件を 1 箇所に集約し、host allowlist とテストで守る。

認証情報の送信先スコープとは別に、URL preview 画像や service icon は第三者 host 由来の public asset URL になり得る。これらを無制限に download すると、悪意ある、または巨大な response により download 量や disk 使用量が過大になる。ユーザー添付ファイルとは性質が異なるため `--max-attachment-size` ではなく、表示用 public preview asset の保護上限として固定 guard limit を設ける。

## 決定

asset download で Slack bot token を送るのは Slack private file URL (`files.slack.com`) のみに限定する。

Slack Web API (`slack.com/api`) への request は従来どおり Authorization header を付ける。URL preview 画像、URL preview service icon、avatar、emoji などの public asset URL には Authorization header を付けない。

URL preview 画像と URL preview service icon には 1 件あたり 5MiB の guard limit を設け、上限を超える場合は保存せず manifest に `skipped_size` として記録する。

## 理由

認証情報は default deny で扱い、必要な送信先だけを allowlist する方が安全である。host allowlist による判定は、URL の用途分類よりも認証情報の送信可否を直接表現できる。

## 影響

- `internal/slack.Client.Download` は `files.slack.com` の場合だけ Authorization header を付ける。
- unit test で Slack private file URL には Authorization header が付くこと、public asset URL には付かないことを確認する。
- unit test で URL preview 画像と URL preview service icon に固定 guard limit が適用されることを確認する。
- integration test の public service icon fixture は Authorization header が来た場合に失敗する。
- AI agent 向け共通 rule として `doc/guidelines/credential-scope-guidelines.md` を追加し、Cursor / Claude / Codex / Copilot の入口にも反映する。

## 後から見直す条件

- Slack private file の host が追加または変更された場合。
- ツール自身による Open Graph fetch など、第三者 host への新しい外部通信を追加する場合。
- asset download API を private file 用と public asset 用に分離する必要が出た場合。
