#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)
export AGENT_BUS_NAME='#stats-go@localhost'
worker_token=$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")
export AGENT_BUS_TOKEN="$worker_token"
export STATS_ROOT=${STATS_ROOT:-${EXAMPLES_ROOT:-$repo}}
worker_bin=$(bash "$here/build.sh")
exec "$worker_bin" "$@"
