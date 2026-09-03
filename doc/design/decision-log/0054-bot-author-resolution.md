# 0054 bot 投稿の投稿者名 / avatar 解決と APP 表示

- 状態: decided
- 作成日: 2026-09-03
- 最終更新日: 2026-09-03
- 関連: `doc/design/slack-api-usage.md`, `doc/design/html-rendering.md`, `doc/design/output-format.md`, `doc/design/cache.md`, `doc/design/decision-log/0025-slack-api-usage-policy.md`, `doc/design/decision-log/0035-avatar-assets.md`

## 背景

実 workspace の export(Issue #182)で、Slack App が投稿した message の投稿者名が app 名ではなく生の `bot_id`(`B` で始まる ID)になり、avatar も app icon ではなくイニシャル 1 文字の placeholder になっていた。Slack の画面では同じ投稿が app 名と app icon で表示される。

原因は message の形にある。slash command の `in_channel` 応答、incoming webhook、`response_url` 経由の投稿は `subtype: bot_message` と `bot_id` だけを持ち、`user` も `bot_profile` も `username` も持たない。0025 は「bot 投稿は `bot_profile` / `username` を優先」とだけ決めており、その両方が無い場合が未定義だった。0035 は「bot 投稿で profile が無い場合はイニシャル fallback」と明示的に決めていたため、実装は仕様どおりに動いていた。

`user` が無い以上 `users.info` では解決できない。Slack は `bot_id` を `bots.info` に渡して名前と icon を得る経路を用意しており、必要 scope は既存の `users:read` のままでよい。

併せて、名前と icon を解決すると bot 投稿が人間の投稿と見分けられなくなる。Slack の画面には投稿者名の右に `APP` バッジがあるが、slapex の出力には相当する表示が無い。

## 候補

表示名 / avatar の解決:

1. `bots.info` を bot ID ごとに 1 回呼び、既存の fallback 列に差し込む。
2. `bot_profile` が無い bot 投稿は現状どおりイニシャル fallback のままにする(0035 を維持)。
3. `users.list` などの一括取得で bot user も含めて解決する。

bot 投稿の識別表示:

1. 投稿者名の右に `APP` chip を出す(Slack の `APP` バッジ相当)。
2. 識別表示を持たない(名前と icon の解決だけで十分とする)。
3. avatar の角丸を変えるなど、chip 以外の視覚差で示す。

## 検討内容

- 候補 2(据え置き)は実装コストが 0 だが、実 workspace では slash command 応答が bot 投稿の大半を占め、そのすべてが「B」1 文字の avatar と生 ID の名前になる。Slack の画面との差が大きく、export の用途(あとから読み返す)を損なう。
- 候補 3(一括取得)は `bot_id` からの逆引き経路が無い。user object の `profile.bot_id` は bot user → bot_id の向きであり、今回のように bot user を持たない app には効かない。0025 が `users.list` を採らなかった理由(大規模 workspace での過剰取得)もそのまま残る。
- 候補 1 は既存の user 解決とほぼ同形になる。unique な ID ごとに 1 回、実行内 cache、失敗は警告して継続。scope 追加も manifest 変更も要らない。
- `bot_profile` が名前と icon の両方を持つ message は `bots.info` を呼ばずに済む。呼び出し回数を「解決が必要な bot ID」に絞れる。
- 既存の `bot_profile` は名前だけを読んでおり `icons` を struct に持っていなかった。そのため `bot_profile` が付いている投稿(`slapex --demo` の bot 投稿を含む)でも avatar が出ていなかった。`icons` を読むだけで解決するので、同じ変更に含める。
- 識別表示の候補 3(avatar の形を変える)は、app icon が角丸の正方形である Slack の見た目から離れる。候補 1 は Slack default 風という 0012 の方針に最も近く、CSS の chip 1 つで済む。
- bot 判定には `bot_id` / `bot_profile` に加えて `users.info` の `is_bot` が要る。bot user として `chat.postMessage` した投稿は普通の user ID を持ち、message だけからは人間と区別できないためである。Slackbot は `is_bot` が false なので自然に対象外になる。

## 決定

1. 収集: timeline と thread replies の message のうち、`user` が空で `bot_id` を持つものから unique な `bot_id` を集める。`bot_profile` が名前と icon URL の両方を持つ message は除く。
2. 解決: 集めた `bot_id` ごとに `bots.info` を 1 回呼ぶ。同一実行内で再問い合わせしない。失敗(`bot_not_found`、scope 不足、network error など)は警告して継続し、export 全体は失敗させない(`users.info` 失敗と同じ扱い)。
3. 表示名の優先順位: `users.info` の表示名 → `bot_profile.name` → `username` → `bots.info` の `name` → `bot_id` → `(unknown)`。既存の順序に `bots.info` を `bot_id` の直前へ差し込む形とし、`bot_profile.name` と `username` の順序は変えない。
4. avatar の優先順位: `users.info` の image → `bot_profile.icons`(`image_72` 優先、`image_48` fallback)→ `bots.info` の `icons`(同じ優先)→ イニシャル fallback。
5. asset: bot icon は既存の `avatar` 種別で `assets/avatars/` に保存する。新しい種別は作らない。人間の avatar と同じく public asset として扱い、`Authorization` header を送らない(`doc/guidelines/credential-scope-guidelines.md`)。
6. cache: `.cache/slack_api_cache.json` に `bots`(bot ID → 解決済み name / avatar URL)を追加し、`users` に `is_bot` を追加する。どちらも既存 key への追加であるため `schema_version` は据え置く。`--reuse-cache` は `bots` があれば `bots.info` を省略し、無ければ通常どおり呼ぶ。
7. 進捗表示: Users phase に bot の件数を含める(`resolving 13 users, 1 bot ...` / `13 users, 1 bot resolved`)。bot が 0 件の channel は従来の文言のままとする。未解決の bot は `could not resolve bot <ID>: ...` の警告を出す。
8. 識別表示: bot 投稿には投稿者名の右に `APP` chip を表示する。bot 判定は「`bot_id` を持つ」「`bot_profile` を持つ」「`users.info` の `is_bot` が true」のいずれか。chip は timeline と thread 内の返信の両方に出し、system 行と thread label の参加者 avatar には出さない。

## 理由

- Slack の公式仕様が `bot_id` → `bots.info` を名前 / icon の解決経路として明示しており、必要 scope も既存の `users:read` に収まる。既存の user 解決とほぼ同じ形で実装でき、失敗時の振る舞いも揃う。
- 既存の fallback 列を置き換えず `bot_id` の直前に差し込むだけなので、`bot_profile` / `username` を明示的に override している app の表示は変わらない。
- `bots` と `is_bot` を既存 key への追加に留めることで、旧 cache の再利用可否に影響を与えずに済む。schema version を上げると、既存 cache がすべて再利用不能になる代償が変更内容に見合わない。
- `APP` chip は Slack default 風という 0012 の方針に沿い、JavaScript 無しの静的 HTML(0012)で CSS だけで表現できる。

## 影響

- `slack-api-usage.md` の使用 API に `bots.info` を追加し、「bot 投稿の表示名と avatar」節を新設した。0025 が未定義にしていた「`bot_profile` も `username` も無い場合」がここで確定する。
- 0035 の「bot 投稿で profile が無い場合はイニシャル fallback」は、`bots.info` でも icon を取れない場合に限定される。
- `html-rendering.md` の subtype 表と表示方針に `APP` chip を追記した。
- `output-format.md` の avatar 行に bot icon の取得元を追記した。`assets/avatars/` の中身に app icon が混ざる。
- `cache.md` の `slack_api_cache.json` の表に `bots` と `users` の `is_bot` を追記した。
- 旧 cache(`bots` / `is_bot` 無し)を `--reuse-cache` した場合、`bots.info` は呼び直されるので bot 投稿の名前と icon は変わらない。ただし `is_bot` は `users.info` からしか分からず、cache 済み user は再解決しないため、bot user が投稿した message の `APP` chip だけは復元されない。これは受け入れる劣化とする。
- `slapex --demo` の bot 投稿が app icon と `APP` chip を持つようになるため、`doc/samples/` と README の preview screenshot を再生成した。
- `doc/help/slack-app-setup.md` の scope 表で `users:read` の目的に bot / app の解決を追記した。App manifest は変更しない。

## 後から見直す条件

- message 上の `icons` override(`icons.image_*` / `icons.emoji`)を反映する必要が出た場合(本決定ではスコープ外)。
- Slack の画面が `username` override を優先表示することに合わせて、`bot_profile.name` と `username` の優先順位を見直す場合(別 Issue とする)。
- bot 投稿を多く含む channel で `bots.info` の呼び出し回数が実行時間の支配項になった場合。
- `bots.info` の必要 scope が `users:read` から変更された場合。
