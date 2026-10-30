#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/eventlog>}
mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
install -m 0755 "$here/eventlog.py" "$dest/eventlog.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/config.json.dist" "$dest/"
for file in metadata.py event_rings.py subscriptions.py subscription_filters.py; do
  install -m 0644 "$here/../shared/$file" "$dest/support/agents/shared/"
  cmp -s "$here/../shared/$file" "$dest/support/agents/shared/$file" || exit 1
done
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"
