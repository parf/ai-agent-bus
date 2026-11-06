#!/bin/sh
# No client or external dependencies: the runner supplies and answers the envelope.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/qdrant>}
. "$here/../package-tree.sh"
package_begin "$dest"
mkdir -p "$dest"
install -m 0755 "$here/qdrant.py" "$dest/qdrant.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"

mkdir -p "$dest/support/agents/shared"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/provider_http.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../VERSION" "$dest/support/agents/VERSION"
install -m 0644 "$here/../protocol.json" "$dest/support/agents/protocol.json"
package_commit
