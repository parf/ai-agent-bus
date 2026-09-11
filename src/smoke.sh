#!/bin/bash
# Acceptance for the built stages. Builds, runs a daemon on loopback and a
# private socket, exercises the verbs, exits non-zero on any failure.
#
#   ./smoke.sh          the fast run: everything that costs under a second
#   ./smoke.sh --slow    all of it, including the race detector
#
# The fast run is for the edit-run loop. **A change is measured against
# --slow**, and so is every mutation: a check that did not run caught
# nothing (Plans/PoC/README.md#mutation-first-then-belief).
set -u
cd "$(dirname "$0")"
D=$(mktemp -d); DPID=""
cleanup() { [ -n "$DPID" ] && kill "$DPID" 2>/dev/null; rm -rf "$D"; }
trap cleanup EXIT
# The CLI keeps its reply context under XDG_CACHE_HOME, and this run wants a
# private one. Go's build cache lives there too by default, so redirecting it
# made every run recompile the world — 2.7s of "go vet" that is 0.1s warm,
# and four times that under -race. Keep Go's cache where it was.
export GOCACHE=${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build}
export XDG_CACHE_HOME=$D/cache
PORT=${PORT:-7911}

go build -o "$D/agent-busd" ./cmd/agent-busd || exit 1
go build -o "$D/agent-bus"  ./cmd/agent-bus  || exit 1

