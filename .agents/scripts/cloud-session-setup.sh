#!/usr/bin/env bash
# cloud-session-setup: Claude Code on the web の cloud session で Compose の実行環境を整える。
#
# 背景:
#   cloud session の sandbox には docker CLI / dockerd / compose が入っているが daemon は
#   起動していない。sandbox の Go toolchain は go.mod の要求より古い場合があるため、開発コマンドは
#   通常どおり `docker compose run --rm dev go ...` で実行する
#   (doc/guidelines/development-command-guidelines.md)。この script は daemon を起動し、
#   dev service の image が無ければ pull して、その前提を満たす。cloud 固有の差分 (container を
#   agent proxy 経由にする、image を mirror から取る) は compose.cloud.yaml に置き、
#   COMPOSE_FILE で重ねる。
#   あわせて、GitHub MCP tool に無い操作を補うための gh (GitHub CLI) を `--provision` で入れる。
#   認証は platform の GitHub proxy が request ごとに差し替えるため、token は扱わない
#   (doc/guidelines/github-mcp-guidelines.md の「cloud session」)。
#
# 呼び出し元:
#   - .claude/settings.json の SessionStart hook (startup|resume)。既定 mode。
#     CLAUDE_CODE_REMOTE=true 以外では何も出力せず exit 0 する。
#   - environment の setup script (claude.ai/code の UI 設定): `--provision`。
#     `--print-stub` が生成した stub を UI に貼る。setup script は agent proxy が立つ前に
#     走るため container 内から外へは出られない。そこで gh の導入 (Ubuntu archive)、dev image の
#     pull (mirror)、state file の記録だけを行い、`go mod download` などは行わない。environment は
#     slapex 専用とし、branch をまたいで共有されるため、stub は script が無ければ何もせず exit 0 する。
#
# 安全策:
#   - 冪等。何度実行しても同じ状態に収束する。
#   - 非対話。secret を扱わない。環境変数の値や log 全文を stdout に出さない。
#   - `--doctor` と引数の誤り以外は、失敗しても exit 0 とする。setup script が非 0 で終わると
#     session が起動せず、SessionStart hook の stdout は agent の context に入るため、状況は
#     短く stdout に出す。
#   - `set -e` は使わない。外部コマンドの失敗は個別に扱う。
#
# 使い方:
#   .agents/scripts/cloud-session-setup.sh [--force | --provision | --doctor | --print-stub]
#
# 詳細: doc/guidelines/cloud-session-guidelines.md

set -uo pipefail

self="cloud-session-setup"
mode="hook"

usage() {
  cat <<'USAGE'
usage: cloud-session-setup.sh [--force | --provision | --doctor | --print-stub]

Claude Code on the web の cloud session で Docker daemon を起動し、dev service の image を用意する。
引数なしのときは SessionStart hook としての動作で、CLAUDE_CODE_REMOTE=true 以外では何もしない。

options:
  --force       CLAUDE_CODE_REMOTE=true でなくても hook と同じ動作を行う
  --provision   environment の setup script 用。gh を導入し、dev image を pull し、state file を
                書き、自分で起動した daemon を止める
  --doctor      daemon / image / gh / environment cache の状態を表示し、問題があれば exit 1
  --print-stub  environment の setup script に貼る stub を生成する
  -h, --help    この help を表示する
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --force) mode="force"; shift ;;
    --provision) mode="provision"; shift ;;
    --doctor) mode="doctor"; shift ;;
    --print-stub) mode="print-stub"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "$self: unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# guard 1: local の Claude Code でも hook は走る。cloud 以外では無出力で抜ける。
