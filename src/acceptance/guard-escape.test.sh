#!/bin/bash
# guard.test.sh killed as the smoke driver bounds a shard (timeout -k: TERM to its process group, KILL 3 s later)
# while a guarded run's watcher waits: the fake podman is gone, the watcher in its own session outlives the kill, and
# it must still never reach a podman on PATH (J36). A decoy podman on PATH records any call.
# What this pins is the harness's command: with the fake gone, a watcher given a podman by name would find the decoy
# here, one given it by path fails and stops at its first listing. guard.sh's own calls past that listing (unshare)
# are not reached in this scenario; guard.test.sh's live decoy pins them.
set -u
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/../../tmp"
outer=$(mktemp -d "$here/../../tmp/guard-escape.XXXXXX")
# Only processes naming this run's own directory are ever signalled.
trap 'pkill -KILL -f -- "$outer" 2>/dev/null; rm -rf "$outer"' EXIT
fail=0
bad() { echo "FAIL $*"; fail=1; }
mkdir -p "$outer/bin"
printf '#!/bin/sh\necho "$*" >>"%s/decoy"\nexit 125\n' "$outer" >"$outer/bin/podman"
chmod +x "$outer/bin/podman"

GUARD_TEST_TMP=$outer PATH=$outer/bin:$PATH setsid bash "$here/guard.test.sh" >"$outer/test.out" 2>&1 &
group=$!
# Case 4: a run killed with a start in flight; its watcher is waiting.
up=
for _ in $(seq 1 120); do grep -qs '^up$' "$outer"/guard-test.*/kill.out && { up=1; break; }; sleep .1; done
[ -n "$up" ] || bad "guard.test.sh never reached its SIGKILL case: $(tail -3 "$outer/test.out" | tr '\n' ';')"
# What the check stands on is ordered here, never left to timing: the fake is gone and the watcher is known to be
# running BEFORE the kill. Waiting for guard.test.sh's own EXIT trap to remove the fake raced the watcher (a watcher
# that listed first found the fake, finished, and left nothing to see: J62), and looking for the watcher after the
# kill raced its five-second give-up. The watcher belongs to the run, so it can only go on once the run is killed;
# with the fake gone it can only fail its listing, or reach the decoy if it named a podman instead of using the path.
fake=$(ls "$outer"/guard-test.*/fake/podman 2>/dev/null | head -1)
[ -n "$fake" ] || bad "there was no fake podman to remove, so nothing was tested"
rm -f "$fake"
[ ! -e "$fake" ] || bad "the fake podman could not be removed"
watchers=
for _ in $(seq 1 50); do
  watchers=$(pgrep -f -- "^sh -c alive.*$outer/guard-test" | tr '\n' ' ')
  [ -n "$watchers" ] && break
  sleep .1
done
[ -n "$watchers" ] || bad "no watcher was running before the kill, so nothing was tested"
kill -TERM -- "-$group" 2>/dev/null
# The driver's timeout -k follows TERM with KILL; wait for the TERM to be obeyed (the EXIT trap has then run), bounded.
for _ in $(seq 1 100); do kill -0 "$group" 2>/dev/null || break; sleep .1; done
kill -KILL -- "-$group" 2>/dev/null; wait "$group" 2>/dev/null

# The path under test: the fake is gone and the watchers go on without it, each bounded.
for _ in $(seq 1 100); do ls -d "$outer"/guard-test.* >/dev/null 2>&1 || break; sleep .1; done
ls -d "$outer"/guard-test.* >/dev/null 2>&1 && bad "guard.test.sh's EXIT trap did not remove its fake podman"
for _ in $(seq 1 120); do
  alive=
  for w in $watchers; do kill -0 "$w" 2>/dev/null && alive=1; done
  [ -z "$alive" ] && break
  sleep .25
done
[ -z "${alive:-}" ] || bad "a watcher still ran 30 s after the kill"
[ ! -s "$outer/decoy" ] || bad "a killed test's watcher reached a podman on PATH: $(head -3 "$outer/decoy" | tr '\n' ';')"

[ "$fail" = 0 ] && echo "PASS a killed guard.test.sh never reaches a podman on PATH"
exit "$fail"
