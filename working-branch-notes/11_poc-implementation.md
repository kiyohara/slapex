# 作業ブランチメモ

- ブランチ: poc-implementation
- PR: #11
- 最終更新: 2026-06-10

## 目的

PoC 実装(3 ステップ作業の Step 3)。選定アーキテクチャ(Go + stdlib-first、PR #10)が確定仕様(PR #9)の要件を満たせるかを、happy path の実装と実 workspace への E2E 実行で確認する。テストコードは対象外(依頼条件)。

## 現在の状況

- PoC 本体を実装済み。`go vet` / 全パッケージ build / CLI 基本動作(--help / --version / token 未設定 exit 3 / 不正引数 exit 2)/ 4 target(darwin/linux × amd64/arm64)のクロスコンパイルを確認済み。
- Docker Compose 開発環境(`compose.yaml`、service `dev`)を追加し、`development-command-guidelines.md` と rule shim を具体化。
- 実 token E2E を完了。検証用 workspace の実 channel に対して export を複数回実行し、仕様の主要経路の動作を確認した(詳細は「検証」)。機能充足性の結論: **選定アーキテクチャ(Go + stdlib-first)で確定仕様を実装できることを確認**。

## 決定事項

- パッケージ構成は `architecture.md` の目安どおり: `cmd/slapex` / `internal/slack`(thin client + 429/Retry-After/バックオフ)/ `internal/export`(orchestration + channel 選択)/ `internal/render`(mrkdwn 変換 + html/template + style.css)/ `internal/output`(出力 root、label slug、assets、.cache)/ `internal/emoji`。
- huh v2 の module path は `charm.land/huh/v2`。label の NFC 正規化に `golang.org/x/text` を準標準枠で追加(decision log 0033 に追記)。
- 標準絵文字は iamcal/emoji-data から `tools/genemoji` で生成した約 2,000 件の対応表を `go:embed` で同梱。
- PoC で顕在化した仕様ギャップ「avatar 画像が保存 assets 一覧に未定義」を decision log 0035 で解消し、`output-format.md` / `cache.md` に反映。

## PoC スコープ除外(実装フェーズで対応)

- `--reuse-cache`: flag は受け付けるが未実装。指定時は警告を表示して通常取得にフォールバックする。
- rich text(`blocks`)構造レンダリング、`<!date^...>` の書式展開(fallback 文字列を表示)、skin tone 合成: 仕様どおり将来検討。
- legacy attachment はタイトル・本文・preview 画像の簡易表示のみ(色付き枠などの完全模倣はしない。仕様どおり)。

## 次にやること

- PR 作成、採番後に note rename、自己マージ。
- 3 ステップ作業はこれで完了。次フェーズは本実装(テスト整備、--reuse-cache、リリース整備が候補。`progress.md` と decision log index の未決事項を参照)。

## 検証

机上検証:

- `docker compose run --rm dev go vet ./...`: 成功。
- `docker compose run --rm dev go build ./...`: 成功。
- CLI 基本動作: `--version` exit 0 / `--help` exit 0 / token 未設定 exit 3(help URL 案内)/ `--days 0` exit 2 / 不正な `--max-attachment-size` exit 2。
- クロスコンパイル: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 すべて成功(バイナリ 14〜16MB)。

実 token E2E(検証用 workspace の実 channel、token は 1Password secret reference 経由で実行時のみ注入。reference・token は repo に残していない):

- `auth.test` → workspace 解決と label 表示(名前 + domain + team_id)を確認。
- scope 不足時: `missing_scope` → **exit 3 + help URL 案内**が実機で機能(ユーザーが scope 追加・再 install 後に続行)。
- channel ID 指定 / keyword 部分一致(一意)→ 確定、Target label 表示。候補 85 件の no-arg 実行 → 候補過多メッセージ + exit 2(non-TTY 経路)。
- 取得: timeline 4 件 + thread 1 本(replies 4 件、画像付き reply 含む)。pagination・`--days` 境界・ts 昇順整列を確認。
- mrkdwn → HTML: 太字 / inline code / 引用 / URL リンク / mention(`@表示名` 解決 + ハイライト)/ 標準絵文字(Unicode 直接)/ カスタム絵文字(ローカル画像 + alt)を実出力で確認。
- reaction: 標準(🙏)+ カスタム(画像)と件数表示を確認。
- 画像: thumb + original を保存し、thumb 表示 → original クリックの構造を確認。
- URL unfurl: og-image のローカル保存、タイトル・本文・service 表示を確認。
- avatar 保存と表示、date divider、stdout(出力 path 1 行)/ stderr(進捗)分離、`.cache/` 3 ファイルの schema 準拠(metadata / manifest / api cache)、`--keep-cache` を確認。
- 出力 label: workspace label が domain 由来の `kzrb`、channel label が `slack_posts_dumper_test` になることを確認(0029)。

E2E 未確認(実装はあるが検証データに含まれず):

- fenced code block(``` )、`--max-attachment-size` 超過時の置換表示、system 行(channel_join 等)、tombstone、`<!date^>` token、429 レート制限時の待機表示。
- TTY interactive selection(huh)は agent 実行環境に TTY が無く未確認。ユーザーの手元で `./bin/slapex eng` のような曖昧 keyword で確認可能。

## PoC 所見

- **標準 `flag` の制約**: 標準 flag は最初の非 flag 引数で parse を止めるため、仕様の `slapex [channel] [options]`(positional の後ろに option)がそのままでは通らなかった。2 パス parse で stdlib のまま解決(E2E で発見、修正済み)。
- huh v2 の module path は `charm.land/huh/v2`(0033 に追記済み)。
- 表示仕様と保存仕様のギャップ(avatar)を 0035 として解消。
- thin client(自前)は問題なく機能。form encode + `ok`/`error` envelope + cursor pagination は素直に実装できた(0033 の判断を裏づけ)。

## リスク・ブロッカー

- E2E は小規模 channel での確認であり、大規模 channel(数千件、レート制限発生)での挙動は未検証。実装フェーズでの確認事項。
- コンテナ内実行では local timezone が UTC になる(仕様どおり `TZ` で制御可能、0028)。

## セッションログ

- 2026-06-10: PR #10(アーキテクチャ確定)merge 後、`poc-implementation` ブランチを作成。Docker 利用可否を確認し、compose.yaml と Go module を初期化。
- 2026-06-10: thin client / export / render / output / emoji / CLI を実装。初回 build で全パッケージ成功。vet・CLI 基本動作・4 target クロスコンパイルを確認。
- 2026-06-10: development-command-guidelines と rule shim を Go + `dev` service で具体化。architecture.md / 0033 に実装時の確定事項を追記。avatar の仕様ギャップを 0035 として記録・反映。
- 2026-06-10: 実 token E2E を実施。初回で positional 後の option が通らない問題を発見し 2 パス parse で修正。scope 不足 → exit 3 の UX を実機確認。ユーザーに検証用コンテンツ(装飾・thread・画像・reaction)を投稿してもらい、リッチコンテンツの取得・変換・保存を全項目確認。
