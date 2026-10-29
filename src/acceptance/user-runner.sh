#!/bin/bash
# RT31 acceptance: a person's user-account runner on a disposable user in a
# disposable real-systemd container. Nothing touches this host's users.
# Usage: user-runner.sh <archive> <new-output-directory>
set -euo pipefail
archive=$(realpath "${1:?release archive required}")
checksum="$archive.sha256"
out=$(realpath -m "${2:?new evidence directory required}")
[ -f "$archive" ] && [ -f "$checksum" ] || { echo "no archive or checksum: $archive" >&2; exit 1; }
[ ! -e "$out" ] || { echo "output directory must be new" >&2; exit 1; }
mkdir -m 700 -p "$out/package" "$out/fixture" "$out/evidence"
cp "$archive" "$checksum" "$out/package/"
cp "$(dirname "$0")/user-runner-container.sh" "$out/fixture/run.sh"
image=${FRESH_INSTALL_IMAGE:-localhost/agent-bus-fresh-install:arch-systemd}
podman image exists "$image" || podman build --pull=never -t "$image" -f "$(dirname "$0")/fresh-install.Containerfile" "$(dirname "$0")" >"$out/image-build.log"
name="agent-bus-user-runner-$RANDOM-$$"
podman run -d --rm --pull=never --privileged --systemd=always --network=none \
  --security-opt label=disable --hostname urnode --name "$name" \
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
