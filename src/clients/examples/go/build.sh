#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)
build=${AGENT_BUS_EXAMPLE_BUILD_DIR:-$repo/tmp/go-example}
mkdir -p "$build/tmp"
(cd "$here" && TMPDIR="$build/tmp" go build -o "$build/worker" .)
printf '%s\n' "$build/worker"
