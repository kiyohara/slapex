# Cloud Session 実行ルール

この文書は、Claude Code on the web の cloud session で slapex を扱うとき、および cloud session 向けの設定を変更するときの共通正本である。AI agent と人間の両方がこのルールに従う。経緯は `doc/design/decision-log/0058-cloud-session-environment.md` を参照する。

cloud session とは、Anthropic が host する使い捨ての VM(sandbox)で Claude Code が動く形態を指す。ローカルの Claude Code、Cursor、Codex には影響しない。cloud session かどうかは `CLAUDE_CODE_REMOTE=true` で判定する。

## なぜ専用の設定が要るか

cloud session の sandbox では、このリポジトリの前提が次のように崩れる。

| 前提 | sandbox の状態 | 対処 |
|---|---|---|
| 開発コマンドは Compose 経由(`doc/guidelines/development-command-guidelines.md`) | docker CLI / dockerd / compose は入っているが daemon は起動していない | SessionStart hook が daemon を起動し、`dev` の image を用意する。agent が daemon を起動してよい(ユーザーへの確認は要らない) |
| Go は `dev` container の toolchain を正とする | host の Go は `go.mod` の要求より古いことがあり、`GOTOOLCHAIN=auto` が別版を download する | host の `go` は使わない。Compose 経由を維持する |
| 外向き通信 | 許可リスト方式で、直接接続(`NO_PROXY` に含まれる `proxy.golang.org` など)は gateway が TLS を再終端する。container 内の CA では検証できない。Docker Hub の blob 配信元は setup script の文脈から届かない | container は sandbox の agent proxy を経由させ、`dev` の image は許可リスト内の mirror から取る(`compose.cloud.yaml`) |
| GitHub 操作は `github-op-integrated` MCP を第一選択(`doc/guidelines/github-mcp-guidelines.md`) | `op` が無く、MCP server は起動できない。`gh` は image に無い(setup script が入れる)。組み込みの GitHub tool は allowlist 外の write も見せる | 組み込みの GitHub tool を第一選択にし、allowlist 外の tool は使わない。`gh` は組み込み tool に無い操作だけ `gh api`(REST)で補う(同 guideline の「cloud session」) |
| Issue 記載のブランチ名で作業する(`doc/guidelines/issue-driven-task-execution.md`) | 作業ブランチは session 作成時に platform が決め、push はそのブランチにだけ許可される | session のブランチをそのまま使う(同 guideline と `doc/guidelines/git-operation-guidelines.md` の「cloud session」) |
| commit / tag 署名は 1Password(`doc/guidelines/git-operation-guidelines.md`) | 署名と author は platform 側で行われる。tag の push はできない | 1Password 連携の節は適用しない。tag の作成・push と `release` skill は cloud session では行わない |
| worktree の local config(`.worktreeinclude`、`.agents/scripts/worktree-setup.sh`) | main worktree も 1Password も無い | cloud session では使わない |

## 設定ファイル

| ファイル | commit | 役割 |
|---|---|---|
| `.agents/scripts/cloud-session-setup.sh` | yes | 処理本体。hook(既定)/ `--force` / `--provision` / `--doctor` / `--print-stub` の mode を持つ。動作の詳細は script 冒頭のコメントと `--help` を正とする |
| `.claude/settings.json` | yes | SessionStart hook(`startup` と `resume`)で上記 script を呼ぶ登録だけを持つ。処理や恒久ルールを書かない。個人の設定は gitignored な `.claude/settings.local.json` に置く |
| `compose.cloud.yaml` | yes | cloud session 専用の override。対象は `dev` だけで、container を host network にして agent proxy を通し、image を mirror(`mirror.gcr.io/library/golang`)から取る。tag は `compose.yaml` と揃え、`--doctor` が一致を検査する |
| environment の setup script | no(claude.ai/code の UI 設定) | `--print-stub` が生成する数行の stub。上記 script を `--provision` で呼ぶだけにし、処理を UI 側に書かない。script が無い repository や branch では何もせず exit 0 する(environment は repository と branch をまたいで共有される) |
| state file(VM 内 `/opt/slapex-cloud/state`) | no | `--provision` が snapshot の出自(入力の digest、作成時刻、image の名前)を記録する。hook が drift 検出に使う |

