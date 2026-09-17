#!/bin/bash
# Real SSH acceptance in a disposable rootless container, no host accounts,
# state, sockets or keys mounted. Requires a locally cached Arch Linux image
# with base account/PAM tools, Podman, Python, OpenSSH tools and built programs.
# Usage: bash src/acceptance/ssh-onboarding.sh <built-program-dir> <new-output-dir>
set -euo pipefail
bin=$(realpath "${1:?built program directory required}")
out=$(realpath -m "${2:?new evidence directory required}")
[ ! -e "$out" ] || { echo 'output directory must be new' >&2; exit 1; }
mkdir -m 700 -p "$out"
mkdir "$out/fixture" "$out/evidence"
cp "$(dirname "$0")/ssh-onboarding-container.sh" "$out/fixture/container.sh"
python3 - "$bin" "$out/fixture" <<'PY'
import pathlib,re,shutil,subprocess,sys
binary,root=map(pathlib.Path,sys.argv[1:])
for d in ('bin','ssh-tools','ssh-libs'): (root/d).mkdir()
for name in ('agent-busd','agent-bus-admin','agent-bus-setup','agent-bus-token'):
 shutil.copy2(binary/name,root/'bin'/name)
tools=[pathlib.Path(shutil.which(n) or n) for n in ('sshd','ssh','ssh-keygen')]
for name in ('sshd-session','sshd-auth'):
 for directory in ('/usr/libexec/openssh','/usr/lib/ssh','/usr/lib/openssh'):
  f=pathlib.Path(directory)/name
  if f.is_file(): tools.append(f); break
for tool in tools:
 shutil.copy2(tool,root/'ssh-tools'/tool.name)
 for dep in re.findall(r'=> (/[^ ]+)',subprocess.check_output(['ldd',str(tool)],text=True)):
  # Keep the image's loader/libc; copy other dependencies only if absent there.
  if pathlib.Path(dep).name not in ('libc.so.6','libresolv.so.2'):
   shutil.copy2(dep,root/'ssh-libs'/pathlib.Path(dep).name)
PY
podman run --rm --pull=never --network=none --security-opt label=disable \
 -v "$out/fixture:/fixture:ro" -v "$out/evidence:/evidence:rw" \
 "${SSH_ACCEPTANCE_IMAGE:-docker.io/library/archlinux:latest}" \
 bash /fixture/container.sh