# The daemon belongs to a principal, and that is who may hand out a
# credential for a name nobody owns yet. Stated rather than taken from the
# account running the suite, so the checks read the same everywhere.
OWNER=parf@localhost
"$D/agent-busd" -addr 127.0.0.1:$PORT -socket "$D/bus.sock" -token-file "$D/token" -owner "$OWNER" -dump-file "$D/dump.json" -dump-every 0 >"$D/daemon.log" 2>&1 &
DPID=$!
for _ in $(seq 1 50); do [ -S "$D/bus.sock" ] && break; sleep 0.1; done
# One line per principal, `name token`: the owner's is what the daemon wrote
# at start, and every other name gets one from it on first use.
TOKEN=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/token")
tok() {
  local f="$D/tok.$(printf '%s' "$1" | tr '/@.' '___')"
  [ -s "$f" ] || AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER     "$D/agent-bus" token "$1" >"$f" 2>/dev/null
  cat "$f" 2>/dev/null
}
ab() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
# The same, for `&`: exec so that $! is the binary. Backgrounding the function
# instead makes $! a subshell, and a signal sent to it leaves the service
# running — which is how a check that a service stops passed without one.
abx() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 exec "$D/agent-bus" "${@:2}"; }
pass=0; fail=0; skipped=0
# Anything that takes more than a second is opt-in: the default run is the
# one a person waits for, and the full run is what a change is measured
# against. `SLOW=1` (or --slow) runs everything, and the mutation harness
# always does — a mutant that survives because its check was skipped is the
# worst kind of green (Plans/PoC/README.md#mutation-first-then-belief).
SLOW=${SLOW:-0}
[ "${1:-}" = "--slow" ] && SLOW=1
slow() { [ "$SLOW" = 1 ]; }
# Counters are cumulative since the daemon started, so a check on one has to
# be a delta. Asserting the absolute value worked only while this section
# happened to run first, and broke the moment another one dropped a message.
count() { ab parf@localhost status | sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p"; }
delta() { # label expected before after
  if [ "$(( $4 - $3 ))" -eq "$2" ]; then echo "  ok   $1"; pass=$((pass+1));
  else echo "  FAIL $1: expected +$2, got $3 -> $4"; fail=$((fail+1)); fi
}
# Per-service counters ride on the record a listing returns, not on the
# daemon-wide status, and omitempty means an absent field reads as zero.
svc() { local n; n=$(ab parf@localhost ls "$1" | sed -n "s/.*\"$2\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }
# Section timing, so "slow" is a measurement and not a hunch.
sec_name=""; sec_t0=0
sec() {
  local now; now=$(date +%s%3N)
  [ -n "$sec_name" ] && printf '%6s %s\n' "$((now-sec_t0))" "$sec_name" >> "$D/timing"
  sec_name=$1; sec_t0=$now
  echo "== $1"
}
has() { if echo "$2" | grep -q -- "$3"; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: [$2] lacks [$3]"; fail=$((fail+1)); fi; }
ok_exit()   { if [ "$2" -eq 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: exit $2"; fail=$((fail+1)); fi; }
bad_exit()  { if [ "$2" -ne 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: expected failure, got exit 0"; fail=$((fail+1)); fi; }
# "nothing came back" needs its own check: `$(cmd; echo -n nothing)` contains
# the word whatever cmd did, which is how two checks here passed hollow.
is_empty()  { if [ -z "$2" ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: got [$2]"; fail=$((fail+1)); fi; }
code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $2" "http://unix$3"; }
post_body() { curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $(tok "$1")" -d "$3" "http://unix$2"; }
post_code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: $2" -d "$4" "http://unix$3"; }

sec "the Go checks"
# Run here, not only by hand: this script is what a change is measured
# against, and a mutation of anything the unit tests cover was invisible to
# it while they lived outside. CLAUDE.md asks for all three to be green.
checks=("vet:go vet ./..." "test:go test ./...")
slow && checks=("vet:go vet ./..." "race:go test -race ./...")
for c in "${checks[@]}"; do
  out=$(eval "${c#*:}" 2>&1); rc=$?
  ok_exit "go ${c%%:*}" $rc
  [ $rc -eq 0 ] || echo "$out" | tail -15 | sed 's/^/    /'
done

# A layer rule nobody checks is a comment. These are the two directions that
# matter: nothing inward may name an adapter, and the adapter must actually
# be reached from somewhere, or the first check passes because the seam is
# empty. See docs/10-modules.md#the-rule.
sec "the layers hold"
is_empty "core never imports an adapter" \
  "$(go list -deps ./internal/core ./internal/auth ./internal/ports | grep -E 'internal/(store|dump)')"
is_empty "nor does a face" \
  "$(go list -deps ./internal/api ./cmd/agent-bus | grep -E 'internal/(store|dump)')"
is_empty "and a port names no outside world of its own" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/ports 2>&1 | grep -E '^(os|net|net/http|os/exec|database/sql)$')"
has "while the process that assembles them holds the ones that persist" \
  "$(go list -deps ./cmd/agent-busd | grep -E 'internal/(store|dump)' | tr '\n' ' ')" 'internal/dump/jsonfile .*internal/store/file'
has "and core is what asks for it" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/auth 2>&1)" 'internal/ports'

sec "status on both listeners"
has "unix socket" "$(ab parf@localhost status)" '"services"'
has "loopback tcp" "$(AGENT_BUS_ADDR=http://127.0.0.1:$PORT AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status)" '"up"'

sec "the two parameters are checked"
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=nope AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status 2>&1); rc=$?
has "wrong token says so" "$out" 'bad token'; bad_exit "wrong token exits non-zero" $rc
has "401 for a wrong token" "$(code parf@localhost nope /status)" '401'
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf "$D/agent-bus" status 2>&1); rc=$?
bad_exit "a name without a realm is refused" $rc
has "401 for a realm-less name" "$(code parf "$TOKEN" /status)" '401'

sec "your own socket supplies both parameters"
# Nothing to set up locally: the daemon knows the account at the other end
# from which socket it arrived on. See docs/02-access.md#local-socket.
ACCOUNT=$(id -un)
MINE=$D/user-$ACCOUNT.sock
has "the daemon opened one for the account it runs as" "$([ -S "$MINE" ] && echo yes)" 'yes'
has "and it is that account's alone" "$(stat -c %a "$MINE")" '^600$'
has "a call with no name and no token is served" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" status)" '"up"'
has "and the daemon says whose call it was" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" status)" "\"you\":\"$OWNER\""
has "a write lands under the socket's principal, not under nobody" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" register sock-made@srv1 --descr "from the socket" >/dev/null; ab nobody2@srv1 ls sock-made@srv1)" "\"owner\":\"$OWNER\""
# The socket hides the two parameters; it does not replace them. Stating
# somebody else's name on it is the same forgery as stating it remotely.
has "claiming another name on it is refused" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$MINE" -H "X-Agent-Bus-User: bob@srv1" "http://unix/status")" '403'
has "and the refusal says whose socket it is" \
  "$(curl -s --unix-socket "$MINE" -H "X-Agent-Bus-User: bob@srv1" "http://unix/status")" "this socket is $OWNER's"
has "while stating your own name on it is fine" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$MINE" -H "X-Agent-Bus-User: $OWNER" "http://unix/status")" '200'
# One daemon, many people. A second mapped account gets a socket of its own
# and is a different principal on it — which is the whole point of the
# arrangement, and is not provable with one socket.
if id -u nobody >/dev/null 2>&1; then
  mkdir -p "$D/multi"
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+6)) -socket "$D/multi/bus.sock" -token-file "$D/token" \
    -owner "$OWNER" -user "nobody=nemo@srv1" -dump-file "$D/multi/dump.json" -dump-every 0 >"$D/multi.log" 2>&1 &
  MPID2=$!
  for _ in $(seq 1 50); do [ -S "$D/multi/user-nobody.sock" ] && break; sleep 0.1; done
  has "a second account gets a socket of its own" "$([ -S "$D/multi/user-nobody.sock" ] && echo yes)" 'yes'
  has "and is a different principal on it" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" "http://unix/status")" '"you":"nemo@srv1"'
  has "while the owner's socket in the same directory is still the owner" \
    "$(curl -s --unix-socket "$D/multi/user-$ACCOUNT.sock" "http://unix/status")" "\"you\":\"$OWNER\""
  has "and neither may speak as the other" \
    "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/multi/user-nobody.sock" -H "X-Agent-Bus-User: $OWNER" "http://unix/status")" '403'
  # Unprivileged, the chown cannot land, and the daemon has to say so rather
  # than leave a socket that looks like somebody else's and is not.
  has "it says out loud when it could not hand the socket over" "$(cat "$D/multi.log")" 'CAP_CHOWN'
  kill $MPID2 2>/dev/null; wait $MPID2 2>/dev/null
else
  skipped=$((skipped+1))
fi
has "the shared socket still wants both" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/bus.sock "$D/agent-bus" status 2>&1)" 'AGENT_BUS_TOKEN'
has "and the directory is walk-through only, not readable" "$(stat -c %a "$D")" '^711$'

sec "a name is checked against the credential it arrived with"
# The forgery to catch is a whole request made under the wrong name, not a
# `from` field rewritten inside one: the face already did the second and it
# stopped nothing. See docs/02-access.md#two-parameters.
alice=$(tok alice@srv1); bob=$(tok bob@srv1)
has "a token that backs one name is refused under another" \
  "$(code alice@srv1 "$bob" /status)" '403'
has "and the refusal names whose token it is" \
  "$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: alice@srv1" -H "X-Agent-Bus-Token: $bob" "http://unix/status")" 'belongs to bob@srv1'
has "which is a different answer from a token nobody holds" \
  "$(code alice@srv1 not-a-token /status)" '401'
has "the check guards a consume too" "$(code alice@srv1 "$bob" "/consume?wait=0s")" '403'
has "and a send" \
  "$(post_code alice@srv1 "$bob" /send '{"to":"bob@srv1","body":"x"}')" '403'
has "and a registration" \
  "$(post_code alice@srv1 "$bob" /register '{"name":"alice@srv1"}')" '403'
has "while the right pair is served" "$(code alice@srv1 "$alice" /status)" '200'

sec "who may ask for whose credential"
has "a principal may get its own" \
  "$(post_code alice@srv1 "$alice" /token '{"name":"alice@srv1"}')" '200'
has "but not somebody else's" \
  "$(post_code alice@srv1 "$alice" /token '{"name":"bob@srv1"}')" '403'
ab alice@srv1 register alice-svc@srv1 --descr "hers" >/dev/null
has "and may get one for a service it owns" \
  "$(post_code alice@srv1 "$alice" /token '{"name":"alice-svc@srv1"}')" '200'
has "while somebody else may not" \
  "$(post_code bob@srv1 "$bob" /token '{"name":"alice-svc@srv1"}')" '403'
has "the daemon's owner may ask for any name" \
  "$(post_code $OWNER "$TOKEN" /token '{"name":"nobody-owns-this@srv1"}')" '200'
again=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$alice AGENT_BUS_NAME=alice@srv1 "$D/agent-bus" token alice@srv1 2>&1)
if [ "$again" = "$alice" ]; then echo "  ok   asking twice is a read, not a rotation"; pass=$((pass+1));
else echo "  FAIL asking twice is a read, not a rotation: [$again]"; fail=$((fail+1)); fi
# Rotation: two are accepted, the one before them is not. A refresh that
# stranded traffic already queued under the old token would be worse than
# no rotation at all. See docs/02-access.md#token-lifetime.
ab owner@srv1 register rotor@srv1 >/dev/null
first=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token rotor@srv1)
second=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token rotor@srv1 --rotate)
third=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token rotor@srv1 --rotate)
if [ -n "$second" ] && [ "$second" != "$first" ] && [ "$third" != "$second" ]; then
  echo "  ok   --rotate hands out a new token each time"; pass=$((pass+1))
else
  echo "  FAIL --rotate hands out a new token each time: [$first/$second/$third]"; fail=$((fail+1))
fi
has "the current one works" "$(code rotor@srv1 "$third" /status)" '200'
has "and the one before it, so queued traffic is not stranded" "$(code rotor@srv1 "$second" /status)" '200'
has "but the one before that is refused" "$(code rotor@srv1 "$first" /status)" '401'
# Every principal is in the file, not just the owner's: a second daemon
# reading it hands the same credentials back, which is what "tokens are
# durable" has to mean once there is more than one.
cp "$D/token" "$D/token2"
mkdir -p "$D/r2"
"$D/agent-busd" -addr 127.0.0.1:$((PORT+4)) -socket "$D/r2/bus.sock" -token-file "$D/token2" -owner "$OWNER" -dump-file "$D/r2/dump.json" -dump-every 0 >"$D/daemon2.log" 2>&1 &
RPID=$!
for _ in $(seq 1 50); do [ -S "$D/r2/bus.sock" ] && break; sleep 0.1; done
has "a restart keeps both of a rotated principal's tokens" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r2/bus.sock" -H "X-Agent-Bus-User: rotor@srv1" -H "X-Agent-Bus-Token: $second" "http://unix/status")" '200'
has "and still refuses the one it dropped" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r2/bus.sock" -H "X-Agent-Bus-User: rotor@srv1" -H "X-Agent-Bus-Token: $first" "http://unix/status")" '401'
has "a restart keeps every principal, not only the owner's" \
  "$(AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$alice AGENT_BUS_NAME=alice@srv1 "$D/agent-bus" status)" '"up"'
