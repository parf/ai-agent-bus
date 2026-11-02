#!/bin/bash
# Runs the agent-guide examples on a private test bus (docs/user/write-agent-*.md
# quote the files this runs). Modes:
#   fast    the bash time service through `agent-bus run`, nothing else
#   normal  every example: CLI, packaged for the runner (with Configuration,
#           Secret and the ordinary confinement views), and the Python process
#   slow    normal, plus the CPU limit actually enforced
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
  (cd "$ROOT/src" && go build -o "$BIN/" ./cmd/agent-busd ./cmd/agent-bus ./cmd/agent-bus-runner) || { echo "FAIL the programs did not build"; exit 1; }
  trap 'guide_down; rm -rf "$GB_BUILT"' EXIT
fi
guide_up "$BIN" || exit 1
users asker@localhost || { echo "FAIL could not create the caller"; exit 1; }

NOW() { date -u +%s; }
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
help=$(gb asker@localhost call "$T" --help | jq -r .body)
eq "help is answered by the script, with no business effect" "$help" "time [iso|epoch] - the current UTC time"
unsupported=$(answer "$T" asker@localhost 'rm -rf /')
eq "a body that is not a format is data and is never echoed back" "$unsupported" "unsupported format; ask help"
status_answer=$(gb asker@localhost call "$T" --status | jq -r .body)
eq "an ask the script does not know is refused by name" "$status_answer" "unsupported: status"
cli_stop
finish() { echo; echo "passed $pass, failed $fail"; [ "$fail" -eq 0 ]; exit; }
[ "$MODE" = fast ] && finish

KEY=disposable-stamp-key-1
# The advanced checks, shared by the CLI and the runner: the same service behaves the same.
# advanced_checks LABEL NAME OWNERNAME: Configuration, Secret, state, status, body-as-data.
advanced_checks() {
  local label=$1 name=$2 manager=$3 seen="" before after got stamp want
  gb "$manager" config "$name" '{"format":"epoch"}' >/dev/null || bad "$label: Configuration was set"
  gb "$manager" secret "$name" "TIME_STAMP_KEY=$KEY" >/dev/null || bad "$label: Secret was set"
  help=$(gb asker@localhost call "$name" --help | jq -r .body)
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
  got=$(gb asker@localhost call "$name" --status | jq -r .body); seen+=$got
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
got=$(gb asker@localhost call "$A" --status | jq -r .body)
eq "CLI advanced: progress survives a restart" "$got" "reports: 2; format: epoch"
cli_stop
finish
