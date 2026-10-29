#!/bin/sh
# The runner package for a person's user-account runner (docs/07-setup.md#a-persons-user-account-runner);
# run it as that person: package.sh ~/.local/share/agent-bus/agent.d/tmux-sessions
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/tmux-sessions>}
mkdir -p "$dest/support/agents/shared" "$dest/support/internal/version"
install -m 0755 "$here/sessions.py" "$dest/sessions.py"
install -m 0644 "$here/agent.json" "$here/config.json.dist" "$dest/"
install -m 0644 "$here/../../shared/metadata.py" "$here/../../shared/envfile.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../../internal/version/VERSION" "$dest/support/internal/version/"
# Licenses sit in agents/ of a release and at the root of a checkout.
for legal in "$here/../.." "$here/../../../.."; do [ -f "$legal/LICENSE.md" ] && break; done
install -m 0644 "$legal/LICENSE.md" "$legal/NOTICE.md" "$legal/THIRD-PARTY-NOTICES.md" "$dest/"
