#!/bin/bash
# guard.test.sh killed as the smoke driver bounds a shard (timeout -k: TERM to its process group, KILL 3 s later)
# while a guarded run's watcher waits: its EXIT trap removes the fake podman, the watcher in its own session outlives
# the kill, and it must still never reach a podman on PATH (J36). A decoy podman on PATH records any call.
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
kill -TERM -- "-$group" 2>/dev/null; sleep 3; kill -KILL -- "-$group" 2>/dev/null; wait "$group" 2>/dev/null

# The path under test: the fake is gone and a watcher is still running.
ls -d "$outer"/guard-test.* >/dev/null 2>&1 && bad "guard.test.sh's EXIT trap did not remove its fake podman"
pgrep -f -- "$outer/guard-test" >/dev/null || bad "no watcher outlived the kill, so nothing was tested"
for _ in $(seq 1 80); do pgrep -f -- "$outer/guard-test" >/dev/null || break; sleep .25; done
pgrep -f -- "$outer/guard-test" >/dev/null && bad "a watcher still ran 20 s after the kill"
[ ! -s "$outer/decoy" ] || bad "a killed test's watcher reached a podman on PATH: $(head -3 "$outer/decoy" | tr '\n' ';')"

[ "$fail" = 0 ] && echo "PASS a killed guard.test.sh never reaches a podman on PATH"
exit "$fail"
