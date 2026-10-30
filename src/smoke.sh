#!/bin/bash
# Acceptance for the built stages. Builds once, then runs the sections as
# shards in parallel, each against a daemon of its own on loopback and a
# private socket; exits non-zero on any failure.
#
#   ./smoke.sh            touched fast checks (alias --fast); not a landing gate
#   ./smoke.sh --normal   changed subsystems normal, others fast (landing gate)
#   ./smoke.sh --slow L   L slow, others fast; --slow all before a release
#   ./smoke.sh --shard S  one shard, with the same mode flags and PORT
#   SMOKE_TEST_PATTERN=pattern  filter a Bun agent shard for mutant kills only
#   SMOKE_TIMES=1         list every section and shard's wall time
#
# A landing runs normal for changed subsystems and fast for the rest.
# Mutants run only their owning check. Slow is for a release or huge change.
#
# A shard is a function below and SHARDS lists them. Its sections run in file
# order against its own daemon, state directory and ports, so a section moved
# out of the serial run starts with a fixture of the names it used to inherit.
# The last line is always `passed N, failed M`, summed over every shard.
set -u
cd "$(dirname "$0")"
SELF=$PWD/$(basename "$0")
source "$PWD/clients/cargo-target.sh"
# This run builds its own fixture: whatever bus the caller is already talking
# to is not it. A session launcher exports AGENT_BUS_TOKEN, NAME, ADDR, PUSH,
# RUNTIME, DESCR and the control pair, and the MCP face reads all of them —
# so run from an agent session and the face joins the live daemon in push
# mode, half the checks fail, and the failure looks like the tree.
# Setting the few this script knows about is not enough; the rest have to go.
for v in $(env | sed -n 's/^\(AGENT_BUS_[A-Z_]*\)=.*/\1/p'); do
  case $v in AGENT_BUS_ROLE|AGENT_BUS_FDS) ;; *) unset "$v" ;; esac
done
# Modes are parsed separately so their contract is checked without a build.
source ./smoke-options.sh
smoke_options "$@" || exit $?
declare -A LEVELS
load_plan() { while IFS=$'\t' read -r check level; do LEVELS[$check]=$level; done <"$SMOKE_PLAN"; }
level() { echo "${LEVELS[${CURRENT_CHECK:-${SHARD:-driver}}]:-normal}"; }
# Fast membership is an explicit command allowlist in smoke-map.json.
normal() { [[ "$(level)" = normal || "$(level)" = slow ]]; }
slow() { [ "$(level)" = slow ]; }
# Slow-only tests are assigned separately after their normal replacement is proved.
# The CLI keeps its reply context under XDG_CACHE_HOME, and this run wants a
# private one. Go's build cache lives there too by default, so redirecting it
# made every run recompile the world — 2.7s of "go vet" that is 0.1s warm,
# and four times that under -race. Keep Go's cache where it was.
export GOCACHE=${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build}
# In file order of their first section. `core` is the serial run the fast mode
# always was, in one process against one daemon; every other shard is a
# section that is slow or keeps a daemon of its own, moved out of it.
SHARDS=(core go_checks process_titles readers call_damage reply_to script_inbox
  script_done script_confined listing backlog ttl full_queue enrolment restart
  orphans transfer corrupt_pair supervisor
  mcp_rpc mcp_smoke mcp_push mcp_codex mcp_launcher mcp_rename web_ts samples tls_port
  client_python client_rust client_php client_typescript client_java client_go client_examples
  agent_slack agent_telegram agent_server_info agent_systemd agent_git agent_logwatch agent_sessions agent_ai agent_sms agent_voyage agent_db agent_qdrant agent_rag agent_webhook agent_fetch agent_smtp agent_imap agent_cron agent_at agent_subscriptions container_image distro_install agent_eventlog)
if [ -n "$SHARD" ]; then
  case " ${SHARDS[*]} " in
    *" $SHARD "*) ;;
    *) echo "no shard [$SHARD]; the shards are: ${SHARDS[*]}" >&2; exit 2 ;;
  esac
fi

# Shard k owns PORT+15k .. PORT+15k+14, so a whole run owns fifteen ports per
# shard from PORT. Unset, the run claims a free block under a lock held until
# every shard has exited (smoke-port.sh): a run started at the same moment
# takes the next, where a shared default found the other's daemon ("bad token").
source ./smoke-port.sh
source ./smoke-gofail.sh
smoke_port $((15 * ${#SHARDS[@]})) || exit 2

if [ -n "${SMOKE_TEST_PATTERN:-}" ]; then
  case "$SHARD" in
    agent_slack|agent_telegram) echo "FILTERED mutant shard [$SHARD]: $SMOKE_TEST_PATTERN" ;;
    *) echo "SMOKE_TEST_PATTERN is only for a Bun agent mutant shard, never a full gate" >&2; exit 2 ;;
  esac
fi

if [ -z "$SHARD" ]; then
  # The driver: one build, every shard at once, then each shard's output in
  # SHARDS order, whole and prefixed, and the counts summed. A shard that
  # ended without a result — a fixture `exit 1`, a crash, the bound below —
  # is a failure of its own, so no shard can go missing behind green ones.
  B=$(mktemp -d)
  trap 'kill $(jobs -p) 2>/dev/null; wait; rm -rf "$B"' EXIT
  trap 'exit 130' INT TERM
  export SMOKE_PLAN=$B/plan
  python3 ./smoke-select.py --mode "$MODE" --targets "$TARGETS" --out "$SMOKE_PLAN" || exit 2
  load_plan
  selected=()
  for name in "${SHARDS[@]}"; do
    if [ "$name" = core ]; then
      for check in core_{1..10}; do [ "${LEVELS[$check]}" != skip ] && { selected+=(core); break; }; done
    elif [ "${LEVELS[$name]:-normal}" != skip ]; then selected+=("$name"); fi
  done
  SHARDS=("${selected[@]}")
  [ "${#SHARDS[@]}" -gt 0 ] || { echo "FAIL empty selection"; exit 1; }
  echo "Selected ${#SHARDS[@]} shards: ${SHARDS[*]}; ports from $PORT"
  export BUILD_STARTED=$(date +%s)
  mkdir -p "$B/bin"
  if printf '%s\n' "${LEVELS[@]}" | grep -Eq '^(normal|slow)$'; then
    bash ./build.sh "$B/bin" || exit 1
  fi
  export BUILD_FINISHED=$(date +%s)
  # Read-only from here: each shard hard-links it into its own directory.
  export SMOKE_BIN=$B/bin SMOKE_OUT=$B/out MODE TARGETS
  mkdir -p "$SMOKE_OUT"
  # A shard that does not return is bounded, not waited for: a mutant that
  # turns a refusal into something that runs for ever must fail the run.
  LIMIT=${SMOKE_SHARD_TIMEOUT:-$( [ "$MODE" = slow ] && echo 600 || echo 240 )}
  t0=$(date +%s%3N)
  for k in "${!SHARDS[@]}"; do
    name=${SHARDS[$k]}
    ( s0=$(date +%s%3N)
      child=""
      # The driver kills this timing wrapper. Forward that signal to
      # timeout, which owns the shard's process group, then reap it before
      # the driver removes the shared build directory.
      trap 'if [ -n "$child" ]; then kill -TERM "$child" 2>/dev/null; wait "$child" 2>/dev/null; fi; exit 143' TERM INT
      PORT=$((PORT + 15*k)) timeout -k 10 "$LIMIT" bash "$SELF" --shard "$name" >"$SMOKE_OUT/$name.log" 2>&1 &
      child=$!
      wait "$child"
      echo $? >"$SMOKE_OUT/$name.rc"
      echo "$(( $(date +%s%3N) - s0 )) $name" >"$SMOKE_OUT/$name.wall" ) &
  done
  wait
  wall=$(( $(date +%s%3N) - t0 ))
  pass=0; fail=0; skipped=0
  for name in "${SHARDS[@]}"; do
    rc=$(cat "$SMOKE_OUT/$name.rc" 2>/dev/null || echo lost)
    note=""
    if read -r p f s 2>/dev/null <"$SMOKE_OUT/$name.result"; then
      pass=$((pass+p)); fail=$((fail+f)); skipped=$((skipped+s))
      if [ "$rc" != 0 ] && [ "$f" = 0 ]; then
        note="  FAIL shard $name exited $rc with no failed check"; fail=$((fail+1))
      fi
    else
      why="exit $rc"; [ "$rc" = 124 ] && why="killed after ${LIMIT}s"
      note="  FAIL shard $name ended without its result ($why)"; fail=$((fail+1))
    fi
    { echo "######## $name"; cat "$SMOKE_OUT/$name.log" 2>/dev/null
      [ -n "$note" ] && echo "$note"; } | sed "s/^/[$name] /"
  done
  echo
  echo "slowest sections (ms):"
  cat "$SMOKE_OUT"/*.timing 2>/dev/null | sort -rn | { [ "${SMOKE_TIMES:-0}" = 1 ] && cat || head -14; } | sed 's/^/  /'
  echo "shards by wall-clock (ms), whole run $wall:"
  cat "$SMOKE_OUT"/*.wall | sort -rn | { [ "${SMOKE_TIMES:-0}" = 1 ] && cat || head -6; } | sed 's/^/  /'
  if [ "$skipped" -gt 0 ]; then
    if normal; then
      echo "SKIPPED $skipped sections, slow-set or impossible on this host; ./smoke.sh --slow includes the slow set."
    else
      echo "SKIPPED $skipped gate sections and the race detector — this is NOT a full run."
      echo "         run ./smoke.sh --normal before landing a change."
    fi
  fi
  if [ "$MODE" = fast ]; then
    awk '$1 > 15000 { print "FAST OVER 15s: " $2 " " $1/1000 "s (needs a narrower allowlist)" }' "$SMOKE_OUT"/*.wall
    echo "NOT A LANDING GATE"
  fi
  echo; echo "passed $pass, failed $fail"
  [ "$fail" -eq 0 ]
  exit
fi

# A shard: its own directory, daemon and ports.
D=$(mktemp -d); DPID=""; SUPUNIT=""
# Wait for it: a daemon flushes on the way out, and a flush written while the
# directory is being removed leaves the directory behind.
cleanup() {
  [ -n "$SUPUNIT" ] && systemctl --user stop "$SUPUNIT" >/dev/null 2>&1
  [ -n "$DPID" ] && { kill "$DPID" 2>/dev/null; wait "$DPID" 2>/dev/null; }
  rm -rf "$D"
}
trap cleanup EXIT
# The driver's bound arrives as TERM; leaving through exit runs the cleanup.
trap 'exit 143' TERM INT
if [ -z "${SMOKE_PLAN:-}" ]; then
  export SMOKE_PLAN=$D/plan
  python3 ./smoke-select.py --mode "$MODE" --targets "$TARGETS" --out "$SMOKE_PLAN" || exit 2
  echo "SHARD OVERRIDE [$SHARD]: NOT A LANDING GATE"
fi
load_plan
# A filtered mutant must exercise the normal production assertion, never a fast substitute.
[ -z "${SMOKE_TEST_PATTERN:-}" ] || LEVELS[$SHARD]=normal
# An explicit shard is always run, even when it is outside the changed paths.
[ "${LEVELS[$SHARD]:-normal}" != skip ] || LEVELS[$SHARD]=fast
if [ "$(level)" = fast ]; then
  python3 -B ./smoke-fast.py "$SHARD"
  rc=$?
  if [ -n "${SMOKE_OUT:-}" ]; then
    if [ "$rc" = 0 ]; then echo '1 0 0'; else echo '0 1 0'; fi >"$SMOKE_OUT/$SHARD.result"
  fi
  exit "$rc"
fi
export XDG_CACHE_HOME=$D/cache
# A running script agent leaves its note, its log and its work directory
# here (docs/06-dedicated-runner.md#stopping-it-and-reading-what-it-said). Private
# to the run, or two runs would see each other's agents as already running
# — which is what the mutation harness does, twenty at a time.
export XDG_STATE_HOME=$D/state

if [ -n "${SMOKE_BIN:-}" ]; then
  # Hard links, not a copy: one build, every shard reading the same bytes.
  cp -al "$SMOKE_BIN"/. "$D"/ 2>/dev/null || cp -a "$SMOKE_BIN"/. "$D"/ || exit 1
else
  export BUILD_STARTED=$(date +%s)
  bash ./build.sh "$D" || exit 1
  export BUILD_FINISHED=$(date +%s)
fi

# The daemon belongs to a principal, which provisions the fixture names
# before issuing their credentials. Stated rather than taken from the
# account running the suite, so the checks read the same everywhere.
OWNER=parf@localhost
"$D/agent-busd" --addr 127.0.0.1:$PORT --socket "$D/bus.sock" --owner "$OWNER" --db "$D/bus.db" --create --flush-every 0 >"$D/daemon.log" 2>&1 &
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
# The owner's credential is in the daemon's database, which the daemon holds
# exclusively; it is asked for the way the owner would, over the daemon
# account's own socket (docs/02-access.md#local-socket). Every other name gets
# one from it on first use.
ACCOUNT=$(id -un)
owner_token() { AGENT_BUS_ADDR="$1/user-$ACCOUNT.sock" "$D/agent-bus" token "$OWNER"; }
TOKEN=$(owner_token "$D")
tok() {
  local f="$D/tok.$(printf '%s' "$1" | tr '/@.' '___')"
  if [ ! -s "$f" ]; then
    local pending
    pending=$(mktemp "$f.XXXXXX") || return
    if ! AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token "$1" >"$pending"; then
      rm -f "$pending"; return 1
    fi
    mv -f "$pending" "$f" || { rm -f "$pending"; return 1; }
  fi
  cat "$f" 2>/dev/null
}
ab() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
# The same, for `&`: exec so that $! is the binary. Backgrounding the function
# instead makes $! a subshell, and a signal sent to it leaves the agent
# running — which is how a check that an agent stops passed without one.
abx() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 exec "$D/agent-bus" "${@:2}"; }
# Like ab, but bounded. For a verb whose REFUSAL is the point: a mutation that
# turns the refusal into a foreground agent otherwise blocks the whole run,
# and a batch that times out loses every mutant after it.
abt() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 timeout 10 "$D/agent-bus" "${@:2}"; }
pass=0; fail=0; skipped=0
# Counters are cumulative since the daemon started, so a check on one has to
# be a delta. Asserting the absolute value worked only while this section
# happened to run first, and broke the moment another one dropped a message.
count() { local n; n=$(ab parf@localhost status | sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }
delta() { # label expected before after
  if [ "$(( $4 - $3 ))" -eq "$2" ]; then echo "  ok   $1"; pass=$((pass+1));
  else echo "  FAIL $1: expected +$2, got $3 -> $4"; fail=$((fail+1)); fi
}
# Per-record counters ride on the record a listing returns, not on the
# daemon-wide status, and omitempty means an absent field reads as zero.
svc() { local n; n=$(ab parf@localhost ls "$1" | sed -n "s/.*\"$2\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }
wait_reader() {
  for _ in $(seq 1 50); do
    ab '#asker@srv1' ls "$1" | grep -q '"readers":1' && return 0
    sleep 0.2
  done
  return 1
}
# Section timing, so "slow" is a measurement and not a hunch.
sec_name=""; sec_t0=0
sec() {
  local now; now=$(date +%s%3N)
  [ -n "$sec_name" ] && printf '%6s %s\n' "$((now-sec_t0))" "$sec_name" >> "$D/timing"
  sec_name=${1:-}; sec_t0=$now
  [ -n "$sec_name" ] && echo "== $1"
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

# Provision people separately from tokens. Every User has its own user record
# (docs/constitution.md#-user), so a person is always an inbox too. A failed
# fixture stops the run, rather than letting a later refusal look like the
# behavior under test.
users() {
  local name
  for name in "$@"; do
    curl -fsS --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" \
      -d "{\"name\":\"$name\",\"create\":true}" http://unix/user >/dev/null || exit 1
  done
}

# A section moved out of the serial run into a shard of its own starts on a
# fresh daemon, so it first makes what the sections before it used to leave:
# the same users, records and scripts, made the same way, with no check of its
# own. Each such shard opens with those lines, under an "Inherited:" comment.

# A check of seconds runs beside the rest of its shard; bg_report waits for each, in order, and counts it as ok_exit
# would. It stays in the shard's process group, which the driver's timeout signals whole.
bg_names=() bg_pids=()
bg_start() { bg_names+=("$1"); shift; "$@" >"$D/bg.${#bg_pids[@]}" 2>&1 & bg_pids+=($!); }
bg_report() {
  local i rc
  for i in "${!bg_pids[@]}"; do
    wait "${bg_pids[$i]}"; rc=$?
    ok_exit "${bg_names[$i]}" "$rc"
    [ "$rc" = 0 ] || cat "$D/bg.$i"
  done
}

go_checks() {
# These two mostly wait (fake podman, stub journal): they run beside the rest and are reported last (J35).
bg_start "a private-store podman harness killed by any signal leaves no container and no run directory (J32)" bash ./acceptance/guard.test.sh
bg_start "journal-quiet fails a run whose containers wrote to the journal, against a stub journal (J32)" bash ./journal-quiet.test.sh
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s . -p smoke_privacy_test.py -v 2>&1); rc=$?
ok_exit "live SMS recipient absent from tracked files" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s maintenance -p test_cleanup.py -v 2>&1); rc=$?
ok_exit "node cleanup attribution and retention" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s . -p smoke_select_test.py -v 2>&1); rc=$?
ok_exit "subsystem selector contract" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s . -p smoke_fast_test.py -v 2>&1); rc=$?
ok_exit "fast allowlist refuses empty matches" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(bash ./smoke-options.test.sh 2>&1); rc=$?
ok_exit "smoke mode contract" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(bash ./smoke-gofail.test.sh 2>&1); rc=$?
ok_exit "a failed go run says why: setup failures print go's error, test failures stay terse" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest check_markers_test 2>&1); rc=$?
ok_exit "the conflict-marker check fails each marker and passes a setext heading" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 ./check-markers.py 2>&1); rc=$?
ok_exit "this tree adds no git conflict marker against main" "$rc"
[ "$rc" = 0 ] || echo "$out"
# One package.sh build serves both archive checks; each built its own, the shard's largest CPU cost (J35).
out=$(./package.sh "$D/dist" 2>&1); rc=$?
ok_exit "package.sh builds the host's archives" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(RELEASE_DIST=$D/dist bash ./release-verify.real.test.sh 2>&1); rc=$?
ok_exit "release evidence passes the signed archives package.sh builds and fails each bad release" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(bash ./smoke-port.test.sh 2>&1); rc=$?
ok_exit "smoke port claim contract" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(NPM_DIST=$D/dist bash ./npm-pack.test.sh 2>&1); rc=$?
ok_exit "the npm wrapper installs from its tarballs and runs a request, a reply and an MCP call" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(bash ./acceptance/guard-lock.test.sh 2>&1); rc=$?
ok_exit "a private-store podman harness refuses to start a container nobody holds the smoke-gate lock for (J32)" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p agent_docs_test.py -v 2>&1); rc=$?
ok_exit "every bundled agent has its page, index entry and docs link" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p docs_order_test.py -v 2>&1); rc=$?
ok_exit "the docs-order checker fails each way a page can break the rule, and the enforced pages follow it" "$rc"
[ "$rc" = 0 ] || echo "$out"
out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s . -p smoke_agent_pool_test.py -v 2>&1); rc=$?
ok_exit "agent file pool accounts for every assertion" "$rc"
[ "$rc" = 0 ] || echo "$out"
sec "the Go checks"
# Run here, not only by hand: a mutation of anything the unit tests cover was
# invisible to this script while they lived outside. CLAUDE.md asks for all
# three to be green.
checks=("vet:go vet ./..." "test:go test ./...")
# The race run's own deadline is under the shard's 240 s, so a test that
# hangs is named by go test (its package and the test still running) rather
# than the shard being killed with nothing said.
normal && checks=("vet:go vet ./..." "race:go test -race -timeout 200s ./...")
for c in "${checks[@]}"; do
  out=$(eval "${c#*:}" 2>&1); rc=$?
  ok_exit "go ${c%%:*}" $rc
  # What failed, wherever it sits: see smoke-gofail.sh.
  [ $rc -eq 0 ] || echo "$out" | go_failure_text
done

# A layer rule nobody checks is a comment. These are the two directions that
# matter: nothing inward may name an adapter, and the adapter must actually
# be reached from somewhere, or the first check passes because the seam is
# empty. See src/MODULES.md#the-rule.
sec "the layers hold"
is_empty "core never imports an adapter" \
  "$(go list -deps ./internal/core ./internal/auth ./internal/ports | grep -E 'internal/(store|dump|directory|signature|journal)')"
is_empty "nor does a face" \
  "$(go list -deps ./internal/api ./cmd/agent-bus ./cmd/agent-bus-setup ./cmd/agent-bus-admin | grep -E 'internal/(store|dump|directory|signature|journal)')"
is_empty "and a port names no outside world of its own" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/ports 2>&1 | grep -E '^(os|net|net/http|os/exec|database/sql)$')"
has "while the process that assembles them holds the ones that persist" \
  "$(go list -deps ./cmd/agent-busd | grep -E 'internal/(store|dump|directory|signature|journal)' | tr '\n' ' ')" 'internal/directory/file .*internal/directory/github .*internal/journal .*internal/signature/sshkeygen .*internal/store/sqlite'
has "and core is what asks for it" \
  "$(go list -f '{{join .Imports "\n"}}' ./internal/auth 2>&1)" 'internal/ports'
sec "the fixture harness checks"
bg_report
}
process_titles() {
if normal; then
  sec "shared version and live process titles"
  out=$(timeout 60 bash ./smoke-process.sh "$D" 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "process version smoke" "$rc"
else skipped=$((skipped+1)); fi

}
core_1() {
sec "every program's help opens with its name and the shared version"
want_version=$(cat internal/version/VERSION)
for prog in agent-bus agent-busd agent-bus-admin agent-bus-setup; do
  has "$prog --help names $want_version" "$("$D/$prog" --help 2>/dev/null | head -1)" "^$prog $want_version\b"
  "$D/$prog" --help >/dev/null 2>&1; ok_exit "and succeeds" $?
done
has "and bare agent-bus prints the same" "$("$D/agent-bus" 2>&1 | head -1)" "^agent-bus $want_version\b"
sec "status on both listeners"
has "unix socket" "$(ab parf@localhost status)" '"records"'
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
local_token=$(env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" token "$OWNER")
has "the token helper discovers the user socket and retrieves the existing credential" \
  "$([ "$local_token" = "$TOKEN" ] && echo yes)" '^yes$'
alice_token=$(tok alice@srv1)
local_token=$(env -u AGENT_BUS_ADDR AGENT_BUS_TOKEN=$alice_token XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" token alice@srv1)
has "the token helper discovers the shared socket for an existing token" \
  "$([ "$local_token" = "$alice_token" ] && echo yes)" '^yes$'
out=$(env -u AGENT_BUS_ADDR AGENT_BUS_TOKEN=$alice_token XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" token "$OWNER" 2>&1); rc=$?
bad_exit "token helper discovery never borrows the local account's authority" $rc
out=$(AGENT_BUS_ADDR=$D/missing.sock XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" token "$OWNER" 2>&1); rc=$?
bad_exit "the token helper does not replace an explicit unavailable address" $rc
has "the token helper reports the explicit failing socket" "$out" 'missing.sock'
has "a global CLI address takes precedence over an invalid environment address" \
  "$(env -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/missing.sock "$D/agent-bus" --addr "$MINE" status)" "\"you\":\"$OWNER\""
has "the equals form of a global CLI address takes precedence too" \
  "$(env -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$D/missing.sock "$D/agent-bus" --addr="$MINE" status)" "\"you\":\"$OWNER\""
has "an explicit environment address is not replaced by discovered sockets" \
  "$(AGENT_BUS_ADDR=$D/missing.sock AGENT_BUS_TOKEN=$TOKEN XDG_RUNTIME_DIR=$D/discovery "$D/agent-bus" status 2>&1)" 'missing.sock'
has "a write lands under the socket's principal, not under nobody" \
  "$(env -u AGENT_BUS_NAME -u AGENT_BUS_TOKEN AGENT_BUS_ADDR=$MINE "$D/agent-bus" register '#sock-made@srv1' --kind agent --allow '*' --descr "from the socket" >/dev/null; ab nobody2@srv1 ls '#sock-made@srv1')" "\"owner\":\"$OWNER\""
# One daemon, many people. A second mapped account gets a socket of its own
# and is a different principal on it — which is the whole point of the
# arrangement, and is not provable with one socket.
if id -u nobody >/dev/null 2>&1; then
  mkdir -p "$D/multi"
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+6)) --socket "$D/multi/bus.sock" \
    --owner "$OWNER" --user "nobody=nemo@srv1" --db "$D/multi/bus.db" --create --flush-every 0 >"$D/multi.log" 2>&1 &
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
  curl -fsS --unix-socket "$D/multi/user-$ACCOUNT.sock" \
    -d '{"name":"nemo@srv1","create":true}' http://unix/user >/dev/null || exit 1
  has "and is a different principal on it" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" "http://unix/status")" '"you":"nemo@srv1"'
  has "while the owner's socket in the same directory is still the owner" \
    "$(curl -s --unix-socket "$D/multi/user-$ACCOUNT.sock" "http://unix/status")" "\"you\":\"$OWNER\""
  # The same credential that administers users administers this durable map;
  # the listener itself changes only when the supervisor is restarted. Seed
  # the replacement principal and one private agent so the check proves
  # both discovery and use, rather than only echoing /status.
  curl -fsS --unix-socket "$D/multi/user-$ACCOUNT.sock" \
    -d '{"name":"mapped@srv1","create":true}' http://unix/user >/dev/null || exit 1
  mapped_token=$(AGENT_BUS_ADDR=$D/multi/user-$ACCOUNT.sock "$D/agent-bus" token mapped@srv1)
  AGENT_BUS_ADDR=$D/multi/bus.sock AGENT_BUS_TOKEN=$mapped_token AGENT_BUS_NAME=mapped@srv1 \
    "$D/agent-bus" register '#mapped-private@srv1' --kind agent >/dev/null || exit 1
  out=$(AGENT_BUS_ADDR=$D/multi/user-nobody.sock "$D/agent-bus" ls '#mapped-private@srv1' 2>&1); rc=$?
  bad_exit "the old mapping cannot discover the replacement principal's private agent" $rc
  out=$(AGENT_BUS_HOME=$D/admin-home AGENT_BUS_ADDR=$D/multi/user-$ACCOUNT.sock \
    "$D/agent-bus-admin" account set nobody mapped@srv1 2>&1); rc=$?
  ok_exit "the existing administrative credential changes the account map" $rc
  has "the persisted change says a restart is required" "$out" 'restart agent-busd'
  has "the listener does not change identity in place" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" http://unix/status)" '"you":"nemo@srv1"'
  kill $MPID2 2>/dev/null; wait $MPID2 2>/dev/null
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+6)) --socket "$D/multi/bus.sock" \
    --owner "$OWNER" --user "nobody=nemo@srv1" --db "$D/multi/bus.db" --create --flush-every 0 >"$D/multi.log2" 2>&1 &
  MPID2=$!
  ready "$D/multi/user-nobody.sock" || echo "  WARNING: the remapped socket never answered"
  has "the durable map overrides the contrary setup seed after restart" \
    "$(curl -s --unix-socket "$D/multi/user-nobody.sock" http://unix/status)" '"you":"mapped@srv1"'
  has "the changed mapping reveals what the replacement principal may see" \
    "$(AGENT_BUS_ADDR=$D/multi/user-nobody.sock "$D/agent-bus" ls '#mapped-private@srv1')" '"name":"#mapped-private@srv1"'
  out=$(AGENT_BUS_ADDR=$D/multi/user-nobody.sock "$D/agent-bus" send '#mapped-private@srv1' mapped-use 2>&1); rc=$?
  ok_exit "and it may use that private agent through the remapped socket" $rc
  has "an unrelated account mapping is unchanged" \
    "$(curl -s --unix-socket "$D/multi/user-$ACCOUNT.sock" http://unix/status)" "\"you\":\"$OWNER\""
  out=$(AGENT_BUS_HOME=$D/admin-home AGENT_BUS_ADDR=$D/multi/user-$ACCOUNT.sock \
    "$D/agent-bus-admin" account list 2>&1)
  has "the applied account map is listed" "$out" $'nobody\tmapped@srv1'
  lacks "and no longer claims a pending restart" "$out" 'restart required'
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
  "$(post_body alice@srv1 /register '{"kind":"agent","name":"#alice-wrote@srv1","allow":["*"]}' >/dev/null; ab bob@srv1 ls '#alice-wrote@srv1')" '"owner":"alice@srv1"'

sec "who may ask for whose credential"
has "a principal may get its own" \
  "$(post_code alice@srv1 /token '{"name":"alice@srv1"}')" '200'
has "but not somebody else's" \
  "$(post_code alice@srv1 /token '{"name":"bob@srv1"}')" '403'
ab alice@srv1 register '#alice-svc@srv1' --kind agent --allow '*' --descr "hers" >/dev/null
has "and may get one for a record it owns" \
  "$(post_code alice@srv1 /token '{"name":"#alice-svc@srv1"}')" '200'
has "while somebody else may not" \
  "$(post_code bob@srv1 /token '{"name":"#alice-svc@srv1"}')" '403'
has "even the daemon owner cannot mint a credential for an unknown name" \
  "$(post_code "$OWNER" /token '{"name":"#nobody-owns-this@srv1"}')" '^401$'
has "nor rotate one into existence" \
  "$(post_code "$OWNER" /token '{"name":"#nobody-owns-this@srv1","rotate":true}')" '^401$'
lacks "a refused token ask creates no stored credential" \
  "$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" http://unix/users)" '#nobody-owns-this@srv1'
has "and the directory that would show one is readable" \
  "$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" http://unix/users)" "$OWNER"
ab "$OWNER" register '#nobody-owns-this@srv1' --kind agent --allow '*' >/dev/null || exit 1
has "the owner may issue a credential after registering the name" \
  "$(post_code "$OWNER" /token '{"name":"#nobody-owns-this@srv1"}')" '^200$'
users owner@srv1
again=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$alice AGENT_BUS_NAME=alice@srv1 "$D/agent-bus" token alice@srv1 2>&1)
if [ "$again" = "$alice" ]; then echo "  ok   asking twice is a read, not a rotation"; pass=$((pass+1));
else echo "  FAIL asking twice is a read, not a rotation: [$again]"; fail=$((fail+1)); fi
# Rotation: two are accepted, the one before them is not. A refresh that
# stranded traffic already queued under the old token would be worse than
# no rotation at all. See docs/02-access.md#token-lifetime.
ab owner@srv1 register '#rotor@srv1' --kind agent --allow '*' >/dev/null
first=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token '#rotor@srv1')
second=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token '#rotor@srv1' --rotate)
third=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok owner@srv1) AGENT_BUS_NAME=owner@srv1 "$D/agent-bus" token '#rotor@srv1' --rotate)
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
# second run does not know about answers for nothing and is dropped at that
# start, which is the rule and not a fault
# (docs/02-access.md#ownerless-credentials). Minting for names nobody keeps is
# how a directory fills with rows nothing is behind.
mkdir -p "$D/r2"
r2_up() {
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+4)) --socket "$D/r2/bus.sock" \
    --owner "$OWNER" --db "$D/r2/bus.db" --create --flush-every 0 >"$D/daemon2-$1.log" 2>&1 &
  RPID=$!
  ready "$D/r2/bus.sock" || echo "  WARNING: $D/r2/bus.sock never answered"
  R2TOK=$(owner_token "$D/r2")
}
r2ab() { AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$R2TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
r2tok() { AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$R2TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token "$@"; }
r2code() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/r2/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix/status"; }
r2_up first
r2ab register '#kept-rotor@srv1' --kind agent --allow '*' >/dev/null
r2ab register '#kept-other@srv1' --kind agent --allow '*' >/dev/null
dropped=$(r2tok '#kept-rotor@srv1')
previous=$(r2tok '#kept-rotor@srv1' --rotate)
current=$(r2tok '#kept-rotor@srv1' --rotate)
other=$(r2tok '#kept-other@srv1')
has "both rotation slots work before the restart" "$(r2code "$previous")$(r2code "$current")" '200200'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
r2_up second
has "a restart keeps both of a rotated principal's tokens" "$(r2code "$current")" '200'
has "and the one before it, across the restart" "$(r2code "$previous")" '200'
has "and still refuses the one it dropped" "$(r2code "$dropped")" '401'
has "a restart keeps every principal, not only the owner's" \
  "$(AGENT_BUS_ADDR=$D/r2/bus.sock AGENT_BUS_TOKEN=$other AGENT_BUS_NAME=#kept-other@srv1 "$D/agent-bus" status)" '"up"'
kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
# What the store keeps, it keeps to itself. See docs/07-setup.md#storage.
has "the database is that account's alone" "$(stat -c %a "$D/r2/bus.db")" '^600$'
# A credential that could not be written down is one a restart forgets, so it
# is not handed out either. A held SQLite database cannot be made to refuse a
# write from outside the process, so that refusal is proved in-process by
# internal/auth TestMintHandsOutNothingItCouldNotWrite.
# `run` runs until it is stopped, so this check leans on the refusal to end
# it. Under a mutant that allows it, it ran until the harness's own timeout
# and took the whole batch with it — hence the bound, and hence 124 counting
# as a failure of the check rather than the refusal it was looking for.
out=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$bob AGENT_BUS_NAME=bob@srv1 timeout 5 "$D/agent-bus" run '#alice-svc@srv1' --allow '*' --algo args /bin/echo 2>&1); rc=$?
if [ "$rc" -ne 0 ] && [ "$rc" -ne 124 ]; then
  echo "  ok   and starting an agent you do not own is refused"; pass=$((pass+1))
else
  echo "  FAIL and starting an agent you do not own is refused: exit $rc"; fail=$((fail+1))
fi
has "because the record is not his to take over" "$out" 'belongs to someone else'

sec "a record belongs to whoever published it"
users thief2@srv1 stranger2@srv1
# Publishing is open; changing is not. See docs/01-identity-and-authority.md#ownership.
ab owner@srv1 register '#owned@srv1' --kind agent --allow '*' --descr "mine" >/dev/null
out=$(ab thief2@srv1 register '#owned@srv1' --kind agent --allow '*' --descr "stolen" 2>&1); rc=$?
bad_exit "somebody else cannot re-register it --kind agent" $rc
has "and is told whose it is" "$out" "#owned@srv1 is owner@srv1's"
has "the description it stated did not land" "$(ab nobody2@srv1 ls '#owned@srv1')" '"descr":"mine"'
has "its owner may still change it" \
  "$(ab owner@srv1 register '#owned@srv1' --kind agent --allow '*' --descr "changed" >/dev/null; ab nobody2@srv1 ls '#owned@srv1')" '"descr":"changed"'
has "and the record itself may refresh its own, as an agent does on every start" \
  "$(ab '#owned@srv1' register '#owned@srv1' --kind agent --allow '*' --descr "self" >/dev/null; ab nobody2@srv1 ls '#owned@srv1')" '"descr":"self"'
# Only a service is reached at an address (K.28): an agent is sent to by name.
out=$(ab owner@srv1 register '#owned@srv1' --kind agent --allow '*' --descr "self" --addr x:1 2>&1); rc=$?
bad_exit "an agent registered with an address is refused" $rc
has "and told only a service takes one" "$out" 'only a service does'
lacks "leaving no address on it" "$(ab nobody2@srv1 ls '#owned@srv1')" '"addr"'
has "while an existing user may publish a new name" \
  "$(ab stranger2@srv1 register '#brand-new@srv1' --kind agent --allow '*' --descr "open" >/dev/null; ab nobody2@srv1 ls '#brand-new@srv1')" '"descr":"open"'

# Message-flow fixtures opt into sharing explicitly. The Go checks above
# separately pin empty-ACL restrictions, including restored records.
sec "register and --kind agent ls"
users sender@srv1 someone@srv1 nobody@srv1
ab owner@srv1 register '#fixer@srv1' --allow '*' --kind agent --descr "fixes things" >/dev/null
ab owner@srv1 register '#asker@srv1' --allow '*' --kind agent >/dev/null
has "ls shows the description" "$(ab '#asker@srv1' ls --all)" 'fixes things'
# With no option, ls lists the agents that can take a message now: one with a
# waiting reader is there, the same one idle is not, and a queue needs --kind.
# Its own agent and a bounded wait that ends by itself: `ab ... &` is a
# subshell, so a kill would miss the reader and it would take later mail.
ab owner@srv1 register '#lister@srv1' --allow '*' --kind agent >/dev/null
ab owner@srv1 register idle-q@srv1 --kind queue --allow '*' >/dev/null
lacks "ls with no option leaves out an agent nobody reads" "$(ab '#asker@srv1' ls)" '"name":"#lister@srv1"'
has "though --all has it" "$(ab '#asker@srv1' ls --all)" '"name":"#lister@srv1"'
ab '#lister@srv1' consume --wait 3s >/dev/null 2>&1 &
LISTPID=$!
for _ in $(seq 1 30); do ab '#asker@srv1' ls | grep -q '"name":"#lister@srv1"' && break; sleep 0.1; done
has "and lists it once a reader waits on it" "$(ab '#asker@srv1' ls)" '"name":"#lister@srv1"'
lacks "while a queue is listed only when asked for" "$(ab '#asker@srv1' ls)" 'idle-q@srv1'
has "as --kind queue does" "$(ab '#asker@srv1' ls --kind queue)" 'idle-q@srv1'
wait $LISTPID 2>/dev/null

sec "a send moves the record's day by exactly one"
# The whole day, not the open slot: a boundary between the two reads moves the
# message into the slot before, and the day still holds it once.
ab owner@srv1 register '#tally@srv1' --allow '*' --kind agent >/dev/null || exit 1
day_in() { tbody "$(tok owner@srv1)" "/activity?name=%23tally%40srv1" | grep -o '"in":[0-9]*' | awk -F: '{s+=$2} END {print s+0}'; }
BEFORE=$(day_in)
ab '#asker@srv1' send '#tally@srv1' "count me" >/dev/null
has "the day counts one more accepted" "$(day_in)" "^$((BEFORE+1))\$"
has "the day is 144 slots" "$(tbody "$(tok owner@srv1)" "/activity?name=%23tally%40srv1" | grep -o '"at":' | wc -l)" '^144$'

sec "send, then reply matched by topic and tag"
ab '#fixer@srv1' consume --wait 10s > "$D/got.json" & CPID=$!
sleep 0.3
ab '#asker@srv1' send '#fixer@srv1' --topic deploy-42 --tag q1 "run the migration?" >/dev/null
wait $CPID
has "body arrived" "$(cat "$D/got.json")" 'run the migration?'
has "sender is named" "$(cat "$D/got.json")" '#asker@srv1'
MSGID=$(sed 's/.*"message_id":"\([^"]*\)".*/\1/' "$D/got.json")
ab '#fixer@srv1' reply "$MSGID" "yes, running it" >/dev/null
REPLY=$(ab '#asker@srv1' consume --topic deploy-42 --tag q1 --wait 5s)
has "reply came back" "$REPLY" 'yes, running it'
has "reply keeps the tag" "$REPLY" '"tag":"q1"'

sec "a filtered waiter is served ahead of the unfiltered reader"
ab '#fixer@srv1' consume --wait 6s > "$D/reader.json" 2>/dev/null & UPID=$!
sleep 0.3
ab '#fixer@srv1' consume --topic prio --tag p1 --wait 6s > "$D/waiter.json" 2>/dev/null & FPID=$!
sleep 0.3
ab '#asker@srv1' send '#fixer@srv1' --topic prio --tag p1 "for the waiter" >/dev/null
wait $FPID 2>/dev/null
has "the filtered waiter got it" "$(cat "$D/waiter.json")" 'for the waiter'
if grep -q 'for the waiter' "$D/reader.json" 2>/dev/null; then
  echo "  FAIL the unfiltered reader stole it"; fail=$((fail+1))
else
  echo "  ok   the unfiltered reader did not steal it"; pass=$((pass+1))
fi
ab '#asker@srv1' send '#fixer@srv1' --topic other --tag x "for the reader" >/dev/null
wait $UPID 2>/dev/null
has "the unfiltered reader got the other one" "$(cat "$D/reader.json")" 'for the reader'

sec "a message sent while nobody is reading waits"
ab '#asker@srv1' send '#fixer@srv1' --topic later --tag t1 "queued while down" >/dev/null
sleep 0.2
has "backlog arrives later" "$(ab '#fixer@srv1' consume --wait 5s)" 'queued while down'

}
readers() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 sender@srv1
ab owner@srv1 register '#fixer@srv1' --allow '*' --kind agent --descr "fixes things" >/dev/null || exit 1
if normal; then
  sec "one reader per inbox"
  reader_count() {
    local n=$1 want=$2 count
    for _ in $(seq 1 100); do
      count=$(svc "$n" readers)
      [ "$count" -eq "$want" ] && return 0
      sleep 0.1
    done
    echo "  FAIL $n: expected $want readers, got $count"
    fail=$((fail+1))
    return 1
  }
  abx '#fixer@srv1' consume --wait 30s >"$D/fixer.reader" 2>&1 & RPID=$!
  if ! reader_count '#fixer@srv1' 1; then
    cat "$D/fixer.reader"
    kill "$RPID" 2>/dev/null; wait "$RPID" 2>/dev/null
    return
  fi
  has "second unfiltered read refused" "$(ab '#fixer@srv1' consume --wait 1s 2>&1)" 'already has a reader'
  out=$(ab '#fixer@srv1' consume --topic x --tag y --wait 1s 2>&1); rc=$?
  ok_exit "filtered waiter allowed beside it" $rc
  # A pool is the exception, and asking is what makes it one. Either side
  # declining keeps the refusal: a reader that wants the inbox to itself
  # still gets it, which is the rule this replaces, not removes.
  has "a sharing reader beside one that wants the inbox to itself is refused too" \
    "$(ab '#fixer@srv1' consume --share --wait 1s 2>&1)" 'already has a reader'
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null
  reader_count '#fixer@srv1' 0 || return

  sec "several workers behind one name"
  # Competing consumers on a BACKLOG already worked. The rule bit on an
  # EMPTY inbox, which is the steady state of a worker pool: the readers
  # block first and the work arrives afterwards. Prefilling the queue is how
  # the first draft of this criterion passed without testing anything, so
  # nothing is sent until all three are provably blocked.
  ab owner@srv1 register '#pool@srv1' --allow '*' --kind agent >/dev/null
  for i in 1 2 3; do
    abx '#pool@srv1' consume --share --wait 30s >"$D/pool.$i" 2>&1 &
    eval "P$i=\$!"
  done
  if ! reader_count '#pool@srv1' 3; then
    cat "$D"/pool.[123]
    kill "$P1" "$P2" "$P3" 2>/dev/null
    wait "$P1" "$P2" "$P3" 2>/dev/null
    return   # Sending now would test a backlog, not three blocked workers.
  fi
  echo "  ok   three workers wait on one empty inbox at once"; pass=$((pass+1))
  for i in 1 2 3; do ab sender@srv1 send '#pool@srv1' "job-$i" >/dev/null; done
  wait $P1 $P2 $P3 2>/dev/null
  got=$(cat "$D"/pool.[123] | grep -o 'job-[123]' | sort | tr '\n' ' ')
  has "and the work reaches all three as it arrives" "$got" 'job-1 job-2 job-3'
  is_empty "with none of them refused as a second reader" \
    "$(grep -l 'already has a reader' "$D"/pool.[123] 2>/dev/null)"
  is_empty "and no job handed to two of them" \
    "$(cat "$D"/pool.[123] | grep -o 'job-[123]' | sort | uniq -d)"

