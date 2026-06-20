# 作業ブランチメモ

- ブランチ: v1/16-final-e2e
- PR: #58
- 最終更新: 2026-06-20

## 目的

v1.0 リリース前の実 token 総合 E2E を実施する。自動テストでは覆えない TTY interactive selection、配布形態での直接実行、実 workspace のリッチコンテンツ表示をユーザー協働で確認し、結果を記録する。

## 現在の状況

- Issue #30 の依存(v1-10〜v1-15)は `progress.md` 上すべて done。
- 作業ブランチを `main` から作成済み。
- E2E checklist (a)〜(g) は実施済み。追加 rendering E2E も実施済み。見つかった所見は #54〜#57 / #59 として Issue 化済み。

## 決定事項

- このタスク内では問題修正を行わない。問題が見つかった場合は新しい GitHub Issue として起票し、修正と再 E2E の要否はユーザー判断に委ねる。
- note / PR / Issue コメントには token、secret reference、個人情報、workspace 固有の非公開情報を書かない。
- PR の変更内容はこの note と `progress.md` 更新を基本とする。

## 次にやること

- PR を作成し、採番後に note rename と `progress.md` の PR 番号反映を行う。

## 検証

### (a) TTY interactive selection

- 担当: ユーザー
- 結果: 成功(条件付き)
- 記録:
  - macOS バイナリを 1Password CLI 経由で直接実行した場合、`stdin` は TTY だが `stdout` が TTY 判定されず、interactive selection には入らず候補一覧 + exit 2 になった。
  - `script` 経由で pseudo-TTY を割り当てると、曖昧 keyword に対する候補 7 件の選択 UI が表示された。
  - 選択 UI から member channel を選択し、export が成功した。

### (b) non-TTY 経路

- 担当: agent
- 結果: 成功
- 記録:
  - `--no-interactive` 付きで曖昧 keyword を指定した場合、候補一覧を表示し、より具体的な channel 指定を促して exit 2 になることを確認。
  - `--no-interactive` 付きで channel 未指定の候補過多ケースでは、候補数を表示し、より具体的な channel 指定を促して exit 2 になることを確認。

### (c) `--reuse-cache` 実機

- 担当: agent / ユーザー
- 結果: 成功
- 記録:
  - `--keep-cache` 付きで 1 回 export し、`.cache/metadata.json` / `.cache/slack_api_cache.json` / `.cache/assets_manifest.json` が残ることを確認。
  - 同条件で `--reuse-cache` を指定した 2 回目の実行で、cache 再利用ログ、8 users の再利用、custom emoji list の再利用、15 assets の copy-from-cache を確認。
  - 1 回目と 2 回目の assets は同一。`index.html` の差分は `Exported` の実行時刻のみ。

### (d) TZ

- 担当: agent
- 結果: 成功
- 記録:
  - `TZ=Asia/Tokyo` 付き実行で、HTML header の `Exported` が `UTC+09:00` を表示することを確認。
  - message 時刻も UTC timestamp に対して JST 表示になっていることを確認。

### (e) 配布形態

- 担当: ユーザー / agent
- 結果: 成功
- 記録:
  - v1-13 の snapshot macOS arm64 バイナリを直接実行し、`--version` が snapshot version を表示することを確認。
  - 同バイナリで実 token export が成功することを確認。
  - TZ 未指定の直接実行でも、HTML header の `Exported` が `UTC+09:00` を表示することを確認。

### (f) 件数の多い channel

- 担当: agent / ユーザー
- 結果: 実施
- 記録:
  - 実運用 channel で `--days 90 --max-posts 100` を実行し、timeline 26 件、assets 15 件を取得。
  - rate limit 待機表示は今回の実データでは発生しなかった。

### (g) 生成 HTML の目視レビュー

- 担当: ユーザー
- 結果: 実施
- 記録:
  - `e2e-out` 配下の生成物を確認。`index.html` / `style.css` / assets が生成され、HTML から参照されるローカルファイルの欠落はなかった。
  - 生成 assets は avatar / custom emoji / URL preview image / upload thumbnail / upload original を含む。
  - system 行、actor prefix、reaction、URL preview image、upload image の表示を実データで確認。
  - 1Password CLI 経由の direct 実行では interactive selection に入れないため、TTY 選択の実利用には `script` などの pseudo-TTY workaround が必要だった。
  - app / bot が channel に追加された system message で、追加操作を行ったユーザー情報が表示されない。
  - 長い URL を含む channel topic system message で、時刻表示が折り返される。
  - URL preview の service 部分に favicon / service icon 相当の小アイコンが表示されない。

### 追加 rendering E2E

- 担当: ユーザー / agent
- 結果: 実施
- 記録:
  - rendering 確認用の投稿を追加し、実 token export が成功。timeline 10 件、thread 2 本、replies 6 件、assets 11 件を取得。
  - HTML から参照されるローカルファイルの欠落はなかった。
  - bold / italic / strike / inline code / quote / fenced code block / plain URL / 標準 emoji の表示を確認。
  - thread reply 6 件、標準 emoji reaction、custom emoji reaction を確認。
  - upload image 3 件、添付 PDF 1 件、URL preview image 1 件、edited marker を確認。
  - 削除済み投稿は今回の出力には現れなかった。
  - Slack-style labeled link 相当の投稿が label 表示にならないケースを確認し、#59 として Issue 化した。

### 所見の Issue 化

- 結果: 実施
- 記録:
  - #54: 1Password CLI 経由でも TTY interactive selection を使いやすくする。
  - #55: app / bot 追加の system message で追加操作を行ったユーザーを表示する。
  - #56: 長い URL を含む system message で時刻表示が折り返される。
  - #57: URL preview に service icon / favicon 相当の表示を追加する。
  - #59: mrkdwn の labeled link が実データで label 表示にならないケースを調査する。

## リスク・ブロッカー

- Issue #30 の完了を妨げるブロッカーはなし。
- 1Password CLI 経由の直接実行では stdout が TTY 判定されず、interactive selection が無効化される環境がある。`script` 経由では選択 UI が表示されることを確認済み。

## セッションログ

- 2026-06-20: Issue #30 を GitHub MCP で確認。`progress.md` で依存 v1-10〜v1-15 が done であることを確認。ユーザーが `git fetch origin main` を実行し、`main...origin/main` が一致していることを確認後、`v1/16-final-e2e` ブランチを作成。
