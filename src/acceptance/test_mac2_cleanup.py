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
from pathlib import Path
spec = importlib.util.spec_from_file_location('mac2', sys.argv[1])
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
mode = sys.argv[2]
action, mode = mode.split(':', 1)
present = mode != 'absent'
attribute_mode = mode.removeprefix('user-only-')
user_only = mode == 'missing-group' or mode.startswith('user-only-')
id_value = {'uid-zero':'0','uid-199':'199','uid-400':'400'}.get(attribute_mode,'200')
paths = {str(p): False for p in (m.HOME, m.RUN, m.PREFIX, m.PLIST)}
link = Path('/usr/local/bin/agent-bus')
m.COMMAND_LINKS = {link: Path('/fresh/current/agent-bus')}
if mode.startswith('leftover-'):
    paths[str({'home':m.HOME,'run':m.RUN,'prefix':m.PREFIX,'plist':m.PLIST,'link':link,'web':Path('/usr/local/bin/agent-bus-web')}[mode[9:]])] = True
if mode.startswith('links-'): paths[str(link)] = True
if mode=='path-remains': paths[str(m.HOME)] = True
booted_out = [False]
def run(args, **kw):
    args = list(map(str,args)); print('STUB ' + json.dumps(args),flush=True)
    code,out,err = 0,'',''
    if args[0]=='/usr/bin/dscl':
        node=args[-1].strip('/').split('/')[0]
        if mode=='unreadable': code,err=40,'directory unavailable'
        elif '-search' in args:
            owner = 'other-group' if mode=='gid-clash' else m.ROLE
            out = owner+'\t\tPrimaryGroupID = (\n    '+id_value+'\n)\n' if not user_only else ''
            if mode=='one-line-search': out=owner+' PrimaryGroupID: '+id_value+'\n'
        elif 'GroupMembership' in args:
            privileged = args[-2].split('/')[-1]
            member = m.ROLE if attribute_mode == privileged+'-member' else 'root'
            out = 'GroupMembership: '+member+'\n'
            if attribute_mode==privileged+'-read-error': code=40
            if attribute_mode==privileged+'-stderr': err='directory warning'
            if attribute_mode==privileged+'-no-membership': out='RecordName: '+privileged+'\n'
        elif '-list' in args:
            exists=present and not ((user_only or mode=='gid-clash') and node=='Groups') and not (mode=='group-only' and node=='Users')
            out=m.ROLE+'\n' if exists else ''
        elif node=='Users':
            fields={'UserShell':'/usr/bin/false','IsHidden':'1','UniqueID':id_value,'PrimaryGroupID':id_value,'NFSHomeDirectory':str(m.HOME)}
            changes={'login':('UserShell','/bin/zsh'),'visible':('IsHidden','0'),'uid':('UniqueID','501'),'gid':('PrimaryGroupID','501'),'home':('NFSHomeDirectory','/Users/parf'),'bad-id':('UniqueID','abc')}
            if mode=='teardown-unsafe' and booted_out[0]: fields['UserShell']='/bin/zsh'
            if attribute_mode in changes:
                key,value=changes[attribute_mode];fields[key]=value
            out=''.join(('dsAttrTypeNative:' if k=='IsHidden' else '')+k+': '+v+'\n' for k,v in fields.items())
        else: out='PrimaryGroupID: '+('201' if mode=='wrong-group' else id_value)+'\n'
    elif args[:2]==['/bin/launchctl','print']:
        code=0 if mode=='job' or mode=='job-remains' else 113
    elif args[:2]==['/bin/launchctl','bootout']: booted_out[0]=True
    elif args[0]=='/bin/rm':
        for path in args[2:]:
            if mode!='path-remains': paths[path]=False
    elif args[0] in ('/usr/sbin/sysadminctl','/usr/sbin/dseditgroup'):
        raise AssertionError('test must keep the role account')
    result=subprocess.CompletedProcess(args,code,out,err)
    if kw.get('check',True) and code:raise RuntimeError(err)
    return result
