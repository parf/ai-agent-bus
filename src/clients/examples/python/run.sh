#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export AGENT_BUS_NAME=${AGENT_BUS_NAME:-'#stats-python@localhost'}
export AGENT_BUS_TOKEN=${AGENT_BUS_TOKEN:-$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")}
export STATS_ROOT=${STATS_ROOT:-$EXAMPLES_ROOT}
export PYTHONPATH="$here/../../python${PYTHONPATH:+:$PYTHONPATH}"
exec python3 "${AB_PYTHON_EXAMPLE_FILE:-$here/worker.py}" "$@"
