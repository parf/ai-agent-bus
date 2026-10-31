"""Local command stubs only: never contacts a Mac or invokes sudo."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest


STUB = r'''
import importlib.util, json, subprocess, sys
spec = importlib.util.spec_from_file_location('mac2', sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
mode = sys.argv[2]
present = {'Users': mode != 'absent', 'Groups': mode != 'absent'}
def run(args, **kw):
    args = list(map(str, args))
    print('STUB ' + json.dumps(args), flush=True)
    code, out, err = 0, '', ''
    if args[0] == '/usr/bin/dscl':
        node = args[-1].strip('/').split('/')[0]
        if '-delete' in args:
            code, err = 40, 'eDSPermissionError -14120'
        elif mode == 'unreadable':
            code, err = 40, 'directory unavailable'
        elif '-list' in args:
            out = m.ROLE + '\n' if present[node] else ''
        else:
            code = 0 if present[node] else 185
    elif args[0] in ('/usr/sbin/sysadminctl', '/usr/sbin/dseditgroup'):
        node = 'Users' if args[0].endswith('sysadminctl') else 'Groups'
        if mode == 'user-removes-group' and node == 'Users':
            present['Users'] = present['Groups'] = False
        elif mode == 'group-disappears' and node == 'Groups':
            present[node] = False
            code, err = 64, 'group already absent'
        elif mode == 'fail' or mode == 'group-fail' and node == 'Groups':
            code, err = 1, 'eDSPermissionError -14120'
        elif mode != 'false-success':
            present[node] = False
    p = subprocess.CompletedProcess(args, code, out, err)
    if kw.get('check', True) and code:
        raise RuntimeError(err)
    return p
m.run = run
m.sudo = lambda *args, **kw: run(list(args), **kw)
m.root_exists = lambda path: False
if mode.startswith('links-'):
    class Link:
        def __str__(self): return '/usr/local/bin/agent-bus'
        def is_symlink(self): return True
    link = Link()
    m.COMMAND_LINKS = {link: '/fresh/current/agent-bus'}
    m.root_exists = lambda path: path is link
    m.os.readlink = lambda path: '/other/agent-bus' if mode == 'links-other-target' else '/fresh/current/agent-bus'
m.uninstall([])
'''


class Cleanup(unittest.TestCase):
    def run_cleanup(self, mode):
        return subprocess.run([sys.executable, '-c', STUB,
                               str(Path(__file__).with_name('mac2-install-check.py')), mode],
                              capture_output=True, text=True, timeout=5)

    def test_fresh_full_archive_command_link_is_removed(self):
        p = self.run_cleanup('links-success')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('["/bin/rm", "-f", "/usr/local/bin/agent-bus"]', p.stdout)
        self.assertIn('PASS uninstall', p.stdout)

    def test_different_command_target_refuses_cleanup(self):
        p = self.run_cleanup('links-other-target')
        self.assertNotEqual(p.returncode, 0, p.stdout)
        self.assertIn('unexpected command link; refusing removal', p.stderr)
        self.assertNotIn('["/bin/rm", "-f", "/usr/local/bin/agent-bus"]', p.stdout)
        self.assertNotIn('PASS uninstall', p.stdout)

    def test_removes_user_and_group_with_supported_tools(self):
        p = self.run_cleanup('success')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('["/usr/sbin/sysadminctl", "-deleteUser", "_agentbusd"]', p.stdout)
        self.assertIn('["/usr/sbin/dseditgroup", "-o", "delete", "_agentbusd"]', p.stdout)
        self.assertNotIn('"-delete"', p.stdout)
        self.assertIn('PASS uninstall', p.stdout)

    def test_failure_names_both_leftovers_and_recovery_and_exits_nonzero(self):
        for mode in ('fail', 'false-success', 'unreadable'):
            with self.subTest(mode=mode):
                p = self.run_cleanup(mode)
                self.assertNotEqual(p.returncode, 0, p.stdout)
                self.assertNotIn('PASS uninstall', p.stdout)
                for text in ('user _agentbusd', 'group _agentbusd', 'Full Disk Access',
                             'System Settings', 'sysadminctl', 'dseditgroup'):
                    self.assertIn(text, p.stdout + p.stderr)

    def test_group_failure_does_not_claim_user_is_left(self):
        p = self.run_cleanup('group-fail')
        self.assertNotEqual(p.returncode, 0, p.stdout)
        self.assertIn('leftover group _agentbusd', p.stderr)
        self.assertNotIn('leftover user _agentbusd', p.stderr)
        self.assertNotIn('PASS uninstall', p.stdout)

    def test_already_absent_roles_need_no_delete(self):
        p = self.run_cleanup('absent')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertNotIn('STUB ["/usr/sbin/', p.stdout)

    def test_user_deletion_also_removes_group(self):
        p = self.run_cleanup('user-removes-group')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('["/usr/sbin/sysadminctl", "-deleteUser", "_agentbusd"]', p.stdout)
        self.assertNotIn('STUB ["/usr/sbin/dseditgroup"', p.stdout)
        self.assertIn('PASS uninstall', p.stdout)

    def test_group_already_gone_after_nonzero_delete_is_success(self):
        p = self.run_cleanup('group-disappears')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('STUB ["/usr/sbin/dseditgroup"', p.stdout)
        self.assertIn('PASS uninstall', p.stdout)


class Archive(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('mac2_archive', Path(__file__).with_name('mac2-install-check.py'))
        self.m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.m)
        scratch = Path(__file__).resolve().parents[2] / 'tmp'
        scratch.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.cpu = {'arm64': 'aarch64', 'x86_64': 'x86_64'}[self.m.platform.machine()]

    def bundle(self, unchecked=False, badlink=False, example=False):
        name = 'agent-bus-1.2.3-darwin-' + self.cpu
        root = self.work / name
        root.mkdir()
        for program in ('agent-bus', 'agent-busd', 'agent-bus-admin', 'agent-bus-runner', 'agent-bus-setup', 'agent-bus-ts'):
            path = root / program
            path.write_text('fixture executable')
            path.chmod(0o755)
        (root / 'internal/version').mkdir(parents=True)
        (root / 'internal/version/VERSION').write_text('1.2.3\n')
        (root / 'MANIFEST.sha256').write_text(''.join(
            hashlib.sha256(p.read_bytes()).hexdigest() + '  ' + str(p.relative_to(root)) + '\n'
            for p in sorted(root.rglob('*')) if p.is_file()))
        if unchecked:
            (root / 'unchecked').write_text('not in manifest')
        if example:
            (root / 'launchers').mkdir()
            (root / 'launchers/review-settings.json.dist').write_text('non-runtime example')
        if badlink:
            (root / 'ab-claude').symlink_to('../../outside')
        archive = self.work / (name + '.tar.gz')
        with tarfile.open(archive, 'w:gz') as tar:
            tar.add(root, arcname=name)
        Path(str(archive) + '.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive.name + '\n')
        output = self.work / 'unpack'
        output.mkdir()
        return archive, output

    def test_native_archive_builds_match_the_source_revision(self):
        revision = 'a' * 40
        queries = []
        def run(args, **kw):
            queries.append((args[0].name, args[1:]))
            return subprocess.CompletedProcess(args, 0, '1.2.3\n' if args[0].name == 'agent-bus-ts' and args[1] != 'web' else '1.2.3\nbuild_info: u@h 2026-01-01 00:00:00 ' + revision + '\n', '')
        self.m.run = run
        self.m.check_archive_build(self.work, '1.2.3', revision)
        self.assertEqual(len(queries), 8)
        self.assertIn(('agent-bus-ts', ['mcp', '--version']), queries)
        self.assertIn(('agent-bus-ts', ['launcher', 'claude', '--version']), queries)

    def test_real_typescript_version_answers_are_accepted(self):
        revision = 'a' * 40
        source = Path(__file__).resolve().parents[1] / 'ts-main.ts'
        env = {key: value for key, value in os.environ.items() if not key.startswith('AGENT_BUS_')}
        env.update(AGENT_BUS_BUILD_VERSION='1.2.3',
                   AGENT_BUS_BUILD_INFO='fixture@host 2026-01-01 00:00:00 ' + revision)
        answers = {}
        def run(args, **kw):
            if args[0].name == 'agent-bus-ts':
                result = subprocess.run(['bun', '--tsconfig-override', str(source.parent / 'web/tsconfig.json'), str(source), *args[1:]], env=env,
                                        capture_output=True, text=True, timeout=10, check=True)
                answers[args[1]] = result.stdout
                return result
            return subprocess.CompletedProcess(args, 0,
                '1.2.3\nbuild_info: fixture@host 2026-01-01 00:00:00 ' + revision + '\n', '')
        self.m.run = run
        self.m.check_archive_build(self.work, '1.2.3', revision)
        self.assertEqual(answers['mcp'], '1.2.3\n')
        self.assertEqual(answers['launcher'], '1.2.3\n')
        self.assertIn('build_info: fixture@host 2026-01-01 00:00:00 ' + revision, answers['web'])

    def test_native_archive_of_the_same_version_from_another_revision_is_refused(self):
        revision = 'a' * 40
        for wrong in ('agent-bus', 'agent-busd', 'agent-bus-admin', 'agent-bus-runner', 'agent-bus-setup', 'web'):
            with self.subTest(program=wrong):
                def run(args, **kw):
                    program = args[1] if args[0].name == 'agent-bus-ts' else args[0].name
                    stamp = 'b' * 40 if program == wrong else revision
                    return subprocess.CompletedProcess(args, 0, '1.2.3\n' if args[0].name == 'agent-bus-ts' and args[1] != 'web' else '1.2.3\nbuild_info: u@h 2026-01-01 00:00:00 ' + stamp + '\n', '')
                self.m.run = run
                with self.assertRaisesRegex(AssertionError, 'archive revision differs from source'):
                    self.m.check_archive_build(self.work, '1.2.3', revision)

    def test_native_archive_without_a_full_revision_stamp_is_refused(self):
        for stamp in ('', 'a' * 8, 'b' * 40 + ' ' + 'a' * 40 + 'extra'):
            with self.subTest(stamp=stamp):
                self.m.run = lambda args, **kw: subprocess.CompletedProcess(args, 0, '1.2.3\nbuild_info: u@h 2026-01-01 00:00:00 ' + stamp + '\n', '')
                with self.assertRaisesRegex(AssertionError, 'archive revision differs from source'):
                    self.m.check_archive_build(self.work, '1.2.3', 'a' * 40)

    def test_full_archive_checks_all_delivered_bytes_before_install(self):
        archive, output = self.bundle()
        root, version = self.m.archive_bundle(archive, output)
        self.assertEqual(version, '1.2.3')
        self.assertTrue((root / 'agent-busd').is_file())

    def test_existing_nonruntime_example_is_allowed(self):
        archive, output = self.bundle(example=True)
        root, version = self.m.archive_bundle(archive, output)
        self.assertEqual(version, '1.2.3')
        self.assertTrue((root / 'launchers/review-settings.json.dist').is_file())

    def test_corrupt_archive_refused_before_extract(self):
        archive, output = self.bundle()
        archive.write_bytes(archive.read_bytes() + b'changed')
        with self.assertRaisesRegex(AssertionError, 'archive checksum mismatch'):
            self.m.archive_bundle(archive, output)
        self.assertFalse((output / 'archive').exists())

    def test_unchecked_archive_member_refused(self):
        archive, output = self.bundle(unchecked=True)
        with self.assertRaisesRegex(AssertionError, 'unchecked files'):
            self.m.archive_bundle(archive, output)

    def test_archive_link_escape_refused_before_extract(self):
        archive, output = self.bundle(badlink=True)
        with self.assertRaisesRegex(AssertionError, 'unsafe archive link'):
            self.m.archive_bundle(archive, output)

    def test_client_archive_refused(self):
        archive, output = self.bundle()
        client = archive.with_name(archive.name.replace('agent-bus-', 'agent-bus-client-', 1))
        archive.rename(client)
        with self.assertRaisesRegex(AssertionError, 'full Darwin archive'):
            self.m.archive_bundle(client, output)


if __name__ == '__main__':
    unittest.main()
