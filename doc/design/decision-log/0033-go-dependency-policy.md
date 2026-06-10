# 0033 Go の依存方針と主要ライブラリ

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/architecture.md`, `doc/design/slack-api-usage.md`, `doc/design/decision-log/0032-implementation-language.md`

## 背景

実装言語を Go に決めた(0032)ことを受け、外部依存の許容範囲と、領域ごとのライブラリ選定を確定する必要がある。開発基盤方針(0002)はサプライチェーン攻撃対策を明示しており、依存ツリーの大きさはこのリスクに直結する。

## 候補

- Slack API client: (A) 自前 thin client(`net/http` + `encoding/json`)。(B) コミュニティ SDK `slack-go/slack`。
- CLI 引数 parse: (A) 標準 `flag`。(B) `spf13/cobra`。(C) `urfave/cli`。
- TTY interactive selection: (A) `charmbracelet/huh`。(B) `AlecAivazis/survey`。(C) `manifoldco/promptui`。(D) 素の `bubbletea` で自作。
- HTML 生成: (A) 標準 `html/template`。(B) サードパーティテンプレートエンジン。

## 検討内容

- **Slack client**: 使用する method は 7 種だけ(`slack-api-usage.md`)で、必要なのは form / JSON のリクエスト組み立て、`ok` / `error` 形式のレスポンス処理、cursor pagination、429 / `Retry-After` 制御。`slack-go/slack` は全 API を覆う大きな依存で、rate limit 制御の方針(0025)は結局自前の wrapper が必要になる。thin client なら依存ゼロで、リトライ・平準化を transport 層に一元化できる。リスクはリクエスト形式の細部を自前で扱うことだが、PoC の実 E2E で検証できる。
- **CLI parse**: subcommand を採用しない(0006)ため、cobra は過剰で依存も大きい。標準 `flag` は long flag(`--max-posts`)を扱え、9 個程度の option には十分。help 出力の整形が物足りなくなったら `spf13/pflag` 等を再検討する。
- **TTY selection**: カーソル上下 + Enter の選択 UI(`usage-flow.md`)は raw mode 制御が必要で、標準ライブラリだけでは実装負担が大きい。`survey` はアーカイブ済み、`promptui` は更新停滞。`huh` は活発に維持され(v2.0.3、2026-03)、Select コンポーネントが要件と一致し、MIT license。素の `bubbletea` 自作は柔軟だが選択 UI 1 つのためには過剰。
- **HTML 生成**: `html/template` は標準ライブラリで、contextual auto-escaping により「全テキストをエスケープして自前マークアップのみ生成」(0026)を実装レベルで強制できる。サードパーティを採用する理由がない。
- **TTY 判定**: `--no-interactive` と non-TTY 分岐(`usage-flow.md`)に `golang.org/x/term` を使う。`x/*` は Go チーム管理の準標準として許容する。
- **標準絵文字データ**: shortcode → Unicode の対応表はライブラリではなくデータの問題。実行時依存を避け、`go:embed` で vendored JSON を同梱する。データ源は Slack 自身が利用していることで知られる公開データセット(iamcal/emoji-data 系)を想定し、PoC で具体化する。

## 決定

- 依存方針: 標準ライブラリ第一。外部依存は `charmbracelet/huh`(TTY 選択 UI)と `golang.org/x/term`(TTY 判定)に限定する。依存の追加・変更は decision log に記録する。
- Slack API client は自前 thin client とし、`slack-go/slack` は採用しない。
- CLI 引数は標準 `flag`、HTML 生成は標準 `html/template`、標準絵文字データは `go:embed` の組込み JSON とする。

## 理由

- サプライチェーンリスクと保守対象を最小化しつつ、要件(7 method、選択 UI、自動エスケープ)を満たす最小構成だから。
- rate limit 制御(0025)のような本ツールの中核挙動を、外部 SDK の実装都合に依存させないため。

## 影響

- `architecture.md` の主要コンポーネント表に反映した。
- PoC では thin client の API 形式(エンコーディング、エラー処理、pagination)を実 E2E で確認する。問題があれば本ログを見直す。
- huh は bubbletea 系の transitive 依存を持つ。`go.mod` / `go.sum` で固定し、更新は通常の依存更新手順で扱う。

## 後から見直す条件

- PoC で thin client の実装コストが想定を大きく超えた場合。
- Slack Web API の形式変更が頻発し、SDK 追従の方が安全になった場合。
- help 整形・補完など CLI UX の要求が標準 `flag` の範囲を超えた場合。
