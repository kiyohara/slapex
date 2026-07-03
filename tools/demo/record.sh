#!/usr/bin/env bash
# README 用ターミナルデモ GIF の録画スクリプト(Issue #115)。
#
# dev container で linux 向けの slapex / gensample バイナリをビルドし、
# vhs container(compose service `vhs`)で tape を再生して
# assets/demo/slapex-demo-ja.gif を生成する。録画はコンテナ内で完結し、
# 通信先は gensample の fake Slack API server(架空データ)だけ。
#
# 使い方:
#   bash tools/demo/record.sh              # demo-ja.tape を録画
#   bash tools/demo/record.sh <tape>...    # 指定した tape を録画
set -euo pipefail
cd "$(dirname "$0")/../.."

docker compose run --rm dev go build -o tools/demo/bin/slapex ./cmd/slapex
docker compose run --rm dev go build -o tools/demo/bin/gensample ./tools/gensample

tapes=("$@")
if [ ${#tapes[@]} -eq 0 ]; then
  tapes=(tools/demo/demo-ja.tape)
fi
for tape in "${tapes[@]}"; do
  docker compose run --rm vhs "$tape"
done
