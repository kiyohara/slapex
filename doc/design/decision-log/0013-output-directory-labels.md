# 0013 Output Directory Labels

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../output-format.md`

## 背景

出力 root 配下には `<workspace-label>/<channel-label>/` のような階層を作る想定である。

この directory 名を Slack API 上の ID で作るか、人間が読みやすい workspace / channel label で作るかを明確にする必要があった。

## 候補

- Slack の `team_id` や channel ID を directory 名に使う。
- workspace 名や channel 名など、人間が読みやすい label を directory 名に使う。
- 人間が読みやすい label を優先し、衝突や取得不能時だけ ID を suffix / fallback として使う。

## 検討内容

ID ベースの directory 名は一意性が高いが、利用者が出力先を見たときにどの workspace / channel の export か分かりにくい。

label ベースの directory 名は、利用者が成果物を確認しやすい。一方で、filesystem で使えない文字、空白、重複、rename された channel などへの対応が必要になる。

label を優先しつつ、必要な場合だけ ID を suffix や fallback として使えば、読みやすさと一意性を両立できる。

## 決定

出力 directory の `<workspace-label>` と `<channel-label>` は、Slack API 上の ID そのものではなく、人間が読みやすい label をもとに作る。

- `<workspace>` は token から解決した workspace 名、workspace domain、またはそれに準ずる人間向け label から作る。
- `<channel>` は確定した channel 名から作る。
- directory 名は filesystem-safe な slug に正規化する。
- label が取得できない場合や、正規化後に衝突する場合は、短い `team_id` / channel ID などを suffix または fallback として使う。
- 元の workspace / channel ID、表示名、slug は metadata / cache に記録する。

## 理由

利用者が export 結果をローカルで確認、保存、共有する際には、ID よりも workspace 名や channel 名で識別できる方が扱いやすい。

一方で、filesystem 上の安全性と一意性は必要であるため、実装では slug 化と衝突回避を行う。

## 影響

- `usage-flow.md` の出力イメージでは `<workspace-label>/<channel-label>/` と表記する。
- `--output` は出力 root を指定するだけで、workspace / channel label はツールが解決する。
- 実装では workspace / channel label を filesystem-safe に正規化する処理が必要になる。
- 実装では directory 名衝突時の suffix 付与が必要になる。
- metadata / cache には元 ID と label、実際に使った slug を記録する。

## 後から見直す条件

- channel rename 前後の export を同じ directory として扱いたい要件が出る。
- ID ベースの安定 path を優先したい CI / automation 要件が出る。
- label の slug 化規則を利用者が指定したい要件が出る。
