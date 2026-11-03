# 0041 導入手段の拡充（install script と Homebrew tap）

- 状態: decided
- 作成日: 2026-06-22
- 最終更新日: 2026-11-03
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

2026-10-17 に v1.3.0 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.3.0"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合と `slapex --version` が `slapex 1.3.0` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `brew upgrade slapex`(auto-update を含む)が `1.2.1 -> 1.3.0` の upgrade に成功し、`slapex --version` が `slapex 1.3.0` を返すことを確認した。

2026-11-03 に v1.4.0 release workflow でも GoReleaser から `kiyohara/homebrew-tap` の `Casks/slapex.rb` が `version "1.4.0"` へ自動更新されることを確認した。GitHub Release は公開済みで、darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt` が添付されている。Linux asset は dev コンテナ上で checksum 照合(amd64 / arm64)と `slapex --version` が `slapex 1.4.0` を返すことを確認した。さらに、ユーザー手元の Homebrew 環境で `brew upgrade slapex` が `1.3.0 -> 1.4.0` の upgrade に成功し、`slapex --version` が `slapex 1.4.0` を返すことを確認した。

## 理由

- 案B は最小コストで「1 コマンド導入」を即時に届けられ、外部依存もない。
- 案A は導入体験を最も良くするが、ユーザー側インフラ（repo + token）が前提のため、案B と分離して進める方が価値提供が早い。
- 主経路（Releases 単一バイナリ）を維持し補助手段を足す形にすることで、配布方針の一貫性を保てる。
- cask 採用は GoReleaser の現行 publish 経路に合わせるため。formula の方が CLI ツールとして直感的な面はあるが、deprecated な設定へ新規に寄せるより、現行経路へ合わせる判断を優先する。
- quarantine hook は未署名 binary の導入体験を Homebrew 経由で成立させるための暫定措置。署名・公証を行うのが最も正攻法だが、現時点では配布コストを抑える判断を優先する。

## 影響

- 実装: `scripts/install.sh`（POSIX sh、OS / arch 判定・checksum 検証・install 先指定・`--version` / `--bin-dir` / `--dry-run` / `--help`）と `scripts/install_test.sh`（検出マッピングのテスト）を追加（案B）。
- ドキュメント: `README.md` のインストール節にクイックインストール（案B）を追加し、手動手順を詳細版として残す。`doc/design/architecture.md` の配布方式に install script を追記。
- 案A: 専用 tap repo の作成と write token の Actions secret 追加（ユーザー作業）、goreleaser 設定（`homebrew_casks`）、未署名 binary の Gatekeeper warning 対策として cask install 後に quarantine 属性を外す hook を入れる構成で実装済み。v1.0.1 / v1.1.0 / v1.1.1 / v1.1.2 / v1.2.0 / v1.2.1 / v1.3.0 / v1.4.0 release で release workflow から tap repo への cask 自動更新と、Homebrew 経由の upgrade を確認済み。
- decision log: `index.md` の未決事項から「Homebrew tap」を移し、本ログを現在有効な主要方針に追加。0034 に関連を追記。

## 追記(2026-10-01): `postflight` の非推奨化への対応

2026-09-25、Homebrew 7.0.6 で同じ tap の別の Formula を install したとき、slapex の cask の `postflight` について「Calling `postflight` is deprecated! Use `postflight_steps` instead.」の警告が出た(Issue #239)。slapex の cask を操作していない場面でも出ており、Homebrew は tap への報告を求めていた。

- Homebrew は 7.0.0(2026-09-13)から、cask の `preflight` / `postflight` / `uninstall_preflight` / `uninstall_postflight` を非推奨(警告。`HOMEBREW_DEVELOPER=1` の下では error)にしている。6.0.16(2026-08-10)で置いたコメントの placeholder を、7.0.0 で有効にした。代わりの `*_steps` は、リテラル引数の step(`run`、`remove`、`on_macos` など)だけを並べる宣言的な DSL で、sandbox の中で実行される。
- Homebrew の方針(Deprecating, Disabling and Removing)は、非推奨にした public API を次の minor または major release で disabled(全利用者に error)にするとしており、7.0.0 の次の minor または major release(7.1.0 など)がこれに当たる。日付は公表されていないが、Homebrew は前の minor / major release から 1 か月未満では次の minor / major release を作らないため(`docs/Releases.md`)、早くても 2026-10-13 ごろになる。disabled になると、`brew install --cask slapex` と `brew upgrade --cask slapex` が失敗する。
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

この変更が tap の cask に届くのは、merge の後に slapex を release し、GoReleaser が `Casks/slapex.rb` を作り直したときである。それまで tap の cask は v1.2.1 の `postflight` のままで、Homebrew が disabled にした後は install と upgrade が失敗する。そのため、Homebrew が disabled にする前(早くても 2026-10-13 ごろ)に release する必要がある。release の時期は、本ログとは別に決める。

理由:

- 案1 は、GoReleaser の対応より先に Homebrew が disabled にすると install と upgrade が壊れ、その時期を slapex の側で制御できない。
- 案2 は、今の GoReleaser のまま次の release で警告を無くせ、quarantine 属性を外す処理も残る。
- 案3 は、Homebrew の利用者の初回実行で Gatekeeper の警告を出す。案4 は Apple Developer Program の維持コストを伴い、本ログの決定(必要になった時点で別途判断する)を変える理由にならない。

実装上の注意:

- `custom_block` は cask の先頭(`version` の前)に出力される。Homebrew の stanza の順序の規約から外れるが、規約を強制するのは公式 tap の audit と style であり、第三者 tap の install には影響しない。
- 旧 hook は `Dir["#{staged_path}/slapex_darwin_*"]` で binary を探していたが、`run` は glob も shell も展開しない。staged_path には download した binary だけが置かれるため、`chdir: "."`(staged_path が基準になる)で staged_path の全体から属性を再帰的に外す。Homebrew の `{{staged_path}}` の token は、GoReleaser の template と区切りが衝突するため使わない。
- 旧 hook の `system_command` は、失敗すると install を止めていた(`must_succeed: true`)。`run` の既定も同じため、`must_succeed` は指定しない。`xattr -dr` は、属性が無い場合も終了コード 0 を返す。
- 新しい cask を読み込めるのは、Homebrew 6.0.13(2026-07-27)以降である。`postflight_steps` は 5.1.14、steps の `on_macos` は 6.0.12、`run`(`chdir:` を含む)は 6.0.13 で入った。それより前の Homebrew では cask の評価が失敗し、Homebrew を上げるまで install と upgrade ができない。旧 cask にはこの制約が無かった。`brew update` は Homebrew と tap を一緒に上げるため影響は小さいが、`HOMEBREW_NO_AUTO_UPDATE=1` のまま新しく tap を入れる場合などに起こりうる。
- 確認: GoReleaser v2.18.2 の snapshot build で生成した cask は、`postflight do` を含まず `postflight_steps` を含む。Homebrew 7.0.6 に読み込ませると、旧 cask は非推奨の error(`HOMEBREW_DEVELOPER=1` の下)になり、新 cask は `PostflightSteps` の artifact として読み込まれる。Homebrew の sandbox の下で `run` が属性を実際に外せるかは、次の release の後に `brew update && brew upgrade --cask slapex` で確かめ、非推奨の警告と Gatekeeper の警告が出ないことと合わせて本ログに残す。
- 確認(2026-10-17、v1.3.0): tap の `Casks/slapex.rb` は `postflight do` を含まず、`postflight_steps` を含む。ユーザー手元の Homebrew 7.0.9 で `brew upgrade slapex` が成功し、出力に `postflight` の非推奨の警告は出なかった。upgrade 後の `Caskroom/slapex/1.3.0/slapex_darwin_arm64` に `com.apple.quarantine` の属性は残っておらず、Homebrew の sandbox の下でも `run` が属性を外せている。upgrade 後の `slapex --version` は Gatekeeper に止められずに実行でき、`slapex 1.3.0` を返した。2026-10-01 に挙げた確認項目(非推奨の警告と Gatekeeper の警告)は、どちらも出ないことを確認した。

## 追記(2026-11-03): GoReleaser の `install_steps` への移行

GoReleaser v2.19.0(2026-10-22)が、`homebrew_casks` の `hooks.{pre,post}.{install,uninstall}_steps` を足した(goreleaser/goreleaser#6873。2026-10-12 に merge され、issue goreleaser/goreleaser#6870 も close された)。`hooks.post.install_steps` は `postflight_steps` を出力し、生の Ruby の `install` / `uninstall` は非推奨になった。template の field の `.StagedPath` / `.AppDir` は、Homebrew の `{{staged_path}}` / `{{appdir}}` の token に展開される。2026-10-01 の追記で見直す条件に挙げた「GoReleaser が `*_steps` を出力する設定を提供した場合」に当たる(Issue #300)。release workflow は `~> v2` で最新の v2 を使うため、v2.19.0 で動き、workflow の変更は要らない。

候補:

- 案A: `custom_block` の step(`chdir: "."` で staged_path を基準にする)を、そのまま `hooks.post.install_steps` に移す。
- 案B: `hooks.post.install_steps` に移し、staged_path を `{{ .StagedPath }}` で引数に渡す。GoReleaser の文書の例と同じく `{{ .StagedPath }}` を使う。ただし文書の例は binary 1 つの path(`{{ .StagedPath }}/foo`)を渡すのに対し、案B は staged_path の全体を渡す(実装上の注意)。

決定: 案B を採る。`.goreleaser.yaml` の `homebrew_casks` から `custom_block` を外し、次を置く。

```yaml
hooks:
  post:
    install_steps: |
      on_macos do
        run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{ .StagedPath }}"]
      end
