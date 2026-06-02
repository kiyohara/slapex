# 開発コマンド実行ルール

この文書は、slack_posts_exporter リポジトリで AI agent が開発用コマンドを実行するときの共通正本である。

この方針は AI agent の挙動を規律するものであり、`README.md` の人間向け手順は対象外とする。AI agent は `README.md` に host OS 側の `bundle exec ...` 例があっても、自身が直接実行する根拠にはしない。

## 基本方針

- このプロジェクトの開発基盤は Docker / Docker Compose を前提とする。
- AI agent は Rails / Bundler / Node.js / Yarn 系のコマンドを host OS 側で直接実行しない。
- `bundle install`、`bundle exec rails ...`、`rails ...`、`npm install`、`npm run ...`、`yarn install`、`yarn ...` は原則 Docker Compose 経由で実行する。
- host OS 側でこれらを直接実行する必要がある場合は、理由を説明し、ユーザーの明示承認を得てから実行する。

## Docker の確認

開発コマンドを実行する前に、Docker が利用できるか確認する。

```sh
docker --version
docker compose version
docker info
```

`docker compose` が使えず `docker-compose` だけが使える環境では、同等の Compose コマンドとして `docker-compose` を使ってよい。

`docker` コマンドが見つからない、または Docker daemon に接続できない場合は、host OS 側の `bundle` / `npm` / `yarn` へ即時 fallback しない。Docker Desktop などを起動して再試行できるかユーザーに確認する。

## 推奨コマンド

Rails / Bundler:

```sh
docker compose run --rm web bundle exec rails ...
docker compose run --rm web bundle install
```

DB などの依存 service が不要な確認コマンドでは、不要な container 起動を避けるため `--no-deps` を付けてよい。

```sh
docker compose run --rm --no-deps web bundle exec rails --version
docker compose run --rm --no-deps build yarn --version
```

Rails test:

```sh
docker compose run --rm web bundle exec rails test
docker compose run --rm web bundle exec rails test test/path/to/test.rb
```

Yarn / stylesheet(Node.js ツールチェーンは `build` service 側に同梱されている):

```sh
docker compose run --rm build yarn install
docker compose run --rm build yarn build:css
```

アプリケーション起動:

```sh
docker compose up
```

## development helper の扱い

`development/` 配下には人間が Docker 操作を簡易に行うための helper がある。AI agent はこれらを既存運用の参考として読んでよいが、直接利用する前提にはしない。

AI agent が実行するコマンドは、できるだけ `docker compose run --rm ...` や `docker compose up` のような標準的な Compose コマンドとして明示する。

## 例外

次のようなコマンドは host OS 側で実行してよい。

- `git status`、`git diff`、`git log` などの Git 確認コマンド。
- `rg`、`sed`、`find`、`ls` など、リポジトリの調査に使う読み取り中心のコマンド。
- `docker` / `docker compose` 自体の確認コマンド(`docker info`、`docker compose ps`、`docker compose config` など)。

ただし、依存関係の install、Rails アプリケーションの実行、test、asset build は Docker Compose 経由を原則とする。