else skipped=$((skipped+1)); fi
}
core_2() {
sec "pub/sub: a copy per recipient, in that recipient's own inbox"
users drive-by@srv1 nobody-here@srv1
# A channel is the 📮/📣 record; an envelope's topic is a label on one
# message. The verb and the flag say channel from 0.6.7, and the old
# spellings are not kept as aliases. See docs/03-records-channel.md.
has "the old topic verb is gone, and the refusal names what the CLI has" \
  "$(ab owner@srv1 topic create gone@srv1 --kind pubsub 2>&1)" 'unknown verb'
ab owner@srv1 channel create news@srv1 --allow '*' --kind pubsub --descr "broadcast" >/dev/null
lacks "publish no longer takes --topic for the channel" \
  "$(ab owner@srv1 publish --topic news@srv1 "by the old flag" 2>&1)" '"message_id"'
has "and names the refused flag" \
  "$(ab owner@srv1 publish --topic news@srv1 "by the old flag" 2>&1)" 'unknown option --topic'
ab owner@srv1 register '#sub-a@srv1' --allow '*' --kind agent >/dev/null
ab owner@srv1 register '#sub-b@srv1' --allow '*' --kind agent >/dev/null
# A 📣 carries two lists. The ACL is who may publish; the Deliver-To list is
# who receives, and nothing puts itself on it. See docs/04-messaging.md#deliver-to.
deliver_to() { post_body owner@srv1 /manage "{\"name\":\"$1\",\"subs\":$2}"; }
out=$(post_body '#sub-a@srv1' /subscribe '{"channel":"news@srv1","off":false}' 2>&1)
is "a name cannot put itself on a Deliver-To list" \
  "$(post_code '#sub-a@srv1' /subscribe '{"channel":"news@srv1","off":false}')" '403'
has "and is told whose list it is" "$out" 'decides who news@srv1 delivers to'
is_empty "so the topic names nobody" \
  "$(ab owner@srv1 ls news@srv1 | grep -o '"subs"')"
# The wire moved too: the field is channel, so an old client's topic field is
# not read and the daemon is handed an empty name. Both halves are asserted
# against a name that exists, or the refusal is about the caller instead
# and the pair passes whichever field the daemon reads.
has "and /subscribe no longer reads a topic field" \
  "$(post_body '#sub-a@srv1' /subscribe '{"topic":"news@srv1"}' 2>&1)" 'bad name \\"\\"'
has "while the channel field names the channel for real" \
  "$(post_body '#sub-a@srv1' /subscribe '{"channel":"news@srv1"}' 2>&1)" 'delivers to'
deliver_to news@srv1 '["#sub-a@srv1","#sub-b@srv1"]' >/dev/null
has "the channel says who it delivers to" "$(ab owner@srv1 ls news@srv1)" '"subs":\["#sub-a@srv1","#sub-b@srv1"\]'
ab drive-by@srv1 publish --channel news@srv1 "to everyone" >/dev/null
# Both, not one: a fan-out that hands the message to whoever reads first is a
# 📮 channel, and that is the kind this is NOT.
has "one recipient gets a copy" "$(ab '#sub-a@srv1' consume --wait 5s)" 'to everyone'
has "and so does the other, of the same publication" "$(ab '#sub-b@srv1' consume --wait 5s)" 'to everyone'
# The copy is addressed to the recipient: it is in an inbox, and an inbox
# belongs to a name (docs/04-messaging.md#inbox-queues).
ab drive-by@srv1 publish --channel news@srv1 "addressed" >/dev/null
has "and the copy is addressed to the recipient, not to the channel" \
  "$(ab '#sub-a@srv1' consume --wait 5s)" '"to":"#sub-a@srv1"'
ab '#sub-b@srv1' consume --wait 5s >/dev/null
# The topic keeps nothing of its own — that is the whole difference from a
# queue topic, and "queued" on its record is where it would show.
is_empty "while the topic itself keeps nothing" \
  "$(ab owner@srv1 ls news@srv1 | grep -o '"queued":[1-9][0-9]*')"
has "though its publications are counted" "$(ab owner@srv1 ls news@srv1)" '"in":2'
# Nobody to take it is a broken flow, refused and counted nowhere
# (docs/constitution.md#pubsub-routing), and not a message kept for later.
ab owner@srv1 channel create void@srv1 --allow '*' --kind pubsub >/dev/null
has "a publish with nobody on the list is refused" \
  "$(ab drive-by@srv1 publish --channel void@srv1 "into the void" 2>&1)" 'has no recipient to take it'
is_empty "and kept and counted for nobody" "$(ab owner@srv1 ls void@srv1 | grep -o '"queued":[1-9][0-9]*\|"in":[1-9][0-9]*')"
# Leaving is the one thing a recipient may do to the list, and it stops the
# copies. Not the same as never having been on it.
ab '#sub-b@srv1' unsubscribe news@srv1 >/dev/null
ab drive-by@srv1 publish --channel news@srv1 "second round" >/dev/null
has "a recipient that stayed still gets it" "$(ab '#sub-a@srv1' consume --wait 5s)" 'second round'
is_empty "and one that left gets nothing" "$(ab '#sub-b@srv1' consume --wait 1s)"
has "and the topic no longer names it" "$(ab owner@srv1 ls news@srv1)" '"subs":\["#sub-a@srv1"\]'
# A User takes no published copy; the list names agents and groups.
has "a known user cannot be on the list" \
  "$(deliver_to news@srv1 '["#sub-a@srv1","nobody-here@srv1"]' 2>&1)" 'a user takes no delivered copy'
has "and a refused list stores none of itself" "$(ab owner@srv1 ls news@srv1)" '"subs":\["#sub-a@srv1"\]'
# A queue topic that EXISTS, so the refusal is about its kind and not about
# the name being unknown.
ab owner@srv1 channel create work@srv1 --allow '*' --descr "a queue topic" >/dev/null
# A queue's deliver_to is one slot (docs/04-messaging.md#deliver-to).
has "and a queue delivers to one destination, not two" \
  "$(deliver_to work@srv1 '["#sub-a@srv1","#sub-b@srv1"]' 2>&1)" 'holds one destination'
out=$(ab '#sub-a@srv1' unsubscribe work@srv1 2>&1); rc=$?
bad_exit "nor is a queue topic something to leave" $rc
has "refused for its kind, not for being unknown" "$out" 'only a pubsub channel has a deliver-to list'
# The two lists are separate, and that is the whole point: the ACL decides
# publishing and decides nothing about who receives. Asserted on a topic
# whose ACL admits ONLY its owner, with somebody else on the Deliver-To list.
ab owner@srv1 channel create members@srv1 --kind pubsub --allow owner@srv1 >/dev/null
deliver_to members@srv1 '["#sub-b@srv1"]' >/dev/null
ab owner@srv1 publish --channel members@srv1 "members only" >/dev/null
has "a recipient the ACL does not admit still receives" \
  "$(ab '#sub-b@srv1' consume --wait 5s)" 'members only'
out=$(ab '#sub-b@srv1' publish --channel members@srv1 "may I?" 2>&1); rc=$?
bad_exit "while that same name may not publish there" $rc
ab owner@srv1 channel create members@srv1 --kind pubsub --allow owner@srv1 --descr "restated" >/dev/null
has "the list survives the record being restated" "$(ab owner@srv1 ls members@srv1)" '"subs":\["#sub-b@srv1"\]'
# A @group on the list stands for its members at the publish, so somebody
# added to the group afterwards receives without the list being touched.
# Its own names, because everything published here lands in them and a
# leftover copy is somebody else's check reading the wrong inbox.
ab owner@srv1 register '#grp-a@srv1' --allow '*' --kind agent >/dev/null
ab owner@srv1 register '#grp-b@srv1' --allow '*' --kind agent >/dev/null
post_body "$OWNER" /group '{"name":"@smoke-readers","members":["#grp-a@srv1"]}' >/dev/null
# Only a manager of the group puts it on a list (docs/04-messaging.md#deliver-to).
post_body "$OWNER" /manage '{"name":"@smoke-readers","maintainers":"owner@srv1"}' >/dev/null
deliver_to members@srv1 '["@smoke-readers"]' >/dev/null
ab owner@srv1 publish --channel members@srv1 "to the group" >/dev/null
has "a group on the list delivers to its member" "$(ab '#grp-a@srv1' consume --wait 5s)" 'to the group'
post_body "$OWNER" /group '{"name":"@smoke-readers","members":["#grp-a@srv1","#grp-b@srv1"]}' >/dev/null
ab owner@srv1 publish --channel members@srv1 "and to the new one" >/dev/null
has "and to a member added after the list was written" \
  "$(ab '#grp-b@srv1' consume --wait 5s)" 'and to the new one'
has "while the stored list is still the group, not its members" \
  "$(ab owner@srv1 ls members@srv1)" '"subs":\["@smoke-readers"\]'
# Removing yourself from a group-derived delivery would take nothing out of
# the list, so it is refused rather than reported as a removal.
out=$(ab '#grp-b@srv1' unsubscribe members@srv1 2>&1); rc=$?
bad_exit "a recipient reached through a group cannot remove itself" $rc
has "and the refusal names the group to leave instead" "$out" '@smoke-readers'
ab owner@srv1 publish --channel members@srv1 "still arriving" >/dev/null
has "so the copies the refusal promised keep arriving" \
  "$(ab '#grp-b@srv1' consume --wait 5s)" 'still arriving'
# One recipient cannot hold the topic hostage: its own bound applies to its
# own copy, and the others still get theirs.
ab owner@srv1 register '#full-sub@srv1' --allow '*' --kind agent --bound 1 >/dev/null
deliver_to news@srv1 '["#sub-a@srv1","#full-sub@srv1"]' >/dev/null
ab drive-by@srv1 publish --channel news@srv1 "fills it" >/dev/null
ok_exit "a publish a full recipient cannot take still succeeds" \
  "$(ab drive-by@srv1 publish --channel news@srv1 "does not fit" >/dev/null 2>&1; echo $?)"
has "and the recipient with room gets both" \
  "$(ab '#sub-a@srv1' consume --wait 5s; ab '#sub-a@srv1' consume --wait 5s)" 'does not fit'
has "while the copy that would not fit is counted as a loss" \
  "$(ab parf@localhost status)" '"dropped":[1-9]'
# And against the RECIPIENT, whose own bound refused it — not against the
# topic, which keeps nothing and so can lose nothing. The node total rises
# wherever it is charged, which is exactly why it cannot be the check.
has "and it is the recipient's loss, its bound having refused it" \
  "$(ab owner@srv1 ls '#full-sub@srv1')" '"dropped":[1-9]'
is_empty "not the topic's, which keeps nothing to lose" \
  "$(ab owner@srv1 ls news@srv1 | grep -o '"dropped":')"

sec "a person can see what they hold a credential for, and never the credential"
users holder@srv1
ab holder@srv1 register '#holder-svc@srv1' --kind agent --allow '*' --descr "something they own" >/dev/null
ab holder@srv1 register '#holder-cold@srv1' --kind agent --allow '*' --descr "owned, never asked for" >/dev/null
# Somebody else's name, registered here and given a credential here, so that
# "not in mine" is checked against a name that exists and holds one. A name
# nobody has a credential for would be dropped by the token store anyway,
# and the check would pass whatever the registry had handed it.
ab alice@srv1 register '#alice-held@srv1' --kind agent --allow '*' --descr "hers, and it holds one" >/dev/null
tok '#alice-held@srv1' >/dev/null
# Owning a name is not holding a credential for it: one has to be asked for,
# which is what an agent's owner does before starting it.
tok '#holder-svc@srv1' >/dev/null
HTOK=$(tok holder@srv1)
NAMES=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $HTOK" "http://unix/names")
has "their own name is there" "$NAMES" '"name":"holder@srv1"'
has "and an agent they own, which has its own credential" "$NAMES" '"name":"#holder-svc@srv1"'
# The view answers what you hold, not what you own: a name with no credential
# has nothing to show and is simply absent.
is_empty "but not one they own and have never got a credential for" \
  "$(printf '%s' "$NAMES" | grep -o '#holder-cold@srv1')"
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
  "$(printf '%s' "$NAMES" | grep -o '#alice-held@srv1')"
OTHER=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $alice" "http://unix/names")
is_empty "and mine are not in theirs" "$(printf '%s' "$OTHER" | grep -o '#holder-svc@srv1')"
# A rotation is a new credential, so it has a new fingerprint: one that did
# not change would say the old token still works.
FP0=$(printf '%s' "$NAMES" | sed -n 's/.*"name":"holder@srv1","fingerprint":"\([0-9a-f]*\)".*/\1/p')
# Straight from the rotation, not through tok(), which caches to a file and
# would hand back the credential that has just been replaced.
RTOK=$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token holder@srv1 --rotate 2>/dev/null)
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
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+14)) --socket "$D/r3/bus.sock" \
    --owner "$OWNER" --db "$D/r3/bus.db" --create --flush-every 0 >"$D/daemon3-$1.log" 2>&1 &
  NPID=$!
  ready "$D/r3/bus.sock" || echo "  WARNING: $D/r3/bus.sock never answered"
  R3TOK=$(owner_token "$D/r3")
}
r3_up first
# Registered and kept, for the same reason as the rotation checks above.
AGENT_BUS_ADDR=$D/r3/bus.sock AGENT_BUS_TOKEN=$R3TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" register '#kept-holder@srv1' --kind agent --allow '*' >/dev/null
HTOK=$(AGENT_BUS_ADDR=$D/r3/bus.sock AGENT_BUS_TOKEN=$R3TOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#kept-holder@srv1' --rotate 2>/dev/null)
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
ab owner@srv1 register '#refused-by@srv1' --kind agent --allow owner@srv1 >/dev/null
n0=$(count credential)
tcode not-a-token /status >/dev/null 2>&1
delta "a credential that is not one is counted as that" 1 "$n0" "$(count credential)"
n0=$(count unknown)
ab caller@srv1 send nobody-at-all@srv1 "into the void" >/dev/null 2>&1
delta "an unknown receiver is counted as unknown" 1 "$n0" "$(count unknown)"
n0=$(count acl)
ab stranger@srv1 send '#refused-by@srv1' "let me in" >/dev/null 2>&1
delta "a call the ACL refuses is counted as acl" 1 "$n0" "$(count acl)"
n0=$(count malformed)
ab owner@srv1 register '#refused-by@srv1' --kind agent --allow '*' --overflow nonsense >/dev/null 2>&1
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
out=$(ab '#asker@srv1' send ghost@nowhere hi 2>&1); rc=$?
has "says no such receiver" "$out" 'no such receiver'; bad_exit "and exits non-zero" $rc

sec "a name that is not a name"
bad_exit "register without --kind agent a realm" "$(ab '#asker@srv1' register no-realm --allow '*' >/dev/null 2>&1; echo $?)"

sec "one name, however it is spelled: trim, lower-case, ASCII"
ab owner@srv1 register '  #PAD@Srv1  ' --allow '*' --kind agent >/dev/null
has "ls shows the canonical form" "$(ab '#asker@srv1' ls --all)" '"name":"#pad@srv1"'
ab '#asker@srv1' send ' #Pad@SRV1 ' --topic pad --tag p "padded name" >/dev/null
has "a padded, upper-case send reaches it" "$(ab '#pad@srv1' consume --topic pad --tag p --wait 3s)" 'padded name'
bad_exit "a non-ASCII name is refused" "$(ab '#asker@srv1' register 'pärf@srv1' --kind agent --allow '*' >/dev/null 2>&1; echo $?)"

sec "call and ack: an agent answers, and says it got the message first"
ab owner@srv1 register '#svc@srv1' --allow '*' --kind agent --descr "answers calls" >/dev/null
(
  msg=$(ab '#svc@srv1' consume --wait 10s)
  id=$(printf '%s' "$msg" | sed 's/.*"message_id":"\([^"]*\)".*/\1/')
  [ -n "$id" ] && ab '#svc@srv1' ack "$id" >/dev/null
  [ -n "$id" ] && ab '#svc@srv1' reply "$id" "the answer is 42" >/dev/null
) &
SPID=$!
out=$(ab caller@srv1 call '#svc@srv1' --wait 15s "what is the answer?" 2>"$D/call.err"); rc=$?
wait $SPID 2>/dev/null
ok_exit "call returns" $rc
has "call gets the answer, not the receipt" "$out" 'the answer is 42'
has "the ack was seen and reported" "$(cat "$D/call.err")" 'ack from '#svc@srv1''
has "a call to nobody fails" "$(ab caller@srv1 call ghost@nowhere --wait 2s hi 2>&1)" 'no such receiver'

sec "channels: a publisher with no record of its own, a consumer that was down"
users reader@srv1
ab owner@srv1 channel create jobs@srv1 --allow '*' --descr "work queue" >/dev/null
has "the topic is in ls" "$(ab owner@srv1 ls --kind queue)" 'jobs@srv1'
ab drive-by@srv1 publish --channel jobs@srv1 "sweep the floor" >/dev/null
has "a consumer that was down still finds it" "$(ab reader@srv1 consume --inbox jobs@srv1 --wait 5s)" 'sweep the floor'
# Reading a topic and filtering your own inbox are different inboxes, so the
# check needs a message in each: asserting "nothing came back" passed happily
# with the rule mutated to read the topic in both cases.
ab '#asker@srv1' send caller@srv1 --topic jobs@srv1 --tag mine "PRIVATE" >/dev/null
ab drive-by@srv1 publish --channel jobs@srv1 "TOPIC" >/dev/null
has "a topic plus a tag filters my own inbox" "$(ab caller@srv1 consume --topic jobs@srv1 --tag mine --wait 3s)" 'PRIVATE'
has "an explicit inbox reads that inbox" "$(ab caller@srv1 consume --inbox jobs@srv1 --wait 3s)" 'TOPIC'
out=$(ab caller@srv1 consume --inbox jobz@srv1 --wait 1s 2>&1); rc=$?
bad_exit "an unknown explicit inbox is refused" $rc
has "and says which name" "$out" 'no inbox for jobz@srv1'

ab owner@srv1 register '#keeper@srv1' --kind agent --allow '*' >/dev/null || exit 1
}
call_damage() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1
ab owner@srv1 register '#svc@srv1' --allow '*' --kind agent --descr "answers calls" >/dev/null || exit 1
ab owner@srv1 register '#keeper@srv1' --kind agent --allow '*' >/dev/null || exit 1
if normal; then
  sec "a call does not damage what it calls from"
  ab owner@srv1 register '#keeper@srv1' --allow '*' --kind agent --descr "KEEP ME" >/dev/null
  # the whole record, not a word from it: kind, addr, description, owner and the
  # timestamp all change if the caller re-states itself. last_used is the one
  # field a call is meant to move (docs/05-discovery.md#what-a-listing-answers).
  record() { ab '#keeper@srv1' ls --all | grep -o '{[^}]*"name":"#keeper@srv1"[^}]*}' | sed 's/,"last_used":"[^"]*"//'; }
  before=$(record)
  ab '#keeper@srv1' call '#svc@srv1' --wait 1s "nobody is listening" >/dev/null 2>&1
  after=$(record)
  if [ -n "$before" ] && [ "$before" = "$after" ]; then
    echo "  ok   the caller's own record survives its call"; pass=$((pass+1))
  else
    echo "  FAIL the caller's own record survives its call: [$before] became [$after]"; fail=$((fail+1))
  fi
  ab owner@srv1 register '#unheard@srv1' --allow '*' --kind agent >/dev/null
  out=$(ab caller@srv1 call '#unheard@srv1' --wait 5q "typo" 2>&1); rc=$?
  bad_exit "a bad --wait is refused" $rc
  is_empty "and refused before the message is sent" "$(ab '#unheard@srv1' consume --wait 1s)"
  # The status code, not the body: the accepted envelope echoes the field back,
  # so grepping for "receipt" passed with the check for it removed.
  has "a third receipt value is refused" \
    "$(post_code caller@srv1 /send '{"to":"#svc@srv1","receipt":"maybe","body":"x"}')" '400'
  has "and the two real ones are not" \
    "$(post_code caller@srv1 /send '{"to":"#svc@srv1","receipt":"done","re":"0","body":"x"}')" '200'

else skipped=$((skipped+1)); fi
}
core_3() {
sec "a shell script is an agent"
users greeter@srv1 launcher@srv1
for name in hello envelope defaulted; do ab owner@srv1 register "#$name@srv1" --kind agent --allow '*' >/dev/null || exit 1; done
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
abx '#hello@srv1' run '#hello@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "greets you" >"$D/start.log" 2>&1 &
HPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'greets you' && break; sleep 0.2; done
has "the script is registered and discoverable" "$(ab '#asker@srv1' ls --all)" 'greets you'
has "it answers a call" "$(ab greeter@srv1 call '#hello@srv1' --wait 15s world)" 'Hello world'
kill $HPID 2>/dev/null; wait $HPID 2>/dev/null

env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN XDG_RUNTIME_DIR=$D/discovery \
  "$D/agent-bus" run '#local-script@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "discovered local runner" >"$D/local-start.log" 2>&1 &
LOCALPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls '#local-script@srv1' 2>/dev/null | grep -q 'discovered local runner' && break; sleep 0.1; done
has "a runner on a discovered user socket switches to its agent identity" \
  "$(ab greeter@srv1 call '#local-script@srv1' --wait 5s discovery)" 'Hello discovery'
kill $LOCALPID 2>/dev/null; wait $LOCALPID 2>/dev/null
# The account socket alone is enough (docs/02-access.md#local-socket): with
# no shared socket beside it — as on a host it was forwarded to — the runner
# stays on it and the daemon takes its agent's token there.
mkdir -p "$D/onesock"
ln -sf "$D/user-$ACCOUNT.sock" "$D/onesock/user-$ACCOUNT.sock"
env -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME AGENT_BUS_ADDR=$D/onesock/user-$ACCOUNT.sock \
  "$D/agent-bus" run '#one-socket@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "one socket runner" >"$D/onesock.log" 2>&1 &
ONEPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls '#one-socket@srv1' 2>/dev/null | grep -q 'one socket runner' && break; sleep 0.1; done
has "a runner on the account socket alone answers as its agent" \
  "$(ab greeter@srv1 call '#one-socket@srv1' --wait 5s onesock)" 'Hello onesock'
kill $ONEPID 2>/dev/null; wait $ONEPID 2>/dev/null

cat > "$D/envelope.sh" <<'SH'
#!/bin/sh
envelope=$(cat)
case "$envelope" in *payload*) got=yes ;; *) got=no ;; esac
case "$envelope" in *'"ask"'*) askin=yes ;; *) askin=no ;; esac
echo "stdin=$got topic=$AGENT_BUS_TOPIC from=$AGENT_BUS_FROM ask=$AGENT_BUS_ASK askin=$askin"
SH
chmod +x "$D/envelope.sh"
abx '#envelope@srv1' run '#envelope@srv1' --allow '*' --algo json "$D/envelope.sh" -2 --descr "reads the envelope" >>"$D/start.log" 2>&1 &
SPID2=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'reads the envelope' && break; sleep 0.2; done
has "json gets the envelope on stdin and in the environment" \
  "$(ab greeter@srv1 call '#envelope@srv1' --topic t9 --wait 15s payload)" 'stdin=yes topic=t9 from=greeter@srv1 ask= askin=no'
# An ask is the envelope's field, told in AGENT_BUS_ASK alone: the script's
# input carries neither it nor any text (docs/constitution.md#agent-self-description).
has "an ask reaches a script as AGENT_BUS_ASK, with no input and no ask in it" \
  "$(ab greeter@srv1 call '#envelope@srv1' --topic t9 --wait 15s --help)" 'stdin=no topic=t9 from=greeter@srv1 ask=help askin=no'
kill $SPID2 2>/dev/null; wait $SPID2 2>/dev/null

# The default form, stated in docs/06-dedicated-runner.md#script-agents, is what
# an agent gets when it says nothing.
abx '#defaulted@srv1' run '#defaulted@srv1' --allow '*' "$D/hello-world.sh" --descr "says no form" >>"$D/start.log" 2>&1 &
NOFORMPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'says no form' && break; sleep 0.2; done
has "no form named passes the body as an argument" \
  "$(ab greeter@srv1 call '#defaulted@srv1' --topic t10 --wait 15s payload)" 'Hello payload'
kill $NOFORMPID 2>/dev/null; wait $NOFORMPID 2>/dev/null

# An agent can be described as JSON on stdin instead of in flags, asked for
# with `-`; `-3` beside it still means three at a time.
echo "{\"name\":\"#fromfile@srv1\",\"algo\":\"args\",\"script\":\"$D/hello-world.sh\",\"descr\":\"from a file\"}" \
  | abx launcher@srv1 run -3 - --allow '*' >>"$D/start.log" 2>&1 &
JPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'from a file' && break; sleep 0.2; done
has "an agent described as JSON on stdin" "$(ab greeter@srv1 call '#fromfile@srv1' --wait 15s again)" 'Hello again'
kill $JPID 2>/dev/null; wait $JPID 2>/dev/null

# Without a script or `-` there is nothing to serve: run says how, and where
# to read more, rather than waiting on stdin for JSON nobody knows to type.
# Bounded, since the failure is a hang (K.38).
has "a bare run answers with the guide instead of waiting" \
  "$(timeout 10 env AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok launcher@srv1)" AGENT_BUS_NAME=launcher@srv1 \
      "$D/agent-bus" run </dev/null 2>&1)" 'github.com/parf/ai-agent-bus/blob/main/docs/user/dedicated-runner.md'
# The dedicated runner is `run`, and only that: no second name answers for it.
out=$(timeout 10 env AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok launcher@srv1)" AGENT_BUS_NAME=launcher@srv1 \
  "$D/agent-bus" start '#nostart@srv1' --algo args /bin/echo </dev/null 2>&1); rc=$?
bad_exit "there is no start verb" $rc
has "and it is unknown, not a dedicated runner" "$out" 'unknown verb "start"'

