#!/bin/bash
# Wave A acceptance — Plans/PoC/TODO.md. Builds, runs a daemon on loopback and
# a private socket, exercises the six verbs, exits non-zero on any failure.
set -u
cd "$(dirname "$0")"
D=$(mktemp -d); trap 'kill ${DPID:-0} 2>/dev/null; rm -rf "$D"' EXIT
export XDG_CACHE_HOME=$D/cache
PORT=${PORT:-7911}

go build -o "$D/agent-busd" ./cmd/agent-busd || exit 1
go build -o "$D/agent-bus"  ./cmd/agent-bus  || exit 1

"$D/agent-busd" -addr 127.0.0.1:$PORT -socket "$D/bus.sock" -token-file "$D/token" >"$D/daemon.log" 2>&1 &
DPID=$!
for _ in $(seq 1 50); do [ -S "$D/bus.sock" ] && break; sleep 0.1; done
TOKEN=$(cat "$D/token")
ab() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
pass=0; fail=0
has() { if echo "$2" | grep -q -- "$3"; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: [$2] lacks [$3]"; fail=$((fail+1)); fi; }

echo "== status on both listeners"
has "unix socket" "$(ab parf@localhost status)" '"services"'
has "loopback tcp" "$(AGENT_BUS_ADDR=http://127.0.0.1:$PORT AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status)" '"up"'

echo "== the two parameters are checked"
has "wrong token" "$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=nope AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status 2>&1)" 'bad token'
has "name needs a realm" "$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf "$D/agent-bus" status 2>&1)" 'X-Agent-Bus-User'

echo "== register and ls"
ab fixer@srv1 register fixer@srv1 --kind agent --descr "fixes things" >/dev/null
ab asker@srv1 register asker@srv1 --kind agent >/dev/null
has "ls shows the description" "$(ab asker@srv1 ls)" 'fixes things'

echo "== send, then reply matched by topic and tag"
ab fixer@srv1 consume --wait 10s > "$D/got.json" & CPID=$!
sleep 0.3
ab asker@srv1 send fixer@srv1 --topic deploy-42 --tag q1 "run the migration?" >/dev/null
wait $CPID
has "body arrived" "$(cat "$D/got.json")" 'run the migration?'
has "sender is named" "$(cat "$D/got.json")" 'asker@srv1'
MSGID=$(sed 's/.*"message_id":"\([^"]*\)".*/\1/' "$D/got.json")
ab fixer@srv1 reply "$MSGID" "yes, running it" >/dev/null
REPLY=$(ab asker@srv1 consume --topic deploy-42 --tag q1 --wait 5s)
has "reply came back" "$REPLY" 'yes, running it'
has "reply keeps the tag" "$REPLY" '"tag":"q1"'

echo "== a message sent while nobody is reading waits"
ab asker@srv1 send fixer@srv1 --topic later --tag t1 "queued while down" >/dev/null
sleep 0.2
has "backlog arrives later" "$(ab fixer@srv1 consume --wait 5s)" 'queued while down'

echo "== one reader per inbox"
ab fixer@srv1 consume --wait 3s >/dev/null 2>&1 & RPID=$!
sleep 0.3
has "second unfiltered read refused" "$(ab fixer@srv1 consume --wait 1s 2>&1)" 'already has a reader'
has "filtered waiter allowed beside it" "$(ab fixer@srv1 consume --topic x --tag y --wait 1s 2>&1; echo no-error)" 'no-error'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null

echo "== unknown receiver"
has "404" "$(ab asker@srv1 send ghost@nowhere hi 2>&1)" 'no such receiver'

echo; echo "passed $pass, failed $fail"; [ $fail -eq 0 ]
