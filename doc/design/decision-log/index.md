# 方針決定ログ index

このディレクトリには、各種方針決定に伴う検討内容を記録していく。

この `index.md` は、AI agent と人間が最初に読む入口である。詳細な検討内容は個別ログファイルに分け、ここには現在有効な主要方針、未決事項、参照先だけを簡潔に記載する。

記録方法の詳細は `../../guidelines/decision-log-guidelines.md` を参照する。

なお、仕様の正本は `doc/design/` 直下の各 spec 文書であり、decision log と本 `index.md` はその確定経緯を辿る参考ログである。decision log を仕様の正本として扱わない(詳細は `../../guidelines/decision-log-guidelines.md` の「正本と参照の関係」)。

## 現在有効な主要方針

| ID | 状態 | 主題 | 現在の結論 | 詳細 |
|---|---|---|---|---|
| 0001 | decided | 試作プロジェクトとの関係 | 既存の試作プロジェクトは参考資料として扱うが、そこでの方針を本リポジトリへ暗黙採用しない | [0001-relationship-to-prototype.md](0001-relationship-to-prototype.md) |
| 0002 | decided | 開発基盤として Docker / Docker Compose を前提とする | サプライチェーン攻撃対策と環境再現性のため、開発コマンドは原則コンテナ経由とする(確定) | [0002-docker-compose-baseline.md](0002-docker-compose-baseline.md) |
| 0003 | superseded | workspace 指定の扱い | bot token 固定の前提は 0042 で上書き。workspace keyword を必須にせず token から workspace を解決する考え方は維持する | [0003-workspace-selection.md](0003-workspace-selection.md), [0042-default-user-token.md](0042-default-user-token.md) |
| 0004 | decided | channel 指定と選択 | channel は optional positional argument `[channel]` として受け取る。interactive selection は候補 10 件以下に制限し、11 件以上、non-TTY、または `--no-interactive` では候補と usage を表示して非 0 exit とする | [0004-channel-selection.md](0004-channel-selection.md) |
| 0005 | decided | cache の扱い | 中間ファイルは `.cache/` 配下に置き、`--keep-cache` 指定時だけ成否に関係なく保持する。再利用 option を用意し、`--no-cache` は初期採用しない | [0005-cache-handling.md](0005-cache-handling.md) |
| 0006 | decided | 初期 CLI の subcommand | 初期 CLI では subcommand を採用せず、root command に option を直接指定する。必要になったら将来再検討する | [0006-no-subcommands-initially.md](0006-no-subcommands-initially.md) |
| 0007 | decided | CLI command name | CLI command name は `SLAck Posts EXporter` の略として `slapex` とする | [0007-cli-command-name.md](0007-cli-command-name.md) |
| 0008 | decided | output root の既定値 | `--output` は省略可能とし、省略時はコマンド実行時刻を使って `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する | [0008-default-output-root.md](0008-default-output-root.md) |
| 0009 | superseded | Slack App の作成主体 | bot token を通常利用の前提にする方針は 0042 で上書き。利用者自身が Slack App / token を管理する考え方は維持する | [0009-user-managed-slack-app.md](0009-user-managed-slack-app.md), [0042-default-user-token.md](0042-default-user-token.md) |
| 0010 | decided | 添付ファイルの保存対象 | 画像以外の添付ファイルも可能な限り保存対象に含め、`--max-attachment-size` default `10MB` を超えるものは保存せず HTML 上で置換メッセージを表示する | [0010-attachment-file-downloads.md](0010-attachment-file-downloads.md) |
| 0011 | decided | HTML 粒度と取得範囲 | 初期出力は channel 単位 HTML とし、取得範囲は `--max-posts` と `--days` の AND で制限する。`--max-posts` は親投稿数だけを数え、thread replies は 1 thread 1000 件まで扱う | [0011-channel-html-and-fetch-limits.md](0011-channel-html-and-fetch-limits.md) |
| 0012 | decided | HTML 表示仕様 | `index.html` は JavaScript なしの静的 HTML とし、`style.css` を分離する。投稿は oldest から latest、thread replies は親投稿下に展開してインデント表示する | [0012-html-rendering-style.md](0012-html-rendering-style.md) |
| 0013 | decided | 出力 directory の label | `<workspace-label>` と `<channel-label>` は ID そのものではなく人間が読みやすい label から作り、衝突や取得不能時だけ ID を suffix / fallback として使う | [0013-output-directory-labels.md](0013-output-directory-labels.md) |
| 0014 | decided | URL preview の取得元 | URL preview 画像は Slack API で取得できる unfurl / attachment 情報だけを使い、ツール自身による Open Graph fetch は行わない | [0014-url-preview-source.md](0014-url-preview-source.md) |
| 0015 | decided | channel scope 設定 | 初期利用手順では public / private channel の scope を同じ設定手順で扱い、`channels:*` と `groups:*` をまとめて案内する | [0015-channel-scope-setup.md](0015-channel-scope-setup.md) |
| 0016 | decided | asset ファイル名 | asset ファイル名は PoC と同じ URL hash ベースにし、人間向け情報は manifest と HTML 表示に保持する | [0016-asset-filenames.md](0016-asset-filenames.md) |
| 0017 | decided | uploaded image assets | ユーザーアップロード画像は thumbnail と original の両方を保存し、HTML では thumbnail を表示してクリックで original を開けるようにする | [0017-uploaded-image-assets.md](0017-uploaded-image-assets.md) |
| 0018 | decided | CLI help pages | Slack App セットアップ手順は GitHub 上で参照できる help ページに分離し、CLI エラーは短い診断と URL 案内に絞る | [0018-cli-help-pages.md](0018-cli-help-pages.md) |
| 0019 | decided | document directory structure | 設計文書は `doc/design/`、利用者向け help は `doc/help/`、作業状況は root の `progress.md` に分ける | [0019-document-directory-structure.md](0019-document-directory-structure.md) |
| 0020 | decided | 処理対象 workspace / channel の表示 | token から解決した workspace と確定した channel を実行中、完了時、生成 HTML に表示する。画面表示用 label と directory 用 label は役割を分ける | [0020-target-label-display.md](0020-target-label-display.md) |
| 0021 | decided | 仕様文書の分割 | `usage-flow.md` を利用者の操作の流れに絞り、出力形式・HTML 表示仕様・cache を `output-format.md` / `html-rendering.md` / `cache.md` に分割する | [0021-spec-document-split.md](0021-spec-document-split.md) |
| 0022 | decided | 正本と参照の関係の明文化 | 仕様の正本は `doc/design/` 直下の spec 文書、decision log は参考ログと位置づけ、guideline / template / index / rule shim に明記して取り違えを防ぐ | [0022-spec-vs-decision-log-authority.md](0022-spec-vs-decision-log-authority.md) |
| 0023 | decided | project MCP config | secret-free な MCP host 設定は project 設定として git 管理し、`github-op-integrated` の secret reference は MCP 専用 `.config/github-op-integrated.conf` で管理する | [0023-project-mcp-config.md](0023-project-mcp-config.md) |
| 0024 | decided | CLI option と exit code | option 一覧と default を `cli-interface.md` に確定。token は環境変数のみ、exit code は 0/1/2/3/4 の 5 分類、stdout は出力 path のみで進捗・診断は stderr | [0024-cli-options-and-exit-codes.md](0024-cli-options-and-exit-codes.md) |
| 0025 | decided | Slack API 利用方針 | 使用 method、cursor pagination、429 + Retry-After 遵守と指数バックオフを確定。2025 年の非 Marketplace 配布アプリ向け rate limit 強化は internal App が対象外であることを確認 | [0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md) |
| 0026 | decided | mrkdwn → HTML 変換 | 本文は `text`(mrkdwn)を正とし対応表に従って変換。全テキストをエスケープ後に自前マークアップのみ生成し、href は http/https のみ。`blocks` 完全対応は将来検討 | [0026-mrkdwn-html-conversion.md](0026-mrkdwn-html-conversion.md) |
| 0027 | decided | message subtype の表示 | 通常表示 / システム行 / 置換表示(tombstone)/ 未知 subtype fallback の 4 分類で表示。thread_broadcast は timeline と thread の両方に表示し、`channel_join` は inviter があれば補足表示 | [0027-message-subtypes-rendering.md](0027-message-subtypes-rendering.md) |
| 0028 | decided | 時刻表示とタイムゾーン | 実行環境の local timezone で `YYYY-MM-DD HH:MM` 表示。ヘッダに使用 timezone を明記し、title 属性に ISO 8601 UTC を併記 | [0028-timestamp-timezone-display.md](0028-timestamp-timezone-display.md) |
| 0029 | decided | directory label の正規化規則 | workspace は domain 優先、channel は NFC + 禁止文字置換 + 64 文字制限。Unicode は保持し、空・衝突時だけ ID を使う | [0029-directory-label-rules.md](0029-directory-label-rules.md) |
| 0030 | decided | cache schema と再利用検証 | `.cache/` 3 ファイルの schema を確定。`--reuse-cache` は schema_version / team_id / channel ID の 3 点一致で再利用し、不一致は警告して通常取得にフォールバック | [0030-cache-schema-and-reuse-validation.md](0030-cache-schema-and-reuse-validation.md) |
| 0031 | decided | 対象プラットフォーム | macOS / Linux を対象とし、CI は GitHub Actions Linux runner を想定。Windows は初期対象外として将来検討に記録 | [0031-supported-platforms.md](0031-supported-platforms.md) |
| 0032 | decided | 実装言語 | Go(1.26 系)を採用。単一バイナリ配布とクロスコンパイル、標準ライブラリの守備範囲、依存最小化で総合判断。Rust / TS / Python / Ruby は理由付きで見送り | [0032-implementation-language.md](0032-implementation-language.md) |
| 0033 | decided | Go の依存方針とライブラリ | stdlib-first。外部依存は huh(TTY 選択、module path は charm.land/huh/v2)と x/term、x/text(NFC 正規化)に限定。Slack client は自前 thin client、CLI は標準 flag、HTML は html/template、絵文字データは go:embed | [0033-go-dependency-policy.md](0033-go-dependency-policy.md) |
| 0034 | decided | 配布方式 | GitHub Releases に darwin / linux × amd64 / arm64 の単一バイナリと checksum を添付する。v1.0.0 はこの方式で公開済み。Homebrew tap は cask として 0041 で後続実装方針を確定 | [0034-distribution-method.md](0034-distribution-method.md) |
| 0035 | decided | avatar 画像の保存対象化 | 登場する投稿者の avatar を `assets/avatars/` に保存し、取得不可時はイニシャル表示に fallback。PoC で顕在化した表示仕様と保存仕様のギャップを解消 | [0035-avatar-assets.md](0035-avatar-assets.md) |
| 0036 | decided | v1.0 リリースのスコープ | テスト整備・CI・`--reuse-cache`・TZ 課題・actor 表示・リリース整備・README / LICENSE・総合 E2E を含む本実装プランは v1.0.0 公開で完了。差分取得などの未決事項は post-v1 に据え置き | [0036-v1-release-scope.md](0036-v1-release-scope.md) |
| 0037 | decided | Issue 駆動タスク消化の運用方式 | 1 Issue = 1 ブランチ = 1 PR の自己完結 Issue を直列消化する。共通手順は `doc/guidelines/issue-driven-task-execution.md`、索引は `progress.md` のタスク表、merge はユーザーが行う | [0037-issue-driven-task-execution.md](0037-issue-driven-task-execution.md) |
| 0038 | decided | ライセンス選定 | 公開ライセンスは MIT(copyright `2026 Tomokazu Kiyohara`)に確定。repo root の `LICENSE` を `README.md` のライセンス節から参照する。特許付与が必要な利用形態が出れば Apache-2.0 を再検討 | [0038-license-selection.md](0038-license-selection.md) |
| 0039 | decided | decision log の対象読者とリンク方針 | decision log は開発時参照の内部ドキュメント。利用者向けドキュメント(`README.md` / `doc/help/`)からは直接リンクしない。`doc/design/` spec からの決定経緯参照(0022)は許容 | [0039-decision-log-audience.md](0039-decision-log-audience.md) |
| 0040 | decided | asset download 時の認証情報送信先 | Slack OAuth token は Slack Web API と Slack private file URL(`files.slack.com`)にだけ送る。URL preview 画像、service icon、avatar、emoji などの public asset URL には送らない | [0040-credential-scope-for-asset-downloads.md](0040-credential-scope-for-asset-downloads.md) |
| 0041 | decided | 導入手段の拡充 | 配布の主経路は GitHub Releases 単一バイナリ(0034)を維持しつつ、導入補助として install script と専用 tap repo 上の Homebrew cask を提供する。v1.1.2 release でも tap repo への cask 自動更新と Homebrew upgrade を確認済み | [0041-install-convenience.md](0041-install-convenience.md) |
| 0042 | decided | Slack token 種別の基本方針 | デフォルト利用方法は user token とし、bot token も CI / automation 向けに正式サポートする。環境変数名は `SLACK_TOKEN` とする | [0042-default-user-token.md](0042-default-user-token.md) |
| 0043 | decided | interactive selection の stream 判定 | interactive selection は controlling terminal (`/dev/tty`) を開けるときだけ開始し、prompt の入出力も `/dev/tty` に固定する。stdin / stdout / stderr の TTY 状態は判定に使わない。stdout は成功時 path 専用のまま維持する | [0043-interactive-selection-streams.md](0043-interactive-selection-streams.md) |
| 0044 | decided | SLACK_TOKEN 未設定時の対話入力 | `SLACK_TOKEN` 未設定時に controlling terminal (`/dev/tty`) を開けて `--no-interactive` でないときだけ、no-echo の token 入力プロンプトを `/dev/tty` に出す。入力値は in-memory のみで、ファイル・cache・log・HTML に保存しない。CI / pipe / `--no-interactive` は従来どおり未設定エラー(exit 3) | [0044-interactive-token-prompt.md](0044-interactive-token-prompt.md) |
| 0045 | decided | CLI 出力スタイル | stderr の進捗・診断は styled(フェーズ行 + 状態記号 + braille spinner)/ plain(ASCII prefix の行追記のみ)の 2 モード。plain 化条件は `--no-color` / `NO_COLOR` / `TERM=dumb` / `CI` / 非 TTY。`--no-color` を正式 option 化。実装は `internal/ui` の自前 ANSI helper で新規依存なし | [0045-cli-output-style.md](0045-cli-output-style.md) |
| 0046 | decided | Slack API 接続先の内部 override | 内部環境変数 `SLAPEX_API_BASE_URL` が非空のときだけ base URL を差し替える。デモ録画・fixture 実行用の内部用途で、ユーザー向けドキュメント / `--help` には載せない。未指定時に default のままである negative test と指定時のみ差し替わる positive test を維持する。録画は引き続きこの機構を使い、利用者向けの token 不要試用は 0047 の `--demo` で別に提供する | [0046-api-base-url-override.md](0046-api-base-url-override.md) |
| 0047 | decided | token 不要の demo 実行 | 公開 option `--demo` を追加し、token / Slack App なしで同梱の架空 fixture(`internal/demo`)を in-process fake server 経由で export する。fixture / fake server は `tools/gensample` から `internal/demo` へ切り出して共有。locale で ja / en を自動選択し、pacing は省略。stdout 契約と credential-scope(fake token を loopback にだけ送る)を維持 | [0047-token-free-demo-run.md](0047-token-free-demo-run.md) |
| 0048 | decided | ドキュメント文体 | 利用者向け(README / `doc/help/`)はですます調、開発者・AI agent 向けは常体。トーンは簡潔・中立で絵文字なし、技術用語は英語表記のまま。正本は `doc/guidelines/document-style-guidelines.md`。既存文書への一括適用は #126 | [0048-document-style.md](0048-document-style.md) |
| 0049 | decided | 利用者→開発者ドキュメントのリンク方針 | 利用者向け(README / `doc/help/`)の本文から開発者向け(`doc/design/` spec、decision log 等)へ直接リンクしない。必要時は文末の脚注に「開発者向け」と明示して置く。decision log 直リンク禁止(0039)は本方針に包含。正本は `document-style-guidelines.md`、既存棚卸しは #123 | [0049-user-doc-to-dev-doc-links.md](0049-user-doc-to-dev-doc-links.md) |

## 未決事項

| 主題 | 状態 | 次に決めること | 関連ログ |
|---|---|---|---|
| 差分取得と再実行 | open | 既存出力への上書き、差分取得、再実行時の扱い(メッセージ raw response の cache 保存方針も含む) | [0011-channel-html-and-fetch-limits.md](0011-channel-html-and-fetch-limits.md), [0030-cache-schema-and-reuse-validation.md](0030-cache-schema-and-reuse-validation.md) |
| CI artifact 化 | open | CI で生成した HTML / assets / cache を artifact として保存・共有する方法 | [0008-default-output-root.md](0008-default-output-root.md) |
| thread replies を含む全体取得量 | open | 取得前の見込み表示、または親投稿と thread replies を合わせた全体上限を設けるか | [0011-channel-html-and-fetch-limits.md](0011-channel-html-and-fetch-limits.md) |
| workspace mismatch の guard option | open | CI 等で誤 token を強制停止する要件が出た場合に `--expect-team-id` / `--expect-workspace-domain` を追加するか | [0020-target-label-display.md](0020-target-label-display.md) |
| rich text(`blocks`)の完全レンダリング | open | `text` fallback で再現できない投稿が問題になった場合に、`blocks` の構造レンダリングへ対応するか | [0026-mrkdwn-html-conversion.md](0026-mrkdwn-html-conversion.md) |
| 出力量の制御 option | open | `--quiet` / `--verbose` を追加するか(`--no-color` は 0045 で正式 option 化済み) | [0024-cli-options-and-exit-codes.md](0024-cli-options-and-exit-codes.md), [0045-cli-output-style.md](0045-cli-output-style.md) |
| user 解決の最適化 | open | 多人数 channel で `users.info` の呼び出し回数が問題になった場合の一括取得への切り替え | [0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md) |
| Windows 対応 | open | 需要が確認できた場合の対応範囲(ファイル名制約、コンソール挙動、配布 target) | [0031-supported-platforms.md](0031-supported-platforms.md) |

解決済みの旧未決事項: 「`.cache/` 再利用時の整合性検証」は [0030-cache-schema-and-reuse-validation.md](0030-cache-schema-and-reuse-validation.md) で確定した。「code block 内の URL 構文の表示」は [0026-mrkdwn-html-conversion.md](0026-mrkdwn-html-conversion.md) の追記で方針を確定し、修正済み。「コンテナ実行時のタイムゾーン」は [0028-timestamp-timezone-display.md](0028-timestamp-timezone-display.md) の追記で、dev / E2E の Docker Compose 実行時に host の `TZ` を forward し、専用 CLI option は導入しない方針として確定した。「system メッセージの actor 表示」は [0027-message-subtypes-rendering.md](0027-message-subtypes-rendering.md) の追記で、`channel_topic` / `channel_purpose` / `channel_name` の actor prefix 補完として確定した。「Homebrew tap」は [0041-install-convenience.md](0041-install-convenience.md) で、導入補助の方針として確定し、v1.1.2 release でも tap repo への cask 自動更新と Homebrew upgrade を確認済み。「user token default への移行」は [0042-default-user-token.md](0042-default-user-token.md) の 2026-06-26 追記と Issue #81 対応で、`SLACK_TOKEN` 専用・user token default・bot token support・v1.0.1 方針として確定した。

## 運用メモ

- AI agent はまずこの `index.md` を読み、必要な個別ログだけを参照する。
- 詳細ログは 1 テーマ 1 ファイルを原則とする。
- 決定が変わった場合は古いログを消さず、状態を `superseded` にして新しいログへリンクする。
- ファイル名は `<連番>-<短い英語slug>.md` を基本とする。