# The runner is its person's agent, reads what is installed, answers
# status and reload, and serves each installed agent as that agent (M.1, M.2),
# each created for its person with the token that creation answered (M.10);
# a malformed agent.json is refused by name, never skipped
# (Plans/R1.0-Release/TODO.md#runner-waves). The status answer is JSON
# inside the envelope's body, so its quotes arrive escaped.
# Isolation needs a home outside /usr; the private shard mktemp has an EXIT cleanup trap.
RH="$D/runner-home"
mkdir -p "$RH/agent.d/hello" "$RH/agent.d/typo" "$RH/agent.d/mail" "$RH/agent.d/slow" "$RH/agent.d/fast" "$RH/runner/mail/work"
printf '#!/bin/sh\necho "hello, $1"\n' >"$RH/agent.d/hello/hello.sh"
printf '#!/bin/sh\necho "mail for $IMAP_USER at $IMAP_HOST"\n' >"$RH/agent.d/mail/fetch.sh"
printf '#!/bin/sh\nsleep 6; echo slow\n' >"$RH/agent.d/slow/slow.sh"
printf '#!/bin/sh\necho fast\n' >"$RH/agent.d/fast/fast.sh"
chmod 755 "$RH"/agent.d/*/*.sh
echo '{"description":"Smoke fixture","command":"./hello.sh","form":"args","processes":"0..3"}' >"$RH/agent.d/hello/agent.json"
echo '{"comand":"./x.sh"}' >"$RH/agent.d/typo/agent.json"
echo '{"description":"Smoke fixture","command":"./fetch.sh","form":"args"}' >"$RH/agent.d/mail/agent.json"
printf '# the account to fetch\nIMAP_HOST=imap.example.com  # a default\nIMAP_USER= # required\n' >"$RH/agent.d/mail/env.dist"
echo "IMAP_USER='bob'" >"$RH/runner/mail/work/env"
echo '{"description":"Smoke fixture","command":"./slow.sh","form":"args","processes":1}' >"$RH/agent.d/slow/agent.json"
echo '{"description":"Smoke fixture","command":"./fast.sh","form":"args","processes":1}' >"$RH/agent.d/fast/agent.json"
"$PWD/agents/debug/package.sh" "$RH/agent.d/rdebug"
cp -r "$RH/agent.d/rdebug" "$RH/agent.d/docdebug"
# Keep the RD105 per-call injection fixture: native debug is tested separately.
python3 - "$RH/agent.d/rdebug/agent.json" <<'DEBUGARGS'
import json,sys
p=sys.argv[1]; m=json.load(open(p)); m.pop('serves'); m['processes']='0..1'
open(p,'w').write(json.dumps(m))
DEBUGARGS
# What a sandboxed agent can see of the host: no /etc, no /home, only a
# loopback interface, and only its own processes; the same script with the
# sandbox off is the control that sees them (runner.md#sandboxing).
mkdir -p "$RH/agent.d/jail" "$RH/agent.d/free"
for a in jail free; do
  printf '#!/bin/sh\n[ -e /etc/passwd ] && e=yes || e=no\n[ -e /home ] && h=yes || h=no\nn=$(grep -c ":" /proc/net/dev)\necho "etc=$e home=$h ifaces=$n"\n' >"$RH/agent.d/$a/probe.sh"
  chmod 755 "$RH/agent.d/$a/probe.sh"
  echo '{"description":"Smoke fixture","command":"./probe.sh","form":"args"}' >"$RH/agent.d/$a/agent.json"
done
# What an agent cannot reach, from inside its sandbox: other processes, the
# bus's sockets, a sibling's work directory, anything but its work directory
# to write; and the limits its cgroup and the runner give it (M.4). The
# message names the paths to look for.
mkdir -p "$RH/agent.d/pen" "$RH/agent.d/hog" "$RH/agent.d/forky" "$RH/agent.d/filler"
printf '#!/bin/sh\nhead -c 2097152 /dev/zero >"./fill.$(date +%%s%%N)" && echo filled\n' >"$RH/agent.d/filler/fill.sh"; chmod 755 "$RH/agent.d/filler/fill.sh"
echo '{"description":"Smoke fixture","command":"./fill.sh","form":"args"}' >"$RH/agent.d/filler/agent.json"
printf '#!/bin/sh\np=$(ls -d /proc/[0-9]* | wc -l)\nseen=no; for f in $1; do [ -e "$f" ] && seen=yes; done\ntouch /usr/x 2>/dev/null && w=yes || w=no\ntouch "$(dirname "$0")/x" 2>/dev/null && w=yes\ntouch ./mine 2>/dev/null && m=yes || m=no\nunshare -U true 2>/dev/null && u=yes || u=no\necho "procs=$p seen=$seen wrote=$w mine=$m userns=$u files=$(ulimit -n) tmpkb=$(df -k /tmp | awk '"'"'NR==2{print $2}'"'"')"\n' >"$RH/agent.d/pen/pen.sh"
cat >"$RH/agent.d/hog/hog.sh" <<'HOG'
#!/bin/sh
echo "held $(python3 -c "a = bytearray([120]) * ($1 << 20); print(len(a) >> 20)")"
HOG
# Forks that succeed, each child sleeping on: 20 unless something bounds them.
cat >"$RH/agent.d/forky/forky.sh" <<'FORKY'
#!/usr/bin/python3
import os, time
n = 0
for _ in range(20):
    try:
        pid = os.fork()
    except OSError:
        break
    if pid == 0:
        time.sleep(3)
        os._exit(0)
    n += 1
print("forked", n)
FORKY
chmod 755 "$RH/agent.d/pen/pen.sh" "$RH/agent.d/hog/hog.sh" "$RH/agent.d/forky/forky.sh"
echo '{"description":"Smoke fixture","command":"./pen.sh","form":"args"}' >"$RH/agent.d/pen/agent.json"
# Namespaces by the other doors: clone with a namespace flag, and clone3,
# which must answer ENOSYS so that threads still start, through clone
# (x86_64 numbers; this host's).
mkdir -p "$RH/agent.d/ns"
cat >"$RH/agent.d/ns/ns.py" <<'NS'
#!/usr/bin/python3
import ctypes, os, threading
libc = ctypes.CDLL(None, use_errno=True)
pid = libc.syscall(56, 0x10000000 | 17, 0, 0, 0, 0)
if pid == 0:
    os._exit(0)
if pid > 0:
    os.waitpid(pid, 0)
r = libc.syscall(435, 0, 0)
t = threading.Thread(target=lambda: None)
t.start()
t.join()
print("clonens=%s clone3=%d threads=ok" % ("yes" if pid > 0 else "no", ctypes.get_errno() if r < 0 else 0))
NS
chmod 755 "$RH/agent.d/ns/ns.py"
echo '{"description":"Smoke fixture","command":"./ns.py","form":"args"}' >"$RH/agent.d/ns/agent.json"
echo '{"description":"Smoke fixture","command":"./hog.sh","form":"args"}' >"$RH/agent.d/hog/agent.json"
echo '{"description":"Smoke fixture","command":"./forky.sh","form":"args"}' >"$RH/agent.d/forky/agent.json"
# An agent that serves itself (M.9): the runner keeps its process running
# with the agent's own token and the bus socket, and restarts it when it
# exits; it reads its own inbox and answers, here through the CLI in its own
# directory. Its greeting comes from its host env file.
mkdir -p "$RH/agent.d/selfie" "$RH/runner/selfie" "$RH/agent.d/selfrange"
cp "$D/agent-bus" "$RH/agent.d/selfie/agent-bus"
cat >"$RH/agent.d/selfie/selfie.sh" <<'SELFIE'
#!/bin/sh
here=$(dirname "$0")
trap 'touch "$AGENT_BUS_WORK/termed"; exit 0' TERM
while :; do
  m=$("$here/agent-bus" consume --wait 20s 2>/dev/null) || { sleep 1; continue; }
  id=$(printf '%s' "$m" | sed -n 's/.*"message_id":"\([^"]*\)".*/\1/p')
  body=$(printf '%s' "$m" | sed -n 's/.*"body":"\([^"]*\)".*/\1/p')
  [ -n "$id" ] && "$here/agent-bus" reply "$id" "$SELFIE_GREETING: $body" >/dev/null 2>&1
done
SELFIE
chmod 755 "$RH/agent.d/selfie/selfie.sh"
echo '{"description":"Smoke fixture","command":"./selfie.sh","serves":"itself","processes":1}' >"$RH/agent.d/selfie/agent.json"
echo 'SELFIE_GREETING=hello from itself' >"$RH/runner/selfie/env"
echo '{"description":"Smoke fixture","command":"./selfie.sh","serves":"itself","processes":"1..2"}' >"$RH/agent.d/selfrange/agent.json"
mkdir -p "$RH/agent.d/selfnew"; cp "$RH/agent.d/selfie/selfie.sh" "$RH/agent.d/selfnew/"
echo '{"description":"Smoke fixture","command":"./selfie.sh","serves":"itself","processes":1}' >"$RH/agent.d/selfnew/agent.json"
mkdir -p "$RH/agent.d/privateerr" "$RH/runner/privateerr"
printf '#!/bin/sh\nprintf "%%s %%s\\n" "$FILE_SECRET" "$BUS_SECRET" >&2\nexit 1\n' >"$RH/agent.d/privateerr/run.sh"
chmod 755 "$RH/agent.d/privateerr/run.sh"
echo '{"description":"Private stderr fixture","command":"./run.sh","form":"args"}' >"$RH/agent.d/privateerr/agent.json"
echo 'FILE_SECRET=rd105-file-sentinel' >"$RH/runner/privateerr/env"
# An env file may not speak for the runner: what it says of the sender loses
# to the runner's own stamp.
mkdir -p "$RH/runner/rdebug"; printf 'AGENT_BUS_FROM=spoof\nFILE_SECRET=rd105-file-sentinel\n' >"$RH/runner/rdebug/env"
echo '{"hello":{"enabled":true},"mail":{"enabled":true},"mail/work":{"enabled":true},"free":{"sandbox":"off"},"pen":{"limits":{"files":64}},"hog":{"limits":{"memory":"64M"}},"forky":{"limits":{"processes":8}},"filler":{"limits":{"disk":"1M"}},"selfie":{"limits":{"memory":"256M","processes":96,"cpu":"75%","io_weight":77}}}' >"$RH/runner/agents.json"
# An agent of launcher's that the runner did not create: it is not taken.
mkdir -p "$RH/agent.d/held"; printf '#!/bin/sh\necho held\n' >"$RH/agent.d/held/held.sh"; chmod 755 "$RH/agent.d/held/held.sh"
echo '{"description":"Smoke fixture","command":"./held.sh","form":"args"}' >"$RH/agent.d/held/agent.json"
ab launcher@srv1 register '#held@srv1' --kind agent >/dev/null
# Started with its person's own token, it refuses: it is never its person.
# Bounded, since a runner that accepted would serve on.
out=$(AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok launcher@srv1)" timeout 10 "$D/agent-bus-runner" --agents "$RH/agent.d" --host "$RH/runner" 2>&1); rc=$?
has "a runner given its person's own token refuses to run" "$out" 'launcher@srv1 is a person, not a runner'
bad_exit "and exits" $rc
ab launcher@srv1 register '#runner/launcher@srv1' --kind agent --allow '@owner' >/dev/null
# An agent that serves itself is registered by its Owner, never the runner
# (Q151), and its token handed over once, as for any existing agent.
ab launcher@srv1 register '#selfie@srv1' --kind agent >/dev/null
mkdir -p "$RH/runner/tokens" && AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#selfie@srv1' >"$RH/runner/tokens/%23selfie@srv1"
ab launcher@srv1 register '#docdebug@srv1' --kind agent --allow '@owner' >/dev/null
AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#docdebug@srv1' >"$RH/runner/tokens/%23docdebug@srv1"
AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok '#runner/launcher@srv1')" \
  systemd-run --user --scope --quiet -p Delegate=yes "$D/agent-bus-runner" --agents "$RH/agent.d" --host "$RH/runner" >>"$D/runner.log" 2>&1 &
RUNPID=$!
for _ in $(seq 1 50); do grep -q '#runner/launcher@srv1 is the runner' "$D/runner.log" && break; sleep 0.2; done
# Who may call an agent is its record's, set the ordinary way, not the
# runner's (Plans/R1.0-Release/runner-config.md#where-an-option-lives).
for a in hello mail/work rdebug slow fast jail free held pen hog forky filler ns selfie; do ab launcher@srv1 manage "#$a@srv1" --add-allow '*' >/dev/null; done
doctor=$(ab launcher@srv1 call '#docdebug@srv1' --wait 20s selftest)
has "native debug selftest labels local and bus checks" "$doctor" 'Local handler checks; KV and lock use the bus'
lacks "native debug selftest has no failed check" "$doctor" '❌'
has "native debug selftest has six successful checks" "$(printf '%s' "$doctor" | grep -o '✅' | wc -l)" '^6$'
has "native debug masks its own credential" "$(ab launcher@srv1 call '#docdebug@srv1' --wait 10s env)" 'AGENT_BUS_TOKEN=\*\*\*'
ab launcher@srv1 call '#docdebug@srv1' --wait 1s crash >/dev/null 2>&1
has "runner restarts crashed debug and it answers again" "$(ab launcher@srv1 call '#docdebug@srv1' --wait 20s ping)" 'answered_at'
st=$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s status)
has "an agent the runner created is its person's" \
  "$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok launcher@srv1)" "http://unix/lookup?name=%23hello@srv1")" '"owner":"launcher@srv1"'
# What the next check reads is what the runner holds: the token it kept for
# what it created; its running identity comes from its status answer.
# Asked afresh, not through tok: an earlier #hello@srv1 in this run left its
# own credential in tok's cache.
has "it keeps the token the creating register answered" "$(cat "$RH/runner/tokens/%23hello@srv1" 2>/dev/null)" \
  "^$(AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#hello@srv1')\$"
has "and answers as its own agent" "$st" '\\"runner\\":\\"#runner/launcher@srv1\\"'
has "an existing agent it was never handed is refused by name" "$st" '\\"name\\":\\"#held@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"#held@srv1 exists and its token is not kept here'
lacks "and the runner holds no token of its person's" "$(cat "$RH"/runner/tokens/* 2>/dev/null)" "$(tok launcher@srv1)"
has "the runner serves an installed agent" "$st" '\\"name\\":\\"#hello@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\",\\"processes\\":{\\"running\\":0,\\"min\\":0,\\"max\\":3}'
has "a malformed agent.json is refused by name" "$st" '\\"name\\":\\"#typo@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"agent.json: json: unknown field'
has "missing required key refuses the plain agent" "$st" '\\"name\\":\\"#mail@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"needs IMAP_USER'
hi=$(ab greeter@srv1 call '#hello@srv1' --wait 15s world)
has "a call to an agent is answered through the runner" "$hi" '"body":"hello, world"'
has "and the answer comes from the agent that was addressed" "$hi" '"from":"#hello@srv1"'
has "an instance is served with its layered env" "$(ab greeter@srv1 call '#mail/work@srv1' --wait 15s go)" '"body":"mail for bob at imap.example.com"'
ab launcher@srv1 secret '#privateerr@srv1' 'BUS_SECRET=rd105-bus-sentinel' >/dev/null
ab launcher@srv1 secret '#rdebug@srv1' 'BUS_SECRET=rd105-bus-sentinel' >/dev/null
ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload >/dev/null
private_answer=$(ab launcher@srv1 call '#privateerr@srv1' --wait 15s go)
private_status=$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s status)
lacks "failed script status and answer contain no injected secret" "$private_answer $private_status" 'rd105-.*sentinel'
has "failed script status retains a masked reason" "$private_status" 'reason.*\*\*\* \*\*\*'
dbg=$(ab greeter@srv1 call '#rdebug@srv1' --wait 15s env)
lacks "debug env masks host and bus secrets" "$dbg" 'rd105-.*sentinel'
has "debug env keeps private variable names" "$dbg" 'FILE_SECRET=\*\*\*'
has "debug env masks the bus Secret too" "$dbg" 'BUS_SECRET=\*\*\*'
secret_read=$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s read runner/rdebug/env)
lacks "runner env read never returns value bytes" "$secret_read" 'rd105-.*sentinel|content'
has "runner env read returns its digest" "$secret_read" "$(sha256sum "$RH/runner/rdebug/env" | cut -d' ' -f1)"
has "the shipped debug agent runs under the runner" "$dbg" 'AGENT_BUS_TO=#rdebug@srv1'
lacks "and is handed no token" "$dbg" 'AGENT_BUS_TOKEN='
has "and is told its real sender, whatever an env file says" "$dbg" 'AGENT_BUS_FROM=greeter@srv1'
lacks "so an env file cannot spoof one" "$dbg" 'AGENT_BUS_FROM=spoof'
debug_help=$(ab greeter@srv1 call '#rdebug@srv1' --wait 15s --help)
debug_manifest_help=$(python3 -c 'import json,sys; description=json.load(open(sys.argv[1]))["description"]; body=json.loads(sys.argv[2])["body"]; print("match" if body.startswith(description + "\n") and "More: agent-bus help" in body and "config | secret | json" in body else "mismatch")' "$RH/agent.d/rdebug/agent.json" "$debug_help")
has "asked --help, debug gets its manifest description without handwritten help" "$debug_manifest_help" '^match$'
# Packaged scripts need no help code; ordinary empty input still reaches them.
has "packaged help is answered from metadata without running the script" \
  "$(ab greeter@srv1 call '#hello@srv1' --wait 15s --help)" 'Smoke fixture'
has "ordinary empty input still runs the script" "$(ab greeter@srv1 call '#hello@srv1' --wait 15s '')" '"body":"hello, "'
has "asked --mcp, the runner answers the MCP shape for none" \
  "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s --mcp)" '"body":"{\\"tools\\":\[\]}"'
has "and so does the shipped debug agent" \
  "$(ab greeter@srv1 call '#rdebug@srv1' --wait 15s --mcp)" '"body":"{\\"tools\\":\[\]}'
has "asked --help, the runner describes its calls" \
  "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s --help)" 'a person'"'"'s agent runner'
jail=$(ab greeter@srv1 call '#jail@srv1' --wait 15s x)
has "a sandboxed agent sees no /etc and no /home" "$jail" '"body":"etc=no home=no'
has "and has no network but loopback" "$jail" 'ifaces=1"'
pen=$(ab greeter@srv1 call '#pen@srv1' --wait 15s "$D/bus.sock $D/user-$ACCOUNT.sock $RH/runner/work/hello $RH/runner/tokens")
has "a sandboxed agent sees only its own few processes" "$pen" '"body":"procs=[1-5] '
has "and none of the bus's sockets, a sibling's work or the runner's files" "$pen" ' seen=no '
has "it writes nowhere but its work directory" "$pen" ' wrote=no mine=yes '
has "it cannot make a namespace of its own: the system-call filter says no" "$pen" ' userns=no '
ns=$(ab greeter@srv1 call '#ns@srv1' --wait 15s x)
has "nor by clone with a namespace flag" "$ns" '"body":"clonens=no '
has "and clone3 answers ENOSYS, so its threads still start" "$ns" ' clone3=38 threads=ok"'
has "its open files are its limit" "$pen" ' files=64 '
has "and its /tmp is bounded, by its memory limit" "$pen" ' tmpkb=524288"'
has "within its memory an agent works" "$(ab greeter@srv1 call '#hog@srv1' --wait 15s 8)" '"body":"held 8"'
# Deliberate OOM and pids-controller rejection are release/stress checks.
# Normal proves the same limit application below without crossing a boundary.
if slow; then
  lacks "and past it the cgroup stops it" "$(ab greeter@srv1 call '#hog@srv1' --wait 6s 200 2>&1)" 'held 200'
  has "its processes are bounded" "$(ab greeter@srv1 call '#forky@srv1' --wait 15s x)" '"body":"forked [0-7]"'
fi
# A work directory past its soft limit stops the agent after the run that
# filled it; status says why by name, and once emptied a reload serves it.
has "a run may fill its work directory" "$(ab greeter@srv1 call '#filler@srv1' --wait 15s x)" '"body":"filled"'
has "and past its disk limit the agent is stopped, by name" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s status)" '\\"name\\":\\"#filler@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"its work directory holds 2097152 bytes, over its disk limit of 1048576'
lacks "so it takes no more work" "$(ab greeter@srv1 call '#filler@srv1' --wait 4s x 2>&1)" '"body":"filled"'
has "a reload while it is still full keeps it stopped" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload)" '\\"name\\":\\"#filler@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"its work directory holds 2097152 bytes'
rm -f "$RH"/runner/work/filler/fill.*
has "emptied, a reload serves it again" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload)" '\\"name\\":\\"#filler@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
# What the host sees: the agent's own cgroup, beside the runner's leaf, with
# its limits written to the kernel's files.
CG=/sys/fs/cgroup$(sed -n 's/^0:://p' /proc/$RUNPID/cgroup); CG=${CG%/runner}
has "each agent has a cgroup of its own, with its memory limit" "$(cat "$CG"/agent-hog-*/memory.max 2>&1)" '^67108864$'
has "and no swap to hide in" "$(cat "$CG"/agent-hog-*/memory.swap.max 2>&1)" '^0$'
if slow; then
  has "and it was the kernel that stopped the one over it" "$(cat "$CG"/agent-hog-*/memory.events 2>&1)" '^oom_kill [1-9]'
else
  has "normal runner fixtures cause no cgroup OOM kills" "$(awk '$1 == "oom_kill" {seen=1; if ($2 != 0) bad=1} END {print seen && !bad ? "quiet" : "missing counter or OOM"}' "$CG"/agent-*/memory.events)" '^quiet$'
  has "normal runner fixtures cause no pids-controller rejections" "$(awk '$1 == "max" {seen=1; if ($2 != 0) bad=1} END {print seen && !bad ? "quiet" : "missing counter or fork rejected"}' "$CG"/agent-*/pids.events)" '^quiet$'
fi
# The agent that serves itself (M.9).
self_pids() { for p in $(cat "$CG"/agent-selfie-*/cgroup.procs 2>/dev/null); do [ "$(cat /proc/$p/comm 2>/dev/null)" = bwrap ] || echo $p; done; }
st=$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s status)
has "an agent that serves itself is kept running by the runner" "$st" '\\"name\\":\\"#selfie@srv1\\",\\"enabled\\":true,\\"state\\":\\"running\\",\\"processes\\":{\\"running\\":1'
has "and answers as itself, from its own inbox" "$(ab greeter@srv1 call '#selfie@srv1' --wait 15s hi)" '"from":"#selfie@srv1".*"body":"hello from itself: hi"'
has "inside its own cgroup" "$(self_pids | wc -w)" '^[1-9]'
# Read the live keeper's actual kernel files, not the runner's status/config.
# The same apply() creates groups for one-shot and kept agents.
has "live keeper memory.max is the configured bound" "$(cat "$CG"/agent-selfie-*/memory.max 2>&1)" '^268435456$'
has "live keeper swap is disabled" "$(cat "$CG"/agent-selfie-*/memory.swap.max 2>&1)" '^0$'
has "live keeper pids.max is the configured bound" "$(cat "$CG"/agent-selfie-*/pids.max 2>&1)" '^96$'
has "live keeper cpu.max is the configured quota" "$(cat "$CG"/agent-selfie-*/cpu.max 2>&1)" '^75000 100000$'
has "live keeper io.weight is the configured weight" "$(cat "$CG"/agent-selfie-*/io.weight 2>&1)" '^default 77$'

has "one it was never handed is refused, not registered" "$st" '\\"name\\":\\"#selfnew@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"#selfnew@srv1 serves itself, so the runner does not register it'
has "and the name stays unregistered" "$(code launcher@srv1 '/lookup?name=%23selfnew@srv1')" '^404$'
has "a range of processes is refused for one" "$st" '\\"name\\":\\"#selfrange@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"an agent that serves itself takes a fixed count'
kill -9 $(self_pids) 2>/dev/null
for _ in $(seq 1 40); do grep -q '#selfie@srv1 exited' "$D/runner.log" && break; sleep 0.1; done
has "killed, it is restarted" "$(cat "$D/runner.log")" '#selfie@srv1 exited .*restarting in'
has "and answers again" "$(ab greeter@srv1 call '#selfie@srv1' --wait 20s again)" '"body":"hello from itself: again"'
echo 'SELFIE_GREETING=hello again' >"$RH/runner/selfie/env"
ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload >/dev/null
has "a changed env restarts it, and it is told the new one" "$(ab greeter@srv1 call '#selfie@srv1' --wait 20s now)" '"body":"hello again: now"'
has "with the sandbox off the same script sees the host" "$(ab greeter@srv1 call '#free@srv1' --wait 15s x)" '"body":"etc=yes home=yes'
# A busy agent holds no other agent's message: the runner reads its one inbox
# one agent at a time (Q162). slow takes 6 s and has one process.
ab greeter@srv1 send '#slow@srv1' one >/dev/null
ab greeter@srv1 send '#slow@srv1' two >/dev/null
sleep 0.5
t0=$(date +%s)
has "an idle agent answers while a busy one works" "$(ab greeter@srv1 call '#fast@srv1' --wait 4s now)" '"body":"fast"'
has "and at once" "$(( $(date +%s) - t0 <= 3 ))" '^1$'
mkdir -p "$RH/agent.d/later"
printf '#!/bin/sh\necho later\n' >"$RH/agent.d/later/later.sh"; chmod 755 "$RH/agent.d/later/later.sh"
echo '{"description":"Smoke fixture","command":"./later.sh","form":"args"}' >"$RH/agent.d/later/agent.json"
lacks "an agent installed after the start is not seen before reload" \
  "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s status)" '#later@srv1'
has "reload rereads what is installed" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload)" '\\"name\\":\\"#later@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
ab launcher@srv1 manage '#later@srv1' --add-allow '*' >/dev/null
has "and serves it" "$(ab greeter@srv1 call '#later@srv1' --wait 15s x)" '"body":"later"'
# Its Owner hands the existing agent over once, and it is served. The script
# an earlier `agent-bus run` left on it no longer says what serves it, so it goes.
post_body launcher@srv1 /register '{"kind":"agent","name":"#held@srv1","script":"exec /old/standalone.sh","descr":"held before"}' >/dev/null
has "a handed-over agent carries the old run's script" "$(ab launcher@srv1 ls '#held@srv1')" '"script":"exec /old/standalone.sh"'
tok '#held@srv1' >"$RH/runner/tokens/%23held@srv1"
ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload >/dev/null
has "an agent handed over is served" "$(ab greeter@srv1 call '#held@srv1' --wait 15s x)" '"body":"held"'
held=$(ab launcher@srv1 ls '#held@srv1')
has "and its listing names the record" "$held" '"name":"#held@srv1"'
lacks "and no longer names the old run's script" "$held" '"script"'
has "and keeps its description" "$held" '"descr":"held before"'
# Transferred to somebody else, it is no longer this runner's to serve: the
# next reload drops it, by name, and its new person's runner may read it.
ab launcher@srv1 manage '#held@srv1' --owner greeter@srv1 >/dev/null
has "an agent transferred away is dropped at the next reload" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload)" '\\"name\\":\\"#held@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"#held@srv1 is greeter@srv1'"'"'s now'
# A kept token rotated out of use (twice: the previous one still answers) is
# not served on; handed over again, it is.
for _ in 1 2; do curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" -d '{"name":"#hello@srv1","rotate":true}' http://unix/token >/dev/null; done
has "a served agent whose kept token is dead is refused at the next reload" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload)" '\\"name\\":\\"#hello@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"the token kept for #hello@srv1 is not its any more'
AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#hello@srv1' >"$RH/runner/tokens/%23hello@srv1"
ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s reload >/dev/null
has "and served again once handed over" "$(ab greeter@srv1 call '#hello@srv1' --wait 15s back)" '"body":"hello, back"'
# Restarted, the runner serves what it created with the tokens it kept.
lacks "runner journal contains no host or bus secret sentinel" "$(cat "$D/runner.log")" 'rd105-.*sentinel'
kill $RUNPID 2>/dev/null; wait $RUNPID 2>/dev/null
AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok '#runner/launcher@srv1')" \
  systemd-run --user --scope --quiet -p Delegate=yes "$D/agent-bus-runner" --agents "$RH/agent.d" --host "$RH/runner" >>"$D/runner.log" 2>&1 &
RUNPID=$!
has "after a restart it serves what it created" "$(ab greeter@srv1 call '#hello@srv1' --wait 15s again)" '"body":"hello, again"'
# Without a cgroup subtree of its own the runner cannot bound an agent, so a
# sandboxed one is refused, by name, never run unbounded.
RH2="$D/runner-nocg"; mkdir -p "$RH2/agent.d/lone" "$RH2/runner"
cp "$RH/agent.d/fast/fast.sh" "$RH2/agent.d/lone/"; echo '{"description":"Smoke fixture","command":"./fast.sh","form":"args"}' >"$RH2/agent.d/lone/agent.json"
ab launcher@srv1 register '#nocg@srv1' --kind agent >/dev/null
AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok '#nocg@srv1')" "$D/agent-bus-runner" --agents "$RH2/agent.d" --host "$RH2/runner" >>"$D/runner-nocg.log" 2>&1 &
NOCG=$!
has "a runner with no cgroup of its own refuses a sandboxed agent" "$(ab launcher@srv1 call '#nocg@srv1' --wait 15s status)" '\\"name\\":\\"#lone@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"the sandbox is on and this host cannot give the agent a cgroup'
kill $NOCG 2>/dev/null; wait $NOCG 2>/dev/null
has "the runner refuses a call it does not answer" "$(ab launcher@srv1 call '#runner/launcher@srv1' --wait 15s frobnicate)" 'answers status, reload, start, stop, restart, enable, disable, read, write, create and delete, not \\"frobnicate\\"'
# The host's controls (M.5): stop, start and restart last until a reload;
# enable and disable are written to agents.json and last.
RB='#runner/launcher@srv1'
has "stop takes an agent down, and says it was a call" "$(ab launcher@srv1 call "$RB" --wait 15s stop '#fast@srv1')" '\\"name\\":\\"#fast@srv1\\",\\"enabled\\":true,\\"state\\":\\"stopped\\",\\"reason\\":\\"stopped by a call'
lacks "so it answers nothing" "$(ab greeter@srv1 call '#fast@srv1' --wait 3s x 2>&1)" '"body":"fast"'
has "a stopped agent's sibling still answers" "$(ab greeter@srv1 call '#later@srv1' --wait 15s x)" '"body":"later"'
has "start serves it again" "$(ab launcher@srv1 call "$RB" --wait 15s start fast)" '\\"name\\":\\"#fast@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
has "and it answers" "$(ab greeter@srv1 call '#fast@srv1' --wait 15s x)" '"body":"fast"'
ab launcher@srv1 call "$RB" --wait 15s stop '#fast@srv1' >/dev/null
has "a stop lasts only until reload" "$(ab launcher@srv1 call "$RB" --wait 15s reload)" '\\"name\\":\\"#fast@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
has "disable writes the host's file" "$(ab launcher@srv1 call "$RB" --wait 15s disable '#later@srv1')" '\\"name\\":\\"#later@srv1\\",\\"enabled\\":false,\\"state\\":\\"disabled\\"'
has "and is there, beside what the host wrote" "$(tr -d ' \n' <"$RH/runner/agents.json")" '"later":{"enabled":false}.*"mail/work":{"enabled":true}.*"pen":{"limits":{"files":64}}'
has "so it outlasts a reload" "$(ab launcher@srv1 call "$RB" --wait 15s reload)" '\\"name\\":\\"#later@srv1\\",\\"enabled\\":false,\\"state\\":\\"disabled\\"'
has "enable serves it again" "$(ab launcher@srv1 call "$RB" --wait 15s enable later)" '\\"name\\":\\"#later@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
has "a control names an installed agent" "$(ab launcher@srv1 call "$RB" --wait 15s stop '#nope@srv1')" 'error: no agent #nope@srv1 is installed here'
has "and only one" "$(ab launcher@srv1 call "$RB" --wait 15s stop)" 'error: stop takes one agent'
has "a control's verb is exact" "$(ab launcher@srv1 call "$RB" --wait 15s Stop '#fast@srv1')" 'answers status, reload, start'
has "enable writes an entry for an agent that had none" "$(ab launcher@srv1 call "$RB" --wait 15s enable '#fast@srv1'; tr -d ' \n' <"$RH/runner/agents.json")" '"fast":{"enabled":true}'
# A message sent while an agent is stopped waits in its inbox and is answered
# once it is started.
ab launcher@srv1 call "$RB" --wait 15s stop '#fast@srv1' >/dev/null
ab greeter@srv1 send '#fast@srv1' --topic held --tag 1 queued >/dev/null
has "the stopped agent holds the queued call" "$(ab greeter@srv1 ls '#fast@srv1')" '"queued":1'
ab launcher@srv1 call "$RB" --wait 15s start '#fast@srv1' >/dev/null
has "a message sent while stopped is answered after start" "$(ab greeter@srv1 consume --topic held --tag 1 --wait 15s)" '"body":"fast"'
is_empty "a fast queued call has no extra ack or answer" "$(ab greeter@srv1 consume --topic held --tag 1 --wait 0s)"
# Status is for whoever the runner's ACL admits; the controls and the host's
# files only for its Owner and Maintainers, as the daemon says at the send.
ab launcher@srv1 manage "$RB" --add-allow greeter@srv1 >/dev/null
has "a caller the runner's ACL admits reads its status" "$(ab greeter@srv1 call "$RB" --wait 15s status)" '\\"runner\\":\\"#runner/launcher@srv1\\"'
has "but not the host's files" "$(ab greeter@srv1 call "$RB" --wait 15s read runner/mail/work/env)" "error: read is for the runner's Owner and Maintainers"
lacks "nor their secrets" "$(ab greeter@srv1 call "$RB" --wait 15s read runner/mail/work/env)" 'bob'
has "and writes none" "$(ab greeter@srv1 call "$RB" --wait 15s "$(printf 'write runner/hello/env\nX=1')"; ls "$RH/runner/hello/env" 2>&1)" 'No such file'
has "nor stops an agent" "$(ab greeter@srv1 call "$RB" --wait 15s stop '#fast@srv1')" "error: stop is for the runner's Owner and Maintainers"
ab launcher@srv1 manage "$RB" --add-maintainers greeter@srv1 >/dev/null
has "a Maintainer reads env names, not values" "$(ab greeter@srv1 call "$RB" --wait 15s read runner/mail/work/env)" 'IMAP_USER'
ab launcher@srv1 manage "$RB" --remove-maintainers greeter@srv1 --remove-allow greeter@srv1 >/dev/null
# The host's files over the bus: the author's read only, the host's written
# whole after a check, and nothing else under the runner's home named at all.
has "read answers the author's env.dist" "$(ab launcher@srv1 call "$RB" --wait 15s read agent.d/mail/env.dist)" 'exists\\":true,\\"content\\":\\"# the account to fetch\\\\nIMAP_HOST=imap.example.com  # a default\\\\nIMAP_USER= # required\\\\n\\"'
has "and says when a host env file is absent" "$(ab launcher@srv1 call "$RB" --wait 15s read runner/hello/env)" 'exists\\":false,\\"size\\":0,\\"names\\":\[\]'
has "the author's files are read only" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write agent.d/mail/env.dist\nIMAP_HOST=evil')")" 'error: agent.d/mail/env.dist is the author.s, read only'
has "and unchanged" "$(cat "$RH/agent.d/mail/env.dist")" '^IMAP_HOST=imap.example.com  # a default$'
has "a kept token is not a file it names" "$(ab launcher@srv1 call "$RB" --wait 15s read runner/tokens/%23hello@srv1)" 'error: runner/tokens/%23hello@srv1 is not a file the runner reads or writes'
lacks "nor is it answered" "$(ab launcher@srv1 call "$RB" --wait 15s read runner/tokens/%23hello@srv1)" 'exists'
has "nor a path out of its home" "$(ab launcher@srv1 call "$RB" --wait 15s read runner/../env)" 'error: runner/../env is not a file the runner reads or writes'
has "create adds an instance, refused until configured" "$(ab launcher@srv1 call "$RB" --wait 15s create mail/home)" '\\"name\\":\\"#mail/home@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"needs IMAP_USER'
has "status names each agent's key, which names its files" "$(ab launcher@srv1 call "$RB" --wait 15s status)" '\\"name\\":\\"#mail/home@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"needs IMAP_USER\\",\\"key\\":\\"mail/home\\"}'
has "with an env only its user reads" "$(stat -c %a "$RH/runner/mail/home/env")" '^600$'
has "and refuses one already there" "$(ab launcher@srv1 call "$RB" --wait 15s create mail/home)" 'error: mail/home is already an instance here'
has "a bad env file is refused before it is written" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/mail/home/env\nnot a line')")" 'error: runner/mail/home/env: '
is_empty "and nothing was written" "$(cat "$RH/runner/mail/home/env")"
has "written, the instance is served" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/mail/home/env\nIMAP_USER=carol')")" '\\"name\\":\\"#mail/home@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\"'
has "with what was written" "$(ab launcher@srv1 call '#mail/home@srv1' --wait 15s go)" '"body":"mail for carol at imap.example.com"'
lacks "env read back contains no stored value" "$(ab launcher@srv1 call "$RB" --wait 15s read runner/mail/home/env)" 'carol|content'
cp "$RH/runner/agents.json" "$D/agents.before"
has "an agents.json that would refuse an agent is not written" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/agents.json\n{"hog":{"limits":{"cpu":"2x"}}}')")" 'error: hog: agents.json: limits.cpu'
has "and the host's file is as it was" "$(cmp "$D/agents.before" "$RH/runner/agents.json" && echo same)" '^same$'
has "a host overrides the author's run options" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/mail/agent.json\n{"processes":2}')")" '\\"name\\":\\"#mail/work@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\",\\"processes\\":{\\"running\\":0,\\"min\\":2,\\"max\\":2}'
has "and an instance's null falls back to the built default" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/mail/home/agent.json\n{"processes":null}')")" '\\"name\\":\\"#mail/home@srv1\\",\\"enabled\\":true,\\"state\\":\\"idle\\",\\"processes\\":{\\"running\\":0,\\"min\\":0,\\"max\\":1}'
has "what the code is may not be overridden" "$(ab launcher@srv1 call "$RB" --wait 15s "$(printf 'write runner/mail/agent.json\n{"command":"./evil.sh"}')")" 'error: runner/mail/agent.json: command may not be overridden'
echo '{"form":"std"}' >"$RH/runner/mail/agent.json"
has "and one placed by hand refuses the agent, by name" "$(ab launcher@srv1 call "$RB" --wait 15s reload)" '\\"name\\":\\"#mail/work@srv1\\",\\"enabled\\":true,\\"state\\":\\"refused\\",\\"reason\\":\\"runner/mail/agent.json: form may not be overridden'
rm -f "$RH/runner/mail/agent.json"
has "delete removes an instance" "$(ab launcher@srv1 call "$RB" --wait 15s delete mail/home)" '\\"name\\":\\"#mail/work@srv1\\"'
lacks "so it is no longer listed" "$(ab launcher@srv1 call "$RB" --wait 15s status)" '#mail/home@srv1'
has "nor are its files" "$(ls "$RH/runner/mail/" | tr '\n' ' ')" '^work $'
lacks "nor its entry" "$(cat "$RH/runner/agents.json")" 'mail/home'
# A stopping runner stops what it keeps: taken before the stop, since a
# process left behind is no longer the runner's to be asked about.
CG=/sys/fs/cgroup$(sed -n 's/^0:://p' /proc/$RUNPID/cgroup); CG=${CG%/runner}
for _ in $(seq 1 50); do [ -n "$(self_pids)" ] && break; sleep 0.1; done
ab launcher@srv1 call "$RB" --wait 30s stop '#selfie@srv1' >/dev/null
for _ in $(seq 1 100); do [ -z "$(self_pids)" ] && break; sleep 0.1; done
is_empty "stop ends a kept agent's processes" "$(self_pids)"
ab launcher@srv1 call "$RB" --wait 30s start '#selfie@srv1' >/dev/null
for _ in $(seq 1 50); do [ -n "$(self_pids)" ] && break; sleep 0.1; done
before=$(self_pids)
ab launcher@srv1 call "$RB" --wait 30s restart '#selfie@srv1' >/dev/null
for _ in $(seq 1 50); do [ -n "$(self_pids)" ] && break; sleep 0.1; done
has "restart replaces a kept agent's processes" "$(echo $before | wc -w) $(self_pids | grep -cxF -f <(echo "$before" | tr ' ' '\n'))" '^[1-9] 0$'
has "and it answers again" "$(ab greeter@srv1 call '#selfie@srv1' --wait 20s back)" '"body":"hello again: back"'
kept=$(self_pids)
has "a kept process is running before the runner stops" "$(echo $kept | wc -w)" '^[1-9]'
rm -f "$RH/runner/work/selfie/termed"
lacks "runner journal contains no host or bus secret sentinel" "$(cat "$D/runner.log")" 'rd105-.*sentinel'
kill $RUNPID 2>/dev/null; wait $RUNPID 2>/dev/null
has "it was asked to stop, not killed: its TERM handler ran" "$(ls "$RH/runner/work/selfie/" 2>&1)" '^termed$'
left=""; for p in $kept; do kill -0 $p 2>/dev/null && left="$left $p"; done
is_empty "and none of it is left once the runner has stopped" "$left"

