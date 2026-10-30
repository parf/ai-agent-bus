#!/bin/bash
# Release evidence for one archive (docs/07-setup.md#release-evidence): every
# shipped program reports the release's version and, for Go, a build stamp;
# the licence is packed; the manifest still matches. Reads only, never
# installs. Compare with the source: SOURCE_VERSION (default
# internal/version/VERSION beside this script).
#
#   src/release-verify.sh <archive.tar.gz>...
#   src/release-verify.sh [--arches "x86_64 aarch64"] [--os "linux darwin"] <release-dir>
# A directory holds exactly the expected full/client archives and SHA256SUMS.
# Checksums and manifests are required; release signatures are not used.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src=${SOURCE_VERSION:-$here/internal/version/VERSION}
usage="usage: release-verify.sh <archive.tar.gz>... | [--arches \"x86_64 aarch64\"] [--os \"linux darwin\"] <release-dir>"
arches="x86_64 aarch64" oses=linux
while [ $# -gt 0 ]; do
  case $1 in
    --arches) arches=${2:?$usage}; shift 2 ;;
    --os) oses=${2:?$usage}; shift 2 ;;
    --*) echo "unsupported option $1: $usage" >&2; exit 2 ;;
    *) break ;;
  esac
done
[ $# -gt 0 ] || { echo "$usage" >&2; exit 2; }
want=$(tr -d '[:space:]' < "$src")
mkdir -p "$here/../tmp"
work=$(mktemp -d "$here/../tmp/verify.XXXXXX")
trap 'rm -rf "$work"' EXIT
fail=0
bad() { echo "release-verify: $name: $*" >&2; fail=1; }
if [ "$#" = 1 ] && [ -d "$1" ]; then
  [ $# = 1 ] && [ -d "$1" ] || { echo "$usage" >&2; exit 2; }
  rel=$(cd "$1" && pwd); name=release
  set --
  for o in $oses; do
    for edition in agent-bus agent-bus-client; do
      [ "$o $edition" = "darwin agent-bus" ] && continue # macOS has the client edition only
      for a in $arches; do set -- "$@" "$rel/$edition-$want-$o-$a.tar.gz"; done
    done
  done
  for f in "$@"; do [ -f "$f" ] || bad "$(basename "$f") is missing"; done
  for f in "$rel"/*; do
    case " $* $rel/SHA256SUMS " in *" $f "*) ;; *) bad "$(basename "$f") is not part of this release" ;; esac
  done
  if [ -f "$rel/SHA256SUMS" ]; then
    for f in "$@"; do
      b=$(basename "$f")
      [ -f "$f" ] && { grep -qxF "$(cd "$rel" && sha256sum "$b")" "$rel/SHA256SUMS" || bad "$b does not match SHA256SUMS"; }
    done
    [ "$(wc -l <"$rel/SHA256SUMS")" = $# ] || bad "SHA256SUMS does not list exactly the $# archives"
  else
    bad "SHA256SUMS is missing"
  fi
  [ $fail = 0 ] || exit 1
fi
for archive in "$@"; do
  name=$(basename "$archive" .tar.gz)
  if ! echo "$name" | grep -Eq '^agent-bus(-client)?-[0-9]+\.[0-9]+\.[0-9]+-(linux|darwin)-(x86_64|aarch64)$'; then
    bad "not a release archive name"; continue
  fi
  edition=full; [[ $name == agent-bus-client-* ]] && edition=client
  arch=${name##*-}; rest=${name%-"$arch"}; os=${rest##*-}
  if [ "$os" = darwin ] && [ "$edition" = full ]; then bad "there is no full node for darwin"; continue; fi
  dir="$work/$name"; mkdir "$dir"
  tar -C "$dir" -xzf "$archive" || { bad "cannot unpack"; continue; }
  root="$dir/$name"
  [ -d "$root" ] || { bad "no top directory $name"; continue; }
  named=${name#agent-bus-}; named=${named#client-}; named=${named%-"$os"-"$arch"}
  [ "$named" = "$want" ] || bad "named $named, source is $want"
  [ "$(tr -d '[:space:]' < "$root/internal/version/VERSION" 2>/dev/null)" = "$want" ] || bad "VERSION file is not $want"
  [ -s "$root/LICENSE.md" ] || bad "licence LICENSE.md missing or empty"
  [ -s "$root/licenses/NOTICE.md" ] || bad "notices licenses/NOTICE.md missing or empty"
  (cd "$root" && sha256sum -c MANIFEST.sha256 >/dev/null 2>&1) || bad "MANIFEST.sha256 does not match the files"
  progs=(agent-bus agent-bus-runner)
  [ $edition = full ] && progs+=(agent-busd agent-bus-admin agent-bus-setup)
  # Runs here only when this host is that OS and CPU (a Mac calls arm64 what the archive calls aarch64).
  hostarch=$(uname -m); [ "$hostarch" = arm64 ] && hostarch=aarch64
  native=no; [ "$(uname -s | tr '[:upper:]' '[:lower:]')" = "$os" ] && [ "$hostarch" = "$arch" ] && native=yes
  for p in "${progs[@]}"; do
    f="$root/$p"
    [ -x "$f" ] || { bad "$p missing or not executable"; continue; }
    if [ $native = yes ]; then
      out=$("$f" --version 2>/dev/null) || { bad "$p --version failed"; continue; }
      [ "$(sed -n 1p <<<"$out")" = "$want" ] || bad "$p reports $(sed -n 1p <<<"$out"), source is $want"
      stamp=$(sed -n 2p <<<"$out")
      [[ $stamp == "build_info: "?* && $stamp != *unstamped* ]] || bad "$p has no build stamp ($stamp)"
    else
      # Another OS or CPU: it cannot run here, so read the stamp and version out
      # of it. A stamp is "user@host date time"; "development (unstamped)" also
      # sits in every binary as help text, so its absence proves nothing.
      grep -aqE '[A-Za-z0-9._-]+@[A-Za-z0-9._-]+ [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' "$f" || bad "$p carries no build stamp"
      grep -aqF "$want" "$f" || bad "$p does not carry version $want"
    fi
  done
  # The compiled TypeScript parts are one program, run under its first argument:
  # `mcp --version` and `launcher <runtime> --version` answer the release version
  # alone (the web part has no version query and would start a server). Where it
  # cannot run, its bytes are read as the Go programs' are. Either way the build
  # stamp is in the binary.
  ts="$root/agent-bus-ts"
  if [ ! -x "$ts" ]; then
    bad "agent-bus-ts missing or not executable"
  else
    if [ $native = yes ]; then
      for ask in "mcp" "launcher claude"; do
        # The exit status is the program's own, not a pipe's: a failed query is a failure whatever it printed.
        # shellcheck disable=SC2086
        if out=$("$ts" $ask --version 2>/dev/null); then
          got=$(sed -n 1p <<<"$out")
          [ "$got" = "$want" ] || bad "agent-bus-ts $ask reports ${got:-nothing}, source is $want"
        else
          bad "agent-bus-ts $ask --version failed"
        fi
      done
    else
      grep -aqF "$want" "$ts" 2>/dev/null || bad "agent-bus-ts does not carry version $want"
    fi
    grep -aqE '[A-Za-z0-9._-]+@[A-Za-z0-9._-]+ [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' "$ts" || bad "agent-bus-ts carries no build stamp"
  fi
  # The SSH forced command is a script that runs agent-bus: present and runnable.
  if [ $edition = full ]; then
    [ -x "$root/agent-bus-token" ] || bad "agent-bus-token missing or not executable"
    if [ $native = yes ] && [ -x "$root/agent-bus-token" ]; then
      if out=$("$root/agent-bus-token" --version 2>/dev/null); then
        [ "$(sed -n 1p <<<"$out")" = "$want" ] || bad "agent-bus-token does not report version $want"
      else
        bad "agent-bus-token --version failed"
      fi
    fi
  fi
  [ $fail = 0 ] && echo "release-verify: $name ok (version $want, $edition, $([ $native = yes ] && echo queried || echo read))"
done
exit $fail
