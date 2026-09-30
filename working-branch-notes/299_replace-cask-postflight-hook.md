# 作業ブランチメモ

- ブランチ: `replace-cask-postflight-hook`
- PR: #299
- 最終更新: 2026-10-01

## 目的

Issue #239。Homebrew が cask の `postflight` を非推奨にし、tap の `Casks/slapex.rb` の `postflight` について警告を出すようになった。GoReleaser が生成する cask から `postflight` を無くし、quarantine 属性を外す処理を `postflight_steps` で残す。

ローカルの Mac の Claude Code で、`drive-issue-to-reviewed-pr` の P1(`run-issue-task`)として進めている。

## 現在の状況

- main `d21b6fe` から作業している。
- `.goreleaser.yaml` の変更と decision log 0041 の追記を済ませ、検証した。PR #299 を作り、note を採番した。P2 の review(指摘 3 件)に P4 で対応し、P5 の再確認で未対応 0 件になった。残りは人間の手番である。

## 決定事項

- Issue の作業内容 1 の確認結果:
  - GoReleaser は最新の v2.18.2(2026-09-17)でも `homebrew_casks.hooks` を `postflight do ... end` として出力する。`*_steps` を出力する設定は無い(v2.18.2 の `internal/pipe/cask/templates/cask.rb` と `pkg/config/config.go` で確認)。対応する PR goreleaser/goreleaser#6873 は open、別案の #7156 は merge されずに close。
  - Homebrew の Cask Cookbook の `*flight_steps` は、リテラル引数の step だけを並べる宣言的な DSL。`run` は shell も glob も展開せず、既定で失敗時に install を止める。相対 path(`chdir:` を含む)は staged_path が基準。
  - Homebrew は 7.0.0(2026-09-13)から flight block を非推奨(警告)にしている。6.0.16〜6.0.22 の `Library/Homebrew/cask/dsl.rb` はコメントの placeholder(`# odeprecated`)で、7.0.0 から有効になる(tag ごとに確認。当初 6.0.16 からとしたのは、grep がコメント行も数えたための誤りで、P2 の `[must]` で直した)。方針上は 7.0.0 の次の minor / major release(7.1.0 など)で disabled にする。日付は公表されていないが、前の minor / major release から 1 か月未満では作らないため、早くても 2026-10-13 ごろ。
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
- Homebrew が `postflight` を disabled にする前(早くても 2026-10-13 ごろ)に slapex を release する。release の時期はユーザーが決める(P2 の `[ask]`)。
- 人間の手番: 3 thread(`[must]`、`[ask]`、`[imo]`。いずれも修正確認済み)の resolve、PR #299 の merge、release の時期の判断。
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

## P2 / P3 の記録

- review cycle: `claude-code-c236958-20260930225828`。Reviewed head: `c2369580db11215bd0536de3f6b96d5658217f96`。完了要約は review body(pullrequestreview-5372958500)。
- 指摘 3 件(inline 3、top-level 0)。prefix ごとでは `[must]` 1、`[ask]` 1、`[imo]` 1、`[nits]` 0、`[fyi]` 0。
  - `[must]` decision log 0041 の追記(64〜65 行目): Homebrew が非推奨にした時期は 6.0.16 ではなく 7.0.0。disabled の見込みの書き方。note と PR description にも同じ記述。
  - `[ask]` 同(88 行目): 効果が届くのは slapex の release の後。Homebrew の次の minor release より前に release するか、その前提を残すか。
  - `[imo]` 同(96 行目): 新しい cask を読める Homebrew の下限(6.0.13)を残す。
- review は、生成した cask の steps を Homebrew の `Runner` で scratch の擬似 staged_path に対して実行し、quarantine 属性が消えることも確かめた(sandbox の外)。
- P3: 指摘が 3 件のため P4 へ進んだ。

## P4 の記録

