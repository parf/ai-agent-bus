#!/bin/bash
set -euo pipefail
cd /usr/local/src/ai-agent-bus/tmp/ab-w1c-macos-release
export GOCACHE=/usr/local/src/ai-agent-bus/tmp/gocache
export BUILD_COMMIT=$(git rev-parse HEAD)
work=$(mktemp -d /usr/local/src/ai-agent-bus/tmp/macos-release-proof.XXXXXX)
trap 'rm -rf "$work"' EXIT
mkdir "$work/collected"
for arch in arm64 amd64; do
  echo "BUILD darwin $arch stamp=$BUILD_COMMIT"
  GOOS=darwin GOARCH=$arch src/package.sh "$work/$arch"
  src/release-verify.sh "$work/$arch/"*.tar.gz
  cp "$work/$arch/"*.tar.gz "$work/collected/"
done
(cd "$work/collected" && sha256sum *.tar.gz > SHA256SUMS)
src/release-verify.sh --os darwin "$work/collected"
echo 'PASS complete Darwin full/client sets, both Mach-O CPUs and stamps'
GOOS=linux GOARCH=amd64 src/package.sh "$work/linux"
src/release-verify.sh "$work/linux/"*.tar.gz
cp "$work/linux/"*.tar.gz "$work/collected/"
(cd "$work/collected" && sha256sum *.tar.gz > SHA256SUMS)
src/release-verify.sh --arches x86_64 --os linux "$work/linux"/*.tar.gz
v=$(tr -d '[:space:]' <src/internal/version/VERSION)
src/npm-pack.sh "$work/collected/agent-bus-$v-linux-x86_64.tar.gz" "$work/collected/agent-bus-client-$v-linux-x86_64.tar.gz" "$work/collected/SHA256SUMS" "$work/npm" --darwin-client "$work/collected/agent-bus-client-$v-darwin-aarch64.tar.gz" --darwin-client "$work/collected/agent-bus-client-$v-darwin-x86_64.tar.gz"
python3 - "$work/npm" "$v" <<'CHECK'
import json, pathlib, sys, tarfile
out=pathlib.Path(sys.argv[1]); version=sys.argv[2]
for cpu in ('arm64','x64'):
 name=f'ai-agent-bus-client-darwin-{cpu}'
 with tarfile.open(out/f'{name}-{version}.tgz') as t:
  p=json.load(t.extractfile('package/package.json'))
  assert p['name']==name and p['os']==['darwin'] and p['cpu']==[cpu]
  assert not any(n.endswith('/agent-busd') or n.endswith('/agent-bus-setup') for n in t.getnames())
 print('PASS npm client-only',name)
for name in ('ai-agent-bus','ai-agent-bus-client'):
 with tarfile.open(out/f'{name}-{version}.tgz') as t:
  p=json.load(t.extractfile('package/package.json'))
  for cpu in ('arm64','x64'): assert p['optionalDependencies'][f'ai-agent-bus-client-darwin-{cpu}']==version
 print('PASS command package Darwin pins',name)
CHECK
bash src/release-verify.test.sh
