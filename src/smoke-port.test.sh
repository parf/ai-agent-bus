#!/bin/bash
set -eu
cd "$(dirname "$0")"
source ./smoke-port.sh
# The cases run inside a block this test claims the real way, held until it
# exits, so two smoke runs at once test on different ports; their locks are private.
unset PORT
smoke_port 1000 || { echo "no block for the test itself"; exit 1; }
own=$PORT
unset PORT
locks=$(mktemp -d)
listener=""
trap 'rm -rf "$locks"; [ -z "$listener" ] || kill "$listener" 2>/dev/null' EXIT
export SMOKE_PORT_LOCKS=$locks SMOKE_PORT_FIRST=$own SMOKE_PORT_STEP=100 SMOKE_PORT_LAST=$((own + 1000))
claim() { (smoke_port 90 && echo "$PORT"); }

got=$(PORT=8011; smoke_port 90 && echo "$PORT")
[ "$got" = 8011 ] || { echo "an explicit PORT became $got"; exit 1; }
[ -z "$(ls -A "$locks")" ] || { echo "an explicit PORT took a lock"; exit 1; }

# Held by one run, the block goes to the next run; let go, it is free again.
(smoke_port 90 && echo "$PORT" >"$locks/held" && sleep 1) &
holder=$!
for _ in $(seq 1 50); do [ -s "$locks/held" ] && break; sleep .05; done
held=$(cat "$locks/held")
second=$(claim)
[ "$second" != "$held" ] || { echo "two runs at once both took $held"; exit 1; }
wait "$holder"
again=$(claim)
[ "$again" = "$held" ] || { echo "after the first run ended its block $held went unclaimed; got $again"; exit 1; }

# A block where another program listens is skipped. That covers a pinned run
# only while its daemon is up, which is why real blocks end below the pinned
# PORTs (smoke-port.sh).
busy=$((held + 7))
python3 -c 'import socket,sys,time; s=socket.socket(); s.bind(("127.0.0.1", int(sys.argv[1]))); s.listen(); time.sleep(30)' "$busy" &
listener=$!
for _ in $(seq 1 50); do ss -Htln "sport = :$busy" | grep -q . && break; sleep .05; done
ss -Htln "sport = :$busy" | grep -q . || { echo "the fixture could not listen on $busy"; exit 1; }
past=$(claim)
[ "$past" -gt "$held" ] || { echo "the block holding a listener on $busy was claimed: $past"; exit 1; }

# The default range ends below the agents' pinned PORTs, and running out is refused.
bound=$(
  unset SMOKE_PORT_FIRST SMOKE_PORT_STEP SMOKE_PORT_LAST
  export SMOKE_PORT_LOCKS=$locks/defaults
  n=0
  while unset PORT && smoke_port 780 2>/dev/null; do
    [ $((PORT + 780)) -le 21000 ] || { echo "block $PORT reaches past 21000"; exit; }
    n=$((n + 1))
    [ "$n" -le 11 ] || { echo "more than eleven blocks under 21000"; exit; }
  done
  echo "ok $n"
)
case $bound in "ok "[1-9]*) ;; *) echo "default range: ${bound:-no answer}"; exit 1 ;; esac
echo "explicit PORT kept, concurrent runs on different blocks, busy blocks skipped, default range bounded: green"