## 実行順序

1. platform がリポジトリを clone する。
2. environment cache が無ければ setup script(stub → `--provision`)が実行され、完了後に filesystem が snapshot される。cache があればこの手順は飛ぶ。setup script は agent proxy が立つ前に走り、container の中から外へは出られない。そのため `--provision` は `gh` の導入(Ubuntu archive)、daemon の起動、`dev` の image の pull(mirror)、state file の記録だけを行う。`gh` の導入や pull に失敗しても exit 0 で続ける。
3. Claude Code が起動し、SessionStart hook が script を hook mode で実行する。`COMPOSE_FILE` を session に設定し、daemon を起動し、image が無ければ pull し、`gh` の有無を表示し、cache の drift を確認する。hook は `gh` を導入しない。hook の stdout は agent の context に入る。
4. 以後の作業は通常どおり。

hook は `resume` でも実行される。VM が作り直された後の再開でも daemon が起動する。

### hook が動かない session

公式ドキュメントによると、複数 repository の session(project の thread を含む)は repo の `.claude/settings.json` の hook を読まない。hook の出力(`cloud-session-setup: ...`)が context に無い session では、開発コマンドの前に手動で実行する。

```sh
bash .agents/scripts/cloud-session-setup.sh --force
export COMPOSE_FILE=compose.yaml:compose.cloud.yaml
```

`--force` は `CLAUDE_CODE_REMOTE` によらず hook と同じ動作をする。`COMPOSE_FILE` は後続の shell へ引き継がれないため、Bash の呼び出しごとに `export` するか、`docker compose -f compose.yaml -f compose.cloud.yaml ...` と書く。

## 開発コマンド

コマンドの形は変えない。`doc/guidelines/development-command-guidelines.md` の基本形と検証コマンドをそのまま使う。

```sh
docker compose run --rm dev go test ./...
```

- hook が `COMPOSE_FILE=compose.yaml:compose.cloud.yaml` を session に設定する。効いていない shell では上記「hook が動かない session」に従う。
- `TZ` は sandbox で未設定のため container は UTC で動く。時刻が出力に影響するコマンドでは `-e TZ=Asia/Tokyo` を明示する。
- host の `go`(`/usr/local/go` など)で得た結果を検証結果として報告しない。
- checkout は shallow のことがある。古い履歴を `git log -S` などで辿れないときは、`git fetch --unshallow` の要否を判断してから実行する。

### 使える Compose service と skill

`compose.cloud.yaml` に override を置いた `dev` だけを使う。`vhs` と `screenshot` は cloud 用の override を持たず、Docker build 中の `apt-get`(`deb.debian.org`)が既定の許可リストに無いため、cloud session では使わない。

| skill | cloud session | 理由 |
|---|---|---|
| `update-sample-exports` | 実行してよい | `dev` と local fake server だけを使う |
| `update-readme-preview-screenshots` | ローカルで実行する | `screenshot` service を使う |
| `update-readme-demo-gif` | ローカルで実行する | `vhs` service を使う |
| `release` | 行わない | 署名付き tag の作成・push ができない |

cloud session で screenshot / GIF の再生成が必要な変更をした場合は、再生成を行わず「ローカルで要再生成」として未検証事項を PR description と working branch note に残す。

## 検証結果の報告

cloud session の `dev` は local と同じ `golang` 公式 image(同じ tag)を mirror から取ったものである。検証結果は Compose 経由の結果として扱い、報告や note には「cloud session で実行」と添える。

## daemon が起動しないとき

hook の出力に `WARNING` が含まれる場合は、次の順で扱う。

1. `bash .agents/scripts/cloud-session-setup.sh --force` で再試行する。
2. `bash .agents/scripts/cloud-session-setup.sh --doctor` で daemon / image / cache の状態を見る。
3. それでも起動しなければ、host の `go` で代替せず、実行できなかった検証を未実施として理由(hook の出力)とともに note と PR に書き、ユーザーに報告する。

