#!/bin/bash
# release-verify.sh on what package.sh really builds: the host's two archives,
# signed as release-sign.sh signs (ssh-keygen -Y, agent-bus-release) with a
# throwaway key. It must pass on the set and fail on a tampered archive, a
# missing one and a bad signature. The other CPU's archives need a cross C
# compiler this node lacks, so the set is the host's arch only (--arches).
set -eu
cd "$(dirname "$0")"
here=$PWD
mkdir -p ../tmp
work=$(mktemp -d "$here/../tmp/rvreal.XXXXXX")
trap 'rm -rf "$work"' EXIT
arch=$(uname -m)
./package.sh "$work/dist" >/dev/null 2>&1
for k in key other; do ssh-keygen -q -t ed25519 -N '' -C throwaway -f "$work/$k"; done
echo "agent-bus-release $(cut -d' ' -f1,2 "$work/key.pub")" >"$work/signers"
sign() { (cd "$1" && rm -f SHA256SUMS SHA256SUMS.sig && sha256sum agent-bus-*.tar.gz >SHA256SUMS && ssh-keygen -Y sign -q -f "$2" -n agent-bus-release SHA256SUMS); }
verify() { "$here/release-verify.sh" --signers "$work/signers" --arches "$arch" "$1"; }
mkdir "$work/rel"
ln "$work"/dist/agent-bus-*.tar.gz "$work/rel/" 2>/dev/null || cp "$work"/dist/agent-bus-*.tar.gz "$work/rel/"
sign "$work/rel" "$work/key"
out=$(verify "$work/rel") || { echo "real release refused: $out"; exit 1; }
grep -q 'ok (version' <<<"$out" || { echo "real release: no ok line: $out"; exit 1; }
expect_fail() { # <label> <message> <dir>
  local o rc=0
  o=$(verify "$3" 2>&1) || rc=$?
  [ $rc != 0 ] || { echo "$1: accepted"; exit 1; }
  grep -qF "$2" <<<"$o" || { echo "$1: failed, but not on '$2': $o"; exit 1; }
}
t() { rm -rf "$work/$1"; mkdir "$work/$1"; cp "$work"/rel/* "$work/$1/"; }
# An archive changed after signing: the sums no longer match it.
t tamper; echo x >>"$work/tamper/agent-bus-client-"*"-linux-$arch.tar.gz"
expect_fail tampered 'does not match SHA256SUMS' "$work/tamper"
# One of the four archives gone.
t missing; rm "$work"/missing/agent-bus-client-*.tar.gz
expect_fail missing 'is missing' "$work/missing"
# Signed by a key the signers file does not name.
t badsig; sign "$work/badsig" "$work/other"
expect_fail badsig 'SHA256SUMS.sig does not verify' "$work/badsig"
# An unsigned release.
t nosig; rm "$work/nosig/SHA256SUMS.sig"
expect_fail nosig 'SHA256SUMS or SHA256SUMS.sig is missing' "$work/nosig"
echo 'real package.sh archives: pass signed; fail tampered, missing, wrongly signed, unsigned'
