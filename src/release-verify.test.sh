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
# verification step; reject a missing/moved/conditional step or a release that
# no longer depends on both native builds.
workflow_step=$(python3 - "$here/../.github/workflows/release.yml" <<'WORKFLOW'
from pathlib import Path
import re
import sys
text = Path(sys.argv[1]).read_text()
assert not re.search(r'ssh-keygen|release-sign|SHA256SUMS\.sig|owner[^\n]*(?:sign|key)|--draft', text, re.I), 'release workflow must have no signing, owner key or draft wait'
build, separator, release = text.partition('\n  release:\n')
assert separator, 'release workflow has no release job'
for job, body in [('build', build), ('release', release)]:
    assert not re.search(r'^    (if|continue-on-error):', body, re.M), job + ' job must be unconditional and stop on failure'
matrix = re.search(r'^        runner: \[(.+)\]$', build, re.M)
assert matrix and {x.strip() for x in matrix[1].split(',')} == {'ubuntu-24.04', 'ubuntu-24.04-arm'}, 'both native architecture builds are required'
assert not re.search(r'^        (exclude|include):', build, re.M), 'the native architecture matrix must not skip or replace builds'
assert re.search(r'^    needs: build$', release, re.M), 'release does not require the verified builds'
assert 'src/release-verify.sh dist' in release, 'publication must verify the complete checksum set'
assert 'dist/*.tar.gz dist/SHA256SUMS' in release, 'publication must include archives and SHA256SUMS'
assert release.index('src/release-verify.sh dist') < release.index('gh release create'), 'publication precedes verification'
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
  # -s is the OS (Linux, so the fixtures take the native read), anything else the CPU.
  printf '#!/bin/sh\ncase "$1" in -s) echo Linux ;; *) printf "%%s\\n" "$WIRED_ARCH" ;; esac\n' >"$sandbox/bin/uname"
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
  # The compiled TypeScript program answers `mcp --version` and `launcher <runtime> --version` with the version alone,
  # and carries its build stamp in its bytes; the stub does both.
  printf '#!/bin/sh\n# u@h 2026-01-01 00:00:00\nprintf "%s\\n"\n' $v >"$root/agent-bus-ts"; chmod +x "$root/agent-bus-ts"
  printf '#!/bin/sh\nexec "$(dirname "$0")/agent-bus" token "$@"\n' >"$root/agent-bus-token"; chmod +x "$root/agent-bus-token"
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
case_ ts-stale 'agent-bus-ts mcp reports nothing, source is 1.2.3' 'echo "#!/bin/sh" >agent-bus-ts'
# The old stub, a comment holding the version: the file greps clean and answers nothing.
case_ ts-comment-only 'agent-bus-ts mcp reports nothing, source is 1.2.3' 'printf "#!/bin/sh\n# 1.2.3 u@h 2026-01-01 00:00:00\n" >agent-bus-ts'
case_ ts-wrong-answer 'agent-bus-ts mcp reports 9.9.9, source is 1.2.3' 'printf "#!/bin/sh\n# 1.2.3 u@h 2026-01-01 00:00:00\nprintf \"9.9.9\\n\"\n" >agent-bus-ts'
case_ ts-unstamped 'agent-bus-ts carries no build stamp' 'printf "#!/bin/sh\nprintf \"1.2.3\\n\"\n" >agent-bus-ts'
case_ ts-launcher-wrong 'agent-bus-ts launcher claude reports 9.9.9, source is 1.2.3' 'printf "#!/bin/sh\n# u@h 2026-01-01 00:00:00\ncase \"$1\" in mcp) echo 1.2.3;; *) echo 9.9.9;; esac\n" >agent-bus-ts'
# Right stdout, failing exit: the answer of a program that crashed after printing.
case_ ts-failed 'agent-bus-ts mcp --version failed' 'printf "#!/bin/sh\n# u@h 2026-01-01 00:00:00\nprintf \"1.2.3\\n\"\nexit 1\n" >agent-bus-ts'
case_ token-failed 'agent-bus-token --version failed' 'printf "#!/bin/sh\nprintf \"1.2.3\\n\"\nexit 1\n" >agent-bus-token'
case_ no-token 'agent-bus-token missing or not executable' 'rm agent-bus-token'
case_ token-stale 'agent-bus-token does not report version 1.2.3' 'printf "#!/bin/sh\necho 9.9.9\n" >agent-bus-token'
case_ renamed 'missing expected archive: dist/agent-bus-1.2.4-linux-' 'echo 1.2.4 >"$SOURCE_VERSION"'
# A macOS archive cannot run here even for this host's own CPU, so its binaries
# are read: the version and a build stamp must be in them.
mkdarwin() { # dir edition stamp-text
  local n=$2-$v-darwin-$arch root="$1/$2-$v-darwin-$arch"
  mkdir -p "$root/internal/version"; echo $v >"$root/internal/version/VERSION"; echo licence >"$root/LICENSE.md"; mkdir -p "$root/licenses"; echo notice >"$root/licenses/NOTICE.md"
  for p in agent-bus agent-bus-runner agent-bus-ts; do printf '\000%s %s\000' $v "$3" >"$root/$p"; chmod +x "$root/$p"; done
  (cd "$root" && find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | sort | xargs sha256sum >MANIFEST.sha256)
  tar -C "$1" -czf "$1/$n.tar.gz" "$n"
}
d="$work/darwin"; mkdir "$d"; mkdarwin "$d" agent-bus-client 'u@h 2026-01-01 00:00:00'
out=$("$here/release-verify.sh" "$d/agent-bus-client-$v-darwin-$arch.tar.gz") && grep -q "darwin-$arch ok (version 1.2.3, client, read)" <<<"$out" || { echo "good darwin client refused: $out"; exit 1; }
d="$work/darwin-unstamped"; mkdir "$d"; mkdarwin "$d" agent-bus-client 'development (unstamped)'
out=$("$here/release-verify.sh" "$d/agent-bus-client-$v-darwin-$arch.tar.gz" 2>&1) && { echo "unstamped darwin accepted"; exit 1; }
grep -qF 'carries no build stamp' <<<"$out" || { echo "unstamped darwin: wrong failure: $out"; exit 1; }
d="$work/darwin-full"; mkdir "$d"; mkdarwin "$d" agent-bus 'u@h 2026-01-01 00:00:00'
out=$("$here/release-verify.sh" "$d/agent-bus-$v-darwin-$arch.tar.gz" 2>&1) && { echo "a full node for darwin accepted"; exit 1; }
grep -qF 'there is no full node for darwin' <<<"$out" || { echo "darwin full: wrong failure: $out"; exit 1; }
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

# The complete set is unsigned, but checksums remain a refusal boundary.
mkdir "$work/set"
cp "$good/$name.tar.gz" "$client" "$work/set/"
(cd "$work/set" && sha256sum *.tar.gz >SHA256SUMS)
out=$("$here/release-verify.sh" --arches "$arch" "$work/set") || { echo "unsigned release set refused: $out"; exit 1; }
cp "$work/set/SHA256SUMS" "$work/sums.good"
sed -i '1s/^[0-9a-f]\{64\}/0000000000000000000000000000000000000000000000000000000000000000/' "$work/set/SHA256SUMS"
out=$("$here/release-verify.sh" --arches "$arch" "$work/set" 2>&1) && { echo 'checksum mismatch: accepted a bad release'; exit 1; }
grep -qF 'does not match SHA256SUMS' <<<"$out" || { echo "checksum mismatch: wrong failure: $out"; exit 1; }
cp "$work/sums.good" "$work/set/SHA256SUMS"
out=$("$here/release-verify.sh" --signers ignored "$work/set" 2>&1) && { echo 'obsolete --signers accepted'; exit 1; }
grep -qF 'unsupported option --signers' <<<"$out" || { echo "wrong obsolete-option refusal: $out"; exit 1; }
echo 'unsigned release set: checksums required; obsolete --signers refused'
