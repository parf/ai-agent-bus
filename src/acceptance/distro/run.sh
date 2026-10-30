#!/bin/bash
# Three-distro install acceptance (P.4): a release built here with SHA256SUMS
# is installed by install.sh inside rootless podman on each
# distribution, then must pass src/check-deploy.sh. Checksum and manifest mismatches must be
# refused first. Containers and their data live under the main checkout's
# tmp/distro-acceptance and are removed after each run, a killed one's at the
# latest when the next starts (guard.sh); the image store there is kept as a
# cache (--clean removes it too).
#
#   src/acceptance/distro/run.sh [--dist <dir>] [--next-dist <dir>] [--keep] [--clean] [almalinux10 ubuntu2604 debian13]
#
# --dist takes the archives src/package.sh built for this host's CPU; without
# it they are built. The other CPU needs cgo and its own runner, so a run
# checksums only the host CPU archives it installs.
# The update is to a "next" release: this tree with VERSION one patch up,
# built from a copy (--next-dist takes one built before).
# --keep leaves the run directory for a look; nothing else is kept.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src=$(cd "$here/../.." && pwd)
common=$(cd "$src" && cd "$(git rev-parse --git-common-dir)/.." && pwd)
base=$common/tmp/distro-acceptance

declare -A images=(
  [almalinux10]=docker.io/library/almalinux:10
  [ubuntu2604]=docker.io/library/ubuntu:26.04
  [debian13]=docker.io/library/debian:13
)
dist="" next_dist="" keep="" clean="" distros=()
while [ $# -gt 0 ]; do
  case $1 in
    --dist) dist=$(realpath "${2:?--dist needs a directory}"); shift ;;
    --next-dist) next_dist=$(realpath "${2:?--next-dist needs a directory}"); shift ;;
    --keep) keep=1 ;;
    --clean) clean=1 ;;
    -*) echo "unknown option $1" >&2; exit 2 ;;
    *) [ -n "${images[$1]:-}" ] || { echo "no distribution $1 (${!images[*]})" >&2; exit 2; }; distros+=("$1") ;;
  esac
  shift
done
[ ${#distros[@]} -gt 0 ] || distros=(almalinux10 ubuntu2604 debian13)
# The containers run this host's CPU, so its archives are the ones installed.
case $(uname -m) in
  x86_64) arch=x86_64 ;;
  aarch64|arm64) arch=aarch64 ;;
  *) echo "no release for CPU $(uname -m)" >&2; exit 1 ;;
esac

# Its own store under tmp/; fuse-overlayfs, as a kernel overlay mount there is refused rootless. Nothing goes to the
# host journal: no podman events, and each container logs to a capped file in the run directory (J32).
podman=(podman --root "$base/storage" --runroot "${XDG_RUNTIME_DIR:?}/agent-bus-distro-acceptance"
  --storage-opt overlay.mount_program=/usr/bin/fuse-overlayfs --events-backend=none)
mkdir -p "$base"
run=$(mktemp -d "$base/run.XXXXXX")
# A run killed by any signal, SIGKILL included, leaves no container and no run directory: guard.sh (J32).
. "$here/../guard.sh"
guard "$run" "$keep"
names=()
cleanup() {
  for n in "${names[@]}"; do "${podman[@]}" rm -f -t 5 "$n" >/dev/null 2>&1 || true; done
  # alice's files are owned by a subordinate uid: remove them inside the user namespace.
  [ -n "$keep" ] || podman unshare rm -rf "$run"
  if [ -n "$clean" ]; then
    "${podman[@]}" system reset -f >/dev/null 2>&1 || true
    podman unshare rm -rf "$base"
  fi
}
trap cleanup EXIT

version=$(tr -d '[:space:]' <"$src/internal/version/VERSION")
if [ -z "$dist" ]; then
  dist=$run/dist
  "$src/package.sh" "$dist" >"$run/package.log" 2>&1 || { cat "$run/package.log" >&2; exit 1; }
fi
next=$(echo "$version" | awk -F. '{print $1 "." $2 "." $3 + 1}')
if [ -z "$next_dist" ]; then
  next_dist=$run/next-dist
  mkdir "$run/next"
  # A worktree's node_modules links are not ignored as directories are: left out, linked below.
  (cd "$src/.." && git ls-files -co --exclude-standard src LICENSE.md licenses |
    grep -v '/node_modules$' | tar -cf - -T -) | tar -xf - -C "$run/next"
  for m in mcp web clients/typescript; do ln -s "$(realpath "$src/$m/node_modules")" "$run/next/src/$m/node_modules"; done
  echo "$next" >"$run/next/src/internal/version/VERSION"
  "$run/next/src/package.sh" "$next_dist" >"$run/package-next.log" 2>&1 || { cat "$run/package-next.log" >&2; exit 1; }
fi

