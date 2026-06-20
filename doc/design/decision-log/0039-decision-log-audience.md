# 0039 decision log の対象読者と利用者向け文書からのリンク方針

- 状態: decided
- 作成日: 2026-06-20
- 最終更新日: 2026-06-20
- 関連: `doc/guidelines/decision-log-guidelines.md`, `doc/design/decision-log/0022-spec-vs-decision-log-authority.md`, `README.md`, `doc/help/README.md`

## 背景

PR #47 のレビュー対応で、`README.md` に decision log(0031)への直接リンクを追加した。一方で decision log は方針決定の検討経緯を残す参考ログであり(0022)、主な読者は AI agent と開発者である。一般利用者が読む前提のドキュメント(`README.md`、`doc/help/` 配下)から decision log へ直接リンクすると、利用者を内部の検討ログへ誘導してしまい、ドキュメントの役割分担が崩れる。decision log の位置づけ(内部参照か利用者向けか)を明文化する必要が生じた。

## 候補

- A: decision log は開発時参照の内部ドキュメントとし、利用者向けドキュメントからは直接リンクしない。
- B: 利用者向けドキュメントからも decision log へリンクしてよい(現状維持)。

## 検討内容

- decision log は「仕様がどう確定したか」を辿る参考ログである(0022 で正本と参考の関係を明文化済み)。利用者が必要とするのは操作手順と仕様であって、決定の経緯ではない。
- 利用者向け文書から内部ログへ誘導すると、利用者が読むべき情報の範囲が曖昧になり、内部の検討メモまで「利用者向け」として整合を保つ負担が生じる。
- `doc/design/` 配下の spec から decision log を「決定経緯」として参照するのは開発者向けの内部参照であり(0022)、本方針の制約対象ではない。
- 利用者にも経緯の共有が有益なケースは、decision log を直接見せるのではなく、利用者向けの spec / help に必要な要点を書く方が役割分担に合う。

## 決定

- decision log は開発時に参照する内部ドキュメントとして扱う。主な読者は AI agent と開発者であり、一般利用者向けではない。
- 利用者向けドキュメント(repo root の `README.md` と `doc/help/` 配下)からは、decision log へ直接リンクしない。利用者に必要な情報は利用者向け文書側に本文として書くか、利用者向けの spec / help へリンクする。
- `doc/design/` 配下の spec 文書からの「決定経緯」としての decision log 参照(0022)は、開発者向けの内部参照として引き続き許容する。
- 本方針を `doc/guidelines/decision-log-guidelines.md` に明記する。

## 理由

- ドキュメントの役割分担(利用者向け = `README.md` / `doc/help/`、内部設計 = `doc/design/` + decision log)を保ち、利用者を検討ログへ誘導しないため。

## 影響

- `README.md` から 0031 への直接リンクを削除した(PR #47)。Windows 非対応の事実は本文で完結させる。
- `doc/guidelines/decision-log-guidelines.md` に「対象読者と利用者向け文書からの参照」節を追加し、`.claude/rules/decision-log-guidelines.md` shim にも要点を反映した。
- `doc/help/README.md` の文体節に、利用者向け help から decision log へ直接リンクしない旨を補足した。
- 既存の利用者向けドキュメントを再確認し、`README.md` の 1 件以外に decision log への直接リンクがないことを確認した。

## 後から見直す条件

- 利用者にも検討経緯を公開することが有益と判断される場合(例: 公開用の rationale / changelog ページを別途用意する場合)。
