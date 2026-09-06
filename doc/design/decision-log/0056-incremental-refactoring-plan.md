# 0056 互換性を維持した段階的リファクタリング

- 状態: decided
- 作成日: 2026-09-05
- 最終更新日: 2026-09-06
- 関連: [architecture](../architecture.md)、[CLI](../cli-interface.md)、[cache](../cache.md)、[HTML](../html-rendering.md)、[依存方針0033](0033-go-dependency-policy.md)、[調査Issue #188](https://github.com/kiyohara/slapex/issues/188)

## 背景

機能追加後の保守負担を下げるため、コードサイズ、モジュール構成、命名、文書を調査した。基準は main `d5aa977725fbf3c8f65c8b3e6bb9bfa8cba29b55`。Goは13,453物理行で、そのうちtestが7,201行、本体が5,453行、開発ツールが799行である。`export.Run` は346行で取得から保存までの状態管理を担う。

ユーザーとは、手書きコードの重複・不要処理削減を主目的とし、既存CLI・出力・cache互換性を維持し、工数上限を設けず段階化する前提を合意した。

## 候補

1. 全面的なpackage再編・framework置換で構造を刷新する。
2. 既存の責務を基に、テスト整理、同一package内の機械移動、工程抽出と重複削減を段階的に行う。
3. ファイル/行数を主指標に、test・コメント・生成物・過去noteを削減する。

## 検討内容

全面再編は依存・配線と回帰リスクを増やす。現時点で新SDKや汎用DIが必要な根拠はなく、0033のstdlib-firstを覆さない。現行のrender/output/slackなどの境界は利用できる。

明確な重複はAPI/downloadのretry制御とCLI/demoのoption転記にある。一方、API body読み取り失敗のretryとdownload streaming、CLIの早期入力診断とexport直接呼び出し時の検証は異なる契約であり、一律に共通化しない。

testの共通準備には整理余地があるが、検証ケースや独立した期待値を削減すべきではない。demo serverとtest serverはrange filtering、故障注入、request countの役割が違い、共通化には依存方向の再編も必要となるため保留する。

長いデータliteral、生成sample、組込みemojiは実行時機能・再現性に必要な資材である。過去noteはプログラムの保守対象量と分けて考え、記録を移転せず削除しない。全変数の長文化やutils集約にも改善の根拠はない。

## 決定

候補2を採用する。これは実施順と評価原則の決定であり、新packageの具体設計や破壊的変更の承認ではない。

- 採用施策: test準備集約 [#189](https://github.com/kiyohara/slapex/issues/189)、export機械分割・命名 [#190](https://github.com/kiyohara/slapex/issues/190)、Run工程/状態整理 [#191](https://github.com/kiyohara/slapex/issues/191)、retry共通化 [#192](https://github.com/kiyohara/slapex/issues/192)、CLI option集約 [#193](https://github.com/kiyohara/slapex/issues/193)、cache入力整理 [#194](https://github.com/kiyohara/slapex/issues/194)、現行設計文書更新 [#195](https://github.com/kiyohara/slapex/issues/195)、作業記録配置の整合 [#196](https://github.com/kiyohara/slapex/issues/196)。
- 全施策は調査PRのmerge後、1 Issue = 1 PRで直列実行する。#191は#189/#190の後、#194の必須依存は#188のみとする。#191後の実施は結果型の再利用と競合回避のための推奨順であり、現行Runにもcache入力型を導入できる。先行した場合は#191がその型を再利用する。
- #196 → #195 → #189 → #190 → #191 → #194 → #192 → #193を推奨順とする。推奨順と必須依存を区別する。
- cache全面型付け、新cache package、fake server統合、screenshot tool再編、日付/timestamp一律統合、過去note cleanupは今回の実装対象にしない。
- 具体的な名称改善は責務抽出の範囲で行い、機械移動とアルゴリズム変更を分離する。

## 理由

重複の削減と責務の明確化を別々に評価することで、行数を減らすための複雑化を避けられる。cacheの同型位置引数をまとめるように、総量が増えても誤り防止の価値が高い施策は採用できる。大きな処理変更の前にtestの準備と機械移動を済ませれば、レビュー時の確認範囲を限定できる。

## 影響

この計画では公開仕様を変更しない。各Issueに互換性条件と検証を記録し、着手時に現行コードを再確認する。後続PRは自分の構成変更に必要なspec同期とsample/demo更新規約を適用する。試行錯誤の詳細はworking branch note、実行条件は各Issue、状態・依存の索引はprogress.mdに置く。

採用しなかった案の再検討条件: fake serverの同期漏れが頻発する、cache schema変更でread/write不一致が実害になる、crop規則が増える/回帰が起きる、timestamp精度の具体的不具合が確認される、または過去noteの情報移転が確認できた場合である。

## 後から見直す条件

実装時に互換性を保てない、型・配線の増加が重複削減効果を上回る、既存testに重要な検証不足が見つかる、または新機能によってpackage境界が変わる場合は、該当Issueの範囲と順序を再評価する。新依存や仕様変更は本計画から暗黙採用しない。

### Reviewでの具体化(2026-09-05)

#189は結合testの共通準備と通常成功時phase完了順のcharacterization test、#190はproduction関数に対応するunit test配置を担当する。共通fixtureを二重移動しない。#190の機械移動と命名変更、#191のthread集合化と工程抽出はそれぞれ別commitにする。#193はOptionsのfield形状を維持し変換関数で集約する。

#190/#191/#193では、着手時のcommitted sampleのfooterに合わせて時刻とTZを固定し、ja/enのHTML・CSS・assetsを再生成してファイル集合とbytesを比較する。現基準は `2026-07-04T16:32:41+09:00`、`TZ=Asia/Tokyo`。`go run ./tools/gensample -time 2026-07-04T16:32:41+09:00 -out /tmp/refactor-samples` をDocker Compose内で実行し、`diff -r doc/samples/ja /tmp/refactor-samples/ja` とen側の比較が無差分であることを確認する。基準commitと変更後で同じ条件を使い、基準に既存差分があれば記録して両生成結果を比較する。説明不能な差分でbaselineを上書きしない。cache JSONはsampleに含まれないため#194で別途検証する。既存sample更新skillの架空データ・参照検証を守り、無差分検証のためだけに生成物をcommitしない。

検証ではZ表記だとNowのlocationがUTCとなりExportedの表示が変わった。+09:00指定ではja/enとも全ファイルが一致したため、時刻の瞬間だけでなくoffsetも固定する。

### 見直し(2026-09-06)

#190 の PR #201 review で、リファクタリングのスコープ外にある既存挙動の不具合・改善点が見つかり、Issue #202〜#211 として登録した。本計画の直列実行と互換性維持の原則は変えず、着手順だけを次のとおり見直す。

- データ破壊(#202、`--reuse-cache` の再利用元が今回の出力先と同じときの自己コピー)は #191 より先に修正する。
- Run の取得工程に関わる修正(#206、#207)は #191 の後に置き、#191 の結果型と thread 集合の整理を前提にする。#207 は #191 で吸収してよい。
- 表示・cache の修正(#203、#204、#205、#208)、API 呼び出し・edge case の改善(#209、#210)、分割後の後片付け(#211)は、リファクタリング施策の間に挟む。#211 の項目は #191 / #194 で該当箇所を触る場合にそちらで吸収してよい。
- 全体の順序と状態は progress.md を索引とする。各 Issue の確認状況(code 読解のみか、実行で再現済みか)は Issue 本文に明記し、着手時に再確認する。
