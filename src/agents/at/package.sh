#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/at>}
mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
install -m 0755 "$here/at.py" "$dest/at.py"
install -m 0644 "$here/agent.json" "$dest/"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/scheduler.py" "$here/../shared/cronexpr.py" "$dest/support/agents/shared/"
for f in scheduler.py cronexpr.py; do cmp -s "$here/../shared/$f" "$dest/support/agents/shared/$f" || { echo "package.sh: the shared engine copy $f differs" >&2; exit 1; }; done
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../../../LICENSE.md" "$here/../../../NOTICE.md" "$here/../../../THIRD-PARTY-NOTICES.md" "$dest/"
