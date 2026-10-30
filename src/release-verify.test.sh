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

# Exercise the command the release workflow actually runs, not a parallel test
# invocation. This workflow intentionally uses an unconditional
# verification step; reject a missing/moved/conditional step or a draft that
# no longer depends on both native builds.
workflow_step=$(python3 - "$here/../.github/workflows/release.yml" <<'WORKFLOW'
from pathlib import Path
import re
import sys
text = Path(sys.argv[1]).read_text()
build, separator, draft = text.partition('\n  draft:\n')
assert separator, 'release workflow has no draft job'
for job, body in [('build', build), ('draft', draft)]:
    assert not re.search(r'^    (if|continue-on-error):', body, re.M), job + ' job must be unconditional and stop on failure'
matrix = re.search(r'^        runner: \[(.+)\]$', build, re.M)
assert matrix and {x.strip() for x in matrix[1].split(',')} == {'ubuntu-24.04', 'ubuntu-24.04-arm'}, 'both native architecture builds are required'
assert not re.search(r'^        (exclude|include):', build, re.M), 'the native architecture matrix must not skip or replace builds'
assert re.search(r'^    needs: build$', draft, re.M), 'draft does not require the verified builds'
steps = re.split(r'(?=^      - )', build, flags=re.M)
found = [i for i, step in enumerate(steps) if re.search(r'^        id: release-evidence$', step, re.M)]
assert len(found) == 1, 'release workflow needs exactly one release-evidence step'
i = found[0]
assert any('run: src/package.sh ' in step for step in steps[:i]), 'verification must follow packaging'
assert any('uses: actions/upload-artifact@' in step for step in steps[i+1:]), 'verification must precede artifact upload'
assert not re.search(r'^        (if|continue-on-error):', steps[i], re.M), 'verification must be unconditional and stop on failure'
shell = re.search(r'^        shell: (.+)$', steps[i], re.M)
shell = shell[1].strip() if shell else ''
assert shell == 'bash', 'verification must pin shell: bash for pipefail'
run = re.search(r'^        run: (.+)$', steps[i], re.M)
assert run, 'verification must have an executable command'
command = run[1]
if command == '|':
    command = '\n'.join(line[10:] for line in steps[i][run.end():].splitlines() if line.startswith('          '))
assert re.search(r'^src/release-verify.sh dist/\*.tar.gz$', command, re.M), 'verification must invoke release-verify.sh directly on every artifact'
assert not re.search(r'(?<!\|)\|(?!\|)', command), 'verification must invoke release-verify.sh directly, without a pipeline'
print(shell)
print(command)
WORKFLOW
)
workflow_shell=${workflow_step%%$'\n'*}
workflow_run=${workflow_step#*$'\n'}
wired_verify() {
  local sandbox="$work/wired" archive
  mkdir -p "$sandbox/src/internal/version" "$sandbox/dist" "$sandbox/bin"
  ln -sf "$SOURCE_VERSION" "$sandbox/src/internal/version/VERSION"
  printf '#!/bin/sh\nprintf "%%s\\n" "$WIRED_ARCH"\n' >"$sandbox/bin/uname"
  chmod +x "$sandbox/bin/uname"
  ln -sf "$here/release-verify.sh" "$sandbox/src/release-verify.sh"
  rm -f "$sandbox/dist/"*.tar.gz
  for archive in "$@"; do ln -s "$archive" "$sandbox/dist/$(basename "$archive")"; done
  # Match GitHub's explicit bash shell; its unpinned Linux default omits pipefail.
  if [ "$workflow_shell" = bash ]; then
    (cd "$sandbox" && WIRED_ARCH="${WIRED_ARCH:-$arch}" PATH="$sandbox/bin:$PATH" bash --noprofile --norc -e -o pipefail -c "$workflow_run")
  else
    (cd "$sandbox" && WIRED_ARCH="${WIRED_ARCH:-$arch}" PATH="$sandbox/bin:$PATH" bash -e -c "$workflow_run")
  fi
}

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
  out=$(wired_verify "$d/$name.tar.gz" "$client" 2>&1) || rc=$?
  [ $rc != 0 ] || { echo "$label: accepted a bad release"; exit 1; }
  echo $v >"$src" # a case may have moved the source version
  grep -qF "$want" <<<"$out" || { echo "$label: failed, but not on '$want': $out"; exit 1; }
  echo "wired $label: refused on $want"
}
good="$work/good"; mkdir "$good"; mk "$good"; pack "$good"
client_name=agent-bus-client-$v-linux-$arch
mkdir "$work/client"
cp -a "$good/$name" "$work/client/$client_name"
rm "$work/client/$client_name/agent-busd" "$work/client/$client_name/agent-bus-admin" "$work/client/$client_name/agent-bus-setup"
(cd "$work/client/$client_name" && find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | sort | xargs sha256sum >MANIFEST.sha256)
client="$work/client/$client_name.tar.gz"
tar -C "$work/client" -czf "$client" "$client_name"
out=$(wired_verify "$good/$name.tar.gz" "$client") && grep -q ' ok (version 1.2.3' <<<"$out" || { echo "good release refused: $out"; exit 1; }
case_ unstamped 'has no build stamp' 'printf "#!/bin/sh\nprintf \"1.2.3\\nbuild_info: development (unstamped)\\n\"\n" >agent-busd'
case_ altered 'agent-bus-runner reports 9.9.9' 'sed -i s/1.2.3/9.9.9/ agent-bus-runner'
case_ nolicence 'licence LICENSE.md missing' 'rm LICENSE.md'
case_ nonotice 'notices licenses/NOTICE.md missing' 'rm licenses/NOTICE.md'
case_ ts-stale 'agent-bus-ts does not carry version' 'echo "#!/bin/sh" >agent-bus-ts'
case_ renamed 'missing expected archive: dist/agent-bus-1.2.4-linux-' 'echo 1.2.4 >"$SOURCE_VERSION"'
# A file changed after the manifest was written.
d="$work/tamper"; mkdir "$d"; mk "$d"; echo x >>"$d/$name/agent-bus-ts"; pack "$d"
out=$(wired_verify "$d/$name.tar.gz" "$client" 2>&1) && { echo "tampered file accepted"; exit 1; }
grep -qF 'MANIFEST.sha256 does not match' <<<"$out" || { echo "tamper: wrong failure: $out"; exit 1; }
echo 'release evidence: version, stamp, licence, notices, manifest each fail on their own message'

