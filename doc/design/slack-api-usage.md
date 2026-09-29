# Slack API 利用方針

このファイルには、`slapex` が使う Slack API method、pagination、rate limit 対応、user / emoji / file の解決方針をまとめる。

想定読者は、取得処理を実装・検証する担当者である。

本ファイルの方針は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

利用者の操作の流れは `usage-flow.md`、取得範囲と保存対象は `output-format.md`、cache の扱いは `cache.md` を参照する。決定経緯は `decision-log/0025-slack-api-usage-policy.md`、`decision-log/0040-credential-scope-for-asset-downloads.md`、`decision-log/0042-default-user-token.md` を参照する。

## 前提とする token と App

- デフォルト利用方法は user token(`xoxp-`)とする(`decision-log/0042-default-user-token.md`)。
- CI 実行、定期実行、チーム共通 automation、個人ユーザーに紐付けたくない運用では bot token(`xoxb-`)も正式サポートする。
- App / token は利用者自身が管理する。ツール提供側は OAuth callback、token exchange、token storage を提供しない。
- 2025-05 に発表された非 Marketplace「配布」アプリ向けの rate limit 強化(`conversations.history` / `conversations.replies` が 1 req/min、最大 15 件/req)は、internal customer-built apps は対象外であることが公式に明言されている。slapex の「利用者自身が App / token を管理する」前提は、この点でも妥当である。
- ただし将来の方針変更に備え、実装は「低い rate limit・小さい page size しか許されない環境でも、時間をかければ完走できる」ことを設計条件とする。具体的には、page size をサーバー側が縮小しても cursor 継続で正しく動き、429 応答には待機で追従する。

token type による主な違い:

| token type | 主用途 | channel 履歴へのアクセス |
|---|---|---|
| user token(`xoxp-`) | 個人が自分の参照できる channel 履歴を手元に保存する | 認可したユーザー本人が見える範囲に従う |
| bot token(`xoxb-`) | CI、定期実行、チーム共通 automation | 対応 scope に加えて、bot / app が対象 conversation の member である必要がある |

出典:

- [Rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/)
- [Clarifying rate limit changes for non-Marketplace apps](https://docs.slack.dev/changelog/2025/06/03/rate-limits-clarity/)
- [Rate limits](https://docs.slack.dev/apis/web-api/rate-limits/)

## 使用する API

| method / 経路 | 用途 | 呼び出しタイミング |
|---|---|---|
| `auth.test` | token 検証、workspace(`team_id`、名前、URL)の解決 | 起動時に 1 回 |
| `team.info` | workspace icon URL の取得 | 起動時に 1 回。取得できない場合は警告して icon なしで継続 |
| `conversations.list` | channel 一覧の取得と keyword 解決 | channel 確定まで(pagination) |
| `conversations.history` | timeline 上の親投稿の取得 | 取得範囲制限に達するまで(pagination) |
| `conversations.replies` | thread replies の取得 | thread を持つ親投稿ごと(pagination) |
| `users.info` | 投稿者・mention の表示名解決 | unique な user ID ごとに 1 回 |
| `bots.info` | bot 投稿の app 名と app icon の解決 | 解決が必要な unique な bot ID ごとに 1 回 |
| `emoji.list` | カスタム絵文字 URL の取得 | 1 回 |
| HTTP GET(`url_private_download`) | 添付ファイル・画像の download | 保存対象 asset ごと |

- `files.info` は呼ばない。file の情報は、`conversations.history` / `conversations.replies` の応答の message の `files` 配列だけから得る。message 内の file object は必要な metadata(size、mimetype、thumbnail / original URL)を含むためである。Slack の文書によると、Slack Connect channel にアップロードされたファイルについて情報を省いた file object(`"file_access": "check_file_info"`)が届くのは、Events API / RTM API で file の event が push される場合だけで、両 method はそのファイルでも完全な file object を返す。slapex は Events API / RTM API を使わない。file object に download URL などが無いファイルは `files.info` で補わず、「file / asset の取得」のとおり扱う(決定経緯は `decision-log/0066-no-files-info-fallback.md`)。
- `users.list` による一括解決は採用しない。大規模 workspace で過剰取得になるためである。多人数 channel で `users.info` の呼び出し回数が問題になる場合の最適化(閾値での `users.list` 切り替えなど)は将来検討とする。
- `team.info` はヘッダーの workspace icon 表示にだけ使う補助情報であるため、scope 不足などで失敗しても export 全体は継続する。

## pagination

- cursor ベースの pagination を使い、`response_metadata.next_cursor` が空になるまで辿る。
- `--date` では local timezone の対象日 00:00 を `conversations.history` の `oldest`、翌日 00:00 を `latest` に指定する。開始境界を含めるため `inclusive=true` も指定する。
- `--date` の応答は client 側でも `[oldest, latest)` に絞り込み、Slack API の境界挙動だけに依存しない。開始境界ちょうどは含め、終了境界ちょうどは除外する。
- `--from` / `--to` では、parse 後の開始を `oldest`、終了を `latest` に指定し、`--date` と同じ bounded range の API 境界と client 側判定を使う。境界は秒未満も含めて小数 6 桁の ts で渡し、マイクロ秒より細かい部分は切り上げる(`cli-interface.md`。決定経緯は `decision-log/0065-subsecond-range-boundaries.md`)。
- `--days` では `oldest` に「実行基準時刻 − `--days` × 24 時間」、`latest` に実行基準時刻を指定し、`--date` と同様に client 側でも半開区間を守る。実行基準時刻は、実行時刻の秒未満を切り捨てた時刻とする(`output-format.md`)。
- 1 ページの要求件数は 200 件を上限とする。サーバー側がより小さいページ(例: 15 件)しか返さなくても、cursor 継続により動作が変わらない設計とする。
- timeline 上で取得範囲と `--exclude-body-emoji` / `--exclude-reaction-emoji` の OR 条件を適用した後に残る親投稿件数が `--max-posts` に達するまで cursor を継続する。emoji filter で除外した投稿は数えない。`conversations.replies` だけに現れる thread replies は数えない(thread_broadcast は timeline に現れるため数える)。
- timeline 親投稿を emoji filter で除外した場合、その親投稿の `conversations.replies` は呼ばない。取得した replies は取得後すぐ同じ message predicate で絞り、除外した message は timestamp だけを保持する。history の取得を終えた後、親投稿が timeline に残った thread についてだけ、除外した replies を除外件数に数え、残った message を user / emoji / asset 解決へ渡す。`thread_broadcast` は timeline と thread の両方で同じ predicate を適用する。
- emoji filter を指定した場合は、timeline の `thread_broadcast` の親が timeline に入らないとき(取得範囲より古い、`--max-posts` の外にある)も、親を判定するためにその thread の `conversations.replies` を呼び、親が条件に一致すれば broadcast も除外する。この thread の replies は表示せず、thread と replies の件数、user / emoji / asset の解決、除外件数のいずれにも含めない。`conversations.replies` でだけ見た親も timeline の候補ではないため、除外件数に数えない。filter を指定しない場合、broadcast の thread は親が timeline にあるときだけ取得する。
- `conversations.replies` も同様に pagination し、1 thread あたり合計 1000 件で打ち切る(`output-format.md`)。

## rate limit とリトライ

- 一次情報は HTTP 429 と `Retry-After` ヘッダとする。`Retry-After` の指定秒数に小さな jitter を加えて待機し、再試行する。
- ただし、Slack の file(`files.slack.com`)以外の asset の download は、`Retry-After` が 60 秒を超える 429 を待たず、その asset の失敗とする。判定は `Retry-After` の秒数で行い、待ちに加える jitter を含めない(60 秒は待つ)。Slack の Web API と `files.slack.com` の `Retry-After` は、長さによらず待つ(`decision-log/0068-lane-wide-rate-limit-wait.md`)。
- `Retry-After` の無い 429、一時的な 5xx、ネットワークエラーは指数バックオフ(初回 1 秒、上限 60 秒、jitter 付き)で再試行する。
- 同一リクエストの再試行は最大 5 回とする。超過した場合、メッセージ取得系は exit code `4` で失敗し、個別 asset は失敗として記録して継続する(`cli-interface.md`)。
- 通常時も同一 method の呼び出しは 1 req/sec を目安に自主的に平準化する(公式推奨に従う)。
- asset の download は平準化しない。同時に取得する数を origin ごとに抑える(「file / asset の取得」、`decision-log/0067-parallel-asset-lanes.md`)。
- asset の download が 429 を受けると、同じ origin の download は `Retry-After` の間、新しい request を出さない。他の origin の download は止めない。その origin の同時数の上限を半分にし、その後の成功に応じて戻す(「file / asset の取得」、`decision-log/0068-lane-wide-rate-limit-wait.md`)。
- rate limit 待機中は、待機理由とおおよその待機時間を進捗表示する(`usage-flow.md` の「処理対象の表示」と同じく stderr)。

## user 解決

- 取得済みメッセージの投稿者、`channel_join` の inviter、mention に現れる unique な user ID を集め、`users.info` で表示名を解決する。
- 投稿者と inviter は、HTML でその user の表示名や avatar を使う行からだけ集める(表示の分類は `html-rendering.md` の「メッセージ種別(subtype)の表示」)。通常表示の message は投稿者を集める。tombstone と本文の無い未知 subtype の行は user を表示しないため、投稿者も inviter も集めない。
- system 行は avatar を表示しないため、`@表示名` の prefix を補完し得る行(`channel_topic` / `channel_purpose` / `channel_name` で、本文が投稿者の mention で始まらないもの)の投稿者と、`(invited by @表示名)` を補足し得る `channel_join` の行(inviter が参加した user と異なり、本文に inviter mention が無いもの)の inviter だけを集める。本文が `@表示名` で始まるかは表示名を解決するまで分からないため、その行の投稿者も集める。
- mention は、HTML で mrkdwn として変換するテキストだけから集める。通常表示の message では本文と legacy attachment の本文テキスト、system 行では本文が対象である(表示の分類は `html-rendering.md` の「メッセージ種別(subtype)の表示」)。表示しないテキスト(system 行が持つ attachment、tombstone の本文など)と、title など mrkdwn を通さない field からは集めない。
- 集める mention は label の無いもの(`<@U…>`)に限る。label 付きの mention(`<@U…|label>`)は label が空でも表示名を解決せず、label(空なら user ID)を表示する(`html-rendering.md` の「本文の変換(mrkdwn → HTML)」)ため、集めない。同じ user が投稿者や label の無い mention としても現れる場合は、そちらで集める。
- 解決結果は `.cache/slack_api_cache.json` に蓄積し、同一実行内で再問い合わせしない。
- 表示名は display name を優先し、無ければ real name、それも無ければ user ID へ fallback する。
- 解決失敗(退会ユーザーなど)は user ID をそのまま表示する。

### bot 投稿の表示名と avatar

slash command の `in_channel` 応答、incoming webhook、`response_url` 経由の投稿は `subtype: bot_message` と `bot_id` だけを持ち、`user` も `bot_profile` も `username` も持たない。この形の投稿は `users.info` では一切解決できないため、`bot_id` を `bots.info` に渡して app 名と app icon を解決する(`decision-log/0054-bot-author-resolution.md`)。

- 収集対象は、`user` が空で `bot_id` を持つ message とする。`bot_profile` が名前と icon の両方を持つ message は `bots.info` を呼ばずに済ませる。
- `bots.info` は unique な bot ID ごとに 1 回だけ呼ぶ。解決結果は `.cache/slack_api_cache.json` の `bots` に蓄積し、同一実行内で再問い合わせしない。
- 表示名の優先順位: `users.info` の表示名 → `bot_profile.name` → `username` → `bots.info` の `name` → `bot_id` → `(unknown)`。
- avatar の優先順位: `users.info` の image → `bot_profile.icons`(`image_72` 優先、`image_48` fallback)→ `bots.info` の `icons`(同じ優先)→ 頭文字 fallback。
- `bots.info` の失敗(`bot_not_found`、scope 不足、ネットワークエラーなど)は警告して継続し、export 全体は失敗させない。`users.info` の失敗と同じ扱いである。
- 必要な scope は `users:read` で、既存の manifest から追加は要らない(`../help/slack-app-setup.md`)。

## emoji 解決

- カスタム絵文字は `emoji.list` を 1 回取得する。`alias:<name>` 形式の alias は再帰的に解決し、循環を検出したら打ち切る。
- 標準絵文字の shortcode は、実装に組み込む標準絵文字データセットで Unicode 文字へ変換し、HTML に直接出力する(`output-format.md`)。
- データセットにもカスタム絵文字にも無い shortcode は、`:shortcode:` の文字列のまま表示する。
- reaction の絵文字名も同じ経路で解決する。skin tone variation は base 絵文字に寄せ、合成表示は将来検討とする。

## file / asset の取得

- message の `files` 配列を情報源とし、ファイル本体(画像では original)を `url_private_download`(無ければ `url_private`)から、画像の表示用 thumbnail を `thumb_*` から HTTP GET で取得する。
- `Authorization: Bearer` ヘッダ(Slack OAuth token)は、送信先の host が `files.slack.com`(Slack private file)の場合だけ付ける。asset の種類ではなく送信先の host で判定する(`doc/guidelines/credential-scope-guidelines.md`、`decision-log/0040-credential-scope-for-asset-downloads.md`)。
- URL preview 画像、URL preview service icon、workspace icon、avatar、emoji など、Slack private file ではない public asset URL へは `Authorization: Bearer` ヘッダを送らない。
- 画像は表示用 thumbnail と original の両方を保存する(`decision-log/0017-uploaded-image-assets.md`)。
- file object の `size` が `--max-attachment-size` を超えるものは download しない(`output-format.md`)。
- 削除済み(`mode` が `tombstone`)のファイルと、Free plan の制限で非表示(`mode` が `hidden_by_limit`)のファイルは、thumbnail を含めて何も download せず、`.cache/assets_manifest.json` にも記録しない。
- 削除済みと Free plan の制限で非表示のファイルを除き、外部サービス連携のファイル(`is_external` が true)と、download URL(`url_private_download` / `url_private`)を持たないファイルは、ファイル本体(画像では original)を download せず、サイズ上限の判定もせず、`.cache/assets_manifest.json` にも記録しない。外部サービス連携のファイルの `url_private` / `url_private_download` は Slack ではなく外部サービスを指し、download すると外部サービスが返す page(ログイン画面など)を保存しうるためである(`decision-log/0017-uploaded-image-assets.md` の 2026-09-27 の追記)。ただし、thumbnail のある画像は、thumbnail を他の画像と同じく保存し、`upload_thumb` として記録する(取得に失敗した場合の status は `failed`)。
- これらのファイルの表示は、`html-rendering.md` の「画像と添付ファイルの表示」の、ファイル本体を download しないファイルの表とその下の注記を参照する。
- asset の download にも上記のリトライ方針を適用する。失敗した asset は HTML 上で置換表示にし、export 全体は継続する。
- asset は、描画から求めた取得の計画(`decision-log/0064-two-pass-asset-planning.md`)を並列に取得する。`--reuse-cache` の copy で済むものを計画の順に先に済ませ、残りを origin(scheme + host + port)ごとの lane で download する(`decision-log/0067-parallel-asset-lanes.md`)。
  - 各 lane は最初の 1 件だけを先に出し、その download が接続を得てから残りを出す。残りはその接続を共有する(HTTP/2 の場合)。
  - 同時に download する数は、接続が HTTP/2 の origin で 16 件、それ以外の origin で 6 件、全体で 64 件までとする。
  - lane の中は、サイズが分かるもの(Slack の file の原本と添付)を大きい順に始め、サイズの分からないものをその後に計画の順に始める。4 MiB 以上のものは、lane あたり同時 4 件までとする。
  - 429 を受けた lane は、`Retry-After` の間(429 を受けた download が待つのと同じ、jitter を加えた時間)、新しい request を出さない。既に出ている request はそのまま続け、後から来た 429 がより長い待ちを求めたら延ばす。`Retry-After` の無い 429 では、lane は待たない(`decision-log/0068-lane-wide-rate-limit-wait.md`)。
  - 429 を受けると、lane の同時数の上限を半分(1 未満にはしない)にする。上限を半分にする前に枠を得ていた download の 429 では、重ねて半分にしない。半分にした後に枠を得た download が 200 の応答を 4 件受け取るごとに、上限を 1 戻す(lane が開いたときの上限まで)。
  - retry を待つ download は、待つ間は lane の枠を使わず、次の request の前に枠を取り直す。枠を取り直す download は、まだ始まっていない download より先に枠を得る。5xx とネットワークエラーでは、lane の上限を変えない。
  - Slack の file 以外の asset で、lane が待つ 429 の `Retry-After` の残り(jitter を除く)が 60 秒を超える間に request を出そうとした download は、request を出さずにその asset の失敗とする(「rate limit とリトライ」)。
- download の試行は、request を送り終えてから最終の応答 header まで 30 秒(1xx の中間応答では待ちを止めない)、body が 1 byte も進まないまま 30 秒経つと打ち切る。Slack の file(`files.slack.com`)は、全体の所要時間では打ち切らない。それ以外の asset(URL preview 画像、アイコン、avatar、emoji など)は、1 試行が、接続と応答 header の待ちを含めて 5 分を超えると打ち切る。応答 header の前の打ち切りは、ネットワークエラーと同じく再試行する。body の途中の失敗は、5 分の打ち切りを含めて再試行せず、その asset の失敗とする(`decision-log/0068-lane-wide-rate-limit-wait.md`)。Web API の呼び出しは、従来どおり 1 回 120 秒で打ち切る。
- 並列に取得しても、retry と rate limit 待機の通知、asset の警告は、計画の順(直列に取得した場合と同じ順)に stderr へ出す。そのため、後ろの asset の通知は、前の asset の取得が終わるまで出ないことがある。
- export の実行中に SIGINT(Ctrl-C)または SIGTERM を受けると、新しい download を始めず、進行中の download を止めて一時ファイルを消す(`cli-interface.md` の「exit code」)。

## 取得の並行化

この節の方針は PF-06(#278)と PF-07(#279)で実装する。実装されるまでは、Web API は 1 件ずつ直列に呼び、asset の download は Assets 工程で始まる(決定経緯は `decision-log/0069-api-method-lanes-and-prefetch.md`)。

- export の工程(`usage-flow.md` の「処理対象の表示」のフェーズ行の順)は、今の順に 1 つずつ進める。後の工程が必ず出す request は、それが確定した時点で先に出し(先行取得)、工程は自分の番に来たときにその結果を使う。工程の処理(`--max-posts`、emoji filter による除外、truncated の判定、user と bot の解決、描画)は request の結果を直列の場合と同じ順に受け取るため、結果は直列に取得した場合と同じになる。
- Web API は method ごとの lane で呼ぶ。同じ method の呼び出しは同時に 1 件までとし、来た順に、前の呼び出し(retry を含む)が終わってから、かつ前の呼び出しの開始から 1 秒以上空けて始める(「rate limit とリトライ」の平準化)。429 の `Retry-After` を待つ間は、同じ method の次の呼び出しも待つ。異なる method の呼び出しは並行する。
- 先に出すのは、今の直列の処理がこの後必ず同じ request を出すと分かった request だけとする。後で除外され得る投稿や `--max-posts` の外になり得る投稿の分は、工程で必要になるまで出さない。同じ request は、先行取得と工程の取得を合わせて 1 回だけ出す。そのため、成功した export が出す request は直列の場合と同じになる(asset の例外は下記)。先行取得は、Messages 工程の開始から(対象の確定、`--reuse-cache` の検証、出力先の作成の後)行う。

| request | 先に出す時点 |
|---|---|
| `auth.test`、`team.info`、`conversations.list`、`conversations.history` | 先に出さない。工程の順に呼ぶ |
| `emoji.list` | Messages 工程の開始時。`--reuse-cache` の cache の custom emoji を使う場合は呼ばない |
| `conversations.replies` | `conversations.history` の page で残った親投稿の thread を、その page を受け取った時点。emoji filter を指定した場合の `thread_broadcast` の thread は先に出さない |
| `users.info`、`bots.info` | 確定した message に、「user 解決」と「bot 投稿の表示名と avatar」の規則で集める ID が現れた時点。`--reuse-cache` の cache にある ID は呼ばない |
| asset の download | 確定した message の asset は、その URL が分かった時点(custom emoji の画像は custom emoji の一覧を得た後)。avatar は `users.info` / `bots.info`(または cache)の結果を得た時点、workspace icon は Messages 工程の開始時。`--reuse-cache` の copy で済むもの、file object の `size` がサイズの上限を超えるものは先に download しない |

- 確定した message は次のとおりとする。
  - emoji filter を指定しない場合: `conversations.history` が残した message(filter と `--max-posts` の内のもの)と、取得した thread の replies。
  - emoji filter を指定した場合: thread に属さない message は、`conversations.history` が残した時点で確定する。thread に属する message(親、`thread_broadcast`、replies)は、Messages 工程の終わりに確定する。thread の除外は、`conversations.replies` で見た親や、後の page に来る親で決まることがあるためである(「pagination」)。
- 先に出した request の成否と通知は、工程がその結果を受け取るまで出さない。工程は結果を直列の場合と同じ位置で同じように扱う(致命的な失敗は export を終え、`users.info` と `bots.info` の失敗は警告して続け、asset の失敗は置換表示にする)。retry と rate limit 待機の通知は、工程が結果を受け取るときにまとめて出し、その後の通知はそのまま出す。stderr の行の内容と順序、exit code は、直列に取得した場合と同じになる(`cli-interface.md` の「exit code」)。先に出した request の失敗を理由に、工程がそこに来る前に export を止めることはしない。
- 先に download した asset は、内容を一時ファイルに置き、download の結果(内容 hash、Content-Type、内容から判別した形式、サイズ、または失敗)を保つ。Assets 工程の取得(「file / asset の取得」の計画)が同じ URL を扱うときに、保った結果と計画の kind・meta から、直列に download した場合と同じ名前と manifest の entry を作って `assets/` へ移し、失敗の警告も計画の kind で出す。計画の kind と meta は URL ごとの最初の要求のもので、先に download したときの要求と食い違うことがある(custom emoji の alias の名前など)が、download の request は URL だけで決まるため、取り直さない。
- サイズの上限は計画の kind で判定する。先の download が計画の上限を超えていれば、上限で止めた場合と同じに扱う。先の download が計画の kind より小さい上限で止まった場合は取り直し、計画がサイズの上限で download しない URL の先の結果は使わない。このため、同じ URL を、kind や file object のサイズが食い違う複数の要求が求める場合に限り、成功した export の request が直列より 1 件増えることがある。
- export が終わるとき(成功、失敗、SIGINT / SIGTERM による中断)は、終わっていない先行取得を止め、止まるのを待つ。工程が受け取らなかった結果、通知、警告は捨て、`assets/` へ移さなかった一時ファイルは消す。失敗した export の出力先にも、直列の場合に無いファイルは残らない。

## 取得の整合性

- export は単発実行であり、実行中に Slack 側で起きた更新との完全な整合は保証しない。取得開始時点のスナップショットとして扱う。
- API の返却順には依存せず、レンダリング時に `ts` で昇順(oldest → latest)に整列する。
- timeline と thread の両方に現れるメッセージ(thread_broadcast)は、Slack の表示と同様に両方へ表示する(`html-rendering.md`)。

## 参考

- Slack Developer Docs: [Tokens](https://docs.slack.dev/authentication/tokens/)
- Slack Developer Docs: [Slack Connect: Additional check required to access file info](https://docs.slack.dev/apis/slack-connect/#check_file_info)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test)
- Slack Developer Docs: [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list)
- Slack Developer Docs: [`conversations.history`](https://docs.slack.dev/reference/methods/conversations.history)
- Slack Developer Docs: [`conversations.replies`](https://docs.slack.dev/reference/methods/conversations.replies/)
- Slack Developer Docs: [`users.info`](https://docs.slack.dev/reference/methods/users.info)
- Slack Developer Docs: [`bots.info`](https://docs.slack.dev/reference/methods/bots.info)
- Slack Developer Docs: [`bot_message` subtype](https://docs.slack.dev/reference/events/message/bot_message)
- Slack Developer Docs: [`emoji.list`](https://docs.slack.dev/reference/methods/emoji.list)
- Slack Developer Docs: [Rate limits](https://docs.slack.dev/apis/web-api/rate-limits/)
