#!/bin/bash
# Acceptance for the built stages. Builds, runs a daemon on loopback and a
# private socket, exercises the verbs, exits non-zero on any failure.
#
#   ./smoke.sh          the fast run: everything that costs under a second
#   ./smoke.sh --slow    all of it, including the race detector
#
# The fast run is for the edit-run loop. **A change is measured against
# --slow**, and so is every mutation: a check that did not run caught
# nothing (Plans/done/PoC/README.md#mutation-first-then-belief).
set -u
cd "$(dirname "$0")"
# This run builds its own fixture: whatever bus the caller is already talking
# to is not it. A session launcher exports AGENT_BUS_TOKEN, NAME, ADDR, PUSH,
# RUNTIME, DESCR and the control pair, and the MCP face reads all of them —
# so run from an agent session and the face joins the live daemon in push
# mode, half the checks fail, and the failure looks like the tree.
# Setting the few this script knows about is not enough; the rest have to go.
for v in $(env | sed -n 's/^\(AGENT_BUS_[A-Z_]*\)=.*/\1/p'); do
  case $v in AGENT_BUS_ROLE|AGENT_BUS_FDS) ;; *) unset "$v" ;; esac
done
D=$(mktemp -d); DPID=""; SUPUNIT=""
# Wait for it: a daemon dumps on the way out, and a dump written while the
# directory is being removed leaves the directory behind.
cleanup() {
  [ -n "$SUPUNIT" ] && systemctl --user stop "$SUPUNIT" >/dev/null 2>&1
  [ -n "$DPID" ] && { kill "$DPID" 2>/dev/null; wait "$DPID" 2>/dev/null; }
  rm -rf "$D"
}
trap cleanup EXIT
# The CLI keeps its reply context under XDG_CACHE_HOME, and this run wants a
# private one. Go's build cache lives there too by default, so redirecting it
# made every run recompile the world — 2.7s of "go vet" that is 0.1s warm,
# and four times that under -race. Keep Go's cache where it was.
export GOCACHE=${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build}
export XDG_CACHE_HOME=$D/cache
# A running script service leaves its note, its log and its work directory
# here (docs/08-runner-role.md#stopping-it-and-reading-what-it-said). Private
# to the run, or two runs would see each other's services as already running
# — which is what the mutation harness does, twenty at a time.
export XDG_STATE_HOME=$D/state
# A run owns PORT..PORT+14, so two of them need bases fifteen apart — closer
# and the second finds the first's daemon, which reads as "bad token".
PORT=${PORT:-7911}

export BUILD_STARTED=$(date +%s)
bash ./build.sh "$D" || exit 1
export BUILD_FINISHED=$(date +%s)

# The daemon belongs to a principal, which provisions the fixture names
# before issuing their credentials. Stated rather than taken from the
# account running the suite, so the checks read the same everywhere.
OWNER=parf@localhost
"$D/agent-busd" -addr 127.0.0.1:$PORT -socket "$D/bus.sock" -token-file "$D/token" -owner "$OWNER" -dump-file "$D/dump.json" -dump-every 0 >"$D/daemon.log" 2>&1 &
DPID=$!
# Wait for an answer, not for the socket: the supervisor binds it before the
# bus that serves it exists, so the file appearing means nothing yet.
# Any answer at all will do — a refusal is the bus answering. An unserved
# socket gives nothing back, because the connection waits in the backlog.
ready() { for _ in $(seq 1 100); do
  [ -n "$(curl -s --max-time 1 --unix-socket "$1" http://unix/status 2>/dev/null)" ] && return 0
  sleep 0.1
done; return 1; }
ready "$D/bus.sock" || { echo "daemon did not start"; cat "$D/daemon.log"; exit 1; }
# One line per principal, `name token`: the owner's is what the daemon wrote
# at start, and every other name gets one from it on first use.
TOKEN=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/token")
tok() {
  local f="$D/tok.$(printf '%s' "$1" | tr '/@.' '___')"
  [ -s "$f" ] || AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER     "$D/agent-bus-token" "$1" >"$f" || return
  cat "$f" 2>/dev/null
}
ab() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
# The same, for `&`: exec so that $! is the binary. Backgrounding the function
# instead makes $! a subshell, and a signal sent to it leaves the service
# running — which is how a check that a service stops passed without one.
abx() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 exec "$D/agent-bus" "${@:2}"; }
# Like ab, but bounded. For a verb whose REFUSAL is the point: a mutation that
# turns the refusal into a foreground service otherwise blocks the whole run,
# and a batch that times out loses every mutant after it.
abt() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 timeout 10 "$D/agent-bus" "${@:2}"; }
pass=0; fail=0; skipped=0
# Anything that takes more than a second is opt-in: the default run is the
# one a person waits for. `SLOW=1` (or --slow) runs everything, and the
# mutation harness always does — a mutant that survives because its check was skipped is the
# worst kind of green (Plans/done/PoC/README.md#mutation-first-then-belief).
SLOW=${SLOW:-0}
[ "${1:-}" = "--slow" ] && SLOW=1
slow() { [ "$SLOW" = 1 ]; }
# Counters are cumulative since the daemon started, so a check on one has to
# be a delta. Asserting the absolute value worked only while this section
# happened to run first, and broke the moment another one dropped a message.
count() { local n; n=$(ab parf@localhost status | sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }
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
lacks() { if echo "$2" | grep -q -- "$3"; then echo "  FAIL $1: [$2] still has [$3]"; fail=$((fail+1)); else echo "  ok   $1"; pass=$((pass+1)); fi; }
ok_exit()   { if [ "$2" -eq 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: exit $2"; fail=$((fail+1)); fi; }
bad_exit()  { if [ "$2" -ne 0 ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: expected failure, got exit 0"; fail=$((fail+1)); fi; }
# "nothing came back" needs its own check: `$(cmd; echo -n nothing)` contains
# the word whatever cmd did, which is how two checks here passed hollow.
is_empty()  { if [ -z "$2" ]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1: got [$2]"; fail=$((fail+1)); fi; }
code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok "$1")" "http://unix$2"; }
# One section of a rendered page, so that a check about one view cannot be
# satisfied by a name that also appears in another — the registry lists every
# record, and half the views below it are subsets of that list.
sect() { printf '%s' "$2" | sed -n "/<h2 id=$1>/,/<h2 id=[a-z]*>/p"; }
# Which of two strings a section mentions first, by name. An order check that
# only asked "is A present" passes however the rows are sorted.
first_of() { printf '%s' "$1" | grep -o -e "$2" -e "$3" | head -1; }
tbody() { curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix$2"; }
tcode() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix$2"; }
post_body() { curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok "$1")" -d "$3" "http://unix$2"; }
post_code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok "$1")" -d "$3" "http://unix$2"; }

# Provision people separately from tokens and service records. Drop only their
# backing inbox, retaining the profile: tests about a sender without an inbox
# must not accidentally give it one. A failed fixture stops the run, rather
# than letting a later refusal look like the behavior under test.
users() {
  local name
  for name in "$@"; do
    curl -fsS --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" \
      -d "{\"name\":\"$name\",\"create\":true}" http://unix/user >/dev/null || exit 1
    ab "$name" unregister "$name" >/dev/null || exit 1
  done
}

sec "the Go checks"
# Run here, not only by hand: a mutation of anything the unit tests cover was
# invisible to this script while they lived outside. CLAUDE.md asks for all
# three to be green.
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
  "$(go list -deps ./internal/core ./internal/auth ./internal/ports | grep -E 'internal/(store|dump|directory|signature)')"
is_empty "nor does a face" \
  "$(go list -deps ./internal/api ./cmd/agent-bus ./cmd/agent-bus-web ./cmd/agent-bus-setup ./cmd/agent-bus-token ./cmd/agent-bus-admin | grep -E 'internal/(store|dump|directory|signature)')"
is_empty "and a port names no outside world of its own" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/ports 2>&1 | grep -E '^(os|net|net/http|os/exec|database/sql)$')"
has "while the process that assembles them holds the ones that persist" \
  "$(go list -deps ./cmd/agent-busd | grep -E 'internal/(store|dump|directory|signature)' | tr '\n' ' ')" 'internal/directory/file .*internal/directory/github .*internal/dump/jsonfile .*internal/signature/sshkeygen .*internal/store/file'
has "and core is what asks for it" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/auth 2>&1)" 'internal/ports'

if slow; then
  sec "shared version and live process titles"
  out=$(timeout 60 bash ./smoke-process.sh "$D" 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "process version smoke" "$rc"
else skipped=$((skipped+1)); fi

sec "status on both listeners"
has "unix socket" "$(ab parf@localhost status)" '"services"'
has "loopback tcp" "$(AGENT_BUS_ADDR=http://127.0.0.1:$PORT AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=parf@localhost "$D/agent-bus" status)" '"up"'
# The API has no page of its own, so a person who opened it in a browser is
# sent to the one that has. Permanently, and only from the exact root.
# See docs/05-discovery.md#where-it-listens.
has "the api root sends a browser to the dashboard" \
  "$(curl -sS -o /dev/null -w '%{http_code} %{redirect_url}' "http://127.0.0.1:$PORT/")" \
  '^301 http://127.0.0.1:6780/$'
has "and a mistyped route is still a mistyped route" \
  "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/no-such-route")" '404'

sec "the token is the whole of a call"
users alice@srv1
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=nope "$D/agent-bus" status 2>&1); rc=$?
has "wrong token says so" "$out" 'bad token'; bad_exit "wrong token exits non-zero" $rc
has "401 for a wrong token" "$(tcode nope /status)" '401'
# Nothing but the token goes on the wire, and the daemon reads the caller out
# of it. See docs/02-access.md#what-a-call-carries.
has "a token with nothing beside it is served" "$(tcode "$TOKEN" /status)" '200'
has "and the daemon reads the caller out of it" \
  "$(env -u AGENT_BUS_NAME AGENT_BUS_ADDR=http://127.0.0.1:$PORT AGENT_BUS_TOKEN=$(tok alice@srv1) "$D/agent-bus" status)" '"you":"alice@srv1"'

sec "your own socket says who you are"
users nobody2@srv1
# Nothing to set up locally: the daemon knows the account at the other end
# from which socket it arrived on. See docs/02-access.md#local-socket.
ACCOUNT=$(id -un)
MINE=$D/user-$ACCOUNT.sock
has "the daemon opened one for the account it runs as" "$([ -S "$MINE" ] && echo yes)" 'yes'
has "and it is that account's alone" "$(stat -c %a "$MINE")" '^600$'
has "the token-authenticated shared socket is reachable across local accounts" "$(stat -c %a "$D/bus.sock")" '^666$'
has "a call with no name and no token is served" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" status)" '"up"'
has "and the daemon says whose call it was" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" status)" "\"you\":\"$OWNER\""
mkdir -p "$D/discovery"
ln -s "$D" "$D/discovery/agent-bus"
has "the CLI discovers its local user socket without an address or token" \
  "$(env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" status)" "\"you\":\"$OWNER\""
has "the CLI discovers the shared socket when a token supplies the identity" \
  "$(env -u AGENT_BUS_ADDR AGENT_BUS_TOKEN=$(tok alice@srv1) XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" status)" '"you":"alice@srv1"'
local_token=$(env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus-token" "$OWNER")
has "the token helper discovers the user socket and retrieves the existing credential" \
  "$([ "$local_token" = "$TOKEN" ] && echo yes)" '^yes$'
alice_token=$(tok alice@srv1)
local_token=$(env -u AGENT_BUS_ADDR AGENT_BUS_TOKEN=$alice_token XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus-token" alice@srv1)
has "the token helper discovers the shared socket for an existing token" \
  "$([ "$local_token" = "$alice_token" ] && echo yes)" '^yes$'
out=$(env -u AGENT_BUS_ADDR AGENT_BUS_TOKEN=$alice_token XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus-token" "$OWNER" 2>&1); rc=$?
bad_exit "token helper discovery never borrows the local account's authority" $rc
out=$(AGENT_BUS_ADDR=$D/missing.sock XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus-token" "$OWNER" 2>&1); rc=$?
bad_exit "the token helper does not replace an explicit unavailable address" $rc
has "the token helper reports the explicit failing socket" "$out" 'missing.sock'
has "a global CLI address takes precedence over an invalid environment address" \
  "$(env -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/missing.sock "$D/agent-bus" --addr "$MINE" status)" "\"you\":\"$OWNER\""
has "the equals form of a global CLI address takes precedence too" \
  "$(env -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/missing.sock "$D/agent-bus" --addr="$MINE" status)" "\"you\":\"$OWNER\""
has "an explicit environment address is not replaced by discovered sockets" \
  "$(AGENT_BUS_ADDR=$D/missing.sock AGENT_BUS_TOKEN=$TOKEN XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" status 2>&1)" 'missing.sock'
has "a write lands under the socket's principal, not under nobody" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" register sock-made@srv1 --allow '*' --descr "from the socket" >/dev/null; ab nobody2@srv1 ls sock-made@srv1)" "\"owner\":\"$OWNER\""
# One daemon, many people. A second mapped account gets a socket of its own
# and is a different principal on it — which is the whole point of the
# arrangement, and is not provable with one socket.
if id -u nobody >/dev/null 2>&1; then
  mkdir -p "$D/multi"
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+6)) -socket "$D/multi/bus.sock" -token-file "$D/token" \
    -owner "$OWNER" -user "nobody=nemo@srv1" -dump-file "$D/multi/dump.json" -dump-every 0 >"$D/multi.log" 2>&1 &
  MPID2=$!
  ready "$D/multi/user-nobody.sock" || echo "  WARNING: $D/multi/user-nobody.sock never answered"
  has "a second account gets a socket of its own" "$([ -S "$D/multi/user-nobody.sock" ] && echo yes)" 'yes'
  # Socket ownership identifies a caller; it does not create a bus user.
  for route in "GET /status" "POST /register" "POST /token" "POST /user" "POST /session"; do
    m=${route%% *}; path=${route#* }
    has "an unknown mapped user cannot $route, even for itself" \
      "$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/multi/user-nobody.sock" \
        -d '{"name":"nemo@srv1","create":true}' "http://unix$path")" '^401$'
  done
  has "and the refusal is about an unknown principal" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" http://unix/status)" 'answers for nobody'
  curl -fsS --unix-socket "$D/multi/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" \
    -d '{"name":"nemo@srv1","create":true}' http://unix/user >/dev/null || exit 1
  has "and is a different principal on it" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" "http://unix/status")" '"you":"nemo@srv1"'
  has "while the owner's socket in the same directory is still the owner" \
    "$(curl -s --unix-socket "$D/multi/user-$ACCOUNT.sock" "http://unix/status")" "\"you\":\"$OWNER\""
  # Unprivileged, the chown cannot land, and the daemon has to say so rather
  # than leave a socket that looks like somebody else's and is not.
  has "it says out loud when it could not hand the socket over" "$(cat "$D/multi.log")" 'CAP_CHOWN'
  kill $MPID2 2>/dev/null; wait $MPID2 2>/dev/null
else
  skipped=$((skipped+1))
fi
has "the shared socket still wants a token" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/bus.sock "$D/agent-bus" status 2>&1)" 'AGENT_BUS_TOKEN'
has "and the directory is walk-through only, not readable" "$(stat -c %a "$D")" '^711$'

sec "a token names its principal and nothing else does"
users bob@srv1
alice=$(tok alice@srv1); bob=$(tok bob@srv1)
has "two tokens are two principals on one socket" \
  "$(tbody "$alice" /status)" '"you":"alice@srv1"'
has "and the other one is the other" \
  "$(tbody "$bob" /status)" '"you":"bob@srv1"'
has "a token nobody holds is refused" "$(tcode not-a-token /status)" '401'
has "a write lands under the token's principal" \
  "$(post_body alice@srv1 /register '{"name":"alice-wrote@srv1","allow":["*"]}' >/dev/null; ab bob@srv1 ls alice-wrote@srv1)" '"owner":"alice@srv1"'

sec "who may ask for whose credential"
has "a principal may get its own" \
  "$(post_code alice@srv1 /token '{"name":"alice@srv1"}')" '200'
has "but not somebody else's" \
  "$(post_code alice@srv1 /token '{"name":"bob@srv1"}')" '403'
ab alice@srv1 register alice-svc@srv1 --allow '*' --descr "hers" >/dev/null
has "and may get one for a service it owns" \
  "$(post_code alice@srv1 /token '{"name":"alice-svc@srv1"}')" '200'
has "while somebody else may not" \
  "$(post_code bob@srv1 /token '{"name":"alice-svc@srv1"}')" '403'
has "even the daemon owner cannot mint a credential for an unknown name" \
  "$(post_code "$OWNER" /token '{"name":"nobody-owns-this@srv1"}')" '^401$'
has "nor rotate one into existence" \
  "$(post_code "$OWNER" /token '{"name":"nobody-owns-this@srv1","rotate":true}')" '^401$'
is_empty "a refused token ask creates no stored credential" \
  "$(awk '$1 == "nobody-owns-this@srv1" {print $1}' "$D/token")"
ab "$OWNER" register nobody-owns-this@srv1 --allow '*' >/dev/null || exit 1
has "the owner may issue a credential after registering the name" \
  "$(post_code "$OWNER" /token '{"name":"nobody-owns-this@srv1"}')" '^200$'
users owner@srv1
again=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$alice AGENT_BUS_NAME=alice@srv1 "$D/agent-bus-token" alice@srv1 2>&1)
if [ "$again" = "$alice" ]; then echo "  ok   asking twice is a read, not a rotation"; pass=$((pass+1));
else echo "  FAIL asking twice is a read, not a rotation: [$again]"; fail=$((fail+1)); fi
# Rotation: two are accepted, the one before them is not. A refresh that
# stranded traffic already queued under the old token would be worse than
# no rotation at all. See docs/02-access.md#token-lifetime.
ab owner@srv1 register rotor@srv1 --allow '*' >/dev/null
first=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus-token" rotor@srv1)
second=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus-token" rotor@srv1 --rotate)
third=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus-token" rotor@srv1 --rotate)
if [ -n "$second" ] && [ "$second" != "$first" ] && [ "$third" != "$second" ]; then
  echo "  ok   --rotate hands out a new token each time"; pass=$((pass+1))
