"""Synthetic post-attempt script fixtures; runtime tests follow the core landing.

No live checkout, account, service or external command destination is used.
Fixtures deliberately retain failed-script effects so ordering and process
cleanup checks can assert what actually happened, rather than mock a send.
"""
import copy
import json
import os
from pathlib import Path
import sys


def attempt_record(**changes):
    """The owner-approved result shape with only synthetic, public values."""
    record = {
        'v': 1, 'host': 'host.fixture.test', 'repo': 'fixture', 'branch': 'main',
        'result': 'ok', 'action': 'autopull', 'trigger': 'event', 'mode': 'ff',
        'at': '2026-10-29T12:00:00-04:00', 'agent': '#git@fixture.test',
        'remote': 'fixture/project', 'requested_by': '#pipeline@fixture.test',
        'id': 'pipeline-fixture', 'from': 'a' * 40, 'to': 'b' * 40,
        'requested': 'b' * 40, 'discarded': None, 'script': None,
        'error': None, 'ms': 12,
    }
    record.update(copy.deepcopy(changes))
    return record


class ScriptFixtures:
    """Installed allowlist plus checkout, rooted in a caller-owned temp dir."""
    def __init__(self, base):
        self.base = Path(base)
        self.state = self.base / 'state' / 'agent-bus-git'
        self.allowlist = self.state / 'autopull.d'
        self.checkout = self.base / 'checkout'
        self.journal = self.base / 'script-order.jsonl'
        self.child_effect = self.base / 'late-child-effect'
        self.state.parent.mkdir(mode=0o700)
        self.state.mkdir(mode=0o700)
        self.allowlist.mkdir(mode=0o700)
        self.checkout.mkdir(mode=0o700)
        self.journal.touch(mode=0o600)

    def publish(self, record):
        self.append({'kind': 'published', 'record': copy.deepcopy(record)})

    def append(self, value):
        with self.journal.open('a', encoding='utf-8') as stream:
            stream.write(json.dumps(value, ensure_ascii=False) + '\n')

    def events(self):
        return [json.loads(line) for line in self.journal.read_text().splitlines()]

    def install(self, name, *, exit_code=0, stderr=b'', stdout_size=0,
                delay=0, ignore_term=False, child_delay=None, read_stdin=True, capture_head=False, child_session=False, self_signal=None):
        """Create a real executable, never a shell-expanded command string."""
        assert name and Path(name).name == name and name not in ('.', '..')
        path = self.allowlist / name
        source = '#!' + sys.executable + '\n' + '''import json, os, pathlib, signal, subprocess, sys, time
'''
        if ignore_term:
            source += 'signal.signal(signal.SIGTERM, signal.SIG_IGN)\n'
        source += 'record = json.load(sys.stdin)\n' if read_stdin else 'record = None\n'
        source += 'child = None\n'
        if child_delay is not None:
            child = ('import pathlib,time; time.sleep(' + repr(child_delay) + '); '
                     'pathlib.Path(' + repr(str(self.child_effect)) + ').write_text("late effect")')
            source += 'child = subprocess.Popen([' + repr(sys.executable) + ', "-c", ' + repr(child) + '], start_new_session=' + repr(child_session) + ').pid\n'
        source += 'head = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], text=True).strip()\n' if capture_head else 'head = None\n'
        source += (
            'entry = {"kind":"script", "name":' + repr(name)
            + ', "cwd":os.getcwd(), "uid":os.getuid(), "head":head, "record":record, "child":child, "env":sorted(os.environ), "umask":os.umask(0o077)}\n'
            + 'with open(' + repr(str(self.journal)) + ', "a", encoding="utf-8") as stream:\n'
            + '    stream.write(json.dumps(entry, ensure_ascii=False) + "\\n")\n'
        )
        if stdout_size:
            source += ('for offset in range(0, ' + repr(stdout_size) + ', 65536):\n'
                       '    os.write(1, b"x" * min(65536, ' + repr(stdout_size) + ' - offset))\n')
        if stderr:
            source += 'os.write(2, ' + repr(stderr) + ')\n'
        if delay:
            source += 'time.sleep(' + repr(delay) + ')\n'
        if self_signal is not None:
            source += 'os.kill(os.getpid(), ' + repr(int(self_signal)) + ')\n'
        source += 'sys.exit(' + repr(exit_code) + ')\n'
        path.write_text(source, encoding='utf-8')
        path.chmod(0o700)
        return path

    def alias(self, name, target):
        path = self.allowlist / name
        path.symlink_to(target)
        return path


