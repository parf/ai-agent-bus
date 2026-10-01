#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
(cd "$here/../../typescript" && bun install --frozen-lockfile >&2)
export AGENT_BUS_NAME='#stats-ts@localhost'
if [ -n "${EXAMPLES_TOKENS:-}" ]; then
  export AGENT_BUS_TOKEN=$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")
fi
exec bun "${AB_TS_EXAMPLE_FILE:-$here/worker.ts}"