else
  echo "  FAIL --rotate hands out a new token each time: [$first/$second/$third]"; fail=$((fail+1))
fi
has "the current one works" "$(tcode "$third" /status)" '200'
has "and the one before it, so queued traffic is not stranded" "$(tcode "$second" /status)" '200'
has "but the one before that is refused" "$(tcode "$first" /status)" '401'
# Every principal is in the file, not just the owner's: a second daemon
# reading it hands the same credentials back, which is what "tokens are
# durable" has to mean once there is more than one.
# Its own daemon, stopped and started again on the same file. The principals
# are registered first and deliberately kept: a credential for a name the
# second start does not know about answers for nothing and is dropped at that
# start, which is the rule and not a fault
# (docs/02-access.md#ownerless-credentials). Minting for names nobody keeps is
# how a directory fills with rows nothing is behind.
mkdir -p "$D/r2"
r2_up() {
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+4)) -socket "$D/r2/bus.sock" -token-file "$D/r2/token" \
    -owner "$OWNER" -dump-file "$D/r2/dump.json" -dump-every 0 >"$D/daemon2-$1.log" 2>&1 &
  RPID=$!
  ready "$D/r2/bus.sock" || echo "  WARNING: $D/r2/bus.sock never answered"
  R2TOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/r2/token")
}
r2ab() { AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$R2TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
r2tok() { AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$R2TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" "$@"; }
r2code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r2/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix/status"; }
r2_up first
r2ab register kept-rotor@srv1 --allow '*' >/dev/null
r2ab register kept-other@srv1 --allow '*' >/dev/null
dropped=$(r2tok kept-rotor@srv1)
previous=$(r2tok kept-rotor@srv1 --rotate)
current=$(r2tok kept-rotor@srv1 --rotate)
other=$(r2tok kept-other@srv1)
has "both rotation slots work before the restart" "$(r2code "$previous")$(r2code "$current")" '200200'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
r2_up second
has "a restart keeps both of a rotated principal's tokens" "$(r2code "$current")" '200'
has "and the one before it, across the restart" "$(r2code "$previous")" '200'
has "and still refuses the one it dropped" "$(r2code "$dropped")" '401'
has "a restart keeps every principal, not only the owner's" \
  "$(AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$other AGENT_BUS_NAME=kept-other@srv1 "$D/agent-bus" status)" '"up"'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
# What the store keeps, it keeps to itself. See docs/09-setup.md#storage.
has "the credential file is that account's alone" "$(stat -c %a "$D/token")" '^600$'
# A credential that could not be written down is one a restart forgets, so
# it is not handed out either: the daemon says so instead.
mkdir -p "$D/ro" && cp "$D/token" "$D/ro/token"
mkdir -p "$D/ro-run"
"$D/agent-busd" -addr 127.0.0.1:$((PORT+5)) -socket "$D/ro-run/bus.sock" -token-file "$D/ro/token" -owner "$OWNER" -dump-file "$D/ro-run/dump.json" -dump-every 0 >"$D/daemon3.log" 2>&1 &
OPID=$!
ready "$D/ro-run/bus.sock" || echo "  WARNING: $D/ro-run/bus.sock never answered"
AGENT_BUS_ADDR=$D/ro-run/bus.sock AGENT_BUS_TOKEN=$TOKEN "$D/agent-bus" register unsaveable@srv1 --allow '*' >/dev/null || exit 1
chmod 0500 "$D/ro"
out=$(AGENT_BUS_ADDR=$D/ro-run/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" unsaveable@srv1 2>&1); rc=$?
bad_exit "a credential the store could not keep is not handed out" $rc
is_empty "and nothing that looks like one is printed" "$(printf '%s' "$out" | grep -o '^[0-9a-f]\{48\}$')"
chmod 0700 "$D/ro"
has "while the same ask succeeds once the store can be written" \
  "$(AGENT_BUS_ADDR=$D/ro-run/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" unsaveable@srv1)" '^[0-9a-f]\{48\}$'
kill $OPID 2>/dev/null; wait $OPID 2>/dev/null
# `start` runs until it is stopped, so this check leans on the refusal to end
# it. Under a mutant that allows it, it ran until the harness's own timeout
# and took the whole batch with it — hence the bound, and hence 124 counting
# as a failure of the check rather than the refusal it was looking for.
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$bob AGENT_BUS_NAME=bob@srv1 timeout 5 "$D/agent-bus" start alice-svc@srv1 --allow '*' --algo args /bin/echo 2>&1); rc=$?
if [ "$rc" -ne 0 ] && [ "$rc" -ne 124 ]; then
  echo "  ok   and starting a service you do not own is refused"; pass=$((pass+1))
else
  echo "  FAIL and starting a service you do not own is refused: exit $rc"; fail=$((fail+1))
fi
has "because the record is not his to take over" "$out" 'belongs to someone else'

sec "a record belongs to whoever published it"
users thief2@srv1 stranger2@srv1
# Publishing is open; changing is not. See docs/01-identity-and-roles.md#ownership.
ab owner@srv1 register owned@srv1 --allow '*' --descr "mine" --addr first:1 >/dev/null
out=$(ab thief2@srv1 register owned@srv1 --allow '*' --descr "stolen" --addr second:2 2>&1); rc=$?
bad_exit "somebody else cannot re-register it" $rc
has "and is told whose it is" "$out" "owned@srv1 is owner@srv1's"
has "the address it stated did not land" "$(ab nobody2@srv1 ls owned@srv1)" 'first:1'
has "nor the description" "$(ab nobody2@srv1 ls owned@srv1)" '"descr":"mine"'
has "its owner may still change it" \
  "$(ab owner@srv1 register owned@srv1 --allow '*' --descr "mine" --addr third:3 >/dev/null; ab nobody2@srv1 ls owned@srv1)" 'third:3'
has "and the record itself may refresh its own, as a service does on every start" \
  "$(ab owned@srv1 register owned@srv1 --allow '*' --descr "self" --addr third:3 >/dev/null; ab nobody2@srv1 ls owned@srv1)" '"descr":"self"'
has "while an existing user may publish a new name" \
  "$(ab stranger2@srv1 register brand-new@srv1 --allow '*' --descr "open" >/dev/null; ab nobody2@srv1 ls brand-new@srv1)" '"descr":"open"'

# Message-flow fixtures opt into sharing explicitly. The Go checks above
# separately pin empty-ACL restrictions, including restored records.
sec "register and ls"
users sender@srv1 someone@srv1 nobody@srv1
ab owner@srv1 register fixer@srv1 --allow '*' --kind agent --descr "fixes things" >/dev/null
ab owner@srv1 register asker@srv1 --allow '*' --kind agent >/dev/null
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
  # A pool is the exception, and asking is what makes it one. Either side
  # declining keeps the refusal: a reader that wants the inbox to itself
  # still gets it, which is the rule this replaces, not removes.
  has "a sharing reader beside one that wants the inbox to itself is refused too" \
    "$(ab fixer@srv1 consume --share --wait 1s 2>&1)" 'already has a reader'
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
  sleep 2   # the backgrounded client outlives its subshell; let its poll expire

  sec "several workers behind one name"
  # Competing consumers on a BACKLOG already worked. The rule bit on an
  # EMPTY inbox, which is the steady state of a worker pool: the readers
  # block first and the work arrives afterwards. Prefilling the queue is how
  # the first draft of this criterion passed without testing anything, so
  # nothing is sent until all three are provably blocked.
  ab owner@srv1 register pool@srv1 --allow '*' --kind generic >/dev/null
  waiting() { ab parf@localhost status | sed -n 's/.*"waiting":\([0-9]*\).*/\1/p'; }
  base=$(waiting)
  for i in 1 2 3; do
    abx pool@srv1 consume --share --wait 15s >"$D/pool.$i" 2>&1 &
    eval "P$i=\$!"
  done
  for _ in $(seq 1 100); do [ "$(( $(waiting) - base ))" -ge 3 ] && break; sleep 0.1; done
  blocked=$(( $(waiting) - base ))
  if [ "$blocked" -ge 3 ]; then echo "  ok   three workers wait on one empty inbox at once"; pass=$((pass+1));
  else echo "  FAIL three workers wait on one empty inbox at once: only $blocked blocked"; fail=$((fail+1)); fi
  for i in 1 2 3; do ab sender@srv1 send pool@srv1 "job-$i" >/dev/null; done
  wait $P1 $P2 $P3 2>/dev/null
  got=$(cat "$D"/pool.[123] | grep -o 'job-[123]' | sort | tr '\n' ' ')
  has "and the work reaches all three as it arrives" "$got" 'job-1 job-2 job-3'
  is_empty "with none of them refused as a second reader" \
    "$(grep -l 'already has a reader' "$D"/pool.[123] 2>/dev/null)"
  is_empty "and no job handed to two of them" \
    "$(cat "$D"/pool.[123] | grep -o 'job-[123]' | sort | uniq -d)"

else skipped=$((skipped+1)); fi
sec "pub/sub: a copy per subscriber, in the subscriber's own inbox"
users drive-by@srv1 nobody-here@srv1
ab owner@srv1 topic create news@srv1 --allow '*' --kind pubsub --descr "broadcast" >/dev/null
ab owner@srv1 register sub-a@srv1 --allow '*' --kind agent >/dev/null
ab owner@srv1 register sub-b@srv1 --allow '*' --kind agent >/dev/null
ab sub-a@srv1 subscribe news@srv1 >/dev/null
ab sub-b@srv1 subscribe news@srv1 >/dev/null
has "the topic says who subscribed" "$(ab owner@srv1 ls news@srv1)" '"subs":\["sub-a@srv1","sub-b@srv1"\]'
ab drive-by@srv1 publish --topic news@srv1 "to everyone" >/dev/null
# Both, not one: a fan-out that hands the message to whoever reads first is a
# queue topic, and that is the mode this is NOT.
has "one subscriber gets a copy" "$(ab sub-a@srv1 consume --wait 5s)" 'to everyone'
has "and so does the other, of the same publication" "$(ab sub-b@srv1 consume --wait 5s)" 'to everyone'
# The copy is addressed to the subscriber: it is in an inbox, and an inbox
# belongs to a name (docs/04-messaging.md#inbox-queues).
ab drive-by@srv1 publish --topic news@srv1 "addressed" >/dev/null
has "and the copy is addressed to the subscriber, not to the topic" \
  "$(ab sub-a@srv1 consume --wait 5s)" '"to":"sub-a@srv1"'
ab sub-b@srv1 consume --wait 5s >/dev/null
# The topic keeps nothing of its own — that is the whole difference from a
# queue topic, and "queued" on its record is where it would show.
is_empty "while the topic itself keeps nothing" \
  "$(ab owner@srv1 ls news@srv1 | grep -o '"queued":[1-9][0-9]*')"
has "though its publications are counted" "$(ab owner@srv1 ls news@srv1)" '"in":2'
# Nobody listening is not an error, and is not a message kept for later.
ab owner@srv1 topic create void@srv1 --allow '*' --kind pubsub >/dev/null
ok_exit "a publish with no subscribers is accepted" \
  "$(ab drive-by@srv1 publish --topic void@srv1 "into the void" >/dev/null 2>&1; echo $?)"
is_empty "and kept for nobody" "$(ab owner@srv1 ls void@srv1 | grep -o '"queued":[1-9][0-9]*')"
# Leaving stops the copies, and is not the same as never having joined.
ab sub-b@srv1 unsubscribe news@srv1 >/dev/null
ab drive-by@srv1 publish --topic news@srv1 "second round" >/dev/null
has "a subscriber that stayed still gets it" "$(ab sub-a@srv1 consume --wait 5s)" 'second round'
is_empty "and one that left gets nothing" "$(ab sub-b@srv1 consume --wait 1s)"
has "and the topic no longer names it" "$(ab owner@srv1 ls news@srv1)" '"subs":\["sub-a@srv1"\]'
# A subscriber has to own an inbox for the copy to land in, so it is a
# registered name like any receiver.
out=$(ab nobody-here@srv1 subscribe news@srv1 2>&1); rc=$?
bad_exit "a known user without an inbox cannot subscribe" $rc
has "and says to register it first" "$out" 'so its copies have somewhere to land'
# A queue topic that EXISTS, so the refusal is about its mode and not about
# the name being unknown.
ab owner@srv1 topic create work@srv1 --allow '*' --descr "a queue topic" >/dev/null
out=$(ab sub-a@srv1 subscribe work@srv1 2>&1); rc=$?
bad_exit "and a queue topic is not something to subscribe to" $rc
has "refused for its mode, not for being unknown" "$out" 'only a pubsub topic has subscribers'
# The ACL is the capability here, and it is asked at PUBLISH, not only at
# subscribe: access taken away has to stop the copies, or subscribing would
# be a way to go on reading a topic that stopped allowing you.
ab owner@srv1 topic create members@srv1 --kind pubsub --allow sub-a@srv1 >/dev/null
out=$(ab sub-b@srv1 subscribe members@srv1 2>&1); rc=$?
bad_exit "subscribing to a topic you may not see is refused" $rc
ab sub-a@srv1 subscribe members@srv1 >/dev/null
ab owner@srv1 publish --topic members@srv1 "members only" >/dev/null
has "while the one it allows receives it" "$(ab sub-a@srv1 consume --wait 5s)" 'members only'
ab owner@srv1 topic create members@srv1 --kind pubsub --allow owner@srv1 >/dev/null
has "the subscription survives the record being restated" "$(ab owner@srv1 ls members@srv1)" '"subs":\["sub-a@srv1"\]'
ab owner@srv1 publish --topic members@srv1 "still a member?" >/dev/null
is_empty "but a subscriber no longer allowed gets no more copies" \
  "$(ab sub-a@srv1 consume --wait 1s)"
# One subscriber cannot hold the topic hostage: its own bound applies to its
# own copy, and the others still get theirs.
ab owner@srv1 register full-sub@srv1 --allow '*' --kind generic --bound 1 >/dev/null
ab full-sub@srv1 subscribe news@srv1 >/dev/null
ab drive-by@srv1 publish --topic news@srv1 "fills it" >/dev/null
ok_exit "a publish a full subscriber cannot take still succeeds" \
  "$(ab drive-by@srv1 publish --topic news@srv1 "does not fit" >/dev/null 2>&1; echo $?)"
has "and the subscriber with room gets both" \
  "$(ab sub-a@srv1 consume --wait 5s; ab sub-a@srv1 consume --wait 5s)" 'does not fit'
has "while the copy that would not fit is counted as a loss" \
  "$(ab parf@localhost status)" '"dropped":[1-9]'
# And against the SUBSCRIBER, whose own bound refused it — not against the
# topic, which keeps nothing and so can lose nothing. The node total rises
# wherever it is charged, which is exactly why it cannot be the check.
has "and it is the subscriber's loss, its bound having refused it" \
  "$(ab owner@srv1 ls full-sub@srv1)" '"dropped":[1-9]'
is_empty "not the topic's, which keeps nothing to lose" \
  "$(ab owner@srv1 ls news@srv1 | grep -o '"dropped":')"

sec "a person can see what they hold a credential for, and never the credential"
users holder@srv1
ab holder@srv1 register holder@srv1 --allow '*' --descr "a person" >/dev/null
ab holder@srv1 register holder-svc@srv1 --allow '*' --descr "something they own" >/dev/null
ab holder@srv1 register holder-cold@srv1 --allow '*' --descr "owned, never asked for" >/dev/null
# Somebody else's name, registered here and given a credential here, so that
# "not in mine" is checked against a name that exists and holds one. A name
# nobody has a credential for would be dropped by the token store anyway,
# and the check would pass whatever the registry had handed it.
ab alice@srv1 register alice-held@srv1 --allow '*' --descr "hers, and it holds one" >/dev/null
tok alice-held@srv1 >/dev/null
# Owning a name is not holding a credential for it: one has to be asked for,
# which is what a service's owner does before starting it.
tok holder-svc@srv1 >/dev/null
HTOK=$(tok holder@srv1)
NAMES=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $HTOK" "http://unix/names")
has "their own name is there" "$NAMES" '"name":"holder@srv1"'
has "and a service they own, which has its own credential" "$NAMES" '"name":"holder-svc@srv1"'
# The view answers what you hold, not what you own: a name with no credential
# has nothing to show and is simply absent.
is_empty "but not one they own and have never got a credential for" \
  "$(printf '%s' "$NAMES" | grep -o 'holder-cold@srv1')"
# The whole point of a fingerprint: it names the credential without being
# one. A page that renders a token is a page that leaks one.
is_empty "and the token itself is nowhere in the answer" \
  "$(printf '%s' "$NAMES" | grep -o "$HTOK")"
has "a fingerprint stands in for it" "$NAMES" '"fingerprint":"[0-9a-f]\{16\}"'
has "with when it was issued" "$NAMES" '"issued":"20'
has "and when it was last used" "$NAMES" '"used":"20'
# Somebody else's names are not in your answer, and asking cannot be made to
# return them: the question is always about the caller.
is_empty "a name somebody else owns is not in mine" \
  "$(printf '%s' "$NAMES" | grep -o 'alice-held@srv1')"
OTHER=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $alice" "http://unix/names")
is_empty "and mine are not in theirs" "$(printf '%s' "$OTHER" | grep -o 'holder-svc@srv1')"
# A rotation is a new credential, so it has a new fingerprint: one that did
# not change would say the old token still works.
FP0=$(printf '%s' "$NAMES" | sed -n 's/.*"name":"holder@srv1","fingerprint":"\([0-9a-f]*\)".*/\1/p')
# Straight from the rotation, not through tok(), which caches to a file and
# would hand back the credential that has just been replaced.
RTOK=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" holder@srv1 --rotate 2>/dev/null)
FP1=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $RTOK" "http://unix/names" | sed -n 's/.*"name":"holder@srv1","fingerprint":"\([0-9a-f]*\)".*/\1/p')
has "a fingerprint was read before the rotation" "$FP0" '^[0-9a-f]\{16\}$'
has "and another after it" "$FP1" '^[0-9a-f]\{16\}$'
is_empty "a rotated credential has a different fingerprint" \
  "$(test "$FP0" = "$FP1" && echo same)"
# Issued is durable, last-used is this run's (docs/02-access.md#token-lifetime).
# A second daemon reading the same file has never seen the credential used,
# but it must still know when it was minted.
mkdir -p "$D/r3"
r3_up() {
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+14)) -socket "$D/r3/bus.sock" -token-file "$D/r3/token" \
    -owner "$OWNER" -dump-file "$D/r3/dump.json" -dump-every 0 >"$D/daemon3-$1.log" 2>&1 &
  NPID=$!
  ready "$D/r3/bus.sock" || echo "  WARNING: $D/r3/bus.sock never answered"
  R3TOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/r3/token")
}
r3_up first
# Registered and kept, for the same reason as the rotation checks above.
AGENT_BUS_ADDR=$D/r3/bus.sock AGENT_BUS_TOKEN=$R3TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" register kept-holder@srv1 --allow '*' >/dev/null
HTOK=$(AGENT_BUS_ADDR=$D/r3/bus.sock AGENT_BUS_TOKEN=$R3TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" kept-holder@srv1 --rotate 2>/dev/null)
kill $NPID 2>/dev/null; wait $NPID 2>/dev/null
r3_up second
has "and the issue date survives a restart" \
  "$(curl -s --unix-socket "$D/r3/bus.sock" -H "X-Agent-Bus-Token: $HTOK" "http://unix/names")" '"issued":"20'
# A principal with a date and no previous token writes a placeholder where
# the previous one would be, so the fields stay positional. It is a hole in
# the line, not a credential, and reading it back as one would let a single
# character authenticate as somebody who has never rotated.
has "and the placeholder for a missing previous is not a token" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r3/bus.sock" -H "X-Agent-Bus-Token: -" "http://unix/status")" '401'
kill $NPID 2>/dev/null; wait $NPID 2>/dev/null

sec "refusals are counted, and each under its own reason"
users caller@srv1 stranger@srv1
# A bus that is quiet and one that is refusing every call look identical
# from outside (docs/05-discovery.md#what-it-shows). Each kind is checked
# separately: one counter covering them all would say "something is wrong"
# and never which thing.
ab owner@srv1 register refused-by@srv1 --kind generic --allow owner@srv1 >/dev/null
n0=$(count credential)
tcode not-a-token /status >/dev/null 2>&1
delta "a credential that is not one is counted as that" 1 "$n0" "$(count credential)"
n0=$(count unknown)
ab caller@srv1 send nobody-at-all@srv1 "into the void" >/dev/null 2>&1
delta "an unknown receiver is counted as unknown" 1 "$n0" "$(count unknown)"
n0=$(count acl)
ab stranger@srv1 send refused-by@srv1 "let me in" >/dev/null 2>&1
delta "a call the ACL refuses is counted as acl" 1 "$n0" "$(count acl)"
n0=$(count malformed)
ab owner@srv1 register refused-by@srv1 --allow '*' --overflow nonsense >/dev/null 2>&1
delta "and what a caller simply got wrong is one reason, not many" 1 "$n0" "$(count malformed)"
# A refusal the daemon never made is not counted, and one kind is not
# another: without this every check above passes on a single counter that
# every refusal increments.
n0=$(count credential)
ab caller@srv1 send nobody-at-all@srv1 "again" >/dev/null 2>&1
delta "an unknown receiver is not also a bad credential" 0 "$n0" "$(count credential)"
# A refusal is not a failure: the daemon's own faults are a 500 and are
# deliberately not in here, or "how often am I refusing callers?" would be
# answered by a number that includes our bugs.
is_empty "nothing is counted under a reason that has not happened" \
  "$(ab parf@localhost status | grep -o '"second-reader":0\|"full":0\|"enrolment":0')"

sec "unknown receiver"
out=$(ab asker@srv1 send ghost@nowhere hi 2>&1); rc=$?
has "says no such receiver" "$out" 'no such receiver'; bad_exit "and exits non-zero" $rc

sec "a name that is not a name"
bad_exit "register without a realm" "$(ab asker@srv1 register no-realm --allow '*' >/dev/null 2>&1; echo $?)"

sec "one name, however it is spelled: trim, lower-case, ASCII"
ab owner@srv1 register '  PAD@Srv1  ' --allow '*' --kind agent >/dev/null
has "ls shows the canonical form" "$(ab asker@srv1 ls)" '"name":"pad@srv1"'
ab asker@srv1 send ' Pad@SRV1 ' --topic pad --tag p "padded name" >/dev/null
has "a padded, upper-case send reaches it" "$(ab pad@srv1 consume --topic pad --tag p --wait 3s)" 'padded name'
bad_exit "a non-ASCII name is refused" "$(ab asker@srv1 register 'pärf@srv1' --allow '*' >/dev/null 2>&1; echo $?)"

sec "call and ack: a service answers, and says it got the message first"
ab owner@srv1 register svc@srv1 --allow '*' --kind generic --descr "answers calls" >/dev/null
(
  msg=$(ab svc@srv1 consume --wait 10s)
  id=$(printf '%s' "$msg" | sed 's/.*"message_id":"\([^"]*\)".*/\1/')
  [ -n "$id" ] && ab svc@srv1 ack "$id" >/dev/null
  [ -n "$id" ] && ab svc@srv1 reply "$id" "the answer is 42" >/dev/null
) &
SPID=$!
ab caller@srv1 register caller@srv1 --allow '*' >/dev/null || exit 1
out=$(ab caller@srv1 call svc@srv1 --wait 15s "what is the answer?" 2>"$D/call.err"); rc=$?
wait $SPID 2>/dev/null
ok_exit "call returns" $rc
has "call gets the answer, not the receipt" "$out" 'the answer is 42'
has "the ack was seen and reported" "$(cat "$D/call.err")" 'ack from svc@srv1'
has "a call to nobody fails" "$(ab caller@srv1 call ghost@nowhere --wait 2s hi 2>&1)" 'no such receiver'

sec "topics: a publisher with no service record, a consumer that was down"
users reader@srv1
ab owner@srv1 topic create jobs@srv1 --allow '*' --descr "work queue" >/dev/null
has "the topic is in ls" "$(ab owner@srv1 ls --kind topic)" 'jobs@srv1'
ab drive-by@srv1 publish --topic jobs@srv1 "sweep the floor" >/dev/null
has "a consumer that was down still finds it" "$(ab reader@srv1 consume --inbox jobs@srv1 --wait 5s)" 'sweep the floor'
# Reading a topic and filtering your own inbox are different inboxes, so the
# check needs a message in each: asserting "nothing came back" passed happily
# with the rule mutated to read the topic in both cases.
ab someone@srv1 send caller@srv1 --topic jobs@srv1 --tag mine "PRIVATE" >/dev/null
ab drive-by@srv1 publish --topic jobs@srv1 "TOPIC" >/dev/null
has "a topic plus a tag filters my own inbox" "$(ab caller@srv1 consume --topic jobs@srv1 --tag mine --wait 3s)" 'PRIVATE'
has "an explicit inbox reads that inbox" "$(ab caller@srv1 consume --inbox jobs@srv1 --wait 3s)" 'TOPIC'
out=$(ab caller@srv1 consume --inbox jobz@srv1 --wait 1s 2>&1); rc=$?
bad_exit "an unknown explicit inbox is refused" $rc
has "and says which name" "$out" 'no inbox for jobz@srv1'

ab owner@srv1 register keeper@srv1 --allow '*' >/dev/null || exit 1
if slow; then
  sec "a call does not damage what it calls from"
  ab owner@srv1 register keeper@srv1 --allow '*' --kind generic --addr host:1234 --descr "KEEP ME" >/dev/null
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
  ab owner@srv1 register unheard@srv1 --allow '*' --kind generic >/dev/null
  out=$(ab caller@srv1 call unheard@srv1 --wait 5q "typo" 2>&1); rc=$?
  bad_exit "a bad --wait is refused" $rc
  is_empty "and refused before the message is sent" "$(ab unheard@srv1 consume --wait 1s)"
  # The status code, not the body: the accepted envelope echoes the field back,
  # so grepping for "receipt" passed with the check for it removed.
  has "a third receipt value is refused" \
    "$(post_code caller@srv1 /send '{"to":"svc@srv1","receipt":"maybe","body":"x"}')" '400'
  has "and the two real ones are not" \
    "$(post_code caller@srv1 /send '{"to":"svc@srv1","receipt":"done","re":"0","body":"x"}')" '200'

else skipped=$((skipped+1)); fi
sec "a shell script is a service"
users greeter@srv1 launcher@srv1
ab greeter@srv1 register greeter@srv1 --allow '*' >/dev/null || exit 1
for name in hello envelope defaulted; do ab owner@srv1 register "$name@srv1" --allow '*' >/dev/null || exit 1; done
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
abx hello@srv1 start hello@srv1 --allow '*' --algo args "$D/hello-world.sh" --descr "greets you" >"$D/start.log" 2>&1 &
HPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'greets you' && break; sleep 0.2; done
has "the script is registered and discoverable" "$(ab asker@srv1 ls)" 'greets you'
has "it answers a call" "$(ab greeter@srv1 call hello@srv1 --wait 15s world)" 'Hello world'
kill $HPID 2>/dev/null; wait $HPID 2>/dev/null

env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN XDG_RUNTIME_DIR=$D/discovery \
  "$D/agent-bus" start local-script@srv1 --allow '*' --algo args "$D/hello-world.sh" --descr "discovered local runner" >"$D/local-start.log" 2>&1 &
LOCALPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls local-script@srv1 2>/dev/null | grep -q 'discovered local runner' && break; sleep 0.1; done
has "a runner on a discovered user socket switches to its service identity" \
  "$(ab greeter@srv1 call local-script@srv1 --wait 5s discovery)" 'Hello discovery'
kill $LOCALPID 2>/dev/null; wait $LOCALPID 2>/dev/null

cat > "$D/envelope.sh" <<'SH'
#!/bin/sh
envelope=$(cat)
case "$envelope" in *payload*) got=yes ;; *) got=no ;; esac
echo "stdin=$got topic=$AGENT_BUS_TOPIC from=$AGENT_BUS_FROM"
SH
chmod +x "$D/envelope.sh"
abx envelope@srv1 start envelope@srv1 --allow '*' --algo json "$D/envelope.sh" -2 --descr "reads the envelope" >>"$D/start.log" 2>&1 &
SPID2=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'reads the envelope' && break; sleep 0.2; done
has "json gets the envelope on stdin and in the environment" \
  "$(ab greeter@srv1 call envelope@srv1 --topic t9 --wait 15s payload)" 'stdin=yes topic=t9 from=greeter@srv1'