# The unsigned release and its published checksums.
good=$run/release/good/latest/download
mkdir -p "$good"
for edition in agent-bus agent-bus-client; do
  cp "$dist/$edition-$version-linux-$arch.tar.gz" "$good/"
done
(cd "$good" && sha256sum ./*.tar.gz | sed 's|  \./|  |' >SHA256SUMS)

full=agent-bus-$version-linux-$arch.tar.gz
# Change a payload but leave its manifest stale. The tampered variant keeps
# the old outer checksum; forged updates that checksum to reach the manifest check.
mkdir "$run/forge"
tar -xzf "$good/$full" -C "$run/forge"
(
  cd "$run/forge/${full%.tar.gz}"
  printf 'tampered\n' >>INSTALL.md
)
for variant in tampered forged; do
  cp -a "$run/release/good" "$run/release/$variant"
  tar -C "$run/forge" -czf "$run/release/$variant/latest/download/$full" "${full%.tar.gz}"
done
forged=$run/release/forged/latest/download
sed -i "s|^[0-9a-f]\{64\}  $full\$|$(sha256sum "$forged/$full" | cut -d' ' -f1)  $full|" "$forged/SHA256SUMS"
grep -qF "$(sha256sum "$forged/$full" | cut -d' ' -f1)" "$forged/SHA256SUMS"

# The update: next is the latest, and this version stays downloadable by name.
nextdl=$run/release/next/latest/download
mkdir -p "$nextdl" "$run/release/next/download"
for edition in agent-bus agent-bus-client; do
  cp "$next_dist/$edition-$next-linux-$arch.tar.gz" "$nextdl/"
done
(cd "$nextdl" && sha256sum ./*.tar.gz | sed 's|  \./|  |' >SHA256SUMS)
cp -a "$nextdl" "$run/release/next/download/v$next"
cp -a "$good" "$run/release/next/download/v$version"

# Each variant's install.sh: only the release URL changes.
for variant in good tampered forged next; do
  sed -e "s|^RELEASES=\"https://github.com/parf/ai-agent-bus/releases\"\$|RELEASES=\"https://releases.test/$variant\"|" "$src/../install.sh" >"$run/release/$variant/install.sh"
  [ "$(grep -cFx "RELEASES=\"https://releases.test/$variant\"" "$run/release/$variant/install.sh")" = 1 ] ||
    { echo "install.sh no longer has the RELEASES line this harness fills" >&2; exit 1; }
done

mkdir -p "$run/fixture"
cp "$here/in-container.sh" "$here/https.py" "$here/mcp-call.py" "$src/check-deploy.sh" "$src/internal/version/VERSION" "$run/fixture/"
echo "$next" >"$run/fixture/NEXT"
openssl req -x509 -nodes -newkey ec -pkeyopt ec_paramgen_curve:P-256 -days 2 -subj /CN=releases.test \
  -addext subjectAltName=DNS:releases.test -keyout "$run/fixture/tls.key" -out "$run/fixture/tls.crt" 2>/dev/null
chmod 644 "$run/fixture/tls.key"

for d in "${distros[@]}"; do
  "${podman[@]}" build -q --pull=missing --build-arg "BASE=${images[$d]}" -t "localhost/agent-bus-distro:$d" \
    -f "$here/Containerfile" "$here" >/dev/null
done

start() { # distro
  local d=$1 name="agent-bus-distro-$1-$$" state=""
  mkdir -p "$run/evidence-$d"
  "${podman[@]}" run -d --rm --pull=never --privileged --systemd=always --network=none \
    --security-opt label=disable --hostname "node-$d.test" --name "$name" --label "agent-bus-run=$guard_run" \
    --log-driver k8s-file --log-opt "path=$run/$d.ctr.log" --log-opt max-size=10mb \
    -v "$run/release:/release:ro" -v "$run/fixture:/fixture:ro" -v "$run/evidence-$d:/evidence:rw" \
    "localhost/agent-bus-distro:$d" >/dev/null
  for _ in $(seq 1 200); do
    state=$("${podman[@]}" exec "$name" systemctl is-system-running 2>/dev/null || true)
    case $state in running|degraded) break ;; esac
    sleep .1
  done
  case $state in running|degraded) ;; *) echo "FAIL $d: systemd did not start: ${state:-unknown}"; return 1 ;; esac
  "${podman[@]}" exec "$name" bash /fixture/in-container.sh 2>&1 | sed "s/^/$d: /"
  return "${PIPESTATUS[0]}"
}
pids=() status=0
for d in "${distros[@]}"; do
  names+=("agent-bus-distro-$d-$$")
  start "$d" >"$run/$d.log" 2>&1 &
  pids+=($!)
done
for i in "${!distros[@]}"; do
  d=${distros[$i]}
  if wait "${pids[$i]}"; then r=PASS; else r=FAIL; status=1; fi
  cat "$run/$d.log"
  echo "$r $d"
done
[ -z "$keep" ] || echo "kept: $run"
exit "$status"
