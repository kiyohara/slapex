# 作業ブランチメモ

- ブランチ: split-design-spec-docs
- PR: 未採番
- 最終更新: 2026-06-03

## 目的

肥大化した `doc/design/usage-flow.md`(491 行)を、トピックに応じた仕様文書へ分割し、見通しを良くする。「見栄え(HTML 表示仕様)」や「cache の扱い」など、利用手順(操作の流れ)とは独立したトピックを別文書に切り出す。

## 現在の状況

- 分割粒度は「成果物軸で 4 分割」を採用(usage-flow / output-format / html-rendering / cache)。
- `doc/design/output-format.md`、`doc/design/html-rendering.md`、`doc/design/cache.md` を新規作成。
- `doc/design/usage-flow.md` を操作の流れに絞り、冒頭に分割文書への案内を追加。
- `doc/design/decision-log/0021-spec-document-split.md` を追加し、index に追記。
- `doc/design/README.md` の「主な文書」、`.github/copilot-instructions.md` のレビュー観点を更新。

## 決定事項

- 分割軸は「利用者の操作の流れ」と「成果物の仕様(出力・見た目・cache)」。詳細は `decision-log/0021-spec-document-split.md`。
- spec の正本は `doc/design/` 直下の各文書、decision log はその確定経緯を辿る参考ログ、という建て付けを維持する。既存 decision log の本文は履歴として据え置き、内容が移動したログの `関連` リンクのみ移動先の新 spec 文書へ更新する(新旧再編の事実は 0021 に記録)。
- 未決事項は各トピック文書に分散しつつ、全体一覧は `decision-log/index.md` に集約。
- `.cache/` のディレクトリ構造は `output-format.md` の出力イメージに残し、内容・削除・再利用の方針は `cache.md` に集約(重複を避けるため出力イメージ側はポインタにとどめる)。

## 次にやること

- `git diff` で文書差分と相互リンクの整合を最終確認する。
- 必要なら PR を作成し、採番後に本 note を `<PR-number>_...` へリネームする。

## 検証

- 移動した節が新文書に過不足なく収まっているか、原文との対応を確認。
- 相互リンク(usage-flow ↔ output-format / html-rendering / cache、各文書 → decision-log)の整合を確認。
- working branch note に secret 実値が入っていないことを目視確認。

## リスク・ブロッカー

- 内容が移動した decision log の `関連` リンクは移動先の新 spec 文書へ更新済み。本文の影響範囲記述は当時の作業記録として据え置いたため、本文中には旧 spec(usage-flow.md)への言及が残るが、これは履歴として意図的に保持している。

## セッションログ

- 2026-06-03: usage-flow.md をトピック別 4 分割。新文書 3 件作成、decision-log 0021 追加、index / README / copilot-instructions 更新、本 note 作成。
- 2026-06-03: 「spec=正本 / decision log=参考」の建て付け確認。0021・本 note の「正本」表現を是正し、内容が移動した decision log(0005 / 0008 / 0010 / 0011 / 0012 / 0013 / 0014 / 0016 / 0017)の `関連` リンクを移動先の新 spec 文書へ更新。
- 2026-06-03: 取り違え再発防止の仕組みを層構造で導入。`decision-log-guidelines.md` に「正本と参照の関係」節、`_template.md` 冒頭コメント、`index.md` 冒頭一行、`.claude/.cursor` の decision-log shim に一行を追加し、0022 として記録。
- 2026-06-03: Codex 対応を確認。正本/template/index は tool 非依存で Codex も AGENTS.md→正本で到達するが、auto-load JIT 一行は Claude/Cursor のみ。AGENTS.md への強調一行追加は 42ebdb6 の「薄い index」前例に合わせ見送り、非対称を許容する旨を 0022 に追記。