kill $SPID2 2>/dev/null; wait $SPID2 2>/dev/null

# The default form, stated in docs/08-runner-role.md#script-services, is what
# a service gets when it says nothing.
abx defaulted@srv1 start defaulted@srv1 --allow '*' "$D/envelope.sh" --descr "says no form" >>"$D/start.log" 2>&1 &
NOFORMPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'says no form' && break; sleep 0.2; done
has "no form named is the json form" \
  "$(ab greeter@srv1 call defaulted@srv1 --topic t10 --wait 15s payload)" 'stdin=yes topic=t10'
kill $NOFORMPID 2>/dev/null; wait $NOFORMPID 2>/dev/null

# A service can be described as JSON on stdin instead of in flags; `-3`
# beside it still means three at a time.
echo "{\"name\":\"fromfile@srv1\",\"algo\":\"args\",\"script\":\"$D/hello-world.sh\",\"descr\":\"from a file\"}" \
  | abx launcher@srv1 start -3 --allow '*' >>"$D/start.log" 2>&1 &
JPID=$!
for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'from a file' && break; sleep 0.2; done
has "a service described as JSON on stdin" "$(ab greeter@srv1 call fromfile@srv1 --wait 15s again)" 'Hello again'
kill $JPID 2>/dev/null; wait $JPID 2>/dev/null
has "a script and its arguments must be one quoted word" \
  "$(ab launcher@srv1 start x@srv1 --allow '*' --algo args ./greet.sh loudly 2>&1)" 'one script'

if slow; then
  sec "reply-to: the answer goes where the request said, and a dead route is refused now"
  ab owner@srv1 register worker@srv1 --allow '*' --kind generic --descr "does work" >/dev/null
  ab owner@srv1 register third@srv1 --allow '*' --kind agent >/dev/null
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
  ok_exit "a known user without a service inbox may send" \
    "$(ab nobody@srv1 send worker@srv1 --topic rt --tag 3 "no answer wanted" >/dev/null 2>&1; echo $?)"
  has "and it arrives" "$(ab worker@srv1 consume --topic rt --tag 3 --wait 5s)" 'no answer wanted'

  # A script service answers the same way: the runner reads the route off the
  # envelope, so receipts and the answer all go to the third party.
  abx launcher@srv1 start relay@srv1 --allow '*' --algo args "$D/hello-world.sh" --descr "relays" >>"$D/start.log" 2>&1 &
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
  abx launcher@srv1 start slow@srv1 --allow '*' --algo args "$D/slow.sh" -1 --descr "slowly" >>"$D/start.log" 2>&1 &
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
  # with that name turns up. This is the property Legacy-V1's ephemeral channels did
  # not have — a dead channel took its results with it.
  ab launcher@srv1 register absent@srv1 --allow '*' --kind generic --descr "never started" >/dev/null
  ab caller@srv1 send absent@srv1 --topic w --tag 9 "waiting for whoever shows up" >/dev/null
  abx launcher@srv1 start absent@srv1 --allow '*' --algo args "$D/hello-world.sh" --descr "turned up late" >>"$D/start.log" 2>&1 &
  APID=$!
  # The ack comes first and is the sender's answer to "picked up, or lost?" —
  # the question Legacy-V1 needed a delivery journal for.
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
  ab launcher@srv1 register quiet@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start quiet@srv1 --allow '*' --algo args "$D/silent.sh" --descr "says nothing" >>"$D/start.log" 2>&1 &
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
  ab launcher@srv1 register loud@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start loud@srv1 --allow '*' --algo args "$D/hello-world.sh" --descr "answers" >>"$D/start.log" 2>&1 &
  LPID=$!
  for _ in $(seq 1 50); do ab asker@srv1 ls 2>/dev/null | grep -q 'answers' && break; sleep 0.2; done
  ab caller@srv1 send loud@srv1 --topic dn3 --tag 1 "out loud" >/dev/null
  ab caller@srv1 consume --topic dn3 --tag 1 --wait 15s >/dev/null
  has "a service that answers sends no done" \
    "$(ab caller@srv1 consume --topic dn3 --tag 1 --wait 15s)" 'Hello out loud'
  kill $LPID 2>/dev/null; wait $LPID 2>/dev/null

  # The verb itself: one function serves both receipts, so `done` must reach the
  # sender exactly as `ack` does.
  ab owner@srv1 register handy@srv1 --allow '*' --kind generic >/dev/null
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
  abx launcher@srv1 start stopper@srv1 --allow '*' --algo args "$D/quick.sh" --descr "stops" >>"$D/start.log" 2>&1 &
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

  # What the deadline is FOR: a service handed work nobody is waiting for any
  # more does not do it. Both messages are queued before the service starts,
  # so the only difference between them is the deadline — a runner that
  # ignores it runs both, and a queue TTL cannot be what stops the stale one
  # because neither message has one.
  ab launcher@srv1 register judge@srv1 --allow '*' --kind generic >/dev/null
  post_body caller@srv1 /send '{"to":"judge@srv1","topic":"lt","tag":"stale","wait":"1s","body":"stale"}' >/dev/null
  post_body caller@srv1 /send '{"to":"judge@srv1","topic":"lt","tag":"fresh","wait":"60s","body":"fresh"}' >/dev/null
  sleep 1.2
  abx launcher@srv1 start judge@srv1 --allow '*' --algo args "$D/quick.sh" --descr "reads deadlines" >>"$D/start.log" 2>&1 &
  JPID=$!
  ab caller@srv1 consume --topic lt --tag fresh --wait 15s >/dev/null   # the ack
  has "a request still inside its deadline is run" \
    "$(ab caller@srv1 consume --topic lt --tag fresh --wait 15s)" 'ran fresh'
  is_empty "and one whose caller gave up gets no ack and no answer" \
    "$(ab caller@srv1 consume --topic lt --tag stale --wait 2s)"
  has "and the service says it dropped it rather than failing quietly" \
    "$(cat "$D/start.log")" 'after its caller gave up'
  kill $JPID 2>/dev/null; wait $JPID 2>/dev/null