# The bundled debug agent (src/agents/debug), served by the dedicated runner: hello
# greets, env lists what the script was handed — its envelope, and never the
# agent's token, which the runner keeps (docs/06-dedicated-runner.md#script-agents).
abx launcher@srv1 run '#debug@srv1' --algo=args --allow '*' "$PWD/agents/debug/debug.py" >>"$D/start.log" 2>&1 &
DBGPID=$!
for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q '#debug@srv1' && break; sleep 0.2; done
has "the debug agent says hello" "$(ab greeter@srv1 call '#debug@srv1' --wait 15s hello smoke)" 'hello, smoke'
dbgenv=$(ab greeter@srv1 call '#debug@srv1' --wait 15s env)
has "the debug agent lists its envelope" "$dbgenv" 'AGENT_BUS_FROM=greeter@srv1'
lacks "the debug agent was handed no token" "$dbgenv" 'AGENT_BUS_TOKEN='
kill $DBGPID 2>/dev/null; wait $DBGPID 2>/dev/null
has "a script and its arguments must be one quoted word" \
  "$(ab launcher@srv1 run '#x@srv1' --allow '*' --algo args ./greet.sh loudly 2>&1)" 'one script'

}
reply_to() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1 nobody@srv1 launcher@srv1
ab owner@srv1 register '#asker@srv1' --allow '*' --kind agent >/dev/null || exit 1
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
if normal; then
  sec "reply-to: the answer goes where the request said, and a dead route is refused now"
  ab owner@srv1 register '#worker@srv1' --allow '*' --kind agent --descr "does work" >/dev/null
  ab owner@srv1 register '#third@srv1' --allow '*' --kind agent >/dev/null
  ab caller@srv1 send '#worker@srv1' --topic rt --tag 1 --reply-to '#third@srv1' "work for someone else" >/dev/null
  wid=$(ab '#worker@srv1' consume --wait 5s | sed -n 's/.*"message_id":"\([^"]*\)".*/\1/p')
  ab '#worker@srv1' reply "$wid" "here is the answer" >/dev/null
  has "the third party gets the answer" \
    "$(ab '#third@srv1' consume --topic rt --tag 1 --wait 5s)" 'here is the answer'
  # The other half: the answer must NOT also go back to the caller. Checking
  # only that the third party received it passes with the reply broadcast.
  is_empty "and the caller does not" "$(ab caller@srv1 consume --topic rt --tag 1 --wait 1s)"

  # Refused when the request is accepted, not discovered when the answer
  # bounces — the whole point of the rule.
  out=$(ab caller@srv1 send '#worker@srv1' --topic rt --tag 2 --reply-to ghost@nowhere "nobody can hear the answer" 2>&1); rc=$?
  has "a dead reply address is refused at accept" "$out" 'no such reply address'
  bad_exit "and the send exits non-zero" $rc
  is_empty "and nothing was queued for the worker" "$(ab '#worker@srv1' consume --topic rt --tag 2 --wait 1s)"

  # Fire-and-forget asks for nothing back, so it stays open to a sender that
  # owns no queue. A check that only refuses is a check that refuses too much.
  ok_exit "a known user without an inbox of its own may send" \
    "$(ab nobody@srv1 send '#worker@srv1' --topic rt --tag 3 "no answer wanted" >/dev/null 2>&1; echo $?)"
  has "and it arrives" "$(ab '#worker@srv1' consume --topic rt --tag 3 --wait 5s)" 'no answer wanted'

  # A script agent answers the same way: the runner reads the route off the
  # envelope, so receipts and the answer all go to the third party.
  abx launcher@srv1 run '#relay@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "relays" >>"$D/start.log" 2>&1 &
  RPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'relays' && break; sleep 0.2; done
  ab caller@srv1 send '#relay@srv1' --topic rt4 --tag 1 --reply-to '#third@srv1' "via a script" >/dev/null
  has "a fast script answers the third party" \
    "$(ab '#third@srv1' consume --topic rt4 --tag 1 --wait 15s)" 'Hello via a script'
  is_empty "and sends no redundant receipt" "$(ab '#third@srv1' consume --topic rt4 --tag 1 --wait 1s)"
  is_empty "and the caller hears nothing" "$(ab caller@srv1 consume --topic rt4 --tag 1 --wait 1s)"
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null

else skipped=$((skipped+1)); fi
}
script_inbox() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1 launcher@srv1
ab owner@srv1 register '#asker@srv1' --allow '*' --kind agent >/dev/null || exit 1
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
if normal; then
  sec "a script agent is the inbox it registered, and stops when told"
  printf '#!/bin/sh\nsleep 3\necho "did $1"\n' > "$D/slow.sh"; chmod +x "$D/slow.sh"
  abx launcher@srv1 run '#slow@srv1' --allow '*' --algo args "$D/slow.sh" -1 --descr "slowly" >>"$D/start.log" 2>&1 &
  LPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'slowly' && break; sleep 0.2; done
  ab caller@srv1 send '#slow@srv1' --topic w --tag 1 one >/dev/null
  ab caller@srv1 send '#slow@srv1' --topic w --tag 2 two >/dev/null
  sleep 1
  kill -9 $LPID 2>/dev/null; wait $LPID 2>/dev/null
  # The agent read '#slow@srv1', not launcher@srv1, or the first message would
  # still be here; and with its one worker busy it left the second on the
  # daemon, where killing it cannot lose it.
  has "it read the inbox it registered, not the one that launched it" \
    "$(ab '#slow@srv1' consume --wait 3s)" 'two'
  is_empty "and took only what it had a worker for" "$(ab '#slow@srv1' consume --wait 1s)"

  # An agent's inbox belongs to its name, not to the process that reads it:
  # register it --kind agent, let nothing run, and the work is still there when something
  # with that name turns up. This is the property Legacy-V1's ephemeral channels did
  # not have — a dead channel took its results with it.
  ab launcher@srv1 register '#absent@srv1' --allow '*' --kind agent --descr "never started" >/dev/null
  ab caller@srv1 send '#absent@srv1' --topic w --tag 9 "waiting for whoever shows up" >/dev/null
  abx launcher@srv1 run '#absent@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "turned up late" >>"$D/start.log" 2>&1 &
  APID=$!
  has "the late agent answers the waiting message" \
    "$(ab caller@srv1 consume --topic w --tag 9 --wait 15s)" 'Hello waiting for whoever shows up'
  is_empty "and sends no redundant receipt" "$(ab caller@srv1 consume --topic w --tag 9 --wait 1s)"
  kill $APID 2>/dev/null; wait $APID 2>/dev/null

else skipped=$((skipped+1)); fi
}
script_done() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1 launcher@srv1
ab owner@srv1 register '#asker@srv1' --allow '*' --kind agent >/dev/null || exit 1
printf '#!/bin/sh\necho "Hello $1"\n' > "$D/hello-world.sh"; chmod +x "$D/hello-world.sh"
if normal; then
  sec "done: a script that finishes without an answer says so"
  # The gap `done` exists for. A script that succeeds and prints nothing used to
  # leave the caller with an ack and then silence until its deadline: the work
  # was finished and there was no way to hear it. A script that *answers* must
  # not also send one — a reply has plainly finished.
  printf '#!/bin/sh\ntrue\n' > "$D/silent.sh"; chmod +x "$D/silent.sh"
  ab launcher@srv1 register '#quiet@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#quiet@srv1' --allow '*' --algo args "$D/silent.sh" --descr "says nothing" >>"$D/start.log" 2>&1 &
  QPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'says nothing' && break; sleep 0.2; done
  ab caller@srv1 send '#quiet@srv1' --topic dn --tag 1 "do it quietly" >/dev/null
  has "the silent agent says it finished" \
    "$(ab caller@srv1 consume --topic dn --tag 1 --wait 15s)" '"receipt":"done"'
  is_empty "and sends only that done" "$(ab caller@srv1 consume --topic dn --tag 1 --wait 1s)"
  # The point of `done`, and the thing the first cut of this wave did not do:
  # a caller blocked on the answer stops when the work finishes with nothing to
  # return, instead of spending its whole deadline. Timing IS the check — the
  # margin is wide enough not to be a timing test.
  t0=$(date +%s)
  out=$(ab caller@srv1 call '#quiet@srv1' --topic dn5 --tag 1 --wait 20s "quietly again" 2>&1); rc=$?
  t1=$(date +%s)
  has "a call ends on done, saying which outcome it was" "$out" 'finished and sent no answer'
  bad_exit "and does not pretend it got an answer" $rc
  if [ $((t1-t0)) -lt 8 ]; then echo "  ok   and returns on the receipt, not at the deadline"; pass=$((pass+1));
  else echo "  FAIL and returns on the receipt, not at the deadline: waited $((t1-t0))s of 20s"; fail=$((fail+1)); fi
  kill $QPID 2>/dev/null; wait $QPID 2>/dev/null

  # A fast answer is the first message, with no ack or trailing done.
  ab launcher@srv1 register '#loud@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#loud@srv1' --allow '*' --algo args "$D/hello-world.sh" --descr "answers" >>"$D/start.log" 2>&1 &
  LPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'answers' && break; sleep 0.2; done
  ab caller@srv1 send '#loud@srv1' --topic dn3 --tag 1 "out loud" >/dev/null
  has "an agent that answers sends no done" \
    "$(ab caller@srv1 consume --topic dn3 --tag 1 --wait 15s)" 'Hello out loud'
  is_empty "and no receipt follows its answer" "$(ab caller@srv1 consume --topic dn3 --tag 1 --wait 1s)"
  kill $LPID 2>/dev/null; wait $LPID 2>/dev/null

  sec "roles: a script is told what its sender is, and nothing it inherited"
  # Plans/R1.0-Release/roles.md#roles-are-for-agents: the daemon works out
  # owner and maintainer; the runner hands them over as AB_ROLE_<NAME>=1 and
  # clears any AB_ROLE_* of its own, planted here as AB_ROLE_EVIL.
  printf '#!/bin/sh\necho "owner=${AB_ROLE_OWNER:-no} maint=${AB_ROLE_MAINTAINER:-no} evil=${AB_ROLE_EVIL:-none}"\n' > "$D/roles.sh"; chmod +x "$D/roles.sh"
  ab launcher@srv1 register '#roleful@srv1' --allow '*' --kind agent >/dev/null
  AB_ROLE_EVIL=1 abx launcher@srv1 run '#roleful@srv1' --allow '*' --algo args "$D/roles.sh" --roles deploy,read_only --descr "tells roles" >>"$D/start.log" 2>&1 &
  RPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'tells roles' && break; sleep 0.2; done
  has "its Owner is told owner" \
    "$(ab launcher@srv1 call '#roleful@srv1' --topic rl --wait 15s "who am i" 2>&1)" '"body":"owner=1 maint=no evil=none"'
  has "a caller on its allow list is told nothing" \
    "$(ab caller@srv1 call '#roleful@srv1' --topic rl --wait 15s "who am i" 2>&1)" '"body":"owner=no maint=no evil=none"'
  has "the roles it understands are on its record" \
    "$(ab caller@srv1 ls '#roleful@srv1')" '"roles":\["deploy","read_only"\]'
  out=$(ab caller@srv1 send '#roleful@srv1' --topic rl "claimed" 2>&1)
  has "the send verb states no roles of its own" "$out" 'message_id'
  has "and a sender stating roles is refused" \
    "$(post_code caller@srv1 /send '{"to":"#roleful@srv1","body":"x","roles":["owner"]}')" '^400$'
  printf '#!/bin/sh\necho "deploy=${AB_ROLE_DEPLOY:-no} read=${AB_ROLE_READ_ONLY:-no} guest=${AB_ROLE_GUEST:-no}"\n' > "$D/roles.sh"
  ab launcher@srv1 manage '#roleful@srv1' --remove-allow '*' --add-allow 'caller@srv1(deploy, read_only),*(guest)' >/dev/null
  has "assigned ACL roles survive CLI splitting and reach the script" \
    "$(ab caller@srv1 call '#roleful@srv1' --topic rl --wait 15s roles)" '"body":"deploy=1 read=1 guest=1"'
  has "generated roles cannot be assigned in an ACL" \
    "$(post_code launcher@srv1 /manage '{"name":"#roleful@srv1","allow":["caller@srv1(owner)"]}')" '^400$'
  kill $RPID 2>/dev/null; wait $RPID 2>/dev/null

  # The verb itself: one function serves both receipts, so `done` must reach the
  # sender exactly as `ack` does.
  ab owner@srv1 register '#handy@srv1' --allow '*' --kind agent >/dev/null
  ab caller@srv1 send '#handy@srv1' --topic dn4 --tag 1 "by hand" >/dev/null
  hid=$(ab '#handy@srv1' consume --wait 5s | sed -n 's/.*"message_id":"\([^"]*\)".*/\1/p')
  ab '#handy@srv1' done "$hid" >/dev/null
  has "the done verb sends a done receipt" \
    "$(ab caller@srv1 consume --topic dn4 --tag 1 --wait 5s)" '"receipt":"done"'
  # The exit code alone passed with the guard deleted: an unremembered id then
  # sends to an empty receiver and the DAEMON refuses it. Which end refused has
  # to be in the check, or it is not checking this end.
  has "done refuses an id this client never consumed" \
    "$(ab '#handy@srv1' done 0000 2>&1)" 'not one this client consumed'
  bad_exit "and exits non-zero" \
    "$(ab '#handy@srv1' done 0000 >/dev/null 2>&1; echo $?)"

  printf '#!/bin/sh\necho "ran $1"\n' > "$D/quick.sh"; chmod +x "$D/quick.sh"
  abx launcher@srv1 run '#stopper@srv1' --allow '*' --algo args "$D/quick.sh" --descr "stops" >>"$D/start.log" 2>&1 &
  TPID=$!
  for _ in $(seq 1 50); do ab '#asker@srv1' ls --all 2>/dev/null | grep -q 'stops' && break; sleep 0.2; done
  # registered is not the same as waiting: signal it once the daemon says the
  # consume is actually outstanding, or the check can pass on a race.
  wait_reader '#stopper@srv1'; ok_exit "the stopped agent has its own outstanding reader" $?
  kill -TERM $TPID 2>/dev/null
  for _ in $(seq 1 50); do kill -0 $TPID 2>/dev/null || break; sleep 0.1; done
  if kill -0 $TPID 2>/dev/null; then
    echo "  FAIL a stopped agent leaves its consume"; fail=$((fail+1)); kill -9 $TPID 2>/dev/null
    wait $TPID 2>/dev/null
  else
    wait $TPID; ok_exit "a stopped agent leaves its consume, exit 0" $?
  fi
  # A deliberate exit took the registration with it: a stopped agent no
  # longer collects, and a caller is told so at once rather than left
  # waiting on a name nothing serves.
  out=$(ab caller@srv1 send '#stopper@srv1' --topic w --tag 3 "after the stop" 2>&1); rc=$?
  bad_exit "and takes nothing after it stopped" $rc
  has "and says the name is gone" "$out" 'no such receiver'

  # What the deadline is FOR: an agent handed work nobody is waiting for any
  # more does not do it. Both messages are queued before the agent starts,
  # so the only difference between them is the deadline — a runner that
  # ignores it runs both, and a queue TTL cannot be what stops the stale one
  # because neither message has one.
  ab launcher@srv1 register '#judge@srv1' --allow '*' --kind agent >/dev/null
  post_body caller@srv1 /send '{"to":"#judge@srv1","topic":"lt","tag":"stale","wait":"1s","body":"stale"}' >/dev/null
  post_body caller@srv1 /send '{"to":"#judge@srv1","topic":"lt","tag":"fresh","wait":"60s","body":"fresh"}' >/dev/null
  sleep 1.2
  abx launcher@srv1 run '#judge@srv1' --allow '*' --algo args "$D/quick.sh" --descr "reads deadlines" >>"$D/start.log" 2>&1 &
  JPID=$!
  has "a request still inside its deadline is run" \
    "$(ab caller@srv1 consume --topic lt --tag fresh --wait 15s)" 'ran fresh'
  is_empty "and one whose caller gave up gets no ack and no answer" \
    "$(ab caller@srv1 consume --topic lt --tag stale --wait 2s)"
  has "and the agent says it dropped it rather than failing quietly" \
    "$(cat "$D/start.log")" 'after its caller gave up'
  kill $JPID 2>/dev/null; wait $JPID 2>/dev/null

else skipped=$((skipped+1)); fi
}
script_confined() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1 launcher@srv1
ab owner@srv1 register '#asker@srv1' --allow '*' --kind agent >/dev/null || exit 1
printf '#!/bin/sh\necho "ran $1"\n' > "$D/quick.sh"; chmod +x "$D/quick.sh"
if normal; then
  sec "a script agent is confined, and can be stopped and read"
  # One script, three questions, asked through the bus like any other call —
  # so what is checked is the confinement of the process the runner actually
  # started, not of one this file started for the occasion.
  # The script lives in a directory that is NOT a parent of the work
  # directory. Sharing one parent made the script's own read-only bind carry
  # the work directory in with it, and "the work directory is bound in" then
  # held whether it was bound or merely listed as writable.
  mkdir -p "$D/bin"
  printf '#!/bin/sh\ncase "$1" in\n  write-here) touch ./inside && echo inside-ok ;;\n  write-out)  touch /etc/nope 2>&1 | head -1 ;;\n  net)        curl -s -m 2 -o /dev/null "http://127.0.0.1:'"$PORT"'/status" && echo reached || echo no-net ;;\n  env)        echo "from=$AGENT_BUS_FROM work=$AGENT_BUS_WORK" ;;\n  roles)      echo "owner=${AB_ROLE_OWNER:-no} planted=${AB_ROLE_PLANT'"$$"':-none}" ;;\nesac\n' > "$D/bin/confined.sh"; chmod +x "$D/bin/confined.sh"
  note() { [ -s "$D/state/agent-bus/agents/$1.json" ]; }
  waitnote() { for _ in $(seq 1 100); do note "$1" && return 0; sleep 0.1; done; return 1; }

  # Confinement is opted into, so a run that says nothing gets none — even
  # on a host that could have provided one
  # (docs/06-dedicated-runner.md#sandboxing).
  ab launcher@srv1 register '#bare@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#bare@srv1' --allow '*' --algo args "$D/quick.sh" --descr "bare" >>"$D/bare.log" 2>&1 &
  BRPID=$!
  waitnote '#bare@srv1' || echo "  WARNING: '#bare@srv1' never left a note"
  has "a run that asks for no sandbox gets none" "$(cat "$D/bare.log")" 'sandbox off'
  kill $BRPID 2>/dev/null; wait $BRPID 2>/dev/null

  ab launcher@srv1 register '#confined@srv1' --allow '*' --kind agent >/dev/null
  # The HOST decides whether these run, not the agent under test. Asking the
  # log whether it was sandboxed would let "confine nothing" skip its own
  # checks and survive, which is the guarded-block shape of a hollow check.
  if systemd-run --user --pipe --collect --quiet --expand-environment=no /bin/true >/dev/null 2>&1; then
    # A role planted in the user manager, where a confined child starts from:
    # the runner cannot see it, so the child must clear it itself (review of
    # 0.8.97). Unique to this run, and taken out right after its check.
    systemctl --user set-environment "AB_ROLE_PLANT$$=1"
    abx launcher@srv1 run '#confined@srv1' --allow '*' --algo args "$D/bin/confined.sh" --sandbox on --descr "confined" >>"$D/sbx.log" 2>&1 &
    CFPID=$!
    waitnote '#confined@srv1' || echo "  WARNING: '#confined@srv1' never left a note"
    has "the agent says which sandbox it got" "$(cat "$D/sbx.log")" 'sandbox '
    has "and where it may write" "$(cat "$D/sbx.log")" 'work .*/work/#confined@srv1'
    has "a host that can sandbox gives the agent one" "$(cat "$D/sbx.log")" 'sandbox systemd-run'
    # Writing INSIDE has to pass beside the two refusals, or a child that
    # cannot run at all passes the whole set.
    has "a sandboxed script may write in its work directory" \
      "$(ab caller@srv1 call '#confined@srv1' --wait 20s write-here)" 'inside-ok'
    # And in THE work directory, not merely in one of that name inside its
    # own private /tmp: a write the host cannot see afterwards is a work
    # directory the agent does not really have.
    ok_exit "and the file is there on the host afterwards" \
      "$(test -e "$D/state/agent-bus/work/#confined@srv1/inside"; echo $?)"
    has "and may not write outside it" \
      "$(ab caller@srv1 call '#confined@srv1' --wait 20s write-out)" 'Read-only file system'
    has "and has no network unless it asked for one" \
      "$(ab caller@srv1 call '#confined@srv1' --wait 20s net)" 'no-net'
    # A confined child starts from the manager's environment and inherits
    # nothing of the runner's, so the envelope has to be stated or a script
    # that routes on it silently sees empty strings.
    has "and still finds the envelope in its environment" \
      "$(ab caller@srv1 call '#confined@srv1' --wait 20s env)" 'from=caller@srv1'
    has "and its own work directory" \
      "$(ab caller@srv1 call '#confined@srv1' --wait 20s env)" 'work=.*/work/#confined@srv1'
    has "a confined script holds the roles it was handed, and none the manager planted" \
      "$(ab launcher@srv1 call '#confined@srv1' --wait 20s roles)" '"body":"owner=1 planted=none"'
    systemctl --user unset-environment "AB_ROLE_PLANT$$"
    ab launcher@srv1 register '#netty@srv1' --allow '*' --kind agent >/dev/null
    abx launcher@srv1 run '#netty@srv1' --allow '*' --algo args "$D/bin/confined.sh" --network --descr "networked" >>"$D/net.log" 2>&1 &
    NTPID=$!
    waitnote '#netty@srv1' || echo "  WARNING: '#netty@srv1' never left a note"
    has "while one that did has one" \
      "$(ab caller@srv1 call '#netty@srv1' --wait 20s net)" 'reached'
    kill $NTPID 2>/dev/null; wait $NTPID 2>/dev/null
    kill $CFPID 2>/dev/null; wait $CFPID 2>/dev/null
  else
    echo "  WARNING: this host has no sandbox; the confinement checks are skipped"
    skipped=$((skipped+1))
  fi
  # Shared locks (docs/01-identity-and-authority.md#shared-locks): a record is the
  # namespace, its Owner and Maintainers may use its locks, every lock has a
  # ttl, only the holder releases unless --force, and an inactive record has
  # none. outsider is on its allow list: use of the record, not of its locks.
  sec "a record's locks: one holder, a ttl, --force, and the record's own life"
  # caller owns the record: her going inactive is later what deactivates it
  users outsider@srv1
  ab caller@srv1 channel create deploy@srv1 --allow outsider@srv1 >/dev/null
  post_code caller@srv1 /manage '{"name":"deploy@srv1","maintainers":["launcher@srv1"]}' >/dev/null
  has "the Owner takes a lock" \
    "$(abt caller@srv1 try-lock deploy@srv1 db --ttl 2m 2>&1)" '"holder":"caller@srv1"'
  out=$(abt launcher@srv1 try-lock deploy@srv1 db --ttl 2m 2>&1); rc=$?
  bad_exit "a second take is refused" $rc
  has "and names the holder" "$out" 'held by caller@srv1'
  has "holders answers a Maintainer, with the time left" "$(abt launcher@srv1 holders deploy@srv1)" '^deploy@srv1 db caller@srv1 [0-9][0-9hms ]* left$'
  out=$(abt caller@srv1 release deploy@srv1 db 2>&1); rc=$?
  ok_exit "the holder releases" $rc
  out=$(abt outsider@srv1 try-lock deploy@srv1 db --ttl 2m 2>&1); rc=$?
  bad_exit "a caller on the allow list, not a Maintainer, is refused" $rc
  has "as the record not being theirs" "$out" "belongs to someone else"
  abt caller@srv1 try-lock deploy@srv1 db --ttl 2m >/dev/null 2>&1
  out=$(abt launcher@srv1 release deploy@srv1 db 2>&1); rc=$?
  bad_exit "a non-holder cannot release" $rc
  out=$(abt launcher@srv1 release deploy@srv1 db --force 2>&1); rc=$?
  ok_exit "while --force can" $rc
  has "and the audit log names the exact operation" \
    "$(grep -h 'release --force' "$D"/logs/audit.log 2>/dev/null | tail -1)" 'launcher@srv1'
  # The wait ends on release, not on its own deadline.
  abt caller@srv1 try-lock deploy@srv1 wait1 --ttl 2m >/dev/null 2>&1
  ( sleep 0.4; abt caller@srv1 release deploy@srv1 wait1 >/dev/null 2>&1 ) &
  start=$(date +%s%N)
  out=$(abt launcher@srv1 lock deploy@srv1 wait1 --ttl 2m --wait 5s 2>&1); rc=$?
  took=$(( ($(date +%s%N) - start) / 1000000 ))
  ok_exit "a waiting take is granted on release" $rc
  has "and says who holds it" "$out" '"holder":"launcher@srv1"'
  has "the wait ended on the release, after ${took}ms, not its own deadline" \
    "$([ "$took" -lt 3800 ] && echo granted-early)" 'granted-early' 
  # The gate covers every verb: release, force and holders by someone who does not manage it.
  abt caller@srv1 try-lock deploy@srv1 gated --ttl 2m >/dev/null 2>&1
  out=$(abt outsider@srv1 release deploy@srv1 gated 2>&1); rc=$?
  bad_exit "their release is refused" $rc
  has "as the record not being theirs" "$out" "belongs to someone else"
  out=$(abt outsider@srv1 release deploy@srv1 gated --force 2>&1); rc=$?
  bad_exit "their force-release is refused" $rc
  out=$(abt outsider@srv1 holders deploy@srv1 2>&1); rc=$?
  bad_exit "their holders is refused" $rc
  # A ttl ends a hold a crashed holder left.
  abt caller@srv1 try-lock deploy@srv1 brief --ttl 1s >/dev/null 2>&1
  sleep 1.6
  out=$(abt launcher@srv1 try-lock deploy@srv1 brief --ttl 2m 2>&1); rc=$?
  ok_exit "an expired hold is free again" $rc
  # Extend: the holder sets a fresh ttl; a self-take is refused at once.
  abt caller@srv1 try-lock deploy@srv1 held --ttl 1s >/dev/null 2>&1
  out=$(abt caller@srv1 extend deploy@srv1 held --ttl 2m 2>&1); rc=$?
  ok_exit "the holder extends its ttl" $rc
  sleep 1.4
  has "so the old ttl no longer ends it" "$(abt caller@srv1 holders deploy@srv1)" 'held caller@srv1'
  out=$(abt caller@srv1 try-lock deploy@srv1 held --ttl 1m 2>&1); rc=$?
  bad_exit "re-taking your own lock is refused" $rc
  has "and says the way forward" "$out" 'use extend'
  out=$(abt caller@srv1 lock deploy@srv1 held --ttl 1m --wait 3s 2>&1); rc=$?
  bad_exit "even with a wait, at once" $rc
  out=$(abt caller@srv1 try-lock deploy@srv1 held --ttl 25h 2>&1); rc=$?
  bad_exit "a ttl over 24h is refused" $rc
  # An inactive record has no locks: the owner's records go with the owner.
  abt caller@srv1 try-lock deploy@srv1 gone --ttl 2m >/dev/null 2>&1
  # Only the daemon owner may deactivate a user, and the fixture's is parf@localhost.
  adminpost() { curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" -d "$2" "http://unix$1"; }
  out=$(adminpost /user/state '{"name":"caller@srv1","status":"inactive"}')
  has "the owner's state change is taken" "$out" 'caller@srv1' # the record's owner: the record goes with her
  out=$(abt launcher@srv1 try-lock deploy@srv1 gone --ttl 2m 2>&1); rc=$?
  bad_exit "an inactive record refuses a take" $rc
  has "as no such name, like every hidden thing" "$out" 'no such name'
  adminpost /user/state '{"name":"caller@srv1","status":"active"}' >/dev/null
  has "and what it held is gone when it comes back" \
    "$(abt launcher@srv1 try-lock deploy@srv1 gone --ttl 2m 2>&1)" '"holder":"launcher@srv1"'

  sec "a record's key-value store: its managers', three kinds, atomic edits"
  # The same fixture (Plans/R1.0-Release/done/kv.md#per-record-storage): caller owns
  # deploy@srv1, launcher maintains it, outsider is only on its allow list.
  abt caller@srv1 kv set deploy@srv1 stage built >/dev/null 2>&1
  has "a Maintainer reads what the Owner wrote" "$(abt launcher@srv1 kv get deploy@srv1 stage 2>&1)" '^built$'
  out=$(abt outsider@srv1 kv get deploy@srv1 stage 2>&1); rc=$?
  bad_exit "a caller on the allow list does not read the store" $rc
  has "as the record not being theirs" "$out" "belongs to someone else"
  lacks "and is not told the value" "$out" 'built'
  out=$(abt launcher@srv1 kv set deploy@srv1 stage again --add 2>&1); rc=$?
  bad_exit "an add over a present name is refused" $rc
  has "and says so" "$out" 'already stored'
  has "leaving the value as it was" "$(abt caller@srv1 kv get deploy@srv1 stage)" '^built$'
  out=$(abt launcher@srv1 kv set deploy@srv1 nothing x --replace 2>&1); rc=$?
  bad_exit "a replace of an absent name is refused" $rc
  has "counting from zero" "$(abt caller@srv1 kv inc deploy@srv1 runs)" '^1$'
  has "and adding n" "$(abt launcher@srv1 kv inc deploy@srv1 runs 4)" '^5$'
  has "an int reads back as --int" "$(abt caller@srv1 kv get deploy@srv1 runs --int)" '^5$'
  out=$(abt caller@srv1 kv get deploy@srv1 runs 2>&1); rc=$?
  bad_exit "while the same name as a string is another value, absent" $rc
  printf 'a\000\377b' | abt caller@srv1 kv set deploy@srv1 blob - >/dev/null 2>&1
  has "bytes that are not text round-trip" "$(abt caller@srv1 kv get deploy@srv1 blob | od -An -tx1 | tr -d ' \n')" '^6100ff620a$'
  abt caller@srv1 kv json deploy@srv1 work '[{"op":"push","key":"jobs","value":"a"},{"op":"push","key":"jobs","value":"b"}]' >/dev/null 2>&1
  has "a shift answers the first job pushed" "$(abt launcher@srv1 kv json deploy@srv1 work '[{"op":"shift","key":"jobs"}]' 2>&1)" '"value":"a"'
  out=$(abt caller@srv1 kv json deploy@srv1 work '[{"op":"push","key":"jobs","value":"c"},{"op":"inc","key":"jobs","value":1}]' 2>&1); rc=$?
  bad_exit "a list with one refused op is refused" $rc
  has "and changes nothing" "$(abt caller@srv1 kv get deploy@srv1 work --json)" '^{"jobs":\["b"\]}$'
  has "a delete says it removed the value" "$(abt caller@srv1 kv delete deploy@srv1 stage)" 'deploy@srv1 stage deleted'
  has "and a second says there was none" "$(abt caller@srv1 kv delete deploy@srv1 stage)" 'held no string value'

  sec "the runnable KV example: every operation and a repeat run"
  for example_run in 1 2; do
    AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok caller@srv1)" \
      AGENT_BUS_CLI="$D/agent-bus" timeout 15 clients/tests/kv/kv.sh '#kv-example@srv1' \
      >"$D/kv-example-$example_run.log" 2>&1
    rc=$?
    ok_exit "KV example run $example_run" "$rc"
    if [ "$rc" -ne 0 ]; then tail -n 8 "$D/kv-example-$example_run.log"; fi
  done
  has "the first run creates its Agent" "$(cat "$D/kv-example-1.log")" '^Created Agent #kv-example@srv1$'
  has "the second run reuses its Agent" "$(cat "$D/kv-example-2.log")" '^Using Agent #kv-example@srv1$'
  abt caller@srv1 try-lock '#kv-example@srv1' example/kv --ttl 2m >/dev/null
  AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$(tok caller@srv1)" \
    AGENT_BUS_CLI="$D/agent-bus" timeout 15 clients/tests/kv/kv.sh '#kv-example@srv1' \
    >"$D/kv-example-overlap.log" 2>&1
  rc=$?
  bad_exit "an overlapping KV example is refused" "$rc"
  has "the refusal is its held lock" "$(cat "$D/kv-example-overlap.log")" 'you already hold it; use extend'
  is_empty "the refused run never resets a name" "$(grep ' kv delete ' "$D/kv-example-overlap.log")"
  abt caller@srv1 release '#kv-example@srv1' example/kv >/dev/null
  ok_exit "the refused run left the original lock held" "$?"

  # Off is a setting, and asking for one the host cannot give is an error
  # rather than a quiet downgrade.
  ab launcher@srv1 register '#loose@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#loose@srv1' --allow '*' --algo args "$D/quick.sh" --sandbox off --descr "loose" >>"$D/loose.log" 2>&1 &
  LOPID=$!
  waitnote '#loose@srv1' || echo "  WARNING: '#loose@srv1' never left a note"
  has "an agent asked to run unconfined says so out loud" "$(cat "$D/loose.log")" 'sandbox off'
  out=$(abt launcher@srv1 run '#loose@srv1' --allow '*' --algo args "$D/quick.sh" 2>&1); rc=$?
  bad_exit "starting a name already running here is refused" $rc
  has "and says which process holds it" "$out" 'already running here as pid'
  out=$(abt launcher@srv1 run '#askew@srv1' --allow '*' --algo args "$D/quick.sh" --sandbox maybe 2>&1); rc=$?
  bad_exit "a sandbox setting that is neither on nor off is refused" $rc
  has "and says which two settings there are" "$out" 'sandbox is on or off'

  sec "stop ends one agent and leaves its siblings"
  ab launcher@srv1 register '#twin@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#twin@srv1' --allow '*' --algo args "$D/quick.sh" --descr "a twin" >>"$D/twin.log" 2>&1 &
  TWPID=$!
  waitnote '#twin@srv1' || echo "  WARNING: '#twin@srv1' never left a note"
  out=$(ab launcher@srv1 stop '#twin@srv1' 2>&1); rc=$?
  ok_exit "stop exits 0" $rc
  has "and says which agent it stopped" "$out" '#twin@srv1 stopped'
  is_empty "the process is gone when stop returns, not merely signalled" \
    "$(kill -0 $TWPID 2>/dev/null && echo still-there)"
  # The sibling is the control: one name is one process here, and a stop that
  # took the whole account with it would pass every check above.
  has "while the one beside it is still running" \
    "$(kill -0 $LOPID 2>/dev/null && echo yes)" 'yes'
  # A deliberate exit cleans up after itself: the runner unregisters its name
  # when nothing is waiting, so a name nobody serves is gone rather than
  # silently collecting messages.
  sleep 1
  has "a stopped agent says it unregistered itself" "$(cat "$D/twin.log")" '#twin@srv1 unregistered'
  out=$(ab '#asker@srv1' ls '#twin@srv1' 2>&1); rc=$?
  bad_exit "a stopped agent's name is gone" $rc
  has "and the daemon says no such name" "$out" 'no such'
  out=$(ab caller@srv1 send '#twin@srv1' "after the stop" 2>&1); rc=$?
  bad_exit "and takes no more messages" $rc
  has "and says so" "$out" 'no such receiver'
  # A crash is not a deliberate exit: no graceful shutdown ran, so the name
  # and its queue stay for the restart, which is the durability point.
  kill -9 $LOPID 2>/dev/null; wait $LOPID 2>/dev/null
  has "a crashed agent stays registered" "$(ab '#asker@srv1' ls '#loose@srv1')" 'loose'
  ab caller@srv1 send '#loose@srv1' "survived the crash" >/dev/null
  has "and messages still wait in its queue" "$(ab caller@srv1 consume --inbox '#loose@srv1' --wait 3s)" 'survived the crash'
  has "logs shows what it wrote, after it has stopped" \
    "$(ab launcher@srv1 logs '#twin@srv1')" '#twin@srv1 is'
  has "and --lines bounds it" \
    "$(ab launcher@srv1 logs '#twin@srv1' --lines 1 | wc -l | tr -d ' ')" '^1$'
  ab launcher@srv1 unregister '#loose@srv1' >/dev/null
  # An error exit is not a deliberate stop: a runner whose name is held by
  # somebody else's read fails at its first consume and must leave the
  # registration exactly as it found it.
  ab launcher@srv1 register '#clash@srv1' --allow '*' --kind agent >/dev/null
  ab '#asker@srv1' consume --inbox '#clash@srv1' --wait 20s >/dev/null 2>&1 &
  HOLDPID=$!
  wait_reader '#clash@srv1'; ok_exit "the held name has its own outstanding reader" $?
  out=$(abt launcher@srv1 run '#clash@srv1' --allow '*' --algo args "$D/quick.sh" 2>&1); rc=$?
  kill $HOLDPID 2>/dev/null; wait $HOLDPID 2>/dev/null
  bad_exit "a runner on a held name fails" $rc
  has "and says whose reader holds it" "$out" 'already has a reader'
  has "while the name survives the failed start" \
    "$(ab '#asker@srv1' ls '#clash@srv1')" 'clash'
  ab launcher@srv1 unregister '#clash@srv1' >/dev/null
  out=$(ab launcher@srv1 stop '#twin@srv1' 2>&1); rc=$?
  bad_exit "stopping something that is not running is an error" $rc
  has "and says so rather than pretending" "$out" 'not running from this account'
  out=$(ab launcher@srv1 logs never-ran@srv1 2>&1); rc=$?
  bad_exit "and so is asking for a log nothing wrote" $rc
  # An agent killed outright leaves its note behind. A note is not a running
  # agent — the process is the thing — so the stale one is reported as gone
  # and cleared, not offered as something to stop.
  ab launcher@srv1 register '#killed@srv1' --allow '*' --kind agent >/dev/null
  abx launcher@srv1 run '#killed@srv1' --allow '*' --algo args "$D/quick.sh" --descr "killed outright" >/dev/null 2>&1 &
  KLPID=$!
  waitnote '#killed@srv1' || echo "  WARNING: '#killed@srv1' never left a note"
  kill -9 $KLPID 2>/dev/null; wait $KLPID 2>/dev/null
  out=$(abt launcher@srv1 stop '#killed@srv1' 2>&1); rc=$?
  bad_exit "a note with no process behind it is not a running agent" $rc
  has "and says the note outlived the process" "$out" 'left a note but no process'
  ok_exit "and the stale note is cleared rather than left to mislead" \
    "$(test ! -e "$D/state/agent-bus/agents/killed@srv1.json"; echo $?)"
  kill $LOPID 2>/dev/null; wait $LOPID 2>/dev/null

