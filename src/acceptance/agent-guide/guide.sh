#!/bin/bash
# Runs the agent-guide examples on a private test bus (docs/user/write-agent-*.md
# quote the files this runs). Modes:
#   fast    the bash time service through `agent-bus run`, nothing else
#   normal  the CLI and Python-process examples, the packages' static facts and
#           the packaged-help rule through `agent-bus run --package-dir`; no
#           runner, so no systemd scope and nothing in the host journal
#   slow    normal, plus the examples packaged for a private runner (Configuration,
#           Secret, confinement views, live cgroup limits, a kept process that
#           comes back) and the CPU limit actually enforced
# GUIDE_BIN names a directory holding agent-busd, agent-bus and agent-bus-runner
# (smoke passes the build it already made); without it they are built here.
# Prints "  ok" / "  FAIL" lines and "passed N, failed M"; exits 1 on any failure.
set -uo pipefail
MODE=${1:-normal}
case $MODE in fast|normal|slow) ;; *) echo "usage: guide.sh fast|normal|slow" >&2; exit 2 ;; esac
here=$(cd "$(dirname "$0")" && pwd)
source "$here/lib.sh"
trap 'guide_down' EXIT
trap 'exit 143' TERM INT

BIN=${GUIDE_BIN:-}
if [ -z "$BIN" ]; then
  mkdir -p "$ROOT/tmp"
  BIN=$(mktemp -d "$ROOT/tmp/guide-bin.XXXXXX")
  GB_BUILT=$BIN
  programs=(./cmd/agent-busd ./cmd/agent-bus); [ "$MODE" = slow ] && programs+=(./cmd/agent-bus-runner)   # only slow starts a runner
  (cd "$ROOT/src" && go build -o "$BIN/" "${programs[@]}") || { echo "FAIL the programs did not build"; exit 1; }
  trap 'guide_down; rm -rf "$GB_BUILT"' EXIT
fi
JOURNAL_START=$(date '+%Y-%m-%d %H:%M:%S')
guide_up "$BIN" || exit 1
users asker@localhost || { echo "FAIL could not create the caller"; exit 1; }

NOW() { date -u +%s; }
# The worked pages: what they quote is what ran, and the commands they show are run here.
# page_quotes PAGE; page_run PAGE KIND PACKAGE [NAME=value ...]: PACKAGE is copied and is the commands' folder.
page_ingest() { local line; while IFS= read -r line; do case $line in "  ok   "*) ok "${line#  ok   }" ;; "  FAIL "*) bad "${line#  FAIL }" ;; esac; done; }
# page_report LABEL OUTFILE EXITCODE: the checker's lines are counted, and a crash, a non-zero exit or no
# checks at all is a failure of its own, never a silent pass.
page_report() {
  page_ingest < "$2"
  [ "$3" -eq 0 ] || bad "$1: the page checker exited $3" "$(tail -n 3 "$2.err" | tr '\n' ' ')"
  grep -q '^  ok   ' "$2" || bad "$1: the page checker reported no passing check"
}
page_quotes() { local out=$GB_DIR/pq.$RANDOM; python3 -I "$here/pages.py" quotes "$@" >"$out" 2>"$out.err"; page_report "quotes $1" "$out" $?; }
# PAGE_AS is the person the commands run as (default the test bus's owner).
page_run() {
  local page=$1 kind=$2 pkg=$3; shift 3
  local as=${PAGE_AS:-$GB_OWNER}
  local work=$GB_DIR/pg-$kind; rm -rf "$work"; cp -R "$PKG/$pkg" "$work" || { bad "page $kind: the package folder was copied"; return; }
  PAGE_WORK=$work PAGE_ENV="$*" GB_BIN=$BIN GB_ADDR=$GB_ADDR GB_TOKEN=$(gtok "$as") GB_OWNER=$as python3 -I "$here/pages.py" run "$page" "$kind" > "$work.out" 2> "$work.err"
  page_report "run $page $kind" "$work.out" $?
}
iso_epoch() { date -u -d "$1" +%s; }
# What a time answer must be: an ISO or epoch reading taken between two clock reads.
time_window() { # LABEL ANSWER KIND BEFORE AFTER
  local t=$2
  if [ "$3" = iso ]; then
    has "$1 is an ISO UTC time" "$t" '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z'
    t=$(iso_epoch "${t%% *}") || t=0
  else
    has "$1 is an epoch time" "$t" '^[0-9]{10}'
    t=${t%% *}
  fi
  within "$1 was read while the call ran" "$t" "$4" "$(( $5 + 1 ))"
}