has "and still refuses the wrong name with it" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r2/bus.sock" -H "X-Agent-Bus-User: bob@srv1" -H "X-Agent-Bus-Token: $alice" "http://unix/status")" '403'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
# What the store keeps, it keeps to itself. See docs/09-setup.md#storage.
has "the credential file is that account's alone" "$(stat -c %a "$D/token")" '^600$'
# A file holding one bare token is what the PoC wrote, and a host that
# upgrades must not lose the daemon's own credential to the new format.
mkdir -p "$D/old" && printf 'poc-era-bare-token\n' > "$D/old/token"
"$D/agent-busd" -addr 127.0.0.1:$((PORT+7)) -socket "$D/old/bus.sock" -token-file "$D/old/token" -owner "$OWNER" -dump-file "$D/old/dump.json" -dump-every 0 >"$D/daemon4.log" 2>&1 &
BPID=$!
for _ in $(seq 1 50); do [ -S "$D/old/bus.sock" ] && break; sleep 0.1; done
oldsock() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/old/bus.sock" -H "X-Agent-Bus-User: $1" -H "X-Agent-Bus-Token: poc-era-bare-token" "http://unix/status"; }
has "a bare token from the PoC is read as the owner's" "$(oldsock "$OWNER")" '200'
has "and as nobody else's" "$(oldsock bob@srv1)" '403'
kill $BPID 2>/dev/null; wait $BPID 2>/dev/null
# A credential that could not be written down is one a restart forgets, so
# it is not handed out either: the daemon says so instead.
mkdir -p "$D/ro" && cp "$D/token" "$D/ro/token"
mkdir -p "$D/ro-run"
"$D/agent-busd" -addr 127.0.0.1:$((PORT+5)) -socket "$D/ro-run/bus.sock" -token-file "$D/ro/token" -owner "$OWNER" -dump-file "$D/ro-run/dump.json" -dump-every 0 >"$D/daemon3.log" 2>&1 &
OPID=$!
for _ in $(seq 1 50); do [ -S "$D/ro-run/bus.sock" ] && break; sleep 0.1; done
chmod 0500 "$D/ro"
out=$(AGENT_BUS_ADDR=$D/ro-run/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token unsaveable@srv1 2>&1); rc=$?
bad_exit "a credential the store could not keep is not handed out" $rc
is_empty "and nothing that looks like one is printed" "$(printf '%s' "$out" | grep -o '^[0-9a-f]\{48\}$')"
chmod 0700 "$D/ro"
has "while the same ask succeeds once the store can be written" \
  "$(AGENT_BUS_ADDR=$D/ro-run/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token unsaveable@srv1)" '^[0-9a-f]\{48\}$'
kill $OPID 2>/dev/null; wait $OPID 2>/dev/null
# `start` runs until it is stopped, so this check leans on the refusal to end
# it. Under a mutant that allows it, it ran until the harness's own timeout
# and took the whole batch with it — hence the bound, and hence 124 counting
# as a failure of the check rather than the refusal it was looking for.
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$bob AGENT_BUS_NAME=bob@srv1 timeout 5 "$D/agent-bus" start alice-svc@srv1 --algo args /bin/echo 2>&1); rc=$?
if [ "$rc" -ne 0 ] && [ "$rc" -ne 124 ]; then
  echo "  ok   and starting a service you do not own is refused"; pass=$((pass+1))
else
  echo "  FAIL and starting a service you do not own is refused: exit $rc"; fail=$((fail+1))
fi
has "because the record is not his to take over" "$out" 'belongs to someone else'

sec "a record belongs to whoever published it"
# Publishing is open; changing is not. See docs/01-identity.md#ownership.
ab owner@srv1 register owned@srv1 --descr "mine" --addr first:1 >/dev/null
out=$(ab thief2@srv1 register owned@srv1 --descr "stolen" --addr second:2 2>&1); rc=$?
bad_exit "somebody else cannot re-register it" $rc
has "and is told whose it is" "$out" "owned@srv1 is owner@srv1's"
has "the address it stated did not land" "$(ab nobody2@srv1 ls owned@srv1)" 'first:1'
has "nor the description" "$(ab nobody2@srv1 ls owned@srv1)" '"descr":"mine"'
has "its owner may still change it" \
  "$(ab owner@srv1 register owned@srv1 --descr "mine" --addr third:3 >/dev/null; ab nobody2@srv1 ls owned@srv1)" 'third:3'
has "and the record itself may refresh its own, as a service does on every start" \
  "$(ab owned@srv1 register owned@srv1 --descr "self" --addr third:3 >/dev/null; ab nobody2@srv1 ls owned@srv1)" '"descr":"self"'
has "while publishing a name nobody holds stays open to anyone" \
  "$(ab stranger2@srv1 register brand-new@srv1 --descr "open" >/dev/null; ab nobody2@srv1 ls brand-new@srv1)" '"descr":"open"'

sec "register and ls"
ab fixer@srv1 register fixer@srv1 --kind agent --descr "fixes things" >/dev/null
ab asker@srv1 register asker@srv1 --kind agent >/dev/null
has "ls shows the description" "$(ab asker@srv1 ls)" 'fixes things'

sec "send, then reply matched by topic and tag"
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

sec "a filtered waiter is served ahead of the unfiltered reader"
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

sec "a message sent while nobody is reading waits"
ab asker@srv1 send fixer@srv1 --topic later --tag t1 "queued while down" >/dev/null
sleep 0.2
has "backlog arrives later" "$(ab fixer@srv1 consume --wait 5s)" 'queued while down'

if slow; then
  sec "one reader per inbox"
  ab fixer@srv1 consume --wait 2s >/dev/null 2>&1 & RPID=$!
  sleep 0.3
  has "second unfiltered read refused" "$(ab fixer@srv1 consume --wait 1s 2>&1)" 'already has a reader'
  out=$(ab fixer@srv1 consume --topic x --tag y --wait 1s 2>&1); rc=$?
  ok_exit "filtered waiter allowed beside it" $rc
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
  sleep 2   # the backgrounded client outlives its subshell; let its poll expire

else skipped=$((skipped+1)); fi
sec "unknown receiver"
out=$(ab asker@srv1 send ghost@nowhere hi 2>&1); rc=$?
has "says no such receiver" "$out" 'no such receiver'; bad_exit "and exits non-zero" $rc

sec "a name that is not a name"
bad_exit "register without a realm" "$(ab asker@srv1 register no-realm >/dev/null 2>&1; echo $?)"