# A good full archive must not hide a bad client archive in the same upload.
mkdir "$work/client-bad"
cp -a "$work/client/$client_name" "$work/client-bad/$client_name"
sed -i s/1.2.3/9.9.9/ "$work/client-bad/$client_name/agent-bus-runner"
(cd "$work/client-bad/$client_name" && find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | sort | xargs sha256sum >MANIFEST.sha256)
tar -C "$work/client-bad" -czf "$work/client-bad/$client_name.tar.gz" "$client_name"
out=$(wired_verify "$good/$name.tar.gz" "$work/client-bad/$client_name.tar.gz" 2>&1) && { echo 'wired client version: accepted a bad client archive'; exit 1; }
grep -qF 'agent-bus-runner reports 9.9.9' <<<"$out" || { echo "wired client version: wrong failure: $out"; exit 1; }
echo 'wired client version: refused while the full archive remains valid'

# Simulate both native matrix labels with script fixtures, not ARM execution.
# Every edition must exist, even when the remaining archive is perfectly valid.
for cpu in x86_64 aarch64; do
  mkdir -p "$work/matrix-$cpu"
  for edition in agent-bus agent-bus-client; do
    template="$good/$name"
    [ "$edition" = agent-bus ] || template="$work/client/$client_name"
    tree="$edition-$v-linux-$cpu"
    cp -a "$template" "$work/matrix-$cpu/$tree"
    tar -C "$work/matrix-$cpu" -czf "$work/matrix-$cpu/$tree.tar.gz" "$tree"
  done
  full="$work/matrix-$cpu/agent-bus-$v-linux-$cpu.tar.gz"
  small="$work/matrix-$cpu/agent-bus-client-$v-linux-$cpu.tar.gz"
  WIRED_ARCH=$cpu wired_verify "$full" "$small" >/dev/null || { echo "matrix $cpu: valid pair refused"; exit 1; }
  for edition in agent-bus agent-bus-client; do
    remaining=$full
    [ "$edition" != agent-bus ] || remaining=$small
    out=$(WIRED_ARCH=$cpu wired_verify "$remaining" 2>&1) && { echo "missing $edition/$cpu: accepted an incomplete artifact set"; exit 1; }
    grep -qF "missing expected archive: dist/$edition-$v-linux-$cpu.tar.gz" <<<"$out" || { echo "missing $edition/$cpu: wrong failure: $out"; exit 1; }
    echo "wired missing $edition/$cpu: refused"
  done
done
