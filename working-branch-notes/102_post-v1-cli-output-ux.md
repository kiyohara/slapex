# 作業ブランチメモ

- ブランチ: `post-v1/cli-output-ux`
- PR: #102
- 最終更新: 2026-07-02

## 目的

Issue #100「CLI 出力をモダンなターミナル表示と CI 向け plain output に両対応させる」を進める。

ユーザー指示により、実装に入る前の当面のゴールは次のとおり。

1. npm に代表されるモダンな CLI ツールの出力表現(色・アイコン・spinner・進捗表示・plain fallback)を十分に調査する。
2. 調査結果と slapex への適用案を、この working branch note 群に一定レベルのレポートとしてまとめる(必要に応じて画像も使う)。
3. slapex CLI の見栄えについて判断が必要な点は、レポートに基づきユーザーに確認して決める。

## 現在の状況

- 調査完了。レポートを補助 note `102_post-v1-cli-output-ux__modern-cli-research.md` にまとめた。
- レポート用の画像(SVG モック、キャプチャを元に再現)は `assets/cli-output-ux/` に置いた。note と同様に cleanup PR の整理対象とする。
- ユーザー確認済みの方針(案 A / braille / 控えめ)で実装完了。検証も完了(下記)。
  - `internal/ui` パッケージ新設(styled / plain の 2 モード、フェーズ行 + braille spinner + ASCII prefix)。
  - export / cmd の stderr 直書き logf を `ui.Printer` に置き換え。`--no-color` option 追加。
  - `doc/design/cli-interface.md`「出力制御」節、`usage-flow.md` 表示例、README option 表、decision log 0045 を更新。

## 決定事項

- 調査対象は npm / pnpm / yarn 4 / cargo / uv の実出力(Docker 内で TTY / non-TTY 両キャプチャ)と、primer/cli(gh デザイン正本)・clig.dev・NO_COLOR 等の一次資料とした。
- **見栄えはユーザー確認により確定(2026-07-02)**: 全体スタイルは**案 A(ステータス列+フェーズ行)**、長時間待機のインジケーターは **braille spinner**(`⠋⠙⠹…`、TTY 限定)、配色は**控えめ**(状態記号 ✓/!/✗ のみ着色、ラベル bold、メタ情報 dim。値は端末デフォルト色)。
- 実装方式は自前 ANSI helper(stdlib のみ、依存追加なし)。lipgloss / colorprofile への昇格はしない(するなら decision log 0033 追記が必要)。
- plain mode は Issue #100 の方針どおり: stderr 非 TTY / `CI`(空でない値)/ `TERM=dumb` / `NO_COLOR` / `--no-color` のいずれかで ASCII prefix(`OK:`/`WARN:`/`ERROR:`/`INFO:`)の行追記のみ。

## 次にやること

- PR レビュー対応(merge はユーザー)。

## 検証

Issue 記載の検証(すべて Docker Compose 経由、2026-07-02):

- `docker compose run --rm --no-deps dev sh -c "go vet ./... && go test ./..."` → 全パッケージ pass(新設 `internal/ui` のユニットテスト、stdout 契約テスト `TestRunStdoutCarriesOnlyTheResult`、`--no-color` parse / `--help` 反映テストを含む)。
- `go run ./cmd/slapex --help` → `-no-color` が表示されることを確認。
- plain 化条件: 擬似 TTY(`script -qec`)下で `NO_COLOR=1` / `CI=true` / `TERM=dumb` / `--no-color` の各条件、および pipe(非 TTY)で、ANSI escape・spinner・記号なしの `ERROR:`/`OK:`/`WARN:`/`INFO:` 行のみになることを確認。
- styled 経路: 擬似 TTY 下で赤 `✗` のエラー表示、および一時デモ(削除済み)でフェーズ行のライフサイクル(braille spinner 上書き → ✓/! 確定行、warning の割り込み、Done summary)が SVG モックどおりに動くことを確認。
- stdout 契約: `--version` と usage エラーで stdout に診断が混ざらないことをテストで確認(成功時 path 1 行は `main.go` の単一の `fmt.Fprintln(os.Stdout, dir)` のみで、export 側は `ui.Printer`(stderr)しか持たない構造)。

調査キャプチャ(調査段階): `docker run -t`(TTY)/ pipe(non-TTY)で npm 11 / pnpm 11 / yarn 4 / cargo / uv の生出力を取得し、ANSI escape を解析した。non-TTY で全ツールが plain に劣化することを確認。

## リスク・ブロッカー

- (なし)

## セッションログ

- 2026-07-02: ブランチ作成。Issue #100 読了。当面のゴールを「調査レポート + 見栄え方針のユーザー確認」に設定(ユーザー指示)。
- 2026-07-02: 現状把握(cli-interface.md / usage-flow.md / main.go / export の logf 一覧)。モダン CLI 5 種を Docker 内でキャプチャ、primer/cli 等の一次資料を調査。レポートと SVG モック 5 点を作成。lipgloss v2 / colorprofile が huh の transitive 依存として既に go.sum にあること、colorprofile は `CI` を見ないことを確認。
- 2026-07-02: 見栄え方針をユーザー確認(案 A / braille / 控えめ)。`internal/ui` を新設し、export / cmd を配線替え。`--no-color` 追加。設計文書・README・decision log 0045 更新。検証完了。interactive selection(huh)開始前に spinner を止める考慮を chooseChannel に入れた。
