# 作業ブランチメモ

- ブランチ: release-v1.4.0-verification
- PR: 未作成
- 最終更新: 2026-11-03

## 目的

v1.4.0 公開後の release 検証結果を `progress.md` と decision log 0041 に記録する。

## 現在の状況

- release tag: `v1.4.0`(署名付き、`330e025` を指す)
- Release workflow: run `41595044885` / success
- 準備 PR #308 は merge 済み。検証記録は別ブランチ・別 PR で行う。

## 決定事項

- 配布方式・導入手段の方針自体は変えないため、新規 decision log は作らず 0041 へ追記する形にした(v1.0.1 以降の各 release と同じ扱い)。
- 0041 の 2026-11-03 の追記が「次の release の後に確かめ、本ログに残す」としていた `install_steps` 版(案B)の実地確認の結果を、同じ追記の「実装上の注意」の末尾に置いた。
- decision log index の 0041 要約と解決済みの旧未決事項は、確認した release を v1.4.0 に更新し、`install_steps` 版で quarantine 属性が外れることの確認を加えた。
- `progress.md` のリリース履歴の v1.4.0 の行を released にし、現況を更新した。

## 次にやること

- ユーザーが検証記録 PR を merge する。

## 検証

確認した項目は次のとおり。最後の 2 件はユーザー手元の macOS で確認した。

- Release workflow run `41595044885`: success。
- GitHub Release: published(draft / prerelease ではない)。
- assets 5 件: `slapex_darwin_amd64` / `slapex_darwin_arm64` / `slapex_linux_amd64` / `slapex_linux_arm64` / `slapex_checksums.txt`。
- Linux checksum: dev コンテナ上で `slapex_linux_amd64: OK` / `slapex_linux_arm64: OK`。
- Linux `--version`: dev コンテナ(aarch64)上で `slapex_linux_arm64` が `slapex 1.4.0`。
- Homebrew tap: `Casks/slapex.rb` が `version "1.4.0"`。`postflight do` も `custom_block` 由来の先頭の `postflight_steps` も含まず、`depends_on :macos` の後に引数 `{{staged_path}}` の `postflight_steps` を含む。cask の darwin sha256 が Release の `slapex_checksums.txt` と一致することも確認した。
- release asset と cask の download は public asset のため認証情報を付けずに実施した(`doc/guidelines/credential-scope-guidelines.md`)。
- ユーザー手元の macOS / Homebrew 7.0.12: `brew upgrade slapex` で `1.3.0 -> 1.4.0` に upgrade し、非推奨の警告は出なかった。`slapex --version` は `slapex 1.4.0`。
- 同じ macOS で、upgrade 後の cask の binary に `com.apple.quarantine` の属性が残っていないことを `xattr -l` で確認した。

## リスク・ブロッカー

- 検証は完了。残るブロッカーは無い。
- Release workflow の annotation に、Node.js 20 の action の非推奨と、`ubuntu-latest` の Ubuntu 26 への移行の予告が出ている。今回の release には影響しない。

## セッションログ

- 2026-11-03: `release` skill に従い、tag push 後の workflow 監視と公開物検証を実施。ユーザー手元の Homebrew upgrade 確認結果を受け取り、decision log 0041 / index / `progress.md` に記録。