else skipped=$((skipped+1)); fi
if slow; then
  sec "a script service is confined, and can be stopped and read"
  # One script, three questions, asked through the bus like any other call —
  # so what is checked is the confinement of the process the runner actually
  # started, not of one this file started for the occasion.
  # The script lives in a directory that is NOT a parent of the work
  # directory. Sharing one parent made the script's own read-only bind carry
  # the work directory in with it, and "the work directory is bound in" then
  # held whether it was bound or merely listed as writable.
  mkdir -p "$D/bin"
  printf '#!/bin/sh\ncase "$1" in\n  write-here) touch ./inside && echo inside-ok ;;\n  write-out)  touch /etc/nope 2>&1 | head -1 ;;\n  net)        curl -s -m 2 -o /dev/null "http://127.0.0.1:'"$PORT"'/status" && echo reached || echo no-net ;;\n  env)        echo "from=$AGENT_BUS_FROM work=$AGENT_BUS_WORK" ;;\nesac\n' > "$D/bin/confined.sh"; chmod +x "$D/bin/confined.sh"
  note() { [ -s "$D/state/agent-bus/services/$1.json" ]; }
  waitnote() { for _ in $(seq 1 100); do note "$1" && return 0; sleep 0.1; done; return 1; }

  # Confinement is opted into, so a start that says nothing gets none — even
  # on a host that could have provided one
  # (docs/08-runner-role.md#sandboxing).
  ab launcher@srv1 register bare@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start bare@srv1 --allow '*' --algo args "$D/quick.sh" --descr "bare" >>"$D/bare.log" 2>&1 &
  BRPID=$!
  waitnote bare@srv1 || echo "  WARNING: bare@srv1 never left a note"
  has "a start that asks for no sandbox gets none" "$(cat "$D/bare.log")" 'sandbox off'
  kill $BRPID 2>/dev/null; wait $BRPID 2>/dev/null

  ab launcher@srv1 register confined@srv1 --allow '*' --kind generic >/dev/null
  # The HOST decides whether these run, not the service under test. Asking the
  # log whether it was sandboxed would let "confine nothing" skip its own
  # checks and survive, which is the guarded-block shape of a hollow check.
  if systemd-run --user --pipe --collect --quiet /bin/true >/dev/null 2>&1; then
    abx launcher@srv1 start confined@srv1 --allow '*' --algo args "$D/bin/confined.sh" --sandbox on --descr "confined" >>"$D/sbx.log" 2>&1 &
    CFPID=$!
    waitnote confined@srv1 || echo "  WARNING: confined@srv1 never left a note"
    has "the service says which sandbox it got" "$(cat "$D/sbx.log")" 'sandbox '
    has "and where it may write" "$(cat "$D/sbx.log")" 'work .*/work/confined@srv1'
    has "a host that can sandbox gives the service one" "$(cat "$D/sbx.log")" 'sandbox systemd-run'
    # Writing INSIDE has to pass beside the two refusals, or a child that
    # cannot run at all passes the whole set.
    has "a sandboxed script may write in its work directory" \
      "$(ab caller@srv1 call confined@srv1 --wait 20s write-here)" 'inside-ok'
    # And in THE work directory, not merely in one of that name inside its
    # own private /tmp: a write the host cannot see afterwards is a work
    # directory the service does not really have.
    ok_exit "and the file is there on the host afterwards" \
      "$(test -e "$D/state/agent-bus/work/confined@srv1/inside"; echo $?)"
    has "and may not write outside it" \
      "$(ab caller@srv1 call confined@srv1 --wait 20s write-out)" 'Read-only file system'
    has "and has no network unless it asked for one" \
      "$(ab caller@srv1 call confined@srv1 --wait 20s net)" 'no-net'
    # A confined child starts from the manager's environment and inherits
    # nothing of the runner's, so the envelope has to be stated or a script
    # that routes on it silently sees empty strings.
    has "and still finds the envelope in its environment" \
      "$(ab caller@srv1 call confined@srv1 --wait 20s env)" 'from=caller@srv1'
    has "and its own work directory" \
      "$(ab caller@srv1 call confined@srv1 --wait 20s env)" 'work=.*/work/confined@srv1'
    ab launcher@srv1 register netty@srv1 --allow '*' --kind generic >/dev/null
    abx launcher@srv1 start netty@srv1 --allow '*' --algo args "$D/bin/confined.sh" --network --descr "networked" >>"$D/net.log" 2>&1 &
    NTPID=$!
    waitnote netty@srv1 || echo "  WARNING: netty@srv1 never left a note"
    has "while one that did has one" \
      "$(ab caller@srv1 call netty@srv1 --wait 20s net)" 'reached'
    kill $NTPID 2>/dev/null; wait $NTPID 2>/dev/null
    kill $CFPID 2>/dev/null; wait $CFPID 2>/dev/null
  else
    echo "  WARNING: this host has no sandbox; the confinement checks are skipped"
    skipped=$((skipped+1))
  fi
  # Off is a setting, and asking for one the host cannot give is an error
  # rather than a quiet downgrade.
  ab launcher@srv1 register loose@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start loose@srv1 --allow '*' --algo args "$D/quick.sh" --sandbox off --descr "loose" >>"$D/loose.log" 2>&1 &
  LOPID=$!
  waitnote loose@srv1 || echo "  WARNING: loose@srv1 never left a note"
  has "a service asked to run unconfined says so out loud" "$(cat "$D/loose.log")" 'sandbox off'
  out=$(abt launcher@srv1 start loose@srv1 --allow '*' --algo args "$D/quick.sh" 2>&1); rc=$?
  bad_exit "starting a name already running here is refused" $rc
  has "and says which process holds it" "$out" 'already running here as pid'
  out=$(abt launcher@srv1 start askew@srv1 --allow '*' --algo args "$D/quick.sh" --sandbox maybe 2>&1); rc=$?
  bad_exit "a sandbox setting that is neither on nor off is refused" $rc
  has "and says which two settings there are" "$out" 'sandbox is on or off'

  sec "stop ends one service and leaves its siblings"
  ab launcher@srv1 register twin@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start twin@srv1 --allow '*' --algo args "$D/quick.sh" --descr "a twin" >>"$D/twin.log" 2>&1 &
  TWPID=$!
  waitnote twin@srv1 || echo "  WARNING: twin@srv1 never left a note"
  out=$(ab launcher@srv1 stop twin@srv1 2>&1); rc=$?
  ok_exit "stop exits 0" $rc
  has "and says which service it stopped" "$out" 'twin@srv1 stopped'
  is_empty "the process is gone when stop returns, not merely signalled" \
    "$(kill -0 $TWPID 2>/dev/null && echo still-there)"
  # The sibling is the control: one name is one process here, and a stop that
  # took the whole account with it would pass every check above.
  has "while the one beside it is still running" \
    "$(kill -0 $LOPID 2>/dev/null && echo yes)" 'yes'
  # Stopping is not unregistering: the name still owns its queue, which is
  # the whole point of a name-owned inbox.
  has "a stopped service is still registered" "$(ab asker@srv1 ls twin@srv1)" 'a twin'
  ab caller@srv1 send twin@srv1 "after the stop" >/dev/null
  has "and messages still wait in its queue" "$(ab twin@srv1 consume --wait 3s)" 'after the stop'
  has "logs shows what it wrote, after it has stopped" \
    "$(ab launcher@srv1 logs twin@srv1)" 'twin@srv1 is'
  has "and --lines bounds it" \
    "$(ab launcher@srv1 logs twin@srv1 --lines 1 | wc -l | tr -d ' ')" '^1$'
  out=$(ab launcher@srv1 stop twin@srv1 2>&1); rc=$?
  bad_exit "stopping something that is not running is an error" $rc
  has "and says so rather than pretending" "$out" 'not running from this account'
  out=$(ab launcher@srv1 logs never-ran@srv1 2>&1); rc=$?
  bad_exit "and so is asking for a log nothing wrote" $rc
  # A service killed outright leaves its note behind. A note is not a running
  # service — the process is the thing — so the stale one is reported as gone
  # and cleared, not offered as something to stop.
  ab launcher@srv1 register killed@srv1 --allow '*' --kind generic >/dev/null
  abx launcher@srv1 start killed@srv1 --allow '*' --algo args "$D/quick.sh" --descr "killed outright" >/dev/null 2>&1 &
  KLPID=$!
  waitnote killed@srv1 || echo "  WARNING: killed@srv1 never left a note"
  kill -9 $KLPID 2>/dev/null; wait $KLPID 2>/dev/null
  out=$(abt launcher@srv1 stop killed@srv1 2>&1); rc=$?
  bad_exit "a note with no process behind it is not a running service" $rc
  has "and says the note outlived the process" "$out" 'left a note but no process'
  ok_exit "and the stale note is cleared rather than left to mislead" \
    "$(test ! -e "$D/state/agent-bus/services/killed@srv1.json"; echo $?)"
  kill $LOPID 2>/dev/null; wait $LOPID 2>/dev/null

else skipped=$((skipped+1)); fi
sec "--follow, and the one refusal the daemon owes us"
ab owner@srv1 register follower@srv1 --allow '*' --kind agent >/dev/null
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
ab owner@srv1 register code-review/claude@rdvp --allow '*' --kind agent >/dev/null
ab owner@srv1 register code-review/claude-2@rdvp --allow '*' --kind agent >/dev/null
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
  "$(ab owner@srv1 register a/b/c@rdvp --allow '*' 2>&1)" 'a-z 0-9 . _ - + @ only'
# Only the template part is wrong here: the instance name and the realm are
# both fine, so nothing but the template's own check can refuse it.
has "a bad template part is refused on its own" \
  "$(ab owner@srv1 register -nope/claude@rdvp --allow '*' 2>&1)" 'bad template'
# The host is the LAST "@" part, so an instance may be named after the address
# it reads. This is the shape that breaks anything splitting on the first "@".
ab owner@srv1 register mail-sender/parf@comfi.com@host --allow '*' --kind agent >/dev/null
has "an instance name may be an address" \
  "$(ab owner@srv1 ls)" '"name":"mail-sender/parf@comfi.com@host"'
ab sender@srv1 send mail-sender/parf@comfi.com@host "read this one" >/dev/null
has "and routes on the whole name, host split off last" \
  "$(ab mail-sender/parf@comfi.com@host consume --wait 2s)" 'read this one'
has "a dangling at-sign is a typo, not a name" \
  "$(ab owner@srv1 register parf@@host --allow '*' 2>&1)" 'bad name'
ab owner@srv1 register mail-sender/parf+alerts@comfi.com@host --allow '*' --kind agent >/dev/null
has "plus-addressing is a legal instance name" \
  "$(ab owner@srv1 ls)" '"name":"mail-sender/parf+alerts@comfi.com@host"'
has "but the host does not take a plus" \
  "$(ab owner@srv1 register parf@ho+st --allow '*' 2>&1)" 'bad realm'
has "and the realm is a name, not a path" \
  "$(ab owner@srv1 register code-review/claude@rd/vp --allow '*' 2>&1)" 'bad realm'

sec "one name, one answer"
has "lookup answers about a single name" \
  "$(ab owner@srv1 register looked@srv1 --allow '*' --descr "here" >/dev/null; curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok owner@srv1)" "http://unix/lookup?name=looked@srv1")" '"descr":"here"'
has "and says so when there is none" \
  "$(code owner@srv1 "/lookup?name=absent-entirely@srv1")" '404'
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

sec "human listing"
ab owner@srv1 register human@srv1 --allow '*' --kind agent --descr $'first\nsecond\tthird' >/dev/null
ab owner@srv1 send human@srv1 queued >/dev/null
human=$(ab owner@srv1 ls -h --kind agent)
has "human listing has table columns" "$human" '^NAME  *KIND  *OWNER  *READER  *QUEUED  *DESCRIPTION$'
has "human listing shows queue and flattens description" "$human" '^human@srv1  *agent  *owner@srv1  *no  *1  *first second third$'
is_empty "human kind filter excludes service templates" "$(printf '%s\n' "$human" | grep '^looked@srv1 ')"
has "human single lookup works with flag after name" \
  "$(ab owner@srv1 ls human@srv1 -h)" '^human@srv1  *agent  *owner@srv1  *no  *1  *first second third$'
has "ordinary listing remains JSON" "$(ab owner@srv1 ls --kind agent)" '^\[.*"name":"human@srv1"'
has "empty human listing is explicit" "$(ab owner@srv1 ls -h --kind no-such-kind)" '^No matching records\.$'
human_error=$(ab owner@srv1 ls -h absent-entirely@srv1 2>&1)
bad_exit "human missing lookup fails" "$?"
has "human missing lookup reports the error" "$human_error" 'no such name'
ab owner@srv1 register human-external@srv1 --allow '*' --protocol http --addr http://localhost >/dev/null
has "external service has no bus-reader indicator" \
  "$(ab owner@srv1 ls -h human-external@srv1)" '^human-external@srv1  *generic  *owner@srv1  *-  *0'
ab human@srv1 consume --wait 0s >/dev/null
ab human@srv1 consume --wait 10s >"$D/human-reader" & HUMAN_READER=$!
for _ in $(seq 1 100); do
  human=$(ab owner@srv1 ls -h human@srv1)
  printf '%s\n' "$human" | grep -q '^human@srv1 *agent *owner@srv1 *yes ' && break
  sleep .02
done
has "human listing reflects a waiting reader" "$human" '^human@srv1  *agent  *owner@srv1  *yes  *0'
ab owner@srv1 send human@srv1 unblock >/dev/null
wait "$HUMAN_READER"

sec "unregister an idle address"
ab owner@srv1 register retired@srv1 --allow '*' --kind agent >/dev/null
ab retired@srv1 status >/dev/null # issue its credential before removing the address
out=$(ab stranger@srv1 unregister retired@srv1 2>&1); rc=$?
bad_exit "unregister rejects another owner" "$rc"
has "unregister explains ownership refusal" "$out" 'belongs to someone else'
ab owner@srv1 send retired@srv1 keep >/dev/null
out=$(ab owner@srv1 unregister retired@srv1 2>&1); rc=$?
bad_exit "unregister refuses queued messages" "$rc"
has "unregister explains the busy inbox" "$out" 'queued messages'
has "refused removal preserves the message" "$(ab retired@srv1 consume --wait 0s)" 'keep'
has "owner can unregister an idle address" "$(ab owner@srv1 unregister retired@srv1)" '^retired@srv1 unregistered$'
is_empty "unregistered address leaves the listing" "$(ab owner@srv1 ls | grep -o '"name":"retired@srv1"')"
has "unregistered address no longer accepts messages" "$(ab owner@srv1 send retired@srv1 late 2>&1)" 'no such name'
# The credential goes with the address. `tok` caches what it minted, so the
# cached one is exactly what a holder would still be presenting.
out=$(ab retired@srv1 status 2>&1); rc=$?
bad_exit "unregister takes the credential with the address" "$rc"
out=$(ab stranger@srv1 unregister retired@srv1 2>&1); rc=$?
bad_exit "unregister of an absent address fails" "$rc"
# A removed name is reserved for nobody: MVP does not protect it, and
# protecting it is R1.2's (Plans/R1.2/README.md#removed-names).
has "a removed name is free for whoever asks next" "$(ab stranger@srv1 register retired@srv1 --allow '*')" '"name":"retired@srv1"'
has "and belongs to whoever took it" "$(ab stranger@srv1 ls retired@srv1)" '"owner":"stranger@srv1"'
out=$(ab owner@srv1 register retired@srv1 --allow '*' 2>&1); rc=$?
bad_exit "so the previous owner cannot take it back" "$rc"
has "and the taker can remove it in turn" "$(ab stranger@srv1 unregister retired@srv1)" '^retired@srv1 unregistered$'
ab owner@srv1 register selfgone@srv1 --allow '*' --kind agent >/dev/null
has "a principal can unregister itself" "$(ab selfgone@srv1 unregister selfgone@srv1)" 'unregistered'

sec "no token, no serve"
# Stated as a rule, not sampled: every route the daemon exposes, refused
# both ways. A route added later without auth fails here.
for route in "GET /status" "POST /register" "POST /unregister" "GET /ls" "GET /lookup?name=x@h" \
             "POST /configure" "GET /config?name=x@h" "POST /send" "GET /consume?wait=0s"; do
  m=${route%% *}; path=${route#* }
  none=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/bus.sock" \
         -d '{}' "http://unix$path")
  wrong=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" --unix-socket "$D/bus.sock" \
          -H "X-Agent-Bus-Token: not-the-token" -d '{}' "http://unix$path")
  has "$m $path is not served without a token" "$none/$wrong" '401/401'
done

sec "a caller states a record, never what the daemon observes"
is_empty "a registration cannot claim a reader it does not have" \
  "$(post_body owner@srv1 /register '{"name":"probe@srv1","kind":"agent","reading":true,"queued":77}' | grep -o '"reading":true\|"queued":77')"
is_empty "and the claim does not survive into a listing" \
  "$(ab nobody@srv1 ls probe@srv1 | grep -o '"reading":true')"
is_empty "a registration cannot claim a call count" \
  "$(post_body owner@srv1 /register '{"name":"probe3@srv1","in":99,"out":99}' | grep -o '"in":99\|"out":99')"
is_empty "a registration cannot claim loss it did not suffer" \
  "$(post_body owner@srv1 /register '{"name":"probe4@srv1","dropped":42,"expired":42}' | grep -o '"dropped":42\|"expired":42')"
is_empty "nor an age for a queue it has not got" \
  "$(post_body owner@srv1 /register '{"name":"probe5@srv1","oldest":"99h"}' | grep -o 99h)"
is_empty "a registration cannot claim a configuration digest" \
  "$(post_body owner@srv1 /register '{"name":"probe2@srv1","config_sha":"forged"}' | grep -o forged)"
has "an unknown topic mode is refused by the daemon, not only the CLI" \
  "$(post_code owner@srv1 /register '{"name":"modey@srv1","kind":"topic","mode":"garbage"}')" '400'

sec "per-service call counters"
# A drained queue and one nobody ever wrote to both read as empty; these tell
# them apart. The control service proves the counters are the service's own
# and not a daemon-wide total copied onto every record.
ab owner@srv1 register counted@srv1 --allow '*' >/dev/null
ab owner@srv1 register control@srv1 --allow '*' >/dev/null
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

sec "a known user still needs an inbox before consuming"
users ghost@srv1
has "the daemon distinguishes a missing inbox from an unknown user" \
  "$(code ghost@srv1 "/consume?wait=0s")" '404'
has "while a registered name with an empty inbox is 204" \
  "$(ab launcher@srv1 register quiet@srv1 --allow '*' >/dev/null; code quiet@srv1 "/consume?wait=0s")" '204'

if slow; then
  sec "a listing says whether a call would reach anyone"
  # Being in the registry and being callable are different facts: "there is a
  # MySQL on db1:3306" registers fine and nothing on this bus answers for it.
  ab owner@srv1 register db.main@srv1 --allow '*' --protocol mysql --addr db1:3306 --descr "the main database" >/dev/null
  has "a registration can say how to call it" "$(ab nobody@srv1 ls db.main@srv1)" '"protocol":"mysql"'
  # Asserted on the record itself, not on an empty grep: an is_empty that a
  # failed query also satisfies is a check that passes with the daemon down.
  ab owner@srv1 register plain.svc@srv1 --allow '*' >/dev/null
  has "and an ordinary bus service says nothing, because there is nothing to say" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"name":"plain.svc@srv1"\|"protocol":')" '"name":"plain.svc@srv1"'
  is_empty "so no protocol comes back for it" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"protocol":[^,}]*')"
  has "a registered name nobody serves is not shown as read" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"reading":[a-z]*\|"name":"plain.svc@srv1"')" '"name":"plain.svc@srv1"'
  is_empty "and reading is absent rather than false" \
    "$(ab nobody@srv1 ls plain.svc@srv1 | grep -o '"reading":true')"
  # And with a reader attached, the same query says so.
  ab reader@srv1 register reader@srv1 --allow '*' >/dev/null
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
users nosy@srv1 thief@srv1 smuggler@srv1
# The configuration is arbitrary JSON and stays opaque; the one thing that
# matters to the bus is that it never shows up where it should not.
echo '{"model":"opus","depth":3}' | ab owner@srv1 service-template code-review/cfg@rdvp - >/dev/null
has "the configuration comes back as it went in, to the service" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '{"model":"opus","depth":3}'
has "but not to the owner who set it" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp 2>&1)" 'private to the service'
has "and that is a refusal, not a failure of ours" \
  "$(code owner@srv1 "/config?name=code-review/cfg@rdvp")" '403'