- 処置: 採用し修正した 3 件。スコープ外とした指摘は無い。各指摘は tag ごとの Homebrew の source で裏を取った(`dsl.rb` の `odeprecated` の行は 6.0.16〜6.0.22 がコメント、7.0.0 から有効。`install_steps.rb` の `run` と `chdir` は 6.0.13 から、steps の `on_macos` は 6.0.12 から、`postflight_steps` は 5.1.14 から。`docs/Releases.md` の 1 か月の規定)。
  - `[must]`: decision log 0041 の追記の 2 項目、本 note の「決定事項」、PR description の概要を直した。
  - `[ask]`: 前提(disabled になる前に release する)を decision log 0041 の決定の直後に残した。release の時期は agent では決められないため、終了時の報告でユーザーに伝える。
  - `[imo]`: decision log 0041 の実装上の注意に、Homebrew の下限(6.0.13)と理由を足した。
- follow-up: review とは別に、ユーザーの指示で Issue #300(GoReleaser の公式の設定への移行)を起票した。decision log 0041 の見直す条件、本 note、PR description の follow-up 候補を #300 への参照にした。
- 修正 commit: 本 note の更新と同じ commit(返信に SHA を書く)。PR description の編集は push を伴わない。
- 出力生成系 skill の再判断: decision log、note、PR description だけの変更のため、P1 の判断を変えない。

## P5 の記録

- verify-comments(P2 と同じ subagent を再開して委譲)。Reviewed head: `07fbfaf5ae34b5b12c8bf68a6f94174cee5fd5bf`。完了要約は PR conversation comment(issuecomment-5921349265)。
- 区分ごとの確認済み件数: 修正確認済み 3(3 thread とも resolve 可)、スコープ外として確認済み 0、対応不要として確認済み 0。未対応 0(top-level の指摘は無い)。
- Issue #300 への参照の追加も、PR の範囲と整合すると確かめられた。

## P6 の記録

- 終了時の状態: PR #299 は open。review cycle `claude-code-c236958-20260930225828` は未対応 0 件で完了。P5 が確かめた head は `07fbfaf`(check runs 5 件 success)。本 note の P5 / P6 の記録は、その後の note だけの commit である。
- 人間に残る作業: 3 thread の resolve、merge、release の時期の判断(Homebrew が `postflight` を disabled にする前、早くても 2026-10-13 ごろ)。metadata の誤りを訂正できなかった投稿は無い。

## リスク・ブロッカー

- 次の release の後の確認で `run` が失敗した場合は、`must_succeed` や `writable_paths` の見直し、または hook を外す案へ戻る判断が要る(decision log 0041 の見直す条件)。
- GoReleaser が `*_steps` の設定を出したら、`custom_block` から移す。ユーザーの指示で Issue #300 として起票した(2026-10-01。前提は、`*_steps` を出力する設定が GoReleaser v2 の release に含まれていること)。
- 新しい cask は Homebrew 6.0.13 以降でないと読み込めない(decision log 0041 の実装上の注意。P2 の `[imo]`)。

## セッションログ

- 2026-10-01: Issue #239 を読み、GoReleaser v2.18.2 と Homebrew の source と文書で `*_steps` への対応を確かめた。GoReleaser は未対応。ユーザーに方針を確認し、`custom_block` で `postflight_steps` を出力する案に決めた。`.goreleaser.yaml` と decision log 0041(追記、見直す条件、index の行)を更新し、snapshot build と host の Homebrew での読み込みで確かめた。
- 2026-10-01: PR #299 を作り、`number-working-branch-note` で note を採番した(`f06c139`)。P1 の記録を残した。
- 2026-10-01: P2 の review(`claude-code-c236958-20260930225828`、指摘 3 件)を受け、P4 で 3 件とも採用して decision log 0041、note、PR description を直した。ユーザーの指示で follow-up の Issue #300 を起票した。
- 2026-10-01: P5 の再確認で 3 件とも修正確認済み(未対応 0 件)。P6 の終了の状態を残した。
