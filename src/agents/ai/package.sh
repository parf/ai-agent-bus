#!/bin/sh
# Kept processes that serve #ai's own inbox through the bundled Python client; no external dependencies.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/ai>}
. "$here/../package-tree.sh"
package_begin "$dest"
mkdir -p "$dest"
install -m 0755 "$here/ai.py" "$dest/ai.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"

mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/provider_http.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../VERSION" "$dest/support/agents/VERSION"
install -m 0644 "$here/../protocol.json" "$dest/support/agents/protocol.json"
package_commit