m.run=run
m.sudo=lambda *args,**kw:run(list(args),**kw)
m.root_exists=lambda path: paths.get(str(path),mode=='path-remains' and path==m.HOME)
m.Path.is_symlink=lambda path:True
m.os.readlink=lambda path:'/other/agent-bus' if mode=='links-other-target' else '/fresh/current/agent-bus'
def acceptance():
    try:
        if mode=='prior-failure': raise AssertionError('earlier acceptance failed')
    finally: m.uninstall_after_checks([])
if action=='preflight':m.preflight(True)
else:
    m.main=acceptance
    m.cli()
'''


class Cleanup(unittest.TestCase):
    def stub(self, mode, action='cleanup'):
        return subprocess.run([sys.executable,'-B','-c',STUB,str(Path(__file__).with_name('mac2-install-check.py')),action+':'+mode],capture_output=True,text=True,timeout=5)

    def test_preflight_accepts_expected_hidden_nonlogin_role(self):
        p=self.stub('valid','preflight')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)
        self.assertIn('safe to reuse',p.stdout)
        self.assertIn('"-read", "/Users/_agentbusd"',p.stdout)
        self.assertIn('"-read", "/Groups/_agentbusd"',p.stdout)

    def test_preflight_accepts_one_line_search_output(self):
        p=self.stub('one-line-search','preflight')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)

    def test_preflight_accepts_absent_role_pair(self):
        p=self.stub('absent','preflight')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)
        self.assertNotIn('"-read"',p.stdout)

    def test_preflight_refuses_login_or_visible_account(self):
        for mode,field in [('login','UserShell'),('visible','IsHidden')]:
            with self.subTest(mode=mode):
                p=self.stub(mode,'preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)
                self.assertIn(field,p.stderr)

    def test_preflight_refuses_wrong_ids_home_or_partial_pair(self):
        for mode in ('uid','uid-zero','uid-199','uid-400','gid','bad-id','home','wrong-group','unreadable'):
            with self.subTest(mode=mode):
                p=self.stub(mode,'preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)

    def test_preflight_accepts_safe_user_without_group(self):
        p=self.stub('missing-group','preflight')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)
        self.assertIn('safe to reuse',p.stdout)
        self.assertIn('"-search", "/Groups", "PrimaryGroupID", "200"',p.stdout)

    def test_user_only_still_requires_all_role_safety_checks(self):
        for mode in ('login','visible','uid-zero','uid-199','uid-400','gid','bad-id','home','admin-member','wheel-member'):
            with self.subTest(mode=mode):
                p=self.stub('user-only-'+mode,'preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)

    def test_teardown_requires_the_complete_role_pair(self):
        p=self.stub('missing-group')
        self.assertEqual(p.returncode,1,p.stdout+p.stderr)
        self.assertIn('incomplete role account/group pair',p.stderr)
        self.assertNotIn('role account kept (by design)',p.stdout)

    def test_preflight_refuses_group_without_user(self):
        p=self.stub('group-only','preflight')
        self.assertNotEqual(p.returncode,0,p.stdout)
        self.assertIn('group without role user',p.stderr)

    def test_preflight_refuses_primary_gid_owned_by_another_group(self):
        p=self.stub('gid-clash','preflight')
        self.assertNotEqual(p.returncode,0,p.stdout)
        self.assertIn('different group: other-group',p.stderr)

    def test_privileged_group_read_error_is_refused(self):
        self.assert_privileged_inspection_refused('read-error')

    def test_privileged_group_stderr_is_refused(self):
        self.assert_privileged_inspection_refused('stderr')

    def test_privileged_group_missing_membership_is_refused(self):
        self.assert_privileged_inspection_refused('no-membership')

    def assert_privileged_inspection_refused(self, failure):
        for group in ('admin','wheel'):
            with self.subTest(group=group):
                p=self.stub(group+'-'+failure,'preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)
                self.assertNotIn('safe to reuse',p.stdout)

    def test_preflight_refuses_privileged_group_membership(self):
        for group in ('admin','wheel'):
            with self.subTest(group=group):
                p=self.stub(group+'-member','preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)
                self.assertIn('privileged group '+group,p.stderr)

    def test_teardown_rechecks_kept_account_safety(self):
        p=self.stub('teardown-unsafe')
        self.assertEqual(p.returncode,1,p.stdout+p.stderr)
        self.assertIn('unsafe role UserShell',p.stderr)
        self.assertNotIn('role account kept (by design)',p.stdout)

    def test_preflight_refuses_every_other_leftover(self):
        for mode in ('job',*[ 'leftover-'+key for key in ('home','run','prefix','plist','link','web')]):
            with self.subTest(mode=mode):
                p=self.stub(mode,'preflight')
                self.assertNotEqual(p.returncode,0,p.stdout)
                self.assertTrue('existing launchd job' in p.stderr or 'existing setup path' in p.stderr,p.stderr)

    def test_teardown_keeps_roles_and_exits_zero(self):
        p=self.stub('valid')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)
        self.assertIn('role account kept (by design)',p.stdout)
        self.assertIn('PASS uninstall',p.stdout)
        for removed in ('sysadminctl','dseditgroup','-delete','PARTIAL:'):
            self.assertNotIn(removed,p.stdout+p.stderr)

    def test_fresh_full_archive_command_link_is_removed(self):
        p=self.stub('links-success')
        self.assertEqual(p.returncode,0,p.stdout+p.stderr)
        self.assertIn('["/bin/rm", "-f", "/usr/local/bin/agent-bus"]',p.stdout)

    def test_different_command_target_refuses_cleanup(self):
        p=self.stub('links-other-target')
        self.assertNotEqual(p.returncode,0,p.stdout)
        self.assertIn('unexpected command link; refusing removal',p.stderr)
        self.assertNotIn('PASS uninstall',p.stdout)

    def test_other_teardown_failures_stay_fatal(self):
        for mode in ('path-remains','job-remains'):
            with self.subTest(mode=mode):
                p=self.stub(mode)
                self.assertEqual(p.returncode,1,p.stdout+p.stderr)
                self.assertNotIn('PASS uninstall',p.stdout)

    def test_kept_account_never_masks_earlier_failure(self):
        p=self.stub('prior-failure')
        self.assertEqual(p.returncode,1,p.stdout+p.stderr)
        self.assertIn('earlier acceptance failed',p.stderr)
        self.assertIn('role account kept (by design)',p.stdout)


class DsclOutput(unittest.TestCase):
    def test_real_search_output_and_empty_unused_gid(self):
        spec=importlib.util.spec_from_file_location('mac2_search',Path(__file__).with_name('mac2-install-check.py'))
        m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
        # CEO's read-only Mac output, 2026-11-01, tabs and newlines preserved.
        self.assertEqual(m.group_search_rows('staff\t\tPrimaryGroupID = (\n    20\n)\n'), [('staff','20')])
        # CEO's selected user attributes, in dscl's alphabetical order.
        self.assertEqual(m.directory_fields('dsAttrTypeNative:IsHidden: 1\nNFSHomeDirectory: /usr/local/var/agent-bus/daemon\nPrimaryGroupID: 206\nUserShell: /usr/bin/false\n'),
                         {'IsHidden':'1','NFSHomeDirectory':'/usr/local/var/agent-bus/daemon','PrimaryGroupID':'206','UserShell':'/usr/bin/false'})
        self.assertEqual(m.group_search_rows(''), [])
        self.assertEqual(m.group_search_rows('staff PrimaryGroupID: 20\n'), [('staff','20')])
        with self.assertRaisesRegex(AssertionError,'cannot inspect'):
            m.group_search_rows('staff unexpected output\n')


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
        self.assertEqual(answers['mcp'].splitlines()[0], '1.2.3')
        self.assertEqual(answers['launcher'].splitlines()[0], '1.2.3')
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
