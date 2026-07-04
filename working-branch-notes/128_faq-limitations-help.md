# 作業ブランチメモ

- ブランチ: faq-limitations-help
- PR: #128
- 最終更新: 2026-07-04

## 目的

Issue #52 (help-03) の対応。取得範囲の default(30 日 / 1000 件)、1 thread の replies 打ち切り、`blocks` の完全レンダリング非対応、Enterprise Grid org-wide install 非対応、Windows 非対応などの制限が design doc に散在している状態を解消する。利用者が初回 export 前に「取得されない範囲」「現状の制限」を把握できるよう、制限事項・FAQ help を新設する。

## 現在の状況

- Issue #52 と comment を確認済み。依存 help-00 (#100) は done で満たされている。
- 制限事項の正本を design doc から洗い出し済み(下記「決定事項」参照)。

## 決定事項

- 新設ファイル名は `doc/help/faq.md` とする(Issue が faq.md / limitations.md のどちらかを許容。Q&A 形式なので faq を採用)。
- 各項目は要点と参照に留め、正本(`doc/design/output-format.md` / `html-rendering.md` / `cli-interface.md` / `slack-app-setup.md` など)へリンクする。利用者向け help なので decision log へは直接リンクしない(`document-style-guidelines.md`)。
- README の「出力」節付近から 1 行リンクする。
- Issue comment に従い、quickstart の「つまずいたら」節のリンク先のうち FAQ に集約できるものを faq.md へ差し替える。
- 文体は利用者向けのですます調。

## 制限事項の洗い出し(正本)

- 取得範囲 default: `--max-posts` 1000 / `--days` 30、AND 結合 → `output-format.md` / `cli-interface.md`
- 1 thread replies 上限 1000 件で打ち切り(置換表示) → `output-format.md`
- 添付/original 画像は `--max-attachment-size`(default 10MB)超で保存しない、public asset は別途 5MiB guard → `output-format.md`
- `blocks`(rich_text)完全レンダリング非対応、`text` fallback を正とする → `html-rendering.md`
- legacy attachment の色付き枠・フィールド完全模倣は初期対象外 → `html-rendering.md`
- Enterprise Grid org-wide install 非対応(単一 workspace token 前提) → `slack-app-setup.md` / `usage-flow.md`
- Windows 非対応(macOS / Linux のみ) → `cli-interface.md`
- 単発スナップショットで実行中の Slack 更新との整合は非保証 → `slack-api-usage.md`
- token を CLI 引数で渡せない(環境変数のみ) → `cli-interface.md`

## 次にやること

- faq.md 作成、README リンク、quickstart 差し替え、progress.md 更新、PR 作成。

## 検証

- `doc/help/faq.md` から design doc / help への相対リンクとアンカーを、対象見出しの実在で確認済み(`output-format.md#取得範囲` / `#添付ファイルのサイズ制限`、`cli-interface.md#option-一覧` / `#環境変数` / `#exit-code` / `#対象プラットフォーム`、`html-rendering.md#本文の変換mrkdwn--html`、`slack-api-usage.md#取得の整合性`、`slack-app-setup.md#前提` / `#channel-access` / `#よくあるエラー` / `#bot--app-を-channel-に参加させる`)。
- README(`doc/help/faq.md`)/ quickstart(`faq.md`, `faq.md#うまくいかないとき`)からの誘導リンクを確認済み。
- ReadLints: 対象 4 ファイルで lint エラーなし。
- 文体: 利用者向けドキュメントとしてですます調で統一。

## リスク・ブロッカー

- なし。スコープ外(README 全体再構成 #123 など)には手を付けない。

## セッションログ

- 2026-07-04: Issue #52 を run-issue-task で開始。design doc から制限事項を洗い出し、note 作成。
