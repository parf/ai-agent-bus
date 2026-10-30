#!/bin/bash
# Release evidence for one archive (docs/07-setup.md#release-evidence): every
# shipped program reports the release's version and, for Go, a build stamp;
# the licence is packed; the manifest still matches. Reads only, never
# installs. Compare with the source: SOURCE_VERSION (default
# internal/version/VERSION beside this script).
#
#   src/release-verify.sh <archive.tar.gz>...
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src=${SOURCE_VERSION:-$here/internal/version/VERSION}
[ $# -gt 0 ] || { echo "usage: release-verify.sh <archive.tar.gz>..." >&2; exit 2; }
want=$(tr -d '[:space:]' < "$src")
mkdir -p "$here/../tmp"
work=$(mktemp -d "$here/../tmp/verify.XXXXXX")
trap 'rm -rf "$work"' EXIT
fail=0
bad() { echo "release-verify: $name: $*" >&2; fail=1; }
for archive in "$@"; do
  name=$(basename "$archive" .tar.gz)
  if ! echo "$name" | grep -Eq '^agent-bus(-client)?-[0-9]+\.[0-9]+\.[0-9]+-linux-(x86_64|aarch64)$'; then
    bad "not a release archive name"; continue
  fi
  edition=full; [[ $name == agent-bus-client-* ]] && edition=client
  arch=${name##*-}
  dir="$work/$name"; mkdir "$dir"
  tar -C "$dir" -xzf "$archive" || { bad "cannot unpack"; continue; }
  root="$dir/$name"
  [ -d "$root" ] || { bad "no top directory $name"; continue; }
  named=${name#agent-bus-}; named=${named#client-}; named=${named%-linux-*}
  [ "$named" = "$want" ] || bad "named $named, source is $want"
  [ "$(tr -d '[:space:]' < "$root/internal/version/VERSION" 2>/dev/null)" = "$want" ] || bad "VERSION file is not $want"
  [ -s "$root/LICENSE.md" ] || bad "licence LICENSE.md missing or empty"
  (cd "$root" && sha256sum -c MANIFEST.sha256 >/dev/null 2>&1) || bad "MANIFEST.sha256 does not match the files"
  progs=(agent-bus agent-bus-runner)
  [ $edition = full ] && progs+=(agent-busd agent-bus-admin agent-bus-setup)
  native=no; [ "$(uname -m)" = "$arch" ] && native=yes
  for p in "${progs[@]}"; do
    f="$root/$p"
    [ -x "$f" ] || { bad "$p missing or not executable"; continue; }
    if [ $native = yes ]; then
      out=$("$f" --version 2>/dev/null) || { bad "$p --version failed"; continue; }
      [ "$(sed -n 1p <<<"$out")" = "$want" ] || bad "$p reports $(sed -n 1p <<<"$out"), source is $want"
      stamp=$(sed -n 2p <<<"$out")
      [[ $stamp == "build_info: "?* && $stamp != *unstamped* ]] || bad "$p has no build stamp ($stamp)"
    else
      # Another CPU: it cannot run here, so read the stamp and version out of it.
      ! grep -aqF 'development (unstamped)' "$f" || bad "$p is an unstamped build"
      grep -aqF "$want" "$f" || bad "$p does not carry version $want"
    fi
  done
  [ -x "$root/agent-bus-ts" ] || bad "agent-bus-ts missing or not executable"
  grep -aqF "$want" "$root/agent-bus-ts" 2>/dev/null || bad "agent-bus-ts does not carry version $want"
  [ $fail = 0 ] && echo "release-verify: $name ok (version $want, $edition, $([ $native = yes ] && echo queried || echo read))"
done
exit $fail
