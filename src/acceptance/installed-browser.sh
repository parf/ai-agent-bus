#!/bin/bash
# Installed real-browser acceptance of the web face. A disposable real-systemd
# host installs the release archive, adds the sample data, and a real Chromium
# walks the TypeScript face. Unlike fresh-install.sh the container has a
# network: the page loads its pinned CDN assets, which the owner requires.
# Usage: installed-browser.sh <archive> <new-output-directory>
set -euo pipefail
archive=$(realpath "${1:?release archive required}")
checksum="$archive.sha256"
out=$(realpath -m "${2:?new evidence directory required}")
[ -f "$archive" ] || { echo "no archive: $archive" >&2; exit 1; }
[ -f "$checksum" ] || { echo "no checksum: $checksum" >&2; exit 1; }
[ ! -e "$out" ] || { echo "output directory must be new" >&2; exit 1; }
here=$(dirname "$0")
mkdir -m 700 -p "$out/package" "$out/fixture" "$out/evidence"
cp "$archive" "$checksum" "$out/package/"
cp "$here/installed-browser-container.sh" "$out/fixture/run.sh"
cp "$here/installed-browser.py" "$out/fixture/browser.py"

base=${FRESH_INSTALL_IMAGE:-localhost/agent-bus-fresh-install:arch-systemd}
image=${BROWSER_IMAGE:-localhost/agent-bus-installed-browser:arch-systemd}
podman build --pull=never -t "$base" -f "$here/fresh-install.Containerfile" "$here" >"$out/image-build.log"
podman build --pull=never -t "$image" -f "$here/installed-browser.Containerfile" "$here" >>"$out/image-build.log"
name="agent-bus-browser-$RANDOM-$$"
podman run -d --rm --pull=never --privileged --systemd=always \
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
