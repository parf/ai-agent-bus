#!/bin/bash
set -eu
cd "$(dirname "$0")"
source ./smoke-options.sh
check() {
  local label=$1 want=$2; shift 2
  local got
  got=$(unset MODE TARGETS; smoke_options "$@"; echo "$MODE:$TARGETS:$SHARD")
  [ "$got" = "$want" ] || { echo "$label: $got, want $want"; exit 1; }
}
check default-fast fast::
check fast-alias fast:: --fast
check normal-changed normal:: --normal
check normal-override normal:A,F: --normal A,F
check slow-override slow:L: --slow L
check release slow:all: --slow all
check normal-shard normal::go_checks --normal --shard go_checks
[ "$(MODE=normal TARGETS=A; smoke_options --shard core; echo "$MODE:$TARGETS:$SHARD")" = normal:A:core ]
for flag in --heavy --all --unknown; do
  rc=0; smoke_options "$flag" >/dev/null 2>&1 || rc=$?
  [ "$rc" = 2 ] || { echo "$flag accepted: $rc"; exit 1; }
done
echo 'normal, fast, slow, inherited shard modes and removed flags: green'
