#!/bin/bash
# Actual macOS full/client builds: per-edition programs, Mach-O CPU, manifest,
# stamp and verifier. Linux packaging has release-verify.real.test.sh.
set -eu
cd "$(dirname "$0")"
mkdir -p ../tmp
work=$(mktemp -d ../tmp/pkgdarwin.XXXXXX)
trap 'rm -rf "$work"' EXIT
fail() { echo "FAIL $*" >&2; exit 1; }
version=$(tr -d '[:space:]' <internal/version/VERSION)
# Independent expected Mach-O 64-bit magic and CPU words.
macho() {
  local head; head=$(head -c 8 "$1" | od -An -tx1 | tr -d ' \n')
  case $2 in arm64) [ "$head" = cffaedfe0c000001 ] ;; amd64) [ "$head" = cffaedfe07000001 ] ;; esac
}
for goarch in ${DARWIN_TEST_ARCHES:-arm64 amd64}; do
  case $goarch in arm64) arch=aarch64 ;; amd64) arch=x86_64 ;; *) fail "unsupported fixture CPU $goarch" ;; esac
  out=$work/$goarch
  GOOS=darwin GOARCH=$goarch ./package.sh "$out" >"$work/build.log" 2>&1 || { cat "$work/build.log" >&2; fail "package.sh GOOS=darwin GOARCH=$goarch"; }
  expected=$(printf '%s\n' "agent-bus-$version-darwin-$arch.tar.gz" "agent-bus-$version-darwin-$arch.tar.gz.sha256" "agent-bus-client-$version-darwin-$arch.tar.gz" "agent-bus-client-$version-darwin-$arch.tar.gz.sha256" | sort)
  [ "$(ls "$out" | sort)" = "$expected" ] || fail "$goarch: output must contain both full and client archives and their hashes"
  for edition in full client; do
    stem=agent-bus; [ "$edition" = full ] || stem=agent-bus-client
    name=$stem-$version-darwin-$arch
    archive=$out/$name.tar.gz
    (cd "$out" && sha256sum -c --quiet "$name.tar.gz.sha256") || fail "$goarch $edition: archive hash"
    mkdir "$out/$edition"; tar -xzf "$archive" -C "$out/$edition"
    root=$out/$edition/$name
    [ -d "$root" ] || fail "$goarch $edition: no top directory"
    (cd "$root" && sha256sum -c --quiet MANIFEST.sha256) || fail "$goarch $edition: manifest"
    programs='agent-bus agent-bus-runner agent-bus-ts'
    for p in agent-busd agent-bus-setup agent-bus-admin; do
      if [ "$edition" = full ]; then
        [ -x "$root/$p" ] || fail "$goarch: full archive missing $p"
        programs="$programs $p"
      else
        [ ! -e "$root/$p" ] || fail "$goarch: client archive holds $p"
      fi
    done
    for p in $programs; do
      macho "$root/$p" "$goarch" || fail "$goarch $edition: $p is not a Mach-O binary for $goarch"
      grep -aqF "$version" "$root/$p" || fail "$goarch $edition: $p does not carry $version"
      grep -aqE '[A-Za-z0-9._-]+@[A-Za-z0-9._-]+ [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' "$root/$p" || fail "$goarch $edition: $p carries no build stamp"
    done
    # Darwin setup must select its launchd backend, not Linux's systemd one.
    if [ "$edition" = full ]; then
      grep -aqF '/Library/LaunchDaemons/agent-busd.plist' "$root/agent-bus-setup" || fail "$goarch: full setup lacks the launchd path"
    fi
    if [ "$edition" = full ]; then
      python3 -B - "$archive" "$work" "$goarch" <<'PYTHON'
import importlib.util
from pathlib import Path
import sys
import tempfile
spec = importlib.util.spec_from_file_location('mac2', 'acceptance/mac2-install-check.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
m.platform.machine = lambda: 'arm64' if sys.argv[3] == 'arm64' else 'x86_64'
with tempfile.TemporaryDirectory(dir=sys.argv[2]) as directory:
    m.archive_bundle(Path(sys.argv[1]), Path(directory))
PYTHON
    fi
    evidence=$(./release-verify.sh "$archive" 2>&1) && grep -q "darwin-$arch ok (version $version, $edition, read)" <<<"$evidence" || fail "$goarch $edition: release-verify: $evidence"
  done
done
echo 'macOS packaging: full and client, per-edition programs, Mach-O for each CPU, manifests, stamped'