is_empty "a listing never carries it" \
  "$(ab owner@srv1 ls | grep -o '"config":[^,}]*')"
is_empty "and neither does the answer to setting one" \
  "$(echo '{"secret":"x"}' | ab owner@srv1 service-template echoes@srv1 - | grep -o '"config":[^,}]*')"
# What a query gets instead: enough to see that a write landed and that two
# are the same, without handing anyone the configuration.
sha=$(echo '{"secret":"x"}' | ab owner@srv1 service-template digested@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')
has "setting one answers with its digest" "$sha" '"config_sha":"'
has "sharing the configured record succeeds" "$(post_code owner@srv1 /manage '{"name":"digested@srv1","allow":["nobody@srv1"]}')" '200'
has "and a query carries the same digest" "$(ab nobody@srv1 ls)" "$sha"
has "the digest is sha256 of the stored bytes" "$sha" "$(printf '%s' '{"secret":"x"}' | sha256sum | cut -d' ' -f1)"
has "reformatting is not a change" \
  "$(printf '{ "secret" : "x" }' | ab owner@srv1 service-template reformatted@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')" "$sha"
has "a different configuration is a different digest" \
  "$(if [ "$(echo '{"secret":"y"}' | ab owner@srv1 service-template other@srv1 - | grep -o '"config_sha":"[a-f0-9]*"')" != "$sha" ]; then echo differs; fi)" 'differs'
is_empty "an unconfigured service has no digest at all" \
  "$(ab owner@srv1 register plain@srv1 --allow '*' | grep -o '"config_sha":[^,}]*')"
# A send is refused unless the receiver has a record, so this proves the
# record was created — the inbox itself is made lazily by the send either way.
has "configuring creates the service, so it can be sent to" \
  "$(ab owner@srv1 send code-review/cfg@rdvp "it exists" >/dev/null; ab code-review/cfg@rdvp consume --wait 2s)" 'it exists'

has "a stranger may not read it either" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp 2>&1)" 'private to the service'
# The text alone would still read right if every refusal collapsed to a 500.
has "and is refused as forbidden, not as our own fault" \
  "$(code nosy@srv1 "/config?name=code-review/cfg@rdvp")" '403'

has "and may not overwrite it" \
  "$(ab nosy@srv1 service-template code-review/cfg@rdvp '{"model":"theirs"}' 2>&1)" 'belongs to someone else'
has "the owner's configuration survived that" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

has "a configuration that is not JSON is refused" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp 'not json' 2>&1)" 'a configuration is JSON'
# The CLI refuses that one before it leaves; the daemon has to refuse it too,
# and the shape that reaches it is a body carrying no configuration at all.
has "and the daemon refuses an empty one on its own" \
  "$(post_code owner@srv1 /configure '{"name":"code-review/cfg@rdvp"}')" '400'
has "null is not a configuration either" \
  "$(ab owner@srv1 service-template code-review/cfg@rdvp null 2>&1)" 'null is the absence of one'

# A service registers itself on every start. That must refresh its
# description without destroying what it was configured with, and without
# handing the record to whoever registered last.
ab code-review/cfg@rdvp register code-review/cfg@rdvp --allow '*' --kind agent --descr "refreshed" >/dev/null
has "a re-registration keeps the configuration" \
  "$(ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

has "and still refreshes the description" \
  "$(ab owner@srv1 ls)" '"descr":"refreshed"'
ab thief@srv1 register code-review/cfg@rdvp --allow '*' --kind agent >/dev/null
# Ownership is observable through a WRITE: nobody can read a configuration
# but the service, so a read cannot tell us who owns the record.
has "and does not hand the record to whoever registered last" \
  "$(ab thief@srv1 service-template code-review/cfg@rdvp '{"mine":"now"}' 2>&1)" 'belongs to someone else'
has "a registration may not smuggle a configuration in" \
  "$(post_code thief@srv1 /register '{"name":"code-review/cfg@rdvp","config":{"evil":true}}' >/dev/null; ab code-review/cfg@rdvp service-template code-review/cfg@rdvp)" '"model":"opus"'

# On a name that does not exist yet there is no old configuration to keep, so
# this is the only shape that proves register drops the field rather than
# being saved by the preservation rule.
post_code smuggler@srv1 /register '{"name":"fresh@srv1","config":{"evil":true}}' >/dev/null
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
  sec "a backlog says how long its oldest message has been waiting"
  # A count alone cannot tell a busy queue from a stalled one. The age of
  # the head is what does (docs/05-discovery.md#what-it-shows).
  ab owner@srv1 register stalled@srv1 --allow '*' --kind generic >/dev/null
  is_empty "an inbox nobody wrote to has no oldest message" \
    "$(ab owner@srv1 ls stalled@srv1 | grep -o '"oldest"')"
  ab caller@srv1 send stalled@srv1 --topic old --tag 1 "sat here a while" >/dev/null
  sleep 3
  # A second, much younger message: with only one in the queue the newest IS
  # the oldest, and an age taken from the wrong end reads the same.
  ab caller@srv1 send stalled@srv1 --topic old --tag 2 "only just arrived" >/dev/null
  has "one with a backlog says how long the HEAD has waited" \
    "$(ab owner@srv1 ls stalled@srv1)" '"oldest":"[3-9]s"'
  # Drained, not merely read once: an age that outlives its queue would make
  # every emptied inbox look stuck for ever.
  ab stalled@srv1 consume --topic old --tag 1 --wait 3s >/dev/null
  ab stalled@srv1 consume --topic old --tag 2 --wait 3s >/dev/null
  drained=$(ab owner@srv1 ls stalled@srv1)
  # Answered, not merely silent: an is_empty check below would pass just as
  # well if `ls` had failed outright.
  has "the record is still answered once its queue drains" "$drained" 'stalled@srv1'
  is_empty "and stops saying how old its head is" "$(echo "$drained" | grep -o '"oldest"')"

  sec "ttl: a message outlives its worth, and nothing else is counted as that"
  ab owner@srv1 register keeper@srv1 --allow '*' --kind generic --ttl 1h >/dev/null
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
  ab owner@srv1 register brief@srv1 --allow '*' --kind generic --ttl 300ms >/dev/null
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
  ab owner@srv1 register ring.small@srv1 --allow '*' --kind generic --overflow ring --bound 2 >/dev/null
  for i in 1 2 3 4; do ab caller@srv1 send ring.small@srv1 --topic tt --tag r "m$i" >/dev/null; done
  delta "a ring drop is counted as dropped" 2 "$d0" "$(count dropped)"
  delta "and not as expired" 0 "$e2" "$(count expired)"
  # And against the NAME that lost it. A node total says something is
  # losing work; it cannot say which inbox to go and look at.
  has "the inbox that dropped them says so itself" \
    "$(ab owner@srv1 ls ring.small@srv1)" '"dropped":2'
  is_empty "while one that lost nothing says nothing" \
    "$(ab owner@srv1 ls keeper@srv1 | grep -o '"dropped":')"
  has "and expiry is the expired inbox's own, not the ring's" \
    "$(ab owner@srv1 ls keeper@srv1)" '"expired":[1-9]'
  is_empty "which the ring did not suffer" \
    "$(ab owner@srv1 ls ring.small@srv1 | grep -o '"expired":')"

  # The bound is the record's, not one number for the whole daemon.
  ab owner@srv1 register tiny@srv1 --allow '*' --kind generic --bound 1 >/dev/null
  ab caller@srv1 send tiny@srv1 --topic tt --tag s "first" >/dev/null
  out=$(ab caller@srv1 send tiny@srv1 --topic tt --tag s "second" 2>&1); rc=$?
  has "a record's own bound refuses the one past it" "$out" 'queue is full'
  bad_exit "and says so to the sender" $rc

  has "a ttl that is not a duration is refused" \
    "$(ab owner@srv1 register bad.ttl@srv1 --allow '*' --ttl soon 2>&1)" 'ttl is a duration'
  has "a bound that is not a count is refused" \
    "$(ab owner@srv1 register bad.bound@srv1 --allow '*' --bound plenty 2>&1)" 'positive number'
else skipped=$((skipped+1)); fi

if slow; then
  sec "a full queue: refuse by default, drop the oldest if asked"
  users flood@srv1
  ab owner@srv1 register sink@srv1 --allow '*' --kind generic >/dev/null
  ab owner@srv1 register ringy@srv1 --allow '*' --kind generic --overflow ring >/dev/null
  has "a record says what a full queue does, and refuses by default" \
    "$(ab owner@srv1 ls | grep -o '{[^}]*"name":"sink@srv1"[^}]*}')" '"overflow":"strict"'
  has "an overflow mode that is neither is refused" \
    "$(ab owner@srv1 register bad@srv1 --allow '*' --overflow maybe 2>&1)" 'overflow is strict or ring'
  # 1001 into a queue bounded at 1000, twice: strict must refuse the last one,
  # ring must swallow it and lose the first.
  d0=$(count dropped)
  for i in $(seq 0 1000); do ab flood@srv1 send sink@srv1 "msg-$i" >/dev/null 2>&1; done
  out=$(ab flood@srv1 send sink@srv1 "one too many" 2>&1); rc=$?
  bad_exit "strict refuses the send rather than lose a message" $rc
  has "and names the queue that is full" "$out" 'queue is full: sink@srv1'
  has "and says the sender is outrunning the reader, not that we broke" \
    "$(post_code flood@srv1 /send '{"to":"sink@srv1","body":"one more"}')" '429'
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
  if(u.pathname==='/status')return new Response('{\"you\":\"impatient@srv1\"}');
  return new Response('{}');}})" >/dev/null 2>&1 &
MPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$((PORT+3))/ls" && break; sleep 0.2; done
# This stalled-response fixture accepts any token; it is not the real bus.
out=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+3)) AGENT_BUS_TOKEN=fixture-token \
      timeout 5 "$D/agent-bus" call slow@srv1 --wait 500ms "are you there?" 2>&1); rc=$?
kill $MPID 2>/dev/null; wait $MPID 2>/dev/null
if [ "$rc" -eq 124 ]; then
  echo "  FAIL a stalled consume ends at --wait: it ran past the deadline"; fail=$((fail+1))
else
  echo "  ok   a stalled consume ends at --wait"; pass=$((pass+1))
fi
has "and says the message was accepted" "$out" 'do not resend'

sec "the caller's deadline travels to the service"
users asker2@srv1
# The wait belongs to the CALLER, so a service can see the answer is already
# too late and not do the work. The moment is the daemon's: a caller states a
# duration and never an instant, the same way it may not state its own name.
ab owner@srv1 register clockwatch@srv1 --allow '*' --kind generic >/dev/null
# A zero time is still a field, so "it has a deadline" is not the check — the
# year is. Matching the key alone passed with the stamping deleted.
has "a send states a wait and gets the moment it lands on" \
  "$(post_body caller@srv1 /send '{"to":"clockwatch@srv1","topic":"dl","tag":"1","wait":"30s","body":"in time"}')" '"deadline":"20'
has "and the service reads it off the envelope it consumed" \
  "$(ab clockwatch@srv1 consume --wait 5s)" '"deadline":"20'
has "a send with no wait carries no moment" \
  "$(post_body caller@srv1 /send '{"to":"clockwatch@srv1","topic":"dl","tag":"9","body":"whenever"}')" '"deadline":"0001-'
has "and a caller cannot state the moment itself" \
  "$(post_body caller@srv1 /send '{"to":"clockwatch@srv1","topic":"dl","tag":"2","deadline":"2099-01-01T00:00:00Z","body":"claimed"}')" '"deadline":"0001-'
ab clockwatch@srv1 consume --wait 5s >/dev/null
ab clockwatch@srv1 consume --wait 5s >/dev/null
has "a wait that is not a duration is refused" \
  "$(post_body caller@srv1 /send '{"to":"clockwatch@srv1","wait":"soon","body":"no"}')" 'a wait is a duration'
has "and so is one that ran out before it was sent" \
  "$(post_body caller@srv1 /send '{"to":"clockwatch@srv1","wait":"-5s","body":"no"}')" 'a wait is a duration'
# The CLI's own `call` is the caller that waits, so if it does not put its
# --wait on the wire the field travels for nobody.
ab clockwatch@srv1 register clockwatch@srv1 --allow '*' >/dev/null 2>&1
abx asker2@srv1 call clockwatch@srv1 --topic dl --tag cli --wait 9s "how long have I got" >/dev/null 2>&1 &
CLIPID=$!
has "the CLI's own call carries its --wait" \
  "$(ab clockwatch@srv1 consume --wait 5s)" '"deadline":"20'
kill $CLIPID 2>/dev/null; wait $CLIPID 2>/dev/null
# A deadline is not a TTL. The queue bounds a TTL and does not bound this, and
# the bus never acts on it: a message whose caller has gone is still delivered,
# because only the service knows whether the work is worth doing for anyone
# else. Losing that distinction is what makes this its own field.
post_body caller@srv1 /send '{"to":"clockwatch@srv1","topic":"dl","tag":"3","wait":"1s","body":"long gone"}' >/dev/null
sleep 1.2
has "a message past its deadline is still delivered, the judgement being the service's" \
  "$(ab clockwatch@srv1 consume --wait 5s)" 'long gone'
# And the TTL is still the receiver's, untouched by the caller's deadline: a
# generous wait does not keep a message the queue was told to drop.
ab owner@srv1 register brief@srv1 --allow '*' --kind generic --ttl 300ms >/dev/null
post_body caller@srv1 /send '{"to":"brief@srv1","wait":"60s","body":"kept briefly"}' >/dev/null
sleep 0.6
is_empty "while a long wait does not extend what the queue keeps" \
  "$(ab brief@srv1 consume --wait 1s)"

sec "enrolment: a key you hold, not a key you name"
# Its own daemon, because a vouched realm changes what registering means.
# See docs/01-identity-and-roles.md#registration.
mkdir -p "$D/enr"
ssh-keygen -q -t ed25519 -N '' -f "$D/enr/mine" >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$D/enr/theirs" >/dev/null
printf 'newbie %s\n' "$(cat "$D/enr/mine.pub")" > "$D/enr/keys"
printf 'squatter %s\n' "$(cat "$D/enr/mine.pub")" >> "$D/enr/keys"
"$D/agent-busd" -addr 127.0.0.1:$((PORT+10)) -socket "$D/enr/bus.sock" -token-file "$D/enr/token" \
  -owner "$OWNER" -dump-file "$D/enr/dump.json" -dump-every 0 -directory "vouched=$D/enr/keys" >"$D/enr/daemon.log" 2>&1 &
EPID=$!
ready "$D/enr/bus.sock" || echo "  WARNING: $D/enr/bus.sock never answered"
ETOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/enr/token")
AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_TOKEN=$ETOK "$D/agent-bus" register alice@srv1 --allow '*' >/dev/null || exit 1
eab() { AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_TOKEN=$ETOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
etok() { AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_TOKEN=$ETOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" "$@"; }
# A fourth argument is a body, and a body is what makes it a POST: passing an
# empty one turned every GET here into a 405 that read as a refusal.
ecode() {
  if [ $# -lt 4 ]; then
    curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" -H "X-Agent-Bus-Token: $2" "http://unix$3"
  else
    curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" -H "X-Agent-Bus-Token: $2" -d "$4" "http://unix$3"
  fi
}

out=$(eab register newbie@vouched --allow '*' 2>&1); rc=$?
bad_exit "a name in a vouched realm cannot simply be registered" $rc
has "and says it is enrolled instead" "$out" 'enrolled, not registered'
out=$(eab enrol nobody@vouched --key "$D/enr/mine" 2>&1); rc=$?
bad_exit "a login the realm publishes nothing for is not even challenged" $rc
has "and is told that, not left to fail at the signature" "$out" 'publishes no keys'
out=$(eab enrol newbie@vouched --key "$D/enr/theirs" 2>&1); rc=$?
bad_exit "enrolling with a key the realm does not publish for you is refused" $rc
has "and the refusal is the verifier's own words" "$out" 'Could not verify signature'
is_empty "nothing was registered by the attempt" "$(eab ls newbie@vouched 2>/dev/null | grep -o '"name"')"
out=$(eab enrol newbie@vouched --key "$D/enr/mine" 2>&1); rc=$?
ok_exit "enrolling with the key it does publish succeeds" $rc
has "the record it writes is its own owner's" "$out" '"owner":"newbie@vouched"'
has "and the credential comes with the proof" "$out" '"token":"[0-9a-f]\{48\}"'
NEWTOK=$(printf '%s' "$out" | sed -n 's/.*"token":"\([0-9a-f]*\)".*/\1/p')
has "which authenticates as that name" \
  "$(ecode newbie@vouched "$NEWTOK" /status)" '200'
has "somebody else cannot register over an enrolled name" \
  "$(ecode $OWNER "$ETOK" /register '{"name":"newbie@vouched","kind":"generic"}')" '403'
has "nor be handed its credential" \
  "$(ecode alice@srv1 "$(etok alice@srv1 2>/dev/null)" /token '{"name":"newbie@vouched"}')" '403'
# Enrolment is where a credential comes from, so it cannot want one first.
# See docs/02-access.md#proving-possession.
has "a newcomer with no credential at all is still challenged" \
  "$(curl -s --unix-socket "$D/enr/bus.sock" -d '{"name":"squatter@vouched"}' http://unix/enrol)" '"nonce"'
has "while everything else still wants one" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" http://unix/status)" '401'

# agent-bus-token is the program an ordinary user runs, and the only one they
# reach over SSH. See docs/09-setup.md#the-programs.
TOK1=$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus-token" alice@srv1)
has "the token program prints a credential and nothing else" "$TOK1" '^[0-9a-f]\{48\}$'
has "and asking twice is a read, not a rotation" \
  "$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus-token" alice@srv1)" "^$TOK1\$"
is_empty "while --rotate hands out a new one" \
  "$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus-token" alice@srv1 --rotate | grep -x "$TOK1")"
# The point of the key path: nothing to present, and no sshd in it anywhere.
KTOK=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+10)) "$D/agent-bus-token" squatter@vouched --key "$D/enr/mine" 2>&1)
has "a key is credential enough where the realm publishes it" "$KTOK" '^[0-9a-f]\{48\}$'
has "and that credential is that name's" "$(ecode squatter@vouched "$KTOK" /status)" '200'
out=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+10)) "$D/agent-bus-token" newbie@vouched --key "$D/enr/theirs" 2>&1); rc=$?
bad_exit "a key the realm does not publish for that name gets nothing" $rc
# Over SSH the line is the entitlement and the request is what was typed.
out=$(SSH_ORIGINAL_COMMAND="token $OWNER" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
  "$D/agent-bus-token" alice@srv1 2>&1); rc=$?