else skipped=$((skipped+1)); fi
}
core_4() {
sec "--follow, and the one refusal the daemon owes us"
ab owner@srv1 register '#follower@srv1' --allow '*' --kind agent >/dev/null
# --follow keeps reading until it is stopped: that is the verb, so the check
# has to be the one to stop it.
abx '#follower@srv1' consume --follow --wait 5s >"$D/follow.out" 2>&1 &
FPID=$!
sleep 0.3
ab someone@srv1 send '#follower@srv1' "first" >/dev/null
ab someone@srv1 send '#follower@srv1' "second" >/dev/null
for _ in $(seq 1 50); do grep -q second "$D/follow.out" && break; sleep 0.2; done
kill $FPID 2>/dev/null; wait $FPID 2>/dev/null
has "--follow keeps reading" "$(cat "$D/follow.out")" 'first'
has "--follow reads the next one too" "$(cat "$D/follow.out")" 'second'
out=$("$D/agent-busd" --addr 127.0.0.1:$((PORT+1)) --socket "$D/no-owner.sock" --db "$D/no-owner.db" --create --flush-every 0 2>&1); rc=$?
bad_exit "the daemon refuses to derive ownership from the OS account" $rc
has "and requires an explicit owner before opening listeners" "$out" 'required --owner name'
# Agents on other hosts reach the daemon on the address it is given
# (docs/02-access.md#trust-boundary): a wildcard bind starts and answers, and
# the daemon says once that this is plain HTTP.
mkdir -p "$D/pub"
"$D/agent-busd" --addr 0.0.0.0:$((PORT+1)) --socket "$D/pub/bus.sock" --owner "$OWNER" --db "$D/pub/bus.db" --create --flush-every 0 >"$D/pub/log" 2>&1 &
PUBPID=$!
for _ in $(seq 1 50); do curl -fsS "http://127.0.0.1:$((PORT+1))/identity" >/dev/null 2>&1 && break; sleep 0.1; done
out=$(curl -fsS "http://127.0.0.1:$((PORT+1))/identity" 2>&1)
kill $PUBPID 2>/dev/null; wait $PUBPID 2>/dev/null
has "the daemon binds a non-loopback address and answers on it" "$out" '"version"'
has "and warns that it is plain HTTP there" "$(cat "$D/pub/log")" 'not loopback: tokens and message bodies cross that network as plain HTTP'
# AGENT_BUS_ADDR is the CLI's socket path; the daemon's TCP address is its own
# flag, so a shell that points the CLI at a socket cannot move the listener.
out=$(AGENT_BUS_ADDR="$D/cli-only.sock" "$D/agent-busd" -h 2>&1)
has "the daemon's TCP address does not follow the CLI's AGENT_BUS_ADDR" "$out" 'default "127.0.0.1:6767"'
lacks "and never becomes that socket path" "$out" 'cli-only.sock'

sec "a name is name@host, or template/instance-name@host"
# The template part is part of the identity, so it has to survive the whole
# trip: registration, the registry listing, a send and a consume. A "/" in a
# query parameter is where this breaks if anything hand-builds a URL.
ab owner@srv1 register '#code-review/claude@rdvp' --allow '*' --kind agent >/dev/null
ab owner@srv1 register '#code-review/claude-2@rdvp' --allow '*' --kind agent >/dev/null
has "a template-prefixed record registers" \
  "$(ab owner@srv1 ls --all)" '"name":"#code-review/claude@rdvp"'
ab sender@srv1 send '#code-review/claude-2@rdvp' "for the second one" >/dev/null
has "and is addressable by its whole name" \
  "$(ab '#code-review/claude-2@rdvp' consume --wait 2s)" 'for the second one'
# Two records from one template are two inboxes, not one queue shared by
# prefix: the first must still be empty.
is_empty "the sibling's inbox is its own, not the template's" \
  "$(ab '#code-review/claude@rdvp' consume --wait 200ms 2>/dev/null)"
has "a name may not hold two slashes" \
  "$(ab owner@srv1 register a/b/c@rdvp --kind agent --allow '*' 2>&1)" 'a-z 0-9 . _ - + @ only'
# Only the template part is wrong here: the instance name and the realm are
# both fine, so nothing but the template's own check can refuse it.
has "a bad template part is refused on its own" \
  "$(ab owner@srv1 register -nope/claude@rdvp --kind agent --allow '*' 2>&1)" 'bad template'
# The host is the LAST "@" part, so an instance may be named after the address
# it reads. This is the shape that breaks anything splitting on the first "@".
ab owner@srv1 register '#mail-sender/parf@comfi.com@host' --allow '*' --kind agent >/dev/null
has "an instance name may be an address" \
  "$(ab owner@srv1 ls --all)" '"name":"#mail-sender/parf@comfi.com@host"'
ab sender@srv1 send '#mail-sender/parf@comfi.com@host' "read this one" >/dev/null
has "and routes on the whole name, host split off last" \
  "$(ab '#mail-sender/parf@comfi.com@host' consume --wait 2s)" 'read this one'
has "a dangling at-sign is a typo, not a name" \
  "$(ab owner@srv1 register parf@@host --kind agent --allow '*' 2>&1)" 'bad name'
ab owner@srv1 register '#mail-sender/parf+alerts@comfi.com@host' --allow '*' --kind agent >/dev/null
has "plus-addressing is a legal instance name" \
  "$(ab owner@srv1 ls --all)" '"name":"#mail-sender/parf+alerts@comfi.com@host"'
has "but the host does not take a plus" \
  "$(ab owner@srv1 register '#parf@ho'+st --kind agent --allow '*' 2>&1)" 'bad realm'
has "and the realm is a name, not a path" \
  "$(ab owner@srv1 register code-review/claude@rd/vp --kind agent --allow '*' 2>&1)" 'bad realm'

sec "one name, one answer"
has "lookup answers about a single name" \
  "$(ab owner@srv1 register '#looked@srv1' --kind agent --allow '*' --descr "here" >/dev/null; curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $(tok owner@srv1)" "http://unix/lookup?name=%23looked@srv1")" '"descr":"here"'
has "and says so when there is none" \
  "$(code owner@srv1 "/lookup?name=absent-entirely@srv1")" '404'
# The caller's own record is checked with this, not by pulling the registry.
# The verb is agent-template from 0.6.6. The old spelling is not a second
# name for it: an unknown verb is refused, naming what the CLI does have.
# See docs/03-records-agent.md#agent-templates.
lacks "the old service-template spelling is gone" \
  "$(ab owner@srv1 service-template '#looked@srv1' 2>&1)" '"config'
has "and the refusal names the verb rather than failing quietly" \
  "$(ab owner@srv1 service-template '#looked@srv1' 2>&1)" 'unknown verb'
echo '{"s":1}' | ab owner@srv1 agent-template '#looked@srv1' - >/dev/null
is_empty "a lookup never carries a configuration" \
  "$(ab owner@srv1 ls '#looked@srv1' | grep -o '"config":[^,}]*')"
# Querying one record is how its Owner checks its setup without reading it
# back, so the digest is there for whoever may read the configuration — and
# for nobody else, to whom a digest of a short value is the value
# (issues/done/2026-10/2026-10-17.md#A16).
has "but a query for one record carries its digest" \
  "$(ab owner@srv1 ls '#looked@srv1')" '"config_sha":"'
has "and it is the same digest the whole listing gives" \
  "$(ab owner@srv1 ls '#looked@srv1' | grep -o '"config_sha":"[a-f0-9]*"')" \
  "$(ab owner@srv1 ls --all | grep -o '"name":"#looked@srv1"[^}]*' | grep -o '"config_sha":"[a-f0-9]*"')"
row=$(ab nobody@srv1 ls '#looked@srv1')
has "a caller the record only admits sees it" "$row" '"name":"#looked@srv1"'
lacks "but is given no digest of its configuration" "$row" 'config_sha'
has "asking about one name answers about one name" \
  "$(ab owner@srv1 ls '#looked@srv1' | grep -o '"name":' | wc -l)" '1'
has "and says so when there is no such record" \
  "$(ab owner@srv1 ls absent-entirely@srv1 2>&1)" 'no such name'

sec "human listing"
ab owner@srv1 register '#human@srv1' --allow '*' --kind agent --descr $'first\nsecond\tthird' >/dev/null
ab owner@srv1 send '#human@srv1' queued >/dev/null
human=$(ab owner@srv1 ls -h --kind agent)
has "human listing has table columns" "$human" '^NAME  *KIND  *OWNER  *READERS  *QUEUED  *LAST USED  *DESCRIPTION$'
has "human listing shows queue and flattens description" "$human" '^#human@srv1  *👾 Agent  *owner@srv1  *0  *1  *-  *first second third$'
# The kind filter excludes the other four kinds, not merely the templates:
# jobs@srv1 is a queue registered earlier and is a name this filter must drop.
is_empty "human kind filter excludes the kinds it did not ask for" "$(printf '%s\n' "$human" | grep '^jobs@srv1 ')"
has "and the queue is there when that is the kind asked for" \
  "$(ab owner@srv1 ls -h --kind queue)" '^jobs@srv1  *📮 Queue'
has "human single lookup works with flag after name" \
  "$(ab owner@srv1 ls '#human@srv1' -h)" '^#human@srv1  *👾 Agent  *owner@srv1  *0  *1  *-  *first second third$'
abx '#human@srv1' consume --topic NeverArrives --wait 5s >"$D/human-reader" 2>&1 & HPID=$!
for _ in $(seq 1 100); do
  human=$(ab owner@srv1 ls '#human@srv1' -h)
  printf '%s\n' "$human" | grep -q '^#human@srv1  *👾 Agent  *owner@srv1  *1  *1 ' && break
  sleep 0.01
done
has "human listing counts a filtered reader" "$human" '^#human@srv1  *👾 Agent  *owner@srv1  *1  *1  *just now  *first second third$'
kill "$HPID" 2>/dev/null; wait "$HPID" 2>/dev/null
raw_human=$(ab owner@srv1 ls --kind agent)
has "ordinary listing remains JSON" "$raw_human" '^\[.*"name":"#human@srv1"'
has "JSON retains the machine kind" "$raw_human" '"kind":"agent"'
lacks "JSON contains no display glyph" "$raw_human" '👤\|👾\|⚙️'
has "empty human listing is explicit" "$(ab owner@srv1 ls -h --kind no-such-kind)" '^No matching records\.$'
human_error=$(ab owner@srv1 ls -h absent-entirely@srv1 2>&1)
bad_exit "human missing lookup fails" "$?"
has "human missing lookup reports the error" "$human_error" 'no such name'
ab owner@srv1 register human-external@srv1 --allow '*' --protocol http --addr http://localhost >/dev/null
# A service is external and has no queue here, so the listing reports no reader
# count and no backlog for it rather than a zero it never measured.
# See docs/03-records.md#record-kinds.
has "an external service reports no queue it does not have" \
  "$(ab owner@srv1 ls -h human-external@srv1)" '^human-external@srv1  *📡 Service  *owner@srv1  *-  *-'
# A group is marked like every other kind, and has no queue to count either.
has "a group is marked and reports no queue it does not have" \
  "$(ab "$OWNER" ls -h @administrators)" "^@administrators  *👥 Group  *$OWNER  *-  *-"
has "while an agent's own counts are still measured" \
  "$(ab owner@srv1 ls -h '#human@srv1')" '^#human@srv1  *👾 Agent  *owner@srv1  *[0-9]  *[0-9]'
# The legend has to spell the cell the table prints. It explained an em dash
# while every such cell held "-", which explains nothing.
has "and the legend spells the character those cells hold" \
  "$(ab owner@srv1 ls -h human-external@srv1)" '^ *-  for 📡, which is external and has no queue here'
lacks "and nothing may be sent to the external one" \
  "$(ab owner@srv1 send human-external@srv1 nope 2>&1)" '"message_id"'
has "which says why, naming the address to call instead" \
  "$(ab owner@srv1 send human-external@srv1 nope 2>&1)" 'it is not sent to over this bus'
ab '#human@srv1' consume --wait 0s >/dev/null
ab '#human@srv1' consume --wait 10s >"$D/human-reader" & HUMAN_READER=$!
for _ in $(seq 1 100); do
  human=$(ab owner@srv1 ls -h '#human@srv1')
  printf '%s\n' "$human" | grep -q '^#human@srv1 *👾 Agent *owner@srv1 *1 ' && break
  sleep .02
done
has "human listing reflects a waiting reader" "$human" '^#human@srv1  *👾 Agent  *owner@srv1  *1  *0'
ab owner@srv1 send '#human@srv1' unblock >/dev/null
wait "$HUMAN_READER"

sec "unregister an idle address"
ab owner@srv1 register '#retired@srv1' --allow '*' --kind agent >/dev/null
ab '#retired@srv1' status >/dev/null # issue its credential before removing the address
out=$(ab stranger@srv1 unregister '#retired@srv1' 2>&1); rc=$?
bad_exit "unregister rejects another owner" "$rc"
has "unregister explains ownership refusal" "$out" 'belongs to someone else'
ab owner@srv1 send '#retired@srv1' keep >/dev/null
out=$(ab owner@srv1 unregister '#retired@srv1' 2>&1); rc=$?
bad_exit "unregister refuses queued messages" "$rc"
has "unregister explains the busy inbox" "$out" 'queued messages'
has "refused removal preserves the message" "$(ab '#retired@srv1' consume --wait 0s)" 'keep'
has "owner can unregister an idle address" "$(ab owner@srv1 unregister '#retired@srv1')" '^#retired@srv1 unregistered$'
is_empty "unregistered address leaves the listing" "$(ab owner@srv1 ls --all | grep -o '"name":"#retired@srv1"')"
has "unregistered address no longer accepts messages" "$(ab owner@srv1 send '#retired@srv1' late 2>&1)" 'no such name'
# The credential goes with the address. `tok` caches what it minted, so the
# cached one is exactly what a holder would still be presenting.
out=$(ab '#retired@srv1' status 2>&1); rc=$?
bad_exit "unregister takes the credential with the address" "$rc"
out=$(ab stranger@srv1 unregister '#retired@srv1' 2>&1); rc=$?
bad_exit "unregister of an absent address fails" "$rc"
# A removed name is reserved for nobody: MVP does not protect it, and
# protecting it is R1.3's (Plans/R1.3/README.md#removed-names).
has "a removed name is free for whoever asks next" "$(ab stranger@srv1 register '#retired@srv1' --kind agent --allow '*')" '"name":"#retired@srv1"'
has "and belongs to whoever took it" "$(ab stranger@srv1 ls '#retired@srv1')" '"owner":"stranger@srv1"'
out=$(ab owner@srv1 register '#retired@srv1' --kind agent --allow '*' 2>&1); rc=$?
bad_exit "so the previous owner cannot take it back" "$rc"
has "and the taker can remove it in turn" "$(ab stranger@srv1 unregister '#retired@srv1')" '^#retired@srv1 unregistered$'
ab owner@srv1 register '#selfgone@srv1' --allow '*' --kind agent >/dev/null
has "a principal can unregister itself" "$(ab '#selfgone@srv1' unregister '#selfgone@srv1')" 'unregistered'

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
  "$(post_body owner@srv1 /register '{"name":"#probe@srv1","kind":"agent","reading":true,"readers":99,"queued":77}' | grep -o '"reading":true\|"readers":99\|"queued":77')"
is_empty "and the claim does not survive into a listing" \
  "$(ab nobody@srv1 ls '#probe@srv1' | grep -o '"reading":true\|"readers":99')"
is_empty "a registration cannot claim a call count" \
  "$(post_body owner@srv1 /register '{"kind":"agent","name":"#probe3@srv1","in":99,"out":99}' | grep -o '"in":99\|"out":99')"
is_empty "a registration cannot claim loss it did not suffer" \
  "$(post_body owner@srv1 /register '{"kind":"agent","name":"#probe4@srv1","dropped":42,"expired":42}' | grep -o '"dropped":42\|"expired":42')"
is_empty "nor an age for a queue it has not got" \
  "$(post_body owner@srv1 /register '{"kind":"agent","name":"#probe5@srv1","oldest":"99h"}' | grep -o 99h)"
is_empty "a registration cannot claim a configuration digest" \
  "$(post_body owner@srv1 /register '{"kind":"agent","name":"#probe2@srv1","config_sha":"forged"}' | grep -o forged)"
has "a kind the daemon does not know is refused by the daemon, not only the CLI" \
  "$(post_code owner@srv1 /register '{"name":"modey@srv1","kind":"topic"}')" '400'

sec "per-record call counters"
# A drained queue and one nobody ever wrote to both read as empty; these tell
# them apart. The control agent proves the counters are the agent's own
# and not a daemon-wide total copied onto every record.
ab owner@srv1 register '#counted@srv1' --kind agent --allow '*' >/dev/null
ab owner@srv1 register '#control@srv1' --kind agent --allow '*' >/dev/null
cin=$(svc '#counted@srv1' in); cout=$(svc '#counted@srv1' out)
kin=$(svc '#control@srv1' in); kout=$(svc '#control@srv1' out)
ab caller@srv1 send '#counted@srv1' --topic cnt --tag 1 "one" >/dev/null
ab caller@srv1 send '#counted@srv1' --topic cnt --tag 2 "two" >/dev/null
delta "a listing counts what arrived for a record" 2 "$cin" "$(svc '#counted@srv1' in)"
delta "and nothing handed over while the queue still holds them" 0 "$cout" "$(svc '#counted@srv1' out)"
ab '#counted@srv1' consume --wait 2s >/dev/null
delta "consuming counts one out" 1 "$cout" "$(svc '#counted@srv1' out)"
delta "and does not count it in a second time" 2 "$cin" "$(svc '#counted@srv1' in)"
delta "a record nobody wrote to counts nothing in" 0 "$kin" "$(svc '#control@srv1' in)"
delta "nor anything out" 0 "$kout" "$(svc '#control@srv1' out)"
# The inbox has to be *empty* for the next leg, or the reader takes the
# leftover off the queue and never blocks — which is the queued path again,
# and it passed the straight-through mutants until this drain was added.
ab '#counted@srv1' consume --wait 2s >/dev/null
has "and the inbox is empty before a reader blocks on it" "$(svc '#counted@srv1' queued)" '^0$'
ab '#counted@srv1' consume --wait 5s >/dev/null 2>&1 & LPID=$!
sleep 0.3
ab caller@srv1 send '#counted@srv1' --topic cnt --tag 3 "straight through" >/dev/null
wait $LPID 2>/dev/null
delta "a message handed to a waiting reader counts in" 3 "$cin" "$(svc '#counted@srv1' in)"
delta "and out, without ever sitting in the queue" 3 "$cout" "$(svc '#counted@srv1' out)"

sec "a known user consumes from its own user record"
users ghost@srv1
has "a User's inbox is its user record" \
  "$(code ghost@srv1 "/consume?wait=0s")" '204'
has "while an unknown name has none" \
  "$(code never-made@srv1 "/consume?wait=0s")" '40[134]'
has "while a registered name with an empty inbox is 204" \
  "$(ab launcher@srv1 register '#quiet@srv1' --kind agent --allow '*' >/dev/null; code '#quiet@srv1' "/consume?wait=0s")" '204'

}
listing() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 nobody@srv1
if normal; then
  sec "a listing says whether a call would reach anyone"
  # Being in the registry and being callable are different facts: "there is a
  # MySQL on db1:3306" registers fine and nothing on this bus answers for it.
  ab owner@srv1 register db.main@srv1 --allow '*' --protocol mysql --addr db1:3306 --descr "the main database" >/dev/null
  has "a registration can say how to call it" "$(ab nobody@srv1 ls db.main@srv1)" '"protocol":"mysql"'
  # Asserted on the record itself, not on an empty grep: an is_empty that a
  # failed query also satisfies is a check that passes with the daemon down.
  ab owner@srv1 register '#plain.svc@srv1' --kind agent --allow '*' >/dev/null
  has "and an ordinary bus agent says nothing, because there is nothing to say" \
    "$(ab nobody@srv1 ls '#plain.svc@srv1' | grep -o '"name":"#plain.svc@srv1"\|"protocol":')" '"name":"#plain.svc@srv1"'
  is_empty "so no protocol comes back for it" \
    "$(ab nobody@srv1 ls '#plain.svc@srv1' | grep -o '"protocol":[^,}]*')"
  has "a registered name nobody serves is not shown as read" \
    "$(ab nobody@srv1 ls '#plain.svc@srv1' | grep -o '"reading":[a-z]*\|"readers":[0-9]*\|"name":"#plain.svc@srv1"')" '"name":"#plain.svc@srv1"'
  is_empty "and reading is absent rather than false" \
    "$(ab nobody@srv1 ls '#plain.svc@srv1' | grep -o '"reading":true\|"readers":[1-9][0-9]*')"
  # And with a reader attached, the same query says so.
  ab owner@srv1 register '#reader-box@srv1' --kind agent --allow '*' >/dev/null
  ab '#reader-box@srv1' consume --wait 6s >/dev/null 2>&1 &
  reader_pid=$!
  sleep 1
  has "a name something is reading says so" "$(ab nobody@srv1 ls '#reader-box@srv1')" '"reading":true.*"readers":1'
  ab nobody@srv1 send '#reader-box@srv1' "wake up" >/dev/null
  wait $reader_pid
  has "and the depth of what is waiting is visible" \
    "$(ab nobody@srv1 send '#plain.svc@srv1' "one" >/dev/null; ab nobody@srv1 ls '#plain.svc@srv1')" '"queued":1'

