#!/bin/bash
# Full npm setup acceptance in a rootless, disposable systemd container.
# Must be called with four local package tarballs, no registry publication.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
common=$(cd "$root" && cd "$(git rev-parse --git-common-dir)/.." && pwd)
packages=$(realpath "${1:?usage: container-check.sh <tarball directory>}")
base="$common/tmp/npm-acceptance"
mkdir -p "$base"
run=$(mktemp -d "$base/run.XXXXXX")
name="agent-bus-npm-check-$$"
# Nothing goes to the host journal: no podman events, and the container logs to a capped file in the run dir (J32).
podman=(podman --root "$common/tmp/distro-acceptance/storage" --runroot "${XDG_RUNTIME_DIR:?}/agent-bus-distro-acceptance" --storage-opt overlay.mount_program=/usr/bin/fuse-overlayfs --events-backend=none)
# A run killed by any signal, SIGKILL included, leaves no container and no run directory (J32).
. "$here/../acceptance/guard.sh"
guard "$run"
cleanup() {
  "${podman[@]}" rm -f -t 5 "$name" >/dev/null 2>&1 || true
  podman unshare rm -rf "$run"
}
trap cleanup EXIT
"${podman[@]}" build -q --pull=missing -t localhost/agent-bus-npm-check:debian13 -f "$here/Containerfile" "$here" >/dev/null
"${podman[@]}" run -d --rm --pull=never --privileged --systemd=always --network=none \
  --security-opt label=disable --hostname npm-fixture.test --name "$name" --label "agent-bus-run=$guard_run" \
  --log-driver k8s-file --log-opt "path=$run/container.ctr.log" --log-opt max-size=10mb \
  -v "$packages:/packages:ro" -v "$here:/fixture:ro" -v "$run:/evidence:rw" \
  localhost/agent-bus-npm-check:debian13 >/dev/null
for _ in $(seq 1 150); do
  state=$("${podman[@]}" exec "$name" systemctl is-system-running 2>/dev/null || true)
  case $state in running|degraded) break ;; esac
  sleep .1
done
case $state in running|degraded) ;; *) echo 'FAIL fixture systemd did not boot' >&2; exit 1 ;; esac
"${podman[@]}" exec "$name" python3 -B /fixture/container_test.py "${@:2}"
