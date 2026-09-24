# 0058 cloud session の実行環境

- 状態: decided
- 作成日: 2026-09-24
- 最終更新日: 2026-09-24
- 関連: `doc/guidelines/cloud-session-guidelines.md`, `doc/guidelines/development-command-guidelines.md`, `doc/guidelines/agent-configuration-management.md`, `doc/guidelines/github-mcp-guidelines.md`, `doc/guidelines/github-cli-guidelines.md`, `doc/guidelines/git-operation-guidelines.md`, `doc/guidelines/issue-driven-task-execution.md`, [0002-docker-compose-baseline.md](0002-docker-compose-baseline.md), [0023-project-mcp-config.md](0023-project-mcp-config.md), [0051-worktree-local-config-provisioning.md](0051-worktree-local-config-provisioning.md)

## 背景

Claude Code on the web(Anthropic が host する cloud session)で、Issue 駆動タスクを最後まで進められるようにする(Issue #226)。agent に長時間の自律作業を任せるための実行環境を揃える位置づけである。2026-09-24 の cloud session で実測したところ、次の前提が崩れていた。

- docker CLI / dockerd / compose は入っているが daemon は起動していない。`doc/guidelines/development-command-guidelines.md` の「Docker Desktop などを起動して再試行できるかユーザーに確認する」のままでは、毎回人間の手番が生じる。`service docker start` は sandbox で失敗し、`dockerd` を直接起動すれば 1〜2 秒で使える。
- host の Go は `go.mod` の要求より古いことがある(`GOTOOLCHAIN=auto` が別版を download する)。
- 外向き通信は許可リスト方式で、直接接続は gateway が TLS を再終端する。session の `NO_PROXY` に `proxy.golang.org` が含まれ、container 内の Go は `x509: certificate signed by unknown authority` で module を取得できない。agent proxy 経由なら通る。Docker Hub の blob 配信元は、setup script の文脈(agent proxy が立つ前)から届かない。
- `op` と `gh` が無く、`github-op-integrated` は起動できない。session の組み込み GitHub tool は、`github-op-integrated` の allowlist が外している merge / file push / workflow 実行 / resolve などの tool も見せる。
- 作業ブランチは platform が決め、push 先はそのブランチに固定される。commit 署名と author は platform 側で行われ、tag は push できない。
- 実行環境の準備は、environment ごとの setup script(UI 設定、repo 外、filesystem snapshot として cache)と、repo の `.claude/settings.json` に置く SessionStart hook(毎 session)の 2 段で行える。公式ドキュメントによると、複数 repository の session は repo の hook を読まない。

同じ目的の対応は kiyohara/bizdate の Issue #47 / PR #48 と、その後の PR #62(cloud 用 override の無い service はローカルで使う)、PR #74(`gh` の導入)で先に行われている。方針はこれらの判断(bizdate の decision log 0018)に揃える(2026-09-24 にユーザーの意向を確認)。

## 候補

### 開発コマンドの実行方式

- A: Compose 経由を維持する。SessionStart hook から tool 中立な script を呼び、Docker daemon を起動して `dev` の image を用意する。
- B: cloud session だけ host の `go` を直接実行する例外を作る。
- C: 手順だけ文書化し、agent が毎回 daemon を起動する。

### container の TLS / proxy の扱い

- T1: `dev` を `network_mode: host` にして agent proxy を経由し、`HTTPS_PROXY` / `https_proxy` だけを渡す。`NO_PROXY` は渡さず、CA は mount しない(bizdate と同じ)。
- T2: host の CA bundle を container の `/etc/ssl/certs/ca-certificates.crt` へ read-only で mount する。

### override の有効化と image の取得元

- E1: hook が `COMPOSE_FILE=compose.yaml:compose.cloud.yaml` を `$CLAUDE_ENV_FILE` 経由で session に設定する。image は `compose.cloud.yaml` の `image:` で mirror(`mirror.gcr.io/library/golang`)を指す。
- E2: gitignored な `compose.override.yaml` の symlink を作り、image は Docker Hub から取る。

### 文書の置き方

- W1: 新 guideline を 1 本置いて正本にし、既存 guideline には例外と誘導だけを足す。
- W2: 新 guideline を作らず、既存 guideline それぞれに節を足す。

## 検討内容

- A は開発コマンドの形も報告の扱いも変わらない。spike で daemon の起動、mirror からの pull、空の module cache からの `go mod download` と `gofmt` / `go vet` / `go test` / `go build` がすべて通ることを確認した。B は検証の正を 2 系統にし、0002 の原則に恒久的な例外を作る。C は resume 後に daemon が落ちていることに気づかず host の `go` へ流れやすい。
- T1 と T2 はどちらも slapex で通った。T2 は `NO_PROXY` の中身や CA bundle の path といった platform の内部配置に依存する。T1 は bizdate と同じ構成で、platform の変化に対する前提が少ない。
- E1 と E2: E2 の symlink を採る理由は無く、Docker Hub は setup script の文脈で届かない。slapex の `dev` は `Dockerfile` を持たず `image:` 指定のため、bizdate の `ARG BASE_REGISTRY` の代わりに override の `image:` に mirror の参照を書く。tag を `compose.yaml` と二重に持つことになるため、`--doctor` と hook で一致を検査する。
- W1 と W2: cloud session の内容は開発コマンド、GitHub 操作、git 操作、ブランチ、agent 設定の 5 領域にまたがる。W1 は shim 2 本と index 行が増えるが、環境の使い始めや cache の扱いの置き場ができ、cloud session を使う人が読む入口が 1 つになる。
- setup script と hook の分担は bizdate に揃える。setup script の実行結果は snapshot として cache され、setup script のテキスト変更、許可 host の変更、約 7 日の期限でしか作り直されない。入力(script、`compose.yaml`、`compose.cloud.yaml`)の digest を stub のコメントに埋め、hook が drift を検出して貼り直しを促す。
- GitHub 操作: 組み込み tool は allowlist 外の write を露出する。tool の可視性ではなく guideline の境界で禁止を維持する。`gh` は setup script で入れ、組み込み tool に無い操作だけを `gh api`(REST)で補う。session 内で GraphQL が 403 になること、REST の read が通ること、job log の配信元が拒否されることを実測した。
- `vhs` / `screenshot` は Docker build 中に `deb.debian.org` から `apt-get` するが、既定の許可リストに無い。bizdate PR #62 と同じく、cloud 用 override を置かない service は cloud session で使わず、該当作業はローカルで行う。`update-sample-exports` は `dev` と local fake server だけを使うため cloud session で実行できる。
- 0051 の worktree 用の仕組みは、main worktree と 1Password が前提のため cloud session では使わない。
- `github-op-integrated` の wrapper は変更しない。`.mcp.json` を session 種別で分岐できず、wrapper を黙らせても MCP host 側では接続失敗になる。

## 決定

- cloud session でも開発コマンドは Compose 経由とする(候補 A)。0002 の例外は daemon の起動に限り、cloud session では agent が確認なしに `dockerd` を起動してよい。daemon が起動しない場合も host の `go` で代替せず、未実施として報告する。
- 処理本体は `.agents/scripts/cloud-session-setup.sh`、登録は `.claude/settings.json` の SessionStart hook(`startup|resume`)。script は hook / `--force` / `--provision` / `--doctor` / `--print-stub` の mode を持ち、`CLAUDE_CODE_REMOTE=true` のときだけ hook として動く。hook が動かない session では `--force` を手動で実行する。
- cloud 固有の差分は `dev` だけを対象に `compose.cloud.yaml` に置き(T1、E1)、hook が `COMPOSE_FILE` を session に設定する。image は `mirror.gcr.io/library/golang` から取る。
- environment の setup script は任意とし、登録する場合は `--print-stub` が生成する stub に限る。`--provision` は `gh` の導入、daemon 起動、`dev` の image の pull、state file の記録、daemon の停止を行う。
- 文書は新 guideline `doc/guidelines/cloud-session-guidelines.md` を正本とし(W1)、shim 2 本と `AGENTS.md` を揃える。既存 guideline と `run-issue-task` には例外と誘導だけを足す。
- GitHub 操作は組み込み GitHub tool を第一選択とし、allowlist 外の tool は使わない。`gh` は組み込み tool に無い操作だけを `gh api`(REST)で補う。
- 作業ブランチは session が用意したものを使い、Issue 記載の推奨ブランチ名は PR description に記録する。commit 署名は platform に委ねる。tag の作成・push と `release` skill は cloud session では行わない。
- `vhs` / `screenshot` と、それらを使う `update-readme-preview-screenshots` / `update-readme-demo-gif` はローカルで実行する。

## 理由

Compose を維持すれば、検証の正が 1 つのまま cloud session を扱える。hook で daemon を起動する仕組みは script と登録の 2 ファイルで済み、失敗時も「報告して止まる」だけで既存原則と衝突しない。bizdate と同じ構成にすれば、片方で見つかった platform の変化への対処をもう片方へ移しやすい。

## 影響

- `.agents/scripts/cloud-session-setup.sh`、`.claude/settings.json`、`compose.cloud.yaml` を追加した。local の `docker compose config` は変わらない。
- `doc/guidelines/cloud-session-guidelines.md` を追加し、`.claude/rules/` と `.cursor/rules/` の shim、`AGENTS.md` を揃えた。
- `doc/guidelines/development-command-guidelines.md`、`agent-configuration-management.md`、`github-mcp-guidelines.md`、`github-cli-guidelines.md`、`git-operation-guidelines.md`、`issue-driven-task-execution.md`、`.agents/skills/run-issue-task/SKILL.md`、`.agents/mcp/github-op-integrated/README.md` に cloud session の節・文を足した。`.github/copilot-instructions.md` は cloud session の例外と矛盾しないため変更しない。
- environment の setup script(UI)は repo で管理できない。登録内容は `--print-stub` の出力に固定し、script や compose 定義を変えたら貼り直す。
- `compose.yaml` の `dev` の image tag を変えるときは、`compose.cloud.yaml` も同じ変更で揃える。

## 後から見直す条件

- cloud session で Docker daemon が起動できない、または mirror や Go module proxy に到達できない事例が繰り返される場合。
- Claude Code on the web の hook / setup script / snapshot の仕様が変わった場合。特に複数 repository の session で repo の hook が読まれるようになった場合は、手動の `--force` 手順を見直す。
- 組み込み GitHub tool の構成が変わり、guideline の境界で禁止を維持できなくなった場合。
- `vhs` / `screenshot` を cloud session で動かす必要が出た場合(別 Issue)。
- bizdate 側の 0018 が改訂された場合。