# ---- 1. the CLI, simple: a bash time service behind a name --------------------
sec "CLI: the simple time service"
T=#time-simple@localhost
cli_start "$T" "$PKG/time-simple/time-simple.sh" --descr "Tell the current UTC time" || bad "the simple service is reading its inbox"
before=$(NOW); got=$(answer "$T" asker@localhost epoch); after=$(NOW)
time_window "an epoch request" "$got" epoch "$before" "$after"
before=$(NOW); got=$(answer "$T" asker@localhost iso); after=$(NOW)
time_window "an ISO request" "$got" iso "$before" "$after"
help=$(ask "$T" asker@localhost --help)
eq "help is answered by the script, with no business effect" "$help" "time [iso|epoch] - the current UTC time"
unsupported=$(answer "$T" asker@localhost 'rm -rf /')
eq "a body that is not a format is data and is never echoed back" "$unsupported" "unsupported format; ask help"
status_answer=$(ask "$T" asker@localhost --status)
eq "an ask the script does not know is refused by name" "$status_answer" "unsupported: status"
cli_stop
sec "Pages: what the CLI page quotes and shows"
page_quotes docs/user/write-agent-cli.md
page_quotes docs/user/write-agent-process.md
page_quotes docs/user/write-agent-runner.md 0   # it runs the same files; it quotes no region of its own
page_run docs/user/write-agent-cli.md cli-simple time-simple
gb "$GB_OWNER" unregister "$T" >/dev/null 2>&1
finish() { echo; echo "passed $pass, failed $fail"; [ "$fail" -eq 0 ]; exit; }
if [ "$MODE" = fast ]; then
  since_log=$(journalctl -q -t agent-busd -t agent-bus-runner -t agent-bus --since "$JOURNAL_START" -o cat 2>&1)
  eq "the daemon and the CLI wrote nothing to the host journal" "$since_log" ""
  finish
fi