else skipped=$((skipped+1)); fi
}
core_5() {
sec "configuring an agent template produces a configured record"
users nosy@srv1 thief@srv1 smuggler@srv1
# The configuration is arbitrary JSON and stays opaque; the one thing that
# matters to the bus is that it never shows up where it should not.
echo '{"model":"opus","depth":3}' | ab owner@srv1 agent-template '#code-review/cfg@rdvp' - >/dev/null
has "the configuration comes back as it went in, to the agent" \
  "$(ab '#code-review/cfg@rdvp' agent-template '#code-review/cfg@rdvp')" '{"model":"opus","depth":3}'
has "and to the owner who set it" \
  "$(ab owner@srv1 agent-template '#code-review/cfg@rdvp' 2>&1)" '"depth":3'
has "and that is an answer, not a refusal" \
  "$(code owner@srv1 "/config?name=%23code-review/cfg@rdvp")" '200'

is_empty "a listing never carries it" \
  "$(ab owner@srv1 ls --all | grep -o '"config":[^,}]*')"
is_empty "and neither does the answer to setting one" \
  "$(echo '{"secret":"x"}' | ab owner@srv1 agent-template '#echoes@srv1' - | grep -o '"config":[^,}]*')"
# What a query gets instead: enough to see that a write landed and that two
# are the same, without handing anyone the configuration.
sha=$(echo '{"secret":"x"}' | ab owner@srv1 agent-template '#digested@srv1' - | grep -o '"config_sha":"[a-f0-9]*"')
has "setting one answers with its digest" "$sha" '"config_sha":"'
has "sharing the configured record succeeds" "$(post_code owner@srv1 /manage '{"name":"#digested@srv1","allow":["nobody@srv1"]}')" '200'
has "and a query carries the same digest" "$(ab owner@srv1 ls --all)" "$sha"
row=$(ab nobody@srv1 ls --all | grep -o '"name":"#digested@srv1"[^}]*')
has "the caller it was shared with lists it" "$row" '"name":"#digested@srv1"'
lacks "without its digest" "$row" 'config_sha'
has "the digest is sha256 of the stored bytes" "$sha" "$(printf '%s' '{"secret":"x"}' | sha256sum | cut -d' ' -f1)"
has "reformatting is not a change" \
  "$(printf '{ "secret" : "x" }' | ab owner@srv1 agent-template '#reformatted@srv1' - | grep -o '"config_sha":"[a-f0-9]*"')" "$sha"
has "a different configuration is a different digest" \
  "$(if [ "$(echo '{"secret":"y"}' | ab owner@srv1 agent-template '#other@srv1' - | grep -o '"config_sha":"[a-f0-9]*"')" != "$sha" ]; then echo differs; fi)" 'differs'
is_empty "an unconfigured record has no digest at all" \
  "$(ab owner@srv1 register '#plain@srv1' --kind agent --allow '*' | grep -o '"config_sha":[^,}]*')"
# A send is refused unless the receiver has a record, so this proves the
# record was created — the inbox itself is made lazily by the send either way.
has "configuring creates the record, so it can be sent to" \
  "$(ab owner@srv1 send '#code-review/cfg@rdvp' "it exists" >/dev/null; ab '#code-review/cfg@rdvp' consume --wait 2s)" 'it exists'

# A stranger the record's ACL does not admit learns only that there is no such
# name, as for a secret (docs/03-records-agent.md#configuring-a-template).
has "a stranger may not read it either" \
  "$(ab nosy@srv1 agent-template '#code-review/cfg@rdvp' 2>&1)" 'no such name'
# The text alone would still read right if every refusal collapsed to a 500.
has "and is refused as unknown, not as our own fault" \
  "$(code nosy@srv1 "/config?name=%23code-review/cfg@rdvp")" '404'

has "and may not overwrite it" \
  "$(ab nosy@srv1 agent-template '#code-review/cfg@rdvp' '{"model":"theirs"}' 2>&1)" 'belongs to someone else'
has "the owner's configuration survived that" \
  "$(ab '#code-review/cfg@rdvp' agent-template '#code-review/cfg@rdvp')" '"model":"opus"'

has "a configuration that is not JSON is refused" \
  "$(ab owner@srv1 agent-template '#code-review/cfg@rdvp' 'not json' 2>&1)" 'a configuration is JSON'
# The CLI refuses that one before it leaves; the daemon has to refuse it too,
# and the shape that reaches it is a body carrying no configuration at all.
has "and the daemon refuses an empty one on its own" \
  "$(post_code owner@srv1 /configure '{"name":"#code-review/cfg@rdvp"}')" '400'
has "null is not a configuration either" \
  "$(ab owner@srv1 agent-template '#code-review/cfg@rdvp' null 2>&1)" 'null is the absence of one'

# An agent registers itself on every start. That must refresh its
# description without destroying what it was configured with, and without
# handing the record to whoever registered last.
ab '#code-review/cfg@rdvp' register '#code-review/cfg@rdvp' --allow '*' --kind agent --descr "refreshed" >/dev/null
has "a re-registration keeps the configuration" \
  "$(ab '#code-review/cfg@rdvp' agent-template '#code-review/cfg@rdvp')" '"model":"opus"'

has "and still refreshes the description" \
  "$(ab owner@srv1 ls --all)" '"descr":"refreshed"'
ab thief@srv1 register '#code-review/cfg@rdvp' --allow '*' --kind agent >/dev/null
# Ownership is observable through a WRITE: nobody can read a configuration
# but the agent, so a read cannot tell us who owns the record.
has "and does not hand the record to whoever registered last" \
  "$(ab thief@srv1 agent-template '#code-review/cfg@rdvp' '{"mine":"now"}' 2>&1)" 'belongs to someone else'
has "a registration may not smuggle a configuration in" \
  "$(post_code thief@srv1 /register '{"kind":"agent","name":"#code-review/cfg@rdvp","config":{"evil":true}}' >/dev/null; ab '#code-review/cfg@rdvp' agent-template '#code-review/cfg@rdvp')" '"model":"opus"'

# On a name that does not exist yet there is no old configuration to keep, so
# this is the only shape that proves register drops --kind agent the field rather than
# being saved by the preservation rule.
has "not even onto a name that is new: the registration is refused (K.34)" \
  "$(post_code smuggler@srv1 /register '{"kind":"agent","name":"#fresh@srv1","config":{"evil":true}}')" '^400$'
out=$(ab smuggler@srv1 ls --all 2>&1)
lacks "and stores nothing" "$out" '"name":"#fresh@srv1"'

# An agent reads its own configuration and does not write it: private values
# are its Owner's and Maintainers' to set (docs/constitution.md#-private-values).
ab '#keeper@srv1' agent-template '#theirs@srv1' '{"by":"keeper"}' >/dev/null
has "an agent may not configure itself" \
  "$(ab '#theirs@srv1' agent-template '#theirs@srv1' '{"by":"itself"}' 2>&1)" 'belongs to someone else'
has "and its configuration is untouched by the attempt" \
  "$(ab '#theirs@srv1' agent-template '#theirs@srv1')" '"by":"keeper"'

# Reading must work where it is actually used: a script, with no terminal.
has "a read works with no terminal on stdin" \
  "$(ab '#code-review/cfg@rdvp' agent-template '#code-review/cfg@rdvp' </dev/null)" '"depth":3'

sec "a service's secret"
# The credential for reaching something outside, read back by the record's
# Owner and Maintainers; the allow list grants use of the service, not its
# credential. See docs/constitution.md#-private-values.
ab owner@srv1 register vault@srv1 --addr db.example:5432 --protocol postgresql --allow reader@srv1 >/dev/null
stored=$(ab owner@srv1 secret vault@srv1 'PGPASSWORD=hunter2')
has "setting one answers with its digest" "$stored" '"secret_sha":"[0-9a-f]\{64\}"'
lacks "and never with the bytes that were just sent" "$stored" 'hunter2'
has "the allow list grants use of it, not its secret" \
  "$(ab reader@srv1 secret vault@srv1 2>&1)" 'only its Owner and Maintainers'
has "and its owner, who is not on that list" \
  "$(ab owner@srv1 secret vault@srv1)" '^PGPASSWORD=hunter2$'
# Aimed at a service that exists and holds one: a refusal about a name that
# was never registered would read the same with no access check at all.
has "a caller it does not admit is told only that there is no such name" \
  "$(ab nosy@srv1 secret vault@srv1 2>&1)" 'no such name'
# It leaves by that read and by nothing else.
lacks "no listing carries the bytes" "$(ab reader@srv1 ls --all)" 'hunter2'
lacks "not even to the one caller who may read them" \
  "$(ab reader@srv1 ls vault@srv1)" 'hunter2'
is_empty "and no record answer carries a secret field at all" \
  "$(ab reader@srv1 ls vault@srv1 | grep -o '"secret":[^,}]*')"
# What every ordinary answer carries instead, so that "has this credential
# changed?" can be asked without anyone being handed it — by a monitor, a
# deploy check or a second host comparing its own.
has "while a query for one service carries its digest" \
  "$(ab owner@srv1 ls vault@srv1)" '"secret_sha":"[0-9a-f]\{64\}"'
row=$(ab reader@srv1 ls vault@srv1)
has "a caller it only admits sees the service" "$row" '"name":"vault@srv1"'
lacks "and is given no digest of its secret" "$row" 'secret_sha'
# Compared as strings and not as a pattern: two answers that both carry no
# digest at all match each other, which is how the same check about a
# configuration would pass with the field deleted.
one=$(ab owner@srv1 ls vault@srv1 | grep -o '"secret_sha":"[0-9a-f]\{64\}"')
all=$(ab owner@srv1 ls --all | grep -o '"name":"vault@srv1"[^}]*' | grep -o '"secret_sha":"[0-9a-f]\{64\}"')
has "and it is the same digest the whole listing gives" \
  "$(if [ -n "$all" ] && [ "$one" = "$all" ]; then echo same; fi)" 'same'
rotated=$(ab owner@srv1 secret vault@srv1 'PGPASSWORD=rotated' | grep -o '"secret_sha":"[0-9a-f]*"')
has "rotating it moves the digest" \
  "$(if [ "$rotated" != "$(echo "$stored" | grep -o '"secret_sha":"[0-9a-f]*"')" ]; then echo moved; fi)" 'moved'
has "and the new bytes are what comes back" \
  "$(ab owner@srv1 secret vault@srv1)" '^PGPASSWORD=rotated$'
# Reading and writing are different authorities, as they are for a
# configuration: the allow list says who may use the credential, not who may
# replace it.
has "someone the list admits may not write it" \
  "$(ab reader@srv1 secret vault@srv1 'PGPASSWORD=theirs' 2>&1)" 'belongs to someone else'
has "and what is stored is untouched by that attempt" \
  "$(ab owner@srv1 secret vault@srv1)" '^PGPASSWORD=rotated$'
# A registration carries neither half: the bytes would let anyone claim a
# credential, and the digest would let anyone claim to hold one.
has "a registration may not smuggle a secret in: it is refused (K.34)" \
  "$(post_code thief@srv1 /register '{"name":"smuggled@srv1","addr":"h:1","protocol":"https","secret":"K=stolen"}')" '^400$'
has "nor a digest claiming there is one" \
  "$(post_code thief@srv1 /register '{"name":"smuggled@srv1","addr":"h:1","protocol":"https","secret_sha":"forged"}')" '^400$'
has "and neither stored a record" "$(ab thief@srv1 secret smuggled@srv1 2>&1)" 'no such name'
# The other direction of the same rule: a service that re-registers to
# refresh its description must not lose the credential it was given.
ab owner@srv1 register vault@srv1 --addr db.example:5432 --protocol postgresql --allow reader@srv1 --descr "refreshed" >/dev/null
has "and a re-registration keeps the one already stored" \
  "$(ab owner@srv1 secret vault@srv1)" '^PGPASSWORD=rotated$'
# Never given one is a different answer from given an empty one, and the
# second is refused so that the two cannot be confused.
ab owner@srv1 register keyless@srv1 --addr h:1 --protocol https --allow '*' >/dev/null
has "a service that was never given one says so" \
  "$(ab owner@srv1 secret keyless@srv1 2>&1)" 'holds no secret'
is_empty "and carries no digest to suggest otherwise" \
  "$(ab owner@srv1 ls keyless@srv1 | grep -o '"secret_sha":[^,}]*')"
has "an empty secret is the absence of one, not a secret" \
  "$(post_body owner@srv1 /secret '{"name":"keyless@srv1","secret":""}')" 'holds no secret'
# Every other kind is reached by sending to its name, so there is nothing
# outside for a credential to unlock and the field would say something untrue.
ab owner@srv1 channel create plainq@srv1 --allow '*' >/dev/null
has "a queue cannot hold one" \
  "$(ab owner@srv1 secret plainq@srv1 'K=v' 2>&1)" 'on an agent, a service or a group and on no other kind'
is_empty "and nothing was stored on it by the attempt" \
  "$(ab owner@srv1 ls plainq@srv1 | grep -o '"secret[^,}]*')"
# The last place bytes escape from is the log of the process that holds them,
# and nothing here writes a request body down.
lacks "and the daemon's own log never holds a secret" \
  "$(cat "$D/daemon.log")" 'hunter2\|rotated'


}
backlog() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1
if normal; then
  sec "a backlog says how long its oldest message has been waiting"
  # A count alone cannot tell a busy queue from a stalled one. The age of
  # the head is what does (docs/05-discovery.md#what-it-shows).
  ab owner@srv1 register '#stalled@srv1' --allow '*' --kind agent >/dev/null
  is_empty "an inbox nobody wrote to has no oldest message" \
    "$(ab owner@srv1 ls '#stalled@srv1' | grep -o '"oldest"')"
  ab caller@srv1 send '#stalled@srv1' --topic old --tag 1 "sat here a while" >/dev/null
  sleep 3
  # A second, much younger message: with only one in the queue the newest IS
  # the oldest, and an age taken from the wrong end reads the same.
  ab caller@srv1 send '#stalled@srv1' --topic old --tag 2 "only just arrived" >/dev/null
  has "one with a backlog says how long the HEAD has waited" \
    "$(ab owner@srv1 ls '#stalled@srv1')" '"oldest":"[3-9]s"'
  # Drained, not merely read once: an age that outlives its queue would make
  # every emptied inbox look stuck for ever.
  ab '#stalled@srv1' consume --topic old --tag 1 --wait 3s >/dev/null
  ab '#stalled@srv1' consume --topic old --tag 2 --wait 3s >/dev/null
  drained=$(ab owner@srv1 ls '#stalled@srv1')
  # Answered, not merely silent: an is_empty check below would pass just as
  # well if `ls` had failed outright.
  has "the record is still answered once its queue drains" "$drained" '#stalled@srv1'
  is_empty "and stops saying how old its head is" "$(echo "$drained" | grep -o '"oldest"')"

else skipped=$((skipped+1)); fi
}
ttl() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1 caller@srv1
if normal; then
  sec "ttl: a message outlives its worth, and nothing else is counted as that"
  ab owner@srv1 register '#keeper@srv1' --allow '*' --kind agent --ttl 1h >/dev/null
  # The control that matters most: a TTL must not throw the message away
  # early. A check that only proves disappearance passes when everything
  # expires immediately.
  ab caller@srv1 send '#keeper@srv1' --topic tt --tag 1 --ttl 30s "still worth reading" >/dev/null
  has "a message inside its ttl is delivered" \
    "$(ab '#keeper@srv1' consume --topic tt --tag 1 --wait 2s)" 'still worth reading'

  e0=$(count expired)
  ab caller@srv1 send '#keeper@srv1' --topic tt --tag 2 --ttl 300ms "too late to matter" >/dev/null
  ok_exit "the expired-message fixture send is accepted" $?
  sleep 1
  is_empty "a message past its ttl is never handed over" "$(ab '#keeper@srv1' consume --topic tt --tag 2 --wait 1s)"
  delta "and it is counted as expired" 1 "$e0" "$(count expired)"

  # The queue's TTL bounds the sender's: asking for longer does not get it.
  ab owner@srv1 register '#brief@srv1' --allow '*' --kind agent --ttl 300ms >/dev/null
  ab caller@srv1 send '#brief@srv1' --topic tt --tag 3 --ttl 1h "asked for an hour" >/dev/null
  ok_exit "the queue-ttl fixture send is accepted" $?
  sleep 1
  is_empty "a queue's ttl bounds a longer one on the message" "$(ab '#brief@srv1' consume --topic tt --tag 3 --wait 1s)"
  # and the other direction still works: shorter on the message wins
  ab caller@srv1 send '#keeper@srv1' --topic tt --tag 4 --ttl 300ms "shorter than the queue" >/dev/null
  ok_exit "the message-ttl fixture send is accepted" $?
  sleep 1
  is_empty "and a shorter one on the message wins over the queue's" "$(ab '#keeper@srv1' consume --topic tt --tag 4 --wait 1s)"

  # Two counters, two problems. One that counted both could not tell an
  # operator whether the queue is too small or the message too old.
  d0=$(count dropped); e2=$(count expired)
  ab owner@srv1 register '#ring.small@srv1' --allow '*' --kind agent --overflow ring --bound 2 >/dev/null
  for i in 1 2 3 4; do ab caller@srv1 send '#ring.small@srv1' --topic tt --tag r "m$i" >/dev/null; done
  delta "a ring drop is counted as dropped" 2 "$d0" "$(count dropped)"
  delta "and not as expired" 0 "$e2" "$(count expired)"
  # And against the NAME that lost it. A node total says something is
  # losing work; it cannot say which inbox to go and look at.
  has "the inbox that dropped them says so itself" \
    "$(ab owner@srv1 ls '#ring.small@srv1')" '"dropped":2'
  is_empty "while one that lost nothing says nothing" \
    "$(ab owner@srv1 ls '#keeper@srv1' | grep -o '"dropped":')"
  has "and expiry is the expired inbox's own, not the ring's" \
    "$(ab owner@srv1 ls '#keeper@srv1')" '"expired":[1-9]'
  is_empty "which the ring did not suffer" \
    "$(ab owner@srv1 ls '#ring.small@srv1' | grep -o '"expired":')"

  # The bound is the record's, not one number for the whole daemon.
  ab owner@srv1 register '#tiny@srv1' --allow '*' --kind agent --bound 1 >/dev/null
  ab caller@srv1 send '#tiny@srv1' --topic tt --tag s "first" >/dev/null
  out=$(ab caller@srv1 send '#tiny@srv1' --topic tt --tag s "second" 2>&1); rc=$?
  has "a record's own bound refuses the one past it" "$out" 'queue is full'
  bad_exit "and says so to the sender" $rc

  has "a ttl that is not a duration is refused" \
    "$(ab owner@srv1 register '#bad.ttl@srv1' --kind agent --allow '*' --ttl soon 2>&1)" 'ttl is a duration'
  has "a bound that is not a count is refused" \
    "$(ab owner@srv1 register '#bad.bound@srv1' --kind agent --allow '*' --bound plenty 2>&1)" 'positive number'
else skipped=$((skipped+1)); fi

}
full_queue() {
# Inherited: what the serial run had made by here (see the note above go_checks).
users owner@srv1
if normal; then
  sec "a full queue: refuse by default, drop the oldest if asked"
  users flood@srv1
  ab owner@srv1 register '#sink@srv1' --allow '*' --kind agent >/dev/null
  ab owner@srv1 register '#ringy@srv1' --allow '*' --kind agent --overflow ring >/dev/null
  has "a record says what a full queue does, and refuses by default" \
    "$(ab owner@srv1 ls --all | grep -o '{[^}]*"name":"#sink@srv1"[^}]*}')" '"overflow":"strict"'
  has "an overflow mode that is neither is refused" \
    "$(ab owner@srv1 register '#bad@srv1' --kind agent --allow '*' --overflow maybe 2>&1)" 'overflow is strict or ring'
  # 1001 into a queue bounded at 1000, twice: strict must refuse the last one,
  # ring must swallow it and lose the first.
  d0=$(count dropped)
  for i in $(seq 0 1000); do ab flood@srv1 send '#sink@srv1' "msg-$i" >/dev/null 2>&1; done
  out=$(ab flood@srv1 send '#sink@srv1' "one too many" 2>&1); rc=$?
  bad_exit "strict refuses the send rather than lose a message" $rc
  has "and names the queue that is full" "$out" 'queue is full: '#sink@srv1''
  has "and says the sender is outrunning the reader, not that we broke" \
    "$(post_code flood@srv1 /send '{"to":"#sink@srv1","body":"one more"}')" '429'
  delta "strict dropped nothing" 0 "$d0" "$(count dropped)"
  d1=$(count dropped)
  for i in $(seq 0 1000); do ab flood@srv1 send '#ringy@srv1' "msg-$i" >/dev/null 2>&1; done
  has "ring keeps taking, and the oldest is what went" "$(ab '#ringy@srv1' consume --wait 2s)" 'msg-1"'
  delta "and the loss is counted, not silent" 1 "$d1" "$(count dropped)"

else skipped=$((skipped+1)); fi
}
core_6() {
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
      timeout 5 "$D/agent-bus" call '#slow@srv1' --wait 500ms "are you there?" 2>&1); rc=$?
kill $MPID 2>/dev/null; wait $MPID 2>/dev/null
if [ "$rc" -eq 124 ]; then
  echo "  FAIL a stalled consume ends at --wait: it ran past the deadline"; fail=$((fail+1))
else
  echo "  ok   a stalled consume ends at --wait"; pass=$((pass+1))
fi
has "and says the message was accepted" "$out" 'do not resend'

sec "the caller's deadline travels to the receiver"
users asker2@srv1
# The wait belongs to the CALLER, so an agent can see the answer is already
# too late and not do the work. The moment is the daemon's: a caller states a
# duration and never an instant, the same way it may not state its own name.
ab owner@srv1 register '#clockwatch@srv1' --allow '*' --kind agent >/dev/null
# A zero time is still a field, so "it has a deadline" is not the check — the
# year is. Matching the key alone passed with the stamping deleted.
has "a send states a wait and gets the moment it lands on" \
  "$(post_body caller@srv1 /send '{"to":"#clockwatch@srv1","topic":"dl","tag":"1","wait":"30s","body":"in time"}')" '"deadline":"20'
has "and the agent reads it off the envelope it consumed" \
  "$(ab '#clockwatch@srv1' consume --wait 5s)" '"deadline":"20'
has "a send with no wait carries no moment" \
  "$(post_body caller@srv1 /send '{"to":"#clockwatch@srv1","topic":"dl","tag":"9","body":"whenever"}')" '"deadline":"0001-'
has "and a caller cannot state the moment itself" \
  "$(post_body caller@srv1 /send '{"to":"#clockwatch@srv1","topic":"dl","tag":"2","deadline":"2099-01-01T00:00:00Z","body":"claimed"}')" '"deadline":"0001-'
ab '#clockwatch@srv1' consume --wait 5s >/dev/null
ab '#clockwatch@srv1' consume --wait 5s >/dev/null
has "a wait that is not a duration is refused" \
  "$(post_body caller@srv1 /send '{"to":"#clockwatch@srv1","wait":"soon","body":"no"}')" 'a wait is a duration'
has "and so is one that ran out before it was sent" \
  "$(post_body caller@srv1 /send '{"to":"#clockwatch@srv1","wait":"-5s","body":"no"}')" 'a wait is a duration'
# The CLI's own `call` is the caller that waits, so if it does not put its
# --wait on the wire the field travels for nobody.
ab '#clockwatch@srv1' register '#clockwatch@srv1' --kind agent --allow '*' >/dev/null 2>&1
abx asker2@srv1 call '#clockwatch@srv1' --topic dl --tag cli --wait 9s "how long have I got" >/dev/null 2>&1 &
CLIPID=$!
has "the CLI's own call carries its --wait" \
  "$(ab '#clockwatch@srv1' consume --wait 5s)" '"deadline":"20'
kill $CLIPID 2>/dev/null; wait $CLIPID 2>/dev/null
# A deadline is not a TTL. The queue bounds a TTL and does not bound this, and
# the bus never acts on it: a message whose caller has gone is still delivered,
# because only the agent knows whether the work is worth doing for anyone
# else. Losing that distinction is what makes this its own field.
post_body caller@srv1 /send '{"to":"#clockwatch@srv1","topic":"dl","tag":"3","wait":"1s","body":"long gone"}' >/dev/null
sleep 1.2
has "a message past its deadline is still delivered, the judgement being the agent's" \
  "$(ab '#clockwatch@srv1' consume --wait 5s)" 'long gone'
# And the TTL is still the receiver's, untouched by the caller's deadline: a
# generous wait does not keep a message the queue was told to drop.
ab owner@srv1 register '#brief@srv1' --allow '*' --kind agent --ttl 300ms >/dev/null
post_body caller@srv1 /send '{"to":"#brief@srv1","wait":"60s","body":"kept briefly"}' >/dev/null
sleep 0.6
is_empty "while a long wait does not extend what the queue keeps" \
  "$(ab '#brief@srv1' consume --wait 1s)"

}
enrolment() {
sec "enrolment: a key you hold, not a key you name"
# Its own daemon, because a vouched realm changes what registering means.
# See docs/01-identity-and-authority.md#registration.
mkdir -p "$D/enr"
ssh-keygen -q -t ed25519 -N '' -f "$D/enr/mine" >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$D/enr/theirs" >/dev/null
printf 'newbie %s\n' "$(cat "$D/enr/mine.pub")" > "$D/enr/keys"
printf 'squatter %s\n' "$(cat "$D/enr/mine.pub")" >> "$D/enr/keys"
"$D/agent-busd" --addr 127.0.0.1:$((PORT+10)) --socket "$D/enr/bus.sock" \
  --owner "$OWNER" --db "$D/enr/bus.db" --create --flush-every 0 --directory "vouched=$D/enr/keys" >"$D/enr/daemon.log" 2>&1 &
EPID=$!
ready "$D/enr/bus.sock" || echo "  WARNING: $D/enr/bus.sock never answered"
ETOK=$(owner_token "$D/enr")
curl -fsS --unix-socket "$D/enr/bus.sock" -H "X-Agent-Bus-Token: $ETOK" -d '{"name":"alice@srv1","create":true}' http://unix/user >/dev/null || exit 1
eab() { AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_TOKEN=$ETOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
etok() { AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_TOKEN=$ETOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token "$@"; }
# A fourth argument is a body, and a body is what makes it a POST: passing an
# empty one turned every GET here into a 405 that read as a refusal.
ecode() {
  if [ $# -lt 4 ]; then
    curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" -H "X-Agent-Bus-Token: $2" "http://unix$3"
  else
    curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" -H "X-Agent-Bus-Token: $2" -d "$4" "http://unix$3"
  fi
}

out=$(eab register '#newbie@vouched' --kind agent --allow '*' 2>&1); rc=$?
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
  "$(ecode alice@srv1 "$(etok alice@srv1 2>/dev/null)" /register '{"name":"newbie@vouched","kind":"user"}')" '403'
has "nor be handed its credential" \
  "$(ecode alice@srv1 "$(etok alice@srv1 2>/dev/null)" /token '{"name":"newbie@vouched"}')" '403'
# Enrolment is where a credential comes from, so it cannot want one first.
# See docs/02-access.md#proving-possession.
has "a newcomer with no credential at all is still challenged" \
  "$(curl -s --unix-socket "$D/enr/bus.sock" -d '{"name":"squatter@vouched"}' http://unix/enrol)" '"nonce"'
has "while everything else still wants one" \
  "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/enr/bus.sock" http://unix/status)" '401'

# agent-bus token is the credential verb an ordinary user runs, and the only one they
# reach over SSH. See docs/07-setup.md#the-programs.
TOK1=$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus" token alice@srv1)
has "the token program prints a credential and nothing else" "$TOK1" '^[0-9a-f]\{48\}$'
has "and asking twice is a read, not a rotation" \
  "$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus" token alice@srv1)" "^$TOK1\$"
is_empty "while --rotate hands out a new one" \
  "$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus" token alice@srv1 --rotate | grep -x "$TOK1")"
# The point of the key path: nothing to present, and no sshd in it anywhere.
KTOK=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+10)) "$D/agent-bus" token squatter@vouched --key "$D/enr/mine" 2>&1)
has "a key is credential enough where the realm publishes it" "$KTOK" '^[0-9a-f]\{48\}$'
has "and that credential is that name's" "$(ecode squatter@vouched "$KTOK" /status)" '200'
out=$(AGENT_BUS_ADDR=http://127.0.0.1:$((PORT+10)) "$D/agent-bus" token newbie@vouched --key "$D/enr/theirs" 2>&1); rc=$?
bad_exit "a key the realm does not publish for that name gets nothing" $rc
# Over SSH the line is the entitlement and the request is what was typed.
out=$(SSH_ORIGINAL_COMMAND="token $OWNER" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
  "$D/agent-bus" token alice@srv1 2>&1); rc=$?
bad_exit "a key may only ask for the name its line names" $rc
has "and is told which name that is" "$out" 'may ask for alice@srv1'
# Against a reading taken now: --rotate above moved it.
NOW=$(AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK "$D/agent-bus" token alice@srv1)
has "while asking for that one is answered" \
  "$(SSH_ORIGINAL_COMMAND="token alice@srv1" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
     "$D/agent-bus" token alice@srv1)" "^$NOW\$"
has "and a line that names nobody serves what was asked for" \
  "$(SSH_ORIGINAL_COMMAND="token alice@srv1" AGENT_BUS_ADDR=$D/enr/bus.sock AGENT_BUS_NAME=$OWNER AGENT_BUS_TOKEN=$ETOK \
     "$D/agent-bus" token)" "^$NOW\$"

# A provider is an alternative to typing the record, not a dependency.
rm -f "$D/enr/keys"
has "an enrolled principal keeps working with the directory gone" \
  "$(ecode newbie@vouched "$NEWTOK" /status)" '200'
