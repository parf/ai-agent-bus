#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)
export AGENT_BUS_NAME=${AGENT_BUS_NAME:-'#stats-go@localhost'}
export AGENT_BUS_TOKEN=${AGENT_BUS_TOKEN:-$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")}
export STATS_ROOT=${STATS_ROOT:-${EXAMPLES_ROOT:-$repo}}
build=${AGENT_BUS_EXAMPLE_BUILD_DIR:-$repo/tmp/go-example}
mkdir -p "$build/tmp"
(cd "$here" && TMPDIR="$build/tmp" go build -o "$build/worker" .)
exec "$build/worker" "$@"
