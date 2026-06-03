# help

このディレクトリには、利用者が GitHub 上で直接読む help / how-to を置く。

CLI のエラー出力や `--help` から URL で案内する文書は、このディレクトリに置く。

## 置くもの

- 初回セットアップ手順
- Slack App / token 発行など、CLI に全文表示すると長すぎる手順
- 利用者向けの how-to
- よくあるエラーと対処

## 置かないもの

- 設計判断の経緯: `doc/design/decision-log/` に置く
- 仕様検討の素案: `doc/design/` に置く
- AI agent / Git / GitHub / PR などの作業ルール: `doc/guidelines/` に置く

## 文体

利用者がそのまま実行できる手順として書く。設計上の迷いや検討経緯は書かず、必要な場合は decision log へ分ける。
