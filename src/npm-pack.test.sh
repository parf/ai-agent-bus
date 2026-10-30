#!/bin/bash
# Install from a private loopback registry and exercise the installed commands.
# No host installation, publish, accounts, units or timers.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
python3 -B "$here/npm/check.py" "$@"
# The macOS platform packages: built from macOS archives, wired into both command packages.
python3 -B "$here/npm/darwin_check.py"
