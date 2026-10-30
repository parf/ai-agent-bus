#!/bin/sh
# Sign one release for install.sh: run by the owner, on the machine that holds
# the release key, after downloading the draft release's archives.
#
#   src/release-sign.sh <dir-with-archives> <private-key>
#
# Writes <dir>/SHA256SUMS (every *.tar.gz there) and SHA256SUMS.sig, an
# ssh-keygen signature in the agent-bus-release namespace. Upload both to the
# release; install.sh refuses a release without them. The key never leaves
# that machine and is never given to CI (docs/07-setup.md#simple-install).
set -eu
dir=${1:?usage: release-sign.sh <dir-with-archives> <private-key>}
key=${2:?usage: release-sign.sh <dir-with-archives> <private-key>}
cd "$dir"
set -- *.tar.gz
[ -e "$1" ] || { echo "release-sign: no *.tar.gz in $dir" >&2; exit 1; }
sha256sum "$@" > SHA256SUMS.new
mv SHA256SUMS.new SHA256SUMS
rm -f SHA256SUMS.sig
ssh-keygen -Y sign -q -f "$key" -n agent-bus-release SHA256SUMS
echo "signed $(wc -l < SHA256SUMS) archives: $dir/SHA256SUMS and SHA256SUMS.sig"
