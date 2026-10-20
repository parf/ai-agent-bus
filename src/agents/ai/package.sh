#!/bin/sh
# No client or external dependencies: the runner supplies and answers the envelope.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/ai>}
mkdir -p "$dest"
install -m 0755 "$here/ai.py" "$dest/ai.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
