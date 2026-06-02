# 方針決定ログ index

このディレクトリには、各種方針決定に伴う検討内容を記録していく。

この `index.md` は、AI agent と人間が最初に読む入口である。詳細な検討内容は個別ログファイルに分け、ここには現在有効な主要方針、未決事項、参照先だけを簡潔に記載する。

記録方法の詳細は `../../guidelines/decision-log-guidelines.md` を参照する。

## 現在有効な主要方針

| ID | 状態 | 主題 | 現在の結論 | 詳細 |
|---|---|---|---|---|
| 0001 | decided | 試作プロジェクトとの関係 | 既存の試作プロジェクトは参考資料として扱うが、そこでの方針を本リポジトリへ暗黙採用しない | [0001-relationship-to-prototype.md](0001-relationship-to-prototype.md) |
| 0002 | decided | 開発基盤として Docker / Docker Compose を前提とする | サプライチェーン攻撃対策と環境再現性のため、開発コマンドは原則コンテナ経由とする(確定) | [0002-docker-compose-baseline.md](0002-docker-compose-baseline.md) |
| 0003 | decided | workspace 指定の扱い | 初期利用手順では workspace keyword を必須にせず、単一 workspace install の bot token から workspace を解決する。Enterprise org-wide install は初期対象外とする | [0003-workspace-selection.md](0003-workspace-selection.md) |
| 0004 | decided | channel 指定と選択 | channel は optional positional argument `[channel]` として受け取る。interactive selection は候補 10 件以下に制限し、11 件以上、non-TTY、または `--no-interactive` では候補と usage を表示して非 0 exit とする | [0004-channel-selection.md](0004-channel-selection.md) |
| 0005 | decided | cache の扱い | 中間ファイルは `.cache/` 配下に置き、`--keep-cache` 指定時だけ成否に関係なく保持する。再利用 option を用意し、`--no-cache` は初期採用しない | [0005-cache-handling.md](0005-cache-handling.md) |
| 0006 | decided | 初期 CLI の subcommand | 初期 CLI では subcommand を採用せず、root command に option を直接指定する。必要になったら将来再検討する | [0006-no-subcommands-initially.md](0006-no-subcommands-initially.md) |
| 0007 | decided | CLI command name | CLI command name は `SLack Posts EXporter` の略として `slapex` とする | [0007-cli-command-name.md](0007-cli-command-name.md) |
| 0008 | decided | output root の既定値 | `--output` は省略可能とし、省略時はコマンド実行時刻を使って `slapex-<yyyymmdd>-<hhmm>` 形式の出力 root を作成する | [0008-default-output-root.md](0008-default-output-root.md) |
| 0009 | decided | Slack App の作成主体 | 初期利用手順では、利用者自身が自分用の Slack App を作成し、workspace に install して bot token を発行する | [0009-user-managed-slack-app.md](0009-user-managed-slack-app.md) |
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

## 未決事項

| 主題 | 状態 | 次に決めること | 関連ログ |
|---|---|---|---|

## 運用メモ

- AI agent はまずこの `index.md` を読み、必要な個別ログだけを参照する。
- 詳細ログは 1 テーマ 1 ファイルを原則とする。
- 決定が変わった場合は古いログを消さず、状態を `superseded` にして新しいログへリンクする。
- ファイル名は `<連番>-<短い英語slug>.md` を基本とする。
