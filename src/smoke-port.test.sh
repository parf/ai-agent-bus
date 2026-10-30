#!/bin/bash
set -eu
cd "$(dirname "$0")"
source ./smoke-port.sh
locks=$(mktemp -d)
listener=""
trap 'rm -rf "$locks"; [ -z "$listener" ] || kill "$listener" 2>/dev/null' EXIT
export SMOKE_PORT_LOCKS=$locks SMOKE_PORT_FIRST=${SMOKE_PORT_TEST_FIRST:-41000} SMOKE_PORT_STEP=100 SMOKE_PORT_LAST=48000
claim() { (unset PORT; smoke_port 90 && echo "$PORT"); }

got=$(PORT=8011; smoke_port 90 && echo "$PORT")
[ "$got" = 8011 ] || { echo "an explicit PORT became $got"; exit 1; }
[ -z "$(ls -A "$locks")" ] || { echo "an explicit PORT took a lock"; exit 1; }

# Held by one run, the block goes to the next run; let go, it is free again.
(unset PORT; smoke_port 90 && echo "$PORT" >"$locks/held" && sleep 1) &
holder=$!
for _ in $(seq 1 50); do [ -s "$locks/held" ] && break; sleep .05; done
held=$(cat "$locks/held")
second=$(claim)
[ "$second" != "$held" ] || { echo "two runs at once both took $held"; exit 1; }
wait "$holder"
again=$(claim)
[ "$again" = "$held" ] || { echo "after the first run ended its block $held went unclaimed; got $again"; exit 1; }

# A block where another program listens is skipped.
python3 -c 'import socket,sys,time; s=socket.socket(); s.bind(("127.0.0.1", int(sys.argv[1]))); s.listen(); time.sleep(30)' $((held + 7)) &
listener=$!
for _ in $(seq 1 50); do ss -Htln "sport = :$((held + 7))" | grep -q . && break; sleep .05; done
past=$(claim)
[ "$past" -gt "$held" ] || { echo "the block holding a listener on $((held + 7)) was claimed: $past"; exit 1; }
echo "explicit PORT kept, concurrent runs on different blocks, busy blocks skipped: green"
