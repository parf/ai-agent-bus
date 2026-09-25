#!/bin/bash
# K.17 clean-reinstall acceptance (Plans/R0.8/0.7-cutover.md#procedure). The
# disposable host sees a 0.6 release archive, a 0.7 one and this fixture, never
# the checkout, /rd or the live installation.
# Usage: reinstall-install.sh <0.6-archive> <0.7-archive> <new-output-directory>
set -euo pipefail
old=$(realpath "${1:?0.6 release archive required}")
new=$(realpath "${2:?0.7 release archive required}")
out=$(realpath -m "${3:?new evidence directory required}")
for archive in "$old" "$new"; do
  [ -f "$archive" ] || { echo "no archive: $archive" >&2; exit 1; }
  [ -f "$archive.sha256" ] || { echo "no checksum: $archive.sha256" >&2; exit 1; }
done
[ ! -e "$out" ] || { echo "output directory must be new" >&2; exit 1; }
mkdir -m 700 -p "$out/package/old" "$out/package/new" "$out/fixture" "$out/evidence"
cp "$old" "$old.sha256" "$out/package/old/"
cp "$new" "$new.sha256" "$out/package/new/"
cp "$(dirname "$0")/reinstall-install-container.sh" "$out/fixture/run.sh"

image=${FRESH_INSTALL_IMAGE:-localhost/agent-bus-fresh-install:arch-systemd}
podman build --pull=never -t "$image" -f "$(dirname "$0")/fresh-install.Containerfile" "$(dirname "$0")" >"$out/image-build.log"
name="agent-bus-reinstall-$RANDOM-$$"
podman run -d --rm --pull=never --privileged --systemd=always --network=none \
  --security-opt label=disable --name "$name" \
  -v "$out/package:/package:ro" -v "$out/fixture:/fixture:ro" -v "$out/evidence:/evidence:rw" \
  "$image" >/dev/null
cleanup() { podman stop -t 10 "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
for _ in $(seq 1 100); do
  state=$(podman exec "$name" systemctl is-system-running 2>/dev/null || true)
  case "$state" in running|degraded) break ;; esac
  sleep .1
done
case ${state:-} in running|degraded) ;; *) echo "systemd did not start: ${state:-unknown}" >&2; exit 1 ;; esac
podman exec "$name" bash /fixture/run.sh | tee "$out/evidence/result.log"
exit "${PIPESTATUS[0]}"
