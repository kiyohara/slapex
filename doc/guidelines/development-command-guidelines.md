# 開発コマンド実行ルール

この文書は、slack_posts_exporter リポジトリで AI agent が開発用コマンドを実行するときの共通正本である。

この方針は AI agent の挙動を規律するものであり、`README.md` の人間向け手順は対象外とする。AI agent は `README.md` に host OS 上で開発環境を直接操作する例があっても、自身が直接実行する根拠にはしない。

## 前提

- このプロジェクトの開発基盤は Docker / Docker Compose を前提とする(確定方針)。
- 一方で、アプリケーションの実装スタック(言語・フレームワーク・パッケージ管理ツールなど)はまだ確定していない。そのため本文では特定スタック固有のコマンド名を前提化せず、汎用的な表現で規律する。
- 実装スタックが確定したら、その時点の具体的な service 名・コマンド例で本文を更新する。

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

開発作業は、Docker Compose の標準的なコマンドとして明示する。具体的な service 名やコマンドは、実装スタックと Compose 構成が確定した時点で具体化する。

```sh
docker compose run --rm <service> <command>
docker compose up
```

依存 service が不要な確認コマンドでは、不要な container 起動を避けるため `--no-deps` を付けてよい。

```sh
docker compose run --rm --no-deps <service> <command>
```

## 例外

次のようなコマンドは host OS 側で実行してよい。

- `git status`、`git diff`、`git log` などの Git 確認コマンド。
- `rg`、`sed`、`find`、`ls` など、リポジトリの調査に使う読み取り中心のコマンド。
- `docker` / `docker compose` 自体の確認コマンド(`docker info`、`docker compose ps`、`docker compose config` など)。

ただし、依存パッケージの install、アプリケーションの実行、test、asset build など、開発環境を host OS 上に構築・実行する類の作業は Docker Compose 経由を原則とする。