```

生成される cask では、`depends_on` の後に次が出る。

```ruby
postflight_steps do
  on_macos do
    run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}"]
  end
end
```

理由:

- 公式の設定に移すと、`custom_block` の不利(stanza の位置が先頭になる、token の区切りの衝突を避ける回避が要る、GoReleaser 側で cask の生成が変わっても追随しない)が無くなる。
- 案A は v1.3.0 で実際に属性を外せた形をそのまま残せるが、`chdir: "."` は token を避けるための回避であり、`.StagedPath` の field がある今は要らない。案B は token の回避をやめ、GoReleaser の文書と同じく `.StagedPath` で path を渡す構成に戻る。
- 処理は変わらない。macOS だけで、staged_path の全体から `com.apple.quarantine` を再帰的に外し、失敗したら install を止める。案A と案B で、`xattr` に渡す対象は同じ directory である。
- Homebrew は 6.0.13 から、`run` の引数の `{{staged_path}}` を staged_path の絶対 path に展開する(`Library/Homebrew/install_steps.rb` の `expand_template_tokens`)。新しい cask を読み込める Homebrew の下限(6.0.13)は変わらない。

実装上の注意:

- 引数は staged_path の全体とし、GoReleaser の文書の例のような binary の path(`{{ .StagedPath }}/slapex` など)にしない。staged_path に置かれる file の名前は arch ごとに変わり(`slapex_darwin_arm64` / `slapex_darwin_amd64`。`binary` の `target:` の `slapex` は link の名前である)、`run` は glob を展開しないためである。存在しない path を渡すと `xattr` は失敗し得て、その場合は `run` の既定(`must_succeed`)により install が止まる。
- GoReleaser は `install_steps` に field ごとの template を適用せず、生成した cask の全体に 1 回だけ template を適用する(v2.19.0 の `internal/pipe/cask/cask.go`)。`.StagedPath` / `.AppDir` 以外の Homebrew の token(`{{version}}` など)を書く場合は、`{{ "{{version}}" }}` のように quote する。
- 確認(2026-11-03): GoReleaser v2.19.0 の `check` が通る。`release --snapshot --clean` で生成した cask は、`postflight do` も先頭の `postflight_steps` も含まず、上記の `postflight_steps` を `depends_on :macos` の後に含む。非推奨の通知は出ない。Homebrew 7.0.12(使い捨ての Linux の container。install と tap の変更はしない)に、移行前と移行後の生成物の内容を読み込ませると、どちらも `PostflightSteps` の artifact として読み込まれ、`HOMEBREW_DEVELOPER=1` の下でも error にならない。macOS を模して step を実行させる(command を記録するだけで実行しない)と、移行前は `chdir` が staged_path で引数が `.`、移行後は引数が staged_path の絶対 path の `/usr/bin/xattr -dr com.apple.quarantine` になり、どちらも失敗したら install を止める(`must_succeed` が true)。Linux を模すと実行されない。
- 案B の形が Homebrew の sandbox の下で属性を外せるかは、次の release の後に `brew update && brew upgrade --cask slapex` で確かめ、非推奨の警告と Gatekeeper の警告が出ないこと、`slapex --version` が新しい version を返すことと合わせて本ログに残す。
- 確認(2026-11-03、v1.4.0): tap の `Casks/slapex.rb` は `custom_block` 由来の先頭の `postflight_steps` を含まず、`depends_on :macos` の後に `run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}"]` の `postflight_steps` を含む。ユーザー手元の Homebrew 7.0.12 で `brew upgrade slapex` が成功し、出力に非推奨の警告は出なかった。upgrade 後の `Caskroom/slapex/1.4.0/slapex_darwin_arm64` に `com.apple.quarantine` の属性は残っておらず、案B の形でも Homebrew の sandbox の下で `run` が属性を外せている。upgrade 後の `slapex --version` は Gatekeeper に止められずに実行でき、`slapex 1.4.0` を返した。上に挙げた確認項目(非推奨の警告、Gatekeeper の警告、新しい version)は、いずれも問題が無いことを確認した。

## 後から見直す条件

- install script の保守コストや利用実態から、`curl | sh` 経路を縮小・変更する必要が出た場合。
- Homebrew cask の未署名 binary 体験や upgrade 経路に問題が出た場合。
- Windows 対応（0031）など配布 target が増えた場合の install script 拡張。
- GoReleaser が `*_steps` を出力する設定を提供した場合(`custom_block` からその設定へ移す。Issue #300)。扱い済み: 2026-11-03 に v2.19.0 の `hooks.post.install_steps` へ移した(2026-11-03 の追記)。
- `postflight_steps` の `run` が quarantine 属性を外せない(install が失敗する、または Gatekeeper の警告が出る)場合。
