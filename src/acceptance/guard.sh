# Sourced by the rootless podman harnesses that keep a private store (distro/run.sh, npm/container-check.sh): a run
# leaves no container and no run directory behind, however it ends (J32).
#
#   podman=(podman --root ... --runroot ...); run=$(mktemp -d "$base/run.XXXXXX")
#   . guard.sh; guard "$run" [keep]
#   ... "${podman[@]}" run --label "agent-bus-run=$guard_run" ...
#
# guard refuses the run unless someone holds the smoke-gate lock, sweeps what dead runs left (their labelled
# containers, their run directories), records this run as the directory's owner, and starts a watcher in its own
# session that cleans up after a SIGKILL. It sets no signal trap: bash already runs the caller's EXIT trap on INT,
# TERM and HUP at once, while a trap of its own for them waits for the foreground command (a long `podman exec`) to
# finish first. A run is its pid and start time, so a reused pid is a dead run. A store that cannot be listed fails
# the run: it is not a store with nothing in it. The image cache is never touched. Every podman call, unshare
# included, goes through the caller's command, never a podman found on PATH (J36).

# guard_lock: refuse the run, before any podman call, unless someone holds the smoke-gate lock. A container run belongs
# to a gate or a mutant, which hold it; a run outside one is the stray that leaked (J32). A holders answer that
# cannot be read is not a held lock.
guard_lock() {
  local said
  said=$(agent-bus holders smoke-gate@parf.us 2>&1) ||
    { echo "FAIL refused to start a container: agent-bus holders smoke-gate@parf.us failed: $said" >&2; exit 1; }
  grep -q '^smoke-gate@parf\.us slow ' <<<"$said" ||
    { echo "FAIL refused to start a container: nobody holds the slow lock on smoke-gate@parf.us; a container run belongs to a gate or a mutant (agent-bus lock smoke-gate@parf.us slow --ttl=900s)" >&2; exit 1; }
}

# A failed stat lookup is not proof of absence. Ask the kernel for ESRCH;
# permission failures and any other error remain unknown.
guard_pid_gone() {
  python3 - "$1" <<'PY_GONE'
import os
import sys
try:
    os.kill(int(sys.argv[1]), 0)
except ProcessLookupError:
    sys.exit(0)
except (OSError, ValueError, OverflowError):
    pass
sys.exit(1)
PY_GONE
}

# guard_started <pid>: start time, or empty only when the PID is gone/zombie.
# A failed or incomplete stat read for an existing PID is unknown (status 1).
guard_started() {
  local stat
  if ! stat=$(sed 's/.*) //' "/proc/$1/stat" 2>/dev/null); then
    guard_pid_gone "$1" && return 0
    return 1
  fi
  set -- $stat
  [ "$#" -ge 20 ] || return 1
  case ${20} in ''|*[!0-9]*) return 1 ;; esac
  [ "$1" = Z ] || echo "${20}"
}
# guard_dead needs positive death evidence, never an unreadable live PID.
guard_dead() {
  local start pid=${1%%-*} expected=${1#*-}
  case $pid in ''|*[!0-9]*) return 1 ;; esac
  case $expected in ''|*[!0-9]*) return 1 ;; esac
  start=$(guard_started "$pid") || return 1
  [ "$start" != "$expected" ]
}
guard_run=
if guard_start=$(guard_started $$) && [ -n "$guard_start" ]; then
  guard_run=$$-$guard_start
fi

# The watcher: wait until the run is over, then until no `podman run` of it is in flight (an image pull can take
# minutes, and a container made after a removal would survive it), then remove its containers and, unless kept, its
# directory. A failing pgrep or podman ps is not an empty answer: it is asked again. Arguments: run, directory, keep,
# then the podman command with its store options.
GUARD_WATCH="$(declare -f guard_pid_gone)"'
alive() {
  pid=$1 expected=$2
  if ! stat=$(sed "s/.*) //" "/proc/$pid/stat" 2>/dev/null); then
    # Keep watching on an unknown read; only a gone PID is dead.
    guard_pid_gone "$pid" && return 1
    return 0
  fi
  set -- $stat
  [ "$#" -ge 20 ] || return 0
  case ${20} in ""|*[!0-9]*) return 0 ;; esac
  [ "$1" != Z ] && [ "${20}" = "$expected" ]
}
run=$1 dir=$2 keep=$3; shift 3
while alive "${run%%-*}" "${run#*-}"; do sleep 1; done
until pgrep -f -- "^podman .* run .*--label agent-bus-run=$run( |\$)" >/dev/null; [ $? = 1 ]; do sleep 1; done
listed=
for i in 1 2 3 4 5; do names=$("$@" ps -a --filter "label=agent-bus-run=$run" --format "{{.Names}}") && { listed=1; break; }; sleep 1; done
[ -n "$listed" ] || exit 1   # never listed: the directory and its owner file stay, so the next start sweeps them
for n in $names; do "$@" rm -f -t 1 "$n" >/dev/null 2>&1; done
[ -n "$keep" ] || "$@" unshare rm -rf "$dir"'

guard() { # guard <run dir> [keep]
  local run=$1 keep=${2:-} listing n r d
  guard_lock
  [ -n "$guard_run" ] || { echo "FAIL cannot establish the live guard owner" >&2; exit 1; }
  listing=$("${podman[@]}" ps -a --filter label=agent-bus-run --format '{{.Names}} {{index .Labels "agent-bus-run"}}') ||
    { echo "FAIL podman ps on the private store failed, so what an earlier run left cannot be told" >&2; exit 1; }
  while read -r n r; do
    [ -n "$n" ] && guard_dead "$r" && "${podman[@]}" rm -f -t 1 "$n" >/dev/null 2>&1
  done <<<"$listing"
  for d in "$(dirname "$run")"/run.*; do
    [ -f "$d/owner" ] && guard_dead "$(cat "$d/owner")" && "${podman[@]}" unshare rm -rf "$d"
  done
  [ -n "$keep" ] || echo "$guard_run" >"$run/owner"   # a kept directory is not swept
  setsid sh -c "$GUARD_WATCH" sh "$guard_run" "$run" "$keep" "${podman[@]}" </dev/null >/dev/null 2>&1 &
}
