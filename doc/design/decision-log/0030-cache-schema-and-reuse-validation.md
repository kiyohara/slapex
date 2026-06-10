# 0030 cache schema と再利用時の整合性検証

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/cache.md`, `doc/design/decision-log/0005-cache-handling.md`

## 背景

`.cache/` の 3 ファイル(`metadata.json` / `assets_manifest.json` / `slack_api_cache.json`)の内容は「候補」止まりで、field 構成が未確定だった。また `--reuse-cache` の整合性検証(どの不一致で再利用を拒否するか)は 0005 以来の未決事項として残っていた。

## 候補

- 検証条件: (A) workspace / channel / schema_version の一致のみ。(B) (A) に加えて token の scope や取得条件(`--days` / `--max-posts`)の一致も要求。(C) 検証せず常に再利用。
- 不一致時の動作: エラー終了する。警告して cache なしで続行する。
- メッセージ raw response の cache 保存: する / しない。

## 検討内容

- (C) は誤った workspace / channel の cache を混入させる事故経路になる。0005 でも「検証できない cache は使わず再取得」と方向づけられていた。
- (B) の scope 検証は、scope 一覧の取得と比較の実装コストに対して得るものが少ない。scope 不足は asset 取得失敗として通常経路で記録・表示される。取得条件の一致要求は、cache の主な再利用対象(assets manifest、user / emoji 解決)に対して不要に厳しく、再利用が成立する場面をほぼ無くしてしまう。
- 不一致時にエラー終了すると、利用者は cache を消すまで実行できなくなる。警告 + 通常取得フォールバックなら、安全側に倒しつつ実行は完了する。
- メッセージ raw response の保存は、サイズが大きく(添付 metadata 込みで数十 MB になり得る)、Slack 側の編集・削除を反映しない stale data になるリスクが高い。再利用の価値が大きいのは「再 download を避けたい assets」と「再解決を避けたい users / emoji」である。

## 決定

- 3 ファイルの schema(共通 field `schema_version` / `generated_at` と各 field 構成)を `cache.md` に確定した。
- `--reuse-cache` の検証条件は `schema_version` / `team_id` / channel ID の 3 点一致とし、不一致・検証不能時は警告して cache なしで続行する。
- 取得条件の差異と token scope は検証対象にしない。
- メッセージ raw response は cache に保存しない。

## 理由

- 事故経路(別 workspace / channel の cache 混入)だけを確実に塞ぎ、それ以外は実行継続を優先する方が、単発実行ツールとしての使い勝手と安全性のバランスが良い。

## 影響

- 0005 以来の未決事項「再利用時の整合性検証」を解決し、`index.md` の未決事項から外す。
- 実装は cache 読み込み時に 3 点検証を行い、検証結果を stderr に表示する。
- 将来差分取得を導入する場合は、メッセージ raw response の保存方針を再検討する(その際は本ログを superseded にする)。

## 後から見直す条件

- 差分取得・再実行(`index.md` 未決事項)を設計する場合。
- assets 再利用で「同じ URL だが内容が更新された」ケースが実害になった場合(現状は URL hash ベースのファイル名により同一視される)。