sec "one name, however it is spelled: trim, lower-case, ASCII"
ab pad@srv1 register '  PAD@Srv1  ' --kind agent >/dev/null
has "ls shows the canonical form" "$(ab asker@srv1 ls)" '"name":"pad@srv1"'
ab asker@srv1 send ' Pad@SRV1 ' --topic pad --tag p "padded name" >/dev/null
has "a padded, upper-case send reaches it" "$(ab pad@srv1 consume --topic pad --tag p --wait 3s)" 'padded name'
bad_exit "a non-ASCII name is refused" "$(ab asker@srv1 register 'pärf@srv1' >/dev/null 2>&1; echo $?)"

sec "call and ack: a service answers, and says it got the message first"
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

sec "topics: a publisher with no service record, a consumer that was down"
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

if slow; then
  sec "a call does not damage what it calls from"
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
    "$(post_code caller@srv1 "$(tok caller@srv1)" /send '{"to":"svc@srv1","receipt":"maybe","body":"x"}')" '400'
  has "and the two real ones are not" \
    "$(post_code caller@srv1 "$(tok caller@srv1)" /send '{"to":"svc@srv1","receipt":"done","re":"0","body":"x"}')" '200'

else skipped=$((skipped+1)); fi
sec "a shell script is a service"
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

if slow; then
  sec "reply-to: the answer goes where the request said, and a dead route is refused now"
  ab owner@srv1 register worker@srv1 --kind generic --descr "does work" >/dev/null
  ab owner@srv1 register third@srv1 --kind agent >/dev/null
  ab caller@srv1 send worker@srv1 --topic rt --tag 1 --reply-to third@srv1 "work for someone else" >/dev/null
  wid=$(ab worker@srv1 consume --wait 5s | sed -n 's/.*"message_id":"\([^"]*\)".*/\1/p')
  ab worker@srv1 reply "$wid" "here is the answer" >/dev/null
  has "the third party gets the answer" \
    "$(ab third@srv1 consume --topic rt --tag 1 --wait 5s)" 'here is the answer'
  # The other half: the answer must NOT also go back to the caller. Checking
  # only that the third party received it passes with the reply broadcast.
  is_empty "and the caller does not" "$(ab caller@srv1 consume --topic rt --tag 1 --wait 1s)"

  # Refused when the request is accepted, not discovered when the answer
  # bounces — the whole point of the rule.
  out=$(ab caller@srv1 send worker@srv1 --topic rt --tag 2 --reply-to ghost@nowhere "nobody can hear the answer" 2>&1); rc=$?
  has "a dead reply address is refused at accept" "$out" 'no such reply address'
  bad_exit "and the send exits non-zero" $rc
  is_empty "and nothing was queued for the worker" "$(ab worker@srv1 consume --topic rt --tag 2 --wait 1s)"

  # Fire-and-forget asks for nothing back, so it stays open to a sender that
  # owns no queue. A check that only refuses is a check that refuses too much.
  ok_exit "a send from an unregistered sender still works" \
    "$(ab nobody@srv1 send worker@srv1 --topic rt --tag 3 "no answer wanted" >/dev/null 2>&1; echo $?)"
  has "and it arrives" "$(ab worker@srv1 consume --topic rt --tag 3 --wait 5s)" 'no answer wanted'

  # A script service answers the same way: the runner reads the route off the
  # envelope, so receipts and the answer all go to the third party.
  abx launcher@srv1 start relay@srv1 --algo args "$D/hello-world.sh" --descr "relays" >>"$D/start.log" 2>&1 &
  RPID=$!
  for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'relays' && break; sleep 0.2; done
  ab caller@srv1 send relay@srv1 --topic rt4 --tag 1 --reply-to third@srv1 "via a script" >/dev/null
  has "a script service acks to the third party" \
    "$(ab third@srv1 consume --topic rt4 --tag 1 --wait 15s)" '"receipt":"ack"'
  has "and answers it there" \
    "$(ab third@srv1 consume --topic rt4 --tag 1 --wait 15s)" 'Hello via a script'
  is_empty "and the caller hears nothing" "$(ab caller@srv1 consume --topic rt4 --tag 1 --wait 1s)"
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null

else skipped=$((skipped+1)); fi
if slow; then
  sec "a service is the inbox it registered, and stops when told"
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
  ab launcher@srv1 register absent@srv1 --kind generic --descr "never started" >/dev/null
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

else skipped=$((skipped+1)); fi
if slow; then
  sec "done: a script that finishes without an answer says so"
  # The gap `done` exists for. A script that succeeds and prints nothing used to
  # leave the caller with an ack and then silence until its deadline: the work
  # was finished and there was no way to hear it. A script that *answers* must
  # not also send one — a reply has plainly finished.
  printf '#!/bin/sh\ntrue\n' > "$D/silent.sh"; chmod +x "$D/silent.sh"
  ab launcher@srv1 register quiet@srv1 --kind generic >/dev/null
  abx launcher@srv1 start quiet@srv1 --algo args "$D/silent.sh" --descr "says nothing" >>"$D/start.log" 2>&1 &
  QPID=$!
  for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'says nothing' && break; sleep 0.2; done
  ab caller@srv1 send quiet@srv1 --topic dn --tag 1 "do it quietly" >/dev/null
  has "the silent service acks first" \
    "$(ab caller@srv1 consume --topic dn --tag 1 --wait 15s)" '"receipt":"ack"'
  has "and then says it finished" \
    "$(ab caller@srv1 consume --topic dn --tag 1 --wait 15s)" '"receipt":"done"'
  # The point of `done`, and the thing the first cut of this wave did not do:
  # a caller blocked on the answer stops when the work finishes with nothing to
  # return, instead of spending its whole deadline. Timing IS the check — the
  # margin is wide enough not to be a timing test.
  t0=$(date +%s)
  out=$(ab caller@srv1 call quiet@srv1 --topic dn5 --tag 1 --wait 20s "quietly again" 2>&1); rc=$?
  t1=$(date +%s)
  has "a call ends on done, saying which outcome it was" "$out" 'finished and sent no answer'
  bad_exit "and does not pretend it got an answer" $rc
  if [ $((t1-t0)) -lt 8 ]; then echo "  ok   and returns on the receipt, not at the deadline"; pass=$((pass+1));
  else echo "  FAIL and returns on the receipt, not at the deadline: waited $((t1-t0))s of 20s"; fail=$((fail+1)); fi
  kill $QPID 2>/dev/null; wait $QPID 2>/dev/null

  # A service that answers skips `done`: the check is that the message after the
  # ack is the answer, so an unconditional `done` turns it red.
  ab launcher@srv1 register loud@srv1 --kind generic >/dev/null
  abx launcher@srv1 start loud@srv1 --algo args "$D/hello-world.sh" --descr "answers" >>"$D/start.log" 2>&1 &
  LPID=$!
  for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'answers' && break; sleep 0.2; done
  ab caller@srv1 send loud@srv1 --topic dn3 --tag 1 "out loud" >/dev/null
  ab caller@srv1 consume --topic dn3 --tag 1 --wait 15s >/dev/null
  has "a service that answers sends no done" \
    "$(ab caller@srv1 consume --topic dn3 --tag 1 --wait 15s)" 'Hello out loud'
  kill $LPID 2>/dev/null; wait $LPID 2>/dev/null

  # The verb itself: one function serves both receipts, so `done` must reach the
  # sender exactly as `ack` does.
  ab owner@srv1 register handy@srv1 --kind generic >/dev/null
  ab caller@srv1 send handy@srv1 --topic dn4 --tag 1 "by hand" >/dev/null
  hid=$(ab handy@srv1 consume --wait 5s | sed -n 's/.*"message_id":"\([^"]*\)".*/\1/p')
  ab handy@srv1 done "$hid" >/dev/null
  has "the done verb sends a done receipt" \
    "$(ab caller@srv1 consume --topic dn4 --tag 1 --wait 5s)" '"receipt":"done"'
  # The exit code alone passed with the guard deleted: an unremembered id then
  # sends to an empty receiver and the DAEMON refuses it. Which end refused has
  # to be in the check, or it is not checking this end.
  has "done refuses an id this client never consumed" \
    "$(ab handy@srv1 done 0000 2>&1)" 'not one this client consumed'
  bad_exit "and exits non-zero" \
    "$(ab handy@srv1 done 0000 >/dev/null 2>&1; echo $?)"

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

