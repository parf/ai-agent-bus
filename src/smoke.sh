#!/bin/bash
# Wave A, B, C and D acceptance — Plans/PoC/TODO.md. Builds, runs a daemon on loopback and
# a private socket, exercises the eleven verbs, exits non-zero on any failure.
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
# The same, for `&`: exec so that $! is the binary. Backgrounding the function
# instead makes $! a subshell, and a signal sent to it leaves the service
# running — which is how a check that a service stops passed without one.
abx() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$1 exec "$D/agent-bus" "${@:2}"; }
pass=0; fail=0
has() { if echo "$2" | grep -q -- "$3"; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: [$2] lacks [$3]"; fail=$((fail+1)); fi; }
ok_exit()   { if [ "$2" -eq 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: exit $2"; fail=$((fail+1)); fi; }
bad_exit()  { if [ "$2" -ne 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: expected failure, got exit 0"; fail=$((fail+1)); fi; }
# "nothing came back" needs its own check: `$(cmd; echo -n nothing)` contains
# the word whatever cmd did, which is how two checks here passed hollow.
is_empty()  { if [ -z "$2" ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: got [$2]"; fail=$((fail+1)); fi; }
code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $2" "http://unix$3"; }
post_code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $2" -d "$4" "http://unix$3"; }

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

echo "== call and ack: a service answers, and says it got the message first"
ab svc@srv1 register svc@srv1 --kind generic --descr "answers calls" >/dev/null
(
  msg=$(ab svc@srv1 consume --wait 10s)
  id=$(printf '%s' "$msg" | sed 's/.*"message_id":"\([^"]*\)".*/\1/')
  [ -n "$id" ] && ab svc@srv1 ack "$id" >/dev/null
  [ -n "$id" ] && ab svc@srv1 reply "$id" "the answer is 42" >/dev/null
) &
SPID=$!
out=$(ab caller@srv1 call svc@srv1 --wait 15s "what is the answer?" 2>"$D/call.err"); rc=$?
wait $SPID 2>/dev/null
ok_exit "call returns" $rc
has "call gets the answer, not the receipt" "$out" 'the answer is 42'
has "the ack was seen and reported" "$(cat "$D/call.err")" 'ack from svc@srv1'
has "a call to nobody fails" "$(ab caller@srv1 call ghost@nowhere --wait 2s hi 2>&1)" 'no such receiver'

echo "== topics: a publisher with no service record, a consumer that was down"
ab owner@srv1 topic create jobs@srv1 --descr "work queue" >/dev/null
has "the topic is in ls" "$(ab owner@srv1 ls --kind topic)" 'jobs@srv1'
ab drive-by@srv1 publish --topic jobs@srv1 "sweep the floor" >/dev/null
has "a consumer that was down still finds it" "$(ab reader@srv1 consume --topic jobs@srv1 --wait 5s)" 'sweep the floor'
ab owner@srv1 topic create news@srv1 --kind pubsub >/dev/null
has "publishing to a pub/sub topic answers MVP" "$(ab drive-by@srv1 publish --topic news@srv1 hello 2>&1)" 'MVP'
# Reading a topic and filtering your own inbox are different inboxes, so the
# check needs a message in each: asserting "nothing came back" passed happily
# with the rule mutated to read the topic in both cases.
ab someone@srv1 send caller@srv1 --topic jobs@srv1 --tag mine "PRIVATE" >/dev/null
ab drive-by@srv1 publish --topic jobs@srv1 "TOPIC" >/dev/null
has "a topic plus a tag filters my own inbox" "$(ab caller@srv1 consume --topic jobs@srv1 --tag mine --wait 3s)" 'PRIVATE'
has "a topic alone reads the topic" "$(ab caller@srv1 consume --topic jobs@srv1 --wait 3s)" 'TOPIC'
out=$(ab caller@srv1 consume --topic jobz@srv1 --wait 1s 2>&1); rc=$?
bad_exit "a mistyped topic name is an error, not a silent filter" $rc
has "and says which name" "$out" 'no such topic: jobz@srv1'

echo "== a call does not damage what it calls from"
ab keeper@srv1 register keeper@srv1 --kind generic --addr host:1234 --descr "KEEP ME" >/dev/null
# the whole record, not a word from it: kind, addr, description, owner and the
# timestamp all change if the caller re-states itself.
record() { ab keeper@srv1 ls | grep -o '{[^}]*"name":"keeper@srv1"[^}]*}'; }
before=$(record)
ab keeper@srv1 call svc@srv1 --wait 1s "nobody is listening" >/dev/null 2>&1
after=$(record)
if [ -n "$before" ] && [ "$before" = "$after" ]; then
  echo "  ok   the caller's own record survives its call"; pass=$((pass+1))
else
  echo "  FAIL the caller's own record survives its call: [$before] became [$after]"; fail=$((fail+1))
fi
ab unheard@srv1 register unheard@srv1 --kind generic >/dev/null
out=$(ab caller@srv1 call unheard@srv1 --wait 5q "typo" 2>&1); rc=$?
bad_exit "a bad --wait is refused" $rc
is_empty "and refused before the message is sent" "$(ab unheard@srv1 consume --wait 1s)"
# The status code, not the body: the accepted envelope echoes the field back,
# so grepping for "receipt" passed with the check for it removed.
has "a third receipt value is refused" \
  "$(post_code caller@srv1 "$TOKEN" /send '{"to":"svc@srv1","receipt":"maybe","body":"x"}')" '400'
has "and the two real ones are not" \
  "$(post_code caller@srv1 "$TOKEN" /send '{"to":"svc@srv1","receipt":"done","re":"0","body":"x"}')" '200'

echo "== a shell script is a service"
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
abx hello@srv1 start hello@srv1 --algo args "$D/hello-world.sh" --descr "greets you" >"$D/start.log" 2>&1 &
HPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'greets you' && break; sleep 0.2; done
has "the script is registered and discoverable" "$(ab asker@srv1 ls)" 'greets you'
has "it answers a call" "$(ab greeter@srv1 call hello@srv1 --wait 15s world)" 'Hello world'
kill $HPID 2>/dev/null; wait $HPID 2>/dev/null

cat > "$D/std.sh" <<'SH'
#!/bin/sh
envelope=$(cat)
case "$envelope" in *payload*) got=yes ;; *) got=no ;; esac
echo "stdin=$got topic=$AGENT_BUS_TOPIC from=$AGENT_BUS_FROM"
SH
chmod +x "$D/std.sh"
abx std@srv1 start std@srv1 --algo std "$D/std.sh" -2 --descr "reads the envelope" >>"$D/start.log" 2>&1 &
SPID2=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'reads the envelope' && break; sleep 0.2; done
has "std gets the envelope on stdin and in the environment" \
  "$(ab greeter@srv1 call std@srv1 --topic t9 --wait 15s payload)" 'stdin=yes topic=t9 from=greeter@srv1'
kill $SPID2 2>/dev/null; wait $SPID2 2>/dev/null

# The JSON form is what docs/08-runner-role.md promises for a service kept in
# a file; `-3` beside it still means three at a time.
echo "{\"name\":\"json@srv1\",\"algo\":\"args\",\"script\":\"$D/hello-world.sh\",\"descr\":\"from a file\"}" \
  | abx launcher@srv1 start -3 >>"$D/start.log" 2>&1 &
JPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'from a file' && break; sleep 0.2; done
has "a service described as JSON on stdin" "$(ab greeter@srv1 call json@srv1 --wait 15s again)" 'Hello again'
kill $JPID 2>/dev/null; wait $JPID 2>/dev/null
has "a script and its arguments must be one quoted word" \
  "$(ab x@srv1 start x@srv1 --algo args ./greet.sh loudly 2>&1)" 'one script'

echo "== a service is the inbox it registered, and stops when told"
printf '#!/bin/sh\nsleep 3\necho "did $1"\n' > "$D/slow.sh"; chmod +x "$D/slow.sh"
abx launcher@srv1 start slow@srv1 --algo args "$D/slow.sh" -1 --descr "slowly" >>"$D/start.log" 2>&1 &
LPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'slowly' && break; sleep 0.2; done
ab caller@srv1 send slow@srv1 --topic w --tag 1 one >/dev/null
ab caller@srv1 send slow@srv1 --topic w --tag 2 two >/dev/null
sleep 1
kill -9 $LPID 2>/dev/null; wait $LPID 2>/dev/null
# The service read slow@srv1, not launcher@srv1, or the first message would
# still be here; and with its one worker busy it left the second on the
# daemon, where killing it cannot lose it.
has "it read the inbox it registered, not the one that launched it" \
  "$(ab slow@srv1 consume --wait 3s)" 'two'
is_empty "and took only what it had a worker for" "$(ab slow@srv1 consume --wait 1s)"

# A service's inbox belongs to its name, not to the process that reads it:
# register it, let nothing run, and the work is still there when something
# with that name turns up. This is the property V1's ephemeral channels did
# not have — a dead channel took its results with it.
ab owner@srv1 register absent@srv1 --kind generic --descr "never started" >/dev/null
ab caller@srv1 send absent@srv1 --topic w --tag 9 "waiting for whoever shows up" >/dev/null
abx launcher@srv1 start absent@srv1 --algo args "$D/hello-world.sh" --descr "turned up late" >>"$D/start.log" 2>&1 &
APID=$!
# The ack comes first and is the sender's answer to "picked up, or lost?" —
# the question V1 needed a delivery journal for.
has "the late service acks the message it found waiting" \
  "$(ab caller@srv1 consume --topic w --tag 9 --wait 15s)" '"receipt":"ack"'
has "and answers it" \
  "$(ab caller@srv1 consume --topic w --tag 9 --wait 15s)" 'Hello waiting for whoever shows up'
kill $APID 2>/dev/null; wait $APID 2>/dev/null

printf '#!/bin/sh\necho "ran $1"\n' > "$D/quick.sh"; chmod +x "$D/quick.sh"
abx launcher@srv1 start stopper@srv1 --algo args "$D/quick.sh" --descr "stops" >>"$D/start.log" 2>&1 &
TPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'stops' && break; sleep 0.2; done
# registered is not the same as waiting: signal it once the daemon says the
# consume is actually outstanding, or the check can pass on a race.
for _ in $(seq 1 50); do ab asker@srv1 status | grep -q '"waiting":[1-9]' && break; sleep 0.2; done
kill -TERM $TPID 2>/dev/null
for _ in $(seq 1 50); do kill -0 $TPID 2>/dev/null || break; sleep 0.1; done
if kill -0 $TPID 2>/dev/null; then
  echo "  FAIL a stopped service leaves its consume"; fail=$((fail+1)); kill -9 $TPID 2>/dev/null
  wait $TPID 2>/dev/null
else
  wait $TPID; ok_exit "a stopped service leaves its consume, exit 0" $?
fi
ab caller@srv1 send stopper@srv1 --topic w --tag 3 "after the stop" >/dev/null
sleep 1
has "and runs nothing after it stopped" "$(ab stopper@srv1 consume --wait 2s)" 'after the stop'

echo "== the last two of the eleven verbs, and the one refusal the daemon owes us"
ab follower@srv1 register follower@srv1 --kind agent >/dev/null
# --follow keeps reading until it is stopped: that is the verb, so the check
# has to be the one to stop it.
abx follower@srv1 consume --follow --wait 5s >"$D/follow.out" 2>&1 &
FPID=$!
sleep 0.3
ab someone@srv1 send follower@srv1 "first" >/dev/null
ab someone@srv1 send follower@srv1 "second" >/dev/null
for _ in $(seq 1 50); do grep -q second "$D/follow.out" && break; sleep 0.2; done
kill $FPID 2>/dev/null; wait $FPID 2>/dev/null
has "--follow keeps reading" "$(cat "$D/follow.out")" 'first'
has "--follow reads the next one too" "$(cat "$D/follow.out")" 'second'
out=$("$D/agent-busd" -addr 0.0.0.0:$((PORT+1)) -socket "$D/public.sock" -token-file "$D/token" 2>&1); rc=$?
bad_exit "the daemon refuses a public interface" $rc
has "and says why" "$out" 'not loopback'

echo "== a service is service@host, or template/instance-name@host"
# The template part is part of the identity, so it has to survive the whole
# trip: registration, the registry listing, a send and a consume. A "/" in a
# query parameter is where this breaks if anything hand-builds a URL.
ab owner@srv1 register code-review/claude@rdvp --kind agent >/dev/null
ab owner@srv1 register code-review/claude-2@rdvp --kind agent >/dev/null
has "a template-prefixed service registers" \
  "$(ab owner@srv1 ls)" '"name":"code-review/claude@rdvp"'
ab sender@srv1 send code-review/claude-2@rdvp "for the second one" >/dev/null
has "and is addressable by its whole name" \
  "$(ab code-review/claude-2@rdvp consume --wait 2s)" 'for the second one'
# Two services from one template are two inboxes, not one queue shared by
# prefix: the first must still be empty.
is_empty "the sibling's inbox is its own, not the template's" \
  "$(ab code-review/claude@rdvp consume --wait 200ms 2>/dev/null)"
has "a name may not hold two slashes" \
  "$(ab owner@srv1 register a/b/c@rdvp 2>&1)" 'a-z 0-9 . _ - + @ only'
# Only the template part is wrong here: the instance name and the realm are
# both fine, so nothing but the template's own check can refuse it.
has "a bad template part is refused on its own" \
  "$(ab owner@srv1 register -nope/claude@rdvp 2>&1)" 'bad template'
# The host is the LAST "@" part, so an instance may be named after the address
# it reads. This is the shape that breaks anything splitting on the first "@".
ab owner@srv1 register mail-sender/parf@comfi.com@host --kind agent >/dev/null
has "an instance name may be an address" \
  "$(ab owner@srv1 ls)" '"name":"mail-sender/parf@comfi.com@host"'
ab sender@srv1 send mail-sender/parf@comfi.com@host "read this one" >/dev/null
has "and routes on the whole name, host split off last" \
  "$(ab mail-sender/parf@comfi.com@host consume --wait 2s)" 'read this one'
has "a dangling at-sign is a typo, not a name" \
  "$(ab owner@srv1 register parf@@host 2>&1)" 'bad name'
ab owner@srv1 register mail-sender/parf+alerts@comfi.com@host --kind agent >/dev/null
has "plus-addressing is a legal instance name" \
  "$(ab owner@srv1 ls)" '"name":"mail-sender/parf+alerts@comfi.com@host"'
has "but the host does not take a plus" \
  "$(ab owner@srv1 register parf@ho+st 2>&1)" 'bad realm'
has "and the realm is a host, not a path" \
  "$(ab owner@srv1 register code-review/claude@rd/vp 2>&1)" 'bad realm'

echo "== configuring a service template produces a configured service"
# The configuration is arbitrary JSON and stays opaque; the one thing that
# matters to the bus is that it never shows up where it should not.
echo '{"model":"opus","depth":3}' | ab owner@srv1 service-template code-review/cfg@rdvp - >/dev/null
has "the configuration comes back as it went in" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp)" '{"model":"opus","depth":3}'
is_empty "a listing never carries it" \
  "$(ab owner@srv1 ls | grep -o '"config":[^,}]*')"
# A send is refused unless the receiver has a record, so this proves the
# record was created — the inbox itself is made lazily by the send either way.
has "configuring creates the service, so it can be sent to" \
  "$(ab sender@srv1 send code-review/cfg@rdvp "it exists" >/dev/null; ab code-review/cfg@rdvp consume --wait 2s)" 'it exists'
has "the service may read its own configuration" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'
has "a stranger may not read it" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp 2>&1)" 'belongs to someone else'
has "and may not overwrite it" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp '{"model":"theirs"}' 2>&1)" 'belongs to someone else'
has "the owner's configuration survived that" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp)" '"model":"opus"'
has "a configuration that is not JSON is refused" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp 'not json' 2>&1)" 'a configuration is JSON'
# The CLI refuses that one before it leaves; the daemon has to refuse it too,
# and the shape that reaches it is a body carrying no configuration at all.
has "and the daemon refuses an empty one on its own" \
  "$(post_code owner@srv1 $TOKEN /configure '{"name":"code-review/cfg@rdvp"}')" '400'
