#!/bin/bash
# Export only runtime files, preserving the relative imports used by native agents.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: agents/package.sh <runtime-root>}
mkdir -p "$dest/agents"
dest=$(realpath "$dest")
"$here/slack/package.sh" "$dest/agents/slack"
"$here/telegram/package.sh" "$dest/agents/telegram"
"$here/ai/package.sh" "$dest/agents/ai"
if [ -x "$here/debug/package.sh" ]; then
  "$here/debug/package.sh" "$dest/agents/debug"
else
  for file in agent.json env.dist debug.py; do
    mode=0644
    [ "$file" != debug.py ] || mode=0755
    install -D -m "$mode" "$here/debug/$file" "$dest/agents/debug/$file"
  done
fi
files=(
  shared/metadata.py
  standalone/sessions/agent.json standalone/sessions/sessions.py standalone/sessions/install.py
  standalone/install.py
  standalone/systemd/agent.json standalone/systemd/env.dist standalone/systemd/systemd.py standalone/systemd/install.py
  standalone/server-info/agent.json standalone/server-info/env.dist standalone/server-info/server-info.py standalone/server-info/install.sh
  standalone/git/agent.json standalone/git/env.dist standalone/git/git_agent.py standalone/git/install.py standalone/git/autopull.py standalone/git/timers.py
  standalone/logwatch/agent.json standalone/logwatch/env.dist standalone/logwatch/logwatch.py standalone/logwatch/install.py standalone/logwatch/cleaning.py standalone/logwatch/filters.py standalone/logwatch/watcher.py
)
for file in "${files[@]}"; do
  mode=0644
  [ ! -x "$here/$file" ] || mode=0755
  install -D -m "$mode" "$here/$file" "$dest/agents/$file"
done
mkdir -p "$dest/clients/python/agent_bus" "$dest/internal/version"
install -m 0644 "$here"/../clients/python/agent_bus/*.py "$dest/clients/python/agent_bus/"
install -m 0644 "$here/../clients/LICENSE.md" "$dest/clients/LICENSE.md"
install -m 0644 "$here/../internal/version/VERSION" "$dest/internal/version/VERSION"

install -m 0644 "$here/../../LICENSE.md" "$dest/agents/LICENSE.md"
install -m 0644 "$here/../../NOTICE.md" "$dest/agents/NOTICE.md"

install -m 0644 "$here/../../THIRD-PARTY-NOTICES.md" "$dest/agents/THIRD-PARTY-NOTICES.md"