else skipped=$((skipped+1)); fi
sec "the last two of the eleven verbs, and the one refusal the daemon owes us"
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
out=$("$D/agent-busd" -addr 0.0.0.0:$((PORT+1)) -socket "$D/public.sock" -token-file "$D/token" -dump-file "$D/public.dump" -dump-every 0 2>&1); rc=$?
bad_exit "the daemon refuses a public interface" $rc
has "and says why" "$out" 'not loopback'

sec "a service is service@host, or template/instance-name@host"
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

sec "one name, one answer"
has "lookup answers about a single name" \
  "$(ab owner@srv1 register looked@srv1 --descr "here" >/dev/null; curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-User: owner@srv1" -H "X-Agent-Bus-Token: $(tok owner@srv1)" "http://unix/lookup?name=looked@srv1")" '"descr":"here"'
has "and says so when there is none" \
  "$(code owner@srv1 "$(tok owner@srv1)" "/lookup?name=absent-entirely@srv1")" '404'
# The caller's own record is checked with this, not by pulling the registry.
echo '{"s":1}' | ab owner@srv1 service-template looked@srv1 - >/dev/null
is_empty "a lookup never carries a configuration" \
  "$(ab owner@srv1 ls looked@srv1 | grep -o '"config":[^,}]*')"
# Querying one service is how you check its setup without being able to read
# it, so the digest has to be there — for anyone, not only its owner.
has "but a query for one service carries its digest" \
  "$(ab nobody@srv1 ls looked@srv1)" '"config_sha":"'
has "and it is the same digest the whole listing gives" \
  "$(ab nobody@srv1 ls looked@srv1 | grep -o '"config_sha":"[a-f0-9]*"')" \
  "$(ab nobody@srv1 ls | grep -o '"name":"looked@srv1"[^}]*' | grep -o '"config_sha":"[a-f0-9]*"')"
has "asking about one name answers about one name" \
  "$(ab owner@srv1 ls looked@srv1 | grep -o '"name":' | wc -l)" '1'
has "and says so when there is no such service" \
  "$(ab owner@srv1 ls absent-entirely@srv1 2>&1)" 'no such name'

sec "no token, no serve"
# Stated as a rule, not sampled: every route the daemon exposes, refused
# three ways. A route added later without auth fails here.
for route in "GET /status" "POST /register" "GET /ls" "GET /lookup?name=x@h" \
             "POST /configure" "GET /config?name=x@h" "POST /send" "GET /consume?wait=0s"; do
  m=${route%% *}; path=${route#* }
  none=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/bus.sock" \
         -H "X-Agent-Bus-User: owner@srv1" -d '{}' "http://unix$path")
  wrong=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/bus.sock" \
          -H "X-Agent-Bus-User: owner@srv1" -H "X-Agent-Bus-Token: not-the-token" -d '{}' "http://unix$path")
  noname=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/bus.sock" \
           -H "X-Agent-Bus-Token: $TOKEN" -d '{}' "http://unix$path")
  has "$m $path is not served without a token" "$none/$wrong/$noname" '401/401/401'
done

sec "a caller states a record, never what the daemon observes"
is_empty "a registration cannot claim a reader it does not have" \
  "$(post_body owner@srv1 /register '{"name":"probe@srv1","kind":"agent","reading":true,"queued":77}' | grep -o '"reading":true\|"queued":77')"
is_empty "and the claim does not survive into a listing" \
  "$(ab nobody@srv1 ls probe@srv1 | grep -o '"reading":true')"
is_empty "a registration cannot claim a call count" \
  "$(post_body owner@srv1 /register '{"name":"probe3@srv1","in":99,"out":99}' | grep -o '"in":99\|"out":99')"
is_empty "a registration cannot claim a configuration digest" \
  "$(post_body owner@srv1 /register '{"name":"probe2@srv1","config_sha":"forged"}' | grep -o forged)"
has "an unknown topic mode is refused by the daemon, not only the CLI" \
  "$(post_code owner@srv1 "$(tok owner@srv1)" /register '{"name":"modey@srv1","kind":"topic","mode":"garbage"}')" '400'

sec "per-service call counters"
# A drained queue and one nobody ever wrote to both read as empty; these tell
# them apart. The control service proves the counters are the service's own
# and not a daemon-wide total copied onto every record.
ab owner@srv1 register counted@srv1 >/dev/null
ab owner@srv1 register control@srv1 >/dev/null
cin=$(svc counted@srv1 in); cout=$(svc counted@srv1 out)
kin=$(svc control@srv1 in); kout=$(svc control@srv1 out)
ab caller@srv1 send counted@srv1 --topic cnt --tag 1 "one" >/dev/null
ab caller@srv1 send counted@srv1 --topic cnt --tag 2 "two" >/dev/null
delta "a listing counts what arrived for a service" 2 "$cin" "$(svc counted@srv1 in)"
delta "and nothing handed over while the queue still holds them" 0 "$cout" "$(svc counted@srv1 out)"
ab counted@srv1 consume --wait 2s >/dev/null
delta "consuming counts one out" 1 "$cout" "$(svc counted@srv1 out)"
delta "and does not count it in a second time" 2 "$cin" "$(svc counted@srv1 in)"
delta "a service nobody wrote to counts nothing in" 0 "$kin" "$(svc control@srv1 in)"
delta "nor anything out" 0 "$kout" "$(svc control@srv1 out)"
# The inbox has to be *empty* for the next leg, or the reader takes the
# leftover off the queue and never blocks — which is the queued path again,
# and it passed the straight-through mutants until this drain was added.
ab counted@srv1 consume --wait 2s >/dev/null
has "and the inbox is empty before a reader blocks on it" "$(svc counted@srv1 queued)" '^0$'
ab counted@srv1 consume --wait 5s >/dev/null 2>&1 & LPID=$!
sleep 0.3
ab caller@srv1 send counted@srv1 --topic cnt --tag 3 "straight through" >/dev/null
wait $LPID 2>/dev/null
delta "a message handed to a waiting reader counts in" 3 "$cin" "$(svc counted@srv1 in)"
delta "and out, without ever sitting in the queue" 3 "$cout" "$(svc counted@srv1 out)"

sec "consuming as a name nobody registered is refused, not answered with silence"
has "the daemon says register it first" \
  "$(code ghost@srv1 "$(tok ghost@srv1)" "/consume?wait=0s")" '404'
has "while a registered name with an empty inbox is 204" \
  "$(ab quiet@srv1 register quiet@srv1 >/dev/null; code quiet@srv1 "$(tok quiet@srv1)" "/consume?wait=0s")" '204'

if slow; then
  sec "a listing says whether a call would reach anyone"
  # Being in the registry and being callable are different facts: "there is a
  # MySQL on db1:3306" registers fine and nothing on this bus answers for it.
  ab owner@srv1 register db.main@srv1 --protocol mysql --addr db1:3306 --descr "the main database" >/dev/null
  has "a registration can say how to call it" "$(ab nobody@srv1 ls db.main@srv1)" '"protocol":"mysql"'
  # Asserted on the record itself, not on an empty grep: an is_empty that a
  # failed query also satisfies is a check that passes with the daemon down.
  ab owner@srv1 register plain.svc@srv1 >/dev/null
  has "and an ordinary bus service says nothing, because there is nothing to say" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"name":"plain.svc@srv1"\|"protocol":')" '"name":"plain.svc@srv1"'
  is_empty "so no protocol comes back for it" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"protocol":[^,}]*')"
  has "a registered name nobody serves is not shown as read" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"reading":[a-z]*\|"name":"plain.svc@srv1"')" '"name":"plain.svc@srv1"'
  is_empty "and reading is absent rather than false" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"reading":true')"
  # And with a reader attached, the same query says so — this is the whole
  # difference between "registered" and "callable right now".
  ab reader@srv1 register reader@srv1 >/dev/null
  ab reader@srv1 consume --wait 6s >/dev/null 2>&1 &
  reader_pid=$!
  sleep 1
  has "a name something is reading says so" "$(ab nobody@srv1 ls reader@srv1)" '"reading":true'
  ab nobody@srv1 send reader@srv1 "wake up" >/dev/null
  wait $reader_pid
  has "and the depth of what is waiting is visible" \
    "$(ab nobody@srv1 send plain.svc@srv1 "one" >/dev/null; ab nobody@srv1 ls plain.svc@srv1)" '"queued":1'

