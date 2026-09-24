#!/bin/bash
# Fresh-host runtime acceptance. Each disposable real-systemd host sees only
# the release archive, the bundled gate programs, copies of the runtime
# executables and an evidence directory — never the checkout or /rd.
# Usage: fresh-runtime.sh <archive> <new-output-directory> [signed-in-claude-config-dir]
# The first host has no network. Given a claude.ai login, a second host runs
# only the Claude gates with network, for the real model; the login is copied
# into its disposable profile, never into the image. GATES="name ..." runs a
# subset (fresh-runtime-container.sh names them); OFFLINE=0 skips the first host.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
archive=$(realpath "${1:?release archive required}")
out=$(realpath -m "${2:?new evidence directory required}")
login=${3:-}
[ -f "$archive" ] && [ -f "$archive.sha256" ] || { echo "no archive or checksum: $archive" >&2; exit 1; }
[ ! -e "$out" ] || { echo "output directory must be new" >&2; exit 1; }
mkdir -m 700 -p "$out/package" "$out/fixture/acceptance"
cp "$archive" "$archive.sha256" "$out/package/"
cp "$here/fresh-runtime-container.sh" "$out/fixture/run.sh"
# The gate programs, bundled: no source tree follows them onto the host.
bun build --target=bun --outdir "$out/fixture/acceptance" \
  "$here/runtime-installed.ts" "$here/runtime-interactive.ts" "$here/runtime-isolation.ts" \
  "$here/runtime-launch.ts" "$here/runtime-recovery.ts" "$here/claude-channel-live.ts" >"$out/bundle.log"

runtimes=()
for r in codex opencode claude bun; do
  var="${r^^}_EXE"; exe=${!var:-}
  if [ -z "$exe" ]; then
    case $r in opencode) exe=$HOME/.local/lib/opencode/1.18.30/opencode ;; *) exe=$(command -v "$r" || true) ;; esac
  fi
  [ -n "$exe" ] && exe=$(readlink -f "$exe")
  [ -x "$exe" ] || { echo "no $r executable (set $var)" >&2; exit 1; }
  runtimes+=(-v "$exe:/runtimes/$r:ro")
done

image=${FRESH_INSTALL_IMAGE:-localhost/agent-bus-fresh-install:arch-systemd}
podman build --pull=never -t "$image" -f "$here/fresh-install.Containerfile" "$here" >"$out/image-build.log"

names=()
cleanup() {
  for n in "${names[@]}"; do podman stop -t 10 "$n" >/dev/null 2>&1 || true; done
  rm -rf "$out/login"
}
trap cleanup EXIT
host() { # host MODE NETWORK EXTRA...
  local mode=$1 network=$2; shift 2
  local name="agent-bus-runtime-$mode-$RANDOM-$$" state
  names+=("$name")
  mkdir -m 700 -p "$out/evidence-$mode"
  podman run -d --rm --pull=never --privileged --systemd=always --network="$network" --hostname fresh \
    --security-opt label=disable --name "$name" \
    -v "$out/package:/package:ro" -v "$out/fixture:/fixture:ro" -v "$out/evidence-$mode:/evidence:rw" \
    "${runtimes[@]}" "$@" "$image" >/dev/null
  for _ in $(seq 1 100); do
    state=$(podman exec "$name" systemctl is-system-running 2>/dev/null || true)
    case "$state" in running|degraded) break ;; esac
    sleep .1
  done
  case ${state:-} in running|degraded) ;; *) echo "systemd did not start: ${state:-unknown}" >&2; return 1 ;; esac
  podman exec -e GATES="${GATES:-}" "$name" bash /fixture/run.sh "$mode" | tee "$out/evidence-$mode/result.log"
  local code=${PIPESTATUS[0]}
  podman stop -t 10 "$name" >/dev/null 2>&1 || true
  return "$code"
}
status=0
[ "${OFFLINE:-1}" = 0 ] || host offline none || status=1
if [ -n "$login" ]; then
  # Only the login: the credential file and the account fields a profile needs.
  mkdir -m 700 "$out/login"
  install -m 0600 "$login/.credentials.json" "$out/login/.credentials.json"
  bun -e 'const a = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); console.log(JSON.stringify({ oauthAccount: a.oauthAccount, userID: a.userID }))' "$login/.claude.json" >"$out/login/.claude.json"
  chmod 0600 "$out/login/.claude.json"
  host claude pasta -v "$out/login:/login:ro" || status=1
fi
exit "$status"
