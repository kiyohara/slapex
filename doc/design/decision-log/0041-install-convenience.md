# 0041 導入手段の拡充（install script と Homebrew tap）

- 状態: decided
- 作成日: 2026-06-22
- 最終更新日: 2026-10-01
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

2026-07-13 に v1.2.0 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.2.0"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.2.0` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `slapex --version` が upgrade 前に `slapex 1.1.2`、`brew update && brew upgrade --cask slapex && slapex --version` 後に `slapex 1.2.0` を返すことを確認した。

2026-09-03 に v1.2.1 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.2.1"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.2.1` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `slapex --version` が upgrade 前に `slapex 1.2.0`、`brew update && brew upgrade --cask slapex && slapex --version` 後に `slapex 1.2.1` を返すことを確認した。

## 理由

- 案B は最小コストで「1 コマンド導入」を即時に届けられ、外部依存もない。
- 案A は導入体験を最も良くするが、ユーザー側インフラ（repo + token）が前提のため、案B と分離して進める方が価値提供が早い。
- 主経路（Releases 単一バイナリ）を維持し補助手段を足す形にすることで、配布方針の一貫性を保てる。
- cask 採用は GoReleaser の現行 publish 経路に合わせるため。formula の方が CLI ツールとして直感的な面はあるが、deprecated な設定へ新規に寄せるより、現行経路へ合わせる判断を優先する。
- quarantine hook は未署名 binary の導入体験を Homebrew 経由で成立させるための暫定措置。署名・公証を行うのが最も正攻法だが、現時点では配布コストを抑える判断を優先する。

## 影響

- 実装: `scripts/install.sh`（POSIX sh、OS / arch 判定・checksum 検証・install 先指定・`--version` / `--bin-dir` / `--dry-run` / `--help`）と `scripts/install_test.sh`（検出マッピングのテスト）を追加（案B）。
- ドキュメント: `README.md` のインストール節にクイックインストール（案B）を追加し、手動手順を詳細版として残す。`doc/design/architecture.md` の配布方式に install script を追記。
- 案A: 専用 tap repo の作成と write token の Actions secret 追加（ユーザー作業）、goreleaser 設定（`homebrew_casks`）、未署名 binary の Gatekeeper warning 対策として cask install 後に quarantine 属性を外す hook を入れる構成で実装済み。v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 / v1.2.0 release で release workflow から tap repo への cask 自動更新と、Homebrew 経由の upgrade を確認済み。
- decision log: `index.md` の未決事項から「Homebrew tap」を移し、本ログを現在有効な主要方針に追加。0034 に関連を追記。

## 追記(2026-10-01): `postflight` の非推奨化への対応

2026-09-25、Homebrew 7.0.6 で同じ tap の別の Formula を install したとき、slapex の cask の `postflight` について「Calling `postflight` is deprecated! Use `postflight_steps` instead.」の警告が出た(Issue #239)。slapex の cask を操作していない場面でも出ており、Homebrew は tap への報告を求めていた。

- Homebrew は 6.0.16(2026-08-10)から、cask の `preflight` / `postflight` / `uninstall_preflight` / `uninstall_postflight` を非推奨にしている。代わりの `*_steps` は、リテラル引数の step(`run`、`remove`、`on_macos` など)だけを並べる宣言的な DSL で、sandbox の中で実行される。
- Homebrew の方針(Deprecating, Disabling and Removing)は、非推奨にした public API を次の minor または major release で disabled(全利用者に error)にするとしている。7.0.0 では disabled にならなかったが、時期は公表されていない。disabled になると、`brew install --cask slapex` と `brew upgrade --cask slapex` が失敗する。
- GoReleaser は最新の v2.18.2(2026-09-17)でも `homebrew_casks.hooks` を `postflight do ... end` として出力し、`*_steps` を出力する設定を持たない。対応する PR(goreleaser/goreleaser#6873)は 2026-10-01 時点で merge されていない。

候補:

- 案1: hook を残し、GoReleaser の対応を待つ(Issue #239 の仮決め)。
- 案2: GoReleaser の `custom_block` で `postflight_steps` を出力し、`hooks.post.install` をやめる。
- 案3: hook を外し、Homebrew の利用者に手動の `xattr` を案内する。
- 案4: Developer ID の署名と notarization を行う。

決定: 案2 を採る。`.goreleaser.yaml` の `homebrew_casks` から `hooks.post.install` を外し、`custom_block` に次を置く。

```ruby
postflight_steps do
  on_macos do
    run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "."], chdir: "."
  end
end
```

理由:

- 案1 は、GoReleaser の対応より先に Homebrew が disabled にすると install と upgrade が壊れ、その時期を slapex の側で制御できない。
- 案2 は、今の GoReleaser のまま次の release で警告を無くせ、quarantine 属性を外す処理も残る。
- 案3 は、Homebrew の利用者の初回実行で Gatekeeper の警告を出す。案4 は Apple Developer Program の維持コストを伴い、本ログの決定(必要になった時点で別途判断する)を変える理由にならない。

実装上の注意:

- `custom_block` は cask の先頭(`version` の前)に出力される。Homebrew の stanza の順序の規約から外れるが、規約を強制するのは公式 tap の audit と style であり、第三者 tap の install には影響しない。
- 旧 hook は `Dir["#{staged_path}/slapex_darwin_*"]` で binary を探していたが、`run` は glob も shell も展開しない。staged_path には download した binary だけが置かれるため、`chdir: "."`(staged_path が基準になる)で staged_path の全体から属性を再帰的に外す。Homebrew の `{{staged_path}}` の token は、GoReleaser の template と区切りが衝突するため使わない。
- 旧 hook の `system_command` は、失敗すると install を止めていた(`must_succeed: true`)。`run` の既定も同じため、`must_succeed` は指定しない。`xattr -dr` は、属性が無い場合も終了コード 0 を返す。
- 確認: GoReleaser v2.18.2 の snapshot build で生成した cask は、`postflight do` を含まず `postflight_steps` を含む。Homebrew 7.0.6 に読み込ませると、旧 cask は非推奨の error(`HOMEBREW_DEVELOPER=1` の下)になり、新 cask は `PostflightSteps` の artifact として読み込まれる。Homebrew の sandbox の下で `run` が属性を実際に外せるかは、次の release の後に `brew update && brew upgrade --cask slapex` で確かめ、非推奨の警告と Gatekeeper の警告が出ないことと合わせて本ログに残す。

## 後から見直す条件

- install script の保守コストや利用実態から、`curl | sh` 経路を縮小・変更する必要が出た場合。
- Homebrew cask の未署名 binary 体験や upgrade 経路に問題が出た場合。
- Windows 対応（0031）など配布 target が増えた場合の install script 拡張。
- GoReleaser が `*_steps` を出力する設定を提供した場合(`custom_block` からその設定へ移す)。
- `postflight_steps` の `run` が quarantine 属性を外せない(install が失敗する、または Gatekeeper の警告が出る)場合。