# Reading must work where it is actually used: a script, with no terminal.
has "a read works with no terminal on stdin" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp </dev/null)" '"depth":3'

echo "== a full queue: refuse by default, drop the oldest if asked"
ab owner@srv1 register sink@srv1 --kind generic >/dev/null
ab owner@srv1 register ringy@srv1 --kind generic --overflow ring >/dev/null
has "a record says what a full queue does, and refuses by default" \
  "$(ab owner@srv1 ls | grep -o '{[^}]*"name":"sink@srv1"[^}]*}')" '"overflow":"strict"'
has "an overflow mode that is neither is refused" \
  "$(ab owner@srv1 register bad@srv1 --overflow maybe 2>&1)" 'overflow is strict or ring'
# 1001 into a queue bounded at 1000, twice: strict must refuse the last one,
# ring must swallow it and lose the first.
for i in $(seq 0 1000); do ab flood@srv1 send sink@srv1 "msg-$i" >/dev/null 2>&1; done
out=$(ab flood@srv1 send sink@srv1 "one too many" 2>&1); rc=$?
bad_exit "strict refuses the send rather than lose a message" $rc
has "and names the queue that is full" "$out" 'queue is full: sink@srv1'
has "nothing was dropped" "$(ab asker@srv1 status)" '"dropped":0'
for i in $(seq 0 1000); do ab flood@srv1 send ringy@srv1 "msg-$i" >/dev/null 2>&1; done
has "ring keeps taking, and the oldest is what went" "$(ab ringy@srv1 consume --wait 2s)" 'msg-1"'
has "and the loss is counted, not silent" "$(ab asker@srv1 status)" '"dropped":1'

