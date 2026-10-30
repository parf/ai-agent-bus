#!/bin/bash
# release-verify.sh must fail each way a release can be wrong, on its own
# message: an unstamped binary, one altered version, a missing licence, a
# changed file. Stub programs stand in for the Go binaries; the real archive
# is package.sh's and is checked by the release run.
set -eu
cd "$(dirname "$0")"
here=$PWD
mkdir -p "$here/../tmp"
work=$(mktemp -d "$here/../tmp/rvtest.XXXXXX")
trap 'rm -rf "$work"' EXIT
v=1.2.3; arch=$(uname -m)
src="$work/VERSION"; echo $v >"$src"
export SOURCE_VERSION=$src

mk() { # mk <dir> : a good full archive tree under <dir>/<name>
  local name=agent-bus-$v-linux-$arch root="$1/agent-bus-$v-linux-$arch"
  mkdir -p "$root/internal/version"
  echo $v >"$root/internal/version/VERSION"
  echo licence >"$root/LICENSE.md"
  mkdir -p "$root/licenses"; echo notices >"$root/licenses/NOTICE.md"
  for p in agent-bus agent-bus-runner agent-busd agent-bus-admin agent-bus-setup; do
    printf '#!/bin/sh\nprintf "%s\\nbuild_info: u@h 2026-01-01 00:00:00\\n"\n' $v >"$root/$p"; chmod +x "$root/$p"
  done
  printf '#!/bin/sh\n# %s\n' $v >"$root/agent-bus-ts"; chmod +x "$root/agent-bus-ts"
  (cd "$root" && find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | sort | xargs sha256sum >MANIFEST.sha256)
}
pack() { tar -C "$1" -czf "$1/$name.tar.gz" "$name"; }
name=agent-bus-$v-linux-$arch
# case <label> <message wanted> <mutation of the tree, run in its root>
case_() {
  local label=$1 want=$2 mutate=$3 d="$work/$1"
  mkdir "$d"; mk "$d"
  (cd "$d/$name" && eval "$mutate")
  (cd "$d/$name" && find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | sort | xargs sha256sum >MANIFEST.sha256)
  pack "$d"
  local out rc=0
  out=$("$here/release-verify.sh" "$d/$name.tar.gz" 2>&1) || rc=$?
  [ $rc != 0 ] || { echo "$label: accepted a bad release"; exit 1; }
  echo $v >"$src" # a case may have moved the source version
  grep -qF "$want" <<<"$out" || { echo "$label: failed, but not on '$want': $out"; exit 1; }
}
good="$work/good"; mkdir "$good"; mk "$good"; pack "$good"
out=$("$here/release-verify.sh" "$good/$name.tar.gz") && grep -q ' ok (version 1.2.3' <<<"$out" || { echo "good release refused: $out"; exit 1; }
case_ unstamped 'has no build stamp' 'printf "#!/bin/sh\nprintf \"1.2.3\\nbuild_info: development (unstamped)\\n\"\n" >agent-busd'
case_ altered 'agent-bus-runner reports 9.9.9' 'sed -i s/1.2.3/9.9.9/ agent-bus-runner'
case_ nolicence 'licence LICENSE.md missing' 'rm LICENSE.md'
case_ nonotice 'notices licenses/NOTICE.md missing' 'rm licenses/NOTICE.md'
case_ ts-stale 'agent-bus-ts does not carry version' 'echo "#!/bin/sh" >agent-bus-ts'
case_ renamed 'named 1.2.3, source is 1.2.4' 'echo 1.2.4 >"$SOURCE_VERSION"'
# A file changed after the manifest was written.
d="$work/tamper"; mkdir "$d"; mk "$d"; echo x >>"$d/$name/agent-bus-ts"; pack "$d"
out=$("$here/release-verify.sh" "$d/$name.tar.gz" 2>&1) && { echo "tampered file accepted"; exit 1; }
grep -qF 'MANIFEST.sha256 does not match' <<<"$out" || { echo "tamper: wrong failure: $out"; exit 1; }
echo 'release evidence: version, stamp, licence, notices, manifest each fail on their own message'
