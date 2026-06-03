# cache の扱い

このファイルには、中間ファイル `.cache/` の役割、保持・削除・再利用の方針をまとめる。

想定読者は、このツールを利用する人間と、cache 周りを実装・検証する担当者である。

この内容は議論用の素案であり、実装アーキテクチャ、オプション名、出力ディレクトリ構造は未確定である。

`.cache/` を含む出力ディレクトリ構造は `output-format.md`、利用者の操作の流れは `usage-flow.md`、決定経緯は `decision-log/0005-cache-handling.md` を参照する。

## 位置づけ

`.cache/` は最終成果物ではなく、HTML と assets を作成するための中間状態として扱う。

採用理由:

- 良い点: 最終成果物と中間ファイルを分離できるため、利用者が保存・共有すべきファイルが明確になる。
- 良い点: Slack API response、emoji list、asset download manifest、user 解決結果などを再帰的に参照しやすくなり、同じ実行内での重複 API call や重複 download を減らせる。
- 良い点: `--keep-cache` を指定すれば、どこまで取得できたか、どの asset が失敗したかを調査しやすい。
- 注意点: `.cache/` には channel 名、user ID、message ID、file ID、元 URL などが入り得るため、成果物として不用意に共有しない前提にする。
- 注意点: 古い `.cache/` を再利用すると、Slack 側の更新や権限変更を反映しない stale data になる可能性がある。

## `.cache/` に置くファイル

- `.cache/assets_manifest.json`: 元 URL、ローカルパス、asset 種別、Slack file ID、emoji 名、取得成否などを保持する候補として扱う。
- `.cache/metadata.json`: 取得対象 workspace、channel、取得時刻、Slack API 上の ID、取得件数などを保持する候補として扱う。
- `.cache/slack_api_cache.json`: 同じ export 中に何度も参照する Slack API response や解決済み user / emoji / channel 情報を保持する候補として扱う。

いずれの `.cache/` ファイルにも Slack token や secret は保存しない。

## option

| option | 目的 |
|---|---|
| `--keep-cache` | export の成否に関係なく `.cache/` を削除せず残す |
| `--reuse-cache <path>` | 以前に保存した `.cache/` を読み込み、取得済み情報や asset manifest を再利用する |

通常動作では、export の成否に関係なく `.cache/` を削除する。原因調査や cache 再利用のために残したい場合は `--keep-cache` を指定する。process kill や OS 側の異常終了では、cleanup が実行されず `.cache/` が残る可能性がある。

`--reuse-cache` を使う場合は、cache が同じ workspace、channel、token の見える権限範囲、取得条件に対応しているかを検証する必要がある。検証できない cache は使わず、再取得する。

`--no-cache` は初期 option としては採用しない。cache の影響を排除したい場合は `--reuse-cache` を指定せず通常実行すればよく、`--keep-cache` を指定しない限り `.cache/` は削除されるためである。

## 未決事項

- `.cache/` 再利用時の整合性検証方法(workspace / channel / token scope / 取得条件など、どの metadata の不一致を再利用不可とみなすか)。

全体の未決事項一覧は `decision-log/index.md` を参照する。
