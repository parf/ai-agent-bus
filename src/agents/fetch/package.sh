#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/fetch>}
mkdir -p "$dest/support/agents/shared" "$dest/support/internal/version"
install -m 0755 "$here/fetch.py" "$dest/fetch.py"
install -m 0644 "$here/worker.py" "$here/policy.py" "$here/transport.py" "$here/extract.py" "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"
install -m 0644 "$here/../shared/metadata.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
