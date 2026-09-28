# 作業ブランチメモ

- ブランチ: `claude/issue-289-sv54s1`(cloud session が指定。Issue の推奨ブランチ名は `files-info-fallback-policy`)
- PR: 未作成
- 最終更新: 2026-09-28

## 目的

Issue #289。設計文書と decision log 0025 は、message の file object に情報が欠けている場合に `files.info` で補うとしているが、実装は `files.info` を呼ばない。方針(仕様から外すか、補完を実装するか)を決め、設計文書、decision log、実装を一致させる。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 24 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 19 件目である。

## 現在の状況

- 依存は無い(Issue の「依存・順序」)。main `b0a42c9`(PR #288 の merge)から作業した。
- 方針は、推奨を添えて thread のカードで伺い、ユーザーが A(仕様から外す)を選んだ(2026-09-28 22:55Z。「決定事項」)。
- 設計文書と decision log を直し、Issue の「検証」を実行した(「検証」)。
- 次は PR の作成(draft)と採番。

## 決定事項

### 着手時の現行コードと文書

Issue の行番号は main `48c0198` 時点の記載である。main `b0a42c9` では、`slack-api-usage.md` は同じ位置にあり、`output-format.md` は PR #288 の変更で 2 行ずれていた。

- `doc/design/slack-api-usage.md` の「使用する API」の表の下(L46): 「`files.info` は原則呼ばない。... file object に必要情報が欠けている場合だけ補完として呼ぶ。」
- `doc/design/output-format.md` の「保存する assets」の表(L61 / L62。Issue の記載は L59 / L60): ユーザーがアップロードした画像と画像以外の添付ファイルの取得元に `files.info`。
- decision log 0025 の「決定」: 「file は message 内 file object を正とし、`files.info` は欠損時の補完のみ」。同じ「決定」の使用 API の一覧に `files.info` は無い。
- `doc/design/usage-flow.md` の「参考」(L356): `files.info` の Slack の reference への link。
- 実装: Go のコードに `files.info` の呼び出しは無い。`internal/slack/api.go` の `File` は `file_access` を持たない。`internal/export/message_view.go` の `addFiles` / `addImage` / `addAttachmentFile` は、download URL の無いファイルを補完せずに `html-rendering.md` の「画像と添付ファイルの表示」の表のとおり表示する。

### 公式文書の確認(作業内容の 2 つ目)

2026-09-28 に Slack の文書を確かめた。

- Slack Connect の文書の「Additional check required to access file info (`check_file_info`)」の節(https://docs.slack.dev/apis/slack-connect/#check_file_info)は、情報を省いた file object(`"file_access": "check_file_info"`)が届くのは Events API / RTM API を listen する app であるとし、次のように書く。「When accessing conversation files and messages using conversations.history or conversations.replies, full file objects are returned.」「It is only when file events are pushed to your app that you will need an additional API call to view a file's properties.」
- file object の文書(https://docs.slack.dev/reference/objects/file-object)も、`check_file_info` を Events API / legacy RTM API の payload の話として書き、詳細は上記の Slack Connect の文書を参照させる。
- `files.info` の reference: scope は `files:read`(bot / user とも)、rate limit は Tier 4(100+ per minute)。
- `files:read` の scope の文書は、この scope で file の情報の取得と download ができるとする。`doc/help/slack-app-setup.md` の「画像・添付ファイルの情報取得と download」は、この文書の説明と同じで、private file の download に引き続き要るため据え置いた。

slapex は `conversations.history` / `conversations.replies` だけで message を取得し、Events API / RTM API を使わない。このため、Issue が補完の手がかりに挙げた場面は、slapex の取得経路では起きない(公式文書による。実 workspace では確かめていない)。

### 方針(作業内容の 1 つ目)

A(仕様から外す)。推奨を添えて thread のカードで伺い、ユーザーが A を選んだ(2026-09-28 22:55Z)。

- 理由: 公式文書が、`conversations.history` / `conversations.replies` は完全な file object を返すと明記している。補完の他の候補(削除済み、Free plan の制限で非表示、外部サービス連携、download URL の無いファイル)も、`files.info` を呼んで得られる情報が増えない。B は、文書上起きない場面のために Web API の呼び出しと test を足すことになる。
- decision log 0066 を足し、0025 には追記で参照を置く(Issue の (A) のとおり)。0025 の状態は `decided` のまま(他の決定は有効)。
- 変更する文書: `slack-api-usage.md`(「使用する API」の項を「呼ばない」に書き換え、理由と決定経緯を書く。「参考」に Slack Connect の文書を足す)、`output-format.md`(「保存する assets」の表の取得元から `files.info` を外し、表の下に「`files.info` では補わない」ことと決定経緯を書く)、`usage-flow.md`(「参考」から `files.info` の link を外す。slapex が呼ばない method の reference を残さないため)、decision log 0025 / 0066 / `index.md`。
- 実装、test、出力、scope、利用者向け文書は変えない。
- 使う Web API の method の一覧は変わらない(PF-05 #277 の method ごとの lane の設計に `files.info` は入らない)。

### progress.md

#289 は `progress.md` の索引に無いため、更新しない(`run-issue-task` の手順 7)。

### 出力生成系 3 skill の適用判断

3 skill とも適用しない。変更は設計文書、decision log、この note だけで、CLI の出力、出力 HTML / CSS / assets、sample fixture、README の画像参照は変えていない。

- `update-sample-exports`: 出力 HTML / CSS / assets の見栄え、DOM 構造、保存 path、demo fixture、`tools/gensample` のいずれも変えていない。
- `update-readme-preview-screenshots`: README の出力プレビューに映るものを変えていない。
- `update-readme-demo-gif`: CLI の出力、prompt、option、demo mode を変えていない。

## 次にやること

- PR を draft で作成し、note を採番する。
- CI を確かめてから review を subagent に委譲する(P2)。
- 指摘があれば対応し(P4)、再確認を subagent に委譲する(P5)。
- 人間の手番: Codex のクロスレビュー、PR の merge。

## 検証

2026-09-28、cloud session(project の thread)で実行。

| 項目 | 結果 |
|---|---|
| `git diff --check` | 出力なし |
| 変更が設計文書と decision log だけであること(`git diff --name-only origin/main`。main `b0a42c9`) | `doc/design/slack-api-usage.md`、`doc/design/output-format.md`、`doc/design/usage-flow.md`、`doc/design/decision-log/0025-slack-api-usage-policy.md`、`doc/design/decision-log/0066-no-files-info-fallback.md`、`doc/design/decision-log/index.md`、この note だけ。Go のコード、test、sample、README、help、CLI の出力は変えていない |
| 詳細を省いた file object が `conversations.history` / `conversations.replies` に現れるか(公式文書) | 現れない(「公式文書の確認」) |
| `files.info` の残り(`doc/`、`README.md`、Go のコード) | decision log 0025 / 0066 / `index.md` の記録と、`slack-api-usage.md` / `output-format.md` の「呼ばない」「補わない」の記述だけ |

Go のコードを変えていないため、Docker Compose での `go vet` / `go test` は実行していない(Issue の「検証」の (A) に無い)。CI が PR の head で実行する。実 workspace での確認は、実 token が要るため行っていない。

## リスク・ブロッカー

- 未検証: 実 workspace の `conversations.history` / `conversations.replies` の応答に `check_file_info` の file object が現れないこと(公式文書の記述による)。

## セッションログ

- 2026-09-28: Issue #289、project の memory と前 2 件の報告(issue-245-pr-287、issue-210-pr-288)、関連する設計文書、decision log、実装を読んだ。依存は無い。Slack の文書(Slack Connect、file object、`files.info`、`files:read`)を確かめた。
- 2026-09-28: 方針を thread のカードで伺い(推奨は A)、ユーザーが A を選んだ(22:55Z)。A で設計文書と decision log を直した。Issue の「検証」を実行した(「検証」)。
