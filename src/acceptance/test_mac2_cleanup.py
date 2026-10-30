"""Local command stubs only: never contacts a Mac or invokes sudo."""
import json
from pathlib import Path
import subprocess
import sys
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
m.uninstall([])
'''


class Cleanup(unittest.TestCase):
    def run_cleanup(self, mode):
        return subprocess.run([sys.executable, '-c', STUB,
                               str(Path(__file__).with_name('mac2-install-check.py')), mode],
                              capture_output=True, text=True, timeout=5)

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


if __name__ == '__main__':
    unittest.main()
