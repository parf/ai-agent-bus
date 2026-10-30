#!/bin/sh
# No client or external dependencies: the runner supplies and answers the envelope.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/qdrant>}
mkdir -p "$dest"
install -m 0755 "$here/qdrant.py" "$dest/qdrant.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"

mkdir -p "$dest/support/agents/shared" "$dest/support/internal/version"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/provider_http.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
