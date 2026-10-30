#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/imap>}
mkdir -p "$dest/support/agents/shared" "$dest/support/clients/python/agent_bus" "$dest/support/internal/version"
install -m 0755 "$here/imap.py" "$dest/imap.py"
install -m 0644 "$here/cache.py" "$here/wire.py" "$here/budget.py" "$here/matching.py" "$here/progress.py" "$here/outbox.py" "$here/publication.py" "$here/forwarding.py" "$here/mailbox_session.py" "$here/agent.json" "$here/env.dist" "$dest/"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/mail.py" "$here/../shared/mail_oauth.py" "$here/../shared/subscriptions.py" "$here/../shared/subscription_filters.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../clients/python/agent_bus/"*.py "$dest/support/clients/python/agent_bus/"
install -m 0644 "$here/../../clients/LICENSE.md" "$dest/support/clients/LICENSE.md"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
install -m 0644 "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"
