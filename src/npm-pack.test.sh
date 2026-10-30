#!/bin/bash
# Install from a private loopback registry and exercise the installed commands.
# No host installation, publish, accounts, units or timers.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
exec python3 -B "$here/npm/check.py" "$@"
