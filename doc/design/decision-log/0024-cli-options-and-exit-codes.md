# 0024 CLI option と exit code の確定

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-26
- 関連: `doc/design/cli-interface.md`, `doc/design/usage-flow.md`

> 2026-06-26 追記: Slack token 種別の方針転換は [0042-default-user-token.md](0042-default-user-token.md) で確定した。token は引き続き環境変数のみで受け取るが、現在の環境変数名は `SLACK_TOKEN` であり、旧 `SLACK_BOT_TOKEN` 互換は維持しない。

## 背景

option 名と default 値は `usage-flow.md` / `output-format.md` / `cache.md` に分散して記載されており、option の全体一覧、token の受け渡し経路、exit code の具体値、stdout / stderr の使い分けが未確定だった。アーキテクチャ選定と PoC 実装に入る前に、CLI の外形を 1 文書で確定する必要がある。

## 候補

- exit code: (A) 0 / 非 0 の 2 値のみ。(B) 失敗を「指定の誤り」「認証・権限」「実行時失敗」に分類した少数のコード。(C) エラー原因ごとの細かいコード体系。
- token の受け渡し: 環境変数のみ。環境変数に加えて CLI option でも受け取る。
- 出力 stream: すべて stdout。進捗・診断を stderr に分離し stdout は機械可読な結果のみ。

## 検討内容

- exit code (A) は CI で失敗原因を分類できない。(C) は初期実装に対して過剰で、保守コストが高い。(B) は「再実行で直るもの(指定誤り)」「設定作業が必要なもの(認証・権限)」「環境・一時要因(実行時失敗)」を CI 側で区別でき、粒度として十分。
- token を CLI 引数で受けると、プロセス一覧や shell history に実値が残る経路ができる。`usage-flow.md` は secret manager / CI secrets からの環境変数注入を前提としており、環境変数のみに絞っても利用体験を損なわない。
- 進捗や label の繰り返し表示(`decision-log/0020-target-label-display.md`)を stdout に混ぜると、出力 path を後続処理に渡す用途が成立しない。stderr 分離は CI・script 連携の前提として必要。
- `--quiet` / `--verbose` / `--no-color` は需要が確定してから追加できるため、初期 option には含めない。

## 決定

- option 一覧、default、制約を `cli-interface.md` に確定した(`--output` / `--max-posts` / `--days` / `--max-attachment-size` / `--keep-cache` / `--reuse-cache` / `--no-interactive` / `--version` / `--help`)。
- token は環境変数 `SLACK_BOT_TOKEN` のみで受け取り、CLI option では受け取らない。
- exit code は `0` 成功 / `1` 想定外 / `2` 指定誤り・対象未確定 / `3` 認証・権限 / `4` 取得・保存の実行時失敗、とする。
- stdout は成功時の出力ディレクトリ絶対 path 1 行のみ。進捗・診断・summary は stderr に出す。
- 個別 asset の取得失敗は exit code を変えず、置換表示と manifest 記録で継続する。

## 理由

- CI で deterministic に失敗させる方針(`decision-log/0004-channel-selection.md`)を、失敗分類まで含めて実用にするため。
- secret の漏えい経路を環境変数 1 本に絞ることで、help やドキュメントの案内も単純になる。
- stdout / stderr の分離は後から変えると破壊的変更になるため、実装前に確定する価値が高い。

## 影響

- `cli-interface.md` を新設し、option の一覧はここを正とする。`output-format.md` / `cache.md` の option 表は各仕様の文脈説明として残す。
- `usage-flow.md` の「非 0 exit code」記述は exit code `2` に対応付けた。
- PoC では引数 parse、exit code 分類、stdout / stderr 分離を確認対象にする。

## 後から見直す条件

- 差分取得や guard option の導入で option 体系の再編が必要になった場合。
- exit code の 5 分類で CI 運用上の不足が判明した場合。
- 機械可読な実行レポート(JSON 出力など)の需要が出た場合。
