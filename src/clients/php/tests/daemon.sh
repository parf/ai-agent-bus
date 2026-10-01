#!/bin/bash
# Runs "$@" against a throwaway daemon: its own socket, port and database
# under the repository's tmp/, never the node's. The checkout's programs are
# used as built (src/build.sh); CLIENT_BIN names another directory of them.
# The command gets AB_TEST_ADDR, AB_TEST_OWNER, AB_TEST_OWNER_TOKEN and
# AB_TEST_DIR; KEEP=1 leaves the directory behind for a look.
set -uo pipefail
# Hermetic: nothing of the caller's own bus session reaches the tests.
unset AGENT_BUS_ADDR AGENT_BUS_TOKEN AGENT_BUS_NAME AGENT_BUS_TLS_FINGERPRINT
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
bin=${CLIENT_BIN:-$root/src}
for p in agent-busd agent-bus-token; do
  [ -x "$bin/$p" ] || { echo "no $bin/$p: run src/build.sh first" >&2; exit 2; }
done
mkdir -p "$root/tmp"
D=$(mktemp -d "$root/tmp/clients-test.XXXXXX")
export XDG_STATE_HOME=$D/state
port=$((20000 + RANDOM % 20000))
owner=parf@localhost
"$bin/agent-busd" -addr 127.0.0.1:$port -socket "$D/bus.sock" -owner "$owner" \
  -db "$D/bus.db" -create -flush-every 0 >"$D/daemon.log" 2>&1 &
dpid=$!
cleanup() {
  kill "$dpid" 2>/dev/null; wait "$dpid" 2>/dev/null
  [ "${KEEP:-}" = 1 ] && echo "kept $D" >&2 || rm -rf "$D"
}
trap cleanup EXIT
# Any answer at all means the bus is serving: the socket file appears before it is.
for _ in $(seq 1 100); do
  [ -n "$(curl -s --max-time 1 --unix-socket "$D/bus.sock" http://unix/status 2>/dev/null)" ] && break
  sleep 0.1
done
token=$(AGENT_BUS_ADDR="$D/user-$(id -un).sock" "$bin/agent-bus-token" "$owner") || {
  echo "no owner token" >&2; cat "$D/daemon.log" >&2; exit 2; }
AB_TEST_ADDR=$D/bus.sock AB_TEST_OWNER=$owner AB_TEST_OWNER_TOKEN=$token AB_TEST_DIR=$D \
  AB_TEST_TCP=http://127.0.0.1:$port "$@"
