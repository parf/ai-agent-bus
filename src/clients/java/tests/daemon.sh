#!/bin/bash
# Runs "$@" against a throwaway daemon: its own socket, port and database
# under the repository's tmp/, never the node's. The checkout's programs are
# used as built (src/build.sh); CLIENT_BIN names another directory of them.
# The command gets AB_TEST_ADDR, AB_TEST_OWNER, AB_TEST_OWNER_TOKEN,
# AB_TEST_DIR and AB_TEST_TCP; KEEP=1 leaves the directory behind for a look.
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
owner=parf@localhost
# TLS=1: the TCP port answers TLS as well, from a throwaway self-signed
# certificate; AB_TEST_TLS and AB_TEST_PIN name it.
tls=()
if [ "${TLS:-}" = 1 ]; then
  mkdir -p "$D/tls"
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 1 -subj /CN=127.0.0.1 \
    -addext subjectAltName=IP:127.0.0.1 -keyout "$D/tls/key.pem" -out "$D/tls/cert.pem" 2>/dev/null
  tls=(-tls-dir "$D/tls")
  AB_TEST_PIN=sha256:$(openssl x509 -in "$D/tls/cert.pem" -outform der | sha256sum | cut -d' ' -f1)
  # The same key and certificate for a test's own TLS server, as Java reads them.
  openssl pkcs12 -export -in "$D/tls/cert.pem" -inkey "$D/tls/key.pem" -out "$D/tls/server.p12" -passout pass:test 2>/dev/null
  export AB_TEST_PIN AB_TEST_P12=$D/tls/server.p12
fi
# A random port another run already holds is tried again on a new one: the
# bind refusal is the daemon's first word, before the socket is served.
for _ in 1 2 3 4 5; do
  port=$((20000 + RANDOM % 20000))
  rm -f "$D/bus.db"
  "$bin/agent-busd" -addr 127.0.0.1:$port -socket "$D/bus.sock" -owner "$owner" \
    -db "$D/bus.db" -create -flush-every 0 "${tls[@]}" >"$D/daemon.log" 2>&1 &
  dpid=$!
  for _ in $(seq 1 20); do [ -S "$D/bus.sock" ] && break; kill -0 "$dpid" 2>/dev/null || break; sleep 0.05; done
  kill -0 "$dpid" 2>/dev/null && break
  grep -q "address already in use" "$D/daemon.log" || break
done
cleanup() {
  kill "$dpid" 2>/dev/null; wait "$dpid" 2>/dev/null
  [ "${KEEP:-}" = 1 ] && echo "kept $D" >&2 || rm -rf "$D"
}
trap cleanup EXIT
[ "${TLS:-}" = 1 ] && export AB_TEST_TLS=https://127.0.0.1:$port
# Any answer at all means the bus is serving: the socket file appears before it is.
for _ in $(seq 1 100); do
  [ -n "$(curl -s --max-time 1 --unix-socket "$D/bus.sock" http://unix/status 2>/dev/null)" ] && break
  sleep 0.1
done
token=$(AGENT_BUS_ADDR="$D/user-$(id -un).sock" "$bin/agent-bus-token" "$owner") || {
  echo "no owner token" >&2; cat "$D/daemon.log" >&2; exit 2; }
AB_TEST_ADDR=$D/bus.sock AB_TEST_OWNER=$owner AB_TEST_OWNER_TOKEN=$token AB_TEST_DIR=$D \
  AB_TEST_TCP=http://127.0.0.1:$port "$@"
