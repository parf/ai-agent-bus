#!/usr/bin/env python3
"""Runs only inside install-md-container.sh's disposable systemd container.

The Install, Check, Try it and Update commands are the fenced blocks of INSTALL.md itself, run as printed by an
ordinary user with sudo; the only things this fixture adds are the prerequisites the document lists (git, Go with
gcc, Bun at /usr/bin/bun, an SSH key) and the redirection of the document's GitHub URL to this commit.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import time
import unittest

DOC = Path('/fixture/INSTALL.md')
CHECKOUT = Path('/usr/local/src/ai-agent-bus')
USER = 'dev'
ENV = {'PATH': '/opt/go/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin', 'HOME': f'/home/{USER}', 'USER': USER,
       'GOFLAGS': '-mod=mod', 'GOPROXY': 'off', 'GOMODCACHE': '/gomod', 'GOCACHE': '/tmp/gocache', 'GOTOOLCHAIN': 'local',
       'BUN_INSTALL_CACHE_DIR': '/bun-cache', 'CGO_ENABLED': '1', 'LANG': 'C.UTF-8'}


def blocks(heading):
    """The fenced sh blocks under `### heading`, in order, exactly as the document prints them."""
    text = DOC.read_text()
    start = re.search(rf'^### {re.escape(heading)}\n', text, re.M)
    if not start:
        raise AssertionError(f'INSTALL.md has no "### {heading}" section')
    rest = text[start.end():]
    end = re.search(r'^#{2,3} ', rest, re.M)
    section = rest[:end.start()] if end else rest
    found = re.findall(r'```sh\n(.*?)```', section, re.S)
    if not found:
        raise AssertionError(f'"### {heading}" has no sh block')
    return found


def as_user(script, timeout=900):
    """The block through `bash -e`, as the unprivileged user: a failing command ends it, as at a terminal with set -e."""
    return subprocess.run(['runuser', '-u', USER, '--', 'env', '-i', *[f'{k}={v}' for k, v in ENV.items()],
                           'bash', '-euo', 'pipefail', '-c', script],
                          capture_output=True, text=True, timeout=timeout, cwd=f'/home/{USER}')


class InstallFromACheckout(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not Path('/run/.containerenv').exists() or os.geteuid() != 0:
            raise AssertionError('refusing outside the rootless disposable container')
        run = lambda *a: subprocess.run(a, check=True, capture_output=True, text=True)
        run('useradd', '-m', '-s', '/bin/bash', USER)
        Path(f'/etc/sudoers.d/{USER}').write_text(f'{USER} ALL=(ALL) NOPASSWD: ALL\n')
        Path(f'/etc/sudoers.d/{USER}').chmod(0o440)
        # The prerequisites the document lists: Bun at /usr/bin/bun, an SSH key for the first user.
        run('install', '-m', '0755', '/opt/bun/bun', '/usr/bin/bun')
        run('runuser', '-u', USER, '--', 'ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', f'/home/{USER}/.ssh/id_ed25519')
        # The document clones GitHub; this container serves it this commit instead of reaching the network.
        run('git', 'config', '--system', 'url./source.git.insteadOf', 'https://github.com/parf/ai-agent-bus')
        run('git', 'config', '--system', '--add', 'safe.directory', '*')
        run('git', 'config', '--system', 'user.name', 'fixture')
        run('git', 'config', '--system', 'user.email', 'fixture@install-md.test')
        cls.install, = blocks('Install')
        cls.check, = blocks('Check')
        cls.try_serve, cls.try_call = [b for b in blocks('Try it') if 'agent-bus' in b]
        cls.update, = blocks('Update')

    def sh(self, script, **kw):
        done = as_user(script, **kw)
        return done

    def test_1_the_install_block_builds_and_sets_up_a_node(self):
        done = self.sh(self.install)
        self.assertEqual(done.returncode, 0, f'the Install block failed:\n{done.stdout[-1500:]}\n{done.stderr[-2500:]}')
        for program in ('agent-busd', 'agent-bus', 'agent-bus-runner'):
            self.assertTrue(os.access(CHECKOUT / 'src' / program, os.X_OK), f'src/{program} was not built')
        self.assertTrue(Path('/usr/local/bin/agent-bus').is_symlink(), 'git-install.sh did not link agent-bus into /usr/local/bin')
        self.assertEqual(os.readlink('/usr/local/bin/agent-bus'), str(CHECKOUT / 'src/agent-bus'))

    def test_2_the_check_block_finds_both_services_active(self):
        done = self.sh(self.check)
        self.assertEqual(done.returncode, 0, done.stdout[-800:] + done.stderr[-800:])
        self.assertEqual(done.stdout.split()[-2:], ['active', 'active'], done.stdout[-400:])

    def test_3_the_try_it_blocks_serve_a_script_and_answer_a_call(self):
        version = self.sh('agent-bus --version')
        self.assertEqual(version.returncode, 0, version.stderr)
        want = (CHECKOUT / 'src/internal/version/VERSION').read_text().strip()
        self.assertEqual(version.stdout.splitlines()[0], want, 'the installed agent-bus is not this checkout\'s version')
        self.assertRegex(version.stdout, r'build_info: dev@', 'the build carries no stamp from the build.sh the document runs')
        server = subprocess.Popen(['runuser', '-u', USER, '--', 'env', '-i', *[f'{k}={v}' for k, v in ENV.items()],
                                   'bash', '-euo', 'pipefail', '-c', self.try_serve],
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, cwd=f'/home/{USER}', start_new_session=True)
        self.addCleanup(lambda: (os.killpg(server.pid, 15), server.wait(10)))
        reply = None
        for _ in range(30):
            done = self.sh(self.try_call, timeout=60)
            if done.returncode == 0 and done.stdout.strip():
                reply = done.stdout
                break
            self.assertIsNone(server.poll(), f'the served script exited: {server.stderr.read()}')
            time.sleep(1)
        self.assertIsNotNone(reply, f'the call block never got an answer: {done.stdout} {done.stderr}')
        body = json.loads(reply.strip().splitlines()[-1])
        self.assertEqual((body['from'], body['body']), ('#hi', 'hi: ping'))

    def test_4_the_update_block_pulls_builds_and_restarts(self):
        marker = 'UPDATED-BY-THE-FIXTURE'
        # A newer commit in the served repository, so that an update that does not pull cannot pass.
        work = Path('/tmp/source-work')
        subprocess.run(['git', 'clone', '-q', '/source.git', str(work)], check=True)
        (work / 'UPDATE_MARKER').write_text(marker + '\n')
        subprocess.run(['git', '-C', str(work), 'add', 'UPDATE_MARKER'], check=True)
        subprocess.run(['git', '-C', str(work), 'commit', '-q', '-m', 'a newer commit'], check=True)
        subprocess.run(['git', '-C', str(work), 'push', '-q', 'origin', 'HEAD'], check=True)
        before = subprocess.run(['systemctl', 'show', '-P', 'ActiveEnterTimestampMonotonic', 'agent-busd'], capture_output=True, text=True).stdout
        done = self.sh(self.update)
        self.assertEqual(done.returncode, 0, f'the Update block failed:\n{done.stdout[-1000:]}\n{done.stderr[-2500:]}')
        self.assertEqual((CHECKOUT / 'UPDATE_MARKER').read_text().strip(), marker, 'the update did not pull the newer commit')
        after = subprocess.run(['systemctl', 'show', '-P', 'ActiveEnterTimestampMonotonic', 'agent-busd'], capture_output=True, text=True).stdout
        self.assertNotEqual(before, after, 'agent-busd was not restarted')
        for _ in range(100):
            if subprocess.run(['systemctl', 'is-active', '--quiet', 'agent-busd', 'agent-bus-web']).returncode == 0:
                break
            time.sleep(0.2)
        status = self.sh('agent-bus status')
        self.assertEqual(status.returncode, 0, status.stderr)


if __name__ == '__main__':
    unittest.main(verbosity=2)
