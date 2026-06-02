# 作業ブランチメモ

- ブランチ: `draft-usage-flow-how-to`
- PR: 未作成
- 最終更新: 2026-06-03

## 目的

利用者が Slack 投稿を HTML と画像ファイル一式としてローカル保存するまでの流れを、利用者向け How to Use ドキュメントの素案として整理する。

## 現在の状況

- `doc/design/usage-flow.md` を、議論用の How to Use 素案として拡張した。
- `progress.md` に本検討を in_progress として追加した。
- Slack Developer Docs の現行情報を確認し、bot token、`conversations.*`、`files:read`、`auth.test` まわりの前提を反映した。
- `../slack_posts_dumper` の PoC 実装を参照し、assets 保存対象と格納先の参考情報を `usage-flow.md` に追記した。
- Slack token と workspace の対応を確認し、通常利用では workspace keyword を必須にしない方針へ更新した。
- channel 指定と選択の方針を確定し、optional positional argument と TTY 有無で interactive / non-interactive の挙動を分ける内容を `usage-flow.md` と decision log に反映した。
- `assets_manifest.json` や `metadata.json` を最終成果物ではなく `.cache/` 配下の中間ファイルとして扱う方針を確定し、`usage-flow.md` と decision log に反映した。
- Slack App install 後に bot token が発行される時系列に合わせて、利用手順のフェーズ 1 / 2 を入れ替えた。
- 初期 CLI では subcommand を採用しない方針を確定し、コマンド例から `export` を外した。
- CLI command name を `slapex` とする方針を確定し、コマンド例を更新した。
- `--output` は省略可能とし、省略時はコマンド実行時刻を使って `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を自動作成する方針を確定した。

## 決定事項

- `usage-flow.md` の内容は議論用の素案として扱う。
- 標準絵文字、カスタム絵文字、URL preview 画像、ユーザーアップロード画像を保存対象候補として明記した。
- 初期利用手順では、workspace keyword を必須にせず、単一 workspace install の bot token から workspace を解決する。Enterprise org-wide install は初期対象外とする。詳細は `doc/design/decision-log/0003-workspace-selection.md`。
- channel は optional positional argument `[channel]` と TTY 時の interactive selection に対応する。interactive selection は候補 10 件以下に制限し、11 件以上、non-TTY、または `--no-interactive` では候補と usage を表示して非 0 exit とする。`--channel` option は採用しない。詳細は `doc/design/decision-log/0004-channel-selection.md`。
- 中間ファイルは `.cache/` 配下に置き、`--keep-cache` 指定時だけ成否に関係なく保持する。再利用 option を用意し、`--no-cache` は初期採用しない。詳細は `doc/design/decision-log/0005-cache-handling.md`。
- 初期 CLI では subcommand を採用せず、root command に option を直接指定する。詳細は `doc/design/decision-log/0006-no-subcommands-initially.md`。
- CLI command name は `slapex` とする。詳細は `doc/design/decision-log/0007-cli-command-name.md`。
- `--output` は省略可能とし、省略時はコマンド実行時刻を使って `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する。対象投稿の日時ではない。詳細は `doc/design/decision-log/0008-default-output-root.md`。
- Slack App は利用者自身が作成し、自分が扱う workspace に install して bot token を発行する前提にする。配布用 Slack App / OAuth flow は初期対象外とする。詳細は `doc/design/decision-log/0009-user-managed-slack-app.md`。
- 画像以外の添付ファイルも可能な限り保存対象に含め、`--max-attachment-size` default `10MB` を超えるものは保存せず HTML 上で置換メッセージを表示する。詳細は `doc/design/decision-log/0010-attachment-file-downloads.md`。
- 初期出力は channel 単位 HTML とし、取得範囲は `--max-posts` と `--days` の AND で制限する。`--max-posts` は親投稿数だけを数え、thread replies は含めない。詳細は `doc/design/decision-log/0011-channel-html-and-fetch-limits.md`。
- `index.html` は JavaScript なしの静的 HTML とし、`style.css` を分離する。投稿は oldest から latest、thread replies は親投稿下に展開してインデント表示する。reaction は icon と件数を表示し、reaction した user は省略する。詳細は `doc/design/decision-log/0012-html-rendering-style.md`。
- 出力 directory の `<workspace-label>` と `<channel-label>` は ID そのものではなく、人間が読みやすい label をもとに作る。衝突や取得不能時だけ ID を suffix / fallback として使う。詳細は `doc/design/decision-log/0013-output-directory-labels.md`。
- URL preview 画像は Slack API で取得できる unfurl / attachment 情報だけを使い、ツール自身による Open Graph fetch は行わない。詳細は `doc/design/decision-log/0014-url-preview-source.md`。
- 初期利用手順では public / private channel の scope を同じ設定手順で扱い、`channels:*` と `groups:*` をまとめて案内する。詳細は `doc/design/decision-log/0015-channel-scope-setup.md`。
- asset ファイル名は PoC と同じ URL hash ベースにし、人間向け情報は manifest と HTML 表示に保持する。詳細は `doc/design/decision-log/0016-asset-filenames.md`。
- ユーザーアップロード画像は thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開けるようにする。original には `--max-attachment-size` を適用する。詳細は `doc/design/decision-log/0017-uploaded-image-assets.md`。
- Slack App セットアップ手順は GitHub 上で参照できる help ページに分離し、CLI エラーは短い診断と URL 案内に絞る。詳細は `doc/design/decision-log/0018-cli-help-pages.md`。
- 設計文書は `doc/design/`、利用者向け help は `doc/help/`、作業状況は root の `progress.md` に分ける。詳細は `doc/design/decision-log/0019-document-directory-structure.md`。

