#!/bin/bash
# guard.sh against a fake podman: a dead run's containers and directory are swept and a live one's kept, a store that
# cannot be listed fails the run, SIGTERM runs the caller's cleanup, and after a SIGKILL the watcher removes the run's
# container and directory, waiting for a start still in flight. No real podman; a stub says the lock is held.
set -u
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/../../tmp"
work=$(mktemp -d "$here/../../tmp/guard-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
fail=0
bad() { echo "FAIL $*"; fail=1; }
state=$work/containers base=$work/base
mkdir -p "$state" "$base" "$work/bin"
# The fake: global options skipped; containers are files named after them holding their label. A start sleeps
# FAKE_DELAY seconds first, as a process whose command line still reads `podman ... run ...`, as during a pull.
cat >"$work/bin/podman" <<'EOF'
#!/bin/bash
while [ $# -gt 0 ]; do case $1 in --root|--runroot|--storage-opt) shift 2 ;; --*) shift ;; *) break ;; esac; done
verb=$1; shift
case $verb in
  ps)
    [ -z "${FAKE_PS_FAIL:-}" ] && [ ! -e "$FAKE_STATE.psfail" ] || { echo 'storage broke' >&2; exit 125; }
    want=; for a in "$@"; do case $a in label=agent-bus-run=*) want=${a#label=agent-bus-run=} ;; esac; done
    for f in "$FAKE_STATE"/*; do
      [ -e "$f" ] || continue; l=$(cat "$f")
      [ -z "$want" ] || [ "$l" = "$want" ] || continue
      case "$*" in *Labels*) echo "$(basename "$f") $l" ;; *) basename "$f" ;; esac
    done ;;
  run)
    name= label=; prev=
    for a in "$@"; do [ "$prev" = --name ] && name=$a; [ "$prev" = --label ] && label=${a#agent-bus-run=}; prev=$a; done
    exec -a podman python3 -c 'import sys, time; time.sleep(float(sys.argv[1])); open(sys.argv[2], "w").write(sys.argv[3])' \
      "${FAKE_DELAY:-0}" "$FAKE_STATE/$name" "$label" run --name "$name" --label "agent-bus-run=$label" ;;
  rm)
    for a in "$@"; do case $a in -*|[0-9]) ;; *) rm -f "$FAKE_STATE/$a"; echo "rm $a $(date +%s.%N)" >>"$FAKE_STATE.log" ;; esac; done ;;
  unshare) "$@" ;;
esac
EOF
chmod +x "$work/bin/podman"
printf '#!/bin/sh\necho "smoke-gate@parf.us slow #ab-gate 14m left"\n' >"$work/bin/agent-bus"   # the lock held (guard-lock.test.sh refuses)
chmod +x "$work/bin/agent-bus"
export PATH=$work/bin:$PATH FAKE_STATE=$state
start_of() { sed 's/.*) //' "/proc/$1/stat" | cut -d' ' -f20; }
wait_gone() { local i; for i in $(seq 1 "${2:-60}"); do [ -e "$1" ] || return 0; sleep .25; done; return 1; }

# 1. The sweep: a dead run's container and directory go, a live run's and a reused pid's are judged by start time.
echo 999999999-5 >"$state/agent-bus-distro-dead"
echo "$$-$(start_of $$)" >"$state/agent-bus-distro-live"
echo "$$-0" >"$state/agent-bus-distro-reused"   # this pid, an earlier start: dead
mkdir -p "$base/run.dead" "$base/run.live" "$base/run.old"
echo 999999999-5 >"$base/run.dead/owner"; echo "$$-$(start_of $$)" >"$base/run.live/owner"   # run.old: no owner, not ours to judge
run=$(mktemp -d "$base/run.XXXXXX")
out=$(bash -c 'podman=(podman --root x); . "$1"; guard "$2"; echo "owner $(cat "$2/owner")"' sh "$here/guard.sh" "$run" 2>&1)
[ ! -e "$state/agent-bus-distro-dead" ] || bad "sweep: a dead run's container was kept"
[ ! -e "$state/agent-bus-distro-reused" ] || bad "sweep: a reused pid's container was taken for a live run"
[ -e "$state/agent-bus-distro-live" ] || bad "sweep: a live run's container was removed"
[ ! -e "$base/run.dead" ] || bad "sweep: a dead run's directory was kept"
[ -e "$base/run.live" ] && [ -e "$base/run.old" ] || bad "sweep: a live or unowned directory was removed"
grep -q '^owner [0-9]*-[0-9]*$' <<<"$out" || bad "guard did not record the run as the directory's owner: $out"
rm -f "$state"/*

run=$(mktemp -d "$base/run.XXXXXX")   # each case its own: a finished case's watcher removes its directory
# 2. A store that cannot be listed fails the run instead of looking empty.
out=$(FAKE_PS_FAIL=1 bash -c 'podman=(podman); . "$1"; guard "$2"; echo went on' sh "$here/guard.sh" "$run" 2>&1); rc=$?
[ "$rc" != 0 ] && ! grep -q 'went on' <<<"$out" && grep -q 'podman ps' <<<"$out" || bad "a failing podman ps did not fail the run: rc $rc $out"

# 3. SIGTERM during a foreground command ends the harness through its EXIT trap at once, not when the command ends.
run=$(mktemp -d "$base/run.XXXXXX")
bash -c 'podman=(podman); . "$1"; trap "touch \"$3\"" EXIT; guard "$2"; echo up; sleep 29.5' sh "$here/guard.sh" "$run" "$work/cleaned" >"$work/term.out" 2>&1 &
child=$!
for _ in $(seq 1 40); do grep -q up "$work/term.out" && break; sleep .1; done
kill -TERM "$child"; t=$SECONDS; wait "$child"; rc=$?
[ "$rc" = 143 ] && [ -e "$work/cleaned" ] && [ $((SECONDS - t)) -lt 5 ] ||
  bad "SIGTERM did not run the harness's cleanup at once: rc $rc after $((SECONDS - t)) s"
pkill -f '^sleep 29\.5$'   # the foreground command the signal did not reach

# 4. SIGKILL during a start in flight: the watcher waits for the start, then removes the container and the directory.
run=$(mktemp -d "$base/run.XXXXXX")
: >"$state.log"
FAKE_DELAY=2 bash -c 'podman=(podman --root x); . "$1"; guard "$2"; "${podman[@]}" run -d --name agent-bus-distro-late --label "agent-bus-run=$guard_run" image & echo up; sleep 30' \
  sh "$here/guard.sh" "$run" >"$work/kill.out" 2>&1 &
child=$!
for _ in $(seq 1 40); do grep -q up "$work/kill.out" && break; sleep .1; done
kill -KILL "$child"; wait "$child" 2>/dev/null
created=no
for _ in $(seq 1 60); do [ -e "$state/agent-bus-distro-late" ] && { created=yes; break; }; grep -q 'rm agent-bus-distro-late' "$state.log" && break; sleep .1; done
wait_gone "$state/agent-bus-distro-late" && wait_gone "$run" || bad "after SIGKILL the run's container or directory survived (created: $created)"
[ "$created" = yes ] || bad "the watcher removed the container before its start finished, so it was never seen to be made"

# 5. A watcher that can never list the store removes nothing and keeps the run directory and its owner for the sweep.
run=$(mktemp -d "$base/run.XXXXXX")
rm -f "$state"/*
bash -c 'podman=(podman --root x); . "$1"; guard "$2"; "${podman[@]}" run -d --name agent-bus-distro-kept --label "agent-bus-run=$guard_run" image; touch "$3.psfail"; echo up; sleep 30' \
  sh "$here/guard.sh" "$run" "$state" >"$work/psfail.out" 2>&1 &
child=$!
for _ in $(seq 1 40); do grep -q up "$work/psfail.out" && break; sleep .1; done
kill -KILL "$child"; wait "$child" 2>/dev/null
for _ in $(seq 1 40); do pgrep -f "agent-bus-run=$(cat "$run/owner" 2>/dev/null)" >/dev/null || pgrep -fa "^sh -c alive" | grep -qF "$run" || break; sleep .25; done
[ -e "$run/owner" ] && [ -e "$state/agent-bus-distro-kept" ] || bad "a watcher that could not list the store removed the run directory or lost the container"
rm -f "$state.psfail"

# 6. The watcher alone, its run already over: a pgrep error (2) is not "none in flight" and a failed podman ps is asked
# again; only after pgrep says none (1) and ps answers does it remove the container, once, and the directory.
mkdir -p "$work/bin6"
: >"$work/calls"
printf '%s\n' '#!/bin/bash' 'echo pgrep >>"$CALLS"' '[ "$(grep -c "^pgrep" "$CALLS")" -ge 2 ] && exit 1 || exit 2' >"$work/bin6/pgrep"
printf '%s\n' '#!/bin/bash' 'case " $* " in' \
  '  *" ps "*) echo ps >>"$CALLS"; [ "$(grep -c "^ps" "$CALLS")" -ge 2 ] && { echo agent-bus-distro-six; exit 0; } || exit 125 ;;' \
  '  *" unshare "*) shift; "$@" ;;' \
  '  *" rm "*) echo "rm $*" >>"$CALLS" ;;' 'esac' >"$work/bin6/podman"
chmod +x "$work/bin6/pgrep" "$work/bin6/podman"
dir6=$(mktemp -d "$base/run.XXXXXX")
watch=$(bash -c '. "$1"; printf %s "$GUARD_WATCH"' sh "$here/guard.sh")
CALLS=$work/calls PATH=$work/bin6:$PATH timeout 30 sh -c "$watch" sh 999999999-5 "$dir6" "" podman --root x
[ "$(grep -c '^pgrep' "$work/calls")" -ge 2 ] && [ "$(grep -c '^ps' "$work/calls")" -ge 2 ] &&
  [ "$(grep -c '^rm .*agent-bus-distro-six' "$work/calls")" = 1 ] && [ ! -e "$dir6" ] ||
  bad "the watcher took a pgrep or ps error for an answer: $(sort "$work/calls" | uniq -c | tr -s ' \n' ' ') dir $([ -e "$dir6" ] && echo kept || echo gone)"
exit $fail
