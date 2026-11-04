"""Run the Containerfile's actual pre-unpack checksum step, without a build."""
import hashlib
import json
import os
import shlex
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
ARCHIVE = 'agent-bus-1.2.3-linux-x86_64.tar.gz'


def verification_step():
    text = (HERE / 'Containerfile').read_text()
    start = text.index('RUN set -eu; cd /tmp/release; ')
    end = text.index('    mkdir unpacked;', start)
    return text[start + 4:end].replace('\\\n', ' ').replace('cd /tmp/release;', 'cd "$CHECK_RELEASE_DIR";', 1)


class ContainerChecksums(unittest.TestCase):
    def test_default_image_command_prints_help(self):
        text = (HERE / 'Containerfile').read_text()
        command = json.loads(next(line[4:] for line in text.splitlines()
                                  if line.startswith('CMD ')))
        entrypoint = json.loads(next(line[11:] for line in text.splitlines()
                                     if line.startswith('ENTRYPOINT ')))
        self.assertEqual(entrypoint, ['/usr/local/sbin/agent-bus-container'])
        result = subprocess.run(['bash', str(HERE / 'agent-bus-container'), *command],
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn('daemon   agent-busd', result.stdout)
        self.assertIn('runner   agent-bus-runner', result.stdout)
        self.assertEqual(result.stderr, '')

    def verify(self, *, aggregate=True, tampered=False, missing=False, sidecar=False):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='container-checksum-', dir=ROOT / 'tmp') as directory:
            work = Path(directory)
            content = b'release-shaped fixture; no archive is unpacked'
            archive = work / ARCHIVE
            archive.write_bytes(content + (b'tampered' if tampered else b''))
            line = hashlib.sha256(content).hexdigest() + '  ' + ARCHIVE + '\n'
            if aggregate:
                (work / 'SHA256SUMS').write_text(line.replace(ARCHIVE, 'another-archive.tar.gz') if missing
                                                   else line + '0' * 64 + '  unrelated-archive.tar.gz\n')
            if sidecar:
                (work / (ARCHIVE + '.sha256')).write_text(line)
            result = subprocess.run(['sh', '-eu', '-c', verification_step()],
                                    cwd=work, env=dict(os.environ, CHECK_RELEASE_DIR=str(work), ARCHIVE=ARCHIVE),
                                    capture_output=True, text=True, timeout=5)
            return result

    def test_downloaded_release_without_sidecars_verifies(self):
        result = self.verify()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_final_stage_does_not_copy_the_release_directory(self):
        text = (HERE / 'Containerfile').read_text()
        final_stage = text.rsplit('\nFROM ', 1)[-1]
        self.assertNotRegex(final_stage, r'\bmv\s', 'release payload moved across final image layers')
        from_copies = []
        for line in final_stage.splitlines():
            if line.startswith('COPY '):
                args = shlex.split(line)
                if any(arg.startswith('--from=') for arg in args):
                    from_copies.append(args)
                self.assertNotIn('.', args[1:-1], 'whole release directory retained in final image layers')
                self.assertNotIn('--from=release', args, 'raw release input retained in final image layers')
        self.assertEqual(len(from_copies), 1, 'expected one verified payload copy')
        self.assertEqual(from_copies[0][-1], '/usr/local/lib/agent-bus/', 'payload must be copied directly to its final location')

    def test_tampered_release_archive_is_refused(self):
        result = self.verify(tampered=True)
        self.assertNotEqual(result.returncode, 0, 'tampered archive accepted')
        self.assertIn('FAILED', result.stdout + result.stderr)

    def test_missing_aggregate_line_refuses_even_with_a_valid_sidecar(self):
        result = self.verify(missing=True, sidecar=True)
        self.assertNotEqual(result.returncode, 0, 'missing aggregate line accepted via sidecar')
        self.assertIn('missing checksum', result.stderr)

    def test_package_output_without_aggregate_uses_its_sidecar(self):
        result = self.verify(aggregate=False, sidecar=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
