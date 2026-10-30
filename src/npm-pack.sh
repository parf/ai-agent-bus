#!/bin/bash
# Wrap verified full/client release archives; never publish.
# Names are centralized in npm/pack.py, overridable with NPM_BUS_NAME and
# NPM_CLIENT_NAME. Commands keep their agent-bus / ab-* names.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
exec python3 "$here/npm/pack.py" "$@"
