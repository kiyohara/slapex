# 進捗管理表

このファイルには、設計および実装作業の進捗を記録していく。

想定読者は、実装作業を行う人間、および AI agent である。

横断的な作業状況(作業項目、状態、次にやること、検証状況、未解決のリスクやブロッカー)を一覧で把握するための管理表として使う。詳細な検討経緯は `doc/design/decision-log/` に置き、このファイルでは現在の作業状況を把握しやすく保つ。

## 運用メモ

- このファイルは、プロダクト全体の進捗を見渡すための一覧として使う。仕様設計や decision log ではなく、横断的な作業状況の管理表として扱う。
- リリース台帳と、進行中タスクの索引を兼ねる。`release` skill はリリースごとに「リリース履歴」へ行を追加し、Issue 駆動タスクは依存確認と状態更新にこの表を使う(`doc/guidelines/issue-driven-task-execution.md`)。
- `working-branch-notes/` はブランチ単位の作業目的・状況・判断・引き継ぎメモを扱う。このファイルの 1 アイテムが必ずしも 1 ブランチに対応するとは限らない。
- 完了タスクが溜まる、リリース後、まとまった Issue 群を消化し終えた区切りなどで、`maintain-progress` skill を使って定期的に整理する。整理の観点(完了表の圧縮・現況/リリース履歴の更新・参照整合性と境界の維持)はこの skill が正本。
- この運用は暫定であり、実際の作業に合わせて軽く更新していく。

## 現況

