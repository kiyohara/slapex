# 作業ブランチメモ

- ブランチ: `replace-cask-postflight-hook`
- PR: #299
- 最終更新: 2026-10-01

## 目的

Issue #239。Homebrew が cask の `postflight` を非推奨にし、tap の `Casks/slapex.rb` の `postflight` について警告を出すようになった。GoReleaser が生成する cask から `postflight` を無くし、quarantine 属性を外す処理を `postflight_steps` で残す。

ローカルの Mac の Claude Code で、`drive-issue-to-reviewed-pr` の P1(`run-issue-task`)として進めている。

## 現在の状況

- main `d21b6fe` から作業している。
- `.goreleaser.yaml` の変更と decision log 0041 の追記を済ませ、検証した。PR #299 を作り、note を採番した。次は P2(review)。

## 決定事項

- Issue の作業内容 1 の確認結果:
  - GoReleaser は最新の v2.18.2(2026-09-17)でも `homebrew_casks.hooks` を `postflight do ... end` として出力する。`*_steps` を出力する設定は無い(v2.18.2 の `internal/pipe/cask/templates/cask.rb` と `pkg/config/config.go` で確認)。対応する PR goreleaser/goreleaser#6873 は open、別案の #7156 は merge されずに close。
  - Homebrew の Cask Cookbook の `*flight_steps` は、リテラル引数の step だけを並べる宣言的な DSL。`run` は shell も glob も展開せず、既定で失敗時に install を止める。相対 path(`chdir:` を含む)は staged_path が基準。
  - Homebrew は 6.0.16(2026-08-10)から flight block を非推奨にしている(tag ごとの `Library/Homebrew/cask/dsl.rb` で確認)。方針上は次の minor / major release で disabled にするが、7.0.0 では disabled にならず、時期は公表されていない。
- 未決事項 1 の決め直し: Homebrew が disabled にする時期を slapex の側で制御できないため、仮決め(hook を残して待つ)ではなく、GoReleaser の `custom_block` で `postflight_steps` を出力する案をユーザーが選んだ(2026-10-01)。比べた案と理由は decision log 0041 の 2026-10-01 の追記に残した。
- `run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "."], chdir: "."` とする。
  - 旧 hook の glob(`slapex_darwin_*`)は `run` で書けない。staged_path には binary だけが置かれるため、staged_path の全体を対象にする。
  - Homebrew の `{{staged_path}}` の token は、GoReleaser の template(field ごとの適用と、生成物全体への適用)と区切りが衝突するため避けた。`custom_block` は field ごとの template の適用を受けないが、生成物全体への適用は受ける。
  - 旧 hook の `system_command` は `run!`(`must_succeed: true`)で、`run` の既定と同じ。`must_succeed` は指定しない。
