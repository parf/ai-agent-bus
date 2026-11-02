"""Real mount policy, using only this test's labelled container and finally cleanup."""
import json
import socket
import select
import signal
import subprocess
import sys
from pathlib import Path
import unittest
import uuid

import fixtures


class VolumeMounts(unittest.TestCase):
    def mounts(self, *, implicit_only):
        fixtures.gate_held()
        name = f'ab-db-fixture-{fixtures.RUN}-mount-{uuid.uuid4().hex[:8]}'
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        args = fixtures.container_args(fixtures.POSTGRES, name, port, 5432, {'POSTGRES_PASSWORD': 'fixture-only'})
        if implicit_only:
            # Exercise the implicit-volume guard on its own, without adding a
            # test switch to production or replacing its argument construction.
            at = args.index('--tmpfs')
            args = args[:at] + args[at + 2:]
        try:
            fixtures.start_container(name, args)
            inspected = json.loads(fixtures.podman('inspect', name).stdout)[0]
            self.assertEqual(inspected['Name'].lstrip('/'), name)
            self.assertEqual(inspected['Config']['Labels']['ab-db-fixture'], fixtures.RUN)
            # Podman's Mounts omits tmpfs entries; read the kernel's mount
            # table in this same container for the actual filesystem type.
            info = fixtures.podman('exec', name, 'cat', '/proc/self/mountinfo').stdout
            types = {}
            for line in info.splitlines():
                before, after = line.split(' - ', 1)
                types[before.split()[4]] = after.split()[0]
            return inspected['Mounts'], types
        finally:
            fixtures.podman('rm', '-f', '-v', '-t', '1', name, check=False)

    def test_real_data_path_is_tmpfs_and_there_is_no_volume(self):
        mounts, by_path = self.mounts(implicit_only=False)
        self.assertFalse([m for m in mounts if m['Type'] == 'volume'], mounts)
        self.assertEqual(by_path.get('/var/lib/postgresql/data'), 'tmpfs', mounts)

    def test_real_implicit_volumes_are_disabled_without_the_explicit_tmpfs(self):
        mounts, _ = self.mounts(implicit_only=True)
        self.assertFalse([m for m in mounts if m['Type'] == 'volume'], mounts)

    def test_sigterm_during_mount_check_removes_its_container_and_volume(self):
        fixtures.gate_held()
        # Leave the image VOLUME exposed only in this cleanup proof. Pause
        # inside run after creation, when finally cannot run on os._exit.
        code = """import sys, signal
sys.path.insert(0, sys.argv[1])
import fixtures, test_volume_mounts
args = fixtures.container_args
real = fixtures.podman
def exposed(*a, **kw):
    return [v for v in args(*a, **kw) if v != '--image-volume=ignore']
def held(*a, **kw):
    result = real(*a, **kw)
    if a[0] == 'run':
        print(a[a.index('--name') + 1], flush=True)
        signal.pause()
    return result
fixtures.container_args = exposed
fixtures.podman = held
test_volume_mounts.VolumeMounts().mounts(implicit_only=True)
"""
        child = subprocess.Popen([sys.executable, '-B', '-c', code, str(Path(__file__).resolve().parent)],
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        name, volumes = '', []
        try:
            ready, _, _ = select.select([child.stdout], [], [], 20)
            self.assertTrue(ready, 'the child did not start its own container')
            name = child.stdout.readline().strip()
            self.assertTrue(name.startswith(f'ab-db-fixture-{child.pid}-'), name)
            inspected = json.loads(fixtures.podman('inspect', name).stdout)[0]
            self.assertTrue(inspected['Config']['Labels']['ab-db-fixture'].startswith(str(child.pid) + '-'))
            volumes = [m['Name'] for m in inspected['Mounts'] if m['Type'] == 'volume']
            self.assertEqual(len(volumes), 1, inspected['Mounts'])
            child.send_signal(signal.SIGTERM)
            self.assertEqual(child.wait(15), 128 + signal.SIGTERM)
            left = fixtures.podman('ps', '-a', '--filter', 'name=^' + name + '$', '-q').stdout.strip()
            self.assertEqual(left, '', 'SIGTERM left its labelled fixture container')
            remaining = {v['Name'] for v in json.loads(fixtures.podman('volume', 'ls', '--format', 'json').stdout)}
            self.assertFalse(set(volumes) & remaining, 'SIGTERM left the fixture volume')
        finally:
            if child.poll() is None:
                child.kill()
            child.wait(15)
            child.stdout.close()
            child.stderr.close()
            if name:
                fixtures.podman('rm', '-f', '-v', '-t', '1', name, check=False)
            for volume in volumes:
                fixtures.podman('volume', 'rm', volume, check=False)