KEY=disposable-stamp-key-1
# The advanced checks, shared by the CLI and the runner: the same service behaves the same.
# advanced_checks LABEL NAME MANAGER [reload]: Configuration, Secret, state, status, body-as-data.
# With "reload" the settings reach a runner-managed agent through the runner's reload.
advanced_checks() {
  local label=$1 name=$2 manager=$3 seen="" before after got stamp want
  gb "$manager" config "$name" '{"format":"epoch"}' >/dev/null || bad "$label: Configuration was set"
  gb "$manager" secret "$name" "TIME_STAMP_KEY=$KEY" >/dev/null || bad "$label: Secret was set"
  if [ "${4:-}" = reload ]; then runner_call reload >/dev/null; for _ in $(seq 1 100); do wait_reader "$name" && break; done; fi
  help=$(ask "$name" asker@localhost --help); seen+=$help
  has "$label: help names the history method" "$help" 'history'
  before=$(NOW); got=$(answer "$name" asker@localhost now); after=$(NOW); seen+=$got
  time_window "$label: now follows the configured format" "${got%% *}" epoch "$before" "$after"
  stamp=${got##*stamp=}
  want=$(printf '%s%s' "$KEY" "${got%% *}" | sha256sum | cut -c1-8)
  eq "$label: the stamp proves the Secret was used" "$stamp" "$want"
  got=$(answer "$name" asker@localhost iso); seen+=$got
  has "$label: an explicit format wins over the configured one" "$got" '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z stamp=[0-9a-f]{8}$'
  got=$(answer "$name" asker@localhost "iso; touch $GB_DIR/pwned"); seen+=$got
  eq "$label: a body that carries a command is data, not a command" "$got" "unsupported format; ask help"
  [ ! -e "$GB_DIR/pwned" ] && ok "$label: nothing the body asked for ran" || bad "$label: the body ran a command"
  got=$(ask "$name" asker@localhost --status); seen+=$got
  eq "$label: status reports the work state and the configuration" "$got" "reports: 2; format: epoch"
  got=$(answer "$name" asker@localhost history); seen+=$got
  eq "$label: history keeps the reports newest first" "$(printf '%s\n' "$got" | wc -l)" 2
  lacks "$label: the Secret never appears in an answer, status or history" "$seen" "$KEY"
}
sec "CLI: the advanced time service"
A=#time-advanced@localhost
cli_start "$A" "$PKG/time-advanced/time-advanced.sh" --descr "Tell the current UTC time and keep the last five reports" --stay-registered || bad "the advanced service is reading its inbox"
advanced_checks "CLI advanced" "$A" "$GB_OWNER"
# State lives in the work folder and Configuration in the record (kept by --stay-registered), so a restart keeps both.
cli_stop
cli_start "$A" "$PKG/time-advanced/time-advanced.sh" --descr "Tell the current UTC time and keep the last five reports" --stay-registered || bad "the restarted service is reading its inbox"
got=$(ask "$A" asker@localhost --status)
eq "CLI advanced: progress survives a restart" "$got" "reports: 2; format: epoch"
cli_stop

# ---- own process: the same service in Python, with the client library ---------
sec "Own process: the Python time service"
P=#time-process@localhost
gb "$GB_OWNER" register "$P" --kind agent --allow '*' >/dev/null || bad "the Owner registered the process's name"
PTOK=$(gtok "$P")
AGENT_BUS_ADDR=$GB_ADDR AGENT_BUS_TOKEN=$PTOK AGENT_BUS_NAME=$P PYTHONPATH=$ROOT/src/clients/python \
  python3 "$PKG/time-process/time_service.py" >"$GB_DIR/process.log" 2>&1 &
PPID_=$!; GB_PIDS+=("$PPID_")
wait_reader "$P" || bad "the Python service is reading its inbox"
help=$(ask "$P" asker@localhost --help)
has "the Python service answers help itself" "$help" 'time \[now\|iso\|epoch\]'
advanced_checks "process" "$P" "$GB_OWNER"
kvreports=$(gb "$GB_OWNER" kv get "$P" reports --int 2>/dev/null || gb "$GB_OWNER" kv get "$P" reports 2>&1)
has "the process keeps its progress in KV" "$kvreports" '(^|[^0-9])2([^0-9]|$)'
has "and holds no lock once a report is recorded" "$(gb "$GB_OWNER" holders "$P")" 'holds no locks'
log=$(gb "$GB_OWNER" call "#bus" --wait 10s "read $P" | jq -r .body)
has "each report wrote an event to the process's own log" "$log" '"counts":\{"alert":0,"notice":2,"warning":0\}'
kill -TERM "$PPID_"
for _ in $(seq 1 60); do kill -0 "$PPID_" 2>/dev/null || break; sleep 0.1; done
kill -0 "$PPID_" 2>/dev/null && bad "SIGTERM stops the Python service within its drain" || ok "SIGTERM stops the Python service within its drain"
wait "$PPID_" 2>/dev/null
lacks "the Python service's own log never holds the Secret" "$(cat "$GB_DIR/process.log")" "$KEY"
gb "$GB_OWNER" unregister "$A" >/dev/null 2>&1; gb "$GB_OWNER" unregister "$P" >/dev/null 2>&1   # free the names for the runner

sec "Pages: the commands the CLI advanced and process pages show"
page_run docs/user/write-agent-cli.md cli-advanced time-advanced
gb "$GB_OWNER" unregister '#time-advanced' >/dev/null 2>&1   # the page registers the name without a realm
mkdir -p "$GB_DIR/pg-process-src"; cp -R "$PKG/time-process/." "$GB_DIR/pg-process-src/"; cp -R "$ROOT/src/clients/python/agent_bus" "$GB_DIR/pg-process-src/agent_bus"
PKG_SAVE=$PKG; PKG=$GB_DIR; page_run docs/user/write-agent-process.md process pg-process-src AGENT_BUS_ADDR=$GB_ADDR; PKG=$PKG_SAVE
gb "$GB_OWNER" unregister '#time-process' >/dev/null 2>&1

# ---- packaging without a runner: the facts a package must hold, checked statically ----
sec "Packages: what the runner will read (no runner started)"
for d in time-simple time-advanced time-process; do
  j=$PKG/$d/agent.json
  jq -e . "$j" >/dev/null 2>&1 && ok "$d: agent.json parses" || bad "$d: agent.json parses"
  cmd=$(jq -r .command "$j"); first=${cmd%% *}
  case $first in ./*) [ -x "$PKG/$d/${first#./}" ] && ok "$d: command is a package-relative executable" || bad "$d: command is a package-relative executable" "$first is not executable" ;; *) bad "$d: command is a package-relative executable" "$cmd" ;; esac
  [ -n "$(jq -r '.description // empty' "$j")" ] && ok "$d: has a description" || bad "$d: has a description"
done
eq "time-advanced hands help to its script" "$(jq -r .help "$PKG/time-advanced/agent.json")" "native"
eq "time-process serves itself" "$(jq -r .serves "$PKG/time-process/agent.json")" "itself"
# The same packages through `agent-bus run --package-dir`: the runner's help rule without a runner.
S=#time-simple@localhost
cli_start "$S" "$PKG/time-simple/time-simple.sh" --descr "Tell the current UTC time" --package-dir "$PKG/time-simple" || bad "the packaged simple service is reading its inbox"
has "packaged simple: help is the package's description" "$(ask "$S" asker@localhost --help)" 'Tell the current UTC time'
cli_stop
gb "$GB_OWNER" unregister "$S" >/dev/null 2>&1
cli_start "$A" "$PKG/time-advanced/time-advanced.sh" --descr "Tell the current UTC time and keep the last five reports" --package-dir "$PKG/time-advanced" || bad "the packaged advanced service is reading its inbox"
has "packaged advanced: native help comes from the script" "$(ask "$A" asker@localhost --help)" 'history - the last five reports'
cli_stop
gb "$GB_OWNER" unregister "$A" >/dev/null 2>&1

# What our programs wrote to the host journal and syslog during the run: nothing.
since_log=$(journalctl -q -t agent-busd -t agent-bus-runner -t agent-bus --since "$JOURNAL_START" -o cat 2>&1)
eq "the daemon and the CLI wrote nothing to the host journal" "$since_log" ""
[ "$MODE" = slow ] || finish

# ---- the runner: the same services, packaged, with private settings and confinement --
sec "Runner: the same services, packaged (slow: the runner needs a systemd scope)"
RHA=$GB_DIR/rh/agent.d
mkdir -p "$RHA" "$GB_DIR/rh/runner/tokens"
for d in time-simple time-advanced time-process; do mkdir -p "$RHA/$d"; cp -R "$PKG/$d/." "$RHA/$d/"; done
cp -R "$ROOT/src/clients/python/agent_bus" "$RHA/time-process/agent_bus"        # the client travels in the package
for p in probe probenet probefree; do
  mkdir -p "$RHA/$p"; cp "$here/probes/probe.sh" "$RHA/$p/"
  net=""; [ "$p" = probenet ] && net=',"needs":{"network":["127.0.0.1"]}'
  echo "{\"command\":\"./probe.sh\",\"form\":\"args\",\"asks\":\"none\",\"description\":\"Show what the sandbox allows\"$net}" >"$RHA/$p/agent.json"
done
echo '{"time-simple":{"enabled":true},"time-advanced":{"enabled":true},"time-process":{"enabled":true,"limits":{"cpu":"50%","memory":"256M","processes":64}},"probe":{"enabled":true,"limits":{"cpu":"50%"}},"probenet":{"enabled":true},"probefree":{"enabled":true}}' >"$GB_DIR/rh/runner/agents.json"
runner_up || { bad "the private runner is serving"; finish; }
# An agent that serves itself is registered by its Owner, who hands its token over once.
gb launcher@localhost register '#time-process@localhost' --kind agent --allow '*' >/dev/null || bad "the Owner registered the packaged process"
gtok '#time-process@localhost' >"$GB_DIR/rh/runner/tokens/%23time-process@localhost"
runner_call reload >/dev/null
for a in time-simple time-advanced time-process probe probenet probefree; do gb launcher@localhost manage "#$a@localhost" --add-allow '*' >/dev/null 2>&1; done
state=$(runner_call status)
for a in time-simple time-advanced probe; do
  has "the runner serves $a" "$state" "\"name\":\"#$a@localhost\",\"enabled\":true,\"state\":\"idle\""
done
for _ in $(seq 1 100); do wait_reader '#time-process@localhost' && break; done
has "the packaged process is the reader of its own inbox" "$(gb asker@localhost ls '#time-process@localhost')" '"readers":1'

S=#time-simple@localhost
before=$(NOW); got=$(answer "$S" asker@localhost epoch); after=$(NOW)
time_window "runner simple: an epoch request" "$got" epoch "$before" "$after"
has "runner simple: help is the package's description" "$(ask "$S" asker@localhost --help)" 'current UTC time'
eq "runner simple: an ask the package leaves to the runner is refused by name" "$(ask "$S" asker@localhost --status)" "unsupported: status"
advanced_checks "runner advanced" '#time-advanced@localhost' launcher@localhost reload
advanced_checks "runner process" '#time-process@localhost' launcher@localhost reload

sec "Runner: confinement the packages run under"
python3 -m http.server 0 --bind 127.0.0.1 >"$GB_DIR/http.log" 2>&1 &
HTTP_PID=$!; GB_PIDS+=("$HTTP_PID")
for _ in $(seq 1 50); do PORT=$(ss -ltnp 2>/dev/null | grep "pid=$HTTP_PID," | sed -n 's/.*127.0.0.1:\([0-9]*\) .*/\1/p' | head -1); [ -n "$PORT" ] && break; sleep 0.1; done
mkdir -p "$GB_DIR/outside"
view=$(answer '#probe@localhost' asker@localhost "view $GB_DIR/outside/x")
has "the package folder is read-only" "$view" 'package: read-only'
has "the work folder is writable" "$view" 'work: writable'
has "a path outside both is not writable" "$view" 'outside: denied'
[ ! -e "$GB_DIR/outside/x" ] && ok "and nothing was created outside" || bad "the sandbox let a file out"
eq "no network by default: a local listener is not reachable" "$(answer '#probe@localhost' asker@localhost "net $PORT")" "network: denied"
eq "the network the package asked for reaches the same listener" "$(answer '#probenet@localhost' asker@localhost "net $PORT")" "network: reached"
# The limits are the live cgroup's, read from the running process: nothing is pushed over them here.
# Only this private runner's own child: its command line names the private package folder and its cgroup sits
# in the runner's own systemd scope. No global lookup: a miss fails the check and kills nothing.
ppid=""; scope=$(sed 's/^0:://' "/proc/$RUNNER_PID/cgroup" 2>/dev/null)
case $scope in *.scope*) scope=${scope%%.scope*}.scope ;; *) scope="" ;; esac   # no scope: nothing can match
for c in $(pgrep -f "$RHA/time-process"); do
  ccg=$(sed 's/^0:://' "/proc/$c/cgroup" 2>/dev/null)
  [ -n "$scope" ] && case $ccg in "$scope"|"$scope"/*) ppid=$c; break ;; esac
done
[ -n "$ppid" ] && ok "the packaged process is found in this runner's own scope" || bad "the packaged process is found in this runner's own scope" "no child of the private runner matched"
cg=$(sed 's/^0:://' "/proc/${ppid:-0}/cgroup" 2>/dev/null)
eq "the configured CPU share is applied to the live cgroup" "$(cat "/sys/fs/cgroup$cg/cpu.max" 2>/dev/null)" "50000 100000"
eq "the configured memory is applied to the live cgroup" "$(cat "/sys/fs/cgroup$cg/memory.max" 2>/dev/null)" "268435456"
eq "the configured process count is applied to the live cgroup" "$(cat "/sys/fs/cgroup$cg/pids.max" 2>/dev/null)" "64"

sec "Runner: a kept process comes back"
[ -n "$ppid" ] && kill -KILL "$ppid" 2>/dev/null
for _ in $(seq 1 100); do got=$(ask '#time-process@localhost' asker@localhost --status 2>/dev/null); case $got in reports:*) break ;; esac; sleep 0.2; done
has "the runner restarts a process that died and it answers again" "$got" 'reports: 2; format: epoch'

if [ "$MODE" = slow ]; then
  # Enforcement, not configuration: the same four seconds of spinning, under a 50% share and under none.
  # The normal run only read the limits above; this is the one that pushes against them.
  sec "Runner: the CPU share is enforced (slow)"
  limited=$(answer '#probe@localhost' asker@localhost burn)
  free=$(answer '#probefree@localhost' asker@localhost burn)
  lt=${limited##*: }; ft=${free##*: }
  within "an unlimited agent spins for most of four seconds (the control)" "${ft:-0}" 350 450
  within "an agent held to 50% gets at most about half of the same four seconds" "${lt:-0}" 100 290

  # The runner page's commands, on the same runner: first take the packages and names the run above used away.
  sec "Pages: the commands the runner page shows (slow)"
  for d in time-simple time-advanced time-process; do rm -rf "${RHA:?}/$d"; gb launcher@localhost unregister "#$d@localhost" >/dev/null 2>&1; done
  rm -f "$GB_DIR/rh/runner/tokens/%23time-process@localhost"
  runner_call reload >/dev/null
  PAGE_AS=launcher@localhost page_run docs/user/write-agent-runner.md runner . AGENTS=$RHA CLIENTS=$ROOT/src/clients HOST=$GB_DIR/rh/runner RUNNER='#runner/launcher@localhost' REALM=localhost
fi
# What our programs wrote to the host journal and syslog during the run: nothing.
# (systemd's own line for the runner's cgroup scope is not ours.)
since_log=$(journalctl -q -t agent-busd -t agent-bus-runner -t agent-bus --since "$JOURNAL_START" -o cat 2>&1)
eq "the daemon, the runner and the CLI wrote nothing to the host journal" "$since_log" ""
finish
