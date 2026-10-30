#!/usr/bin/env python3
"""Runs only inside the disposable npm systemd fixture."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import threading
import time
import unittest

sys.path.insert(0, '/fixture')
from registry import registry


class ExplicitSetup(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not Path('/run/.containerenv').exists() or os.geteuid() != 0:
            raise AssertionError('refusing outside the rootless disposable container')
        cls.env = {**os.environ, 'npm_config_cache': '/evidence/cache', 'npm_config_update_notifier': 'false'}
        cls.server = registry(Path('/packages'))
        cls.addClassCleanup(cls.server.server_close)
        cls.addClassCleanup(cls.server.shutdown)
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()
        # No lifecycle scripts, and no global /usr/local bin conflicts with
        # the ordinary installer. Only the bus is requested from the registry.
        result = subprocess.run(['npm', 'install', '-g', '--prefix', '/opt/npm', '--ignore-scripts',
                                 '--no-audit', '--no-fund', '--registry', f'http://127.0.0.1:{cls.server.server_port}',
                                 'agent-bus'], env=cls.env, capture_output=True, text=True)
        if result.returncode:
            raise AssertionError(result.stderr)
        cls.bin = Path('/opt/npm/bin')
        cls.bus = Path('/opt/npm/lib/node_modules/agent-bus')
        cls.metadata = json.loads((cls.bus / 'package.json').read_text())
        cls.version = cls.metadata['version']
        resolve = lambda name, where: Path(subprocess.check_output(['node', '-e',
            'console.log(require.resolve(process.argv[1]+"/package.json",{paths:[process.argv[2]]}))', name, str(where)], text=True).strip())
        cls.client_meta = resolve(cls.metadata['agentBus']['client'], cls.bus)
        arch = 'linux-arm64' if os.uname().machine == 'aarch64' else 'linux-x64'
        cls.platform = resolve(cls.metadata['agentBus']['platforms'][arch], cls.bus).parent / 'bundle'
        cls.client_platform = resolve(cls.metadata['agentBus']['clientPlatforms'][arch], cls.client_meta.parent).parent / 'bundle'
        subprocess.run(['useradd', '-m', '-s', '/bin/bash', 'alice'], check=True)
        Path('/etc/sudoers.d/alice').write_text('alice ALL=(ALL) NOPASSWD: ALL\n')
        Path('/etc/sudoers.d/alice').chmod(0o440)
        subprocess.run(['runuser', '-u', 'alice', '--', 'ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', '/home/alice/.ssh/id_ed25519'], check=True)

    def setup_command(self, *options):
        return subprocess.run([str(self.bin / 'agent-bus-setup'), *options], env=self.env,
                              capture_output=True, text=True, timeout=45)

    def assert_clean_stage(self):
        self.assertEqual(list(Path('/run').glob('agent-bus-npm-*')), [], 'root staging tree leaked')

    def test_01_versions_and_changed_byte_refused_before_setup(self):
        # A changed full-only file must fail the copied-byte check, even when
        # setup itself could otherwise handle --dry-run without changing state.
        target = self.platform / 'agent-busd'
        original = target.read_bytes()
        try:
            with target.open('ab') as stream:
                stream.write(b'changed-by-fixture')
            refused = self.setup_command('--dry-run', '--owner', 'alice')
            self.assertNotEqual(refused.returncode, 0)
            self.assertIn('manifest mismatch', refused.stderr)
            self.assertNotIn('Would', refused.stdout, 'setup ran before verifying all files')
            self.assertFalse(Path('/var/lib/agent-bus').exists())
            self.assert_clean_stage()
        finally:
            target.write_bytes(original)
        original = self.client_meta.read_text()
        try:
            value = json.loads(original); value['version'] = '0.0.0'
            self.client_meta.write_text(json.dumps(value))
            refused = self.setup_command('--dry-run', '--owner', 'alice')
            self.assertNotEqual(refused.returncode, 0)
            self.assertIn('versions differ', refused.stderr)
            self.assert_clean_stage()
        finally:
            self.client_meta.write_text(original)

    def test_02_private_root_stage_matches_the_full_manifest(self):
        # Inspect the exact staging function in the real root namespace; no
        # production knob weakens ownership checks for a unit-test fixture.
        result = subprocess.run(['node', '-e', '''
          const fs=require('node:fs');
          const staged=require(process.argv[1])(process.argv[2],process.argv[3]);
          try {const st=fs.statSync(staged.root);console.log(JSON.stringify({root:staged.root,uid:st.uid,mode:st.mode&511}));}
          finally {staged.cleanup();}
        ''', str(self.bus / 'setup.cjs'), str(self.client_platform), str(self.platform)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        stage = json.loads(result.stdout)
        self.assertEqual(stage['uid'], 0)
        self.assertEqual(stage['mode'], 0o700)
        self.assertTrue(stage['root'].startswith('/run/agent-bus-npm-'))
        self.assertNotIn('/opt/npm', stage['root'])
        self.assert_clean_stage()
        failed = self.setup_command('--not-a-setup-option')
        self.assertNotEqual(failed.returncode, 0, 'invalid setup arguments unexpectedly succeeded')
        self.assert_clean_stage()

    def test_03_clean_host_installs_starts_and_answers(self):
        self.assertIsNone(shutil.which('go'))
        self.assertIsNone(shutil.which('bun'))
        self.assertFalse(Path('/var/lib/agent-bus').exists())
        result = subprocess.run(['runuser', '-u', 'alice', '--', 'sudo', str(self.bin / 'agent-bus-setup'),
                                 '--owner', 'alice', '--user', 'alice=alice', '--tls', 'off'], capture_output=True, text=True, timeout=90)
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assert_clean_stage()
        active = subprocess.run(['systemctl', 'is-active', '--quiet', 'agent-busd'])
        self.assertEqual(active.returncode, 0, 'npm setup did not start the daemon')
        self.assertEqual(subprocess.check_output(['/usr/local/bin/agent-busd', '--version'], text=True).splitlines()[0], self.version)
        # Keep exercising the npm prefix commands after setup installed its own
        # links: these must use the npm client, not pass via /usr/local/bin.
        cli = str(self.bin / 'agent-bus')
        def alice(*args):
            result = subprocess.run(['runuser', '-u', 'alice', '--', cli, *args], capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            return result.stdout
        status = json.loads(alice('status'))
        self.assertTrue(status['you'].startswith('alice'))
        for name in ('#asker@npm-fixture.test', '#worker@npm-fixture.test'):
            alice('register', name, '--kind', 'agent', '--allow', '*')
        tokens = {name: alice('token', name).strip() for name in ('#asker@npm-fixture.test', '#worker@npm-fixture.test')}
        def call(name, *args):
            env = {**self.env, 'AGENT_BUS_ADDR': '/run/agent-bus/bus.sock', 'AGENT_BUS_TOKEN': tokens[name], 'AGENT_BUS_NAME': name}
            result = subprocess.run([cli, *args], env=env, capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            return result.stdout
        call('#asker@npm-fixture.test', 'send', '#worker@npm-fixture.test', '--topic', 'npm', '--tag', 'one', 'npm setup question')
        message = json.loads(call('#worker@npm-fixture.test', 'consume', '--wait', '1s'))
        self.assertEqual(message['body'], 'npm setup question')
        call('#worker@npm-fixture.test', 'reply', message['message_id'], 'npm setup answer')
        self.assertEqual(json.loads(call('#asker@npm-fixture.test', 'consume', '--wait', '1s'))['body'], 'npm setup answer')
        requests = [
            {'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'npm-systemd', 'version': '1'}}},
            {'jsonrpc': '2.0', 'method': 'notifications/initialized'},
            {'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call', 'params': {'name': 'ab_ls', 'arguments': {'kind': 'agent'}}},
        ]
        env = {**self.env, 'AGENT_BUS_ADDR': '/run/agent-bus/bus.sock', 'AGENT_BUS_TOKEN': tokens['#asker@npm-fixture.test'], 'AGENT_BUS_NAME': '#asker@npm-fixture.test'}
        process = subprocess.Popen([str(self.bin / 'agent-bus-mcp')], env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            process.stdin.write(('\n'.join(map(json.dumps, requests))+'\n').encode()); process.stdin.flush()
            import select
            until = time.monotonic()+10; buffered=b''; answer=None
            while time.monotonic()<until and answer is None:
                ready,_,_=select.select([process.stdout],[],[],max(0,until-time.monotonic()))
                if not ready:break
                chunk=os.read(process.stdout.fileno(),65536)
                if not chunk:break
                buffered+=chunk
                while b'\n' in buffered:
                    line,buffered=buffered.split(b'\n',1); value=json.loads(line)
                    if value.get('id')==2:answer=value
            self.assertIsNotNone(answer,'npm MCP did not answer on the installed daemon')
            self.assertNotIn('error',answer)
            self.assertFalse(answer.get('result',{}).get('isError',False),answer)
            self.assertIn('#worker@npm-fixture.test',json.dumps(answer))
        finally:
            process.stdin.close()
            try:process.wait(timeout=2)
            except subprocess.TimeoutExpired:process.kill();process.wait()
            process.stdout.close();process.stderr.close()


if __name__ == '__main__':
    unittest.main(verbosity=2)