else skipped=$((skipped+1)); fi
sec "configuring a service template produces a configured service"
# The configuration is arbitrary JSON and stays opaque; the one thing that
# matters to the bus is that it never shows up where it should not.
echo '{"model":"opus","depth":3}' | ab owner@srv1 service-template code-review/cfg@rdvp - >/dev/null
has "the configuration comes back as it went in, to the service" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '{"model":"opus","depth":3}'
has "but not to the owner who set it" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp 2>&1)" 'private to the service'
has "and that is a refusal, not a failure of ours" \
  "$(code owner@srv1 "$(tok owner@srv1)" "/config?name=code-review/cfg@rdvp")" '403'

is_empty "a listing never carries it" \
  "$(ab owner@srv1 ls | grep -o '"config":[^,}]*')"
is_empty "and neither does the answer to setting one" \
  "$(echo '{"secret":"x"}' | ab owner@srv1 service-template echoes@srv1 - | grep -o '"config":[^,}]*')"
# What a query gets instead: enough to see that a write landed and that two
# are the same, without handing anyone the configuration.
sha=$(echo '{"secret":"x"}' | ab owner@srv1 service-template digested@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')
has "setting one answers with its digest" "$sha" '"config_sha":"'
has "and a query carries the same digest" "$(ab nobody@srv1 ls)" "$sha"
has "the digest is sha256 of the stored bytes" "$sha" "$(printf '%s' '{"secret":"x"}' | sha256sum | cut -d' ' -f1)"
has "reformatting is not a change" \
  "$(printf '{ "secret" : "x" }' | ab owner@srv1 service-template reformatted@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')" "$sha"
has "a different configuration is a different digest" \
  "$(if [ "$(echo '{"secret":"y"}' | ab owner@srv1 service-template other@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')" != "$sha" ]; then echo differs; fi)" 'differs'
is_empty "an unconfigured service has no digest at all" \
  "$(ab owner@srv1 register plain@srv1 | grep -o '"config_sha":[^,}]*')"
# A send is refused unless the receiver has a record, so this proves the
# record was created — the inbox itself is made lazily by the send either way.
has "configuring creates the service, so it can be sent to" \
  "$(ab sender@srv1 send code-review/cfg@rdvp "it exists" >/dev/null; ab code-review/cfg@rdvp consume --wait 2s)" 'it exists'

has "a stranger may not read it either" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp 2>&1)" 'private to the service'
# The text alone would still read right if every refusal collapsed to a 500.
has "and is refused as forbidden, not as our own fault" \
  "$(code nosy@srv1 "$(tok nosy@srv1)" "/config?name=code-review/cfg@rdvp")" '403'

has "and may not overwrite it" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp '{"model":"theirs"}' 2>&1)" 'belongs to someone else'
has "the owner's configuration survived that" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

has "a configuration that is not JSON is refused" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp 'not json' 2>&1)" 'a configuration is JSON'
# The CLI refuses that one before it leaves; the daemon has to refuse it too,
# and the shape that reaches it is a body carrying no configuration at all.
has "and the daemon refuses an empty one on its own" \
  "$(post_code owner@srv1 "$(tok owner@srv1)" /configure '{"name":"code-review/cfg@rdvp"}')" '400'
has "null is not a configuration either" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp null 2>&1)" 'null is the absence of one'

# A service registers itself on every start. That must refresh its
# description without destroying what it was configured with, and without
# handing the record to whoever registered last.
ab code-review/cfg@rdvp register code-review/cfg@rdvp --kind agent --descr "refreshed" >/dev/null
has "a re-registration keeps the configuration" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

has "and still refreshes the description" \
  "$(ab owner@srv1 ls)" '"descr":"refreshed"'
ab thief@srv1 register code-review/cfg@rdvp --kind agent >/dev/null
# Ownership is observable through a WRITE: nobody can read a configuration
# but the service, so a read cannot tell us who owns the record.
has "and does not hand the record to whoever registered last" \
  "$(ab thief@srv1 service-template code-review/cfg@rdvp '{"mine":"now"}' 2>&1)" 'belongs to someone else'
has "a registration may not smuggle a configuration in" \
  "$(post_code thief@srv1 "$(tok thief@srv1)" /register '{"name":"code-review/cfg@rdvp","config":{"evil":true}}' >/dev/null; ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

# On a name that does not exist yet there is no old configuration to keep, so
# this is the only shape that proves register drops the field rather than
# being saved by the preservation rule.
post_code smuggler@srv1 "$(tok smuggler@srv1)" /register '{"name":"fresh@srv1","config":{"evil":true}}' >/dev/null
has "not even onto a name that is new" \
  "$(ab fresh@srv1 service-template fresh@srv1)" 'null'

# The service itself is as entitled to configure as its owner: otherwise the
# order of "register" and "configure" decides whether either works. The record
# has to be owned by SOMEONE ELSE for this to test anything.
ab keeper@srv1 service-template theirs@srv1 '{"by":"keeper"}' >/dev/null
has "a service may configure itself, on a record it does not own" \
  "$(ab theirs@srv1 service-template theirs@srv1 '{"by":"itself"}' >/dev/null 2>&1; ab theirs@srv1 service-template theirs@srv1)" '"by":"itself"'