bad_exit "a key may only ask for the name its line names" $rc
has "and is told which name that is" "$out" 'may ask for alice@srv1'
# Against a reading taken now: --rotate above moved it.
NOW=$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus-token" alice@srv1)
has "while asking for that one is answered" \
  "$(SSH_ORIGINAL_COMMAND="token alice@srv1" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
     "$D/agent-bus-token" alice@srv1)" "^$NOW\$"
has "and a line that names nobody serves what was asked for" \
  "$(SSH_ORIGINAL_COMMAND="token alice@srv1" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
     "$D/agent-bus-token")" "^$NOW\$"

# A provider is an alternative to typing the record, not a dependency.
rm -f "$D/enr/keys"
has "an enrolled principal keeps working with the directory gone" \
  "$(ecode newbie@vouched "$NEWTOK" /status)" '200'
out=$(eab enrol squatter@vouched --key "$D/enr/mine" 2>&1); rc=$?
bad_exit "while nobody new can enrol while it is gone" $rc
kill $EPID 2>/dev/null; wait $EPID 2>/dev/null

sec "who may reach what: the service answers first, then master"
users acl-owner@srv1
# Two services alike in everything but the one flag, so what is being
# measured is the policy and not the request.
# See docs/02-access.md#acl.
ab acl-owner@srv1 register open-svc@srv1 --descr "takes master" --allow acl-owner@srv1 >/dev/null
ab acl-owner@srv1 register shut-svc@srv1 --descr "refuses master" --allow acl-owner@srv1 --no-master >/dev/null
ab acl-owner@srv1 register any-svc@srv1 --descr "open to all" --allow '*' >/dev/null
has "the daemon's owner holds master, so it reaches a service that takes it" \
  "$(post_code $OWNER /send '{"to":"open-svc@srv1","body":"by master"}')" '200'
has "and is refused by the one that refuses it" \
  "$(post_code $OWNER /send '{"to":"shut-svc@srv1","body":"by master"}')" '403'
has "with a refusal of its own, not a 404" \
  "$(post_body $OWNER /send '{"to":"shut-svc@srv1","body":"by master"}')" 'may not send to'
has "a principal on neither list is refused by both" \
  "$(post_code alice@srv1 /send '{"to":"open-svc@srv1","body":"by nobody"}')" '403'
has "and by the second as well" \
  "$(post_code alice@srv1 /send '{"to":"shut-svc@srv1","body":"by nobody"}')" '403'
has "while the owner of both still reaches them" \
  "$(post_code acl-owner@srv1 /send '{"to":"shut-svc@srv1","body":"mine"}')" '200'
has "and allow * means anyone who can authenticate" \
  "$(post_code alice@srv1 /send '{"to":"any-svc@srv1","body":"by anyone"}')" '200'
# A record is always its owner's and its own, list or no list: a service
# that could not reach what it registered would not survive its own start.
ab acl-owner@srv1 register terse-svc@srv1 --descr "lists somebody else" --allow alice@srv1 >/dev/null
has "an owner reaches its own service without being on its list" \
  "$(post_code acl-owner@srv1 /send '{"to":"terse-svc@srv1","body":"mine"}')" '200'
has "and the service sees itself in its own listing" \
  "$(ab terse-svc@srv1 ls)" 'lists somebody else'
is_empty "while a stranger sees neither" \
  "$(ab bob@srv1 ls | grep -o 'lists somebody else')"
# Seeing and using are the same question, so a service you may not use is
# not in your listing and does not answer a lookup either.
is_empty "a service that will not have you is not in your listing" \
  "$(ab alice@srv1 ls | grep -o 'refuses master')"
has "though it is in its owner's" "$(ab acl-owner@srv1 ls)" 'refuses master'
has "and a lookup of it says no such name" \
  "$(code alice@srv1 "/lookup?name=shut-svc@srv1")" '404'
has "while its owner gets the record" \
  "$(code acl-owner@srv1 "/lookup?name=shut-svc@srv1")" '200'
# One check per write verb: a shared guard passes the whole set while any
# one path is still open.
ab acl-owner@srv1 topic create shut-topic@srv1 --descr "not yours" --allow acl-owner@srv1 >/dev/null
has "publishing to a topic that will not have you is refused" \
  "$(post_code alice@srv1 /send '{"to":"shut-topic@srv1","topic":"anything","body":"by nobody"}')" '403'
has "and so is registering over its name" \
  "$(post_code alice@srv1 /register '{"name":"shut-svc@srv1","kind":"generic"}')" '403'

sec "the installer makes a service account, and it is not the installer's"
# The privileged step cannot run here, so what is checked is everything it
# would write: an install that puts the daemon under the installer's own
# account is the failure this wave exists to prevent.
# See docs/09-setup.md#the-two-accounts.
UNIT=$("$D/agent-bus-setup" --print-unit --owner "$OWNER" --exec /usr/local/bin/agent-busd)
has "the unit runs the daemon as an account of its own" "$UNIT" '^User=agent-busd$'
is_empty "never as root" "$(printf '%s' "$UNIT" | grep -x 'User=root')"
is_empty "and never as whoever ran setup" "$(printf '%s' "$UNIT" | grep -x "User=$(id -un)")"
has "the store lives under that account's home" "$UNIT" 'token-file /var/lib/agent-bus/daemon/token'
has "and so does the dump" "$UNIT" 'dump-file /var/lib/agent-bus/daemon/dump.json'
# The daemon is confined to its own home, so the runner's is out of reach even
# before either account's mode is consulted.
has "and it may write there and nowhere else" "$UNIT" '^ReadWritePaths=/var/lib/agent-bus/daemon$'
# systemd owns that directory's mode once StateDirectory names it, and re-applies
# its own default on every start. Left unstated, the 0700 setup made becomes
# 0755 the moment the daemon first runs, which no test passing a home would see.
has "and the mode setup made is the mode systemd keeps" "$UNIT" '^StateDirectoryMode=0700$'
has "it comes back after it dies" "$UNIT" '^Restart='
has "it is given one capability, not root" "$UNIT" '^AmbientCapabilities=CAP_CHOWN$'
has "and cannot pick up a second" "$UNIT" '^CapabilityBoundingSet=CAP_CHOWN$'
has "the installer still gets a socket of their own" "$UNIT" "[-]user $(id -un)=$OWNER"
# The runner reaches the local bus over a socket like any other account, so
# the daemon has to know it is one.
# See docs/09-setup.md#the-two-units.
has "and so does the runner, which is a client like anyone else" "$UNIT" "[-]user agent-bus-runner=runner@${OWNER#*@}"
out=$("$D/agent-bus-setup" --owner "$OWNER" 2>&1); rc=$?
bad_exit "setup without root refuses rather than half-installing" $rc
is_empty "and does not try the first step before finding that out" \
  "$(printf '%s' "$out" | grep -i useradd)"
has "and says that step is the only one that needs it" "$out" 'Nothing after this step'
has "and says the line to run instead of just refusing" "$out" 'sudo .*agent-bus-setup'
out=$("$D/agent-bus-setup" --dry-run --owner "$OWNER" 2>&1); rc=$?
ok_exit "a dry run needs nothing and says what it would do" $rc
has "naming the account" "$out" 'would create the system account agent-busd'
# Two accounts, because there are two secret domains and neither may read the
# other's: credentials are the daemon's, configurations the runner's.
# See docs/09-setup.md#the-two-accounts.
has "and the second one, which the daemon may not read" "$out" 'would create the system account agent-bus-runner'
has "the daemon's home is its own alone" "$out" "/var/lib/agent-bus/daemon agent-busd's own, 0700"
has "the runner's home is its own alone" "$out" "/var/lib/agent-bus/runner agent-bus-runner's own, 0700"
# What a service *is* holds no secret and is usually a checkout, so it is
# readable by anyone; what a host decided about it is not.
has "and what a service is, is readable by anyone" "$out" "/var/lib/agent-bus/service.d agent-bus-runner's own, 0755"
has "the unit" "$out" 'would write /etc/systemd/system/agent-busd.service'
has "and the start" "$out" 'would reload systemd'
has "and hands the first user to the program that owns that file" "$out" "would make $OWNER the first user"
out=$("$D/agent-bus-setup" --print-unit --owner parf 2>&1); rc=$?
bad_exit "an owner without a realm is refused before anything is written" $rc

sec "the admin program owns what the account owns"
# Everything an operator does to the account's files, and nothing a user
# needs. The home is stated, so this edits a directory of its own rather than
# a real install. See docs/09-setup.md#the-programs.
# Stating the home is what lets this run at all, and it is also what hides a
# real install's first question: which account? Setup creates one name and
# admin looks up another, and nothing that passes AGENT_BUS_HOME would ever
# notice. So ask admin, with the home unset, who it expects to be.
WANT=$("$D/agent-bus-setup" --dry-run --owner "$OWNER" 2>&1 | sed -n 's/.*would create the system account \([^ ]*\) with home .*/\1/p' | head -1)
if getent passwd "$WANT" >/dev/null; then
  # Ask which account admin selects, without making that account traverse
  # this run's private directory or reading a live install's files.
  mkdir -p "$D/admin-path"
  cat >"$D/admin-path/sudo" <<'SH'
#!/bin/sh
printf '%s\n' "$@"
SH
  chmod +x "$D/admin-path/sudo"
  out=$(env -u AGENT_BUS_HOME PATH="$D/admin-path:$PATH" "$D/agent-bus-admin" user list 2>&1); rc=$?
  ok_exit "with no home stated the admin program finds the account setup creates" $rc
  if [ "$(id -un)" != "$WANT" ]; then
    has "and selects that account for sudo" "$out" "^$WANT\$"
  fi
else
  out=$(env -u AGENT_BUS_HOME "$D/agent-bus-admin" user list 2>&1); rc=$?
  bad_exit "with no home stated the admin program wants the account setup creates" $rc
  has "and it is that account, not one nobody creates" "$out" "no $WANT account"
fi

