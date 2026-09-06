# 作業ブランチメモ

- ブランチ: `register-review-followup-issues`
- PR: #212
- 最終更新: 2026-09-06

## 目的

PR #201(Issue #190 / RF-02)の review 補助分析で見つかった、リファクタリングのスコープ外にある既存挙動の不具合・改善点を GitHub Issue #202〜#211 として登録し、優先順位を整理して `progress.md` の進行中タスク索引へ反映する。あわせて decision log 0056 に着手順の見直しを追記する。

## 現在の状況

- Issue #202〜#211 を作成済み(bug 7 件、enhancement 2 件、label なしの後片付け 1 件)。
- `progress.md` に「進行中タスク: review で見つかった既存挙動の修正」の表(FU-01〜FU-10)と、リファクタリング施策と合わせた全体の着手順を追加。
- decision log 0056 に「見直し(2026-09-06)」を追記し、index.md の要約行を更新。

## 決定事項

- 優先順位は、データ破壊(#202)→ 利用者に見える表示の誤り(#203 / #204 / #205)→ 件数・cache の整合(#206 / #207 / #208)→ API 呼び出しの無駄と edge case(#209 / #210)→ 後片付け(#211)の順。
- #202 だけを RF-03(#191)より先に置く。無警告で asset を 0 byte にするため、次の patch release 候補とする。
- Run の取得工程に関わる #206 / #207 は #191 の後(#206 は #191 を必須依存)。それ以外は技術的に独立だが、運用上は並行実行しない。
- `progress.md` の既存 RF 表の行は並べ替えない。RF-02 行は PR #201 側で `done` に更新されるため、本ブランチでは触らず、両 PR がどの順で merge されても衝突しないようにする。
- 起点 Issue は作らず、`Closes` 対象も無し(PR #167 と同じ扱い)。
- 各 Issue には調査元(PR #201 review 補助分析)、確認状況(code 読解のみか、実行で再現済みか)、行番号の基準 head(`c0e2dc6`)を明記した。#204 の `hidden_by_limit` は実 payload 未確認である旨を Issue に残した。

## 次にやること

- PR #212 の review / merge を待つ。note の rename は完了済み。

## 検証

- GitHub MCP で #202〜#211 が open、関連 PR なし、番号と本文の対応が意図どおりであることを `issue_read` で確認。
- `progress.md` の索引行から各 Issue へ辿れること、依存欄が必須条件(#191、#190)だけであることを確認。
- `git diff --check`。

## リスク・ブロッカー

- #202 / #203 / #205 / #206 / #208 / #209 / #210 は code 読解で確認したもので、実行での再現は各 Issue の着手時に行う。#207 は gensample の実行で再現済み。
- #204 の `hidden_by_limit` の field 構成は Slack API document / 実 workspace で未確認。

## セッションログ

- 2026-09-06: PR #201 の review 補助分析の指摘を code で裏取りし、Issue #202〜#211 を作成。`progress.md` / decision log 0056 / index.md を更新。
- 2026-09-06: PR #212 を作成し、note を採番。
