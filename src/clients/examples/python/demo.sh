#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export PYTHONPATH="$here/../../python${PYTHONPATH:+:$PYTHONPATH}"
exec python3 "${AB_PYTHON_EXAMPLE_FILE:-$here/worker.py}" --demo
