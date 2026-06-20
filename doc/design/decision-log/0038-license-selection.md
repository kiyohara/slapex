# 0038 ライセンス選定

- 状態: decided
- 作成日: 2026-06-20
- 最終更新日: 2026-06-20
- 関連: `README.md`, `LICENSE`, `doc/design/decision-log/0034-distribution-method.md`, `doc/design/decision-log/0036-v1-release-scope.md`, `doc/design/decision-log/0033-go-dependency-policy.md`

## 背景

v1.0 公開(0036)に向けて `README.md` と `LICENSE` を整備する(Issue #29 / 本実装プラン task v1-15)にあたり、公開ライセンスを確定する必要があった。これまでライセンス種別は明文化されていなかった。配布は GitHub Releases での単一バイナリ(0034)で、ソースも公開する前提のため、利用者が安心して使える許諾条件を明示する必要がある。

## 候補

- MIT License
- Apache License 2.0
- BSD-3-Clause
- ライセンス未指定(既定で全権利留保)

## 検討内容

- MIT: 最も簡潔で広く理解されており、許諾条件は「著作権表示とライセンス文の保持」のみ。利用・改変・再配布・商用利用の障壁が低い。特許条項は持たない。
- Apache-2.0: 明示的な特許ライセンス付与と貢献者条項を備え企業利用で好まれるが、本文が長く NOTICE 運用などの手間が増える。本ツールは特許リスクが顕在化する領域ではない。
- BSD-3-Clause: MIT に近いが endorsement 禁止条項が加わる。MIT で十分。
- 未指定: 法的には全権利留保となり、利用者が安心して使えない。OSS 公開方針と矛盾する。
- 依存ライブラリ(0033: huh / x/term / x/text など)はいずれも寛容型(MIT / BSD 系)で、MIT 採用と矛盾しない。

## 決定

- ライセンスは **MIT License** とする(ユーザー確認により確定)。
- copyright 表記は `2026 Tomokazu Kiyohara`。
- repo root に `LICENSE` を置き、`README.md` の「ライセンス」節から参照する。

## 理由

- 単一バイナリ + ソース公開で「手軽に使ってもらう」ことを重視する方針(0034 / 0036)に最も整合する寛容型ライセンスであり、許諾条件が最小だから。特許条項の明示付与が必要となる利用形態は現時点で想定されない。

## 影響

- repo root に `LICENSE`(MIT、copyright `2026 Tomokazu Kiyohara`)を追加した。
- `README.md` に「ライセンス」節を設け、`LICENSE` を参照する。
- 今後ソースにライセンスヘッダや SPDX 識別子を付す場合は `MIT` を使う。

## 後から見直す条件

- 特許ライセンスの明示付与が求められる利用形態(企業からの要請など)が出てきた場合に Apache-2.0 への変更を検討する。
- 取り込む第三者コードがコピーレフトなど別ライセンスを要求する場合。
