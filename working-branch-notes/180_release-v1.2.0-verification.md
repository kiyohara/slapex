# 作業ブランチメモ

- ブランチ: release-v1.2.0-verification
- PR: #180
- 最終更新: 2026-07-13

## 目的

v1.2.0 公開後の release 検証結果を `progress.md` と decision log 0041 に記録する。

## 現在の状況

- release tag: `v1.2.0`
- release target: `efda9fcc84d12ad71dd4350efad4b32909717d2a`
- release workflow: run `29243080813` / success
- GitHub Release: published
- assets: darwin / linux × amd64 / arm64 の 4 binary と `slapex_checksums.txt`
- 検証結果記録 PR: #180

## 決定事項

- 配布方式や導入手段の方針変更はない。
- v1.2.0 でも Homebrew cask 自動更新が機能した確認結果として、既存 decision log 0041 に追記する。

## 次にやること

- ユーザーが PR #180 を merge する。

## 検証

- Release workflow: run `29243080813` / success。
- GitHub Release: published、draft / prerelease ではない。
- Release assets: `slapex_darwin_amd64`、`slapex_darwin_arm64`、`slapex_linux_amd64`、`slapex_linux_arm64`、`slapex_checksums.txt` が添付済み。
- Linux asset checksum: `slapex_linux_amd64: OK`。
- Linux `--version`: `slapex 1.2.0`。
- Homebrew tap: `Casks/slapex.rb` が `version "1.2.0"`。
- ユーザー手元の macOS / Homebrew: upgrade 前 `slapex 1.1.2`、upgrade 後 `slapex 1.2.0`。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-07-13: v1.2.0 tag push 後の release workflow / assets / checksum / Linux version / Homebrew cask を確認。
- 2026-07-13: ユーザー手元の macOS / Homebrew upgrade を確認。
