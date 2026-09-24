# 作業ブランチメモ

- ブランチ: `claude/project-thread-77ip3q`(cloud session が指定。Issue の推奨ブランチ名は `cloud-session-compose-and-github-ops`)
- PR: #232
- 最終更新: 2026-09-24

## 目的

Issue #226。Claude Code on the web の cloud session で、Docker Compose 経由の Go 検証と GitHub 操作を成立させる。bizdate の PR #48 / #62 / #74 の方式(decision log 0018)に揃える。後続の #227(orchestrator skill)を cloud session で実行できる環境を用意する。

## 現在の状況

- 1 Issue・1 PR で進めた。commit は Issue の分割案に合わせて A(実行環境)→ B(GitHub 操作と git の例外)の順に分けた。
- 追加: `.agents/scripts/cloud-session-setup.sh`、`.claude/settings.json`、`compose.cloud.yaml`、`doc/guidelines/cloud-session-guidelines.md`、shim 2 本、decision log 0058。
- 追記: `AGENTS.md`、`development-command-guidelines.md`、`agent-configuration-management.md`、`github-mcp-guidelines.md`、`github-cli-guidelines.md`、`git-operation-guidelines.md`、`issue-driven-task-execution.md`、`run-issue-task/SKILL.md`、`.agents/mcp/github-op-integrated/README.md`。

## 決定事項

- bizdate との差分: slapex の `dev` は `image:` 指定で `Dockerfile` を持たないため、`BASE_REGISTRY` と image の build の代わりに、`compose.cloud.yaml` の `image:` に mirror(`mirror.gcr.io/library/golang:1.26`)を書き、script は pull だけを行う。tag の二重管理は `--doctor` と hook で一致を検査する。
- この session(project の thread、複数 repository)は `/home/claude/<repo>` に clone されていた。
- cloud environment は bizdate と共有せず、slapex 専用に用意する(2026-09-24 にユーザーが指示)。共有向けの仕組みは作らない。
- stub の clone 先は `/home/user/slapex`、`/home/claude/slapex` の順に探す。script は repo root を `CLAUDE_PROJECT_DIR` ではなく自身の位置から解決する(bizdate との差分。hook からの呼び出しでは同じ path になる)。
- `.github/copilot-instructions.md` は変更しない(`:75` は cloud session の例外と矛盾しない)。
- `progress.md` は索引外の単発 Issue のため更新しない。

## 次にやること

- ユーザー: environment の setup script に `--print-stub` の出力を貼り、新しい session で hook の出力と `gh --version` を確認する(未検証事項 1〜3)。
- PR の review と merge はユーザーが行う。

## 検証

2026-09-24、cloud session(project の thread、4 vCPU)で実行。

| 項目 | 結果 |
|---|---|
| `bash -n .agents/scripts/cloud-session-setup.sh` | OK |
| local 相当: `env -u CLAUDE_CODE_REMOTE bash .agents/scripts/cloud-session-setup.sh` | 無出力、exit 0 |
| `docker compose config`(`COMPOSE_FILE` 未設定)の変更前後 diff | 差分なし |
| `--provision`(`gh` 未導入、image 無しから) | exit 0、約 51 秒。`gh` 2.45.0 を `apt-get` で導入、dockerd 起動 2 秒、mirror から pull、state file 記録、dockerd 停止 |
| hook mode(`CLAUDE_CODE_REMOTE=true`、`CLAUDE_ENV_FILE` あり)2 回 | 1 回目 1.4 秒(dockerd 起動 1 秒)、2 回目 0.4 秒。`CLAUDE_ENV_FILE` への `export COMPOSE_FILE=...` は 1 行だけ |
| `--doctor` | exit 0(daemon、image、tag 一致、`gh`、state、cache 一致) |
| `--doctor`(daemon 停止を `DOCKER_HOST` の無効化で再現、review 指摘 1 の修正後) | exit 1。正常時は exit 0 |
| `--doctor`(state の digest を書き換えて drift を再現) | exit 1、貼り直し用の stub を出力。state を戻して exit 0 |
| `docker compose config \| grep -E 'network_mode\|image:'`(cloud) | `mirror.gcr.io/library/golang:1.26`、`network_mode: host` |
| `go version`(container) | go1.26.8 |
| `go mod download`(空の module cache) | OK、約 4 秒 |
| `gofmt -l .` | 出力なし、約 1 秒 |
| `go vet ./...` | OK、約 16 秒(build cache 無し) |
| `go test ./...` | OK、約 5 秒 |
| `go build ./...` | OK、約 1 秒 |
| mirror の digest | `sha256:6c2a5538f964...`。Docker Hub 側は `429 Too Many Requests` で取得できず、一致は未確認 |
| `update-sample-exports` の再生成(`docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample`) | OK。`git diff -- doc/samples` は日時表示(`time` / `date-divider`)と Export information の行だけ。確認後に破棄 |
| `gh api 'repos/kiyohara/slapex/pulls?state=open'`、`gh api user` | OK(REST の read) |
| `gh pr list`(GraphQL) | 403(GraphQL は Claude Code session から使えない旨) |
| `gh auth status` | 「Failed to log in」と誤表示(bizdate と同じ) |
| `gh api .../actions/jobs/<id>/logs` | 配信元 `productionresultssa*.blob.core.windows.net` が Forbidden |
| `git diff --check` | 出力なし |
| `cloud-session-guidelines` の参照 | 正本、shim 2 本、`AGENTS.md` から参照。basename 一致 |

### 未検証事項

1. SessionStart hook の実発火。この session は hook 追加前に始まり、かつ project の thread(複数 repository)で、公式ドキュメント上 repo の hook は読まれない。単一 repository の session での発火は新しい session で確認する。project の thread では `--force` の手動手順になる。
2. setup script の実行時点で repo が clone 済みか、どの path か。stub は複数の候補を探す形にした。実 environment での setup script の完走と所要時間は未確認。
3. setup script を登録した environment の次の session での `gh --version`。
4. local(Docker Desktop)での `docker compose run --rm dev go test ./...` などの共通検証。cloud session のみで実行した。local の `docker compose config` が変わらないことは、`COMPOSE_FILE` 未設定の config 出力の比較で確認した。
5. mirror と Docker Hub の digest 一致(Docker Hub の rate limit)。

## リスク・ブロッカー

- 組み込み GitHub tool の allowlist 外操作は仕組みで塞げず、guideline の禁止に依存する。
- bizdate main の `ab7c88e` 以降に PR #78(bizdate#69 の対応。cloud session での review canonical metadata と `Model` の確認手段)が merge された。#226 のスコープ外だが、#227 を cloud session で実行すると同じ問題に当たる見込みがある。作業内容 0 の再確認結果(HEAD `34b33b1`、PR #78 の分類)は Issue #226 のコメント(https://github.com/kiyohara/slapex/issues/226#issuecomment-5810201422)に追記した。

## セッションログ

- 2026-09-24: bizdate `34b33b1` を確認。Issue #226 の作業内容 0〜11 を実施。spike、script、override、guideline、decision log 0058、note を作成。
- 2026-09-24: review cycle `claude-code-0f65eb9-20260924075407`(Claude Code、subagent)で指摘 3 件。address-comments で全件採用: `--doctor` を daemon 停止・image 無しでも exit 1 にした、既存の Claude Code shim(`github-mcp-guidelines`、`development-command-guidelines`)と Cursor の `development-command-guidelines` に cloud session の例外を 1 行ずつ追加、作業内容 0 の再確認結果を Issue #226 のコメントに追記。