import tempfile
import time
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import autopull as a
import git_agent as g


class ScriptTests(unittest.TestCase):
    def setUp(self):
        # A private fixture home must be outside the group-writable checkout:
        # the real ancestor trust check, not a mocked stat, is exercised.
        self.temp = tempfile.TemporaryDirectory(prefix='.agent-bus-script-fixture-', dir=Path.home())
        self.addCleanup(self.temp.cleanup)
        self.f = ScriptFixtures(self.temp.name)
        self.env = patch.dict(os.environ, {'XDG_STATE_HOME':str(self.f.state.parent),
                             'AGENT_BUS_TOKEN':'fixture-not-for-children',
                             'SSH_AUTH_SOCK':'fixture-not-for-children', 'GIT_CONFIG_COUNT':'99'})
        self.env.start(); self.addCleanup(self.env.stop)

    def invoke(self, repo, record=None, publish=None):
        record = attempt_record() if record is None else record
        self.f.publish(record)
        a.after_attempt(repo, self.f.checkout, record, publish or self.f.publish)
        return self.f.events()

    def test_success_runs_after_publication_with_exact_record_cwd_uid_and_no_tokens(self):
        self.f.install('success')
        record = attempt_record()
        events = self.invoke({'autopull-success-script':'success'}, record)
        self.assertEqual([e['kind'] for e in events], ['published','script'])
        run = events[1]
        self.assertEqual(run['record'], record)
        self.assertEqual(run['cwd'], str(self.f.checkout))
        self.assertEqual(run['uid'], os.geteuid())
        self.assertNotIn('AGENT_BUS_TOKEN', run['env'])
        self.assertNotIn('SSH_AUTH_SOCK', run['env'])
        self.assertNotIn('GIT_CONFIG_COUNT', run['env'])
        self.assertEqual(run['umask'], 0o077)
        self.assertEqual(record, attempt_record())

    def test_both_failed_scripts_publish_once_each_and_preserve_correlation(self):
        self.f.install('success', exit_code=7)
        self.f.install('fail', exit_code=9)
        events = self.invoke({'autopull-success-script':'success', 'autopull-fail-script':'fail'})
        self.assertEqual([e['kind'] for e in events], ['published','script','published','script','published'])
        first, second = events[2]['record'], events[4]['record']
        self.assertEqual(events[3]['record'], first)
        for record, run, code in ((first,'success',7),(second,'fail',9)):
            self.assertEqual(record['action'], 'script')
            self.assertEqual(record['result'], 'fail')
            self.assertEqual(record['error']['code'], 'script')
            self.assertEqual(record['script']['run'], run)
            self.assertEqual(record['script']['exit'], code)
            for key in ('id','from','to'): self.assertEqual(record[key], attempt_record()[key])
            self.assertNotIn(str(self.f.base), json.dumps(record))

    def test_any_stderr_including_empty_line_fails_and_first_line_is_200_characters(self):
        self.f.install('fail')
        for raw, expected in ((b'\n',''),(b' ',' '),(('я'*250+'\nsecond').encode(),'я'*200)):
            self.f.journal.write_text('')
            self.f.install('success', stderr=raw)
            events = self.invoke({'autopull-success-script':'success', 'autopull-fail-script':'fail'})
            self.assertEqual([e['kind'] for e in events], ['published','script','published','script'])
            self.assertEqual(events[2]['record']['script']['stderr'], expected)
            self.assertEqual(events[2]['record']['script']['exit'], 0)

    def test_stdout_flood_and_stdin_never_read_do_not_block_or_fail(self):
        self.f.install('success', stdout_size=2*1024*1024, read_stdin=False)
        events = self.invoke({'autopull-success-script':'success'}, attempt_record(extra='x'*8000))
        self.assertEqual([e['kind'] for e in events], ['published','script'])
        self.assertIsNone(events[1]['record'])

    def test_timeout_kills_group_and_fires_fail_script_with_null_exit(self):
        self.f.install('success', delay=2, child_delay=1.2, ignore_term=True)
        self.f.install('fail')
        with patch.object(a,'SCRIPT_SECONDS',.5):
            events = self.invoke({'autopull-success-script':'success','autopull-fail-script':'fail'})
        self.assertEqual([e['kind'] for e in events], ['published','script','published','script'])
        self.assertIsNone(events[2]['record']['script']['exit'])
        self.assertIsNotNone(events[1]['child'])  # Capture descendant before cleanup.
        time.sleep(.8)
        self.assertFalse(self.f.child_effect.exists())

    def test_normal_exit_also_cleans_background_pipe_holders(self):
        self.f.install('success', child_delay=.25)
        events = self.invoke({'autopull-success-script':'success'})
        self.assertEqual([e['kind'] for e in events], ['published','script'])
        time.sleep(.3)
        self.assertFalse(self.f.child_effect.exists())

    def test_clean_leader_with_escaped_stderr_holder_is_not_a_timeout(self):
        import signal
        self.f.install('success',child_delay=2,child_session=True)
        self.f.install('fail')
        child=None
        try:
            started=time.monotonic()
            with patch.object(a,'SCRIPT_SECONDS',1):
                events=self.invoke({'autopull-success-script':'success','autopull-fail-script':'fail'})
            child=next(e['child'] for e in events if e['kind']=='script')
            self.assertEqual([e['kind'] for e in events],['published','script'])
            self.assertIsNotNone(child)
            self.assertLess(time.monotonic()-started,.75, 'clean exit waited for an escaped pipe holder')
        finally:
            if child is not None:
                try:os.kill(child,signal.SIGKILL)
                except ProcessLookupError:pass

    def test_skip_has_no_script_and_failed_attempt_runs_only_fail(self):
        self.f.install('success');self.f.install('fail')
        config={'autopull-success-script':'success','autopull-fail-script':'fail'}
        self.assertEqual(len(self.invoke(config, attempt_record(result='skip'))),1)
        self.f.journal.write_text('')
        events=self.invoke(config,attempt_record(result='fail'))
        self.assertEqual([e['kind'] for e in events],['published','script'])
        self.assertEqual(events[1]['name'],'fail')

    def test_no_log_or_refused_log_never_suppresses_scripts(self):
        self.f.install('success', exit_code=1); self.f.install('fail')
        for publish in (lambda _record:True,lambda _record:False):
            self.f.journal.write_text('')
            a.after_attempt({'autopull-success-script':'success','autopull-fail-script':'fail'},
                            self.f.checkout,attempt_record(),publish)
            self.assertEqual([e['name'] for e in self.f.events()],['success','fail'])
        self.f.journal.write_text('')
        def unavailable(_): raise OSError('fixture-private-error')
        with patch('sys.stderr'):
            a.after_attempt({'autopull-success-script':'success','autopull-fail-script':'fail'},
                            self.f.checkout,attempt_record(),unavailable)
        self.assertEqual([e['name'] for e in self.f.events()],['success','fail'])

    def test_configuration_rejects_paths_escape_permissions_and_privilege_bits(self):
        good=self.f.install('good')
        def config(name):
            return json.dumps({'repos':{'fixture':{'path':str(self.f.checkout),'access':'deploy','autopull-success-script':name}}})
        self.assertEqual(g.configuration(config('good'))['repos']['fixture']['autopull-success-script'],'good')
        self.f.alias('inside','good')
        fd=g.open_script('inside');os.close(fd)
        self.f.alias('outside',sys.executable)
        for value in ('../good','/bin/sh','..','a/b','outside','missing','good arg'):
            with self.subTest(value=value), self.assertRaises(ValueError):g.configuration(config(value))
        for mode in (0o720,0o702,0o4700,0o2700):
            good.chmod(mode)
            with self.subTest(mode=mode),self.assertRaises(ValueError):g.configuration(config('good'))
        good.chmod(0o700)
        self.f.allowlist.chmod(0o770)
        with self.assertRaises(ValueError):g.configuration(config('good'))
        self.f.allowlist.chmod(0o700)
        self.assertEqual(self.f.events(),[])

    def test_rename_swap_and_permission_change_are_rechecked_before_execution(self):
        script=self.f.install('good');fd=g.open_script('good')
        try:
            replacement=self.f.install('replacement')
            replacement.replace(script)
            with self.assertRaises(ValueError):g.recheck_script(fd,'good')
        finally:os.close(fd)
        fd=g.open_script('good')
        try:
            script.chmod(0o722)
            with self.assertRaises(ValueError):g.recheck_script(fd,'good')
        finally:os.close(fd)
        self.assertEqual(self.f.events(),[])

    def test_file_removed_after_configuration_is_one_script_failure_not_an_exception(self):
        script=self.f.install('success');self.f.install('fail')
        fd=g.open_script('success');os.close(fd);script.unlink()
        events=self.invoke({'autopull-success-script':'success','autopull-fail-script':'fail'})
        self.assertEqual([e['kind'] for e in events],['published','published','script'])
        failure=events[1]['record']
        self.assertEqual(failure['script']['exit'],127)
        self.assertNotIn(str(script),json.dumps(failure))

    def test_interpreter_alias_in_untrusted_directory_is_refused(self):
        directory = self.f.base / 'interpreter'
        directory.mkdir(mode=0o700)
        (directory / 'python').symlink_to(sys.executable)
        script = self.f.allowlist / 'bad-interpreter'
        script.write_text('#!' + str(directory / 'python') + '\n')
        script.chmod(0o700)
        fd=g.open_script('bad-interpreter');os.close(fd)
        directory.chmod(0o777)
        with self.assertRaises(ValueError):g.open_script('bad-interpreter')

    def test_huge_stderr_is_drained_but_only_first_200_characters_retained(self):
        self.f.install('success', stderr=b'x' * (2*1024*1024))
        events=self.invoke({'autopull-success-script':'success'})
        self.assertEqual([e['kind'] for e in events],['published','script','published'])
        self.assertEqual(events[2]['record']['script']['stderr'],'x'*200)
        self.assertEqual(events[2]['record']['script']['exit'],0)

    def test_trusted_state_directory_alias_and_stderr_controls(self):
        self.f.install('success',stderr=b'\x1b[31mERROR\x1b[0m\x00\x7f\nnext')
        alias=self.f.base/'state-alias'
        alias.symlink_to(self.f.state.parent, target_is_directory=True)
        with patch.dict(os.environ,{'XDG_STATE_HOME':str(alias)}):
            fd=g.open_script('success');os.close(fd)
            events=self.invoke({'autopull-success-script':'success'})
        self.assertEqual(events[2]['record']['script']['stderr'],'ERROR')

    def test_script_fd_survives_name_swap_after_final_recheck(self):
        self.f.install('success');replacement=self.f.install('replacement')
        original=g.recheck_script
        def swap(fd,name):
            original(fd,name)
            replacement.replace(self.f.allowlist/name)
        with patch.object(g,'recheck_script',side_effect=swap):
            events=self.invoke({'autopull-success-script':'success'})
        self.assertEqual([e['kind'] for e in events],['published','script'])
        self.assertEqual(events[1]['name'],'success')

    def test_signal_failure_and_buffered_stderr_of_already_exited_leader(self):
        import signal
        self.f.install('signal',self_signal=signal.SIGTERM)
        events=self.invoke({'autopull-success-script':'signal'})
        self.assertEqual(events[2]['record']['script']['exit'],-signal.SIGTERM)
        self.f.install('late',stderr=b'last line\n',read_stdin=False)
        original=a.subprocess.Popen
        def completed(*args,**kwargs):
            child=original(*args,**kwargs)
            child.wait(timeout=2)
            return child
        with patch.object(a.subprocess,'Popen',side_effect=completed):
            outcome=a.run_script('late',self.f.checkout,attempt_record(),time.monotonic()+2)
        self.assertEqual(outcome['exit'],0)
        self.assertEqual(outcome['stderr'],'last line')

    def test_foreign_owner_special_file_and_dotdot_name_are_refused(self):
        import stat
        from types import SimpleNamespace
        with self.assertRaises(ValueError):
            g._trusted(SimpleNamespace(st_mode=stat.S_IFREG|0o700,st_uid=os.geteuid()+1001))
        self.f.install('a..b')
        with self.assertRaises(ValueError):g.open_script('a..b')
        os.mkfifo(self.f.allowlist/'pipe',0o600)
        with self.assertRaises(ValueError):g.open_script('pipe')

    def test_two_script_deadlines_share_one_hook_budget(self):
        outcome={'exit':7,'stderr':None,'ms':1}
        with patch.object(a,'run_script',return_value=outcome) as run, patch.object(a.time,'monotonic',side_effect=[100,100,135]):
            a.after_attempt({'autopull-success-script':'success','autopull-fail-script':'fail'},self.f.checkout,attempt_record(),self.f.publish)
        self.assertEqual([call.args[3] for call in run.call_args_list],[130,160])
        self.assertEqual([call.args[0] for call in run.call_args_list],['success','fail'])

    def test_native_elf_and_large_input_bounds(self):
        import shutil
        shutil.copyfile('/usr/bin/true',self.f.allowlist/'true')
        (self.f.allowlist/'true').chmod(0o700)
        result=a.run_script('true',self.f.checkout,attempt_record(),time.monotonic()+1)
        self.assertEqual(result['exit'],0)
        self.assertIsNone(result['stderr'])
        result=a.run_script('true',self.f.checkout,attempt_record(extra='x'*20000),time.monotonic()+1)
        self.assertEqual(result['exit'],127)


