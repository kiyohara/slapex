# 作業ブランチメモ

- ブランチ: `post-v1/cli-output-ux`
- PR: -
- 最終更新: 2026-07-02

## 目的

Issue #100「CLI 出力をモダンなターミナル表示と CI 向け plain output に両対応させる」を進める。

ユーザー指示により、実装に入る前の当面のゴールは次のとおり。

1. npm に代表されるモダンな CLI ツールの出力表現(色・アイコン・spinner・進捗表示・plain fallback)を十分に調査する。
2. 調査結果と slapex への適用案を、この working branch note 群に一定レベルのレポートとしてまとめる(必要に応じて画像も使う)。
3. slapex CLI の見栄えについて判断が必要な点は、レポートに基づきユーザーに確認して決める。

## 現在の状況

- 調査完了。レポートを補助 note `draft_post-v1-cli-output-ux__modern-cli-research.md` にまとめた。
- レポート用の画像(SVG モック、キャプチャを元に再現)は `assets/cli-output-ux/` に置いた。note と同様に cleanup PR の整理対象とする。
- 見栄え方針(案 A / B / C、spinner、色の濃さ)のユーザー確認待ち。

## 決定事項

- 調査対象は npm / pnpm / yarn 4 / cargo / uv の実出力(Docker 内で TTY / non-TTY 両キャプチャ)と、primer/cli(gh デザイン正本)・clig.dev・NO_COLOR 等の一次資料とした。
- レポートの推奨は案 A(ステータス列+フェーズ行)。実装方式は自前 ANSI helper(依存追加なし)を見立てとして記載。
- 見栄えの最終判断はユーザー確認により行う(未決)。

## 次にやること

- 見栄え方針のユーザー確認と、結果の「決定事項」への記録。
- 決定に基づく実装(設計文書更新 → UI helper → `--no-color` → テスト)。

## 検証

- (Issue 記載の検証は実装後に実施。調査段階では未実施)
- 調査キャプチャ: `docker run -t`(TTY)/ pipe(non-TTY)で npm 11 / pnpm 11 / yarn 4 / cargo / uv の生出力を取得し、ANSI escape を解析した。non-TTY で全ツールが plain に劣化することを確認。

## リスク・ブロッカー

- (なし)

## セッションログ

- 2026-07-02: ブランチ作成。Issue #100 読了。当面のゴールを「調査レポート + 見栄え方針のユーザー確認」に設定(ユーザー指示)。
- 2026-07-02: 現状把握(cli-interface.md / usage-flow.md / main.go / export の logf 一覧)。モダン CLI 5 種を Docker 内でキャプチャ、primer/cli 等の一次資料を調査。レポートと SVG モック 5 点を作成。lipgloss v2 / colorprofile が huh の transitive 依存として既に go.sum にあること、colorprofile は `CI` を見ないことを確認。
