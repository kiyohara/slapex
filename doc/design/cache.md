# cache の扱い

このファイルには、中間ファイル `.cache/` の役割、保持・削除・再利用の方針をまとめる。

想定読者は、このツールを利用する人間と、cache 周りを実装・検証する担当者である。

本ファイルの cache の位置づけ、各ファイルの schema、再利用時の検証規則は確定仕様として扱う。実装アーキテクチャは `architecture.md` を参照する。

`.cache/` を含む出力ディレクトリ構造は `output-format.md`、利用者の操作の流れは `usage-flow.md`、決定経緯は `decision-log/0005-cache-handling.md` と `decision-log/0030-cache-schema-and-reuse-validation.md` を参照する。

## 位置づけ

`.cache/` は最終成果物ではなく、HTML と assets を作成するための中間状態として扱う。

採用理由:

- 良い点: 最終成果物と中間ファイルを分離できるため、利用者が保存・共有すべきファイルが明確になる。
- 良い点: Slack API response、emoji list、asset download manifest、user 解決結果などを再帰的に参照しやすくなり、同じ実行内での重複 API call や重複 download を減らせる。
- 良い点: `--keep-cache` を指定すれば、どこまで取得できたか、どの asset が失敗したかを調査しやすい。
- 注意点: `.cache/` には channel 名、user ID、message ID、file ID、元 URL などが入り得るため、成果物として不用意に共有しない前提にする。
- 注意点: 古い `.cache/` を再利用すると、Slack 側の更新や権限変更を反映しない stale data になる可能性がある。

## `.cache/` に置くファイル

- `.cache/assets_manifest.json`: 元 URL、ローカルパス、asset 種別、Slack file ID、emoji 名、取得成否などを保持する。
- `.cache/metadata.json`: 取得対象 workspace、channel、取得時刻、Slack API 上の ID、取得件数などを保持する。
- `.cache/slack_api_cache.json`: 同じ export 中に何度も参照する解決済み user / emoji / workspace / channel 情報を保持する。

いずれの `.cache/` ファイルにも Slack token や secret は保存しない。

## `.cache/` ファイルの schema

各ファイルは JSON とし、共通 field として `schema_version`(整数、初期値 `1`)と `generated_at`(ISO 8601 UTC)を持つ。schema を後方互換のない形で変更するときは `schema_version` を上げる。

### `metadata.json`

| field | 内容 |
|---|---|
| `tool_version` | slapex の version |
| `workspace` | `team_id`、workspace 名、domain、URL |
| `channel` | channel ID、channel 名、public/private、archived 状態、bot membership |
| `fetch` | `--days` / `--max-posts` / `--max-attachment-size` の実効値、`oldest` 境界、実行時刻 |
| `labels` | 実際に使った `<workspace-label>` / `<channel-label>` と元の表示名 |
| `counts` | timeline メッセージ数、thread 数、replies 数、assets の保存・上限超過・失敗件数 |

### `assets_manifest.json`

`assets` 配列の各要素:

| field | 内容 |
|---|---|
| `kind` | `emoji` / `og_image` / `service_icon` / `upload_thumb` / `upload_original` / `attachment` / `avatar` |
| `source_url` | 元 URL |
| `local_path` | 出力ディレクトリからの相対 path(未保存なら `null`) |
| `file_id` / `emoji_name` | Slack file ID または絵文字名(該当する場合のみ) |
| `original_name` / `mimetype` / `size_bytes` | 元の表示ファイル名と metadata |
| `status` | `saved` / `skipped_size` / `failed` |
| `error` | 失敗理由(失敗時のみ) |

### `slack_api_cache.json`

| field | 内容 |
|---|---|
| `users` | user ID → 解決済み display name / real name / avatar URL |
| `emoji` | 絵文字名 → 画像 URL または alias 先の名前 |
| `workspace` / `channel` | `auth.test` と channel 解決の結果 |

メッセージ本文の raw API response は保持しない。サイズが大きく、stale data になるリスクが高いためである(再検討の条件は決定経緯ログを参照)。

## option

| option | 目的 |
|---|---|
| `--keep-cache` | export の成否に関係なく `.cache/` を削除せず残す |
| `--reuse-cache <path>` | 以前に保存した `.cache/` を読み込み、取得済み情報や asset manifest を再利用する |

通常動作では、export の成否に関係なく `.cache/` を削除する。原因調査や cache 再利用のために残したい場合は `--keep-cache` を指定する。process kill や OS 側の異常終了では、cleanup が実行されず `.cache/` が残る可能性がある。

`--no-cache` は初期 option としては採用しない。cache の影響を排除したい場合は `--reuse-cache` を指定せず通常実行すればよく、`--keep-cache` を指定しない限り `.cache/` は削除されるためである。

## `--reuse-cache` の整合性検証

`--reuse-cache <path>` で指定された cache は、次のすべてを満たす場合だけ再利用する。

1. `schema_version` が現在の実装の値と一致する。
2. `metadata.json` の `team_id` が、今回 `auth.test` で解決した workspace と一致する。
3. `metadata.json` の channel ID が、今回確定した channel と一致する。

いずれかが不一致、または検証不能(ファイル欠落、parse 不能)な場合は、その cache を使わず、警告を表示して通常の取得にフォールバックする(エラー終了にはしない)。

取得条件(`--days` / `--max-posts`)の差異は再利用可否の判定に使わない。cache の主な再利用対象は assets manifest と user / emoji の解決結果であり、メッセージ本文は毎回取得し直すためである。token の scope 差異も事前検証しない。scope 不足による個別 asset の取得失敗は通常の失敗として manifest に記録され、HTML 上では置換表示になる。

## 未決事項

このファイルが扱う範囲に現時点の未決事項はない。以前未決だった「`.cache/` 再利用時の整合性検証」は `decision-log/0030-cache-schema-and-reuse-validation.md` で確定した。

全体の未決事項一覧は `decision-log/index.md` を参照する。