echo "== --wait is the caller's deadline, not just the daemon's"
# Against a bus that answers everything but stalls the consume: the wait the
# daemon is asked for cannot bound a transfer that never finishes, so the
# deadline has to be on the client's own request.
bun -e "Bun.serve({port:$((PORT+3)),async fetch(r){const u=new URL(r.url);
  if(u.pathname==='/consume'){await Bun.sleep(30000);return new Response('{}');}
  if(u.pathname==='/ls')return new Response('[]');
  return new Response('{}');}})" >/dev/null 2>&1 &
MPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$((PORT+3))/ls" && break; sleep 0.2; done
out=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+3)) AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=impatient@srv1 \
      timeout 5 "$D/agent-bus" call slow@srv1 --wait 500ms "are you there?" 2>&1); rc=$?
kill $MPID 2>/dev/null; wait $MPID 2>/dev/null
if [ "$rc" -eq 124 ]; then
  echo "  FAIL a stalled consume ends at --wait: it ran past the deadline"; fail=$((fail+1))
else
  echo "  ok   a stalled consume ends at --wait"; pass=$((pass+1))
fi
has "and says the message was accepted" "$out" 'do not resend'

echo "== the token an SSH forced command hands out"
# Not "contains the token": a debug line printed before it passed that, and
# $(ssh … static-token) would then hold a credential that does not work. The
# proof is the captured value authenticating against the daemon.
issued=$(AGENT_BUS_TOKEN_FILE=$D/token ./static-token); rc=$?
ok_exit "static-token succeeds" $rc
if [ "$issued" = "$TOKEN" ]; then echo "  ok   it prints the token and nothing else"; pass=$((pass+1));
else echo "  FAIL it prints the token and nothing else: [$issued]"; fail=$((fail+1)); fi
has "the token it hands out authenticates" \
  "$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$issued AGENT_BUS_NAME=over-ssh@srv1 "$D/agent-bus" status)" '"up"'
printf '   \n' > "$D/blank-token"
out=$(AGENT_BUS_TOKEN_FILE=$D/blank-token ./static-token 2>&1); rc=$?
bad_exit "an empty token file is refused, not handed out as an empty token" $rc
out=$(SSH_ORIGINAL_COMMAND='cat /etc/passwd' AGENT_BUS_TOKEN_FILE=$D/token ./static-token 2>&1); rc=$?
bad_exit "it refuses any other command" $rc
has "and says why" "$out" 'one command'
out=$(AGENT_BUS_TOKEN_FILE=$D/absent ./static-token 2>&1); rc=$?
bad_exit "a missing token file is an error, not an empty token" $rc

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

  out=$(cd mcp && timeout 120 bun run smoke-codex.ts 2>&1)
  rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "codex adapter smoke" $rc
fi

echo; echo "passed $pass, failed $fail"; [ $fail -eq 0 ]