- `progress.md` は変えない(#239 は索引に無い単発 Issue。PR #248 の補足)。

### 出力生成系 skill の判断

- `update-sample-exports`: 使わない。出力 HTML / CSS / assets、fixture、`tools/gensample` は変わらない。
- `update-readme-preview-screenshots`: 使わない。README の出力プレビューに映るものは変わらない。
- `update-readme-demo-gif`: 使わない。CLI の出力と操作フローは変わらない。

## 次にやること

- PR を作り、note を採番する。(完了)
- 次の release の後に、`brew update && brew upgrade --cask slapex` で非推奨の警告と Gatekeeper の警告が出ないこと、`slapex --version` が新しい version を返すことを確かめ、decision log 0041 に残す(release の作業で行う)。

## 検証

- `docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:v2.18.2 check`: pass(1 configuration file validated)。
- `docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:v2.18.2 release --snapshot --clean`: pass。`dist/homebrew/Casks/slapex.rb` は `postflight do` を含まず、`cask "slapex" do` の直後に `postflight_steps do` / `on_macos do` / `run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "."], chdir: "."` を含む。tap の現行 cask(v1.2.1)と、version、sha256、URL の tag を揃えて比べると、違いは `postflight_steps` の追加と `postflight` の削除だけ。
- host の Homebrew 7.0.6 での読み込み(ユーザーの了承済み。install と tap の変更はしない):
  - 方法: scratch の Ruby script で `Cask::CaskLoader::FromContentLoader` に cask の内容を読み込ませ、`brew ruby` で実行した。`Cask::CaskLoader.load` に path を渡すと「Homebrew requires casks to be in a tap」で拒否されるため。
  - 旧 cask(tap の現行 `Casks/slapex.rb`、v1.2.1): `MethodDeprecatedError: Calling postflight is deprecated! Use postflight_steps instead.`(`HOMEBREW_DEVELOPER=1` の下では警告ではなく error になる)。
  - 新 cask(snapshot の生成物): 読み込めた。artifact は `Binary` と `PostflightSteps`。step は `run`、command は `/usr/bin/xattr`、args は `["-dr", "com.apple.quarantine", "."]`、chdir は base `staged_path` の `.`、guard は `on macos`。`allow_failure` は付かない(失敗時は install を止める)。
  - 注意: 最初の試行で `brew ruby` を `HOMEBREW_DEVELOPER` なしで実行し、Homebrew の developer mode が自動で有効になった。直後に `brew developer off` で戻した(`homebrew.devcmdrun` は未設定に戻り、`brew developer` は disabled を返す。`/opt/homebrew` の git status に変更は無い)。以後は `HOMEBREW_DEVELOPER=1` を 1 回の実行にだけ付け、設定を書き換えずに実行した。
- `xattr -dr com.apple.quarantine` の終了コード(scratch のファイルで確認): 属性の無いファイルと directory で 0、属性のある directory で 0 を返し、属性が消えた。
- `git diff --check`: 問題なし。

未検証事項:

- Homebrew の sandbox の下で `run` が staged_path の属性を実際に外せるか。snapshot の cask の install は、ユーザーの Homebrew の状態を変えるため行っていない。次の release の後の確認で分かる。外せなかった場合、`run` の失敗で install が止まるか、Gatekeeper の警告が出る。

## P1 の記録(drive-issue-to-reviewed-pr)

- PR: #299。検証結果は「検証」、出力生成系 skill の判断は「出力生成系 skill の判断」のとおり。
- `progress.md`: 更新しない(#239 は索引に無い単発 Issue)。
- `run-issue-task` の手順で使った skill の報告から引き上げた項目:
  - `number-working-branch-note`(commit `f06c139`、`draft_replace-cask-postflight-hook.md` → `299_replace-cask-postflight-hook.md`、push 成功。情報統制チェックで直した箇所は無い)。
    - 書き換えた行の一覧: note の `- PR: (未採番)` → `- PR: #299`(`PR:` 欄の記入)。note の「次にやること」の「PR を作り、note を採番する。」の行末に `(完了)`(完了タスク行)。PR description の `working-branch-notes/draft_replace-cask-postflight-hook.md` → `working-branch-notes/299_replace-cask-postflight-hook.md`(ファイル名参照の置換)。title は無し。
    - 触らずに残した行の一覧: note の「現在の状況」の「PR の作成前。」(状況を説明する stale 表現だが、定型の `PR 未作成` に当てはまらない)。skill の外で、orchestrator が P1 の記録と同じ commit で今の状態へ直した。
  - 出力生成系 3 skill: 呼ばなかった(「出力生成系 skill の判断」)。

## リスク・ブロッカー

- 次の release の後の確認で `run` が失敗した場合は、`must_succeed` や `writable_paths` の見直し、または hook を外す案へ戻る判断が要る(decision log 0041 の見直す条件)。
- GoReleaser が `*_steps` の設定を出したら、`custom_block` から移す(follow-up 候補)。

## セッションログ

- 2026-10-01: Issue #239 を読み、GoReleaser v2.18.2 と Homebrew の source と文書で `*_steps` への対応を確かめた。GoReleaser は未対応。ユーザーに方針を確認し、`custom_block` で `postflight_steps` を出力する案に決めた。`.goreleaser.yaml` と decision log 0041(追記、見直す条件、index の行)を更新し、snapshot build と host の Homebrew での読み込みで確かめた。
- 2026-10-01: PR #299 を作り、`number-working-branch-note` で note を採番した(`f06c139`)。P1 の記録を残した。
