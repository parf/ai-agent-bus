#!/bin/bash
# Verify package.sh's real full/client archives and their unsigned SHA256SUMS.
# RELEASE_DIST reuses built archives read-only; otherwise build this host's pair.
set -eu
cd "$(dirname "$0")"
here=$PWD
mkdir -p ../tmp
work=$(mktemp -d "$here/../tmp/rvreal.XXXXXX")
trap 'rm -rf "$work"' EXIT
arch=$(uname -m)
dist=${RELEASE_DIST:-$work/dist}
[ -n "${RELEASE_DIST:-}" ] || ./package.sh "$dist" >/dev/null 2>&1
sums() { (cd "$1" && sha256sum agent-bus-*.tar.gz >SHA256SUMS); }
verify() { "$here/release-verify.sh" --arches "$arch" "$1"; }
mkdir "$work/rel"
cp "$dist"/agent-bus-*.tar.gz "$work/rel/"
sums "$work/rel"
out=$(verify "$work/rel") || { echo "real release refused: $out"; exit 1; }
grep -q 'ok (version' <<<"$out" || { echo "real release: no ok line: $out"; exit 1; }
expect_fail() { # <label> <message> <dir>
  local o rc=0
  o=$(verify "$3" 2>&1) || rc=$?
  [ $rc != 0 ] || { echo "$1: accepted"; exit 1; }
  grep -qF "$2" <<<"$o" || { echo "$1: failed, but not on '$2': $o"; exit 1; }
}
t() { rm -rf "$work/$1"; mkdir "$work/$1"; cp "$work"/rel/* "$work/$1/"; }
# An archive changed after checksums: the sums no longer match it.
t tamper; echo x >>"$work/tamper/agent-bus-client-"*"-linux-$arch.tar.gz"
expect_fail tampered 'does not match SHA256SUMS' "$work/tamper"
# One of the four archives gone.
t missing; rm "$work"/missing/agent-bus-client-*.tar.gz
expect_fail missing 'is missing' "$work/missing"
# A checksum list that names the right hash under a name differing at a dot:
# the archive's name is text, not a pattern, so it does not match.
t dotname
sed -i -E 's/(agent-bus-client-[0-9]+)\./\1x/' "$work/dotname/SHA256SUMS"
expect_fail dotname 'does not match SHA256SUMS' "$work/dotname"
t nosums; rm "$work/nosums/SHA256SUMS"
expect_fail nosums 'SHA256SUMS is missing' "$work/nosums"
out=$("$here/release-verify.sh" --signers ignored "$work/rel" 2>&1) && { echo 'obsolete --signers accepted'; exit 1; }
grep -qF 'unsupported option --signers' <<<"$out" || { echo "wrong --signers refusal: $out"; exit 1; }
echo 'real package.sh archives: pass unsigned; fail tampered, missing, renamed checksum and absent sums'
