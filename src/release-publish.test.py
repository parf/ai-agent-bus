"""Execute the release workflow's publication step against local tool stubs."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


def publication_step():
    text = (ROOT / '.github/workflows/release.yml').read_text()
    release = text.split('\n  release:\n', 1)[1]
    steps = re.split(r'(?=^      - )', release, flags=re.M)
    step, = [s for s in steps if 'name: Publish checked build artifacts\n' in s]
    run = step.split('        run: |\n', 1)[1]
    return '\n'.join(line[10:] for line in run.splitlines() if line.startswith('          '))


class ReleasePublication(unittest.TestCase):
    def publish(self, exists):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='release-publish-', dir=ROOT / 'tmp') as directory:
            work = Path(directory)
            for path in ['bin', 'dist', 'src/internal/version']:
                (work / path).mkdir(parents=True)
            version = '1.2.3'
            (work / 'src/internal/version/VERSION').write_text(version + '\n')
            for edition in ['agent-bus', 'agent-bus-client']:
                for platform in ['linux-x86_64', 'linux-aarch64', 'darwin-aarch64', 'darwin-x86_64']:
                    (work / f'dist/{edition}-{version}-{platform}.tar.gz').write_bytes(b'fixture archive')
            # Archive verification and npm packing have their own checks. Here
            # they produce only fixture assets; no archive is built or executed.
            (work / 'src/release-verify.sh').write_text('#!/bin/sh\nexit 0\n')
            (work / 'src/npm-pack.sh').write_text('''#!/bin/sh
set -eu
mkdir -p npm-dist
version=$(cat src/internal/version/VERSION)
for cpu in arm64 x64; do
  printf fixture > "npm-dist/ai-agent-bus-client-darwin-$cpu-$version.tgz"
done
''')
            (work / 'bin/gh').write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ['GH_ARGV_LOG'], 'a') as log:
    log.write(json.dumps(sys.argv[1:]) + '\\n')
exists = os.environ['GH_RELEASE_EXISTS'] == '1'
command = sys.argv[1:3]
if command == ['release', 'view']:
    sys.exit(0 if exists else 1)
if command == ['release', 'create']:
    sys.exit(9 if exists else 0)
if command == ['release', 'upload']:
    sys.exit(0 if exists else 10)
sys.exit(11)
''')
            for name in ['src/release-verify.sh', 'src/npm-pack.sh', 'bin/gh']:
                (work / name).chmod(0o755)
            log = work / 'argv.jsonl'
            env = dict(os.environ, PATH=str(work / 'bin') + os.pathsep + os.environ['PATH'],
                       GH_TOKEN='fixture-only', GH_RELEASE_EXISTS='1' if exists else '0',
                       GH_ARGV_LOG=str(log), GITHUB_REF_NAME='v' + version,
                       GITHUB_STEP_SUMMARY=str(work / 'summary'))
            result = subprocess.run(['bash', '--noprofile', '--norc', '-e', '-o', 'pipefail', '-c', publication_step()],
                                    cwd=work, env=env, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, 'publication step failed: ' + result.stderr)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            assets = sorted(str(p.relative_to(work)) for p in (work / 'dist').glob('*.tar.gz'))
            assets += ['dist/SHA256SUMS']
            assets += sorted(str(p.relative_to(work)) for p in (work / 'npm-dist').glob('ai-agent-bus-client-darwin-*.tgz'))
            if exists:
                wanted = ['release', 'upload', 'v' + version, *assets, '--clobber']
            else:
                wanted = ['release', 'create', 'v' + version, '--verify-tag', '--title', 'agent-bus ' + version,
                          '--notes', 'Release binaries and SHA256SUMS for npm packaging.', *assets]
            self.assertEqual(calls, [['release', 'view', 'v' + version], wanted],
                             'create-or-upload must keep the checked assets, tag and release notes')

    def test_absent_release_is_created_with_checked_assets_and_notes(self):
        self.publish(False)

    def test_existing_release_uploads_the_same_assets_with_clobber(self):
        self.publish(True)


if __name__ == '__main__':
    unittest.main()
