#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/smtp>}
mkdir -p "$dest/support/agents/shared" "$dest/support/internal/version"
install -m 0755 "$here/smtp.py" "$dest/smtp.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/mail.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
