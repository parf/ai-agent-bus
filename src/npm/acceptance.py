#!/usr/bin/env python3
"""P.1b: what a release must survive before npm wraps it, and what the installed
packages must do. Local tarballs and a loopback registry only: nothing is
published and no account is logged in.

  python3 src/npm/acceptance.py            builds the release archives
  NPM_DIST=<dir> python3 src/npm/acceptance.py    reuses a built set

Refusals happen at the wrapper, before any package exists. Omitting a file from a
release that still checks out passes the wrapper, and the installed commands
must fail their own runtime assertion, so the assertions are shown able to fail.
"""
import contextlib
import getpass
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import select
import shutil
import subprocess
import tarfile
import tempfile
import threading
import time
import unittest

from registry import registry

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('npm_pack', Path(__file__).with_name('pack.py'))
pack = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pack)

PROGRAMS = ('agent-bus', 'agent-bus-runner', 'agent-busd', 'agent-bus-admin', 'agent-bus-setup')
ARCH = 'aarch64' if os.uname().machine == 'aarch64' else 'x86_64'


# --- a release, taken apart and put back together -----------------------------

class Release:
    """The full and client archives extracted, so one thing can be changed."""

    def __init__(self, dist, work):
        self.work = work
        self.full_archive = next(p for p in dist.glob(f'agent-bus-*-linux-{ARCH}.tar.gz') if '-client-' not in p.name)
        self.client_archive = dist / self.full_archive.name.replace('agent-bus-', 'agent-bus-client-', 1)
        self.version = self.full_archive.name.removeprefix('agent-bus-').split('-linux-')[0]
        self.full, self.client = (self._extract(path, label) for path, label in ((self.full_archive, 'full'), (self.client_archive, 'client')))

    def _extract(self, archive, label):
        target = self.work / ('release-' + label)
        shutil.rmtree(target, ignore_errors=True)
        target.mkdir(parents=True)
        with tarfile.open(archive) as tar:
            tar.extractall(target, filter='data')
        return next(target.iterdir())

    def omit(self, relative):
        """The file is gone from both editions and from their manifests: a release that checks out."""
        for root in (self.full, self.client):
            (root / relative).unlink()
        self._rewrite_full_manifest(lambda name: name != relative)
        self._regenerate_client_manifest()

    def _rewrite_full_manifest(self, keep):
        path = self.full / 'MANIFEST.sha256'
        path.write_text(''.join(line + '\n' for line in path.read_text().splitlines() if keep(line.split(maxsplit=1)[1])))

    def _regenerate_client_manifest(self):
        files = sorted(str(p.relative_to(self.client)) for p in self.client.rglob('*') if p.is_file() and not p.is_symlink() and p.name != 'MANIFEST.sha256')
        (self.client / 'MANIFEST.sha256').write_text(''.join(f'{pack.digest(self.client / name)}  {name}\n' for name in files))

    def replace(self, relative, data):
        """The file is there, checked and listed, but is not the real program."""
        for root in (self.full, self.client):
            (root / relative).write_bytes(data)
            (root / relative).chmod(0o755)
        digest = pack.digest(self.full / relative)
        self._rewrite_full_manifest(lambda name: True)
        path = self.full / 'MANIFEST.sha256'
        path.write_text(''.join((f'{digest}  {relative}' if line.split(maxsplit=1)[1] == relative else line) + '\n' for line in path.read_text().splitlines()))
        self._regenerate_client_manifest()

    def unlist(self, relative):
        """The file stays but the full manifest no longer checks it."""
        self._rewrite_full_manifest(lambda name: name != relative)

    def alter(self, relative):
        """A byte of the file changes, the manifest still says the old one."""
        with (self.full / relative).open('ab') as stream:
            stream.write(b'\0altered')

    def repack(self, out, *, corrupt_full_after_sums=False):
        """Archives and a SHA256SUMS for them, under out; the sums are taken before any corruption."""
        out.mkdir(parents=True, exist_ok=True)
        archives = []
        for root in (self.full, self.client):
            archive = out / (root.name + '.tar.gz')
            with tarfile.open(archive, 'w:gz') as tar:
                tar.add(root, arcname=root.name)
            archives.append(archive)
        sums = out / 'SHA256SUMS'
        sums.write_text(''.join(f'{pack.digest(a)}  {a.name}\n' for a in archives))
        if corrupt_full_after_sums:
            with archives[0].open('r+b') as stream:
                stream.seek(100)
                byte = stream.read(1)
                stream.seek(100)
                stream.write(bytes([byte[0] ^ 1]))
        return archives[0], archives[1], sums


