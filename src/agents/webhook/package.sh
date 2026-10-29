#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/webhook>}
mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
install -m 0755 "$here/webhook.py" "$dest/webhook.py"
install -m 0644 "$here/rules.py" "$here/incoming.py" "$here/outgoing.py" "$here/agent.json" "$here/env.dist" "$here/config.json.dist" "$dest/"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/envfile.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../../../LICENSE.md" "$here/../../../NOTICE.md" "$here/../../../THIRD-PARTY-NOTICES.md" "$dest/"