v1.0.0 / v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 / v1.2.0 / v1.2.1 を GitHub Releases で公開済み。配布経路は単一バイナリ(GitHub Releases)、install script(`scripts/install.sh`)、Homebrew cask(`kiyohara/homebrew-tap`)の 3 つ。v1.2.1 の後に merge した修正は未リリースである。うち FU-01(#202 / PR #221、データ破壊の修正)は patch release 候補としている。

リファクタリングの調査・評価(#188)で採用した8施策のうち、未完了の施策を追跡する。実施方針は [decision log 0056](doc/design/decision-log/0056-incremental-refactoring-plan.md)、詳細な作業条件は各Issueを参照する。

PR #201(RF-02)の review などで見つかった既存挙動の修正・改善 Issue も追跡する。優先順位と、リファクタリング施策と合わせた全体の着手順は「進行中タスク: review で見つかった既存挙動の修正」を参照する。

review cycle の skill(`drive-issue-to-reviewed-pr` と `review-pull-request`)の改善 Issue も、「進行中タスク: review cycle の skill 改善」で追跡する。

## 進行中タスク: 段階的リファクタリング

表の順を推奨順として直列実行する。依存欄は必須条件のみとし、単なる推奨順は含めない。RF-03/RF-06は同じexportを触るため推奨順で直列実行するが、RF-06はRF-03なしでも着手可能。他の施策は技術的に分離可能だが、運用上は並行実行しない。完了した施策は「完了済みフェーズ(参考)」を参照する。

| ID | Issue | 状態 | 依存 | 次にやること | PR |
|---|---|---|---|---|---|
| RF-03 | [#191](https://github.com/kiyohara/slapex/issues/191) Run工程・状態整理 | todo | #188, #189, #190 | 工程の入出力を整理 | - |
| RF-06 | [#194](https://github.com/kiyohara/slapex/issues/194) cache入力整理 | todo | #188 | 同型位置引数を集約 | - |
| RF-04 | [#192](https://github.com/kiyohara/slapex/issues/192) retry共通化 | todo | #188 | FU-18の後、streamingとの差を保って共通化 | - |
| RF-05 | [#193](https://github.com/kiyohara/slapex/issues/193) CLI option集約 | todo | #188 | 通常/demoの転記を整理 | - |

## 進行中タスク: review で見つかった既存挙動の修正

PR #201(RF-02)の review 補助分析で見つかった、リファクタリングのスコープ外にある既存挙動を Issue #202〜#211 として登録した。表の順を優先順とし、データ破壊 → 利用者に見える表示の誤り → 件数・cache の整合 → API 呼び出しの無駄と edge case → 後片付け、の順に並べる。各 Issue の確認状況(code 読解のみか、実行で再現済みか)と作業条件は Issue 本文を正本とする。その後 PR #221(FU-01)の review で見つかった Issue #222 も、発生経路と性質が同じ follow-up として同じ表で追跡する。PR #243(FU-03)の実装と review で見つかった Issue #245〜#247、PR #241(FU-04)と PR #242(FU-02)の実装と review で見つかった Issue #249〜#251、PR #238 の CI で見つかった flaky test の Issue #254 も同様に追跡する。完了した項目は「完了済みフェーズ(参考)」を参照する。

段階的リファクタリングと合わせた残りの着手順は次のとおり。Run の取得工程に関わる FU-05 は RF-03(#191)の直後に置く。FU-18 は RF-04 が検証に使う test を直すため、RF-04 より前に置く。FU-13 と FU-14(`addImage`)、FU-15 と FU-16(`output.Assets`)、FU-08 と FU-17(`collectUserIDs`)は、それぞれ同じ箇所を触るため直列に進める。それ以外は技術的に独立だが、運用上は並行実行しない。順序見直しの経緯は decision log 0056 の 2026-09-06 追記を参照する。

RF-03 → FU-05 → FU-13 → FU-14 → FU-18 → FU-15 → FU-16 → RF-06 → RF-04 → RF-05 → FU-08 → FU-17 → FU-09 → FU-10 → FU-12

| ID | Issue | 状態 | 依存 | 次にやること | PR |
|---|---|---|---|---|---|
| FU-05 | [#206](https://github.com/kiyohara/slapex/issues/206) filter 時の broadcast thread 件数不整合 | todo | #191 | RF-03 後に characterization test から着手 | - |
| FU-13 | [#246](https://github.com/kiyohara/slapex/issues/246) 外部連携画像の url_private を original として保存 | todo | PR #243, PR #244 | 実 payload を確認し、original を取得しない形にする | - |
| FU-14 | [#247](https://github.com/kiyohara/slapex/issues/247) URL 無し画像の size 超過で空の source_url を記録 | todo | PR #243, PR #244 | FU-13 の後(同じ `addImage`) | - |
| FU-18 | [#254](https://github.com/kiyohara/slapex/issues/254) 429 retry test がまれに失敗 | todo | - | 失敗理由を log に出し、原因を調べる | - |
| FU-15 | [#249](https://github.com/kiyohara/slapex/issues/249) 同じファイルの size 超過を重複記録 | todo | - | `SkipTooLarge` で記録済みの URL を足さない | - |
| FU-16 | [#250](https://github.com/kiyohara/slapex/issues/250) download 中の size 超過を asset failed と警告 | todo | - | FU-15 の後(同じ `output.Assets`) | - |
| FU-08 | [#209](https://github.com/kiyohara/slapex/issues/209) label 付き mention の users.info | todo | - | 収集条件を描画側に揃える | - |
| FU-17 | [#251](https://github.com/kiyohara/slapex/issues/251) 表示しない投稿者の users.info | todo | - | FU-08 の後(同じ `collectUserIDs`) | - |
| FU-09 | [#210](https://github.com/kiyohara/slapex/issues/210) 取得境界の秒未満切り捨て | todo | - | 精度統一か入力拒否かを決めて実装 | - |
| FU-10 | [#211](https://github.com/kiyohara/slapex/issues/211) export 分割後の後片付け | todo | #190 | RF-03 / RF-06 で吸収可。残った項目だけ実施 | - |
| FU-12 | [#245](https://github.com/kiyohara/slapex/issues/245) URL 無しファイルの設計文書の記述 | todo | PR #243 | `slack-api-usage.md` の記述を実装に揃える | - |

## 進行中タスク: review cycle の skill 改善

PR #233、PR #235、PR #237、PR #242 の作業で見つかった、`drive-issue-to-reviewed-pr` と `review-pull-request` の review cycle に関わる Issue を追跡する。どれも `drive-issue-to-reviewed-pr` を触るため直列に進める。Issue の上では順序を問わないため、表は優先度の順に並べる。

| ID | Issue | 状態 | 依存 | 次にやること | PR |
|---|---|---|---|---|---|
| RV-02 | [#253](https://github.com/kiyohara/slapex/issues/253) スコープ外とした指摘の再確認 | done(PR merge後) | - | merge後は対応なし | [#256](https://github.com/kiyohara/slapex/pull/256) |
| RV-03 | [#255](https://github.com/kiyohara/slapex/issues/255) cloud session の subagent の tool 選択 | done(PR merge後) | - | merge後は対応なし | [#258](https://github.com/kiyohara/slapex/pull/258) |
| RV-01 | [#252](https://github.com/kiyohara/slapex/issues/252) review の prefix の統一への追従 | todo | - | 判断基準の理由と decision log 0059 を直す | - |

## リリース履歴

| バージョン | 状態 | メモ |
|---|---|---|
| v1.0.0 | released | 初版。スコープは decision log 0036、リリース実施は PR #76 |
| v1.0.1 | released | user token default への認証方針転換(Issue #81 / PR #84) |
| v1.1.0 | released | token prompt / interactive selection / CLI output UX / 開発ループ整備を含む minor release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.1 | released | `--reuse-cache` の出力ディレクトリ検出改善、exported HTML header / footer 調整、logo asset 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.1.2 | released | 出力プレビュー・匿名化サンプル、token 不要 `--demo`、Slack App セットアップ help スクリーンショット、export footer への tool version 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.2.0 | released | 取得範囲指定(`--date` / `--from` / `--to`)、emoji 除外(`--exclude-body-emoji` / `--exclude-reaction-emoji`)、footer timezone 改善、README / help 再構成を含む minor release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |
| v1.2.1 | released | bot 投稿の投稿者名 / avatar を `bots.info` で解決し `APP` 表示を追加、asset の保存拡張子を download 内容から決定、PNG logo asset 追加を含む patch release。Release assets / checksum / Linux `--version` / Homebrew cask 更新 / Homebrew 経由 upgrade を確認済み |

## 完了済みフェーズ(参考)

詳細な経緯は decision log と各 Issue / PR を正本とする。ここでは到達点だけ残す。

- 設計基盤・AI agent 入口・ガイドライン整備 — done(PR #1 / #5 ほか)。
- 詳細仕様確定 / アーキテクチャ選定(Go + stdlib-first)/ PoC 実装による機能充足性確認 — done(decision log 0024〜0034)。
- v1.0 リリース実装プラン(v1-01〜v1-17: CI・テスト整備・`--reuse-cache`・コンテナ TZ・goreleaser・README/LICENSE・総合 E2E・リリース実施)— 全 done(Issue #15〜#31 / PR #33〜#76)。運用方式は decision log 0036 / 0037。
- post-v1 改善(配布・導入)— install script(#77 / PR #78)、Homebrew cask(#50 / PR #80)、cask 自動更新の release 検証(#79 / PR #85)、user token default 転換(#81 / PR #84)すべて done。経緯は decision log 0041。
- 1Password integration 整備 — token 注入 help と `op run` 経由の interactive selection を整備済み(Issue #53〜#54 / PR #96・#98)。
- 開発ループ整備 — 既存 Issue 登録 skill、Issue 駆動タスク実行 skill、開発ループ入口ドキュメントを追加済み(Issue #88〜#90 / PR #92〜#94)。
- CLI 出力 UX / 利用者向け help 基盤 / サンプル表示の初期整備 — styled/plain 出力(#100 / PR #102)、README 出力プレビューと同梱サンプル(#51 / PR #114)、Slack App セットアップ画像 help(#48 / PR #119)、FAQ(#52 / PR #128)、quickstart(#49 / PR #124)、README 再構成(#123 / PR #131)、文体・リンク方針(#125 / PR #127、#129 / PR #130)を整備済み。方針は decision log 0045 / 0048 / 0049。
- Agent / 開発環境整備 — worktree の project MCP local config セットアップ(#8 / PR #149、decision log 0051)。関連 follow-up として Cursor MCP command の相対 path 化(#151 / PR #152、decision log 0053)、GitHub MCP 優先規則・PR review skill・手動 resolve 運用(#161 / #162 / #165、PR #163 / #164 / #166)も完了。
- 利用者向け help / サンプル更新運用 — 文体一括修正(#126 / PR #143)、asset 内容 hash(#135 / PR #150、decision log 0052)、preview 枠線(#136 / PR #142)、sample / preview / demo 更新 skill(#137〜#139 / PR #158〜#160)、quickstart 全体フロー SVG(#140 / PR #157)を整備済み。
- 取得範囲指定 — 特定日の `--date`(#153 / PR #168)、任意期間の `--from` / `--to`(#154 / PR #169)、footer の timezone 表示改善(#170 / PR #177)を実装済み。
- emoji 除外(privacy signal) — 本文 shortcode の `--exclude-body-emoji`(#155 / PR #175)と、reaction の `--exclude-reaction-emoji`(#156 / PR #176)を実装済み。
- 段階的リファクタリング(完了分) — RF-00 調査・評価(#188 / PR #197)、RF-08 記録配置の整合(#196 / PR #198)、RF-07 現行設計文書(#195 / PR #199)、RF-01 test 準備集約(#189 / PR #200)、RF-02 export 分割・命名(#190 / PR #201)を完了。方針は decision log 0056。残りの施策は「進行中タスク: 段階的リファクタリング」で追跡する。
- review で見つかった既存挙動の修正(完了分) — FU-01 reuse-cache 自己コピーで asset が 0 byte(#202 / PR #221)、FU-02 download 時の size 超過が取得失敗表示(#203 / PR #242)、FU-03 URL 無し upload が外部連携表示(#204 / PR #243)、FU-04 unfurl text の mention 未解決(#205 / PR #241)、FU-06 Done 経過時間が Now 起点(#207 / PR #240)、FU-07 upload_thumb manifest の mimetype / size(#208 / PR #244)、FU-11 同一ファイル再利用時の summary 文言(#222 / PR #236)を修正済み。FU-02〜FU-04 / FU-06 / FU-07 / FU-11 はユーザー判断で推奨順より先に実施した(経緯は PR #236 / PR #240〜#244)。残りは「進行中タスク: review で見つかった既存挙動の修正」で追跡する。