mkdir -p "$D/adm"
ssh-keygen -q -t ed25519 -N '' -f "$D/adm/user" >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$D/adm/boss" >/dev/null
# The private home isolates files, not daemon calls. Bind the account socket too;
# otherwise provisioning discovers a real installation through ClientSocket.
adm() { AGENT_BUS_HOME=$D/adm AGENT_BUS_ADDR=$MINE "$D/agent-bus-admin" "$@"; }
adm user add smoke-admin-plain@srv1 "$D/adm/user.pub" >/dev/null
adm user add smoke-admin-chief@srv1 "$D/adm/boss.pub" --admin >/dev/null
ADM_USERS=$(curl -s --unix-socket "$MINE" http://unix/users)
has "admin provisioning creates its ordinary user in the fixture daemon" "$ADM_USERS" '"name":"smoke-admin-plain@srv1"'
has "admin provisioning creates its Administrator in the fixture daemon" "$ADM_USERS" '"name":"smoke-admin-chief@srv1"[^}]*"administrator":true'
has "admin provisioning preserves the fixture owner's administrative membership" \
  "$(curl -s --unix-socket "$MINE" http://unix/groups)" "\"@administrators\":\[[^]]*\"$OWNER\""
KEYS="$D/adm/.ssh/authorized_keys"
has "a user's key reaches the token program and nothing else" \
  "$(grep smoke-admin-plain@srv1 "$KEYS")" 'command="[^"]*agent-bus-token smoke-admin-plain@srv1"'
has "an operator's reaches the admin one" \
  "$(grep smoke-admin-chief@srv1 "$KEYS")" 'command="[^"]*agent-bus-admin smoke-admin-chief@srv1"'
has "and neither reaches a shell" "$(grep -c '^restrict,' "$KEYS")" '^2$'
has "the file is the account's alone" "$(stat -c %a "$KEYS")" '^600$'
has "and so is the directory sshd insists on" "$(stat -c %a "$(dirname "$KEYS")")" '^700$'
has "listing says who is there and what they reach" "$(adm user list)" 'smoke-admin-chief@srv1.*agent-bus-admin'
adm user add smoke-admin-plain@srv1 "$D/adm/boss.pub" >/dev/null
has "adding a name again replaces its key rather than adding a second" \
  "$(grep -c smoke-admin-plain@srv1 "$KEYS")" '^1$'
adm user remove smoke-admin-plain@srv1 >/dev/null
is_empty "removing takes the line away" "$(grep smoke-admin-plain@srv1 "$KEYS")"
out=$(adm user remove smoke-admin-plain@srv1 2>&1); rc=$?
bad_exit "and removing somebody who is not there says so" $rc
# The installer's key sits in the installer's home, and this program runs as
# agent-busd, which may not open it. So setup reads it as root and hands the
# bytes over; `-` is how they arrive. See docs/09-setup.md#the-programs.
adm user add smoke-admin-piped@srv1 - >/dev/null <"$D/adm/user.pub"
has "a key given on stdin lands like a key given by name" \
  "$(grep smoke-admin-piped@srv1 "$KEYS")" 'command="[^"]*agent-bus-token smoke-admin-piped@srv1"'
out=$(printf 'not a key at all\n' | adm user add smoke-admin-junk@srv1 - 2>&1); rc=$?
bad_exit "and what arrives that way is judged the same" $rc
has "named as the stdin it came from" "$out" 'the key on stdin is not a public key'
adm user remove smoke-admin-piped@srv1 >/dev/null
out=$(adm user add smoke-admin-oops@srv1 "$D/adm/user" 2>&1); rc=$?
bad_exit "a private key offered by mistake is refused" $rc
has "and named as what it is" "$out" 'is not a public key'
has "the operator still has a key of their own" "$(adm user list)" 'smoke-admin-chief@srv1'
out=$(adm sudo-make-me-a-sandwich 2>&1); rc=$?
bad_exit "a verb it does not have is refused, not guessed at" $rc

sec "the dashboard shows envelopes and no bodies"
# A separate process that speaks the API, because that is what it is in the
# design — see docs/05-discovery.md#dashboard.
SECRET="lemon-curd-9f3a"
ab parf@localhost register board-svc@srv1 --allow '*' --descr "watched by the board" >/dev/null
MSG=$(ab parf@localhost send board-svc@srv1 "$SECRET" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
FEED=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" "http://unix/recent")
has "the daemon remembers the envelope it routed" "$FEED" "$MSG"
has "and who it was between" "$FEED" '"to":"board-svc@srv1"'
is_empty "and never the body" "$(printf '%s' "$FEED" | grep -o "$SECRET")"
# The feed is each caller's own, not the operator's: what you were party to,
# and for master the node's. Refusing everyone but master made the exchanges
# view impossible to show anybody (docs/05-discovery.md#what-it-shows).
ALICE_FEED=$(code alice@srv1 /recent)
has "a caller who is not master may read the feed" "$ALICE_FEED" '200'
ab alice@srv1 register alice-svc@srv1 --allow '*' --descr "hers" >/dev/null
ab alice@srv1 register alice@srv1 --allow '*' --descr "alice herself" >/dev/null
# Party to it BOTH ways: one she sent, and one addressed to her. A feed that
# only ever showed what you sent would pass on the first alone.
ASENT=$(ab alice@srv1 send alice-svc@srv1 "her own traffic" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
AGOT=$(ab parf@localhost send alice@srv1 "addressed to her" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
MINE=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $alice" "http://unix/recent")
has "and sees an exchange they were party to" "$MINE" "$ASENT"
has "and one addressed to them, not only what they sent" "$MINE" "$AGOT"
# The control that matters: seeing your own is worthless if you also see
# everyone else's, which is what the master-only rule was protecting.
is_empty "but not one between two other names" \
  "$(printf '%s' "$MINE" | grep -o "$MSG")"
# Master's view is the NODE's, so it has to hold an exchange master was no
# part of — the earlier feed is all master's own traffic and would pass
# whatever the rule became.
NODE=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" "http://unix/recent")
has "while master sees an exchange between two other names" "$NODE" "$ASENT"
# The child is given NO credential and the shared socket rather than the
# owner's: on the owner's own socket every page it rendered would be the
# owner's, served to whoever connected, and a child with the owner's
# authority is a credential mint. See docs/05-discovery.md#signing-in.
WEB="http://127.0.0.1:$((PORT+9))"
AGENT_BUS_ADDR=$D/bus.sock \
  "$D/agent-bus-web" -addr 127.0.0.1:$((PORT+9)) >"$D/web.log" 2>&1 &
WPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "$WEB/" && break; sleep 0.1; done
ANON=$(curl -s "$WEB/")
has "an anonymous visitor gets the sign-in form" "$ANON" 'name=token'
has "the login header includes its inline bus logo" "$ANON" '<svg class=node-logo'
has "the login header names the node release" "$ANON" "AgentBus V$(cat internal/version/VERSION)"
has "the login header names the daemon owner" "$ANON" "owner: <code>$OWNER</code>"
has "the login header shows uptime" "$ANON" '<strong>uptime</strong>: [0-9][0-9a-z.]*</span>'
has "the login header reports sampled minute calls" "$ANON" '<strong>calls</strong>: minute: [0-9][0-9]*'
has "the login header reports sampled hour calls" "$ANON" 'hour: [0-9][0-9]*'
has "the login header reports total calls" "$ANON" 'total: [0-9][0-9]*'
lacks "observed spans stay out of the header" "$ANON" '(observed '
lacks "the footer has no About section or repeated version" "$ANON" 'About call counts\|Web <code>v'
lacks "OS and inbox readings are removed" "$ANON" 'Host load\|About load readings\|accepted /\|dequeued'
has "the login footer identifies the shared build once" "$ANON" 'Build: <code>'
lacks "identical builds are not repeated" "$ANON" 'Daemon build:\|Web build:'
# Node identity and sampled request counts are public; registry contents stay private.
# See docs/05-discovery.md#what-a-node-says-about-itself.
is_empty "and no records or registry totals" \
  "$(printf '%s' "$ANON" | grep -oE 'watched by the board|[0-9]+ records')"
has "health still answers an empty 200" \
  "$(curl -s -o /dev/null -w '%{http_code}:%{size_download}' "$WEB/healthz")" '^200:0$'
# One message however it failed: telling a bad credential from an unknown
# name is an oracle for which names exist.
has "a refused sign-in says one thing" \
  "$(curl -s -X POST -d 'token=nope' "$WEB/signin")" 'not accepted'
# Deliberately hostile: the child pointed at a socket that supplies the
# identity, where an empty credential would otherwise be answered. Nothing
# signs in, because a web child that can mint the owner is the whole thing
# the arrangement is against. See docs/05-discovery.md#signing-in.
AGENT_BUS_ADDR=$D/user-$ACCOUNT.sock \
  "$D/agent-bus-web" -addr 127.0.0.1:$((PORT+14)) >"$D/web-own.log" 2>&1 &
WOPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$((PORT+14))/" && break; sleep 0.1; done
OWNJAR=$D/web-own.jar; rm -f "$OWNJAR"
curl -s -c "$OWNJAR" -o /dev/null -X POST -d 'token=' "http://127.0.0.1:$((PORT+14))/signin"
is_empty "an empty credential signs nobody in, even on a socket that would answer" \
  "$(grep -o agent_bus_session "$OWNJAR")"
kill $WOPID 2>/dev/null; wait $WOPID 2>/dev/null
JAR=$D/web.jar; rm -f "$JAR"
has "signing in is a redirect, not a page" \
  "$(curl -s -c "$JAR" -o /dev/null -w '%{http_code}' -X POST -d "token=$TOKEN" "$WEB/signin")" '303'
has "and the browser carries a session" \
  "$(awk '/agent_bus_session/{print $NF}' "$JAR")" '^[0-9a-f]\{48\}$'
is_empty "which is never the token itself" "$(grep -o "$TOKEN" "$JAR")"
PAGE=$(curl -s -b "$JAR" "$WEB/")
has "the dashboard renders the envelope" "$(sect exchanges "$PAGE")" 'board-svc@srv1'
has "and the record it was for" "$PAGE" 'watched by the board'
is_empty "and no body reaches the page" "$(printf '%s' "$PAGE" | grep -o "$SECRET")"
# Two principals, one URL, different pages — and what tells them apart is a
# record master may not see, so it is the ACL answering and not the greeting.
OJAR=$D/web-owner.jar; rm -f "$OJAR"
curl -s -c "$OJAR" -o /dev/null -X POST -d "token=$(tok acl-owner@srv1)" "$WEB/signin"
has "a second principal gets their own page" "$(curl -s -b "$OJAR" "$WEB/")" 'refuses master'
is_empty "and master's page holds what master may not see" \
  "$(printf '%s' "$PAGE" | grep -o 'refuses master')"

# The views the MVP owes, each a reshape of what the bus already answered
# THIS caller: the ordering, the grouping and the late mark are the page's
# and nothing else is. See docs/05-discovery.md#what-it-shows.
# A backlog only ever grows where nobody is reading — an unfiltered reader
# is handed the message as it arrives — so a service with a reader is the
# control that says the list is not just every record over again.
ab owner@srv1 register busy-svc@srv1 --allow '*' --descr "somebody home" >/dev/null
ab busy-svc@srv1 consume --wait 10s >/dev/null 2>&1 &
busy_pid=$!
ab parf@localhost register slow-svc@srv1 --allow '*' --descr "nobody home" >/dev/null
ab parf@localhost send slow-svc@srv1 "waiting since before the rest" >/dev/null
# At its bound, which the DAEMON answers: a record that declares none takes
# the daemon's, and the page has no way to know what that is.
ab parf@localhost register tight-svc@srv1 --allow '*' --bound 2 --descr "a small queue" >/dev/null
ab parf@localhost send tight-svc@srv1 "one" >/dev/null
ab parf@localhost send tight-svc@srv1 "two" >/dev/null
# Loss against the name that suffered it, not against a node-wide total: a
# ring keeps the newest and the oldest is gone.
ab parf@localhost register lossy-svc@srv1 --allow '*' --bound 1 --overflow ring --descr "keeps the newest" >/dev/null
ab parf@localhost send lossy-svc@srv1 "first" >/dev/null
ab parf@localhost send lossy-svc@srv1 "second" >/dev/null
# A request and its ack, one exchange: same topic and tag, two envelopes.
# The asker is registered because a receipt goes back to it by name.
ab owner@srv1 register job-caller@srv1 --allow '*' --descr "asks for work" >/dev/null
ab owner@srv1 register work-svc@srv1 --allow '*' --descr "does the work" >/dev/null
ab job-caller@srv1 send work-svc@srv1 --topic job --tag 77 "do it" >/dev/null
( msg=$(ab work-svc@srv1 consume --wait 5s)
  id=$(printf '%s' "$msg" | sed 's/.*"message_id":"\([^"]*\)".*/\1/')
  [ -n "$id" ] && ab work-svc@srv1 ack "$id" ) >/dev/null 2>&1
# The newer backlog is the DEEPER one, so ordering by depth puts it first and
# ordering by age puts the stalled one first. Oldest first is the rule.
sleep 2
ab parf@localhost register burst-svc@srv1 --allow '*' --descr "a burst" >/dev/null
for n in 1 2 3; do ab parf@localhost send burst-svc@srv1 "burst $n" >/dev/null; done
VIEWS=$(curl -s -b "$JAR" "$WEB/")
STUCK=$(sect stuck "$VIEWS")
has "a backlog is listed as stuck" "$STUCK" 'slow-svc@srv1'
is_empty "and a service with a reader and nothing waiting is not" \
  "$(printf '%s' "$STUCK" | grep -o 'busy-svc@srv1')"
has "and the oldest backlog is ahead of a deeper, newer one" \
  "$(first_of "$STUCK" 'slow-svc@srv1' 'burst-svc@srv1')" 'slow-svc@srv1'
# "at capacity when observed", not "full": what was true at the moment of the
# read, never a prediction about the next send
# (Plans/MVP/web/data-dictionary.md#queue).
has "a queue at its bound is marked at capacity, and dated to the observation" \
  "$(printf '%s' "$STUCK" | grep 'tight-svc@srv1')" 'at capacity when observed'
is_empty "and one with room is not" \
  "$(printf '%s' "$STUCK" | grep 'slow-svc@srv1' | grep -o 'at capacity')"
ab parf@localhost send busy-svc@srv1 "go" >/dev/null
wait $busy_pid 2>/dev/null
LOSS=$(sect loss "$VIEWS")
has "loss is shown against the name that suffered it" "$LOSS" 'lossy-svc@srv1'
is_empty "and not against a name that lost nothing" \
  "$(printf '%s' "$LOSS" | grep -o 'slow-svc@srv1')"
XCH=$(sect exchanges "$VIEWS")
has "a request and its ack are one exchange, not two lines" \
  "$(printf '%s' "$XCH" | grep 'job' | grep '77')" '>2<'
NODE_VIEW=$(sect node "$VIEWS")
has "a signed-in caller is told how long the node has been up" "$NODE_VIEW" 'uptime [0-9]'
# Only the reasons that have happened: a reason with a zero beside it is
# noise on every other node. See docs/05-discovery.md#refusals.
tcode not-a-token /ls >/dev/null
has "and what the node is refusing" "$(sect node "$(curl -s -b "$JAR" "$WEB/")")" 'credential'
is_empty "and never a reason with a zero beside it" \
  "$(printf '%s' "$NODE_VIEW" | grep -oE '<code>[a-z-]+</code> 0([^0-9]|$)')"
# A fingerprint names a credential without being one, which is the whole
# reason a page may show it. See docs/02-access.md#token-lifetime.
FP=$(tbody "$TOKEN" /names | sed -n 's/.*"fingerprint":"\([0-9a-f]*\)".*/\1/p' | head -1)
has "a caller's own credential is named by its fingerprint" "$(sect names "$VIEWS")" "$FP"
is_empty "and the page carries no token anywhere on it" \
  "$(printf '%s' "$VIEWS" | grep -o "$TOKEN")"
# The session lives in the bus, so the child has nothing to lose. A session
# map inside the child passes every check above and fails this one.
kill $WPID 2>/dev/null; wait $WPID 2>/dev/null
AGENT_BUS_ADDR=$D/bus.sock \
  "$D/agent-bus-web" -addr 127.0.0.1:$((PORT+9)) >>"$D/web.log" 2>&1 &
WPID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "$WEB/" && break; sleep 0.1; done
has "a web child restarted mid-session logs nobody out" \
  "$(curl -s -b "$JAR" "$WEB/")" 'watched by the board'
curl -s -b "$JAR" -o /dev/null -X POST "$WEB/signout"
is_empty "and signing out ends the session" \
  "$(curl -s -b "$JAR" "$WEB/" | grep -o 'watched by the board')"
kill $WPID 2>/dev/null; wait $WPID 2>/dev/null
# HTTPS is not the default any more, but it is still there for somebody who
# has a certificate: supply one and the dashboard serves it. The suite makes
# its own for `localhost`, which resolves without asking anyone — which costs
# about a second, so this half is opt-in.
# See docs/05-discovery.md#where-it-listens.
if slow; then
mkdir -p "$D/tls"
openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost" \
  -keyout "$D/tls/key" -out "$D/tls/crt" >/dev/null 2>&1
AGENT_BUS_ADDR=$D/bus.sock \
  "$D/agent-bus-web" -addr "localhost:$((PORT+11))" -cert "$D/tls/crt" -key "$D/tls/key" >"$D/webtls.log" 2>&1 &
WTPID=$!
for _ in $(seq 1 50); do curl -sk -o /dev/null "https://localhost:$((PORT+11))/" && break; sleep 0.1; done
TJAR=$D/tls.jar; rm -f "$TJAR"
curl -sS --cacert "$D/tls/crt" -c "$TJAR" -o /dev/null -X POST -d "token=$TOKEN" \
  "https://localhost:$((PORT+11))/signin"
has "a session cookie made over https is marked secure" "$(cat "$TJAR")" 'TRUE.*agent_bus_session'
TLSPAGE=$(curl -sS --cacert "$D/tls/crt" -b "$TJAR" "https://localhost:$((PORT+11))/" 2>&1)
# curl verifies the chain and the hostname against that file alone — no -k —
# so an answer at all is the certificate being the one it was handed.
has "the dashboard answers https when it is given a certificate" \
  "$(sect exchanges "$TLSPAGE")" 'board-svc@srv1'
is_empty "with no body there either" "$(printf '%s' "$TLSPAGE" | grep -o "$SECRET")"
has "it says which scheme it came up on" "$(cat "$D/webtls.log")" 'https://'
kill $WTPID 2>/dev/null; wait $WTPID 2>/dev/null
# Every port is asked for on purpose now that none is a default, so a port it
# may not bind is an error rather than a quiet move to a neighbouring one.
has "a port it may not bind is an error, not a quiet move" \
  "$(AGENT_BUS_ADDR=$D/bus.sock \
     timeout 2 "$D/agent-bus-web" -addr 127.0.0.1:80 -cert "$D/tls/crt" -key "$D/tls/key" 2>&1)" \
  'permission denied'
# Without a pair it is plain HTTP on loopback. Use a suite-owned port so an
# installed dashboard can keep running while the suite checks this behavior.
out=$(AGENT_BUS_ADDR=$D/bus.sock \
  timeout 2 "$D/agent-bus-web" -addr "127.0.0.1:$((PORT+11))" -cert "$D/tls/absent" -key "$D/tls/absent" 2>&1)
has "a certificate that was asked for and is not there refuses to start" "$out" 'refusing to start'
lacks "and never comes up on plain http instead" "$out" "http://127.0.0.1:$((PORT+11))"
# Half a pair is the same ask, and the same refusal.
out=$(AGENT_BUS_ADDR=$D/bus.sock \
  timeout 2 "$D/agent-bus-web" -addr "127.0.0.1:$((PORT+11))" -cert "$D/tls/crt" 2>&1)
has "a certificate with no key is the same refusal" "$out" 'refusing to start'
fi

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
  ready "$D/dur/bus.sock" || echo "  WARNING: $D/dur/bus.sock never answered"
  DTOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/dur/token")
  [ "$1" = first ] || KTOK=$(AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" keeper@srv1 2>/dev/null)
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
dab register keeper@srv1 --allow '*' --descr "keeps things" >/dev/null
KTOK=$(AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" keeper@srv1)
dab send keeper@srv1 "before the restart" >/dev/null
dab send keeper@srv1 "also before it" >/dev/null
has "a reader took the first of them" "$(dkeep consume --wait 0s)" 'before the restart'
dur_down -TERM

dur_up second
has "the registry is back after a graceful stop" "$(dab ls keeper@srv1)" 'keeps things'
is_empty "and status says nothing about a stop that was clean" \
  "$(dab status | grep -o unclean)"
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
# The bus is the process that holds the state, so the bus is what dies here.
# By parent pid, never by a pattern: a pattern matches whatever else is on
# the host, up to and including whoever is running this.
kill -9 "$(pgrep -P "$DUR" | head -1)" 2>/dev/null
ready "$D/dur/bus.sock" || echo "  WARNING: the bus never came back"
has "a bus that dies is started again" "$(cat "$D/dur/third.log")" 'restarting in'
has "and the start that follows says the last one ended badly" \
  "$(cat "$D/dur/third.log")" 'did not stop cleanly'
has "and says from when it is missing traffic" "$(cat "$D/dur/third.log")" 'anything queued after'
# Logged once at start is not enough: whoever comes to look at a gap in the
# work arrives long after that line scrolled away.
has "and status still says so, not only the log" "$(dab status)" '"unclean":true'
is_empty "the message that died with the process is not invented back" \
  "$(dkeep consume --wait 0s 2>&1)"
dur_down -TERM

# Loss is state like the counters: a restart puts it back on the inbox that
# suffered it, and the node total is the sum of those rather than a second
# copy that could disagree.
dur_up fourth
dab register lossy@srv1 --allow '*' --overflow ring --bound 1 >/dev/null
dab send lossy@srv1 "first" >/dev/null; dab send lossy@srv1 "second" >/dev/null
has "an inbox that dropped something says so" "$(dab ls lossy@srv1)" '"dropped":1'
dur_down -TERM
dur_up fifth
has "and still says so after a restart" "$(dab ls lossy@srv1)" '"dropped":1'
has "while the node total is the sum of its inboxes" "$(dab status)" '"dropped":1'
dur_down -TERM

if slow; then
  # A message whose moment passed while the daemon was down is not worth
  # delivering late, and the reload is where that is decided.
  dur_up sixth
  dab send keeper@srv1 --ttl 2s "too late by the time you read this" >/dev/null
  dab send keeper@srv1 "still worth having" >/dev/null
  dur_down -TERM
  sleep 3
  dur_up seventh
  has "a message that outlived its ttl while down is not delivered" \
    "$(dkeep consume --wait 0s)" 'still worth having'
  is_empty "and it is the only one left" "$(dkeep consume --wait 0s 2>&1)"
  dur_down -TERM
fi

sec "a start clears out the records whose owner it does not know"
# The wreckage rule (docs/01-identity-and-roles.md#orphaned-records): a record
# whose owner the daemon knows nothing about answers for nobody, and a start
# takes it rather than leaving a name nobody can reach or reclaim. Its own
# daemon and its own store, because what is under test is what a start reads
# off disk.
#
# The store is edited between the stop and the start, and that is the only way
# to present one: a running daemon refuses every call that would make an
# orphan. This sweep is for a store written by an older daemon or by a hand.
mkdir -p "$D/orph"
orph_up() {
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+7)) -socket "$D/orph/bus.sock" -token-file "$D/orph/token" \
    -owner "$OWNER" -dump-file "$D/orph/dump.json" -dump-every 0 >"$D/orph/$1.log" 2>&1 &
  OPID=$!
  # The status is the caller's to act on, not a warning to scroll past: every
  # call below blocks on an unserved socket rather than failing, so a start
  # that never answered would hang the run instead of failing a check.
  ready "$D/orph/bus.sock" || return 1
  OTOK=$(awk -v n="$OWNER" '$1 == n { print $2 }' "$D/orph/token")
}
orph_down() { kill "$OPID" 2>/dev/null; wait "$OPID" 2>/dev/null; }
otok() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$OTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus-token" "$1" 2>/dev/null; }
oab() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$OTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
oas() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$(otok "$1") AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
# A fixture failure stops the run, and must not leave this section's daemon
# behind when it does: the trap at the top knows about the main one only.
opost() { curl -fsS --unix-socket "$D/orph/bus.sock" -H "X-Agent-Bus-Token: $OTOK" -d "$2" "http://unix$1" >/dev/null || { orph_down; exit 1; }; }
ocode() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/orph/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix/status"; }

# A body rather than a run of statements, so a start that never came up can
# leave it. Recording a failed assertion is not a guard: everything below
# waits on a bound socket nothing serves, and `ready` gives up only after a
# hundred one-second attempts — close to two minutes per call, not a
# failure. Three starts, so the guard has to leave the body rather than
# skip one statement.
orph_checks() {
  orph_up first || { echo "  FAIL the orphan fixture's daemon did not start"; fail=$((fail+1)); return 1; }
  for u in keeper@srv1 napping@srv1 barred@srv1; do
    opost /user "{\"name\":\"$u\",\"create\":true}"
  done
  # ghost@srv1 is a service, not a person: the edit below repoints what it owns,
  # and a name with a profile would still be known however its record reads.
  oab register ghost@srv1 --allow '*' >/dev/null
  oas ghost@srv1 register lost@srv1 --allow '*' --descr "answers for nobody" >/dev/null
  # Three deep, so one pass is not enough: taking chain-a is what orphans
  # chain-b, and taking chain-b is what orphans chain-c.
  oas ghost@srv1 register chain-a@srv1 --allow '*' >/dev/null
  oas chain-a@srv1 register chain-b@srv1 --allow '*' >/dev/null
  oas chain-b@srv1 register chain-c@srv1 --allow '*' >/dev/null
  # Positive controls, one per owner state the daemon still knows about, plus
  # one owned by the daemon owner — whose standing exists only after api.New,
  # so an earlier sweep would eat it.
  oas keeper@srv1 register steady@srv1 --allow '*' >/dev/null
  oas napping@srv1 register napped@srv1 --allow '*' >/dev/null
  oas barred@srv1 register barred-svc@srv1 --allow '*' >/dev/null
  oab register owner-svc@srv1 --allow '*' >/dev/null
  for s in lost@srv1 chain-a@srv1 steady@srv1 napped@srv1 barred-svc@srv1 owner-svc@srv1; do
    oab send "$s" "queued before the stop" >/dev/null
  done
  # Suspended after the queues exist: a suspended owner's service refuses
  # delivery, so seeding second would seed nothing and the controls would
  # survive with nothing to lose.
  opost /user/state '{"name":"napping@srv1","state":"paused"}'
  opost /user/state '{"name":"barred@srv1","state":"banned"}'
  LOSTTOK=$(otok lost@srv1)
  CHAINTOK=$(otok chain-a@srv1)
  has "the wreckage authenticates before the start that clears it" \
    "$(ocode "$LOSTTOK")$(ocode "$CHAINTOK")" '200200'
  orph_down

  # The hand edit. What ghost owns is repointed at a name that has neither a
  # profile nor a record; ghost's own record is owned by the daemon owner and is
  # not touched, so the name the edit points away from is itself a control.
  sed -i 's/"owner":"ghost@srv1"/"owner":"vanished@srv1"/g' "$D/orph/dump.json"
  has "the store now holds a record owned by a name nothing knows" \
    "$(cat "$D/orph/dump.json")" '"owner":"vanished@srv1"'
  # And every trace of the daemon owner as a *principal* goes with it: their
  # profile, and their line in the maintainers group, which a reload turns back
  # into a profile. The owner is made a registered user by the call that builds
  # the face, so a sweep running before that call finds every record of theirs
  # unowned and takes the lot. owner-svc@srv1 below is that check, and it says
  # nothing at all while the store hands the owner back before the face is up.
  # Two substitutions for the profile, because the order of the array is a map's
  # and not stable: dropping the trailing comma when the owner happens to be
  # last would leave JSON the start refuses to read, which checks nothing.
  OWNROW="{\"name\":\"$OWNER\",\"state\":\"[a-z]*\",\"kind\":\"\"}"
  sed -i "s|$OWNROW,||; s|,$OWNROW||; s|\"@administrators\":\[\"$OWNER\"\]|\"@administrators\":[]|" "$D/orph/dump.json"
  # Each read once and checked for shape first: a sed that matched nothing hands
  # back an empty string, which `lacks` accepts as proof of anything.
  EDUSERS=$(sed -n 's/.*\("Users":\[[^]]*\]\).*/\1/p' "$D/orph/dump.json")
  EDGROUPS=$(sed -n 's/.*\("Groups":{[^}]*}\).*/\1/p' "$D/orph/dump.json")
  has "the edited store still lists the users it kept" "$EDUSERS" 'keeper@srv1'
  has "and still has a maintainers group to read" "$EDGROUPS" '"@administrators":'
  lacks "but no profile for the daemon owner, as a hand-edited store may not" \
    "$EDUSERS" "$OWNER"
  lacks "nor a line in the group a reload would rebuild one from" "$EDGROUPS" "$OWNER"

  orph_up second || { echo "  FAIL the start over the edited store did not come up"; fail=$((fail+1)); return 1; }
  has "the start says how many it took" "$(cat "$D/orph/second.log")" 'deleted 4 services'
  REG=$(oab ls)
  lacks "the record whose owner nothing knows is gone" "$REG" 'lost@srv1'
  # All three, not just the first: a single pass leaves the tail of the chain
  # live, owned by a name that has just been deleted.
  lacks "and so is the chain it was holding up, to its end" "$REG" 'chain-c@srv1'
  lacks "not only the link the edit named" "$REG" 'chain-a@srv1'
  lacks "nor only the two above it" "$REG" 'chain-b@srv1'
  has "while the name the edit pointed away from is still there" "$REG" 'ghost@srv1'
  has "the credential that answered for the wreckage no longer authenticates" \
    "$(ocode "$LOSTTOK")" '401'
  # H.5.4's remaining clause, in the same run: a name that is not a registered
  # user and *owns services* is collected too, and this is the start that
  # deleted those services. One run, so the two sweeps cannot disagree about a
  # name — the credential goes because the record went, in that order.
  has "and so does one that was owning services when the start took it" \
    "$(ocode "$CHAINTOK")" '401'
  # Every control keeps its queue. A stopped owner is not a missing one.
  for s in steady@srv1 napped@srv1 barred-svc@srv1 owner-svc@srv1; do
    has "$s survives the start with its queue" "$(oab ls "$s")" '"queued":1'
  done
  # The freed name is reserved to nobody, and carries nothing across.
  oas keeper@srv1 register lost@srv1 --allow '*' --descr "somebody else's now" >/dev/null
  oas keeper@srv1 register chain-a@srv1 --allow '*' >/dev/null
  has "and the freed name registers to somebody else" "$(oab ls lost@srv1)" "somebody else's now"
  is_empty "who is handed none of what was queued for the old one" \
    "$(oas lost@srv1 consume --wait 0s 2>&1)"
  # And the old bytes are refused NOW, with the name registered again. The 401s
  # above prove nothing on their own: a name with no record is refused at the
  # gate whether or not its credential was ever dropped, so a Forget that never
  # ran, or that failed its write, passes them. This is the question they were
  # meant to ask — does the previous holder still authenticate as the name
  # somebody else now owns.
  has "and the credential the old holder kept does not answer for the new one" \
    "$(ocode "$LOSTTOK")" '401'
  has "nor does the one that was owning services" "$(ocode "$CHAINTOK")" '401'
  # The snapshot this start wrote at line one predates the sweep. Read off disk
  # rather than through a second restart, because a graceful stop would write a
  # clean dump either way and prove nothing about the save that follows the
  # sweep: this is what a start that then dies leaves behind.
  #
  # The positive half comes first and is not optional. `lacks` on an empty or
  # unparseable file passes for every absence there is, so a save that wrote
  # nothing at all would satisfy the three checks below on its own.
  # Taken before anything stops, because a graceful stop writes its own clean
  # dump over this one and would prove nothing about the save that follows the
  # sweep. These are the bytes a start that then died would have left.
  cp "$D/orph/dump.json" "$D/orph/after-sweep.json"
  STORE=$(cat "$D/orph/after-sweep.json")
  has "the records that survived are in the snapshot the sweep wrote" "$STORE" '"name":"owner-svc@srv1"'
  has "and their queued work is in it" "$STORE" '"to":"steady@srv1","body":"queued before the stop"'
  lacks "the store on disk is rewritten, so a start that dies repeats nothing" \
    "$STORE" 'chain-c@srv1'
  lacks "and the work that was queued for it is not in it either" "$STORE" '"to":"chain-a@srv1"'
  orph_down

  # Grepping those bytes cannot say they are a snapshot: every `lacks` above
  # passes against a file that is empty, truncated or not JSON at all. So a
  # third start reads them, and the daemon is the parser — it refuses to start
  # on a dump it cannot decode, and `ready` is what noticed. Started from the
  # copy rather than from whatever the stop above wrote.
  cp "$D/orph/after-sweep.json" "$D/orph/dump.json"
  if orph_up third; then
    THIRD=$(oab ls)
    has "a third start reads that snapshot whole" "$THIRD" '"name":"owner-svc@srv1"'
    has "with the queue that survived still on it" "$(oab ls steady@srv1)" '"queued":1'
    has "and the work still in it" "$(oas steady@srv1 consume --wait 0s)" 'queued before the stop'
    lacks "and does not find the wreckage a second time" "$THIRD" 'chain-c@srv1'
  else
    echo "  FAIL a third start reads that snapshot whole: it never answered"; fail=$((fail+1))
  fi
}
orph_checks
orph_down

sec "the supervisor holds the sockets, and the bus serves them"
# One binary, two roles. The process that may chown a socket never serves a
# request; the process that serves is handed listeners that already exist and
# could not make one. See docs/11-processes.md#the-rule.
mkdir -p "$D/sup"
# The production setup unit supplies the delegated subgroup used by web-only
# limits. This development fixture uses an explicit transient user unit with
# the same construction; running the binary directly would correctly leave
# web down rather than silently unbounded.
SUPUNIT=agent-bus-smoke-$BASHPID
systemd-run --user --unit="$SUPUNIT" --collect --wait --pipe --quiet \
  --working-directory="$(pwd)" --setenv=AGENT_BUS_WEB_ADDR=127.0.0.1:$((PORT+12)) \
  --setenv=AGENT_BUS_WEB_USER_DELEGATION=1 \
  -p Delegate=cpu -p Delegate=memory -p Delegate=pids -p DelegateSubgroup=supervisor \
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+13)) -socket "$D/sup/bus.sock" -token-file "$D/sup/token" \
  -owner "$OWNER" -dump-file "$D/sup/dump.json" -dump-every 0 -web >"$D/sup/log" 2>&1 &
SUPRUN=$!
for _ in $(seq 1 100); do
  SUP=$(systemctl --user show "$SUPUNIT" -p MainPID --value 2>/dev/null)
  [ "${SUP:-0}" -gt 0 ] && break
  sleep 0.05
done
ready "$D/sup/bus.sock" || echo "  WARNING: $D/sup/bus.sock never answered"
# By parent pid throughout: a pattern would match anything else on the host.
has "the daemon is a supervisor and its children" "$(pgrep -P "$SUP" | wc -l)" '^2$'
# Bubblewrap owns the web's PID namespace and reaper. Inspect the descendant
# tree, rather than mistaking its wrapper for the renderer or losing orphans.
descendants() {
  local child
  for child in $(pgrep -P "$1"); do echo "$child"; descendants "$child"; done
}
for _ in $(seq 1 50); do
  WEBPID=$(ps -o pid=,comm= -p $(descendants "$SUP" | paste -sd, -) | awk '$2 == "agent-bus-web" {print $1}')
  [ -n "$WEBPID" ] && break
  sleep 0.1
done
has "and the dashboard is one of them, not something it embeds" \
  "$(ps -o comm= -p "$WEBPID")" 'agent-bus-web'
is_empty "the dashboard carries no token of its own" \
  "$(tr '\0' '\n' < /proc/$WEBPID/environ | grep AGENT_BUS_TOKEN)"
# And not the owner's socket either, which is the half a missing token does
# not cover: on that socket it would be the owner without a credential at all.
# See docs/05-discovery.md#signing-in.
has "and reaches the bus over the shared socket, not the owner's" \
  "$(tr '\0' '\n' < /proc/$WEBPID/environ | grep AGENT_BUS_ADDR)" '^AGENT_BUS_ADDR=/bus.sock$'
AGENT_BUS_ADDR=$D/sup/user-$ACCOUNT.sock "$D/agent-bus" register sup-svc@srv1 --allow '*' \
  --descr "seen through the supervisor" >/dev/null
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$((PORT+12))/" && break; sleep 0.1; done
# The record is there and the page still will not say so: a supervised child
# has no more authority than one started by hand.
is_empty "so it names nobody until somebody signs in" \
  "$(curl -s "http://127.0.0.1:$((PORT+12))/" | grep -o 'seen through the supervisor')"
SJAR=$D/sup/jar; rm -f "$SJAR"
curl -s -c "$SJAR" -o /dev/null -X POST -d "token=$(awk '{print $2}' "$D/sup/token" | head -1)" \
  "http://127.0.0.1:$((PORT+12))/signin"
has "and what a signed-in caller sees came from the bus behind it" \
  "$(curl -s -b "$SJAR" "http://127.0.0.1:$((PORT+12))/")" 'seen through the supervisor'
BUSPID=$(pgrep -P "$SUP" -x agent-busd)
INODE=$(stat -c %i "$D/sup/user-$ACCOUNT.sock")
kill -9 "$BUSPID" 2>/dev/null
ready "$D/sup/bus.sock" || echo "  WARNING: the bus never came back"
has "a bus that dies is replaced, and the supervisor is untouched" \
  "$(ps -o pid= -p "$SUP" | tr -d ' ')" "^$SUP\$"
NEWBUS=$(pgrep -P "$SUP" -x agent-busd)
has "and it is a different process than the one that died" \
  "$([ -n "$NEWBUS" ] && [ "$NEWBUS" != "$BUSPID" ] && echo yes)" 'yes'
has "while the socket is the same file, because the fd was handed over" \
  "$(stat -c %i "$D/sup/user-$ACCOUNT.sock")" "^$INODE\$"
# Ask after the pids themselves, not after the supervisor's children: an
# orphan is reparented to init the moment its parent dies, so "it has no
# children" is true of a dead process however badly it left.
KIDS=$(descendants "$SUP" | tr '\n' ' ')
kill -TERM "$SUP" 2>/dev/null; wait "$SUPRUN" 2>/dev/null
SUPUNIT=""
gone() { for _ in $(seq 1 40); do [ -z "$(ps -o pid= -p $1 2>/dev/null)" ] && break; sleep 0.1; done
  ps -o pid= -p $1 2>/dev/null; }
is_empty "stopping the supervisor stops the children" "$(gone "$KIDS")"
# Asked, not killed: only a bus that was told to stop writes a clean dump, so
# this is where a stop that was not passed on shows up.
has "and asks them, so the bus writes what it holds on the way out" \
  "$(cat "$D/sup/dump.json")" '"Clean":true'
is_empty "and takes the sockets it made with it" "$(ls "$D/sup/"*.sock 2>/dev/null)"
# A supervisor that is killed outright cannot tidy up, so the kernel does it:
# an orphaned bus would keep the listeners and the next start would find the
# address in use.
AGENT_BUS_WEB_ADDR=127.0.0.1:$((PORT+12)) \
  "$D/agent-busd" -addr 127.0.0.1:$((PORT+13)) -socket "$D/sup/bus.sock" -token-file "$D/sup/token" \
  -owner "$OWNER" -dump-file "$D/sup/dump.json" -dump-every 0 >"$D/sup/log2" 2>&1 &
SUP2=$!
ready "$D/sup/bus.sock" || echo "  WARNING: $D/sup/bus.sock never answered the second time"
KIDS2=$(pgrep -P "$SUP2" | tr '\n' ' ')
kill -9 "$SUP2" 2>/dev/null; wait "$SUP2" 2>/dev/null
LEFT=$(gone "$KIDS2")
is_empty "a supervisor killed outright leaves no bus behind" "$LEFT"
# A build that does leave one must not leave it for the next run, holding a
# port. By the pids taken above, never by a pattern.
[ -n "$LEFT" ] && kill -9 $LEFT 2>/dev/null

if slow; then
  sec "the MCP face"
  # bun is not optional: the MCP face is part of the required minimum
  # (docs/00-overview.md), so a host without it fails rather than passing green.
  if ! command -v bun >/dev/null 2>&1; then
    echo "  FAIL bun is not installed; the MCP face cannot be checked"
    fail=$((fail + 1))
  else
    # The shared JSON-RPC plumbing, driven directly: the harnesses below only
    # ever have one request in flight and never split a line across chunks, so
    # they leave most of rpc.ts unwatched (mcp/rpc.test.ts says why).
    out=$(cd mcp && timeout 60 bun test rpc.test.ts messages.test.ts ../launchers/local.test.ts ../launchers/terminal.test.ts ../launchers/runtime-auth.test.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "rpc unit tests" $rc

    # each harness runs its own peer in-process, so there is no start-order race
    # These harnesses ask for credentials before their first record refresh.
    # Provision their service identities explicitly; no mint creates a name.
    for name in mcp.session peer peer.third pusher push.session; do
      ab "$OWNER" register "$name@srv1" --allow '*' >/dev/null || exit 1
    done
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

    out=$(timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
          AGENT_BUS_NAME=$OWNER LAUNCHER_BUILD=$D TEST_MAPPED_SOCKET=$D/user-$(id -un).sock bun run launchers/smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "installed launcher smoke" $rc

    out=$(timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
          AGENT_BUS_NAME=$OWNER LAUNCHER_BUILD=$D bun run launchers/rename-smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "coordinated launcher rename" $rc
  fi

else skipped=$((skipped+1)); fi

sec "the run stops what it started"
# $DPID is what the exit trap kills, and this file spawns dozens of
# short-lived things beside it. A section that reuses the name leaves the
# daemon running, holding the port, and the *next* run is the one that fails
# — so the last thing asked is whether the pid the trap holds is still ours.
has "the daemon the trap will stop is the one this run started" \
  "$(kill -0 "$DPID" 2>/dev/null && echo yes)" 'yes'

sec "end"
echo; echo "passed $pass, failed $fail"
if [ "$skipped" -gt 0 ]; then
  echo "SKIPPED $skipped slow sections and the race detector — this is NOT a full run."
  echo "         run ./smoke.sh --slow before believing a change."
fi
echo; echo "slowest sections (ms):"; sort -rn "$D/timing" | head -14 | sed 's/^/  /'
[ $fail -eq 0 ]
