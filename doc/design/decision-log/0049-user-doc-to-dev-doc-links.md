# 0049 利用者向け文書から開発者向け文書へのリンク方針

- 状態: decided
- 作成日: 2026-07-04
- 最終更新日: 2026-07-04
- 関連: `../../guidelines/document-style-guidelines.md`, [0039-decision-log-audience.md](0039-decision-log-audience.md), [0019-document-directory-structure.md](0019-document-directory-structure.md)

## 背景

PR #128(Issue #52 の制限事項・FAQ help 追加)のレビューで、利用者向け help である `doc/help/faq.md` の本文から、開発者向けの `doc/design/` spec へ「詳細は ... を参照してください」と直接リンクしていた点が指摘された。

既存ルールでは、利用者向けドキュメントから decision log への直接リンク禁止(0039)は明文化されている。しかし `doc/design/` の spec 文書全般を含む「開発者向けドキュメント」への本文リンクをどう扱うかは明文化されていなかった。利用者向け文書の本文に開発者向け文書へのリンクを混ぜると、読者層ごとの内容分担(0019 / document-style-guidelines)が崩れ、利用者が開発視点の情報へ誘導されてしまう。

## 候補

1. 利用者向け文書から開発者向け文書へのリンクを一切禁止する。
2. 本文リンクは避けるが、必要な場合に限り文末の脚注として残し、開発者向けであることを明示する。
3. 従来どおり制約を設けない(decision log への直接リンク禁止のみ)。

## 検討内容

- 候補 1 は明快だが、仕様の正本が開発者向け spec にある以上、「厳密な仕様を確認したい少数の利用者」への導線を完全に断つのは過剰。
- 候補 3 は今回の指摘の再発を防げない。
- 候補 2 は、利用者向け本文の読みやすさと読者層分担を保ちつつ、必要な導線を文末の注釈として残せる。GitHub の footnote 記法は文末に自動集約され、本文の可読性を損なわない。実際に PR #128 で faq.md の spec 参照を footnote へ移し、機能することを確認した。

## 決定

利用者向けドキュメント(repo root `README.md` と `doc/help/` 配下)の本文からは、開発者向けドキュメント(`doc/design/` の spec、decision log など)へ直接リンクしない。

- どうしても参照が必要な場合は、本文ではなく文末の脚注に置き、「開発者向け」であることを明示する。
- 利用者に必要な情報は利用者向け文書の本文に書くか、利用者向けの help / spec へリンクする。
- decision log への直接リンク禁止(0039)は、本方針に包含される特例として引き続き有効とする(0039 は decision log の対象読者論も扱うため superseded にはしない)。

正本は `doc/guidelines/document-style-guidelines.md` の「読者層ごとの内容分担」節に置く。

## 理由

- 読者層ごとの内容分担(0019)を、リンクの面でも一貫させる。
- 仕様の正本(開発者向け spec)への最小限の導線は、文末脚注という控えめな形で残せる。
- 既存の decision log 限定ルール(0039)を、開発者向けドキュメント全般へ無理なく一般化できる。

## 影響

- `doc/guidelines/document-style-guidelines.md` に本方針を追記する。
- 既存の document-style rule 入口(`.cursor/rules/document-style-guidelines.mdc` / `.claude/rules/document-style-guidelines.md`)の glob は既に `README.md` / `doc/**/*.md` などを対象としており、利用者向け help を含むため、glob 変更は不要。正本追記が rule 経由で反映される。
- 既存の利用者向け文書に残る開発者向けリンクの棚卸しは、利用者 / 開発者ドキュメント整理(#123)の中で本方針に沿って行う。
- PR #128 では faq.md / quickstart.md に本方針を先行適用済み。

## 後から見直す条件

- 利用者から「仕様の詳細へすぐ飛べない」不満が多く出て、本文リンクを許容する必要が生じた場合。
- ドキュメント構成が変わり、利用者向け / 開発者向けの境界自体を引き直す場合(#123 の結果を含む)。
