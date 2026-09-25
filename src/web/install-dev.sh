#!/bin/bash
# Development install of the TypeScript web face (Plans/Web/README.md#process-and-account):
# the agent-bus-web account, /var/lib/agent-bus/web linked to this checkout,
# and the locked-down unit. Run with sudo. Idempotent; --remove undoes it.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run with sudo" >&2; exit 1; }
here=$(cd "$(dirname "$0")" && pwd -P)
case "$here" in /home/*) echo "the checkout must sit outside /home (ProtectHome=yes): $here" >&2; exit 1 ;; esac
unit=/etc/systemd/system/agent-bus-web.service
link=/var/lib/agent-bus/web

if [ "${1:-}" = --remove ]; then
  systemctl disable --now agent-bus-web.service 2>/dev/null || true
  rm -f "$unit"; systemctl daemon-reload
  [ -L "$link" ] && rm -f "$link"
  echo "removed the unit and the link; the agent-bus-web account is kept"
  exit 0
fi

[ -x /usr/bin/bun ] || { echo "the unit runs /usr/bin/bun; install bun there" >&2; exit 1; }
# Its home is its code, the link: it owns nothing and writes nowhere.
if id agent-bus-web >/dev/null 2>&1; then
  if [ "$(getent passwd agent-bus-web | cut -d: -f6)" != "$link" ]; then
    # An account in use cannot be changed: the unit stops first, and restarts below.
    systemctl stop agent-bus-web.service 2>/dev/null || true
    usermod --home "$link" agent-bus-web
  fi
else
  useradd --system --no-create-home --home-dir "$link" --shell /usr/sbin/nologin agent-bus-web
fi
mkdir -p /var/lib/agent-bus
if [ -e "$link" ] && [ ! -L "$link" ]; then echo "$link exists and is not a link; not touching it" >&2; exit 1; fi
ln -sfn "$here" "$link"
# The unit may execute bun and the libraries it links here, resolved, and
# nothing else; where a distribution keeps them differs.
paths="ExecPaths=/usr/bin/bun $(ldd /usr/bin/bun | grep -o '/[^ ]* (0x' | cut -d' ' -f1 | xargs -r -n1 readlink -f | tr '\n' ' ')"
sed "s|^ExecPaths=.*|${paths% }|" "$here/agent-bus-web.service" > "$unit"; chmod 0644 "$unit"
systemctl daemon-reload
systemctl enable agent-bus-web.service >/dev/null
systemctl restart agent-bus-web.service
addr=$(sed -n 's/^Environment=AGENT_BUS_WEB_ADDR=//p' "$unit")
for _ in $(seq 1 50); do
  curl -sf -o /dev/null "http://$addr/healthz" && { echo "agent-bus-web answers on http://$addr/"; exit 0; }
  sleep 0.2
done
echo "agent-bus-web did not answer on $addr; journalctl -u agent-bus-web" >&2
systemctl status --no-pager agent-bus-web.service | tail -15 >&2
exit 1
