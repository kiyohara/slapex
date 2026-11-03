# 作業ブランチメモ

- ブランチ: `claude/happy-ptolemy-tvj3an`(Issue の推奨ブランチ名は `use-goreleaser-cask-steps-hooks`。cloud session のため platform のブランチを使う)
- PR: (未採番)
- 最終更新: 2026-11-03

## 目的

Issue #300。PR #299(Issue #239)で暫定措置とした GoReleaser の `custom_block` による `postflight_steps` の出力を、GoReleaser の公式の設定へ移す。quarantine 属性を外す処理は変えない。

Claude Code on the web の cloud session で、`drive-issue-to-reviewed-pr` の P1(`run-issue-task`)として進めている。

## 現在の状況

- main `c41d10f` から作業している。
- `.goreleaser.yaml` の変更と decision log 0041(追記、見直す条件、index の行)の更新を済ませ、検証した。PR の作成前。

## 決定事項

- 前提の確認(Issue の「依存・順序」):
  - PR #299 は merge 済み(`5871539`)。v1.3.0 の release 後の確認で、`postflight_steps` の `run` が属性を外せることも確かめ済み(decision log 0041 の 2026-10-17 の確認)。先に行う対応は無い。
  - goreleaser/goreleaser#6873 は 2026-10-12 に merge された(`4ebadf5`)。issue goreleaser/goreleaser#6870 も同日に close された(milestone v2.19.0)。
  - merge された変更は v2.19.0(2026-10-22 公開、"Latest")の tag に含まれる。v2.18.3 には含まれない。tag の `pkg/config/config.go`(`HomebrewCaskHook.InstallSteps` / `UninstallSteps`)と `internal/pipe/cask/templates/cask.rb`(`flight "postflight" .Hooks.Post.InstallSteps ...`)と release notes で確かめた。v2.19.0 の後の main で cask に入った変更は `zap :not_required` の出力だけで、`_steps` とは関係が無い。
  - release workflow は `~> v2` のため、v2.19.0 を使う。workflow は変えない。
- 採用された設定(v2.19.0 の文書と source):
  - 名前と書式: `homebrew_casks[].hooks.post.install_steps` に steps の DSL を文字列で書く。`postflight_steps do ... end` として出力され、cask の stanza の順序どおりの位置(`service` の後、completions の前)に出る。生の Ruby の `install` / `uninstall` は非推奨(`deprecate.Notice`)。両方を書くと `_steps` が優先される。
  - template: `install_steps` は field ごとの template の適用を受けず、生成した cask の全体に 1 回だけ template が適用される(`cask.go` の `WithExtraFields(caskTokens)`)。`.StagedPath` / `.AppDir` は `{{staged_path}}` / `{{appdir}}` に展開される。それ以外の Homebrew の token は `{{ "{{version}}" }}` のように quote する。
  - Homebrew の token の扱い: Homebrew は 6.0.13 から、`run` の引数の `{{staged_path}}` を staged_path の絶対 path に展開する(6.0.13 と 7.0.12 の `Library/Homebrew/install_steps.rb` の `expand_template_tokens` と `template_token_value`、`CONTENT_PATH_TOKENS` に `staged_path` を含む)。cask を読み込める Homebrew の下限(6.0.13)は変わらない。
