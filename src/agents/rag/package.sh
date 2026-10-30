#!/bin/sh
# Self-contained Python application; own manifest SemVer, shared bus client.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/rag>}
mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
for name in rag.py rules.py core.py chunks.py prompts.py blocks.py agent.json env.dist CHANGELOG.md config.json.dist; do
  install -m 0644 "$here/$name" "$dest/$name"
done
chmod 0755 "$dest/rag.py"
install -m 0644 "$here/../shared/metadata.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"