# --- install and run ----------------------------------------------------------

class Installed:
    """The wrapped tarballs installed from a loopback registry into a scratch prefix."""

    def __init__(self, tarballs, work, version, names):
        self.work, self.version, self.names = work, version, names
        self.server = registry(tarballs)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.env = {'PATH': os.environ['PATH'], 'HOME': str(work), 'npm_config_cache': str(work / 'cache'), 'npm_config_update_notifier': 'false'}
        self.prefix = work / 'prefix'
        done = subprocess.run(['npm', 'install', '-g', '--prefix', str(self.prefix), '--no-audit', '--no-fund', '--ignore-scripts',
                               '--registry', f'http://127.0.0.1:{self.server.server_port}', names['bus'] + '@' + version],
                              env=self.env, capture_output=True, text=True)
        if done.returncode:
            raise AssertionError('npm install failed: ' + done.stderr[-1500:])
        self.bin = self.prefix / 'bin'

    def close(self):
        self.server.shutdown()
        self.server.server_close()

    def run(self, command, *, env=None, timeout=20):
        return subprocess.run(command, env={**self.env, **(env or {})}, capture_output=True, text=True, timeout=timeout)

    def bundles(self):
        """The client and bus platform bundles npm installed, resolved the way the launcher does."""
        arch = 'linux-arm64' if ARCH == 'aarch64' else 'linux-x64'
        bus_root = self.prefix / 'lib/node_modules' / self.names['bus']
        meta = json.loads((bus_root / 'package.json').read_text())

        def resolve(name, where):
            out = self.run(['node', '-e', 'console.log(require.resolve(process.argv[1]+"/package.json",{paths:[process.argv[2]]}))', name, str(where)])
            if out.returncode:
                raise AssertionError('a platform package is not installed: ' + name)
            return Path(out.stdout.strip()).parent
        client_root = resolve(meta['agentBus']['client'], bus_root)
        return (resolve(meta['agentBus']['clientPlatforms'][arch], client_root) / 'bundle',
                resolve(meta['agentBus']['platforms'][arch], bus_root) / 'bundle')


def assert_binaries(installed):
    """Every installed program answers --version with the release version and a build stamp."""
    for program in PROGRAMS:
        path = installed.bin / program
        if not path.is_file():
            raise AssertionError('missing installed command: ' + program)
        result = installed.run([str(path), '--version'])
        if result.returncode != 0 or result.stdout.splitlines()[:1] != [installed.version] or 'build_info: ' not in result.stdout:
            raise AssertionError(f'{program} --version failed ({result.returncode}): {result.stderr.strip()[:200]}')