if [ "$mode" = "hook" ] && [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

# repo root は script 自身の位置から解決する。hook は "$CLAUDE_PROJECT_DIR" 配下の script を呼ぶため
# 同じ path になり、setup script からの呼び出しや手動実行でも同じ規則で済む。
script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
repo_root="$(cd "$(dirname "$script_path")/../.." && pwd)"
cd "$repo_root" || { echo "$self: repo root に移動できない" >&2; exit 0; }

compose_files="compose.yaml:compose.cloud.yaml"
state_dir="/opt/slapex-cloud"
state_file="$state_dir/state"
log_file="${TMPDIR:-/tmp}/$self.log"
daemon_log="/var/log/dockerd-cloud-session.log"
# cloud VM の既定 clone 先の候補。stub の fallback にだけ使う (個人環境の path ではない)。
# 単一 repository の session は /home/user/<repo>、複数 repository の session (project の thread
# など) は /home/claude/<repo> に clone される (2026-09-24 実測)。
default_clone_dirs="/home/user/slapex /home/claude/slapex"

started_daemon=0

say() { echo "$self: $*"; }

compose() { COMPOSE_FILE="$compose_files" docker compose "$@"; }

# guard 2: 期待する sandbox の構成か。dockerd が無い環境 (macOS など) では何もしない。
is_cloud_sandbox() {
  [ "$(uname -s)" = "Linux" ] \
    && command -v docker >/dev/null 2>&1 \
    && command -v dockerd >/dev/null 2>&1
}

# environment cache の再構築は setup script のテキストが変わったときにしか起きない。
# snapshot の内容を決める入力の digest を stub に埋め、貼り直しでテキストが変わるようにする。
# script 自身は stdin から読み、clone 先の path が digest に混ざらないようにする。
inputs_digest() {
  {
    sha256sum < "$script_path"
    cat "$repo_root/compose.yaml" "$repo_root/compose.cloud.yaml"
  } 2>/dev/null | sha256sum | cut -d' ' -f1
}

recorded_digest() {
  [ -f "$state_file" ] && sed -n 's/^inputs_sha256=//p' "$state_file"
}

daemon_ready() { docker info >/dev/null 2>&1; }

# pid file の pid が期待する daemon 本体かを見る。VM の作り直しで pid 番号が別 process に
# 再利用されることがあるため、process の存在だけで判断しない。
pid_is_process() {
  local pid="$1" name="$2" comm
  [ -n "$pid" ] || return 1
  comm="$(cat "/proc/$pid/comm" 2>/dev/null || true)"
  [ "$comm" = "$name" ]
}

# /run は root filesystem 上にあり、daemon 起動中に snapshot されると pid file と socket が
# 残る。pid file はその pid が dockerd / containerd でなければ消す。socket は dockerd process が
# 1 つも無いときだけ消し、起動中や応答待ちの daemon の socket には触らない。
remove_stale_runtime_files() {
  local f pid
  for f in /var/run/docker.pid /var/run/docker-ssd.pid; do
    [ -f "$f" ] || continue
    pid="$(cat "$f" 2>/dev/null || true)"
    pid_is_process "$pid" dockerd || rm -f "$f"
  done
  if [ -f /run/containerd/containerd.pid ]; then
    pid="$(cat /run/containerd/containerd.pid 2>/dev/null || true)"
    pid_is_process "$pid" containerd || rm -f /run/containerd/containerd.pid
  fi
  if [ -S /var/run/docker.sock ] && ! pgrep -x dockerd >/dev/null 2>&1; then
    rm -f /var/run/docker.sock
  fi
  return 0
}

# pid file の process が生きているのに API が応答しないのは、起動中か停止中である。
# provision の停止処理の直後や前 session の終了直後に当たるため、少し待ってから判断する。
wait_daemon_transition() {
  local i pid
  pid="$(cat /var/run/docker.pid 2>/dev/null || true)"
  pid_is_process "$pid" dockerd || return 0
  for i in $(seq 1 20); do
    daemon_ready && return 0
    pid_is_process "$pid" dockerd || return 0
    sleep 1
  done
  return 0
}

# `service docker start` は使わない。sandbox では init script の ulimit 変更が
# Operation not permitted で失敗し、daemon が起動しないため dockerd を直接起動する。
start_daemon() {
  local i
  if daemon_ready; then
    say "dockerd: 起動済み"
    return 0
  fi
  wait_daemon_transition
  if daemon_ready; then
    say "dockerd: 起動済み"
    return 0
  fi
  remove_stale_runtime_files
  nohup dockerd >>"$daemon_log" 2>&1 </dev/null &
  for i in $(seq 1 30); do
    if daemon_ready; then
      started_daemon=1
      say "dockerd: 起動した (${i}s)"
      return 0
    fi
    sleep 1
  done
  say "dockerd: 30 秒以内に起動しなかった"
  tail -n 5 "$daemon_log" 2>/dev/null | cut -c1-200 | sed "s/^/$self:   /"
  return 1
}

stop_daemon() {
  local i pid
  [ "$started_daemon" -eq 1 ] || return 0
  pid="$(cat /var/run/docker.pid 2>/dev/null || true)"
  if [ -n "$pid" ]; then
    kill -TERM "$pid" 2>/dev/null || true
  else
    pkill -TERM -x dockerd 2>/dev/null || true
  fi
  # API が閉じた後も process の終了処理が続く。process が消えるまで待ち、pid file を残さない。
  for i in $(seq 1 30); do
    if [ -n "$pid" ]; then
      pid_is_process "$pid" dockerd || break
    else
      pgrep -x dockerd >/dev/null 2>&1 || break
    fi
    sleep 1
  done
  remove_stale_runtime_files
  say "dockerd: 停止した (snapshot に process を残さない)"
  return 0
}

# dev service の image。cloud では compose.cloud.yaml が mirror の参照に差し替える。
# slapex の dev は Dockerfile を持たず `image:` 指定なので、build ではなく pull で用意する。
dev_image() { compose config --images dev 2>>"$log_file" | head -n 1; }
local_dev_image() { docker compose -f compose.yaml config --images dev 2>>"$log_file" | head -n 1; }

# compose.yaml (local) と compose.cloud.yaml (cloud) で dev の image を二重に持つため、
# registry 部分を除いた repository:tag が一致するかを見る。
image_tag_matches() {
  local cloud local_ref
  cloud="$(dev_image)"
  local_ref="$(local_dev_image)"
  [ -n "$cloud" ] && [ -n "$local_ref" ] || return 1
  [ "${cloud##*/}" = "${local_ref##*/}" ]
}

# dev image を用意する。setup script の文脈では daemon が直接 (VM の system CA で) mirror から
# pull できる。hook の文脈では daemon が agent proxy を経由する。
ensure_image() {
  local image
  image="$(dev_image)"
  if [ -z "$image" ]; then
    say "image: compose 定義から dev の image 名を取れない"
    return 1
  fi
  if docker image inspect "$image" >/dev/null 2>&1; then
    say "image: $image あり"
    return 0
  fi
  say "image: $image を pull する"
  if docker pull -q "$image" >>"$log_file" 2>&1 </dev/null; then
    say "image: pull 完了"
    return 0
  fi
  say "image: pull に失敗した"
  tail -n 3 "$log_file" 2>/dev/null | cut -c1-200 | sed "s/^/$self:   /"
  return 1
}

gh_version() { gh --version 2>/dev/null | head -n 1 | cut -d' ' -f3; }

# gh の有無を 1 行で示す。hook では導入しない (session 開始を遅らせず、導入経路を
# environment cache の 1 本に揃える)。
report_gh() {
  if command -v gh >/dev/null 2>&1; then
    say "gh: $(gh_version) あり (GitHub MCP tool に無い操作だけ gh api で行う。詳細: doc/guidelines/github-mcp-guidelines.md)"
  else
    say "gh: 無し (environment の setup script の --provision で入る)"
  fi
}

# gh を Ubuntu archive から入れる。archive.ubuntu.com は既定の許可リストにあり、setup script の
# 文脈 (agent proxy が無い) でも VM の system CA で届く。package list が古くて失敗したときだけ
# update してやり直す。失敗しても session の起動は止めない。
# setup script は 5 分以内に終わる必要があり、後に dev image の pull が続く。apt-get の
# 3 段は gh_step_timeout ずつ、最悪でも合計 135 秒で打ち切る。
gh_step_timeout=45

ensure_gh() {
  if command -v gh >/dev/null 2>&1; then
    say "gh: $(gh_version) あり"
    return 0
  fi
  if [ "$(id -u)" -ne 0 ] || ! command -v apt-get >/dev/null 2>&1; then
    say "gh: root の apt-get が使えないため導入しない"
    return 1
  fi
  say "gh: Ubuntu archive から導入する"
  if DEBIAN_FRONTEND=noninteractive timeout "$gh_step_timeout" apt-get install -y -q --no-install-recommends gh \
       >>"$log_file" 2>&1 </dev/null \
     || { DEBIAN_FRONTEND=noninteractive timeout "$gh_step_timeout" apt-get update -q >>"$log_file" 2>&1 </dev/null \
          && DEBIAN_FRONTEND=noninteractive timeout "$gh_step_timeout" apt-get install -y -q --no-install-recommends gh \
               >>"$log_file" 2>&1 </dev/null; }; then
    say "gh: $(gh_version) を導入した"
    return 0
  fi
  say "gh: 導入に失敗した (session は続行する。GitHub 操作は MCP tool で行う)"
  tail -n 3 "$log_file" 2>/dev/null | cut -c1-200 | sed "s/^/$self:   /"
  return 1
}

# COMPOSE_FILE を session 全体へ渡す。CLAUDE_ENV_FILE は SessionStart hook が後続の
# shell へ環境変数を渡すための file。無い文脈 (setup script、手動実行) では export 文を示す。
export_compose_file() {
  export COMPOSE_FILE="$compose_files"
  if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
    if ! grep -qs "^export COMPOSE_FILE=" "$CLAUDE_ENV_FILE"; then
      echo "export COMPOSE_FILE=$compose_files" >> "$CLAUDE_ENV_FILE"
    fi
    say "COMPOSE_FILE=$compose_files を session に設定した"
  else
    say "この shell の外で使うときは export COMPOSE_FILE=$compose_files を先に実行する"
  fi
}

write_state() {
  mkdir -p "$state_dir" || return 1
  cat > "$state_file" <<STATE
inputs_sha256=$(inputs_digest)
built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
image=$(dev_image)
STATE
  say "state: $state_file を書いた"
}

print_stub() {
  cat <<STUB
#!/bin/bash
# slapex: cloud session の環境構築。正本は repository 側にある。
#   .agents/scripts/cloud-session-setup.sh
# この stub は薄いままにする。中身を変えるときは repository を直す。
# environment は slapex 専用で、branch をまたいで共有される。script が無ければ何もせず exit 0 する
# (setup script が非 0 で終わると session が起動しない)。
# CLOUD_SETUP_INPUTS_SHA256=$(inputs_digest)
#   ↑ script と compose.yaml / compose.cloud.yaml の digest。
#     environment cache は setup script のテキストが変わったときだけ再構築されるため、
#     digest を埋めてテキストが自然に変わるようにしている。
#     貼り直す内容は \`.agents/scripts/cloud-session-setup.sh --print-stub\` で生成する。
for dir in $default_clone_dirs; do
  script="\$dir/.agents/scripts/cloud-session-setup.sh"
  if [ -f "\$script" ]; then
    exec bash "\$script" --provision
  fi
done
echo "slapex cloud-session-setup: この repository / branch に script が無いため provision を skip する"
exit 0
STUB
}

# snapshot が repo の宣言に追いついているかを見る。state が無ければ登録の案内だけ、
# digest がずれていれば貼り直し用の stub を出す。戻り値が 1 なのは digest のずれだけ。
check_drift() {
  local want have
  want="$(inputs_digest)"
  have="$(recorded_digest || true)"
  if [ -z "$have" ]; then
    say "environment cache: 未登録 (setup script に stub を貼ると session 開始が速くなる。生成: --print-stub)"
    return 0
  fi
  if [ "$want" = "$have" ]; then
    say "environment cache: repo と一致 (${have:0:12}...)"
    return 0
  fi
  say "environment cache が repo に追いついていない (cache=${have:0:12}... repo=${want:0:12}...)"
  say "対処: 次の stub を cloud environment の setup script に貼り直すと cache が再構築される。"
  echo
  print_stub
  echo
  return 1
}

doctor() {
  local image rc=0
  if ! is_cloud_sandbox; then
    say "この環境は cloud session の sandbox ではない (dockerd が無い)"
    return 1
  fi
  if daemon_ready; then say "dockerd: 起動済み"; else say "dockerd: 停止中"; fi
  image="$(dev_image)"
  if [ -n "$image" ] && docker image inspect "$image" >/dev/null 2>&1; then
    say "image: $image あり"
  else
    say "image: ${image:-?} 無し"
  fi
  if image_tag_matches; then
    say "image tag: compose.yaml と compose.cloud.yaml で一致 (${image##*/})"
  else
    say "image tag: compose.yaml ($(local_dev_image)) と compose.cloud.yaml (${image:-?}) で一致しない。compose.cloud.yaml の dev の image を揃える"
    rc=1
  fi
  report_gh
  if [ -f "$state_file" ]; then
    say "state: $(sed -n 's/^built_at=//p' "$state_file") に作成"
  else
    say "state: 無し"
  fi
  check_drift || rc=1
  return "$rc"
}

case "$mode" in
  print-stub)
    print_stub
    exit 0
    ;;
  doctor)
    doctor
    exit $?
    ;;