## environment cache と drift

setup script の実行結果は filesystem snapshot として cache され、後続の session はそこから始まる。cache が作り直されるのは次の 3 つの場合だけである。

- setup script のテキストが変わった。
- environment の許可 host が変わった。
- 約 7 日の期限が来た。

`.agents/scripts/cloud-session-setup.sh`、`compose.yaml`、`compose.cloud.yaml` を変えても、stub のテキストが変わらなければ cache は古いまま残る。このため stub には入力の digest をコメントとして埋め、貼り直しでテキストが変わるようにしている。

- hook は毎回、cache に記録された digest と repo の digest を比べる。一致しなければ「environment cache が repo に追いついていない」と警告し、貼り直し用の stub を出力する。state file が無い(setup script 未登録)場合は登録を促す 1 行だけを出す。
- 対処は人手で行う。`--print-stub` の出力を environment の setup script に貼り直す。platform は setup script を API で更新する手段を提供しないため、自動化しない。
- 警告があっても作業は続けられる。image が無い session では hook が pull する。
- snapshot は branch をまたいで共有される。feature branch で `compose.yaml` を変えても、stub を貼り直すまで cache は前の状態のままである。

### 他の repository と environment を共有するとき

setup script は environment に 1 つしか置けない。同じ environment を他の repository(例: 同じ方式の stub を持つ別プロジェクト)と共有する場合は、各 stub を subshell `( ... )` で囲んで順に並べる。stub の `exec` と `exit` は subshell の中で閉じるため、先の stub が後の stub を止めない。各 stub の state file は別の path に置かれ、互いの drift 検出に影響しない。

setup script の文脈の `CLAUDE_PROJECT_DIR` は、environment を共有する別の repository を指すことがある。slapex の stub は候補の path の script が slapex のもの(state dir の名前)であることを確かめてから実行し、script は repo root を自身の位置から解決する。

## Network access

environment の Network access は既定の Trusted のままでよい。使う host は次のとおりで、いずれも既定の許可リストに含まれる。

| 用途 | host |
|---|---|
| `dev` の image の取得(mirror) | `mirror.gcr.io`(`*.gcr.io`)とその配信元(`*.googleapis.com`) |
| Go module の取得 | `proxy.golang.org`、`sum.golang.org` |
| `gh` の導入(`--provision`) | `archive.ubuntu.com`(Ubuntu archive。`gh` 2.45.0) |

許可されていない host への接続は、直接接続なら `403`、proxy 経由なら接続失敗として現れる。`curl -sS "$HTTPS_PROXY/__agentproxy/status"` で proxy 側の拒否理由を確認できる。拒否された host へ迂回しない。必要なら環境設定の変更としてユーザーに報告する。

## 所要時間

sandbox での実測(2026-09-24、4 vCPU、project の thread の session)。

| 処理 | 所要時間 |
|---|---|
| daemon の起動 | 1〜2 秒 |
| `--provision` 全体(`gh` の導入と image の pull を含む) | 約 51 秒 |
| hook(image あり) | 約 1 秒 |
| `go mod download`(空の module cache) | 約 4 秒 |
| `go vet ./...`(初回、build cache 無し) | 約 16 秒 |
| `go test ./...`(vet の後) | 約 5 秒 |
| `gofmt -l .` / `go build ./...` | 各 1 秒程度 |

setup script は 5 分以内に終わる必要がある。`--provision` は通常 1 分程度に収まる。`gh` の導入は `apt-get` の 3 段を各 45 秒で打ち切るため、最悪でも 135 秒に image の pull を足して 3 分程度である。setup script では container の network が通らないため、`go mod download` などの事前実行は含めない。

## 利用開始手順

