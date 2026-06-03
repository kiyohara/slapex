# 0020 Target Label Display

- 状態: decided
- 作成日: 2026-06-03
- 最終更新日: 2026-06-03
- 関連: `../usage-flow.md`, `0003-workspace-selection.md`, `0013-output-directory-labels.md`

## 背景

初期 CLI では workspace keyword を必須にせず、`SLACK_BOT_TOKEN` から単一 workspace install の workspace を解決する方針にしている。

この方針は CLI 入力を単純にできる一方で、利用者が secret manager や CI secrets に意図と異なる bot token を渡した場合、別 workspace に向いていることへ気づきにくい可能性がある。

既存方針では、通常実行時に期待 workspace を示す入力がないため、workspace mismatch を自動検出するエラーにはしないとしていた。そのため、必須 workspace 入力を復活させずに誤認リスクを下げる UX が必要になった。

## 候補

- workspace keyword または `--workspace` を必須入力として復活させる。
- 通常実行では token から workspace を解決し、処理中に workspace / channel の label を表示する。
- CI などのために、期待 workspace を検証する guard option を初期実装から必須または標準化する。

## 検討内容

workspace keyword を必須入力にすると、利用者が毎回 token と workspace を二重指定することになり、`0003-workspace-selection.md` の方針と衝突する。単一 workspace install では token だけで workspace が分かるため、必須入力としては重い。

処理中に workspace / channel label を表示する方式は、自動停止ではないが、利用者が実行直後、channel 選択時、取得開始前、完了時、生成 HTML の閲覧時に対象を確認できる。これは既存方針を保ったまま誤認リスクを下げられる。

表示名だけでは rename や重複に弱いため、画面表示用 label には workspace 名に加えて workspace URL または domain、短い `team_id` を含めるのが望ましい。channel も channel 名だけでなく channel ID、public/private、archived 状態、bot membership を含めると、対象確認に使いやすい。

一方、directory 用の `<workspace-label>` / `<channel-label>` は filesystem-safe な slug と衝突回避が目的である。画面表示用 label と同じ文字列にすると path として扱いにくくなるため、両者は役割を分ける。

CI で誤 token を必ず止めたい要件には、表示だけでは足りない。その要件が明確になった場合は、`--expect-team-id` や `--expect-workspace-domain` のような検証専用 option を追加するのがよい。

## 決定

通常実行では workspace keyword を必須に戻さず、token から解決した workspace と確定した channel を処理中に表示する。

表示タイミングは次を基本とする。

1. `auth.test` などで workspace が確定した直後に workspace label を表示する。
2. channel 候補を表示する場合は、候補 list の前に workspace label を表示する。
3. channel が確定した直後、履歴取得を開始する前に workspace / channel label を表示する。
4. 投稿、thread replies、assets などの進捗表示では、必要に応じて workspace / channel label を含める。
5. 完了時の summary では、出力先 path とあわせて workspace / channel label を表示する。
6. 生成した `index.html` の冒頭にも、取得対象 workspace / channel と export 実行時刻を表示する。

画面表示用 workspace label は、可能な限り workspace 名、workspace URL または domain、短い `team_id` を含める。画面表示用 channel label は、channel 名、channel ID、public/private、archived 状態、bot membership を含める。

directory 用 label は画面表示用 label と同一である必要はなく、filesystem-safe な slug と衝突回避を優先する。

## 理由

この方針は、`0003-workspace-selection.md` の「workspace は token から解決する」という入力設計を保ちながら、利用者が対象 workspace / channel を実行中に確認できるようにする。

誤 token を自動的に止める仕組みではないが、実行直後から完了後の HTML まで対象情報が見えるため、意図しない workspace / channel を扱っていることに気づきやすい。

また、画面表示用 label と directory 用 label を分けることで、対象確認の読みやすさと filesystem 上の安全性を両立できる。

## 影響

- `usage-flow.md` に処理対象の表示タイミングと表示内容を追加する。
- 実装では workspace / channel 解決結果から、画面表示用 label と directory 用 slug を別に生成または保持する必要がある。
- 生成 HTML の冒頭に workspace / channel / export 実行時刻を表示する。
- metadata には従来通り元 ID、表示名、実際に使った slug を記録する。

## 後から見直す条件

- CI や automation で workspace mismatch を必ず失敗させたい要件が出る。
- 利用者が workspace domain や `team_id` を明示的に検証する option を求める。
- Enterprise Grid / org-wide install 対応により、1 token で複数 workspace を扱う必要が出る。
