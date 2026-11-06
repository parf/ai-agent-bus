#!/bin/bash
# The development node rotates the daemon's logs by the tracked rule
# (maintenance/agent-bus.logrotate: daily, 10 MiB, three archives), not by the
# weekly rule `agent-bus-setup` writes on every setup run. Both are the one file
# /etc/logrotate.d/agent-bus, so installing this one replaces setup's: logrotate
# refuses a second rule for the same log files. Usage: git-install-logrotate.sh [dest]
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:-/etc/logrotate.d/agent-bus}
install -m 0644 "$here/maintenance/agent-bus.logrotate" "$dest"
echo "rotating the daemon's logs by $here/maintenance/agent-bus.logrotate ($dest)"