esac

# hook / force / provision
if ! is_cloud_sandbox; then
  say "この環境は cloud session の sandbox ではない (dockerd が無い)。何もしない。"
  exit 0
fi

export_compose_file

# gh は daemon と独立しているため、daemon の起動に失敗しても入れる。hook は有無の表示だけを
# daemon や image の成否より先に行う。
if [ "$mode" = "provision" ]; then
  ensure_gh || true
else
  report_gh
fi

if ! start_daemon; then
  say "WARNING: Docker daemon を起動できなかった。host の go で代替せず、実行できない検証は未実施として報告する。"
  say "  再試行: bash .agents/scripts/cloud-session-setup.sh --force / 状態確認: --doctor / 詳細: doc/guidelines/cloud-session-guidelines.md"
  exit 0
fi

if [ "$mode" = "provision" ]; then
  ensure_image || true
  write_state || say "state: 書けなかった"
  stop_daemon
  say "provision 完了"
  exit 0
fi

if ! ensure_image; then
  say "WARNING: dev の image を用意できなかった。log: $log_file"
  say "  host の go で代替せず、実行できない検証は未実施として報告する。詳細: doc/guidelines/cloud-session-guidelines.md"
  exit 0
fi

if ! image_tag_matches; then
  say "WARNING: compose.yaml と compose.cloud.yaml で dev の image tag が一致しない。--doctor で確認する"
fi
check_drift || true
say "開発コマンドは通常どおり docker compose run --rm dev go ... で実行する。詳細: doc/guidelines/cloud-session-guidelines.md"
exit 0
