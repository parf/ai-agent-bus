"""Real mount policy, using only this test's labelled container and finally cleanup."""
import json
import socket
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
            fixtures.podman(*args)
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
