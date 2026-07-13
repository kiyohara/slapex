# 0041 導入手段の拡充（install script と Homebrew tap）

- 状態: decided
- 作成日: 2026-06-22
- 最終更新日: 2026-07-13
- 関連: `doc/design/architecture.md`, `doc/design/decision-log/0034-distribution-method.md`, `doc/design/decision-log/0031-supported-platforms.md`

## 背景

配布の主経路は GitHub Releases への単一バイナリ添付（0034）。`README.md` のインストール手順は download・checksum 確認・`chmod`・`mv` を手動で行う形式で、正確だがステップ数が多く、初回利用者には離脱要因になりうる（PR #47 レビューで指摘、Issue #50）。0034 では Homebrew tap を将来検討（未決事項）として記録していた。両案をどう扱うか検討した。

## 候補

- 案A: macOS 向け Homebrew tap（`brew install --cask kiyohara/tap/slapex`）。
- 案B: macOS + Linux 向け install script（`scripts/install.sh`、`curl | sh`）。
- 現状維持（手動手順のみ）。

## 検討内容

- install script（案B）は外部インフラ不要で自己完結する。OS / arch 自動判定・checksum 検証・install 先指定を 1 コマンドにまとめられ、既存の GitHub Releases 配布物（0034 の asset 命名・checksum）にそのまま乗る。
- Homebrew tap（案A）は macOS の導入体験を最も良くする。ただし `brew install --cask <user>/tap/<cask>` の UX は「repo 名が `homebrew-` で始まる」tap 規約に依存するため、専用 tap repo（`kiyohara/homebrew-tap`）が要る。goreleaser が別 repo へ cask を push するには cross-repo の write token も要る（release workflow の既存 `GITHUB_TOKEN` は同一 repo 限定）。専用 repo + token 方式が現行の主流。
- 自前 tap の CLI ツールは formula と cask の両方が可能。検討当初は formula を基本としたが、GoReleaser v2.10 以降の Homebrew publish は `homebrew_casks` が現行経路で、旧 Homebrew formula publish は deprecated であるため、GoReleaser 方針に合わせて cask に寄せる。
- 案A は専用 repo + token（ユーザー作業）が前提で、cask は次回 release で初めて自動生成される（v1.0.0 は公開済みのため bootstrap も要る）。案B にはこれらの前提がない。
- cask は GitHub Releases の未署名 raw binary を取得するため、macOS の quarantine 属性が残ると初回実行時に Gatekeeper warning が出る。短期対処として cask の `postflight` hook で `com.apple.quarantine` を外す。Developer ID 署名 + notarization は Apple Developer Program の維持コストを伴うため、必要性が出た時点で別途判断する。

## 決定

- 配布の主経路は引き続き GitHub Releases への単一バイナリ添付とする（0034 を維持）。
- 導入補助として **案B（`scripts/install.sh`）を先行して採用**する。手動手順は `README.md` に「詳細版」として残す。
- **案A（Homebrew tap）も採用方針として確定**し、専用 tap repo + cross-repo write token を前提に後続で実装する（Issue #50）。GoReleaser の現行方針に合わせ、formula ではなく cask を基本とする。
- 0034 の未決事項「Homebrew tap」は、本ログで採用方針（後続実装）に更新する。

2026-06-26 に v1.0.1 release workflow で GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.0.1"` へ自動更新されることを確認した。release workflow と tap repo への cross-repo publish は成功している。さらに、ユーザー手元の Homebrew 環境で `brew upgrade --cask slapex` により 1.0.0 から 1.0.1 へ更新され、`slapex --version` が `slapex 1.0.1` を返すことを確認した。これにより Issue #79 の Homebrew cask 自動更新経路の release 検証は完了した。

2026-07-02 に v1.1.0 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.1.0"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.1.0` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `slapex --version` が upgrade 前に `slapex 1.0.1`、`brew update && brew upgrade --cask slapex && slapex --version` 後に `slapex 1.1.0` を返すことを確認した。

2026-07-03 に v1.1.1 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.1.1"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.1.1` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `slapex --version` が upgrade 前に `slapex 1.1.0`、`brew update && brew upgrade --cask slapex && slapex --version` 後に `slapex 1.1.1` を返すことを確認した。

2026-07-04 に v1.1.2 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.1.2"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.1.2` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `slapex --version` が upgrade 前に `slapex 1.1.1`、`brew update && brew upgrade --cask slapex && slapex --version` 後に `slapex 1.1.2` を返すことを確認した。

2026-07-13 に v1.2.0 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.2.0"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.2.0` を返すことを確認した。

## 理由

- 案B は最小コストで「1 コマンド導入」を即時に届けられ、外部依存もない。
- 案A は導入体験を最も良くするが、ユーザー側インフラ（repo + token）が前提のため、案B と分離して進める方が価値提供が早い。
- 主経路（Releases 単一バイナリ）を維持し補助手段を足す形にすることで、配布方針の一貫性を保てる。
- cask 採用は GoReleaser の現行 publish 経路に合わせるため。formula の方が CLI ツールとして直感的な面はあるが、deprecated な設定へ新規に寄せるより、現行経路へ合わせる判断を優先する。
- quarantine hook は未署名 binary の導入体験を Homebrew 経由で成立させるための暫定措置。署名・公証を行うのが最も正攻法だが、現時点では配布コストを抑える判断を優先する。

## 影響

- 実装: `scripts/install.sh`（POSIX sh、OS / arch 判定・checksum 検証・install 先指定・`--version` / `--bin-dir` / `--dry-run` / `--help`）と `scripts/install_test.sh`（検出マッピングのテスト）を追加（案B）。
- ドキュメント: `README.md` のインストール節にクイックインストール（案B）を追加し、手動手順を詳細版として残す。`doc/design/architecture.md` の配布方式に install script を追記。
- 案A: 専用 tap repo の作成と write token の Actions secret 追加（ユーザー作業）、goreleaser 設定（`homebrew_casks`）、未署名 binary の Gatekeeper warning 対策として cask install 後に quarantine 属性を外す hook を入れる構成で実装済み。v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 / v1.2.0 release で release workflow から tap repo への cask 自動更新を確認済み。v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 では Homebrew 経由の upgrade も確認済み。
- decision log: `index.md` の未決事項から「Homebrew tap」を移し、本ログを現在有効な主要方針に追加。0034 に関連を追記。

## 後から見直す条件

- install script の保守コストや利用実態から、`curl | sh` 経路を縮小・変更する必要が出た場合。
- Homebrew cask の未署名 binary 体験や upgrade 経路に問題が出た場合。
- Windows 対応（0031）など配布 target が増えた場合の install script 拡張。
