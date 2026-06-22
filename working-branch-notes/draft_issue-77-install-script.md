# 作業ブランチメモ

- ブランチ: `issue-77-install-script`
- PR: (未採番)
- 最終更新: 2026-06-22

## 目的

post-v1 改善（Issue #77、#50 案B）。README の手動インストール手順を 1 コマンド化する `scripts/install.sh` を提供し、初回利用者の導入ハードルを下げる。手動手順は「詳細版」として残す。

- スコープの正本: Issue #77（#50 から分割した案B）。
- #50 は案A（Homebrew tap）に scope を絞り、本タスク（案B）を先行。

## 現在の状況

- `scripts/install.sh` を新規作成（POSIX sh）。OS/arch 自動判定、checksum 検証、install 先指定、`--version`/`--bin-dir`/`--dry-run`/`--help`。
- `scripts/install_test.sh` で OS/arch → asset 名マッピングを `uname` stub + `--dry-run`（オフライン）で検証。
- README にクイックインストール節を追加（手動手順は詳細版として保持）。
- decision log `0041` 新設、index と `0034` を更新。`architecture.md` 配布方式に追記。`progress.md` に post-v1 行を追加。

## 決定事項

- **案B 先行・案A 後追い**: 案B は外部依存なしで自己完結するため先行 PR にする。案A（Homebrew tap）は専用 tap repo + cross-repo write token（ユーザー作業）が前提のため #50 で後追い。経緯と前提は #50 本文と decision log 0041 に記録。
- **version 解決**: 既定は GitHub API（`releases/latest`）の `tag_name` を jq 非依存で抽出。`--version` / `SLAPEX_VERSION` で上書き。
- **checksum 検証は必須**: `sha256sum` または `shasum -a 256` で該当行のみ照合し、不一致は中断。`curl|sh` の信頼は HTTPS + repo に依存するため、手動手順（詳細版）を残して検証重視の経路も維持。
- **install 先**: 既定 `/usr/local/bin`、`--bin-dir`/`SLAPEX_BIN_DIR` で変更。書込不可時は sudo か代替 dir を案内（`curl|sh` でも sudo は /dev/tty から prompt 可）。
- **対話選択は不採用**: `curl|sh` は stdin がスクリプトに使われるため、install 先は引数・環境変数で指定（Issue スコープ外に明記）。
- **stdout/stderr 規律**: 進捗・診断は stderr、最終 install path のみ stdout（slapex 本体と同じ規律）。

## 次にやること

- 検証（shellcheck / Linux E2E / 検出テスト）を実行し下記に記録。
- commit / push / PR 作成（`Closes #77`）、note 採番（`number-working-branch-note`）。

## 検証

すべて Docker で実行（`development-command-guidelines.md` 準拠）。

- **shellcheck**（`koalaman/shellcheck:stable`）: `scripts/install.sh` / `scripts/install_test.sh` ともに指摘なし（exit 0）。
- **検出 + checksum テスト**（`scripts/install_test.sh` を busybox sh = `alpine:3` で実行し POSIX 準拠も確認）: 11 ケース全合格。
  - OS/arch → asset 名: Darwin/arm64・Darwin/x86_64・Linux/x86_64・Linux/amd64・Linux/aarch64・Linux/arm64 を正しく対応付け。
  - 非対応: Linux/armv7l・FreeBSD/amd64・Windows_NT/x86_64 を reject。
  - checksum: 一致を受理し、不一致を中断。
- **E2E（実 v1.0.0）**: `alpine:3`（aarch64）で `install.sh --version v1.0.0 --bin-dir /tmp/bin` を実行。`slapex_linux_arm64` を取得 → checksum 検証通過 → install → `slapex --version` が `slapex 1.0.0` を出力。stdout は install path のみ、進捗は stderr。
- darwin 経路は実機未実行だが、検出ロジックを `--dry-run` + `uname` stub で確認（上記）。実バイナリ取得経路は darwin / linux で共通。

## リスク・ブロッカー

- `curl|sh` は remote code 実行を伴う。binary の checksum は検証するが script 自体は HTTPS + repo 信頼に依存。手動手順（詳細版）を残すことで検証重視の導入経路も提供する。
- README のクイックインストール URL は、install.sh を含む ref が必要。v1.0.0 tag には未収録のため当面 `main` を指す（注記済み）。

## セッションログ

- 2026-06-22: Issue #50 を案A/案B に分割（案B = #77 新規、#50 を案A に絞り込み）。`issue-77-install-script` ブランチ作成。install.sh / テスト / README / decision log 0041 / architecture / progress を実装。
