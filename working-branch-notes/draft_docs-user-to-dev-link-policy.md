# 作業ブランチメモ

- ブランチ: docs-user-to-dev-link-policy
- PR: (未採番)
- 最終更新: 2026-07-04

## 目的

Issue #129 の対応。利用者向けドキュメント(repo root `README.md` と `doc/help/` 配下)から開発者向けドキュメント(`doc/design/` spec や decision log など)への本文リンクを極力避け、必要な場合は文末の脚注として「開発者向け」と明示する方針を、decision log とガイドラインに明文化する。PR #128(#52 対応)のレビュー指摘が発端。

## 現在の状況

- 関連正本を確認済み: `document-style-guidelines.md`、`decision-log-guidelines.md`、`agent-configuration-management.md`、decision log 0039 / index.md。
- 既存の document-style rule 入口(Cursor / Claude)の glob は既に `README.md` / `doc/**/*.md` / `progress.md` / `working-branch-notes/**/*.md` / `AGENTS.md` を対象にしており、利用者向け help(`doc/help/**`)を含む。よって rule の glob 変更は不要で、正本追記が rule 経由で反映される。

## 決定事項

- 追記先ガイドラインは `doc/guidelines/document-style-guidelines.md`(「読者層ごとの内容分担(要点)」節)。ユーザー確認済み。
- decision log は新規 0049 を起こす。decision log 0039(decision log への直接リンク禁止)を、開発者向けドキュメント全般へ一般化する位置づけ。0039 は superseded にはせず related として残す(0039 は decision log 固有の対象読者論も含むため)。
- rule の glob は変更しない(既に対象を包含)。
- #123 実行時に反映されるよう #123 へコメント補足する。
- 実施単位は新規 Issue #129 / 別 PR(1 Issue = 1 PR)。

## 方針の要点(明文化する内容)

- 利用者向け文書の本文からは、開発者向け文書(`doc/design/` の spec、decision log など)へ直接リンクしない。
- どうしても参照が必要な場合は、本文ではなく文末の脚注に置き、「開発者向け」であることを明示する。
- 利用者に必要な情報は利用者向け文書本文に書くか、利用者向けの help / spec へリンクする。
- decision log への直接リンク禁止(0039)は本方針に包含される特例として維持。

## 次にやること

- decision log 0049 作成、index.md 更新、guideline 追記、progress.md 更新、PR 作成、#123 コメント。

## 検証

- decision log 0049 を新規作成し、index.md の「現在有効な主要方針」に 0049 行を追加。
- `document-style-guidelines.md` の「読者層ごとの内容分担(要点)」に本文リンク回避 / 文末脚注方針を追記し、decision log 0039 を包含特例として整理。
- 既存 rule 入口(Cursor / Claude)の glob が `README.md` / `doc/**/*.md` などを対象にしており利用者向け help を含むことを確認。glob 変更は不要と判断。
- progress.md に #129(help-08)を登録し、#123(help-05)の依存・次にやることへ本方針を補足。
- ReadLints: 対象ファイルで lint エラーなし。

## リスク・ブロッカー

- 既存文書中の開発者向けリンク全面棚卸しはスコープ外(#123 で扱う)。

## セッションログ

- 2026-07-04: PR #128 レビュー指摘を受け、方針を Issue #129 として起票。別ブランチで decision log + guideline 化に着手。