1. environment は既定の設定(Trusted)で使える。setup script を登録しなくても hook(または手動の `--force`)が初回に image を pull する。ただし `gh` は入らない。
2. `gh` を入れ、session 開始を速くするには、cloud session の中で `bash .agents/scripts/cloud-session-setup.sh --print-stub` を実行し、出力を environment の setup script に貼る(他の repository と共有する場合は上記「他の repository と environment を共有するとき」)。
3. 新しい session を開き、冒頭に hook の出力(daemon、image、`gh`、`COMPOSE_FILE`)が入ることと、`docker compose run --rm dev gofmt -l .` と `gh --version` が通ることを確認する。hook が動かない session では `--force` と `--doctor` で確認する。

## 変更するとき

- 処理を変えるときは `.agents/scripts/cloud-session-setup.sh` を直す。`.claude/settings.json` には呼び出しの登録以外を書かない。UI の stub に処理を書かない。
- script、`compose.yaml`、`compose.cloud.yaml` を変えたら、`--print-stub` を再生成して environment に貼り直す。hook の drift 警告がその合図になる。
- `compose.yaml` の `dev` の image tag を変えるときは、`compose.cloud.yaml` の tag も同じ変更で揃える。
- cloud 向けの処理を足すときは、この script に足すか `.agents/scripts/` に別の script を置く。配置規約は `doc/guidelines/agent-configuration-management.md` の「cloud session の実行環境 script」を参照する。
- 変更後の検証は script を 2 回実行して冪等であること、`CLAUDE_CODE_REMOTE` 未設定で無出力 exit 0 になること、`--provision` が exit 0 で完走すること、local の `docker compose config` が変わらないことを確認する。

## 落とし穴

- `service docker start` は sandbox では失敗する。init script が `ulimit` の変更を要求し、sandbox がそれを許さないためである。script は `dockerd` を直接起動する。
- `/run` は root filesystem の一部であり、daemon 起動中に snapshot されると pid file と socket が残る。script は dockerd / containerd でない pid の file を消し、socket は dockerd process が無いときだけ消す。`--provision` は自分で起動した daemon を最後に止める。
- container に `NO_PROXY` を渡さない。session の `NO_PROXY` には `proxy.golang.org` が含まれ、渡すと直接接続になって container 内の CA で検証できず失敗する(`x509: certificate signed by unknown authority`)。
- hook は local の Claude Code でも起動する。`CLAUDE_CODE_REMOTE` が `true` でなければ無出力で終わるため、local の開発には影響しない。
- `.mcp.json` の `github-op-integrated` は cloud session で常に起動に失敗する(`op` が無い)。想定どおりであり、診断や再接続の依頼をしない。
- `gh auth status` は内部の GraphQL が proxy に拒否され、「token が無効」と誤表示する。認証の問題ではない。`gh` の使い方は `doc/guidelines/github-mcp-guidelines.md` の「cloud session」に従う。
- `GH_TOKEN` / `GITHUB_TOKEN` の placeholder を実際の token や PAT で上書きしない(environment の環境変数にも置かない)。credential は proxy が差し替える。
- 公式ドキュメントは `gh` を preinstall としているが、この environment の image には無かった(2026-09-24)。platform の変更で挙動が変わり得るため、`gh` の導入経路や proxy の規則に関わる変更をするときは実測し直す。
- environment の setup script は agent proxy が立つ前に走る。container の中から外へ出る処理(`go mod download` など)は setup script では通らないため、`--provision` に足さない。
- environment の setup script は repository と branch をまたいで共有される。stub は script が無ければ skip して exit 0 するため、他の repository や script を含まない branch で session を開いても起動を妨げない。stub を `exec` だけの形に書き換えない。

## 関連ルール

- 開発コマンド: `doc/guidelines/development-command-guidelines.md` の「Docker の確認」
- GitHub 操作: `doc/guidelines/github-mcp-guidelines.md` と `doc/guidelines/github-cli-guidelines.md` の「cloud session(Claude Code on the web)」
- git 操作、ブランチ、commit 署名: `doc/guidelines/git-operation-guidelines.md` の「cloud session(Claude Code on the web)」
- Issue 駆動タスクのブランチ: `doc/guidelines/issue-driven-task-execution.md` の進め方 3
- 配置規約: `doc/guidelines/agent-configuration-management.md` の「cloud session の実行環境 script」
