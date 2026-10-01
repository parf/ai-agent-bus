#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export STATS_ROOT=${STATS_ROOT:-${EXAMPLES_ROOT:-$(cd "$here/../../../.." && pwd)}}
worker_bin=$(bash "$here/build.sh")
exec "$worker_bin" -demo "${1:-${DEMO_FILE:?give an absolute file path under STATS_ROOT}}"
