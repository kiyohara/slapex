# 0045 CLI 出力スタイル(styled / plain の 2 モード)

- 状態: decided
- 作成日: 2026-07-02
- 最終更新日: 2026-07-02
- 関連: `doc/design/cli-interface.md`, `doc/design/usage-flow.md`, `doc/design/decision-log/0024-cli-options-and-exit-codes.md`, `doc/design/decision-log/0033-go-dependency-policy.md`, Issue #100

## 背景

利用者向け help / quickstart / README にコマンド実行例を追加する前に、CLI 出力 UX を確定する必要があった(Issue #100)。従来の stderr 出力は全行無装飾で、TTY での見やすさと CI ログの安定性のどちらにも最適化されていなかった。

見栄えの具体化にあたり、モダン CLI ツール(npm / pnpm / yarn 4 / cargo / uv / gh)の実出力キャプチャと一次資料(GitHub CLI デザインガイドライン primer/cli、clig.dev、NO_COLOR)を調査した。調査レポートは PR #(Issue #100 の PR)の working branch note に残した。

## 候補

- 全体スタイル: (A) ステータス列+フェーズ行(gh の記号語彙 × cargo のラベル列 × uv の dim)。(B) 現行の行構造を維持し色・記号だけ追加。(C) npm 的ミニマル(warn/error と summary のみ着色)。
- 待機インジケーター: braille spinner(`⠋⠙⠹…`) / ASCII(`\|/-`) / 静的テキスト。
- 色の濃さ: 記号のみ着色(控えめ) / 数値・強調への着色も追加(標準)。
- 実装方式: 自前 ANSI helper(stdlib のみ) / lipgloss v2 直接利用 / colorprofile writer 併用。

## 検討内容

- slapex の 1 回の実行は「workspace 確定 → channel 解決 → 履歴取得 → user 解決 → emoji → assets/render → summary」の直列フェーズ進行であり、package manager 型の並列カウンタ進捗より、cargo / uv / gh のフェーズ語彙が構造に合う。
- 調査した全ツールが共通して、(1) 基本 8/16 色 + bold/dim のみ、(2) 色は意味の強化にだけ使う、(3) live 更新は一時表示に限定し確定情報は行として積む、(4) non-TTY では plain に自動劣化、を守っていた。
- gh は 2025 年のアクセシビリティ改善で braille spinner を廃止し静的テキストへ移行した。一方 uv 等の最新ツールは braille spinner を採用している。slapex は TTY 限定・長時間待機限定で braille を使い、plain mode では一切出さないことでリスクを限定する。
- `charmbracelet/colorprofile`(huh の transitive 依存)は `NO_COLOR` / `CLICOLOR` / `TERM=dumb` を処理するが `CI` を見ないため、いずれにせよ自前判定が必要。必要な装飾は 8 色 + bold/dim + 行頭復帰/行クリアだけであり、自前 helper で十分小さく書ける。

## 決定

- stderr の進捗・診断は styled / plain の 2 モードとし、既定は自動判定。plain 化条件は `--no-color` / `NO_COLOR`(空でない値)/ `TERM=dumb` / `CI`(空でない値)/ stderr 非 TTY(`cli-interface.md`「出力制御」)。
- styled はフェーズ行形式: 状態記号(`✓` green / `!` yellow / `✗` red)+ bold ラベル列 + 本文、補足メタ情報は dim。値そのものは着色しない(控えめ配色)。長時間待機のみ braille spinner で進行中の 1 行を上書きする(案 A + braille + 控えめ。2026-07-02 ユーザー確認)。
- plain は 1 イベント 1 行の追記のみで、ASCII prefix(`OK:` / `WARN:` / `ERROR:` / `INFO:`)を付ける。ANSI・CR 上書き・装飾記号は出さない。
- `--no-color` を正式 option 化する。名前は NO_COLOR 標準および他ツールの慣行と揃え、色だけでなく装飾全体を抑止する(実態は plain mode 切替)。
- 実装は `internal/ui` の自前 ANSI helper(stdlib + `x/term` のみ)。新規依存は追加しないため 0033 の依存方針は変更しない。lipgloss / colorprofile を直接利用へ昇格する場合は 0033 に追記する。

## 理由

- フェーズ行形式が slapex の処理構造と一致し、「今どこで待っているか」(rate limit 待ち等)を spinner 行として自然に表せるため。
- 基本 8 色 + 記号のみ着色は端末テーマ非依存で、gh のアクセシビリティ知見(4-bit への整列、色は意味の強化のみ)とも一致するため。
- `CI` を「空でない値」で判定するのは、主要 CI が `CI=true` を設定する一方で他の truthy 値を使う環境もあり、CI ではログ安定性へ倒すのが安全なため。

## 影響

- `cli-interface.md` に「出力制御」節と `--no-color` を追加し、将来検討から `--no-color` を外した。
- `usage-flow.md`「処理対象の表示」の表示例をフェーズ行形式(styled / plain)に更新した。
- stderr 出力は `internal/ui.Printer` に集約し、`fmt.Fprintf(os.Stderr, ...)` の直書きを export / cmd から排除した。
- README の option 表に `--no-color` を追加した。

## 後から見直す条件

- braille spinner がフォント・端末互換で問題を起こした場合(ASCII または静的テキストへ後退)。
- `--quiet` / `--verbose` を追加する時(出力量の制御はこの決定の対象外、0024 の未決事項のまま)。
- スクリーンリーダー利用者からのフィードバックで gh 同様の静的テキスト方式が必要になった場合。
