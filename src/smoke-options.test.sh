#!/bin/bash
set -eu
cd "$(dirname "$0")"
source ./smoke-options.sh
check() {
  local label=$1 want=$2; shift 2
  local got
  got=$(unset FAST SLOW; smoke_options "$@"; echo "$FAST:$SLOW:$SHARD")
  [ "$got" = "$want" ] || { echo "$label: $got, want $want"; exit 1; }
}
check normal-gate 0:0:
check fast-subset 1:0: --fast
check slow-superset 0:1: --slow
check normal-shard 0:0:go_checks --shard go_checks
check fast-shard 1:0:go_checks --fast --shard go_checks
check slow-shard 0:1:go_checks --slow --shard go_checks
# A driver passes its selected mode to its shards through the environment.
[ "$(FAST=1 SLOW=0; smoke_options --shard core; echo "$FAST:$SLOW:$SHARD")" = 1:0:core ]
[ "$(FAST=0 SLOW=1; smoke_options --shard core; echo "$FAST:$SLOW:$SHARD")" = 0:1:core ]
for flag in --heavy --all --unknown; do
  rc=0; smoke_options "$flag" >/dev/null 2>&1 || rc=$?
  [ "$rc" = 2 ] || { echo "$flag accepted: $rc"; exit 1; }
done
echo 'normal, fast, slow, inherited shard modes and removed flags: green'
