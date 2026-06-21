---
applyTo: "internal/**/*.go"
---

# Credential Scope Review

この repository の Go 実装をレビューするときは、認証情報の送信先スコープを security-sensitive として確認する。

- `Authorization` / `Cookie` / `X-API-Key` などの header を追加・変更するコードは重点確認する。
- 認証情報は default deny。明示 allowlist の host に一致する場合だけ送る。
- host 判定なしに共通 HTTP client / downloader へ認証情報を持たせる変更は `[must]` で指摘する。
- Slack bot token は Slack Web API と Slack private file URL (`files.slack.com`) にだけ送る。
- URL preview 画像、URL preview service icon、avatar、emoji などの public asset URL へ Slack bot token を送る変更は `[must]` で指摘する。
- 認証情報付与条件を広げる変更では、allowlist 外へ送られない negative test と、必要な host へ送られる positive test があるか確認する。
- fake server / integration test では、public asset endpoint が Authorization header を拒否する経路が保たれているか確認する。

正本: `doc/guidelines/credential-scope-guidelines.md`
