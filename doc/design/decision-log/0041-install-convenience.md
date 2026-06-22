# 0041 導入手段の拡充（install script と Homebrew tap）

- 状態: decided
- 作成日: 2026-06-22
- 最終更新日: 2026-06-22
- 関連: `doc/design/architecture.md`, `doc/design/decision-log/0034-distribution-method.md`, `doc/design/decision-log/0031-supported-platforms.md`

## 背景

配布の主経路は GitHub Releases への単一バイナリ添付（0034）。`README.md` のインストール手順は download・checksum 確認・`chmod`・`mv` を手動で行う形式で、正確だがステップ数が多く、初回利用者には離脱要因になりうる（PR #47 レビューで指摘、Issue #50）。0034 では Homebrew tap を将来検討（未決事項）として記録していた。両案をどう扱うか検討した。

## 候補

- 案A: macOS 向け Homebrew tap（`brew install kiyohara/tap/slapex`）。
- 案B: macOS + Linux 向け install script（`scripts/install.sh`、`curl | sh`）。
- 現状維持（手動手順のみ）。

## 検討内容

- install script（案B）は外部インフラ不要で自己完結する。OS / arch 自動判定・checksum 検証・install 先指定を 1 コマンドにまとめられ、既存の GitHub Releases 配布物（0034 の asset 命名・checksum）にそのまま乗る。
- Homebrew tap（案A）は macOS の導入体験を最も良くする。ただし `brew install <user>/tap/<formula>` の UX は「repo 名が `homebrew-` で始まる」規約に依存するため、専用 tap repo（`kiyohara/homebrew-tap`）が要る。goreleaser が別 repo へ formula を push するには cross-repo の write token も要る（release workflow の既存 `GITHUB_TOKEN` は同一 repo 限定）。専用 repo + token 方式が現行の主流。
- 自前 tap の CLI ツールは formula と cask の両方が可能。formula は `brew upgrade` が素直で goreleaser 本家も formula。cask は Homebrew 公式の将来方向だが CLI では upgrade 周りに難があり、移行は後追いで足りる。
- 案A は専用 repo + token（ユーザー作業）が前提で、formula は次回 release で初めて生成される（v1.0.0 は公開済みのため bootstrap も要る）。案B にはこれらの前提がない。

## 決定

- 配布の主経路は引き続き GitHub Releases への単一バイナリ添付とする（0034 を維持）。
- 導入補助として **案B（`scripts/install.sh`）を先行して採用**する。手動手順は `README.md` に「詳細版」として残す。
- **案A（Homebrew tap）も採用方針として確定**し、専用 tap repo + cross-repo write token を前提に後続で実装する（Issue #50）。formula を基本とし、cask 移行は後追いとする。
- 0034 の未決事項「Homebrew tap」は、本ログで採用方針（後続実装）に更新する。

## 理由

- 案B は最小コストで「1 コマンド導入」を即時に届けられ、外部依存もない。
- 案A は導入体験を最も良くするが、ユーザー側インフラ（repo + token）が前提のため、案B と分離して進める方が価値提供が早い。
- 主経路（Releases 単一バイナリ）を維持し補助手段を足す形にすることで、配布方針の一貫性を保てる。

## 影響

- 実装: `scripts/install.sh`（POSIX sh、OS / arch 判定・checksum 検証・install 先指定・`--version` / `--bin-dir` / `--dry-run` / `--help`）と `scripts/install_test.sh`（検出マッピングのテスト）を追加（案B）。
- ドキュメント: `README.md` のインストール節にクイックインストール（案B）を追加し、手動手順を詳細版として残す。`doc/design/architecture.md` の配布方式に install script を追記。
- 案A（後続）: 専用 tap repo の作成と write token の Actions secret 追加（ユーザー作業）、goreleaser 設定（`brews`）、v1.0.0 向け formula の bootstrap が必要。
- decision log: `index.md` の未決事項から「Homebrew tap」を移し、本ログを現在有効な主要方針に追加。0034 に関連を追記。

## 後から見直す条件

- install script の保守コストや利用実態から、`curl | sh` 経路を縮小・変更する必要が出た場合。
- Homebrew を formula から cask へ移行する判断が必要になった場合。
- Windows 対応（0031）など配布 target が増えた場合の install script 拡張。