- 書き方: GoReleaser の文書の例と同じく、staged_path を `{{ .StagedPath }}` で引数に渡す(decision log 0041 の 2026-11-03 の追記の案B)。`chdir: "."` の形をそのまま移す案A も比べた。案A は v1.3.0 で実機確認済みの形を残せるが、`chdir: "."` は token の衝突を避ける回避であり、Issue の評価(公式の設定に移すと token の回避が要らなくなり、GoReleaser の文書どおりの構成に戻る)に合わせて案B にした。`xattr` に渡す対象は同じ directory で、macOS だけ、再帰的、失敗したら install を止める点も変わらない。
- `progress.md` は変えない(#300 は索引に無い単発 Issue)。

### 出力生成系 skill の判断

- `update-sample-exports`: 使わない。出力 HTML / CSS / assets、fixture、`tools/gensample` は変わらない。
- `update-readme-preview-screenshots`: 使わない。README の出力プレビューに映るものは変わらない。
- `update-readme-demo-gif`: 使わない。CLI の出力と操作フローは変わらない。

## 次にやること

- PR を作り、note を採番する。
- 次の release の後に、`brew update && brew upgrade --cask slapex` で非推奨の警告と Gatekeeper の警告が出ないこと、`slapex --version` が新しい version を返すことを確かめ、decision log 0041 に残す(release の作業で行う)。

## 検証

cloud session で実行した。GoReleaser は Docker Compose の `dev` service(`golang:1.26` の mirror)で `go run` した。v2.19.0 は Go 1.27.3 を要求するため、`-e GOTOOLCHAIN=auto` を付け、container の中で toolchain を取得させた(`.goreleaser.yaml` の `env` と同じ設定)。

- `docker compose run --rm -e GOTOOLCHAIN=auto dev go run github.com/goreleaser/goreleaser/v2@v2.19.0 check`: pass(1 configuration file validated)。変更の前と後の両方で実行した。
- `docker compose run --rm -e GOTOOLCHAIN=auto dev go run github.com/goreleaser/goreleaser/v2@v2.19.0 release --snapshot --clean`: pass。変更の前と後で `dist/homebrew/Casks/slapex.rb` を比べた。
  - 変更の前: `cask "slapex" do` の直後(`version` の前)に `custom_block` 由来の `postflight_steps`(`args: ["-dr", "com.apple.quarantine", "."], chdir: "."`)。
  - 変更の後: 先頭の `postflight_steps` は無く、`depends_on :macos` の後に公式の設定による `postflight_steps do` / `on_macos do` / `run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}"]` が出る。`postflight do` は無い。非推奨の通知は出ない。
  - 他の違いは sha256 だけ。作業ツリーの変更が binary の VCS 情報に入るためで、cask の構成とは関係が無い。tag が無い shallow clone のため、version は `0.0.1-snapshot` になる。
- Homebrew での読み込み(cloud session のため host の Homebrew は無い。使い捨ての container で行い、install と tap の変更はしない):
  - 方法: `homebrew/brew` の image(mirror。Homebrew 4.6.20)の Homebrew を 7.0.12 の tag に切り替えた。7.0.12 が要求する portable Ruby 4.0.9 の取得は、配信元(ghcr.io)が proxy に拒否された(403)。迂回はせず、checkout を `ruby:4.0` の image(mirror。Ruby 4.0.9)の container に移し、`HOMEBREW_USE_RUBY_FROM_PATH=1` と `HOMEBREW_DEVELOPER=1` で system の Ruby を使わせた。container は `--network none` で動かした。scratch の Ruby script で `Cask::CaskLoader::FromContentLoader` に生成物の内容を読み込ませ、`Homebrew::InstallSteps::Runner` に command を記録するだけの class を渡して step を実行させた(`Homebrew::SimulateSystem` で macOS と Linux を模した)。
  - 移行前と移行後の cask: どちらも読み込め、artifact は `PostflightSteps`(Linux のため `binary` は `on_macos` で外れる)。`HOMEBREW_DEVELOPER=1` の下でも error にならない。
  - 移行前: step は `run`、command は `/usr/bin/xattr`、args は `["-dr", "com.apple.quarantine", "."]`、chdir は base `staged_path` の `.`、guard は `on macos`。macOS を模すと、chdir が staged_path の絶対 path、args が `.` で実行され、`must_succeed` は true。
  - 移行後: step は `run`、command は `/usr/bin/xattr`、args は `["-dr", "com.apple.quarantine", "{{staged_path}}"]`、chdir は無し、guard は `on macos`。macOS を模すと、args の `{{staged_path}}` が staged_path の絶対 path に展開されて実行され、`must_succeed` は true。
  - Linux を模すと、どちらも実行されない。
- `git diff --check`: 問題なし。

未検証事項:

- 移行後の形(`{{staged_path}}` を引数に渡す)が、実際の macOS の Homebrew の sandbox の下で属性を外せるか。v1.3.0 で確かめたのは移行前の形(`chdir: "."`)である。次の release の後の確認で分かる。
- 実際の macOS の Homebrew での読み込み(cloud session には無い。Linux の container の Homebrew 7.0.12 で代えた)。

## リスク・ブロッカー

- 次の release の後の確認で `run` が失敗した場合は、案A(`chdir: "."`)へ戻すか、decision log 0041 の見直す条件に従って判断する。
- 新しい cask は、引き続き Homebrew 6.0.13 以降でないと読み込めない(下限は変わらない)。

## セッションログ

- 2026-11-03: Issue #300 の前提(goreleaser/goreleaser#6873 の merge と v2.19.0 への収録、PR #299 の release 後の確認)を確かめ、着手できると判定した。GoReleaser v2.19.0 と Homebrew 6.0.13 / 7.0.12 の source と文書で設定と token の扱いを確かめ、`.goreleaser.yaml` を `hooks.post.install_steps` に移した。decision log 0041(追記、見直す条件)と index の行を更新し、snapshot build と container の Homebrew での読み込みで確かめた。
