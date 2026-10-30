#!/bin/bash
# Three-distro install acceptance (P.4): a release built and signed here with
# a throwaway key is installed by install.sh inside rootless podman on each
# distribution, then must pass src/check-deploy.sh. Forged releases must be
# refused first. Containers and their data live under the main checkout's
# tmp/distro-acceptance and are removed after each run; the image store there
# is kept as a cache (--clean removes it too).
#
#   src/acceptance/distro/run.sh [--dist <dir>] [--keep] [--clean] [almalinux10 ubuntu2604 debian13]
#
# --dist takes the archives src/package.sh built for this host's CPU; without
# it they are built. The other CPU needs cgo and its own runner, so a run signs
# only what it installs, with the same ssh-keygen step release-sign.sh ends with.
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
dist="" keep="" clean="" distros=()
while [ $# -gt 0 ]; do
  case $1 in
    --dist) dist=$(realpath "${2:?--dist needs a directory}"); shift ;;
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

# Its own store under tmp/; fuse-overlayfs, as a kernel overlay mount there is refused rootless.
podman=(podman --root "$base/storage" --runroot "${XDG_RUNTIME_DIR:?}/agent-bus-distro-acceptance"
  --storage-opt overlay.mount_program=/usr/bin/fuse-overlayfs)
mkdir -p "$base"
run=$(mktemp -d "$base/run.XXXXXX")
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
  "$src/package.sh" "$dist" >/dev/null
fi

# The release, signed as the owner would, but with a key made for this run.
good=$run/release/good/latest/download
mkdir -p "$good"
for edition in agent-bus agent-bus-client; do
  cp "$dist/$edition-$version-linux-$arch.tar.gz" "$good/"
done
(cd "$good" && sha256sum ./*.tar.gz | sed 's|  \./|  |' >SHA256SUMS)
ssh-keygen -q -t ed25519 -N '' -C agent-bus-release-acceptance -f "$run/release-key"
ssh-keygen -Y sign -q -f "$run/release-key" -n agent-bus-release "$good/SHA256SUMS"
signer="agent-bus-release $(cut -d' ' -f1-2 "$run/release-key.pub")"

full=agent-bus-$version-linux-$arch.tar.gz
for variant in tampered forged; do
  cp -a "$run/release/good" "$run/release/$variant"
  # Still a valid gzip stream, so only the hash can tell.
  printf 'tampered\n' >>"$run/release/$variant/latest/download/$full"
done
forged=$run/release/forged/latest/download
sed -i "s|^[0-9a-f]\{64\}  $full\$|$(sha256sum "$forged/$full" | cut -d' ' -f1)  $full|" "$forged/SHA256SUMS"
grep -qF "$(sha256sum "$forged/$full" | cut -d' ' -f1)" "$forged/SHA256SUMS"

# Each variant's install.sh: the shipped one, its release URL and key block filled in.
for variant in good tampered forged; do
  sed -e "s|^RELEASES=\"https://github.com/parf/ai-agent-bus/releases\"\$|RELEASES=\"https://releases.test/$variant\"|" \
      -e "s|^RELEASE_KEYS=\"\"\$|RELEASE_KEYS=\"$signer\"|" "$src/../install.sh" >"$run/release/$variant/install.sh"
  [ "$(grep -c "^RELEASES=\"https://releases.test/$variant\"\$\|^RELEASE_KEYS=\"$signer\"\$" "$run/release/$variant/install.sh")" = 2 ] ||
    { echo "install.sh no longer has the RELEASES and RELEASE_KEYS lines this harness fills" >&2; exit 1; }
done

mkdir -p "$run/fixture"
cp "$here/in-container.sh" "$here/https.py" "$src/check-deploy.sh" "$src/internal/version/VERSION" "$run/fixture/"
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
    --security-opt label=disable --hostname "node-$d.test" --name "$name" \
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
