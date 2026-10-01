#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
file=${1:-${DEMO_FILE:-}}
if [ -z "$file" ] || [ $# -gt 1 ]; then echo 'usage: demo.sh /absolute/path/to/file.json' >&2; exit 2; fi
(cd "$here/../../typescript" && bun install --frozen-lockfile >&2)
export AGENT_BUS_NAME=${AGENT_BUS_NAME:-${EXAMPLES_OWNER:-}}
export AGENT_BUS_TOKEN=${AGENT_BUS_TOKEN:-${EXAMPLES_OWNER_TOKEN:-}}
exec bun "$here/demo.ts" "$file"
