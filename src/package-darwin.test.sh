#!/bin/bash
# package.sh for macOS: the client edition only, cross-built here, every
# program a Mach-O binary for the asked CPU, the manifest matching, no daemon
# and no full-node archive. The Linux packaging has release-verify.real.test.sh.
set -eu
cd "$(dirname "$0")"
mkdir -p ../tmp
work=$(mktemp -d ../tmp/pkgdarwin.XXXXXX)
trap 'rm -rf "$work"' EXIT
fail() { echo "FAIL $*" >&2; exit 1; }
version=$(tr -d '[:space:]' <internal/version/VERSION)
# Mach-O 64-bit magic, then the CPU type: arm64 is 0100000c, x86_64 is 01000007 (little endian words).
macho() { # file cpu
  local head; head=$(head -c 8 "$1" | od -An -tx1 | tr -d ' \n')
  case $2 in arm64) [ "$head" = cffaedfe0c000001 ] ;; amd64) [ "$head" = cffaedfe07000001 ] ;; esac
}
for goarch in arm64 amd64; do
  arch=aarch64; [ $goarch = amd64 ] && arch=x86_64
  out=$work/$goarch
  GOOS=darwin GOARCH=$goarch ./package.sh "$out" >/dev/null 2>&1 || fail "package.sh GOOS=darwin GOARCH=$goarch"
  [ "$(ls "$out" | sort | tr '\n' ' ')" = "agent-bus-client-$version-darwin-$arch.tar.gz agent-bus-client-$version-darwin-$arch.tar.gz.sha256 " ] ||
    fail "$goarch: the output is not exactly the client archive and its hash: $(ls "$out" | tr '\n' ' ')"
  (cd "$out" && sha256sum -c --quiet "agent-bus-client-$version-darwin-$arch.tar.gz.sha256") || fail "$goarch: archive hash"
  mkdir "$out/x"; tar -xzf "$out/agent-bus-client-$version-darwin-$arch.tar.gz" -C "$out/x"
  root=$out/x/agent-bus-client-$version-darwin-$arch
  [ -d "$root" ] || fail "$goarch: no top directory"
  (cd "$root" && sha256sum -c --quiet MANIFEST.sha256) || fail "$goarch: manifest"
  for p in agent-bus agent-bus-runner agent-bus-ts; do
    macho "$root/$p" $goarch || fail "$goarch: $p is not a Mach-O binary for $goarch"
  done
  for p in agent-busd agent-bus-setup agent-bus-admin; do
    [ ! -e "$root/$p" ] || fail "$goarch: the client archive holds $p"
  done
  grep -aq "$version" "$root/agent-bus" || fail "$goarch: agent-bus does not carry $version"
  # The stamp is "user@host date time": a build that was never stamped has none
  # ("development (unstamped)" also sits in the binary as help text, so its
  # absence proves nothing).
  grep -aqE '[A-Za-z0-9._-]+@[A-Za-z0-9._-]+ [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' "$root/agent-bus" || fail "$goarch: agent-bus carries no build stamp"
done
echo 'macOS packaging: client archive only, Mach-O for each CPU, manifest, stamped'
