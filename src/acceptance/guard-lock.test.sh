#!/bin/bash
# guard.sh refuses a run, before any podman call, unless someone holds the smoke-gate lock (J32): a stub agent-bus
# answers holders, a fake podman records every call. No real podman, no lock. The fake is called by its path; a
# podman on PATH is a decoy that fails the test if called (J36).
set -u
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/../../tmp"
work=$(mktemp -d "$here/../../tmp/guard-lock-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
fail=0
bad() { echo "FAIL $*"; fail=1; }
mkdir -p "$work/bin" "$work/base" "$work/fake"
printf '#!/bin/sh\necho "$*" >>"$CALLS"\n' >"$work/fake/podman"
printf '#!/bin/sh\necho "$*" >>"$DECOY"\nexit 125\n' >"$work/bin/podman"
printf '#!/bin/sh\necho "$*" >>"$ASKED"\nprintf "%%s" "$SAY"\nexit "${RC:-0}"\n' >"$work/bin/agent-bus"
chmod +x "$work/fake/podman" "$work/bin/podman" "$work/bin/agent-bus"
export PATH=$work/bin:$PATH CALLS=$work/calls ASKED=$work/asked FAKE=$work/fake/podman DECOY=$work/decoy

# try <holders answer> [holders exit]: run guard as a harness does; print its exit, then what it said.
try() {
  rm -f "$CALLS" "$ASKED"
  local run; run=$(mktemp -d "$work/base/run.XXXXXX")
  SAY=$1 RC=${2:-0} timeout 20 bash -c 'podman=("$FAKE" --root x); . "$1"; guard "$2" keep; echo went-on' sh "$here/guard.sh" "$run" 2>&1
  echo "exit $?"
}
refused() { # refused <case> <holders answer> [holders exit] <expected words>
  local what=$1 say=$2 rc=$3 words=$4 out
  out=$(try "$say" "$rc")
  grep -q '^exit 1$' <<<"$out" || bad "$what: guard did not fail the run: $out"
  grep -q 'went-on' <<<"$out" && bad "$what: the harness went on after guard: $out"
  grep -qF "FAIL refused to start a container: $words" <<<"$out" || bad "$what: no refusal saying why: $out"
  [ ! -e "$CALLS" ] || bad "$what: podman was called before the lock was checked: $(cat "$CALLS")"
  [ "$(cat "$ASKED" 2>/dev/null)" = "holders smoke-gate@parf.us" ] || bad "$what: holders was not asked once about the gate record"
}

# 1. Nobody holds the lock.
refused "free lock" "smoke-gate@parf.us holds no locks" 0 "nobody holds the slow lock on smoke-gate@parf.us"
# 2. Another lock on the record is not the gate's.
refused "other lock" "smoke-gate@parf.us fast #ab-w1a 3m left" 0 "nobody holds the slow lock"
# 3. A holders call that fails is not a held lock, whatever it printed.
refused "holders fails" "smoke-gate@parf.us slow #ab-gate 14m left" 1 "agent-bus holders smoke-gate@parf.us failed"
# 4. The lock held: the run goes on to podman.
out=$(try "smoke-gate@parf.us slow #ab-gate 14m left")
grep -q '^went-on$' <<<"$out" && grep -q '^exit 0$' <<<"$out" || bad "held lock: the run did not go on: $out"
grep -q '^--root x ps ' "$CALLS" 2>/dev/null || bad "held lock: guard never listed the store"
# Its watcher lists the run once the run is over; wait for that, so it never outlives the stub podman.
for i in $(seq 1 40); do grep -q 'label=agent-bus-run=' "$CALLS" 2>/dev/null && break; sleep .25; done
grep -q 'label=agent-bus-run=' "$CALLS" 2>/dev/null || bad "held lock: the watcher never listed the run"
sleep .5
[ ! -s "$DECOY" ] || bad "a podman on PATH was called instead of the fake: $(head -3 "$DECOY" | tr '\n' ';')"

[ "$fail" = 0 ] && echo "PASS guard refuses a container run nobody holds the smoke-gate lock for"
exit "$fail"