out=$(eab enrol squatter@vouched --key "$D/enr/mine" 2>&1); rc=$?
bad_exit "while nobody new can enrol while it is gone" $rc
kill $EPID 2>/dev/null; wait $EPID 2>/dev/null

}
core_7() {
sec "who may reach what: direct grants and the owner's agent cohort"
users acl-owner@srv1
# See docs/02-access.md#acl.
ab acl-owner@srv1 register '#direct-svc@srv1' --kind agent --descr "direct access only" --allow acl-owner@srv1 >/dev/null
ab acl-owner@srv1 register '#owner-cohort@srv1' --kind agent --descr "owner cohort" --allow '@owner' >/dev/null
ab acl-owner@srv1 register '#sibling-svc@srv1' --kind agent --descr "same direct owner" >/dev/null
ab acl-owner@srv1 register '#any-svc@srv1' --kind agent --descr "open to all" --allow '*' >/dev/null
has "the daemon Owner has no implicit message-access layer" \
  "$(post_code $OWNER /send '{"to":"#direct-svc@srv1","body":"not an ACL grant"}')" '403'
has "and the refusal is its own, not a hidden lookup" \
  "$(post_body $OWNER /send '{"to":"#direct-svc@srv1","body":"not an ACL grant"}')" 'may not send to'
has "a principal absent from a direct ACL is refused" \
  "$(post_code alice@srv1 /send '{"to":"#direct-svc@srv1","body":"by nobody"}')" '403'
has "while the resource owner still reaches it" \
  "$(post_code acl-owner@srv1 /send '{"to":"#direct-svc@srv1","body":"mine"}')" '200'
has "@owner admits an agent with the same direct owner" \
  "$(post_code '#sibling-svc@srv1' /send '{"to":"#owner-cohort@srv1","body":"sibling"}')" '200'
has "@owner also admits the direct owner" \
  "$(post_code acl-owner@srv1 /send '{"to":"#owner-cohort@srv1","body":"owner"}')" '200'
has "@owner does not admit an unrelated user" \
  "$(post_code alice@srv1 /send '{"to":"#owner-cohort@srv1","body":"unrelated"}')" '403'
has "and allow * means anyone who can authenticate" \
  "$(post_code alice@srv1 /send '{"to":"#any-svc@srv1","body":"by anyone"}')" '200'
# A record is always its owner's and its own, list or no list: an agent
# that could not reach what it registered would not survive its own start.
ab acl-owner@srv1 register '#terse-svc@srv1' --kind agent --descr "lists somebody else" --allow alice@srv1 >/dev/null
has "an owner reaches its own agent without being on its list" \
  "$(post_code acl-owner@srv1 /send '{"to":"#terse-svc@srv1","body":"mine"}')" '200'
has "and the agent sees itself in its own listing" \
  "$(ab '#terse-svc@srv1' ls --all)" 'lists somebody else'
is_empty "while a stranger sees neither" \
  "$(ab bob@srv1 ls --all | grep -o 'lists somebody else')"
# Seeing and using are the same question, so an agent you may not use is
# not in your listing and does not answer a lookup either.
is_empty "an agent that will not have you is not in your listing" \
  "$(ab alice@srv1 ls --all | grep -o 'direct access only')"
has "though it is in its owner's" "$(ab acl-owner@srv1 ls --all)" 'direct access only'
has "and a lookup of it says no such name" \
  "$(code alice@srv1 "/lookup?name=%23direct-svc@srv1")" '404'
has "while its owner gets the record" \
  "$(code acl-owner@srv1 "/lookup?name=%23direct-svc@srv1")" '200'
# One check per write verb: a shared guard passes the whole set while any
# one path is still open.
ab acl-owner@srv1 channel create shut-topic@srv1 --descr "not yours" --allow acl-owner@srv1 >/dev/null
has "publishing to a topic that will not have you is refused" \
  "$(post_code alice@srv1 /send '{"to":"shut-topic@srv1","topic":"anything","body":"by nobody"}')" '403'
has "and so is registering over its name" \
  "$(post_code alice@srv1 /register '{"name":"#direct-svc@srv1","kind":"agent"}')" '403'

sec "the installer makes a service account, and it is not the installer's"
# The privileged step cannot run here, so what is checked is everything it
# would write: an install that puts the daemon under the installer's own
# account is the failure this wave exists to prevent.
# See docs/07-setup.md#the-two-accounts.
# Setup must inspect this fixture's TLS state, never the host's private directory.
export AGENT_BUS_TLS_DIR=$D/setup-tls
UNIT=$("$D/agent-bus-setup" --print-unit --owner "$OWNER" --exec /usr/local/bin/agent-busd)
has "the unit runs the daemon as an account of its own" "$UNIT" '^User=agent-busd$'
is_empty "never as root" "$(printf '%s' "$UNIT" | grep -x 'User=root')"
is_empty "and never as whoever ran setup" "$(printf '%s' "$UNIT" | grep -x "User=$(id -un)")"
has "the database lives under that account's home" "$UNIT" 'db /var/lib/agent-bus/daemon/agent-bus.db'
# The daemon is confined to its own home, so the runner's is out of reach even
# before either account's mode is consulted.
has "and it may write there and to its logs, nowhere else" "$UNIT" '^ReadWritePaths=/var/lib/agent-bus/daemon /var/log/agent-bus$'
has "which it is told the place of" "$UNIT" 'log-dir /var/log/agent-bus'
# TLS on the port is setup's to turn on (docs/07-setup.md#tls): the unit then
# names the directory, and a node without it serves plain HTTP only.
setup_tls() { AGENT_BUS_TLS_DIR=$D/setup-tls "$D/agent-bus-setup" --owner "$OWNER" --exec /usr/local/bin/agent-busd "$@" </dev/null 2>&1; }
has "--tls self-signed puts the TLS directory in the unit" "$(setup_tls --print-unit --tls self-signed)" " --tls-dir $D/setup-tls"
lacks "while a node without TLS gets none" "$(setup_tls --print-unit)" 'tls-dir'
has "and a dry run says it would generate the certificate" "$(setup_tls --dry-run --tls self-signed)" 'would generate a self-signed TLS certificate'
out=$(setup_tls --dry-run --tls files); rc=$?
bad_exit "--tls files without its files is refused" $rc
has "and says what is missing" "$out" 'needs --tls-cert and --tls-key'
# systemd owns that directory's mode once StateDirectory names it, and re-applies
# its own default on every start. Left unstated, the 0700 setup made becomes
# 0755 the moment the daemon first runs, which no test passing a home would see.
has "and the mode setup made is the mode systemd keeps" "$UNIT" '^StateDirectoryMode=0700$'
has "it comes back after it dies" "$UNIT" '^Restart='
has "it is given one capability, not root" "$UNIT" '^AmbientCapabilities=CAP_CHOWN$'
has "and cannot pick up a second" "$UNIT" '^CapabilityBoundingSet=CAP_CHOWN$'
has "the installer still gets a socket of their own" "$UNIT" "[-]user $(id -un)=$OWNER"
# No runner is installed by default, so no account is mapped for one: a
# person's runner reaches the bus with its own agent's token
# (docs/07-setup.md#a-persons-runner).
lacks "and no runner account is mapped" "$UNIT" "agent-bus-runner"
out=$("$D/agent-bus-setup" --owner "$OWNER" 2>&1); rc=$?
bad_exit "setup without root refuses rather than half-installing" $rc
is_empty "and does not try the first step before finding that out" \
  "$(printf '%s' "$out" | grep -i useradd)"
has "and says that step is the only one that needs it" "$out" 'Nothing after this step'
has "and says the line to run instead of just refusing" "$out" 'sudo .*agent-bus-setup'
out=$("$D/agent-bus-setup" --dry-run --owner "$OWNER" 2>&1); rc=$?
ok_exit "a dry run needs nothing and says what it would do" $rc
has "naming the account" "$out" 'would create the system account agent-busd'
has "the daemon's home is its own alone" "$out" "/var/lib/agent-bus/daemon agent-busd's own, 0700"
# A runner is a person's and installed only when asked for, so the default
# install makes no runner account and no agent.d.
lacks "and no runner account by default" "$out" 'agent-bus-runner\|agent\.d'
has "the unit" "$out" 'would write /etc/systemd/system/agent-busd.service'
has "and the start" "$out" 'would reload systemd'
# The web face is its own account and unit (docs/08-processes.md#the-web-face).
has "the web face gets its own account" "$out" 'would create the system account agent-bus-web'
has "its home names the web code directly" "$out" 'with no login and .*/web, its code, for home'
lacks "no obsolete web link" "$out" 'would link /var/lib/agent-bus/web'
has "and its unit is the one the release ships" "$out" 'would write /etc/systemd/system/agent-bus-web.service from .*/web/agent-bus-web.service'

has "and hands the first user to the program that owns that file" "$out" "would make $OWNER the first user"
out=$("$D/agent-bus-setup" --print-unit --owner parf 2>&1); rc=$?
ok_exit "an owner without a realm is a whole name" $rc
has "and is written as given, with nothing appended" "$out" '--owner parf\( \|$\)'
lacks "and the daemon runs no web face of its own" "$out" ' --web\|Delegate'

out=$("$D/agent-bus-setup" --print-unit --owner -parf 2>&1); rc=$?
bad_exit "while an owner that is no name is refused before anything is written" $rc

sec "the admin program owns what the account owns"
# Everything an operator does to the account's files, and nothing a user
# needs. The home is stated, so this edits a directory of its own rather than
# a real install. See docs/07-setup.md#the-programs.
# Stating the home is what lets this run at all, and it is also what hides a
# real install's first question: which account? Setup creates one name and
# admin looks up another, and nothing that passes AGENT_BUS_HOME would ever
# notice. So ask admin, with the home unset, who it expects to be.
WANT=$("$D/agent-bus-setup" --dry-run --owner "$OWNER" 2>&1 | sed -n 's/.*would create the system account \([^ ]*\) with home .*/\1/p' | head -1)
unset AGENT_BUS_TLS_DIR
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
# bytes over; `-` is how they arrive. See docs/07-setup.md#the-programs.
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

# PersonName is read from the host account database by the adapter; the
# command accepts an account name, never caller-supplied profile text.
adm user add smoke-admin-local@srv1 "$D/adm/user.pub" >/dev/null
adm user import-local smoke-admin-local@srv1 root >/dev/null
ADM_USERS=$(curl -s --unix-socket "$MINE" http://unix/users)
has "the local profile adapter imports passwd's person name" "$ADM_USERS" \
  '"name":"smoke-admin-local@srv1"[^}]*"person_name":"root"'
post_body "$OWNER" /user '{"name":"smoke-admin-explicit@srv1","person_name":"Administrator Choice","create":true}' >/dev/null
adm user import-local smoke-admin-explicit@srv1 root >/dev/null
ADM_USERS=$(curl -s --unix-socket "$MINE" http://unix/users)
has "while an Administrator-entered person name outranks a later passwd import" "$ADM_USERS" \
  '"name":"smoke-admin-explicit@srv1"[^}]*"person_name":"Administrator Choice"'
adm user remove smoke-admin-local@srv1 >/dev/null

sec "the dashboard shows envelopes and no bodies"
# A separate process that speaks the API, because that is what it is in the
# design — see docs/05-discovery.md#dashboard.
SECRET="lemon-curd-9f3a"
ab parf@localhost register '#board-svc@srv1' --kind agent --allow '*' --descr "watched by the board" >/dev/null
out=$(ab parf@localhost send '#board-svc@srv1' "$SECRET"); rc=$?
ok_exit "the node-feed fixture send is accepted" "$rc"
MSG=$(printf '%s' "$out" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
has "the node-feed fixture has a message id" "$MSG" '^[0-9a-f][0-9a-f]*$'
FEED=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" "http://unix/recent")
has "the daemon remembers the envelope it routed" "$FEED" "$MSG"
has "and who it was between" "$FEED" '"to":"#board-svc@srv1"'
is_empty "and never the body" "$(printf '%s' "$FEED" | grep -o "$SECRET")"
# The feed is each caller's own, not the operator's: what you were party to,
# and for the daemon Owner the node's. Refusing everyone else made the exchanges
# view impossible to show anybody (docs/05-discovery.md#what-it-shows).
ALICE_FEED=$(code alice@srv1 /recent)
has "an ordinary caller may read their scoped feed" "$ALICE_FEED" '200'
ab alice@srv1 register '#alice-svc@srv1' --kind agent --allow '*' --descr "hers" >/dev/null
# Party to it BOTH ways: one she sent, and one addressed to her. A feed that
# only ever showed what you sent would pass on the first alone.
out=$(ab alice@srv1 send '#alice-svc@srv1' "her own traffic"); rc=$?
ok_exit "the sent-feed fixture send is accepted" "$rc"
ASENT=$(printf '%s' "$out" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
has "the sent-feed fixture has a message id" "$ASENT" '^[0-9a-f][0-9a-f]*$'
# A user takes messages from an agent it may reach: her own, here.
out=$(ab '#alice-svc@srv1' send alice@srv1 "addressed to her"); rc=$?
ok_exit "the received-feed fixture send is accepted" "$rc"
AGOT=$(printf '%s' "$out" | sed -n 's/.*"message_id":"\([0-9a-f]*\)".*/\1/p')
has "the received-feed fixture has a message id" "$AGOT" '^[0-9a-f][0-9a-f]*$'
MINE=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $alice" "http://unix/recent")
has "and sees an exchange they were party to" "$MINE" "$ASENT"
has "and one addressed to them, not only what they sent" "$MINE" "$AGOT"
# The control that matters: seeing your own is worthless if you also see
# everyone else's, which is what the daemon-Owner-only rule protects.
is_empty "but not one between two other names" \
  "$(printf '%s' "$MINE" | grep -o "$MSG")"
# The daemon Owner's view is the NODE's, so it has to hold an exchange the
# Owner was no part of — the earlier feed is all the Owner's own traffic and would pass
# whatever the rule became.
NODE=$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" "http://unix/recent")
has "while the daemon Owner sees an exchange between two other names" "$NODE" "$ASENT"
# The dashboard is the TypeScript web face since 0.8.50, its own unit and
# its own tests: the web_ts shard (src/web/test).

}
restart() {
sec "a restart is not a loss"
# Its own daemon and its own database: the point of this section
# is what a stop and a start do to memory, which needs a process nothing
# else is using. See docs/04-messaging.md#durability.
mkdir -p "$D/dur"
DUR=""
dur_up() {
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+8)) --socket "$D/dur/bus.sock" \
    --owner "$OWNER" --db "$D/dur/bus.db" --create --flush-every "${2:-0}" >"$D/dur/$1.log" 2>&1 &
  DUR=$!
  ready "$D/dur/bus.sock" || echo "  WARNING: $D/dur/bus.sock never answered"
  DTOK=$(owner_token "$D/dur")
  [ "$1" = first ] || KTOK=$(AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#keeper@srv1' 2>/dev/null)
}
dur_down() { kill "$1" "$DUR" 2>/dev/null; wait "$DUR" 2>/dev/null; }
dab() { AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
# Reading an inbox is the inbox's own business, so the drain runs as keeper
# and not as the owner: an error from the wrong name would read as an empty
# queue to every check below.
dkeep() { AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$KTOK AGENT_BUS_NAME=#keeper@srv1 "$D/agent-bus" "$@"; }
dsvc() { local n; n=$(dab ls '#keeper@srv1' | sed -n "s/.*\"$1\":\([0-9]*\).*/\1/p"); echo "${n:-0}"; }

dur_up first
is_empty "a first start has nothing to say about a previous one" \
  "$(grep 'did not stop cleanly' "$D/dur/first.log")"
dab register '#keeper@srv1' --kind agent --allow '*' --descr "keeps things" >/dev/null
dab register '#silent@srv1' --kind agent --allow '*' >/dev/null
KTOK=$(AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token '#keeper@srv1')
dab send '#keeper@srv1' "before the restart" >/dev/null
dab send '#keeper@srv1' "also before it" >/dev/null
has "a reader took the first of them" "$(dkeep consume --wait 0s)" 'before the restart'
dur_down -TERM
# The graceful stop writes each day that counted something, open slot
# included, as one row per name and date; a silent name has none
# (docs/05-discovery.md#activity-history).
TODAY=$(date +%y%m%d)
rows=$(sqlite3 "$D/dur/bus.db" "SELECT day || ' ' || name FROM activity_days ORDER BY name" 2>&1)
has "the stop kept today's traffic as a row of its own" "$rows" "^$TODAY #keeper@srv1\$"
lacks "and a record that stayed silent has no row" "$rows" 'silent'

dur_up second
has "the registry is back after a graceful stop" "$(dab ls '#keeper@srv1')" 'keeps things'
is_empty "and status says nothing about a stop that was clean" \
  "$(dab status | grep -o unclean)"
# Counters are the other half of the state: a queue that was drained and one
# nobody ever wrote to read the same without them.
has "the counters come back too, not just the messages" "$(dsvc in)" '^2$'
has "and a read is remembered as a read" "$(dsvc out)" '^1$'
has "and so is the message nobody had taken" "$(dkeep consume --wait 0s)" 'also before it'
# The start rebuilt the ring from that row: today's range still counts the
# two sends and the read made before the stop.
dday=$(curl -s --unix-socket "$D/dur/bus.sock" -H "X-Agent-Bus-Token: $DTOK" "http://unix/activity/days?name=%23keeper%40srv1&from=$TODAY&to=$TODAY")
has "today's range after a restart still holds the traffic before it" \
  "$(printf '%s' "$dday" | grep -o '"in":[0-9]*' | awk -F: '{s+=$2} END {print s+0}')" '^2$'
dur_down -TERM

dur_up third
out=$(dkeep consume --wait 0s 2>&1); rc=$?
is_empty "a drained queue stays drained — nothing is delivered twice" "$out"
ok_exit "and an empty queue is an answer, not an error" $rc
has "while the record that owned it is still there" "$(dab ls '#keeper@srv1')" 'keeps things'
has "and both reads are still counted" "$(dsvc out)" '^2$'
# A secret is acknowledged only once it is durable, and a graceful stop would
# have written it down whether the write did or not. Set here, before the
# message that dies with the process, because writing it checkpoints.
# See docs/03-records-service.md#secrets.
dab register vault@srv1 --addr db.example:5432 --protocol postgresql --allow '*' >/dev/null
dab secret vault@srv1 'PGPASSWORD=survives-the-kill' >/dev/null
# SIGKILL: no flush happens, so the queues on disk are the ones the start
# wrote, and its unclean mark is what says the run ended badly.
dab send '#keeper@srv1' "lost with the process" >/dev/null
# The bus is the process that holds the state, so the bus is what dies here.
# By parent pid, never by a pattern: a pattern matches whatever else is on
# the host, up to and including whoever is running this.
kill -9 "$(pgrep -P "$DUR" | head -1)" 2>/dev/null
ready "$D/dur/bus.sock" || echo "  WARNING: the bus never came back"
has "a bus that dies is started again" "$(cat "$D/dur/third.log")" 'restarting in'
has "and the start that follows says the last one ended badly" \
  "$(cat "$D/dur/third.log")" 'did not stop cleanly'
has "and says from when it is missing traffic" "$(cat "$D/dur/third.log")" 'queue changes after'
# Logged once at start is not enough: whoever comes to look at a gap in the
# work arrives long after that line scrolled away.
has "and status still says so, not only the log" "$(dab status)" '"unclean":true'
is_empty "the message that died with the process is not invented back" \
  "$(dkeep consume --wait 0s 2>&1)"
has "while a secret written before the kill is there after it" \
  "$(dab secret vault@srv1)" '^PGPASSWORD=survives-the-kill$'
dur_down -TERM

# Loss is state like the counters: a restart puts it back on the inbox that
# suffered it, and the node total is the sum of those rather than a second
# copy that could disagree.
dur_up fourth
dab register '#lossy@srv1' --kind agent --allow '*' --overflow ring --bound 1 >/dev/null
dab send '#lossy@srv1' "first" >/dev/null; dab send '#lossy@srv1' "second" >/dev/null
has "an inbox that dropped something says so" "$(dab ls '#lossy@srv1')" '"dropped":1'
dur_down -TERM
dur_up fifth
has "and still says so after a restart" "$(dab ls '#lossy@srv1')" '"dropped":1'
has "while the node total is the sum of its inboxes" "$(dab status)" '"dropped":1'
dur_down -TERM

# A dedicated runner outlives its daemon: the daemon stops, it waits
# for it, and serves again once it is back (docs/06-dedicated-runner.md#script-agents).
dur_up sixth
printf '#!/bin/sh\necho "pong $1"\n' > "$D/dur/echo.sh"; chmod +x "$D/dur/echo.sh"
# The binary itself in the background, never a function: $! must be the runner.
AGENT_BUS_ADDR=$D/dur/bus.sock AGENT_BUS_TOKEN=$DTOK AGENT_BUS_NAME=$OWNER \
  "$D/agent-bus" run '#echoer@srv1' --algo=args "$D/dur/echo.sh" >"$D/dur/runner.log" 2>&1 &
RUNNER=$!
for _ in $(seq 1 50); do dab ls '#echoer@srv1' 2>/dev/null | grep -q '"readers":1' && break; sleep 0.1; done
has "run records the script that serves the agent" \
  "$(dab ls '#echoer@srv1')" '"script":"[^"]*/dur/echo.sh"'
lacks "and not in addr, a 📡's field" "$(dab ls '#echoer@srv1')" '"addr":'
has "a runner answers before its daemon stops" \
  "$(dab call '#echoer@srv1' --wait 10s before 2>&1)" 'pong before'
dur_down -TERM
sleep 1.5
kill -0 "$RUNNER" 2>/dev/null; ok_exit "and is still running while the daemon is away" $?
has "and says the daemon is away rather than exiting" "$(cat "$D/dur/runner.log")" 'daemon away'
has "and ps says so too" "$(ps -o args= -p "$RUNNER")" '; away$'
dur_up seventh
has "then answers again once the daemon is back" \
  "$(dab call '#echoer@srv1' --wait 30s after 2>&1)" 'pong after'
has "and says it reconnected" "$(cat "$D/dur/runner.log")" 'reconnected after'
sleep 1.2
lacks "and ps no longer does" "$(ps -o args= -p "$RUNNER")" 'away'
kill -TERM "$RUNNER" 2>/dev/null
for _ in $(seq 1 50); do kill -0 "$RUNNER" 2>/dev/null || break; sleep 0.1; done
kill -0 "$RUNNER" 2>/dev/null; bad_exit "and still stops when asked" $?
kill -9 "$RUNNER" 2>/dev/null; wait "$RUNNER" 2>/dev/null
dur_down -TERM

if normal; then
  # A message whose moment passed while the daemon was down is not worth
  # delivering late, and the reload is where that is decided.
  dur_up sixth
  dab send '#keeper@srv1' --ttl 2s "too late by the time you read this" >/dev/null
  dab send '#keeper@srv1' "still worth having" >/dev/null
  dur_down -TERM
  sleep 3
  dur_up seventh
  has "a message that outlived its ttl while down is not delivered" \
    "$(dkeep consume --wait 0s)" 'still worth having'
  is_empty "and it is the only one left" "$(dkeep consume --wait 0s 2>&1)"
  dur_down -TERM

  # The periodic flush is what bounds a crash's loss: a message queued more
  # than one interval before a SIGKILL was written down by the ticker, since
  # nothing else flushes a queue while the daemon runs.
  dur_up eighth 1s
  dab send '#keeper@srv1' "flushed before the kill" >/dev/null
  sleep 2.5
  kill -9 "$(pgrep -P "$DUR" | head -1)" 2>/dev/null
  for _ in $(seq 1 100); do grep -q 'did not stop cleanly' "$D/dur/eighth.log" && break; sleep 0.1; done
  ready "$D/dur/bus.sock" || echo "  WARNING: the bus never came back"
  # Without the kill the message is simply still in memory, so the kill has
  # to be seen to have happened.
  has "the flushing bus was killed" "$(cat "$D/dur/eighth.log")" 'did not stop cleanly'
  has "and a message queued an interval before the kill survives it" \
    "$(dkeep consume --wait 0s 2>&1)" 'flushed before the kill'
  dur_down -TERM

  # A key-value write is committed before it is answered, not on the queue
  # checkpoint: one answered just before a SIGKILL is still there after it
  # (Plans/R1.0-Release/done/kv.md#per-record-storage).
  dur_up ninth
  has "a value is written" "$(dab kv set '#keeper@srv1' stage answered 2>&1)" 'stage set'
  kill -9 "$(pgrep -P "$DUR" | head -1)" 2>/dev/null
  for _ in $(seq 1 100); do grep -q 'did not stop cleanly' "$D/dur/ninth.log" && break; sleep 0.1; done
  ready "$D/dur/bus.sock" || echo "  WARNING: the bus never came back"
  has "the bus holding it was killed" "$(cat "$D/dur/ninth.log")" 'did not stop cleanly'
  has "and the value answered before the kill survives it" "$(dab kv get '#keeper@srv1' stage 2>&1)" '^answered$'
  dur_down -TERM
fi

}
orphans() {
sec "a start ignores and reports the records it could not have written"
# docs/constitution.md#persistence-and-loading: an incorrect stored record is
# ignored — not loaded, not repaired, not deleted — and reported, while the rest
# of the node starts. Its own daemon and its own store, because what is under
# test is what a start reads off disk.
#
# The store is edited between the stop and the start, and that is the only way
# to present one: a running daemon refuses every call that would write one.
mkdir -p "$D/orph"
orph_up() {
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+7)) --socket "$D/orph/bus.sock" \
    --owner "$OWNER" --db "$D/orph/bus.db" --create --flush-every 0 >"$D/orph/$1.log" 2>&1 &
  OPID=$!
  # The status is the caller's to act on, not a warning to scroll past: every
  # call below blocks on an unserved socket rather than failing, so a start
  # that never answered would hang the run instead of failing a check.
  ready "$D/orph/bus.sock" || return 1
  OTOK=$(owner_token "$D/orph")
}
orph_down() { kill "$OPID" 2>/dev/null; wait "$OPID" 2>/dev/null; }
otok() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$OTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token "$1" 2>/dev/null; }
oab() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$OTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
oas() { AGENT_BUS_ADDR=$D/orph/bus.sock AGENT_BUS_TOKEN=$(otok "$1") AGENT_BUS_NAME=$1 "$D/agent-bus" "${@:2}"; }
# A fixture failure stops the run, and must not leave this section's daemon
# behind when it does: the trap at the top knows about the main one only.
opost() { curl -fsS --unix-socket "$D/orph/bus.sock" -H "X-Agent-Bus-Token: $OTOK" -d "$2" "http://unix$1" >/dev/null || { orph_down; exit 1; }; }
ocode() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/orph/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix/status"; }

# A body rather than a run of statements, so a start that never came up can
# leave it: everything below waits on a bound socket nothing serves.
orph_checks() {
  orph_up first || { echo "  FAIL the orphan fixture's daemon did not start"; fail=$((fail+1)); return 1; }
  for u in keeper@srv1 napping@srv1 barred@srv1 ghost@srv1 drifter@srv1; do
    opost /user "{\"name\":\"$u\",\"create\":true}"
  done
  # Two kinds of damage. #lost@srv1's owner is repointed at a name that is
  # nobody; drifter@srv1 loses its user record, which makes the User itself
  # incorrect — and so, on the same start, the agent it owns.
  oas ghost@srv1 register '#lost@srv1' --kind agent --allow '*' --descr "answers for nobody" >/dev/null
  oas drifter@srv1 register '#chain-a@srv1' --kind agent --allow '*' >/dev/null
  # Positive controls, one per owner state the daemon still knows about, plus
  # one owned by the daemon owner.
  oas keeper@srv1 register '#steady@srv1' --kind agent --allow '*' >/dev/null
  oas napping@srv1 register '#napped@srv1' --kind agent --allow '*' >/dev/null
  oas barred@srv1 register '#barred-svc@srv1' --kind agent --allow '*' >/dev/null
  oab register '#owner-svc@srv1' --kind agent --allow '*' >/dev/null
  for s in '#lost@srv1' '#chain-a@srv1' '#steady@srv1' '#napped@srv1' '#barred-svc@srv1' '#owner-svc@srv1'; do
    oab send "$s" "queued before the stop" >/dev/null
  done
  # Deactivated after the queues exist: an inactive owner's agent is no such
  # receiver, so seeding second would seed nothing and the controls would
  # survive with nothing to lose.
  opost /user/state '{"name":"napping@srv1","status":"inactive"}'
  opost /user/state '{"name":"barred@srv1","status":"inactive"}'
  LOSTTOK=$(otok '#lost@srv1')
  CHAINTOK=$(otok '#chain-a@srv1')
  has "the records authenticate before the start that ignores them" \
    "$(ocode "$LOSTTOK")$(ocode "$CHAINTOK")" '200200'
  orph_down

  # The hand edit, with sqlite3 on the stopped daemon's database: a running
  # daemon holds it exclusively.
  ODB=$D/orph/bus.db
  osql() { sqlite3 "$ODB" "$@"; }
  osql "UPDATE records SET body = replace(body, '\"owner\":\"ghost@srv1\"', '\"owner\":\"vanished@srv1\"') WHERE name = '#lost@srv1'"
  osql "DELETE FROM records WHERE name = 'drifter@srv1'"
  has "the store now holds a record owned by a name nothing knows" \
    "$(osql "SELECT body FROM records WHERE name = '#lost@srv1'")" '"owner":"vanished@srv1"'
  lacks "and a User without its user record" "$(osql 'SELECT name FROM records')" 'drifter@srv1'
  has "whose user row is still there" "$(osql 'SELECT name FROM users')" 'drifter@srv1'
  # And every trace of the daemon owner as a *principal* goes with it: their
  # profile, and their line in the administrators group, which a reload could
  # otherwise turn back into a profile. A stored owner must fail closed on that
  # damage rather than silently resurrecting setup's seed.
  cp "$ODB" "$D/orph/before-owner-edit.db"
  # A Group is a record: its membership is the record's allow list.
  osql "DELETE FROM users WHERE name = '$OWNER'; UPDATE records SET body = json_set(body, '\$.allow', json('[]')) WHERE name = '@administrators'"
  EDUSERS=$(osql 'SELECT name FROM users')
  EDGROUPS=$(osql "SELECT name || '=' || coalesce(json_extract(body, '\$.allow'), '[]') FROM records WHERE kind = 'group'")
  has "the edited store still lists the users it kept" "$EDUSERS" 'keeper@srv1'
  has "and still has a maintainers group to read" "$EDGROUPS" '@administrators='
  lacks "but no profile for the daemon owner, as a hand-edited store may not" \
    "$EDUSERS" "$OWNER"
  lacks "nor a line in the group a reload would rebuild one from" "$EDGROUPS" "$OWNER"

  "$D/agent-busd" --addr 127.0.0.1:$((PORT+7)) --socket "$D/orph/bus.sock" \
    --owner "$OWNER" --db "$ODB" --flush-every 0 >"$D/orph/damaged.log" 2>&1 &
  OPID=$!
  sleep 0.5
  is_empty "a damaged current owner never reaches a serving bus" \
    "$(curl -s --max-time 0.2 --unix-socket "$D/orph/bus.sock" http://unix/status 2>/dev/null)"
  has "and startup names the missing durable owner" \
    "$(cat "$D/orph/damaged.log")" "snapshot daemon owner $OWNER is not a registered user"
  orph_down
  # The owner damage has made its point; the rest runs on the store before it.
  cp "$D/orph/before-owner-edit.db" "$ODB"

  rm -f "$D/orph/logs/error.log"
  orph_up second || { echo "  FAIL the start over the edited store did not come up"; fail=$((fail+1)); return 1; }
  ERRLOG=$(cat "$D/orph/logs/error.log" 2>/dev/null)
  has "the start reports the record whose owner is nobody" "$ERRLOG" 'stored record #lost@srv1 is ignored: its owner vanished@srv1 is not a user'
  has "and the User without its user record" "$ERRLOG" 'stored user drifter@srv1 has no user record of its own and is ignored'
  has "and, on the same start, the agent that User owned" "$ERRLOG" 'stored record #chain-a@srv1 is ignored: its owner drifter@srv1 is not a user'
  has "and the queues that went with them" "$ERRLOG" 'stored queue #lost@srv1 has no record that can hold it'
  lacks "while no control is reported" "$ERRLOG" 'steady\|napped\|barred-svc\|owner-svc'
  REG=$(oab ls --all)
  lacks "an ignored record is not listed" "$REG" '#lost@srv1'
  lacks "nor the agent of an ignored User" "$REG" '#chain-a@srv1'
  has "while a record untouched by the edit is" "$REG" 'ghost@srv1'
  has "the credential that answered for an ignored record no longer authenticates" \
    "$(ocode "$LOSTTOK")" '401'
  has "nor the one for the ignored User's agent" "$(ocode "$CHAINTOK")" '401'
  # Every control keeps its queue. A stopped owner is not a missing one.
  for s in '#steady@srv1' '#owner-svc@srv1'; do
    has "$s survives the start with its queue" "$(oab ls "$s")" '"queued":1'
  done
  # An inactive User's agents are no such records, and so are only in the
  # read-only view — where they are, with their queues, not ignored.
  OINACT=$(curl -s --unix-socket "$D/orph/bus.sock" -H "X-Agent-Bus-Token: $OTOK" http://unix/inactive)
  for s in '#napped@srv1' '#barred-svc@srv1'; do
    has "$s survives the start with its queue" "$(printf '%s' "$OINACT" | grep -o "{\"name\":\"$s\"[^}]*")" '"queued":1'
  done
  # The name is free on the running bus, and carries nothing across.
  oas keeper@srv1 register '#lost@srv1' --kind agent --allow '*' --descr "somebody else's now" >/dev/null
  has "and the freed name registers to somebody else" "$(oab ls '#lost@srv1')" "somebody else's now"
  is_empty "who is handed none of what was queued for the old one" \
    "$(oas '#lost@srv1' consume --wait 0s 2>&1)"
  # The old bytes are refused NOW, with the name registered again. A name with
  # no record is refused at the gate whether or not its credential was ever
  # dropped, so the 401 above proves nothing on its own.
  has "and the credential the old holder kept does not answer for the new one" \
    "$(ocode "$LOSTTOK")" '401'
  # What the start committed is on disk before anything stops gracefully: the
  # daemon is killed outright. The positive half comes first: `lacks` on an
  # empty or unreadable store passes for every absence there is.
  OKIDS=$(pgrep -P "$OPID" | tr '\n' ' ')
  kill -9 "$OPID" 2>/dev/null; wait "$OPID" 2>/dev/null
  for _ in $(seq 1 40); do [ -z "$(ps -o pid= -p $OKIDS 2>/dev/null)" ] && break; sleep 0.1; done
  has "the controls are in the store" "$(osql 'SELECT name FROM records')" '#owner-svc@srv1'
  has "and their queued work is in it" "$(osql "SELECT body FROM messages WHERE queue = '#steady@srv1'")" 'queued before the stop'
  # Ignored is not deleted: the database keeps what it held for an operator.
  has "the ignored agent is still in the database" "$(osql 'SELECT name FROM records')" '#chain-a@srv1'
  has "with its queued work" "$(osql "SELECT body FROM messages WHERE queue = '#chain-a@srv1'")" 'queued before the stop'
  has "the new #lost@srv1 is keeper's in the store" "$(osql "SELECT body FROM records WHERE name = '#lost@srv1'")" '"owner":"keeper@srv1"'
  # A freed name is born without a stored queue: the ignored record's messages
  # would otherwise reattach to the new owner on the next start.
  is_empty "and the old holder's queued work went with the old record" \
    "$(osql "SELECT body FROM messages WHERE queue = '#lost@srv1'")"

  # And a third start reads that store the same way: ignoring is not a repair.
  rm -f "$D/orph/logs/error.log"
  if orph_up third; then
    THIRD=$(oab ls --all)
    has "a third start reads that store" "$THIRD" '"name":"#owner-svc@srv1"'
    has "with the queue that survived still on it" "$(oab ls '#steady@srv1')" '"queued":1'
    has "and the work still in it" "$(oas '#steady@srv1' consume --wait 0s)" 'queued before the stop'
    lacks "and still does not load the ignored agent" "$THIRD" '#chain-a@srv1'
    has "and reports it again" "$(cat "$D/orph/logs/error.log" 2>/dev/null)" 'stored record #chain-a@srv1 is ignored'
    has "while the renamed holder's record loads as keeper's" "$(oab ls '#lost@srv1')" '"owner":"keeper@srv1"'
  else
    echo "  FAIL a third start reads that store: it never answered"; fail=$((fail+1))
  fi
}
orph_checks
orph_down

}
core_8() {
sec "the CLI manages a record: status, lists by delta, routes and groups"
# K.16: management through the supported face, each answer read back from the
# daemon rather than from the command's own output.
ab owner@srv1 channel create cli-jobs@srv1 --allow owner@srv1 >/dev/null
ab owner@srv1 register '#cli-box@srv1' --kind agent --allow cli-jobs@srv1 >/dev/null
users cli-bob@srv1
ab owner@srv1 manage cli-jobs@srv1 --add-to-set-allow cli-bob@srv1 >/dev/null
has "add_to_set adds a term" "$(ab owner@srv1 ls cli-jobs@srv1)" '"allow":\["owner@srv1","cli-bob@srv1"\]'
ab owner@srv1 manage cli-jobs@srv1 --add-to-set-allow cli-bob@srv1 >/dev/null
has "and adds it once" "$(ab owner@srv1 ls cli-jobs@srv1)" '"allow":\["owner@srv1","cli-bob@srv1"\]'
has "add refuses a term already there" \
  "$(ab owner@srv1 manage cli-jobs@srv1 --add-allow cli-bob@srv1 2>&1)" 'already names cli-bob@srv1'
has "an unmarked agent term is refused with its spelling" \
  "$(ab owner@srv1 manage cli-jobs@srv1 --add-allow cli-box@srv1 2>&1)" 'the agent is #cli-box@srv1'
ab owner@srv1 manage cli-jobs@srv1 --remove-allow cli-bob@srv1 >/dev/null
has "remove takes it" "$(ab owner@srv1 ls cli-jobs@srv1)" '"allow":\["owner@srv1"\]'
ab owner@srv1 manage cli-jobs@srv1 --deliver-to '#cli-box@srv1' >/dev/null
has "a route is stored and allowed" "$(ab owner@srv1 ls cli-jobs@srv1)" '"subs":\["#cli-box@srv1"\].*"route_allowed":true'
ab owner@srv1 send cli-jobs@srv1 "routed" >/dev/null
has "and a message sent to the queue moves to the agent" \
  "$(ab '#cli-box@srv1' consume --wait 2s)" '"original_to":"cli-jobs@srv1"'
ab owner@srv1 manage cli-jobs@srv1 --status inactive >/dev/null
is_empty "an inactive record is in no listing" "$(ab owner@srv1 ls --all 2>/dev/null | grep -o '"name":"cli-jobs@srv1"')"
has "and only in the inactive view" \
  "$(curl -s --unix-socket "$D/bus.sock" -H "X-Agent-Bus-Token: $TOKEN" http://unix/inactive)" '"name":"cli-jobs@srv1"'
ab owner@srv1 manage cli-jobs@srv1 --status active >/dev/null
has "and reactivation brings it back" "$(ab owner@srv1 ls cli-jobs@srv1)" '"status":"active"'
ab owner@srv1 group @cli-crew cli-bob@srv1 '#cli-box@srv1' >/dev/null
has "a group is set and read back" "$(ab owner@srv1 group @cli-crew | LC_ALL=C sort | tr '\n' ' ')" '^#cli-box@srv1 cli-bob@srv1 $'
has "and is a record owned by its creator" "$(ab owner@srv1 ls @cli-crew)" '"kind":"group".*"owner":"owner@srv1"'

}
transfer() {
sec "the daemon account's socket follows the daemon Owner across a transfer"
# The socket is the daemon Owner's, not the setup seed's: after a transfer,
# with or without a restart under the same --owner flag it answers as the new
# Owner.
mkdir -p "$D/xfer"
xfer_up() {
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+2)) --socket "$D/xfer/bus.sock" \
    --owner "$OWNER" --db "$D/xfer/bus.db" --create --flush-every 0 >"$D/xfer/$1.log" 2>&1 &
  XPID=$!
  ready "$D/xfer/bus.sock" || return 1
}
xfer_down() { kill "$XPID" 2>/dev/null; wait "$XPID" 2>/dev/null; }
xfer_checks() {
  xfer_up first || { echo "  FAIL the transfer fixture did not start"; fail=$((fail+1)); return 1; }
  XSOCK="$D/xfer/user-$ACCOUNT.sock"
  has "the daemon account's socket is the seed Owner at first" \
    "$(curl -s --unix-socket "$XSOCK" http://unix/status)" "\"you\":\"$OWNER\""
  curl -fsS --unix-socket "$XSOCK" -d '{"name":"heir@srv1","create":true}' http://unix/user >/dev/null
  curl -fsS --unix-socket "$XSOCK" -d '{"name":"heir@srv1"}' http://unix/owner >/dev/null
  has "after the transfer, without a restart, it is the new Owner" \
    "$(curl -s --unix-socket "$XSOCK" http://unix/status)" '"you":"heir@srv1","administrator":true,"daemon_owner":true'
  xfer_down
  xfer_up second || { echo "  FAIL the transfer fixture did not restart"; fail=$((fail+1)); return 1; }
  has "after the transfer and a restart it is the new Owner" \
    "$(curl -s --unix-socket "$XSOCK" http://unix/status)" '"you":"heir@srv1","administrator":true,"daemon_owner":true'
}
xfer_checks
xfer_down

}
corrupt_pair() {
sec "a corrupt credential pair reaches the local error log, a refusal does not"
# docs/constitution.md#errors-and-alerts: a violated invariant or corrupt
# stored state produces an error.log line; production syslog severity is
# checked with a fake sink in journal tests so normal never writes host logs. An
# ordinary refusal produces neither. One run exercises both, and each line is
# looked for by a name unique to this run, so another daemon's lines on the
# same host cannot satisfy it.
mkdir -p "$D/pair"
PAIRNAME="#pair-$$@srv1"
GHOST="ghost-$$@srv1"
pair_up() {
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+5)) --socket "$D/pair/bus.sock" \
    --owner "$OWNER" --db "$D/pair/bus.db" --create --flush-every 0 >"$D/pair/$1.log" 2>&1 &
  PPID_=$!
  ready "$D/pair/bus.sock" || return 1
  PTOK=$(owner_token "$D/pair")
}
pair_down() { kill "$PPID_" 2>/dev/null; wait "$PPID_" 2>/dev/null; }
pab() { AGENT_BUS_ADDR=$D/pair/bus.sock AGENT_BUS_TOKEN=$PTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" "$@"; }
pcode() { curl -s -o /dev/null -w '%{http_code}' --unix-socket "$D/pair/bus.sock" -H "X-Agent-Bus-Token: $1" "http://unix/status"; }
pair_checks() {
  pair_up first || { echo "  FAIL the pair fixture's daemon did not start"; fail=$((fail+1)); return 1; }
  pab register "$PAIRNAME" --kind agent >/dev/null
  AGTOK=$(AGENT_BUS_ADDR=$D/pair/bus.sock AGENT_BUS_TOKEN=$PTOK AGENT_BUS_NAME=$OWNER "$D/agent-bus" token "$PAIRNAME")
  has "the agent's credential works before the damage" "$(pcode "$AGTOK")" '200'
  pair_down
  # The damage a lost write would leave: the row names a User the agent's
  # Owner is not.
  sqlite3 "$D/pair/bus.db" "UPDATE credentials SET user_id = user_id + 1000 WHERE name = '$PAIRNAME'"
  rm -f "$D/pair/logs/error.log"
  SINCE=$(date '+%Y-%m-%d %H:%M:%S')
  pair_up second || { echo "  FAIL the start over the damaged pair did not come up"; fail=$((fail+1)); return 1; }
  # And an ordinary refusal in the same run: an unknown receiver.
  pab send "$GHOST" "nobody is here" >/dev/null 2>&1
  has "the ordinary refusal was a refusal" "$(pab send "$GHOST" again 2>&1)" 'no such receiver'
  has "the damaged credential authenticates nothing" "$(pcode "$AGTOK")" '401'
  pair_down
  ERRLOG=$(cat "$D/pair/logs/error.log" 2>/dev/null)
  SYSLOG=$(journalctl -t agent-busd --since "$SINCE" --no-pager -o cat 2>/dev/null)
  want="stored credential for $PAIRNAME is ignored"
  has "the error log reports the corrupt pair" "$ERRLOG" "$want"
  has "at alert severity" "$(printf '%s\n' "$ERRLOG" | grep -F "$want")" ' alert '
  lacks "a test daemon does not copy alerts to host syslog" "$SYSLOG" "$want"
  has "the local alert is recorded once" "$(printf '%s\n' "$ERRLOG" | grep -cF "$want")" '^1$'
  lacks "the error log names the refusal nowhere" "$ERRLOG" "$GHOST"
  lacks "nor does syslog" "$SYSLOG" "$GHOST"
  lacks "neither copy holds the credential" "$ERRLOG$SYSLOG" "$AGTOK"
  is_empty "the error log holds warnings, errors and alerts only" \
    "$(printf '%s\n' "$ERRLOG" | grep -v '^$' | grep -Ev '^[^ ]+ (warning|error|alert) ')"
}
pair_checks
pair_down

# A start refused for corrupt stored authority says why in the error log and
# stderr too (K.33): @administrators owned by somebody who is
# not the daemon Owner refuses the start (docs/constitution.md#what-refuses-the-start).
sqlite3 "$D/pair/bus.db" "UPDATE records SET body = json_set(body, '$.owner', '$GHOST') WHERE name = '@administrators'"
rm -f "$D/pair/logs/error.log"
SINCE=$(date '+%Y-%m-%d %H:%M:%S')
timeout 4 "$D/agent-busd" --addr 127.0.0.1:$((PORT+5)) --socket "$D/pair/bus.sock" \
  --owner "$OWNER" --db "$D/pair/bus.db" --flush-every 0 >"$D/pair/refused.log" 2>&1
has "a refused start says so on stderr" "$(cat "$D/pair/refused.log")" "owned by $GHOST"
has "and in the error log, at alert" "$(grep -F "owned by $GHOST" "$D/pair/logs/error.log" 2>/dev/null | head -1)" ' alert "the daemon cannot start: '
lacks "a refused test start does not write host syslog" "$(journalctl -t agent-busd --since "$SINCE" --no-pager -o cat 2>/dev/null)" "the daemon cannot start: .*owned by $GHOST"

}
supervisor() {
sec "the supervisor holds the sockets, and the bus serves them"
# One binary, two roles. The process that may chown a socket never serves a
# request; the process that serves is handed listeners that already exist and
# could not make one. See docs/08-processes.md#the-rule.
mkdir -p "$D/sup"
# --web ran the Go dashboard as a child until 0.8.50; a unit written before
# then still passes it, so it is accepted and does nothing.
SUPUNIT=""
"$D/agent-busd" --addr 127.0.0.1:$((PORT+13)) --socket "$D/sup/bus.sock" \
  --owner "$OWNER" --db "$D/sup/bus.db" --create --flush-every 0 --web >"$D/sup/log" 2>&1 &
SUPRUN=$!
SUP=$SUPRUN
ready "$D/sup/bus.sock" || echo "  WARNING: $D/sup/bus.sock never answered"
# By parent pid throughout: a pattern would match anything else on the host.
has "the daemon is a supervisor and its one child, the bus" "$(pgrep -P "$SUP" | wc -l)" '^1$'
lacks "and the old --web asks for no dashboard child" "$(pgrep -P "$SUP" -a)" 'agent-bus-web\|bwrap'
descendants() {
  local child
  for child in $(pgrep -P "$1"); do echo "$child"; descendants "$child"; done
}
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
# Asked, not killed: only a bus that was told to stop writes a clean mark, so
# this is where a stop that was not passed on shows up.
has "and asks them, so the bus writes what it holds on the way out" \
  "$(sqlite3 "$D/sup/bus.db" "SELECT value FROM meta WHERE key = 'clean'")" '^1$'
is_empty "and takes the sockets it made with it" "$(ls "$D/sup/"*.sock 2>/dev/null)"
# A supervisor that is killed outright cannot tidy up, so the kernel does it:
# an orphaned bus would keep the listeners and the next start would find the
# address in use.

  "$D/agent-busd" --addr 127.0.0.1:$((PORT+13)) --socket "$D/sup/bus.sock" \
  --owner "$OWNER" --db "$D/sup/bus.db" --create --flush-every 0 >"$D/sup/log2" 2>&1 &
SUP2=$!
ready "$D/sup/bus.sock" || echo "  WARNING: $D/sup/bus.sock never answered the second time"
KIDS2=$(pgrep -P "$SUP2" | tr '\n' ' ')
kill -9 "$SUP2" 2>/dev/null; wait "$SUP2" 2>/dev/null
LEFT=$(gone "$KIDS2")
is_empty "a supervisor killed outright leaves no bus behind" "$LEFT"
# A build that does leave one must not leave it for the next run, holding a
# port. By the pids taken above, never by a pattern.
[ -n "$LEFT" ] && kill -9 $LEFT 2>/dev/null

}
core_9() {
sec "the daemon keeps three logs"
# docs/constitution.md#logs. The main fixture's logs are beside its database.
LOGD=$D/logs
has "an audit log and an error log exist from the start" \
  "$([ -f "$LOGD/audit.log" ] && [ -f "$LOGD/error.log" ] && echo yes)" 'yes'
is_empty "and no debug log, which nobody asked for" "$(ls "$LOGD/debug.log" 2>/dev/null)"
has "they are the daemon account's to write and its group's to read" \
  "$(stat -c %a "$LOGD/audit.log")" '^640$'
ab alice@srv1 register '#audited@srv1' --kind agent --allow '*' --descr "LOGCANARY-descr" >/dev/null
AUDIT=$(cat "$LOGD/audit.log")
has "an entity edit is audited with its actor, operation, target and result" "$AUDIT" \
  'actor="alice@srv1" op="register" target="#audited@srv1" result="ok"'
has "a refused edit is audited too" \
  "$(post_body bob@srv1 /manage '{"name":"#audited@srv1","descr":"not bob'"'"'s"}' >/dev/null; cat "$LOGD/audit.log")" \
  'actor="bob@srv1" op="manage" target="#audited@srv1" result="refused 403"'
ab alice@srv1 send '#audited@srv1' "LOGCANARY-body" >/dev/null
lacks "a send writes no audit entry" "$(cat "$LOGD/audit.log")" 'op="send"'
lacks "and no value the caller sent reaches the log" "$(cat "$LOGD/"*.log)" 'LOGCANARY'
lacks "nor any credential" "$(cat "$LOGD/"*.log)" "$TOKEN"
has "only the daemon owner switches the debug log" \
  "$(post_code alice@srv1 /debug '{"on":true}')" '^403$'
is_empty "and a refused switch leaves it off" "$(ls "$LOGD/debug.log" 2>/dev/null)"
has "the daemon owner can" "$(post_body "$OWNER" /debug '{"on":true}')" '"on":true'
ab alice@srv1 ls '#audited@srv1' >/dev/null
has "and then every request has a line" "$(cat "$LOGD/debug.log")" 'caller="alice@srv1" "GET" "/lookup" status=200'
post_body "$OWNER" /debug '{"on":false}' >/dev/null
ab alice@srv1 status >/dev/null
lacks "until it is switched off again" "$(cat "$LOGD/debug.log")" '"/status"'
has "while the line before the switch is still there" "$(cat "$LOGD/debug.log")" '"/lookup"'
lacks "and the debug log holds no credential either" "$(cat "$LOGD/debug.log")" "$(tok alice@srv1)"

}
# The MCP face was one section of six harnesses run one after another, half a
# minute together. Each is its own shard now, on its own daemon, so each asks
# for the identities it uses; mcp_face() says why bun is not optional.
mcp_face() { # name, then the body that needs bun
  local name=$1; shift
  if normal; then
    sec "the MCP face: $name"
    # bun is not optional: the MCP face is part of the required minimum
    # (docs/00-overview.md), so a host without it fails rather than passing green.
    if ! command -v bun >/dev/null 2>&1; then
      echo "  FAIL bun is not installed; the MCP face cannot be checked"
      fail=$((fail + 1))
    else
      "$@"
    fi
  else skipped=$((skipped+1)); fi
}
mcp_names() {
    # each harness runs its own peer in-process, so there is no start-order race
    # These harnesses ask for credentials before their first record refresh.
    # Provision their agent identities explicitly; no mint creates a name.
    for name in mcp.session peer peer.third pusher push.session; do
      ab "$OWNER" register "#$name@srv1" --kind agent --allow '*' >/dev/null || exit 1
    done
}
mcp_rpc_body() {
    # The shared JSON-RPC plumbing, driven directly: the harnesses below only
    # ever have one request in flight and never split a line across chunks, so
    # they leave most of rpc.ts unwatched (mcp/rpc.test.ts says why).
    out=$(cd mcp && timeout 60 bun test ../launchers/restart.test.ts ../launchers/sessions.test.ts ../launchers/plugin-steps.test.ts rpc.test.ts messages.test.ts catalogue.test.ts ../launchers/local.test.ts ../launchers/terminal.test.ts ../launchers/runtime-auth.test.ts adapter-status.test.ts push.test.ts face-mark.test.ts face.test.ts resources.test.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "rpc unit tests" $rc
}
mcp_smoke_body() {
    mcp_names
    out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock \
          AGENT_BUS_OWNER=$OWNER AGENT_BUS_OWNER_TOKEN=$TOKEN \
          AGENT_BUS_TOKEN=$(tok '#mcp.session@srv1') \
          AGENT_BUS_NAME='#mcp.session@srv1' SMOKE_PEER='#peer@srv1' bun run smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "mcp smoke" $rc
}
mcp_push_body() {
    mcp_names
    out=$(cd mcp && timeout 120 env AGENT_BUS_ADDR=$D/bus.sock \
          AGENT_BUS_OWNER=$OWNER AGENT_BUS_OWNER_TOKEN=$TOKEN \
          AGENT_BUS_TOKEN=$(tok '#pusher@srv1') \
          AGENT_BUS_NAME='#pusher@srv1' PUSH_NAME='#push.session@srv1' bun run smoke-push.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "claude push smoke" $rc
}
mcp_codex_body() {
    out=$(cd mcp && timeout 120 bun run smoke-codex.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "codex adapter smoke" $rc
}
mcp_launcher_body() {
    out=$(timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
          AGENT_BUS_NAME=$OWNER LAUNCHER_BUILD=$D TEST_MAPPED_SOCKET=$D/user-$(id -un).sock bun run launchers/smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "installed launcher smoke" $rc
}
mcp_rename_body() {
    out=$(timeout 120 env AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN \
          AGENT_BUS_NAME=$OWNER LAUNCHER_BUILD=$D bun run launchers/rename-smoke.ts 2>&1)
    rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "coordinated launcher rename" $rc
}
mcp_rpc() { mcp_face "rpc unit tests" mcp_rpc_body; }
mcp_smoke() { mcp_face "mcp smoke" mcp_smoke_body; }
mcp_push() { mcp_face "claude push" mcp_push_body; }
mcp_codex() { mcp_face "codex adapter" mcp_codex_body; }
mcp_launcher() { mcp_face "installed launcher" mcp_launcher_body; }
mcp_rename() { mcp_face "coordinated launcher rename" mcp_rename_body; }
# agent-bus-setup --samples fills a node with a galaxy far, far away, and
# --remove-samples takes exactly that away again. Its own daemon, so the
# samples meet no other section's names (docs/07-setup.md#sample-data).
# The TCP port answers TLS beside plain HTTP only when a TLS directory is given
# (docs/07-setup.md#tls): its own daemons, a certificate made here.
tls_port() {
  sec "the TCP port answers TLS and plain HTTP, only when TLS is enabled"
  local T=$D/tlsp P1=$((PORT+11)) P2=$((PORT+12)) TPID PPID2 out
  mkdir -p "$T/tls" "$T/plain" "$T/on"
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 2 -subj /CN=localhost \
    -addext "subjectAltName=IP:127.0.0.1,DNS:localhost" -keyout "$T/tls/key.pem" -out "$T/tls/cert.pem" 2>/dev/null
  # Without a TLS directory the port is plain HTTP alone.
  "$D/agent-busd" --addr 127.0.0.1:$P1 --socket "$T/plain/bus.sock" --owner "$OWNER" --db "$T/plain/bus.db" --create --flush-every 0 >"$T/plain/log" 2>&1 &
  PPID2=$!
  ready "$T/plain/bus.sock" || echo "  WARNING: the plain daemon never answered"
  has "without --tls-dir the port answers plain HTTP" "$(curl -sS http://127.0.0.1:$P1/identity)" '"version"'
  out=$(curl -sS --max-time 5 --cacert "$T/tls/cert.pem" https://127.0.0.1:$P1/identity 2>&1); rc=$?
  bad_exit "and refuses a TLS handshake" $rc
  kill $PPID2 2>/dev/null; wait $PPID2 2>/dev/null
  # With it, the same port answers both, HTTP/2 over TLS included.
  "$D/agent-busd" --addr 127.0.0.1:$P2 --socket "$T/on/bus.sock" --owner "$OWNER" --db "$T/on/bus.db" --create --flush-every 0 --tls-dir "$T/tls" >"$T/on/log" 2>&1 &
  TPID=$!
  ready "$T/on/bus.sock" || echo "  WARNING: the TLS daemon never answered"
  has "with --tls-dir the port answers TLS" "$(curl -sS --cacert "$T/tls/cert.pem" https://127.0.0.1:$P2/identity)" '"version"'
  has "negotiating HTTP/2" "$(curl -sS --http2 -o /dev/null -w '%{http_version}' --cacert "$T/tls/cert.pem" https://127.0.0.1:$P2/identity)" '^2$'
  has "and still answers plain HTTP on the same port" "$(curl -sS http://127.0.0.1:$P2/identity)" '"version"'
  local want
  want=$(openssl x509 -in "$T/tls/cert.pem" -noout -fingerprint -sha256 | sed 's/.*=//; s/://g' | tr A-F a-f)
  has "the log names the certificate's fingerprint" "$(cat "$T/on/log")" "fingerprint sha256:$want"
  # The fingerprint to pin comes from the token helper, asked directly or
  # through the forced SSH command, and from admin; no credential is needed.
  has "agent-bus token --fingerprint prints it" "$(AGENT_BUS_TLS_DIR=$T/tls "$D/agent-bus" token --fingerprint)" "^sha256:$want$"
  has "and so does the SSH request token --fingerprint" "$(AGENT_BUS_TLS_DIR=$T/tls SSH_ORIGINAL_COMMAND='token --fingerprint' "$D/agent-bus" token someone@srv1)" "^sha256:$want$"
  has "agent-bus-admin tls describes the certificate" "$(AGENT_BUS_TLS_DIR=$T/tls "$D/agent-bus-admin" tls)" "fingerprint sha256:$want"
  # A client reaches the https:// port with the pin, and with nothing else.
  local TTOK
  TTOK=$(owner_token "$T/on")
  tcli() { env -u AGENT_BUS_NAME AGENT_BUS_ADDR=https://127.0.0.1:$P2 AGENT_BUS_TOKEN=$TTOK "$@"; }
  has "agent-bus over https:// with the pin" "$(tcli AGENT_BUS_TLS_FINGERPRINT=sha256:$want "$D/agent-bus" status 2>&1)" '"you"'
  out=$(tcli AGENT_BUS_TLS_FINGERPRINT=sha256:$(printf '0%.0s' $(seq 64)) "$D/agent-bus" status 2>&1); rc=$?
  bad_exit "a wrong pin is refused" $rc
  has "and says it is not the pinned certificate" "$out" 'not the pinned'
  out=$(tcli "$D/agent-bus" status 2>&1); rc=$?
  bad_exit "and with no pin a self-signed certificate is not trusted" $rc
  kill $TPID 2>/dev/null; wait $TPID 2>/dev/null
  # A TLS directory that does not load refuses the start instead of serving plain HTTP alone.
  out=$(timeout 10 "$D/agent-busd" --addr 127.0.0.1:$P2 --socket "$T/on/bus.sock" --owner "$OWNER" --db "$T/on/bus.db" --create --flush-every 0 --tls-dir "$T/missing" 2>&1); rc=$?
  bad_exit "a TLS directory that does not load refuses the start" $rc
  [ $rc -ne 124 ]; ok_exit "at once, before anything is bound, not in a restart loop" $?
  has "and says which" "$out" "tls-dir $T/missing"
}

samples() {
  sec "setup adds sample data, and takes exactly it away"
  mkdir -p "$D/smp"
  "$D/agent-busd" --addr 127.0.0.1:$((PORT+1)) --socket "$D/smp/bus.sock" --owner "$OWNER" \
    --db "$D/smp/bus.db" --create --flush-every 0 >"$D/smp/log" 2>&1 &
  local SPID=$!
  ready "$D/smp/bus.sock" || echo "  WARNING: $D/smp/bus.sock never answered"
  local S=$D/smp/user-$ACCOUNT.sock STOK
  STOK=$(owner_token "$D/smp")
  sget() { curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" "http://unix$1"; }
  # A real record under a sample's name, with its own description, is left alone.
  curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" \
    -d '{"name":"schwartz@vega","kind":"service","addr":"real:1","protocol":"https","descr":"not a sample"}' http://unix/register >/dev/null
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --samples 2>&1)
  has "the samples are added" "$out" 'sample data added: 9 users, 18 records, 4 groups'
  has "except a real record under a sample's name, which is named and left alone" "$out" 'left alone, since they are not samples: schwartz@vega'
  has "and keeps its own owner" "$(sget '/lookup?name=schwartz%40vega')" "\"owner\":\"$OWNER\""
  has "an agent belongs to its sample user" "$(sget '/lookup?name=%23c3po%40tatooine')" '"owner":"luke@tatooine"'
  has "a service carries its realm and its own protocol" "$(sget '/lookup?name=death-star%40empire')" '"protocol":"superlaser"'
  has "and a group its members" "$(sget /groups)" '"@rebels":\["han@corellia","leia@alderaan","luke@tatooine"\]'
  has "and a little traffic" "$(sget '/lookup?name=jabba-debts%40tatooine')" '"queued":1'
  has "a queue routes on to an agent" "$(sget '/lookup?name=droid-inbox%40tatooine')" '"subs":\["#r2d2@naboo"\]'
  # Its secret is vader's and @empire's to read: the node owner managing it
  # is given no digest of it (issues/done/2026-10/2026-10-17.md#A16).
  has "a service has Maintainers" "$(sget '/lookup?name=death-star%40empire')" '"maintainers":\["@empire"\]'
  lacks "and the node owner is given no digest of its secret" "$(sget '/lookup?name=death-star%40empire')" 'secret_sha'
  has "a queue and a topic are Personal" "$(sget '/lookup?name=falcon-repairs%40corellia')$(sget '/lookup?name=jedi-journal%40dagobah')" '"personal":true.*"personal":true'
  has "and one record is inactive" "$(sget /inactive)" 'death-star-ii@endor'
  again=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --samples 2>&1); rc=$?
  ok_exit "a second run succeeds, the inactive sample included" $rc
  has "and says so" "$again" 'sample data added: 9 users, 18 records, 4 groups'
  has "a second run changes nothing, and sends nothing twice" "$(sget '/lookup?name=jabba-debts%40tatooine')" '"queued":1'
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --remove-samples 2>&1)
  has "removal takes the sample records, draining their inboxes first" "$out" 'sample data removed: 17 records unregistered'
  has "and names what it left alone" "$out" 'left alone.*schwartz@vega'
  has "which is still there as it was" "$(sget '/lookup?name=schwartz%40vega')" '"descr":"not a sample"'
  lacks "no sample record remains" "$(sget /ls)" 'c3po\|death-star\|air-supply'
  lacks "and no sample group" "$(sget /groups)" '@rebels\|@jedi'
  has "while the sample users are deactivated, never removed" "$(sget /users)" '"name":"luke@tatooine"[^}]*"status":"inactive"'
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --samples --dry-run 2>&1); rc=$?
  bad_exit "and the option takes no other" $rc
  # A person's runner (docs/07-setup.md#a-persons-runner), planned against
  # this daemon: a dry run looks and changes nothing, so it needs no root.
  curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" -d '{"name":"rplain","create":true}' http://unix/user >/dev/null
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner install --for rplain --dry-run 2>&1); rc=$?
  if [ -e /var/lib/agent-bus/runners ] || [ -L /var/lib/agent-bus/runners ]; then
    # A checkout can be gated before the one-time dev migration. Setup must
    # refuse the real legacy tree before asking this fixture daemon anything.
    bad_exit "a legacy runner tree needs a manual migration first" $rc
    has "refusal names the tree" "$out" 'legacy runner tree /var/lib/agent-bus/runners'
    lacks "refusal plans no host changes" "$out" 'would '
  else
  ok_exit "a person's runner install is planned" $rc
  has "with an account of its own" "$out" 'would create the system account rplain-ab-runner '
  has "a private directory the account alone reads" "$out" "would make /var/lib/agent-bus/rplain-ab-runner rplain-ab-runner's own, 0700"
  has "the person's own agent on the bus" "$out" "would create #runner/rplain@$(hostname), rplain's agent, allowing @owner"
  has "its token where only the runner reads it" "$out" "would write /var/lib/agent-bus/rplain-ab-runner/runner.env with #runner/rplain@$(hostname)'s token"
  has "and its start" "$out" 'would enable and start agent-bus-runner@rplain.service'
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner install --for nobodyhere --dry-run 2>&1); rc=$?
  bad_exit "a runner for a name that is no bus User is refused" $rc
  has "and says why" "$out" 'nobodyhere is not an active bus User'
  curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" -d '{"name":"rgone","create":true}' http://unix/user >/dev/null
  curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" -d '{"name":"rgone","status":"inactive"}' http://unix/user/state >/dev/null
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner install --for rgone --dry-run 2>&1); rc=$?
  bad_exit "and so is one for an inactive User" $rc
  has "for the same reason" "$out" 'rgone is not an active bus User'
  curl -s --unix-socket "$D/smp/bus.sock" -H "X-Agent-Bus-Token: $STOK" -d '{"name":"rqueue","kind":"queue"}' http://unix/register >/dev/null
  has "a queue of that plain name exists" "$(sget '/lookup?name=rqueue')" '"kind":"queue"'
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner install --for rqueue --dry-run 2>&1); rc=$?
  bad_exit "and a runner for a name that is a record but not a User is refused" $rc
  has "as no User" "$out" 'rqueue is not an active bus User'
  fi
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner frobnicate --for rplain --dry-run 2>&1); rc=$?
  bad_exit "a verb that is not one is refused" $rc
  has "and names the valid runner verbs" "$out" 'the verbs are install'
  out=$(AGENT_BUS_ADDR=$S "$D/agent-bus-setup" --runner install --for rplain --samples --dry-run 2>&1); rc=$?
  bad_exit "and --runner takes no other option" $rc
  has "and names the conflicting option" "$out" '--runner takes --for, --purge, --user-runner and --dry-run, not --samples'
  kill "$SPID" 2>/dev/null; wait "$SPID" 2>/dev/null
}