# Reading must work where it is actually used: a script, with no terminal.
has "a read works with no terminal on stdin" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp </dev/null)" '"depth":3'


if slow; then
  sec "ttl: a message outlives its worth, and nothing else is counted as that"
  ab owner@srv1 register keeper@srv1 --kind generic --ttl 1h >/dev/null
  # The control that matters most: a TTL must not throw the message away
  # early. A check that only proves disappearance passes when everything
  # expires immediately.
  ab caller@srv1 send keeper@srv1 --topic tt --tag 1 --ttl 30s "still worth reading" >/dev/null
  has "a message inside its ttl is delivered" \
    "$(ab keeper@srv1 consume --topic tt --tag 1 --wait 2s)" 'still worth reading'

  e0=$(count expired)
  ab caller@srv1 send keeper@srv1 --topic tt --tag 2 --ttl 300ms "too late to matter" >/dev/null
  sleep 1
  is_empty "a message past its ttl is never handed over" "$(ab keeper@srv1 consume --topic tt --tag 2 --wait 1s)"
  delta "and it is counted as expired" 1 "$e0" "$(count expired)"

  # The queue's TTL bounds the sender's: asking for longer does not get it.
  ab owner@srv1 register brief@srv1 --kind generic --ttl 300ms >/dev/null
  ab caller@srv1 send brief@srv1 --topic tt --tag 3 --ttl 1h "asked for an hour" >/dev/null
  sleep 1
  is_empty "a queue's ttl bounds a longer one on the message" "$(ab brief@srv1 consume --topic tt --tag 3 --wait 1s)"
  # and the other direction still works: shorter on the message wins
  ab caller@srv1 send keeper@srv1 --topic tt --tag 4 --ttl 300ms "shorter than the queue" >/dev/null
  sleep 1
  is_empty "and a shorter one on the message wins over the queue's" "$(ab keeper@srv1 consume --topic tt --tag 4 --wait 1s)"

  # Two counters, two problems. One that counted both could not tell an
  # operator whether the queue is too small or the message too old.
  d0=$(count dropped); e2=$(count expired)
  ab owner@srv1 register ring.small@srv1 --kind generic --overflow ring --bound 2 >/dev/null
  for i in 1 2 3 4; do ab caller@srv1 send ring.small@srv1 --topic tt --tag r "m$i" >/dev/null; done
  delta "a ring drop is counted as dropped" 2 "$d0" "$(count dropped)"
  delta "and not as expired" 0 "$e2" "$(count expired)"

  # The bound is the record's, not one number for the whole daemon.
  ab owner@srv1 register tiny@srv1 --kind generic --bound 1 >/dev/null
  ab caller@srv1 send tiny@srv1 --topic tt --tag s "first" >/dev/null
  out=$(ab caller@srv1 send tiny@srv1 --topic tt --tag s "second" 2>&1); rc=$?
  has "a record's own bound refuses the one past it" "$out" 'queue is full'
  bad_exit "and says so to the sender" $rc

  has "a ttl that is not a duration is refused" \
    "$(ab owner@srv1 register bad.ttl@srv1 --ttl soon 2>&1)" 'ttl is a duration'
  has "a bound that is not a count is refused" \
    "$(ab owner@srv1 register bad.bound@srv1 --bound plenty 2>&1)" 'positive number'
else skipped=$((skipped+1)); fi

if slow; then
  sec "a full queue: refuse by default, drop the oldest if asked"
  ab owner@srv1 register sink@srv1 --kind generic >/dev/null
  ab owner@srv1 register ringy@srv1 --kind generic --overflow ring >/dev/null
  has "a record says what a full queue does, and refuses by default" \
    "$(ab owner@srv1 ls | grep -o '{[^}]*"name":"sink@srv1"[^}]*}')" '"overflow":"strict"'
  has "an overflow mode that is neither is refused" \
    "$(ab owner@srv1 register bad@srv1 --overflow maybe 2>&1)" 'overflow is strict or ring'
  # 1001 into a queue bounded at 1000, twice: strict must refuse the last one,
  # ring must swallow it and lose the first.
  d0=$(count dropped)
  for i in $(seq 0 1000); do ab flood@srv1 send sink@srv1 "msg-$i" >/dev/null 2>&1; done
  out=$(ab flood@srv1 send sink@srv1 "one too many" 2>&1); rc=$?
  bad_exit "strict refuses the send rather than lose a message" $rc
  has "and names the queue that is full" "$out" 'queue is full: sink@srv1'
  has "and says the receiver cannot take it, not that we broke" \
    "$(post_code flood@srv1 "$(tok flood@srv1)" /send '{"to":"sink@srv1","body":"one more"}')" '503'
  delta "strict dropped nothing" 0 "$d0" "$(count dropped)"
  d1=$(count dropped)
  for i in $(seq 0 1000); do ab flood@srv1 send ringy@srv1 "msg-$i" >/dev/null 2>&1; done
  has "ring keeps taking, and the oldest is what went" "$(ab ringy@srv1 consume --wait 2s)" 'msg-1"'
  delta "and the loss is counted, not silent" 1 "$d1" "$(count dropped)"

else skipped=$((skipped+1)); fi
sec "--wait is the caller's deadline, not just the daemon's"
# Against a bus that answers everything but stalls the consume: the wait the
# daemon is asked for cannot bound a transfer that never finishes, so the
# deadline has to be on the client's own request.
bun -e "Bun.serve({port:$((PORT+3)),async fetch(r){const u=new URL(r.url);
  if(u.pathname==='/consume'){await Bun.sleep(30000);return new Response('{}');}
  if(u.pathname==='/ls')return new Response('[]');
  return new Response('{}');}})" >/dev/null 2>&1 &
MPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$((PORT+3))/ls" && break; sleep 0.2; done
out=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+3)) AGENT_BUS_TOKEN=$(tok impatient@srv1) AGENT_BUS_NAME=impatient@srv1 \
      timeout 5 "$D/agent-bus" call slow@srv1 --wait 500ms "are you there?" 2>&1); rc=$?
kill $MPID 2>/dev/null; wait $MPID 2>/dev/null
if [ "$rc" -eq 124 ]; then
  echo "  FAIL a stalled consume ends at --wait: it ran past the deadline"; fail=$((fail+1))
else
  echo "  ok   a stalled consume ends at --wait"; pass=$((pass+1))
fi
has "and says the message was accepted" "$out" 'do not resend'

sec "the token an SSH forced command hands out"
# Not "contains the token": a debug line printed before it passed that, and
# $(ssh … static-token) would then hold a credential that does not work. The
# proof is the captured value authenticating against the daemon.
mine=$(tok over-ssh@srv1)
issued=$(AGENT_BUS_TOKEN_FILE=$D/token ./static-token over-ssh@srv1); rc=$?
ok_exit "static-token succeeds" $rc
if [ "$issued" = "$mine" ]; then echo "  ok   it prints that principal's token and nothing else"; pass=$((pass+1));
else echo "  FAIL it prints that principal's token and nothing else: [$issued]"; fail=$((fail+1)); fi
has "the token it hands out authenticates" \
  "$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$issued AGENT_BUS_NAME=over-ssh@srv1 "$D/agent-bus" status)" '"up"'
