#!/bin/bash
# src/INSTALL.md's checkout install and update, run as printed in a clean systemd container: the commands are read
# out of the document, never copied here, so the document cannot drift from what works.
# Usage: install-md-container.sh [INSTALL.md]   (the document to run; default: this checkout's)
# The container has no network. The checkout it clones is this commit, served to the document's own `git clone` URL
# through git's url.<base>.insteadOf; Go modules and Bun packages come from the host's caches, mounted read-only.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
common=$(cd "$root" && cd "$(git rev-parse --git-common-dir)/.." && pwd)
doc=$(realpath "${1:-$root/src/INSTALL.md}")
goroot=$(go env GOROOT)
gomod=$(go env GOMODCACHE)
bunbin=$(command -v bun)
buncache=${BUN_INSTALL_CACHE_DIR:-$HOME/.bun/install/cache}
base="$common/tmp/install-md-acceptance"
mkdir -p "$base"
run=$(mktemp -d "$base/run.XXXXXX")
name="agent-bus-install-md-$$"
# Nothing goes to the host journal: no podman events, and the container logs to a capped file in the run dir (J32).
podman=(podman --root "$common/tmp/distro-acceptance/storage" --runroot "${XDG_RUNTIME_DIR:?}/agent-bus-distro-acceptance" --storage-opt overlay.mount_program=/usr/bin/fuse-overlayfs --events-backend=none)
. "$here/guard.sh"
guard "$run"
cleanup() {
  "${podman[@]}" rm -f -t 5 "$name" >/dev/null 2>&1 || true
  podman unshare rm -rf "$run"
}
trap cleanup EXIT
git clone -q --bare --no-hardlinks "$root" "$run/source.git"
mkdir "$run/fixture"
cp "$here/install_md_test.py" "$run/fixture/"
cp "$doc" "$run/fixture/INSTALL.md"
"${podman[@]}" build -q --pull=missing -t localhost/agent-bus-install-md:debian13 -f "$here/install-md.Containerfile" "$here" >/dev/null
"${podman[@]}" run -d --rm --pull=never --privileged --systemd=always --network=none \
  --security-opt label=disable --hostname install-md.test --name "$name" --label "agent-bus-run=$guard_run" \
  --log-driver k8s-file --log-opt "path=$run/container.ctr.log" --log-opt max-size=10mb \
  -v "$run/source.git:/source.git:rw" -v "$run/fixture:/fixture:ro" \
  -v "$goroot:/opt/go:ro" -v "$gomod:/gomod:ro" \
  -v "$bunbin:/opt/bun/bun:ro" -v "$buncache:/bun-cache:O" \
  localhost/agent-bus-install-md:debian13 >/dev/null
for _ in $(seq 1 150); do
  state=$("${podman[@]}" exec "$name" systemctl is-system-running 2>/dev/null || true)
  case $state in running|degraded) break ;; esac
  sleep .1
done
case $state in running|degraded) ;; *) echo 'FAIL fixture systemd did not boot' >&2; exit 1 ;; esac
"${podman[@]}" exec "$name" python3 -B /fixture/install_md_test.py "${@:2}"