## 次にやること

- 利用者向け手順として過不足がないかレビューする。
- 出力構造、assets manifest、secret manager 連携、CI artifact 化を後続検討する。
- `.cache/` の再利用条件検証を後続検討する。
- 確定方針が出たら decision log を作成または更新する。

## 検証

- ドキュメント編集のため、アプリケーション test は未実行。

## リスク・ブロッカー

- Slack App の作成画面や scope 要件は変わる可能性があるため、実装時点で Slack Developer Docs を再確認する必要がある。
- private channel の扱いは bot の参加状態と workspace 管理ポリシーに依存する。
- CI 実行を必須条件にする場合、interactive な credential flow に依存しない設計が必要。
- non-TTY で channel 候補を出力する場合、CI log に channel 名が残る可能性があるため、表示範囲や verbosity を検討する必要がある。
- `.cache/` には channel 名、user ID、message ID、file ID、元 URL などが入り得るため、共有や CI artifact 化の扱いに注意が必要。

## セッションログ

- 2026-06-02: `draft-usage-flow-how-to` ブランチを作成し、利用者向け How to Use 素案を作成した。
- 2026-06-02: `../slack_posts_dumper` の `AssetManager` / `AssetDownloader` / `EmojiResolver` を参照し、PoC の `assets/` と `assets_manifest.json` の扱いを出力案に反映した。
- 2026-06-02: Slack Developer Docs で token と workspace の関係を確認し、workspace 指定を通常必須にしない方針を decision log に記録した。
- 2026-06-02: `--output` は workspace/channel を含む最終ディレクトリではなく、出力 root として指定する例に更新した。
- 2026-06-02: channel 指定が曖昧または未指定の場合の選択動作を、TTY / non-TTY の 2 モードで整理した。
- 2026-06-02: `assets_manifest.json` / `metadata.json` を `.cache/` 配下の中間ファイルとして扱い、成功時に削除する案を整理した。
- 2026-06-02: channel 指定方法と cache の扱いを採用方針として decision log に記録した。
- 2026-06-02: Slack App 準備後に bot token を実行時注入する順序へ、利用者フローを修正した。
- 2026-06-02: 初期 CLI では subcommand を採用しない方針を decision log に記録し、usage のコマンド例を更新した。
- 2026-06-02: CLI command name を `slapex` として decision log に記録し、usage のコマンド例を更新した。
- 2026-06-02: CLI 名検討メモから、`slapex` / `slarch` / `slack2html` の候補比較を `0007-cli-command-name.md` に追記した。
- 2026-06-02: `--no-interactive` を採用し、`--no-cache` は初期 option から外す方針を `usage-flow.md` と decision log に反映した。
- 2026-06-02: `.cache/` は成功/失敗ではなく `--keep-cache` の有無だけで保持を制御する方針に更新した。
- 2026-06-02: `--output` を省略可能にし、コマンド実行時刻ベースの出力 root を自動作成する方針を decision log に記録した。対象投稿の日時ではないことも明記した。
- 2026-06-02: channel selector を `slapex general` のような optional positional argument にする案を検討した。channel が export 対象そのものであるため primary CLI syntax としては自然だが、未指定時の interactive selection を維持するなら「必須引数」ではなく optional positional selector と整理するのが正確。
- 2026-06-02: channel selector は optional positional argument として採用し、`--channel` option は初期 CLI から外す方針を decision log に反映した。
- 2026-06-02: 初期 CLI の option 名は `--output`、`--no-interactive`、`--keep-cache`、`--reuse-cache` で足りると整理し、抽象的な未決事項の「option 名」を削除した。
- 2026-06-02: interactive selection は候補 10 件以下に制限し、11 件以上の場合はより具体的な channel 引数で再実行するエラーとして扱う方針を decision log に反映した。
- 2026-06-02: Slack App は利用者自身が作成する前提とし、配布用 Slack App / OAuth flow は初期対象外とする方針を decision log に記録した。
- 2026-06-02: 画像以外の添付ファイルも可能な限り保存対象に含め、`--max-attachment-size` default `10MB` のサイズ上限超過時は HTML 上で置換メッセージを表示する方針を decision log に記録した。
- 2026-06-02: 初期 HTML は channel 単位とし、取得範囲は `--max-posts` default 1000 / max 10000 と `--days` default 30 / max 90 の AND で制限する方針を decision log に記録した。`--max-posts` は親投稿数だけを数え、thread replies が 1000 件を超えた場合は上限到達メッセージに置き換える。
- 2026-06-02: 最終 HTML は Slack default 風の見た目、oldest から latest の並び、thread replies 展開表示、絶対時刻表示、CSS 分離、JavaScript 不使用とする方針を decision log に記録した。
- 2026-06-02: reaction は icon と件数を可能な限り再現し、reaction した user の情報は省略する方針を HTML 表示仕様に追記した。
- 2026-06-02: 出力 directory の workspace / channel は ID そのものではなく、人間が読みやすい label を優先する方針を decision log に記録した。
- 2026-06-02: URL preview 画像は Slack API の unfurl / attachment 情報だけを使い、ツール自身による Open Graph fetch は行わない方針を decision log に記録した。
- 2026-06-02: Enterprise org-wide install は初期対象外とし、`--team-id` option は提供しない方針を decision log に反映した。
- 2026-06-02: public / private channel の scope は同じ設定手順で扱い、`channels:*` と `groups:*` をまとめて案内する方針を decision log に記録した。
- 2026-06-02: asset ファイル名は PoC と同じ URL hash ベースにする方針を decision log に記録し、usage の出力例を `<url-hash>.<ext>` に更新した。
- 2026-06-02: ユーザーアップロード画像は original ではなく Slack file object の thumbnail / preview 相当を保存する案を評価した。Slack docs 上の `preview` は画像 upload の preview 画像とは限らないため、実装上は `thumb_1024` から `thumb_360` までの最大 available thumbnail を優先し、original は fallback / 将来 option として扱う案が妥当。
- 2026-06-02: 添付ファイルを保存対象に含める方針との整合から、ユーザーアップロード画像は thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開く案を評価した。大きな問題はないが、original download には `--max-attachment-size` を適用し、上限超過時は thumbnail 表示のみまたは置換メッセージにする整理が必要。
- 2026-06-02: ユーザーアップロード画像は thumbnail と original の両方を保存する方針を決定し、decision log に記録した。
- 2026-06-03: `assets/uploads/` と `assets/attachments/` の分類名を評価した。現行仕様では `uploads/` はユーザーアップロード画像専用、`attachments/` は画像以外の添付ファイル用だが、利用者視点では `uploads` が画像以外の uploaded file も含むように読めるため、`images/` と `files/` など content type ベースの名称へ寄せる案が自然。
- 2026-06-03: 標準 emoji と custom emoji の保存分類を評価した。Slack docs では取得メッセージ内の標準 emoji は colon format で返り Unicode へ戻せるとされ、`emoji.list` / `emoji:read` は custom emoji 取得が主用途であるため、標準 emoji 用の `assets/emoji/` と custom emoji 用の `assets/custom-emoji/` を分ける実益は小さい。標準 emoji は Unicode fallback を基本にし、画像 asset として保存するのは custom emoji 中心にする案が自然。
- 2026-06-03: カスタム絵文字ファイルは `assets/emoji/` 配下に保存し、`assets/custom-emoji/` は設けない方針を usage と decision log に反映した。標準 emoji は原則 Unicode fallback として HTML に直接表示する。
- 2026-06-03: Slack App セットアップ手順を `doc/help/slack-app-setup.md` に分離し、CLI エラーでは詳細手順ではなく help URL を案内する方針を usage と decision log に反映した。
- 2026-06-03: Slack 公式 docs で manifest から App を作成できることを確認し、help ページに `https://api.slack.com/apps?new_app=1`、manifest 貼り付け手順、Bot Token Scopes 入りの YAML 例を追記した。
- 2026-06-03: `doc/product/` を廃止し、設計文書を `doc/design/`、利用者向け help を `doc/help/`、進捗管理を root の `progress.md` に移動した。各ディレクトリの配置判断は `README.md` を入口にする方針を decision log に記録した。