# The principal is in the forced command, so the key a person holds picks
# the line. Asking for a name it was not given is the forgery this stops.
if [ "$issued" = "$TOKEN" ]; then echo "  FAIL it handed out the owner's token instead"; fail=$((fail+1));
else echo "  ok   and it is not the owner's"; pass=$((pass+1)); fi
out=$(AGENT_BUS_TOKEN_FILE=$D/token ./static-token 2>&1); rc=$?
bad_exit "without a principal it hands out nothing" $rc
has "and says the forced command must name one" "$out" 'authorized_keys'
out=$(AGENT_BUS_TOKEN_FILE=$D/token ./static-token nobody@nowhere 2>&1); rc=$?
bad_exit "a principal with no token is an error, not a blank one" $rc
printf '   \n' > "$D/blank-token"
out=$(AGENT_BUS_TOKEN_FILE=$D/blank-token ./static-token over-ssh@srv1 2>&1); rc=$?
bad_exit "an empty token file is refused, not handed out as an empty token" $rc
out=$(SSH_ORIGINAL_COMMAND='cat /etc/passwd' AGENT_BUS_TOKEN_FILE=$D/token ./static-token over-ssh@srv1 2>&1); rc=$?
bad_exit "it refuses any other command" $rc
has "and says why" "$out" 'one command'
out=$(AGENT_BUS_TOKEN_FILE=$D/absent ./static-token over-ssh@srv1 2>&1); rc=$?
bad_exit "a missing token file is an error, not an empty token" $rc

sec "a restart is not a loss"
# Its own daemon, its own store and its own dump: the point of this section
# is what a stop and a start do to memory, which needs a process nothing
# else is using. See docs/04-messaging.md#durability.
mkdir -p "$D/dur"
DUR=""
dur_up() {
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+8)) -socket "$D/dur/bus.sock" -token-file "$D/dur/token" \
    -owner "$OWNER" -dump-file "$D/dur/dump.json" -dump-every 0 >"$D/dur/$1.log" 2>&1 &
  DUR=$!
  for _ in $(seq 1 50); do [ -S "$D/dur/bus.sock" ] && break; sleep 0.1; done
  DTOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/dur/token")
  KTOK=$(AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token keeper@srv1 2>/dev/null)
}
dur_down() { kill "$1" "$DUR" 2>/dev/null; wait "$DUR" 2>/dev/null; }
dab() { AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
# Reading an inbox is the inbox's own business, so the drain runs as keeper
# and not as the owner: an error from the wrong name would read as an empty
# queue to every check below.
dkeep() { AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$KTOK AGENT_BUS_NAME=keeper@srv1 "$D/agent-bus" "$@"; }
dsvc() { local n; n=$(dab ls keeper@srv1 | sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }

dur_up first
is_empty "a first start has nothing to say about a previous one" \
  "$(grep 'did not stop cleanly' "$D/dur/first.log")"
dab register keeper@srv1 --descr "keeps things" >/dev/null
KTOK=$(dab token keeper@srv1)
dab send keeper@srv1 "before the restart" >/dev/null
dab send keeper@srv1 "also before it" >/dev/null
has "a reader took the first of them" "$(dkeep consume --wait 0s)" 'before the restart'
dur_down -TERM

dur_up second
has "the registry is back after a graceful stop" "$(dab ls keeper@srv1)" 'keeps things'
# Counters are the other half of the state: a queue that was drained and one
# nobody ever wrote to read the same without them.
has "the counters come back too, not just the messages" "$(dsvc in)" '^2$'
has "and a read is remembered as a read" "$(dsvc out)" '^1$'
has "and so is the message nobody had taken" "$(dkeep consume --wait 0s)" 'also before it'
dur_down -TERM

dur_up third
out=$(dkeep consume --wait 0s 2>&1); rc=$?
is_empty "a drained queue stays drained — nothing is delivered twice" "$out"
ok_exit "and an empty queue is an answer, not an error" $rc
has "while the record that owned it is still there" "$(dab ls keeper@srv1)" 'keeps things'
has "and both reads are still counted" "$(dsvc out)" '^2$'
# SIGKILL: no dump is written, so the snapshot on disk is the one the start
# wrote, and its own flag is what says the run ended badly.
dab send keeper@srv1 "lost with the process" >/dev/null
kill -9 "$DUR" 2>/dev/null; wait "$DUR" 2>/dev/null

dur_up fourth
has "after an ungraceful kill the next start says so" \
  "$(cat "$D/dur/fourth.log")" 'did not stop cleanly'
has "and says from when it is missing traffic" "$(cat "$D/dur/fourth.log")" 'anything queued after'
is_empty "the message that died with the process is not invented back" \
  "$(dkeep consume --wait 0s 2>&1)"
dur_down -TERM

if slow; then
  # A message whose moment passed while the daemon was down is not worth
  # delivering late, and the reload is where that is decided.
  dur_up fifth
  dab send keeper@srv1 --ttl 2s "too late by the time you read this" >/dev/null
  dab send keeper@srv1 "still worth having" >/dev/null
  dur_down -TERM
  sleep 3
  dur_up sixth
  has "a message that outlived its ttl while down is not delivered" \
    "$(dkeep consume --wait 0s)" 'still worth having'
  is_empty "and it is the only one left" "$(dkeep consume --wait 0s 2>&1)"
  dur_down -TERM
fi

if slow; then
  sec "the MCP face"
  # bun is not optional: the MCP face and both push modes are the PoC
  # (docs/12-stages.md#poc), so a host without it fails rather than passing green.
  if ! command -v bun >/dev/null 2>&1; then
    echo "  FAIL bun is not installed; the MCP face cannot be checked"
    fail=$((fail + 1))
  else
    # The shared JSON-RPC plumbing, driven directly: the harnesses below only
    # ever have one request in flight and never split a line across chunks, so
    # they leave most of rpc.ts unwatched (mcp/rpc.test.ts says why).
    out=$(cd mcp && timeout 60 bun test rpc.test.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "rpc unit tests" $rc

    # each harness runs its own peer in-process, so there is no start-order race
    out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock \
          AGENT_BUS_OWNER=$OWNER AGENT_BUS_OWNER_TOKEN=$TOKEN \
          AGENT_BUS_TOKEN=$(tok mcp.session@srv1) \
          AGENT_BUS_NAME=mcp.session@srv1 SMOKE_PEER=peer@srv1 bun run smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "mcp smoke" $rc

    out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock \
          AGENT_BUS_OWNER=$OWNER AGENT_BUS_OWNER_TOKEN=$TOKEN \
          AGENT_BUS_TOKEN=$(tok pusher@srv1) \
          AGENT_BUS_NAME=pusher@srv1 PUSH_NAME=push.session@srv1 bun run smoke-push.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "claude push smoke" $rc

    out=$(cd mcp && timeout 120 bun run smoke-codex.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "codex adapter smoke" $rc
  fi

else skipped=$((skipped+1)); fi
sec "end"
echo; echo "passed $pass, failed $fail"
if [ "$skipped" -gt 0 ]; then
  echo "SKIPPED $skipped slow sections and the race detector — this is NOT a full run."
  echo "         run ./smoke.sh --slow before believing a change."
fi
echo; echo "slowest sections (ms):"; sort -rn "$D/timing" | head -14 | sed 's/^/  /'
[ $fail -eq 0 ]