import test_git as git_fixture


class CoreScriptFlow(unittest.TestCase):
    run_git = git_fixture.GitAgent.run_git
    advance = git_fixture.GitAgent.advance

    def setUp(self):
        home = Path.home()
        git_fixture.GitAgent.setUp(self)
        self.addCleanup(git_fixture.GitAgent.tearDown, self)
        self.scripts_home = tempfile.TemporaryDirectory(prefix='.agent-bus-script-core-',dir=home)
        self.addCleanup(self.scripts_home.cleanup)
        self.f=ScriptFixtures(self.scripts_home.name)
        os.environ['XDG_STATE_HOME']=str(self.f.state.parent)

    def test_real_attempt_publishes_then_runs_script_under_locked_checkout(self):
        import fcntl, hashlib
        new=self.advance()
        self.f.install('success',capture_head=True)
        events=[]
        fixture=self.f
        root=self.repo
        class Bus:
            def publish(self,channel,body,**meta):
                lock=g.state_directory()/(hashlib.sha256(str(root).encode()).hexdigest()+'.lock')
                with lock.open('a') as handle:
                    try:fcntl.flock(handle,fcntl.LOCK_EX|fcntl.LOCK_NB)
                    except BlockingIOError:pass
                    else:raise AssertionError('attempt publication lost the checkout lock')
                record=json.loads(body);events.append((channel,record,meta));fixture.publish(record)
                return 'accepted'
        config={'repos':{'ab':{**self.repos['ab'],'autopull-success-script':'success'}},'autopull-log':'logs'}
        g.configuration(json.dumps(config))
        record=a.attempt('ab',config,bus=Bus(),agent='#git@fixture.test',event_id='pipeline-fixture')
        self.assertEqual(record['to'],new)
        journal=self.f.events()
        self.assertEqual([entry['kind'] for entry in journal],['published','script'])
        self.assertEqual(journal[1]['record'],record)
        self.assertEqual(journal[1]['cwd'],str(self.repo))
        self.assertEqual(journal[1]['head'],new)
        self.assertEqual(len(events),1)
        self.assertEqual(events[0][2]['tag'],'ok')

    def test_drifted_script_does_not_block_git_or_another_repository(self):
        from types import SimpleNamespace
        bad=self.f.install('bad');self.f.install('good',capture_head=True)
        other=self.base/'other'
        self.run_git(self.base,'clone',str(self.up),str(other))
        config={'repos':{
            'ab':{**self.repos['ab'],'autopull-success-script':'bad'},
            'other':{'path':str(other),'access':'commit','autopull-success-script':'good'}},'autopull-log':'logs'}
        g.configuration(json.dumps(config))
        bad.chmod(0o722)
        with self.assertRaises(ValueError):g.configuration(json.dumps(config))
        bus=SimpleNamespace(name='#git@fixture.test',config=config,refresh=lambda *_:None)
        runtime=a.Runtime(bus)
        try:loaded=runtime.config()
        finally:runtime.close()
        new=self.advance()
        records=[];fixture=self.f
        class Publish:
            def publish(self,_channel,body,**_meta):
                value=json.loads(body);records.append(value);fixture.publish(value);return 'accepted'
        first=a.attempt('ab',loaded,bus=Publish())
        second=a.attempt('other',loaded,bus=Publish())
        self.assertEqual((first['to'],second['to']),(new,new))
        self.assertEqual([r['action'] for r in records],['autopull','script','autopull'])
        runs=[row for row in self.f.events() if row['kind']=='script']
        self.assertEqual([row['name'] for row in runs],['good'])
        self.assertEqual(runs[0]['head'],new)

    def test_real_no_log_and_refused_log_still_run_scripts(self):
        import contextlib,io
        self.f.install('success')
        new=self.advance()
        config={'repos':{'ab':{**self.repos['ab'],'autopull-success-script':'success'}}}
        first=a.attempt('ab',config)
        self.assertEqual(first['to'],new)
        self.assertEqual([e['kind'] for e in self.f.events()],['script'])
        self.f.journal.write_text('')
        (self.seed/'a').write_text('third revision\n')
        self.run_git(self.seed,'commit','-am','third')
        self.run_git(self.seed,'push')
        class Refusing:
            def publish(self,*args,**kwargs):raise OSError('private fixture transport')
        config['autopull-log']='logs'
        with contextlib.redirect_stderr(io.StringIO()):
            a.attempt('ab',config,bus=Refusing())
        self.assertEqual([e['kind'] for e in self.f.events()],['script'])


if __name__ == '__main__':unittest.main()
