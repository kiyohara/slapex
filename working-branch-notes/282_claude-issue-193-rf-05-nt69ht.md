# 作業ブランチメモ

- ブランチ: `claude/issue-193-rf-05-nt69ht`(cloud session が指定。Issue の推奨ブランチ名は `refactor-cli-option-mapping`)
- PR: #282
- 最終更新: 2026-09-27

## 目的

Issue #193(RF-05)。`cmd/slapex/main.go` は、option の parse と検証、Slack token と controlling terminal、`--demo` の起動、exit code への対応づけを 1 つのファイルに持っていた。通常実行と `--demo` は、CLI option をそれぞれ別の literal で `export.Options` と `demo.Options` へ転記し、`demo.Export` も `demo.Options` を `export.Options` へ転記していた。共通の option を 1 つ足すたびに、複数か所を揃える必要がある。責務ごとにファイルを分け、通常実行と `--demo` に共通の option の変換を 1 か所に集め、`parseCLIArgs` を読める単位に分ける。

維持する挙動(Issue の完了条件): 明示した空文字と未指定の区別、`--version` / `--help` の優先順位、channel の前後の option、`/dev/tty`、stdout の path、stderr の診断、exit code、`--demo` が実認証情報を要しないこと。

本 Issue は `drive-issue-to-reviewed-pr` skill で、review と再確認を済ませた PR まで進める。ユーザーの指示(2026-09-25)で open Issue を 1 件ずつ直列に処理する流れの 17 件目で、Issue ごとに新しい thread で進める方式(2026-09-26)の 12 件目である。PF-01(#273)の次に進める Issue として、ユーザーが選んだ(2026-09-27)。

## 現在の状況

- 依存(#188)は merge 済み(RF-00 / PR #197)。main `375e8f3`(PR #281 の merge)から作業した。
- ファイルの分割(`0e6ae69`)、characterization test(`60f80eb`)、option の変換の集約(`9ef307f`)、`parseCLIArgs` の分割(`a8a2e73`)、設計文書の同期(`db851b4`)を実装し、Issue の「検証」を実行した(「検証」)。
- PR #282 作成済み(draft)。採番と `progress.md` の PR 欄の反映を済ませた。

## 決定事項

### 着手時の現行コード

Issue の背景は main `d5aa977` 時点の記述である(`main.go` 537 行、`parseCLIArgs` 164 行)。main `375e8f3` では PF-01(#273)の HTTP trace の配線が加わって `main.go` は 547 行だったが、構成は同じだった。

- `parseCLIArgs`: flag を local 変数に定義し、channel の前後で 2 段階に parse し、`fs.Visit` で明示された flag を調べて検証し、最後に `cliOptions` の literal へ転記する。診断は、問題を見つけた箇所ごとに `slapex: ...` を出していた。
- `run`: `cliOptions` から `export.Options` の literal を組み立てる。`--demo` なら `runDemo` に `cliOptions` を渡す。
- `runDemo`: `cliOptions` から `demo.Options` の literal を組み立て、`demo.Export` を呼ぶ。
- `demo.Export`: `demo.Options` から `export.Options` の literal を組み立てる。channel、fake client、`NoInteractive`、pacing の上書きもここにある。gensample と demo の test も `demo.Export` を使う。

### ファイルの分割(`0e6ae69`)

| ファイル | 置いたもの |
|---|---|
| `main.go` | `main`、`run`、`apiBaseURLFromEnv`、`newSlackClient`(起動と通常実行の組立) |
| `options.go` | `cliOptions`、`errUsage`、`parseCLIArgs`、`parseSize` |
| `token.go` | `SLACK_TOKEN`、token の prompt、controlling terminal(`/dev/tty`)、token が無いときの報告 |
| `demo.go` | `runDemo`、locale による scenario の選択 |
| `exitcode.go` | exit code、認証系の Slack error code、失敗の報告(`reportRunError`、`classify`) |

- `httptrace.go` は PF-01 のまま。unit test も対応する `*_test.go` に移した。
- 宣言は本文を変えずに移した。`main.go` の const block(`helpURL`、`slackTokenEnv`、`apiBaseURLEnv`)だけは、使う側のファイルの単独の const に分けた。
- 確かめ方: go/parser で main の `main.go` と `main_test.go` の top-level の宣言 54 個(doc comment を含む)を取り出し、分割後のファイルに一字一句同じ宣言があることを確かめた。一致しなかったのは const block から出した 2 つ(`helpURL` と `apiBaseURLEnv`)だけで、差は gofmt の桁揃えとコメントのインデントである。

### option の変換の集約(`9ef307f`)

- `cliOptions.exportOptions()`: CLI option を `export.Options` へ変換する唯一の場所。通常実行はこれに `PromptTTY`(controlling terminal)だけを足す。
- `demo.Run(ctx, sc, opts export.Options, printer)`: fixture が決める値だけを差し替えて export を実行する。channel keyword を fixture の channel にし、`NoInteractive` を立て、`PromptTTY` を外す。fake token、base URL、pacing を省く client もここで作る。`--demo` は `exportOptions()` の結果をそのまま渡す。
- `demo.Export(ctx, sc, o demo.Options, printer)`: 引数の形は変えず、`demo.Options` を `export.Options` へ変換して `demo.Run` に委ねる。gensample と demo の test はこれまでどおり `demo.Export` を使う。

#### 「demo 固有の上書きは demo.Export に保持する」の解釈

Issue は、demo 固有の channel、fake client、`NoInteractive`、pacing の上書きを `demo.Export` に保持するとしている。一方で `demo.Export` の引数は `demo.Options` であり、その field の形は保つ(Issue の条件)。`--demo` が `demo.Export` を呼び続ける限り、`cliOptions` から `demo.Options` への転記が残り、集約にならない。

そこで、上書きを internal/demo の新しい `demo.Run`(`export.Options` を受け取る)に置き、`demo.Export` は `demo.Run` に委ねる形にした。上書きは internal/demo の 1 か所にあり、gensample(`demo.Export` 経由)と `--demo`(`demo.Run` を直接)が共有する。上書きを cmd/slapex へ持ち込んでいないため、demo 固有の上書きを demo package に閉じるという Issue の意図に沿うと判断した。review で確かめてほしい点に挙げる。

選ばなかった案:

- `runDemo` に `cliOptions` から `demo.Options` への変換関数を置く案: 変換関数が 2 つ(`export.Options` 用と `demo.Options` 用)になり、共通の option を足すたびに両方を揃える必要が残る。
- `demo.Export` の引数を `export.Options` に変える案: gensample と demo の test の literal を一括で移すことになる。Issue のスコープ外である(「既存test/gensampleのliteral一括移行は対象外」)。
- `demo.Options` に `export.Options` を埋め込む案: Issue のスコープ外である(「fieldの埋め込み型化」)。

### parseCLIArgs の分割(`a8a2e73`)

標準の flag package と 2 段階の parse は変えていない。

| 単位 | 行数 | 責務 |
|---|---|---|
| `parseCLIArgs` | 30 | 2 段階の parse、`--version`、余分な引数、検証の呼び出し、診断の出力 |
| `newFlagSet` | 28 | flag の定義と usage。flag は `cliOptions` の field に直接結びつけ、変換が要る 3 つ(添付の上限、emoji の list 2 つ)だけ `rawFlags` に文字列で受ける |
| `parseFlags` | 7 | flag の誤りを usage error にする(`--help` は `flag.ErrHelp`) |
| `givenFlags` | 5 | 明示された flag の集合(未指定と、明示した空文字や既定値を区別する) |
| `validate` | 19 | 検証の順序と、変換した値の格納 |
| `validateFetchRange` | 37 | 取得範囲(`--from` / `--to`、`--date`、`--days`)の検証 |
| `parseMaxAttachmentSize` | 7 | 添付の上限の変換と診断 |
| `parseEmojiFilter` | 10 | emoji の list の変換と診断(未指定なら filter なし) |

- 検証は最初に見つけた問題を error で返し、`parseCLIArgs` が `slapex: ` を付けて 1 か所で出す。flag の誤り(flag package が usage と一緒に出す)と、余分な引数(usage を添える)の診断は従来どおり。
- 検査の順序は変えていない。max-posts、取得範囲、添付の上限、body の emoji、reaction の emoji の順で、取得範囲の中は `--from` / `--to` なら組み合わせ、`--date` との併用、`--days` との併用、`--from` の書式、`--to` の書式、前後の順、`--date` なら書式、`--days` との併用の順である。`--date` の分岐だけ書式の検査が併用の検査より先なのは従来の挙動で、そのまま残した。
- 日付の CLI 側の早期診断は、export 側の検証と一体化していない(Issue の評価のとおり)。

### 転記箇所の数(Issue の完了条件の報告)

対象は、通常実行と `--demo` が共通に受け取る 12 field(`OutputDir`、`MaxPosts`、`Days`、`Date`、`From`、`To`、`ExcludeBodyEmoji`、`ExcludeReactionEmoji`、`MaxAttachBytes`、`KeepCache`、`ReuseCache`、`ToolVersion`)である。`ToolVersion` は flag ではなく `version` から入る。Issue の指示どおり、literal を helper へ移しただけのものは削減に数えず、定義と転記の総数を比べた。

| 箇所 | 種別 | main `375e8f3` | head |
|---|---|---|---|
| `cliOptions` の field | 定義 | 11 | 11 |
| `rawFlags` の field | 定義 | - | 3 |
| flag の定義 | 定義 | 11 | 11 |
| 値の変換(添付の上限、emoji の list 2 つ) | 変換 | 3 | 3 |
| `parseCLIArgs` の `cliOptions` の literal | 転記 | 11 | - |
| `export.Options` の field | 定義 | 12 | 12 |
| `run` の `export.Options` の literal | 転記 | 12 | - |
| `cliOptions.exportOptions` | 転記 | - | 12 |
| `demo.Options` の field | 定義 | 12 | 12 |
| `runDemo` の `demo.Options` の literal | 転記 | 12 | - |
| `demo.Export` の `export.Options` の literal | 転記 | 12 | 12 |
| 計 | | 96 | 76 |

- 共通の option(値の変換が要らない flag)を 1 つ足すときに触る箇所: main では 8 か所(`cliOptions` の field、flag の定義、`parseCLIArgs` の literal、`export.Options` の field、`run` の literal、`demo.Options` の field、`runDemo` の literal、`demo.Export` の literal)。head では 4 か所(`cliOptions` の field、flag の定義、`export.Options` の field、`exportOptions`)。`--demo` には `exportOptions` の結果がそのまま届くため、`demo.Options` と `demo.Export`(+2)は、gensample か test がその option を指定する場合だけ足す。
- 足し忘れは `TestExportOptions` が検出する。`export.Options` の field ごとに、どれかの command line で設定されるか、呼び出し側に任せる field(`PromptTTY`、`Now`)として列挙されているかを問う。
- 行数: `main.go` は 547 → 114 行。`parseCLIArgs` は 164 → 30 行(分けた単位は上の表)。`run` は 80 → 64 行、`runDemo` は 25 → 12 行。

### test

- characterization test(`60f80eb`): 集約と分割の前に、現行の挙動を固定する test を足した。
  - `TestParseArgsDiagnostics`: 不正な指定 29 種の診断の全文(flag の誤りと usage を含む)、明示した空文字、複数の問題があるときに報告する最初の問題。
  - `TestParseArgsDefaults`: 未指定の既定値と、空の `--output` / `--reuse-cache` が未指定と同じになること。
  - `TestParseArgsVersionAndHelp`、`TestRunHelp`: channel の前後での `--version` と `--help` の優先順位、exit code、stdout と stderr。
  - `TestRunDemoNeedsNoSlackToken`: `--demo` は `SLACK_TOKEN` なしで動き、設定されていても Slack API の接続先(`SLAPEX_API_BASE_URL`)に request を送らず、token を表示せず、HTTP trace も書かない。channel 引数は無視する。
- 集約の test(`9ef307f`):
  - `TestExportOptions`: CLI option が `export.Options` の各 field に届くこと(上記の足し忘れの検出を含む)。
  - `TestRunNormalAndDemoShareOptions`: 同じ option で、通常実行(demo の fixture server に架空の token で接続)と `--demo` を `run()` から実行し、`.cache/metadata.json` に記録された取得範囲、filter、上限、tool version が一致すること。どちらも `--reuse-cache` の path を報告し、`--output` の下に書くこと。取得範囲は `--days` と上限、`--date`、`--from` / `--to` と filter の 3 通り。
  - `TestRunUsesTheFixtureChannel`: `demo.Run` が、渡された channel keyword に関わらず fixture の channel を export すること。
- 変異: 集約に 7 種、parse の分割に 12 種の誤りを 1 つずつ入れ、すべて test が検出した(commit はしていない)。
  - 集約: `exportOptions` が reaction の emoji を落とす、`demo.Run` が `MaxPosts` を上書きする、`ReuseCache` を落とす、body の emoji を落とす、`OutputDir` を落とす、channel を差し替えない、通常実行が `Days` を上書きする。
  - parse: `--date` の併用の検査を書式より先にする、`--from` / `--to` の書式を組み合わせより先に検査する、emoji を添付の上限より先に検査する、未指定の emoji の list も parse する、空の emoji の list を未指定として扱う、`--date` や `--from` / `--to` で `--days` の既定値を残す(2 種)、診断の `slapex: ` を落とす、余分な引数の検査を `--version` より先にする、`--version` で他の option も返す、2 段階の parse をやめる、余分な引数で usage を出さない。

### 出力生成系 skill

3 skill とも呼ばなかった。

- `update-sample-exports`: 出力 HTML / CSS / assets、DOM、asset の保存 path、demo の fixture を変えていない。gensample は従来どおり `demo.Export` を使い、`demo.Export` が `demo.Run` に委ねる形になっただけで、gensample が指定する option は同じである。固定 sample の再生成は、commit 済みの `doc/samples/` と main の生成物の両方に一致した(「検証」)。
- `update-readme-preview-screenshots`: sample が変わらないため当たらない。
- `update-readme-demo-gif`: Issue は CLI が対象のため、この skill の適用を求めている。skill の「いつ使うか」は、ターミナルデモの操作フロー、表示内容、録画結果が変わる場合に使うとしている。`cmd/slapex/**` と `internal/demo/**` を変えたため、同節の「影響するか判断できない場合」に従い、`tools/demo/demo-ja.tape` が実行する経路を変更前後の binary で再現して比べた。すなわち、`gensample -serve` の fake server に対し、`SLACK_TOKEN` なしで styled の `slapex` を実行し、token の prompt に架空の token を入力し、channel の picker で 2 つ目を選ぶ経路である(「検証」の E2E)。token の prompt、picker、進捗表示、完了表示が一致したため、録画結果は変わらないと判断し、再録画しない。

### 既存の挙動で気づいた点(本 PR では変えない)

- `--demo` で、前の demo の出力を `--reuse-cache` に渡すと、custom emoji の URL は cache の値(前の demo の in-process server。port は実行ごとに変わる)が使われる。前の実行で保存していない custom emoji は、閉じた port への取得を 5 回 retry した後に取得失敗になる。例: `--demo --keep-cache --max-posts 3` の出力(custom emoji を使う投稿を含まない)を、次の `--demo --reuse-cache` に渡すと、custom emoji 2 件が失敗した(`assets: 13 saved, 0 skipped by size limit, 2 failed`)。main `375e8f3` の binary でも同じ結果だった。demo だけで起きる既存の挙動で(実 workspace の emoji の URL は実行ごとに変わらない)、本 Issue の範囲(option の変換の集約)の外のため変えていない。follow-up の候補として終了時に報告する。

### decision log

作らない。decision log 0056 の計画(RF-05)の範囲の整理で、方針は変えていない。

## 次にやること

- draft PR を作成し、note を採番する。(完了)
- `progress.md` の RF-05 の PR 欄に PR 番号を記入し、採番の報告から引き上げた項目を「セッションログ」の P1 に残して push する。(完了)
- CI を確かめてから review を subagent に委譲する(P2)。

## 検証

2026-09-27、cloud session(project の thread)で、Docker Compose(`docker compose run --rm dev ...`)で実行した。実 token は使わず、架空の token と、demo の fake Slack server(in-process と `gensample -serve`)だけを使った。

- `gofmt -l .`: 出力なし。`go vet ./...`、`go build ./...`: 成功。
- `go test ./cmd/slapex ./internal/demo ./internal/export`、`go test ./...`: ok。
- `go test -count=5 -shuffle=on`、`go test -race -count=2`(`CGO_ENABLED=1`): 上の 3 package で ok。
- cross compile(darwin / linux × amd64 / arm64 の `go build ./cmd/slapex`): ok。
- `git diff --check 375e8f3 HEAD`: 問題なし。
- ファイルの分割: 宣言が一字一句そのまま移ったことを go/parser で確かめた(「ファイルの分割」)。
- characterization test は、集約と分割の前後で test を変えずに ok。変異 19 種をすべて検出した(「test」)。
- 固定 sample: Issue の手順どおり、`TZ=Asia/Tokyo` で `go run ./tools/gensample -time 2026-07-04T16:32:41+09:00 -out /tmp/refactor-samples` を実行し、`diff -r doc/samples/ja /tmp/refactor-samples/ja` と en 側が一致した(各 18 ファイル、計 36 ファイル)。commit 済みの footer は `2026-07-04 16:32 (UTC+09:00) / 2026-07-04T07:32:41Z` で、`-time` と一致する。main `375e8f3` で同じ条件で生成した結果とも一致した。生成物は commit していない。cache は固定 sample に含まれないため、Issue のとおり #194 の JSON 比較(`TestWriteCachesPayloads`。`go test ./...` で ok)で補い、加えて E2E で `.cache/` を含む出力を変更前後で比べた。
- E2E(変更前後の binary の比較): main `375e8f3` と head で build した `slapex` を、同じ入力で実行して比べた。harness(`bin/rf05/e2e.py`。`bin/` は git の管理外)は commit していない。61 通りすべてが一致した。
  - CLI の表面 47 通り: `--help`、`-h`、`--version`(channel の前後、余分な引数や不正な値との組み合わせ)10 通り、usage error 32 通り(`TestParseArgsDiagnostics` の全ケースを含む)、option は正しく token が無い 5 通り(channel の有無、`--no-interactive`、空の `--output` / `--reuse-cache`)。exit code、stdout、stderr が byte 単位で一致した。
  - `--demo` 8 通り: 既定、`--days` と `--max-posts` と `--max-attachment-size` と `--keep-cache`、`--date`、`--from` / `--to` と emoji の filter 2 つ、前の demo の出力を指す `--reuse-cache`、存在しない `--reuse-cache`、channel 引数と `--no-interactive`、日本語の locale。exit code、stdout、stderr、出力ファイル一式(`.cache/` を含む)が一致した。
  - 通常実行 4 通り(`gensample -serve` の fixture server に架空の token で接続): `--keep-cache`、`--date` と reaction の filter と `--max-attachment-size`、`--from` / `--to` と body の filter と `--max-posts`、部分一致の channel と `--reuse-cache`。同じく一致した。
  - 対話 2 通り: (a) `/dev/tty` だけを pty にし、stdout と stderr を pipe にした実行(`op run` の状況)。`SLACK_TOKEN` が無いと token の prompt が `/dev/tty` に出て、入力した架空の token で接続し、channel の picker で 1 つ目を選んで export した。exit code、stdout、stderr、出力ファイルが一致し、token は表示されなかった。(b) demo の録画(`tools/demo/demo-ja.tape`)と同じく、すべての stream を terminal にした styled の実行で、picker では 2 つ目を選んだ。exit code と出力ファイルに加え、terminal に出た内容(token の prompt の各行、picker の title と選択肢、選んだ channel、Done の行、messages / assets / output の行)が一致した。picker は変わった行だけを描き直し、その frame が行に分かれる位置は読み取りの timing で変わるため、frame そのものではなく表示された内容を比べた。(a) と (b) は 3 回繰り返し、毎回一致した。
  - 比較の前に、実行ごとに変わる値を正規化した: 時刻、所要時間、出力先の path(既定の出力先の名前 `slapex-<yyyymmdd>-<hhmm>` を含む)、in-process の fixture server の port、avatar の保存順。avatar の保存順(`assets_manifest.json` の並び)は実行ごとに変わる既知の挙動で、PF-01 の申し送りとして #274 に引き継ぎ済みのため、並びを無視して比べた。

## リスク・ブロッカー

- なし。

## セッションログ

- 2026-09-27: #193 に着手。ファイルの分割(`0e6ae69`)、characterization test(`60f80eb`)、option の変換の集約(`9ef307f`)、`parseCLIArgs` の分割(`a8a2e73`)、設計文書の同期(`db851b4`)、検証。
- 2026-09-27 P1: draft PR #282 を作成し、note を採番した(`94e864f`)。`progress.md` の RF-05 の PR 欄を反映した。検証は「検証」のとおりすべて ok。出力生成系 3 skill は呼ばなかった(「出力生成系 skill」)。`number-working-branch-note` の報告から引き上げた項目: 書き換えた行は、note の状況の stale 表現 1 行(`PR 未作成。` → `PR #282 作成済み。`)、note の完了タスク行 1 行(「draft PR を作成し、note を採番する。」に `(完了)`)、PR description の note のファイル名参照 1 行(`draft_` → `282_`)。title は書き換えていない。触らずに残した行は note、PR description、title とも無し。ほかに note の `PR:` 欄に `#282` を記入した。情報統制チェックで直した箇所は無い。PR の assignee に kiyohara を設定した。review の依頼は、PR の作成者と同じ account のため GitHub に受け付けられなかった。
