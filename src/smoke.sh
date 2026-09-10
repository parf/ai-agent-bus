#!/bin/bash
# Wave A acceptance — Plans/PoC/TODO.md. Builds, runs a daemon on loopback and
# a private socket, exercises the six verbs, exits non-zero on any failure.
set -u
cd "$(dirname "$0")"
D=$(mktemp -d); DPID=""
cleanup() { [ -n "$DPID" ] && kill "$DPID" 2>/dev/null; rm -rf "$D"; }
trap cleanup EXIT
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
ok_exit()   { if [ "$2" -eq 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: exit $2"; fail=$((fail+1)); fi; }
bad_exit()  { if [ "$2" -ne 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: expected failure, got exit 0"; fail=$((fail+1)); fi; }
code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $2" "http://unix$3"; }

echo "== status on both listeners"
has "unix socket" "$(ab parf@localhost status)" '"services"'
has "loopback tcp" "$(AGENT_BUS_ADDR=http://127.0.0.1:$PORT AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status)" '"up"'

echo "== the two parameters are checked"
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=nope AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status 2>&1); rc=$?
has "wrong token says so" "$out" 'bad token'; bad_exit "wrong token exits non-zero" $rc
has "401 for a wrong token" "$(code parf@localhost nope /status)" '401'
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf "$D/agent-bus" status 2>&1); rc=$?
bad_exit "a name without a realm is refused" $rc
has "401 for a realm-less name" "$(code parf "$TOKEN" /status)" '401'

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

echo "== a filtered waiter is served ahead of the unfiltered reader"
ab fixer@srv1 consume --wait 6s > "$D/reader.json" 2>/dev/null & UPID=$!
sleep 0.3
ab fixer@srv1 consume --topic prio --tag p1 --wait 6s > "$D/waiter.json" 2>/dev/null & FPID=$!
sleep 0.3
ab asker@srv1 send fixer@srv1 --topic prio --tag p1 "for the waiter" >/dev/null
wait $FPID 2>/dev/null
has "the filtered waiter got it" "$(cat "$D/waiter.json")" 'for the waiter'
if grep -q 'for the waiter' "$D/reader.json" 2>/dev/null; then
  echo "  FAIL the unfiltered reader stole it"; fail=$((fail+1))
else
  echo "  ok   the unfiltered reader did not steal it"; pass=$((pass+1))
fi
ab asker@srv1 send fixer@srv1 --topic other --tag x "for the reader" >/dev/null
wait $UPID 2>/dev/null
has "the unfiltered reader got the other one" "$(cat "$D/reader.json")" 'for the reader'

echo "== a message sent while nobody is reading waits"
ab asker@srv1 send fixer@srv1 --topic later --tag t1 "queued while down" >/dev/null
sleep 0.2
has "backlog arrives later" "$(ab fixer@srv1 consume --wait 5s)" 'queued while down'

echo "== one reader per inbox"
ab fixer@srv1 consume --wait 2s >/dev/null 2>&1 & RPID=$!
sleep 0.3
has "second unfiltered read refused" "$(ab fixer@srv1 consume --wait 1s 2>&1)" 'already has a reader'
out=$(ab fixer@srv1 consume --topic x --tag y --wait 1s 2>&1); rc=$?
ok_exit "filtered waiter allowed beside it" $rc
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
sleep 2   # the backgrounded client outlives its subshell; let its poll expire

echo "== unknown receiver"
out=$(ab asker@srv1 send ghost@nowhere hi 2>&1); rc=$?
has "says no such receiver" "$out" 'no such receiver'; bad_exit "and exits non-zero" $rc

echo "== a name that is not a name"
bad_exit "register without a realm" "$(ab asker@srv1 register no-realm >/dev/null 2>&1; echo $?)"

echo "== one name, however it is spelled: trim, lower-case, ASCII"
ab pad@srv1 register '  PAD@Srv1  ' --kind agent >/dev/null
has "ls shows the canonical form" "$(ab asker@srv1 ls)" '"name":"pad@srv1"'
ab asker@srv1 send ' Pad@SRV1 ' --topic pad --tag p "padded name" >/dev/null
has "a padded, upper-case send reaches it" "$(ab pad@srv1 consume --topic pad --tag p --wait 3s)" 'padded name'
bad_exit "a non-ASCII name is refused" "$(ab asker@srv1 register 'pärf@srv1' >/dev/null 2>&1; echo $?)"

echo "== the MCP face"
# bun is not optional: the MCP face and both push modes are the PoC
# (docs/12-stages.md#poc), so a host without it fails rather than passing green.
if ! command -v bun >/dev/null 2>&1; then
  echo "  FAIL bun is not installed; the MCP face cannot be checked"
  fail=$((fail + 1))
else
  # each harness runs its own peer in-process, so there is no start-order race
  out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
        AGENT_BUS_NAME=mcp.session@srv1 SMOKE_PEER=peer@srv1 bun run smoke.ts 2>&1)
  rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "mcp smoke" $rc

  out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
        AGENT_BUS_NAME=pusher@srv1 PUSH_NAME=push.session@srv1 bun run smoke-push.ts 2>&1)
  rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "claude push smoke" $rc
fi

echo; echo "passed $pass, failed $fail"; [ $fail -eq 0 ]
