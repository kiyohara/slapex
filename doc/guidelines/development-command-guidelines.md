# 開発コマンド実行ルール

この文書は、slapex リポジトリで AI agent が開発用コマンドを実行するときの共通正本である。

この方針は AI agent の挙動を規律するものであり、`README.md` の人間向け手順は対象外とする。AI agent は `README.md` に host OS 上で開発環境を直接操作する例があっても、自身が直接実行する根拠にはしない。

## 前提

- このプロジェクトの開発基盤は Docker / Docker Compose を前提とする(確定方針。経緯は `doc/design/decision-log/0002-docker-compose-baseline.md`)。
- アプリケーションの実装スタックは Go である(`doc/design/architecture.md`、経緯は `doc/design/decision-log/0032-implementation-language.md`)。
- Compose 構成は repo root の `compose.yaml` に置き、開発用 service 名は `dev` とする。`dev` は `golang` 公式 image を base にし、Go の module / build cache を named volume に保持する。

## 基本方針

- AI agent は、依存パッケージの install、アプリケーションの実行、test、asset build など、**開発環境を host OS 上に直接構築・実行する類のコマンド**を host OS 側で直接実行しない。
- これらの作業は原則 Docker / Docker Compose 経由で実行する。
- host OS 側で直接実行する必要がある場合は、理由を説明し、ユーザーの明示承認を得てから実行する。

## Docker の確認

開発コマンドを実行する前に、Docker が利用できるか確認する。

```sh
docker --version
docker compose version
docker info
```

`docker compose` が使えず `docker-compose` だけが使える環境では、同等の Compose コマンドとして `docker-compose` を使ってよい。

`docker` コマンドが見つからない、または Docker daemon に接続できない場合は、host OS 上で開発環境を直接構築する方法へ即時 fallback しない。Docker Desktop などを起動して再試行できるかユーザーに確認する。

## 実行の基本形

開発作業は `dev` service 経由の `docker compose run` を基本形とする。

```sh
docker compose run --rm dev <command>
```

代表的なコマンド例:

```sh
docker compose run --rm dev go build ./...
docker compose run --rm dev go vet ./...
docker compose run --rm dev go mod tidy
docker compose run --rm dev go run ./cmd/slapex --help
docker compose run --rm dev go run ./tools/genemoji
```

時刻が出力内容や出力ディレクトリ名に影響するコマンド(アプリ実行や E2E など)では、timezone を明示する。`dev` service は host の `TZ` を引き継ぐため、host 側で `TZ` が設定済みなら通常の Compose 実行で反映される。実行ごとに固定したい場合は `-e TZ=Asia/Tokyo` のように指定する。

```sh
TZ=Asia/Tokyo docker compose run --rm dev go run ./cmd/slapex
docker compose run --rm -e TZ=Asia/Tokyo dev go run ./cmd/slapex
```

実行時に環境変数(例: `SLACK_BOT_TOKEN`)を渡す場合は、`-e` で host 環境から forward し、compose ファイルや repo 内に実値を書かない。

```sh
docker compose run --rm -e SLACK_BOT_TOKEN dev ./bin/slapex <channel>
```

依存 service が不要な確認コマンドでは、不要な container 起動を避けるため `--no-deps` を付けてよい。

```sh
docker compose run --rm --no-deps dev <command>
```

## 例外

次のようなコマンドは host OS 側で実行してよい。

- `git status`、`git diff`、`git log` などの Git 確認コマンド。
- `rg`、`sed`、`find`、`ls` など、リポジトリの調査に使う読み取り中心のコマンド。
- `docker` / `docker compose` 自体の確認コマンド(`docker info`、`docker compose ps`、`docker compose config` など)。

ただし、依存パッケージの install、アプリケーションの実行、test、asset build など、開発環境を host OS 上に構築・実行する類の作業は Docker Compose 経由を原則とする。
