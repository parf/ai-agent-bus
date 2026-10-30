#!/usr/bin/env python3
"""Local-registry npm acceptance, without a host install or publish."""
import getpass
import importlib.util
import json
import os
from pathlib import Path
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


class Packages(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        cls.scratch = tempfile.TemporaryDirectory(prefix='npm-check-', dir=ROOT / 'tmp')
        cls.addClassCleanup(cls.scratch.cleanup)
        cls.work = Path(cls.scratch.name)
        dist = Path(os.environ['NPM_DIST']) if 'NPM_DIST' in os.environ else cls.work / 'dist'
        if 'NPM_DIST' not in os.environ:
            built = subprocess.run([str(ROOT / 'src/package.sh'), str(dist)], capture_output=True, text=True)
            if built.returncode:
                raise AssertionError('release build failed: ' + built.stderr[-2000:])
        arch = 'aarch64' if os.uname().machine == 'aarch64' else 'x86_64'
        full = next(p for p in dist.glob('agent-bus-*-linux-' + arch + '.tar.gz') if '-client-' not in p.name)
        client = dist / full.name.replace('agent-bus-', 'agent-bus-client-', 1)
        sums = cls.work / 'SHA256SUMS'
        sums.write_text(''.join(f'{pack.digest(p)}  {p.name}\n' for p in (full, client)))
        cls.expected_sums = pack.digest(sums)
        cls.tarballs = cls.work / 'tarballs'
        pack.build(full, client, sums, cls.tarballs)
        cls.names = pack.names()
        cls.version = full.name.removeprefix('agent-bus-').split('-linux-')[0]
        cls.server = registry(cls.tarballs)
        cls.addClassCleanup(cls.server.server_close)
        cls.addClassCleanup(cls.server.shutdown)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.prefix = cls.work / 'prefix'
        cls.env = {'PATH': os.environ['PATH'], 'HOME': str(cls.work),
                   'npm_config_cache': str(cls.work / 'cache'), 'npm_config_update_notifier': 'false'}
        installed = subprocess.run(['npm', 'install', '-g', '--prefix', str(cls.prefix), '--no-audit', '--no-fund',
                                    '--ignore-scripts', '--registry', f'http://127.0.0.1:{cls.server.server_port}',
                                    cls.names['bus'] + '@' + cls.version], env=cls.env, capture_output=True, text=True)
        if installed.returncode:
            raise AssertionError('installing bus alone failed: ' + installed.stderr[-2000:])
        cls.bin = cls.prefix / 'bin'
        cls.bus_root = cls.prefix / 'lib/node_modules' / cls.names['bus']

    def run_command(self, command, *, env=None, timeout=15, input=None):
        return subprocess.run(command, env={**self.env, **(env or {})}, input=input,
                              capture_output=True, text=True, timeout=timeout)

    def client_root(self):
        result = self.run_command(['node', '-e', 'console.log(require.resolve(process.argv[1]+"/package.json",{paths:[process.argv[2]]}))',
                                   self.names['client'], str(self.bus_root)])
        self.assertEqual(result.returncode, 0, result.stderr)
        return Path(result.stdout.strip()).parent

    def test_bus_alone_installs_exact_client_and_platform_pins(self):
        bus = json.loads((self.bus_root / 'package.json').read_text())
        self.assertEqual(bus['dependencies'], {self.names['client']: self.version})
        client = json.loads((self.client_root() / 'package.json').read_text())
        self.assertEqual(client['version'], bus['version'])
        for package in (bus, client):
            self.assertTrue(package['optionalDependencies'])
            self.assertTrue(all(v == self.version for v in package['optionalDependencies'].values()))
            self.assertNotIn('scripts', package, 'npm install must run no lifecycle hooks')
            self.assertEqual(package['os'], ['linux'])
            self.assertEqual(package['cpu'], ['x64', 'arm64'])

    def test_installed_binaries_versions_stamps_and_exit_status(self):
        for program in ('agent-bus', 'agent-bus-runner', 'agent-busd', 'agent-bus-admin', 'agent-bus-setup'):
            self.assertTrue((self.bin / program).is_file(), 'missing installed binary: ' + program)
            result = self.run_command([str(self.bin / program), '--version'])
            self.assertEqual(result.returncode, 0, program + ': ' + result.stderr)
            self.assertEqual(result.stdout.splitlines()[0], self.version, program)
            self.assertIn('build_info: ', result.stdout, program)
            self.assertNotIn('unstamped', result.stdout, program)
        result = self.run_command([str(self.bin / 'agent-bus')])
        self.assertEqual(result.returncode, 1)
        for program in ('agent-bus-mcp', 'ab-claude', 'ab-codex', 'ab-opencode', 'ab-opencode1'):
            self.assertTrue((self.bin / program).is_file(), program)

    def test_license_provenance_and_no_duplicate_release_bytes(self):
        platform_files = {}
        for archive in self.tarballs.glob('*.tgz'):
            with tarfile.open(archive) as tar:
                meta = json.load(tar.extractfile('package/package.json'))
                self.assertIn('package/LICENSE.md', tar.getnames(), 'package license missing')
                self.assertGreater(tar.getmember('package/LICENSE.md').size, 0)
                self.assertEqual(meta['agentBusRelease']['sha256sums'], self.expected_sums)
                if 'agentBus' not in meta:
                    platform_files[meta['name']] = {entry.name for entry in tar if entry.isfile() and entry.name.startswith('package/bundle/')}
        self.assertEqual(len(platform_files), 2)
        a, b = platform_files.values()
        self.assertFalse(a & b, 'bus and client duplicate release files')
        union = a | b
        for program in ('agent-busd', 'agent-bus-admin', 'agent-bus-setup', 'agent-bus', 'agent-bus-runner', 'agent-bus-ts'):
            self.assertIn('package/bundle/' + program, union)

    def test_missing_platform_refuses_clearly(self):
        bus_meta = json.loads((self.bus_root / 'package.json').read_text())
        platform = bus_meta['agentBus']['platforms']['linux-' + ('arm64' if os.uname().machine == 'aarch64' else 'x64')]
        result = self.run_command(['node', '-e', 'console.log(require.resolve(process.argv[1]+"/package.json",{paths:[process.argv[2]]}))', platform, str(self.bus_root)])
        self.assertEqual(result.returncode, 0, result.stderr)
        directory = Path(result.stdout.strip()).parent
        hidden = directory.with_name(directory.name + '.hidden')
        directory.rename(hidden)
        try:
            refused = self.run_command([str(self.bin / 'agent-busd'), '--version'])
            self.assertNotEqual(refused.returncode, 0)
            self.assertIn('package is not installed', refused.stderr)
        finally:
            hidden.rename(directory)

    def test_nonroot_setup_requires_explicit_sudo(self):
        if os.geteuid() == 0:
            self.skipTest('ordinary-user refusal belongs to the host test')
        result = self.run_command([str(self.bin / 'agent-bus-setup'), '--owner', 'fixture'])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('explicit sudo', result.stderr)

    def test_setup_informational_flag_alone_needs_no_sudo(self):
        for flag in ('--version', '-version', '--help', '-h'):
            with self.subTest(flag=flag):
                result = self.run_command([str(self.bin / 'agent-bus-setup'), flag])
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn('explicit sudo', result.stderr)
                self.assertTrue(result.stdout or result.stderr, 'informational request produced no output')

    def test_setup_help_after_positional_still_requires_install_guards(self):
        if os.geteuid() == 0:
            self.skipTest('ordinary-user refusal belongs to the host test')
        setup = str(self.bin / 'agent-bus-setup')
        options = ['--dry-run', '--owner', 'fixture', '--tls', 'off',
                   '--exec', str(self.bin / 'agent-busd'), 'ignored-positional']
        for flag in ('--help', '-h', '--version', '-version'):
            with self.subTest(flag=flag):
                result = self.run_command([setup, *options, flag])
                self.assertNotEqual(result.returncode, 0, 'trailing help/version bypassed sudo guards')
                self.assertIn('explicit sudo', result.stderr)
                self.assertNotIn('would', result.stdout.lower(), 'setup ran despite its install guards')

    def test_request_reply_and_mcp_through_installed_commands(self):
        directory = self.work / 'daemon'
        directory.mkdir()
        log = (directory / 'log').open('w')
        daemon = subprocess.Popen([str(self.bin / 'agent-busd'), '--addr', '127.0.0.1:0', '--socket', str(directory / 'bus.sock'),
                                   '--owner', 'owner@fixture', '--db', str(directory / 'bus.db'), '--create', '--flush-every', '0',
                                   '--ssh-keys', str(directory / 'ssh/authorized_keys')], env=self.env, stdout=log, stderr=log)
        try:
            local = directory / ('user-' + getpass.getuser() + '.sock')
            for _ in range(100):
                if local.exists() or daemon.poll() is not None:
                    break
                time.sleep(0.05)
            token = self.run_command([str(self.bin / 'agent-bus'), 'token', 'owner@fixture'], env={'AGENT_BUS_ADDR': str(local)})
            self.assertEqual(token.returncode, 0, token.stderr + (directory / 'log').read_text())
            base = {'AGENT_BUS_ADDR': str(directory / 'bus.sock'), 'AGENT_BUS_TOKEN': token.stdout.strip(), 'AGENT_BUS_NAME': 'owner@fixture'}
            cli = str(self.bin / 'agent-bus')
            for name in ('#asker@fixture', '#worker@fixture'):
                registered = self.run_command([cli, 'register', name, '--kind', 'agent', '--allow', '*'], env=base)
                self.assertEqual(registered.returncode, 0, registered.stderr)
            tokens = {}
            for name in ('#asker@fixture', '#worker@fixture'):
                result = self.run_command([cli, 'token', name], env=base)
                self.assertEqual(result.returncode, 0, result.stderr)
                tokens[name] = {**base, 'AGENT_BUS_TOKEN': result.stdout.strip(), 'AGENT_BUS_NAME': name}
            sent = self.run_command([cli, 'send', '#worker@fixture', '--topic', 'npm', '--tag', 'request', 'npm question'], env=tokens['#asker@fixture'])
            self.assertEqual(sent.returncode, 0, sent.stderr)
            got = self.run_command([cli, 'consume', '--wait', '1s'], env=tokens['#worker@fixture'])
            self.assertEqual(got.returncode, 0, got.stderr)
            message = json.loads(got.stdout)
            self.assertEqual(message['body'], 'npm question')
            reply = self.run_command([cli, 'reply', message['message_id'], 'npm answer'], env=tokens['#worker@fixture'])
            self.assertEqual(reply.returncode, 0, reply.stderr)
            answer = self.run_command([cli, 'consume', '--topic', 'npm', '--tag', 'request', '--wait', '1s'], env=tokens['#asker@fixture'])
            self.assertEqual(answer.returncode, 0, answer.stderr)
            self.assertEqual(json.loads(answer.stdout)['body'], 'npm answer')
            requests = [
                {'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'npm-fixture', 'version': '1'}}},
                {'jsonrpc': '2.0', 'method': 'notifications/initialized'},
                {'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call', 'params': {'name': 'ab_ls', 'arguments': {'kind': 'agent'}}},
            ]
            process = subprocess.Popen([str(self.bin / 'agent-bus-mcp')], env={**self.env, **tokens['#asker@fixture']}, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                process.stdin.write(''.join(json.dumps(row) + '\n' for row in requests)); process.stdin.flush()
                # MCP handles input asynchronously: keep stdin alive until the
                # matching tool reply arrives instead of closing it at send.
                import select
                deadline = time.monotonic() + 10
                result = None
                buffered = b''
                while time.monotonic() < deadline:
                    ready, _, _ = select.select([process.stdout], [], [], max(0, deadline-time.monotonic()))
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
                            result = value
                    if result is not None:
                        break
                self.assertIsNotNone(result, 'installed MCP emitted no matching tool result')
                self.assertNotIn('error', result)
                self.assertFalse(result.get('result', {}).get('isError', False), result)
                self.assertIn('#worker@fixture', json.dumps(result))
            finally:
                process.stdin.close()
                try: process.wait(timeout=2)
                except subprocess.TimeoutExpired: process.kill(); process.wait()
                process.stdout.close(); process.stderr.close()
        finally:
            daemon.terminate()
            try: daemon.wait(timeout=5)
            except subprocess.TimeoutExpired: daemon.kill(); daemon.wait()
            log.close()


if __name__ == '__main__':
    unittest.main(verbosity=2)
