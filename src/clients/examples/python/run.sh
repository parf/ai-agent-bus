#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export AGENT_BUS_NAME='#stats-python@localhost'
worker_token=$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")
export AGENT_BUS_TOKEN="$worker_token"
export STATS_ROOT=${STATS_ROOT:-$EXAMPLES_ROOT}
export PYTHONPATH="$here/../../python${PYTHONPATH:+:$PYTHONPATH}"
exec python3 "${AB_PYTHON_EXAMPLE_FILE:-$here/worker.py}" "$@"
