# 認証情報の送信先スコープルール

この文書は、slapex リポジトリで token / cookie / Authorization header などの認証情報を扱う実装・レビュー時の共通正本である。

## 基本方針

- 認証情報は default deny で扱う。必要性が明確な送信先にだけ付与する。
- 送信先は scheme ではなく host で判定し、明示 allowlist に一致した場合だけ認証情報を送る。
- public asset、URL preview、avatar、emoji、外部 URL fetch など、第三者 host へ向かう可能性がある通信には認証情報を送らない。
- 認証情報を持つ HTTP client / downloader を汎用化する場合は、認証情報付与の条件を 1 箇所に集約し、negative test を追加する。

## 実装 checklist

外部通信や asset download を実装・変更するときは、少なくとも次を確認する。

1. 認証情報を付ける host が明示されている。
2. allowlist 外 host へ認証情報が送られない unit test がある。
3. 認証情報が必要な host へは引き続き送られる positive test がある。
4. fake server / integration test で、public asset endpoint が Authorization header を拒否する経路を少なくとも 1 つ持つ。
5. log、cache、manifest、working branch note、PR description に token 実値や署名付き URL を残さない。

## slapex での現在の方針

- Slack Web API (`slack.com/api`) への request には Slack OAuth token を `Authorization: Bearer` で送る。
- Slack private file URL (`files.slack.com`) への download には Slack OAuth token を送る。
- URL preview 画像、URL preview service icon、avatar、emoji などの public asset URL には Slack OAuth token を送らない。
- ツール自身による Open Graph fetch は初期対象外であり、将来追加する場合も同じ default deny 方針に従う。

## 実 token E2E の確認と記録

リリース前や token 周りの変更時に実 token E2E を行う場合は、実 token の値を記録しない形で次を確認する。

| token type | 確認内容 |
|---|---|
| user token | public channel、参加済み private channel、thread replies、file download、emoji、user 解決 |
| bot token | bot / app 参加済み public channel、bot / app 参加済み private channel、thread replies、file download、emoji、user 解決 |

確認結果を PR や working branch note に残す場合は、token 実値、workspace 固有の非公開情報、channel 固有の非公開情報を書かない。必要な文脈は抽象化し、成功 / 失敗、未確認項目、再現に必要な公開可能な条件だけを記録する。

## レビュー観点

- `Authorization` / `Cookie` / `X-API-Key` などの header 追加は security-sensitive として扱う。
- host 判定なしに共通 HTTP client へ認証情報を持たせる変更は原則として差し戻す。
- 認証情報付与の条件を広げる変更では、allowlist 外へ送られないことを示すテストが無ければ `[must]` 相当の指摘対象とする。

## 関連

- `doc/design/slack-api-usage.md`
- `doc/design/decision-log/0040-credential-scope-for-asset-downloads.md`

## Copilot Review 同期メモ

- この正本の実装レビュー向け要点は `.github/instructions/credential-scope.instructions.md` に抜粋している。
- 認証情報の送信先スコープ方針を変更した場合は、同 instruction file の同期要否を確認する。
