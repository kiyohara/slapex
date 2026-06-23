# 作業ブランチメモ

- ブランチ: issue-50-homebrew-cask
- PR: #80
- 最終更新: 2026-06-24

## 目的

Issue #50 として、macOS 向け Homebrew tap 経由の導入を提供する。GoReleaser の現行方針に合わせ、formula ではなく cask に寄せる。

## 現在の状況

- `kiyohara/homebrew-tap` はユーザーが作成済み。
- `kiyohara/homebrew-tap` は `gh` 経由で README と v1.0.0 用 `Casks/slapex.rb` を整備済み。Gatekeeper warning 対策として cask `postflight` で quarantine 属性を外す hook も反映済み。
- tap repo へ書き込む token と slapex 本体 repo の Actions secret `HOMEBREW_TAP_GITHUB_TOKEN` はユーザーが設定済み。GitHub 上で secret 名の存在を確認済み。
- v1.0.0 は既に公開済みのため、tap repo へ v1.0.0 用 cask を手動 bootstrap 済み。
- GitHub Issue #50 の本文は cask 方針へ更新済み。
- 次回 release workflow での cask 自動更新確認は Issue #79 に切り出し済み。

## 決定事項

- Homebrew 対応は GoReleaser の `homebrew_casks` を使う。
- cask 名は `slapex` とし、tap repo は `kiyohara/homebrew-tap`、配置先は `Casks/slapex.rb` とする。
- release workflow では tap repo 書き込み用 secret として `HOMEBREW_TAP_GITHUB_TOKEN` を参照する。
- bootstrap は v1.0.0 用 cask を tap repo に手動追加する方針。
- 未署名 binary のため、短期対処として cask install 後に `com.apple.quarantine` を外す。長期的に必要が出た場合は Developer ID 署名 + notarization を検討する。

## 次にやること

- #80 のレビューと merge 判断をユーザーが行う。
- 次回 release 時に Issue #79 として GoReleaser release 経路と tap repo 更新を検証する。

## 検証

- `docker run --rm -v /Users/kiyohara/projects/slapex:/src -w /src goreleaser/goreleaser:v2.16.0 check`: pass。
- `docker run --rm -v /Users/kiyohara/projects/slapex:/src -w /src goreleaser/goreleaser:v2.16.0 release --snapshot --clean`: pass。`dist/homebrew/Casks/slapex.rb` が生成され、cask は darwin/amd64 と darwin/arm64 のみを参照することを確認。
- `docker compose run --rm --no-deps dev go test ./...`: pass。
- v1.0.0 の公開 checksum を取得し、bootstrap cask 用の sha256 を確認。
- `/opt/homebrew/bin/gh` で `kiyohara/homebrew-tap` の README 更新 commit `6ede7db` と cask bootstrap commit `ae4e16d` を作成し、反映後の repo contents を確認。
- `ruby -c /private/tmp/slapex-homebrew-tap-slapex.rb`: pass。
- `/opt/homebrew/bin/gh secret list -R kiyohara/slapex`: `HOMEBREW_TAP_GITHUB_TOKEN` の存在を確認。secret 値と実 token 権限は GitHub から読み出せないため未確認。
- Homebrew cask 自動更新経路の release 検証は Issue #79 として作成済み。
- `slapex --version` 実行時の Gatekeeper warning を受け、`.goreleaser.yaml` の `homebrew_casks.hooks.post.install` を追加。`docker run --rm -v /Users/kiyohara/projects/slapex:/src -w /src goreleaser/goreleaser:v2.16.0 check`: pass。
- 同 hook 追加後の `docker run --rm -v /Users/kiyohara/projects/slapex:/src -w /src goreleaser/goreleaser:v2.16.0 release --snapshot --clean`: pass。生成 cask に `postflight` が出力されることを確認。
- `ruby -c dist/homebrew/Casks/slapex.rb`: pass。
- `/opt/homebrew/bin/gh` で `kiyohara/homebrew-tap` の v1.0.0 bootstrap cask に同じ `postflight` を反映。commit `5f69db6`。
- ユーザー手元で `brew reinstall --cask slapex` 後の `slapex --version`: pass。出力は `slapex 1.0.0`。

## v1.0.0 bootstrap cask

tap repo 側の `Casks/slapex.rb` に手動追加する内容:

```rb
cask "slapex" do
  version "1.0.0"

  on_macos do
    on_intel do
      sha256 "441d271371002b640e7914f4b91bae4385b3342987de65de80f74790d9914279"
      url "https://github.com/kiyohara/slapex/releases/download/v1.0.0/slapex_darwin_amd64"
      binary "slapex_darwin_amd64", target: "slapex"
    end
    on_arm do
      sha256 "3967d95d57fc808ba0bcc8e55cadb74c465374ec5e8defb61ba252d70ee7a20b"
      url "https://github.com/kiyohara/slapex/releases/download/v1.0.0/slapex_darwin_arm64"
      binary "slapex_darwin_arm64", target: "slapex"
    end
  end

  name "slapex"
  desc "Export Slack channel posts and assets as local static HTML"
  homepage "https://github.com/kiyohara/slapex"

  livecheck do
    skip "Auto-generated on release."
  end

  postflight do
    if OS.mac?
      Dir["#{staged_path}/slapex_darwin_*"].each do |binary|
        system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", binary]
      end
    end
  end
end
```

## リスク・ブロッカー

- `HOMEBREW_TAP_GITHUB_TOKEN` の値や実 token 権限は読み出せないため、release workflow から tap repo へ実際に publish する検証は次回 release 時に Issue #79 で行う。
- cask は未署名 binary を扱うため、短期的に quarantine 属性を外す hook で対処する。Developer ID 署名 + notarization は必要性が出たら別途判断する。

## セッションログ

- 2026-06-24: Issue #50 と関連 docs を確認。ユーザー方針として formula ではなく cask 採用、v1.0.0 bootstrap 実施、tap repo 作成済み、token 未準備を確認。`issue-50-homebrew-cask` ブランチを作成。
- 2026-06-24: `.goreleaser.yaml` / release workflow / README / architecture / decision log / progress / working branch note を更新。GoReleaser check と snapshot build で cask 生成を確認。Issue #50 本文を cask 方針へ同期。
- 2026-06-24: ユーザー指定に従い、`kiyohara/homebrew-tap` は `github-op-integrated` MCP を使わず `/opt/homebrew/bin/gh` で操作。repo description を設定し、README と v1.0.0 用 cask を main に反映。
- 2026-06-24: ユーザーが PAT 発行と `kiyohara/slapex` repo の Actions secret 設定を完了。`gh secret list` で `HOMEBREW_TAP_GITHUB_TOKEN` の存在を確認。
- 2026-06-24: release workflow から tap repo へ実際に publish する検証は次回 release 時でないと確定できないため、Issue #79 に切り出し、`progress.md` に追記。
- 2026-06-24: Homebrew cask install 後の `slapex --version` で Gatekeeper warning が出たため、GoReleaser 設定と tap repo bootstrap cask に quarantine removal hook を追加。
- 2026-06-24: ユーザー手元で再検証し、`slapex --version` が `slapex 1.0.0` を返すことを確認。