# The TypeScript web face: its unit tests and its contract tests, which run
# every page through the real handler against a disposable daemon built here.
# Each bun test is one check, so a test that fails is a failed check.
web_ts() {
  sec "TypeScript web face (src/web): unit and contract tests"
  local out rc p f
  out=$(cd "$(dirname "$SELF")/web" && AGENT_BUS_BIN_DIR="$D" AGENT_BUS_TMP="$D" timeout 180 bun test 2>&1); rc=$?
  p=$(sed -n 's/^ *\([0-9][0-9]*\) pass$/\1/p' <<<"$out" | tail -1); f=$(sed -n 's/^ *\([0-9][0-9]*\) fail$/\1/p' <<<"$out" | tail -1)
  p=${p:-0}; f=${f:-0}
  pass=$((pass + p)); fail=$((fail + f))
  grep -A12 '^(fail)\|error:' <<<"$out" | head -60 | sed 's/^/  /'
  if [ "$rc" != 0 ] && [ "$f" = 0 ]; then echo "  FAIL bun test exited $rc with no failed test"; fail=$((fail + 1)); fi
  [ "$p" -gt 40 ] || { echo "  FAIL only $p web tests ran"; fail=$((fail + 1)); }
  echo "  ok   $p web tests"
}
core_10() {
sec "the run stops what it started"
# $DPID is what the exit trap kills, and this file spawns dozens of
# short-lived things beside it. A section that reuses the name leaves the
# daemon running, holding the port, and the *next* run is the one that fails
# — so the last thing asked is whether the pid the trap holds is still ours.
has "the daemon the trap will stop is the one this run started" \
  "$(kill -0 "$DPID" 2>/dev/null && echo yes)" 'yes'

}
# The serial run, in the order it always ran: each part leaves the names and
# the counters the next one reads.
core() { for CURRENT_CHECK in core_{1..10}; do "$CURRENT_CHECK"; done; unset CURRENT_CHECK; }

client_check() {
  if normal; then
    sec "$1 client conformance"
    local out rc
    out=$(CLIENT_BIN="$D" timeout 180 bash ./clients/check.sh "$1" 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "$1 client conformance" "$rc"
  else
    skipped=$((skipped+1))
  fi
}
client_python() { client_check python; }
client_rust() { client_check rust; }
client_php() { client_check php; }
client_typescript() { client_check typescript; }
client_java() { client_check java; }
client_go() { client_check go; }
client_examples() { client_check examples; }
# The Slack agent's own suite (src/agents/slack): a real daemon from this
# build, the actual agent, a fake Slack with a Socket Mode websocket
# (Plans/R1.2/done/slack/TODO.md S.7).
agent_telegram() {
  if normal; then
    sec "telegram agent"
    local out rc
    local pattern=()
    [ -z "${SMOKE_TEST_PATTERN:-}" ] || pattern=(--test-name-pattern "$SMOKE_TEST_PATTERN")
    out=$(SMOKE_TEST_SET="$(level)" SMOKE_BIN="$D" timeout 240 python3 ./smoke-agent-pool.py ./agents/telegram "${pattern[@]}" 2>&1); rc=$?
    echo "$out" | grep -E '^\(fail\)| pass$| fail$|^error:|^FILE_TIMING' | sed 's/^/  /'
    ok_exit "telegram agent tests" "$rc"
  else
    skipped=$((skipped+1))
  fi
}

agent_slack() {
  if normal; then
    sec "slack agent"
    local out rc
    local pattern=()
    [ -z "${SMOKE_TEST_PATTERN:-}" ] || pattern=(--test-name-pattern "$SMOKE_TEST_PATTERN")
    out=$(SMOKE_TEST_SET="$(level)" SMOKE_BIN="$D" timeout 240 python3 ./smoke-agent-pool.py ./agents/slack "${pattern[@]}" 2>&1); rc=$?
    echo "$out" | grep -E '^\(fail\)| pass$| fail$|^error:|^FILE_TIMING' | sed 's/^/  /'
    ok_exit "slack agent tests" "$rc"
    out=$(timeout 60 bun test "${pattern[@]}" ./agents/shared/subscriptions.test.ts 2>&1); rc=$?
    echo "$out" | grep -E '^\(fail\)| pass$| fail$|^error:' | sed 's/^/  /'
    ok_exit "shared subscription engine tests" "$rc"
  else
    skipped=$((skipped+1))
  fi
}

agent_server_info() {
  if normal; then
    sec "server-info and standalone installation"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/server-info -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "server-info fixed sections and argument refusal" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh python3 ./agents/standalone/server-info/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "standalone install, host facts and removal" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_systemd() {
  if normal; then
    sec "systemd roles, declared units and standalone privilege"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/systemd -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "systemd role tiers and exact unit scope" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh sudo -n --preserve-env=AB_TEST_TCP,AB_TEST_OWNER,AB_TEST_OWNER_TOKEN,AB_TEST_DIR,CLIENT_BIN,PYTHONDONTWRITEBYTECODE python3 ./agents/standalone/systemd/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "systemd standalone install and real privilege loss" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_git() {
  if normal; then
    sec "git declared repositories, roles and SSH identity"
    local out rc
    go build -o "$D/autopull-client-fixture" ./clients/fixture || { ok_exit "autopull bus fixture builds" 1; return; }
    out=$(AUTOPULL_BUS_FIXTURE="$D/autopull-client-fixture" SMOKE_TEST_SET="$(level)" PYTHONDONTWRITEBYTECODE=1 AGENT_BUS_TMP="$D" python3 -m unittest discover -s agents/standalone/git -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git operations and refusal boundaries" "$rc"
    out=$(sudo -n env AB_TEST_DIR="$D" PYTHONDONTWRITEBYTECODE=1 python3 ./agents/standalone/git/ssh_check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git configured SSH key and account fallback" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh sudo -n --preserve-env=AB_TEST_TCP,AB_TEST_OWNER,AB_TEST_OWNER_TOKEN,AB_TEST_DIR,CLIENT_BIN,PYTHONDONTWRITEBYTECODE python3 ./agents/standalone/git/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git standalone installation and delivered roles" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_logwatch() {
  if normal; then
    sec "logwatch ring, sources and subscriptions"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/logwatch -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "logwatch bounded history, cleaning, rescans and journals" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh python3 ./agents/standalone/logwatch/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "logwatch standalone serving, ACL and destination subscriptions" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_sessions() {
  if normal; then
    sec "configured session set and detached tmux startup"
    local out rc
    local systemd_check=0
    if slow; then systemd_check=1; fi
    out=$(AB_SESSIONS_SYSTEMD_CHECK=$systemd_check PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/tmux-sessions -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "session argv, configuration, collisions and real sleeping stub" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_ai() {
  if normal; then
    sec "AI proxy pass-through and private provider credential"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/ai -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "AI proxy provider transport, bounds and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_voyage() {
  if normal; then
    sec "Voyage proxy byte preservation and private provider credential"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/voyage -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "Voyage transport, bounds, path and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_db() {
  if normal; then
    sec "Database proxy: read enforced by the database on MySQL, PostgreSQL and ClickHouse"
    local out rc
    # The whole run, under journal-quiet: no fixture container or podman event of it reaches the host journal (J32).
    out=$(PYTHONDONTWRITEBYTECODE=1 ./journal-quiet.sh ab-db-fixture python3 -m unittest discover -s agents/db -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "database proxy roles, read-only enforcement, bounds and password secrecy" "$rc"
    has "the database fixtures wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for ab-db-fixture$"
    # A fixture container whose test process is gone was leaked (J32); a neighbour's live run is not one.
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 agents/db/fixtures.py leaked 2>&1)
    is_empty "no database fixture container outlives its test" "$out"
  else skipped=$((skipped+1)); fi
}

agent_imap() {
  if normal; then
    sec "IMAP bounded private UID cache and raw external publication"
    local out rc
    go build -o "$D/imap-client-fixture" ./clients/fixture || { ok_exit "IMAP bus fixture builds" 1; return; }
    out=$(IMAP_BUS_FIXTURE="$D/imap-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/imap -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "IMAP TLS/auth, bounded literals/cache, raw publication and exported reader" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_eventlog() {
  if normal; then
    sec "eventlog rings, persistent alerts, roles and once-only subscriptions"
    local out rc
    go build -o "$D/eventlog-client-fixture" ./clients/fixture || { ok_exit "eventlog bus fixture builds" 1; return; }
    out=$(EVENTLOG_BUS_FIXTURE="$D/eventlog-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/eventlog -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "eventlog bounds, fold/restart, pull/clean, direct roles, subscription-first and package" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_subscriptions() {
  sec "shared subscription vectors, authority and bounded delivery"
  local out rc
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/shared -p test_subscriptions.py -v 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "subscription parity, revoked creators, drop without pause, bounded helpers" "$rc"
}

# The container image from this tree's release archive, under rootless podman
# (src/container/README.md#acceptance). The archive sits in the shard's own
# directory, which its cleanup removes; the acceptance removes what it made.
container_image() {
  if normal; then
    sec "container image: both roles, delivery, persistence, sandbox refusal"
    local out rc dist="$D/container-dist" base
    # The build never pulls: a host without the pinned base cannot build it,
    # and a selected check that cannot run fails rather than passing unrun.
    base=$(sed -n 's/^FROM //p' container/Containerfile | head -1)
    if [ -z "$base" ] || ! podman image exists "$base" 2>/dev/null; then
      echo "  the base image '$base' is not in the local store, or podman is unavailable; podman pull it once"
      ok_exit "container image prerequisites: podman and the pinned base image" 1; return
    fi
    out=$(timeout 240 ./package.sh "$dist" 2>&1 >/dev/null); rc=$?
    [ "$rc" != 0 ] || { out=$(timeout 240 ./journal-quiet.sh agent-bus-acc- ./container/acceptance.sh "$dist" 2>&1); rc=$?; }
    echo "$out" | sed 's/^/  /'
    ok_exit "container image acceptance from the release archive" "$rc"
    has "the container acceptance wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for agent-bus-acc-$"
  else skipped=$((skipped+1)); fi
}

distro_install() {
  if slow; then
    sec "install from the release archive on AlmaLinux 10, Ubuntu 26.04 and Debian 13: check-deploy, tamper, update (P.4)"
    local out rc
    out=$(timeout 540 ./journal-quiet.sh agent-bus-distro- ./acceptance/distro/run.sh 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "curl | sh installs a node that passes check-deploy, refuses forged releases and updates keeping its configuration" "$rc"
    has "the distro acceptance wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for agent-bus-distro-$"
  else skipped=$((skipped+1)); fi
}

agent_cron() {
  if normal; then
    sec "cron subscriptions and one-shots: destination authority, single firing, RD138 delivery"
    local out rc
    go build -o "$D/cron-client-fixture" ./clients/fixture || { ok_exit "cron bus fixture builds" 1; return; }
    out=$(CRON_BUS_FIXTURE="$D/cron-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/cron -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "cron parser, /manages at add and firing, one sender per slot, restarts and unconfirmed pauses" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_at() {
  if normal; then
    sec "at: deferred one-shot calls on the shared scheduler engine"
    local out rc
    go build -o "$D/at-client-fixture" ./clients/fixture || { ok_exit "at bus fixture builds" 1; return; }
    out=$(CRON_BUS_FIXTURE="$D/at-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/at -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "at grammar and HH:MM, one send per call, restarts, ACL pauses, shared engine packaged unchanged" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_smtp() {
  if normal; then
    sec "SMTP verified TLS, account isolation and one submission"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/smtp -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "SMTP recipients, TLS/AUTH, MIME, bounds and delivery outcomes" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_qdrant() {
  if normal; then
    sec "Qdrant configured URL, application roles and private key"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/qdrant -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "Qdrant tiers, exact API routes, transport bounds and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_rag() {
  if normal; then
    sec "RAG core, prompt cache and disposable bus block identities"
    local out rc
    go build -o "$D/rag-client-fixture" ./clients/fixture || { ok_exit "RAG bus fixture builds" 1; return; }
    out=$(RAG_BUS_FIXTURE="$D/rag-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/rag -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "RAG scoped storage, provider preparation, cache and identity" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_fetch() {
  if normal; then
    sec "Fetch pinned public addresses, verified TLS, bounded workers and readable text"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s agents/fetch -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "fetch SSRF, redirects, TLS, framing, deadlines, extraction and offline package" "$rc"
  else
    fast_check agent_fetch
  fi
}

agent_webhook() {
  if normal; then
    sec "Webhook authenticated routes, bounded HTTP and private configured outbound"
    local out rc
    go build -o "$D/webhook-client-fixture" ./clients/fixture || { ok_exit "webhook bus fixture builds" 1; return; }
    out=$(WEBHOOK_BUS_FIXTURE="$D/webhook-client-fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/webhook -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "webhook auth, exact delivery, concurrency, TLS, deadlines, caps and package" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_sms() {
  if normal; then
    sec "SMS provider at-most-once transport and private key"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/sms-telnyx -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "SMS validation, provider boundary and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

"$SHARD"
sec
if [ -n "${SMOKE_OUT:-}" ]; then
  echo "$pass $fail $skipped" >"$SMOKE_OUT/$SHARD.result"
  sed "s/\$/ [$SHARD]/" "$D/timing" >"$SMOKE_OUT/$SHARD.timing" 2>/dev/null
else
  echo; echo "slowest sections (ms):"; sort -rn "$D/timing" | head -14 | sed 's/^/  /'
  [ "$skipped" -gt 0 ] && echo "SKIPPED $skipped sections in shard $SHARD."
  echo; echo "passed $pass, failed $fail"
fi
[ $fail -eq 0 ]
