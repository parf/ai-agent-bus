#!/bin/bash
# Usage: run.sh <archive> <new-evidence-dir>   (the caller holds the smoke-gate slow lock for the podman steps)
set -euo pipefail
archive=$(realpath "$1"); out=$(realpath -m "$2"); here=$(cd "$(dirname "$0")" && pwd)
repo=/usr/local/src/ai-agent-bus
mkdir -m 700 -p "$out/package" "$out/fixture" "$out/evidence"
cp "$archive" "$archive.sha256" "$out/package/"; cp "$here/inside.sh" "$out/fixture/run.sh"
image=localhost/agent-bus-m11:arch-systemd
if [ "${BUILD:-1}" = 1 ]; then
  podman build --pull=missing --build-arg BASE=docker.io/library/archlinux:latest -t "$image" -f "$repo/src/acceptance/fresh-install.Containerfile" "$repo/src/acceptance" >"$out/image-build.log" 2>&1
fi
name=agent-bus-m11-$RANDOM-$$
cid=$(podman run -d --rm --pull=never --privileged --systemd=always --network=none --security-opt label=disable --name "$name" \
  -v "$out/package:/package:ro" -v "$out/fixture:/fixture:ro" -v "$out/evidence:/evidence:rw" "$image")
echo "$cid" >"$out/container.id"
for _ in $(seq 1 100); do s=$(podman exec "$cid" systemctl is-system-running 2>/dev/null || true); case $s in running|degraded) break;; esac; sleep .1; done
podman exec "$cid" bash /fixture/run.sh | tee "$out/evidence/result.log" || true
echo "exit ${PIPESTATUS[0]}" >"$out/evidence/exit.txt"
podman stop -t 10 "$cid" >/dev/null 2>&1 || true