def assert_mcp_tool_call(installed):
    """A started installed daemon answers an MCP tool call through the installed agent-bus-mcp."""
    # A unix socket path is short: the daemon's directory sits right under tmp/.
    directory = Path(tempfile.mkdtemp(prefix='d-', dir=ROOT / 'tmp'))
    log = (directory / 'log').open('w')
    daemon = subprocess.Popen([str(installed.bin / 'agent-busd'), '--addr', '127.0.0.1:0', '--socket', str(directory / 'bus.sock'),
                               '--owner', 'owner@fixture', '--db', str(directory / 'bus.db'), '--create', '--flush-every', '0',
                               '--ssh-keys', str(directory / 'ssh/authorized_keys')], env=installed.env, stdout=log, stderr=log)
    try:
        local = directory / ('user-' + getpass.getuser() + '.sock')
        for _ in range(100):
            if local.exists() or daemon.poll() is not None:
                break
            time.sleep(0.05)
        cli = str(installed.bin / 'agent-bus')
        token = installed.run([cli, 'token', 'owner@fixture'], env={'AGENT_BUS_ADDR': str(local)})
        if token.returncode:
            raise AssertionError('no token from the installed daemon: ' + token.stderr[-300:])
        base = {'AGENT_BUS_ADDR': str(directory / 'bus.sock'), 'AGENT_BUS_TOKEN': token.stdout.strip(), 'AGENT_BUS_NAME': 'owner@fixture'}
        for name in ('#asker@fixture', '#worker@fixture'):
            registered = installed.run([cli, 'register', name, '--kind', 'agent', '--allow', '*'], env=base)
            if registered.returncode:
                raise AssertionError('register failed: ' + registered.stderr[-300:])
        asker = installed.run([cli, 'token', '#asker@fixture'], env=base)
        if asker.returncode:
            raise AssertionError('no token for the asker: ' + asker.stderr[-300:])
        base = {**base, 'AGENT_BUS_TOKEN': asker.stdout.strip(), 'AGENT_BUS_NAME': '#asker@fixture'}
        requests = [
            {'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'npm-acceptance', 'version': '1'}}},
            {'jsonrpc': '2.0', 'method': 'notifications/initialized'},
            {'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call', 'params': {'name': 'ab_ls', 'arguments': {'kind': 'agent'}}},
        ]
        process = subprocess.Popen([str(installed.bin / 'agent-bus-mcp')], env={**installed.env, **base}, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        answer, buffered, until = None, b'', time.monotonic() + 15
        try:
            process.stdin.write(''.join(json.dumps(row) + '\n' for row in requests))
            process.stdin.flush()
            while time.monotonic() < until and answer is None:
                ready, _, _ = select.select([process.stdout], [], [], max(0, until - time.monotonic()))
                if not ready:
                    break
                chunk = os.read(process.stdout.fileno(), 65536)
                if not chunk:
                    break
                buffered += chunk
                while b'\n' in buffered:
                    line, buffered = buffered.split(b'\n', 1)
                    value = json.loads(line)
                    if value.get('id') == 2:
                        answer = value
        finally:
            process.stdin.close()
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            error = process.stderr.read()
            process.stdout.close()
            process.stderr.close()
        if answer is None or 'error' in answer or answer.get('result', {}).get('isError') or '#worker@fixture' not in json.dumps(answer):
            raise AssertionError('the installed MCP gave no tool answer: ' + (error or json.dumps(answer))[-300:])
    finally:
        daemon.terminate()
        try:
            daemon.wait(timeout=5)
        except subprocess.TimeoutExpired:
            daemon.kill()
            daemon.wait()
        log.close()
        shutil.rmtree(directory, ignore_errors=True)


# --- the checks ---------------------------------------------------------------

class Acceptance(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        scratch = tempfile.TemporaryDirectory(prefix='npm-acceptance-', dir=ROOT / 'tmp')
        cls.addClassCleanup(scratch.cleanup)
        cls.work = Path(scratch.name)
        if 'NPM_DIST' in os.environ:
            dist = Path(os.environ['NPM_DIST'])
        else:
            dist = cls.work / 'dist'
            built = subprocess.run([str(ROOT / 'src/package.sh'), str(dist)], capture_output=True, text=True)
            if built.returncode:
                raise AssertionError('release build failed: ' + built.stderr[-1500:])
        cls.dist = dist
        cls.names = pack.names()

    def release(self, name):
        directory = self.work / name
        directory.mkdir()
        return Release(self.dist, directory), directory

    def wrap_and_install(self, release, directory):
        full, client, sums = release.repack(directory / 'archives')
        with contextlib.redirect_stdout(None):
            pack.build(full, client, sums, directory / 'tarballs')
        installed = Installed(directory / 'tarballs', directory / 'install', release.version, self.names)
        self.addCleanup(installed.close)
        return installed

    def refused(self, release, directory, message, **repack):
        full, client, sums = release.repack(directory / 'archives', **repack)
        with self.assertRaises(ValueError) as caught, contextlib.redirect_stdout(None):
            pack.build(full, client, sums, directory / 'tarballs')
        self.assertIn(message, str(caught.exception))
        self.assertEqual(list((directory / 'tarballs').glob('*')) if (directory / 'tarballs').exists() else [], [],
                         'a package was built from a release that was refused')

    # The control: unchanged, both runtime assertions pass, and the installed
    # bytes are the release manifest's.
    def test_00_the_release_passes_its_runtime_assertions_and_installs_what_the_manifest_lists(self):
        release, directory = self.release('control')
        installed = self.wrap_and_install(release, directory)
        assert_binaries(installed)
        assert_mcp_tool_call(installed)
        client_bundle, bus_bundle = installed.bundles()
        listed = pack.manifest(release.full / 'MANIFEST.sha256')
        client_listed = pack.manifest(release.client / 'MANIFEST.sha256')
        self.assertTrue({'agent-bus', 'agent-bus-runner', 'agent-bus-ts', 'agent-busd'} <= set(listed), 'the manifest names the executables')
        for name, checksum in listed.items():
            where = client_bundle if name in client_listed else bus_bundle
            self.assertTrue((where / name).is_file(), 'not installed: ' + name)
            self.assertEqual(pack.digest(where / name), checksum, 'installed bytes differ from the release manifest: ' + name)

    def test_01_a_release_without_an_executable_installs_and_fails_the_binary_assertion(self):
        release, directory = self.release('omit-exe')
        release.omit('agent-bus-runner')
        installed = self.wrap_and_install(release, directory)
        with self.assertRaises(AssertionError) as caught:
            assert_binaries(installed)
        self.assertIn('agent-bus-runner', str(caught.exception))
        assert_mcp_tool_call(installed)  # nothing else broke: the assertion named the omission

    def test_02_a_release_without_an_mcp_asset_installs_and_fails_the_mcp_assertion(self):
        release, directory = self.release('omit-mcp')
        release.omit('agent-bus-ts')
        installed = self.wrap_and_install(release, directory)
        assert_binaries(installed)  # the executables are all there
        with self.assertRaises(AssertionError) as caught:
            assert_mcp_tool_call(installed)
        self.assertIn('MCP', str(caught.exception))

    def test_02b_a_checked_but_broken_mcp_face_installs_and_fails_the_mcp_assertion(self):
        # The omitted file is refused by the full setup itself; a present, listed
        # and consistent file that does not work is what only the tool call can tell.
        release, directory = self.release('broken-mcp')
        release.replace('agent-bus-ts', b'#!/bin/sh\nexit 1\n')
        installed = self.wrap_and_install(release, directory)
        assert_binaries(installed)
        with self.assertRaises(AssertionError) as caught:
            assert_mcp_tool_call(installed)
        self.assertIn('MCP', str(caught.exception))

    def test_03_a_corrupt_archive_is_refused_before_anything_is_built(self):
        release, directory = self.release('corrupt')
        self.refused(release, directory, 'does not match SHA256SUMS', corrupt_full_after_sums=True)

    def test_04_an_altered_file_is_refused_against_its_manifest(self):
        release, directory = self.release('altered')
        release.alter('agent-bus-runner')
        self.refused(release, directory, 'release manifest mismatch: agent-bus-runner')

    def test_05_an_incomplete_manifest_is_refused_before_unchecked_files_are_packaged(self):
        release, directory = self.release('unlisted')
        release.unlist('agent-bus-runner')
        self.refused(release, directory, 'release has files its manifest does not check: agent-bus-runner')


if __name__ == '__main__':
    unittest.main(verbosity=2)
