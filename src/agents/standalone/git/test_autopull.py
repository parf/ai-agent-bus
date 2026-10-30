"""Real local Git fixtures for the autopull/event/publication boundary."""
import contextlib
from concurrent.futures import ThreadPoolExecutor
import fcntl
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import autopull as a
import git_agent as g
import test_git as fixture


class Bus:
    name = '#git@fixture.test'
    def __init__(self, config=None): self.messages=[]; self.config=config
    def publish(self, channel, body, **kwargs): self.messages.append((channel,json.loads(body),kwargs)); return 'accepted'
    def refresh(self, *_): pass


class Core(unittest.TestCase):
    setUp=fixture.GitAgent.setUp; tearDown=fixture.GitAgent.tearDown
    run_git=fixture.GitAgent.run_git; advance=fixture.GitAgent.advance

    def config(self, **entry): return {'repos':{'ab':{**self.repos['ab'],**entry}},'autopull-cmd':'autopull-cmd','autopull-log':'autopull-log'}
    def attempt(self, config=None, **kw):
        self.bus=Bus()
        return a.attempt('ab',config or self.config(),bus=self.bus,agent=self.bus.name,**kw)
    def event(self, **kw): return json.dumps({'v':1,'type':'push','remote':'up','branch':'main','commit':self.first,**kw})
    def message(self, body, channel=None, roles=()):
        answers=[]
        m=SimpleNamespace(body=body, original_to=channel, roles=roles, receipt=None,re=None,ask=None,reply=answers.append)
        setattr(m,'from','#pipeline@fixture.test'); m.answers=answers
        return m
    def wait(self, predicate):
        end=time.monotonic()+5
        while not predicate() and time.monotonic()<end: time.sleep(.01)
        self.assertTrue(predicate(),'bounded fixture operation did not finish')

    def test_config_mode_follow_caps_and_channels(self):
        self.assertEqual(g.configuration(json.dumps(self.config()))['repos']['ab']['access'],'commit')
        for entry in ({'mode':'mirror','access':'read'},{'follow':True,'access':'read'},{'follow':'yes'},{'mode':'hard'},{'branch':'-main'},{'branch':'x.lock'},{'branch':'x//y'}):
            with self.subTest(entry=entry), self.assertRaises(ValueError): g.configuration(json.dumps(self.config(**entry)))
        v=self.config(follow=True);del v['autopull-cmd']
        with self.assertRaises(ValueError):g.configuration(json.dumps(v))
        self.assertFalse(g.configuration(json.dumps({'repos':self.repos}))['repos']['ab'].get('follow',False))

    def test_event_parser_strict_fields_types_and_hex(self):
        self.assertEqual(a.event(self.event())['commit'],self.first)
        for kw in ({'v':2},{'v':True},{'v':1.0},{'extra':1},{'commit':'a'*39},{'commit':'A'*40},{'branch':'--evil'},{'branch':'a..b'},{'id':'x/y'},{'id':''},{'remote':'../bad'},{'type':'clean'}):
            with self.subTest(kw=kw), self.assertRaises(ValueError): a.event(self.event(**kw))
        for raw in (self.event().replace('"v": 1', '"v": 1, "v": 1'), '{"v":NaN}', '{"x":'+ '['*10+'0'+']'*10+'}', 'x'*(a.EVENT_MAX+1)):
            with self.subTest(raw=raw[:30]), self.assertRaises(ValueError): a.event(raw)
        self.assertEqual(a.event(self.event(type='deploy',commit=self.first[:7],id='p.1_-'))['id'],'p.1_-')

    def test_deploy_call_grammar_and_options(self):
        self.assertEqual(g.parse('autopull ab now'),{'method':'autopull','repo':'ab','now':True})
        self.assertEqual(g.parse('deploy ab main '+self.first[:7])['rev'],self.first[:7])
        for body in ('deploy ab --evil','deploy ab main --upload-pack=x','deploy ab main HEAD','autopull ab later','{"method":"autopull","repo":"ab","now":1}', '{"method":"deploy","repo":"ab","branch":"main","roles":["owner"]}'):
            with self.subTest(body=body),self.assertRaises(ValueError):g.parse(body)

    def test_remote_normalizes_without_url_or_path(self):
        for origin in ('https://user:password@git.example.test/team/project.git','ssh://git@git.example.test/team/project.git','git@git.example.test:team/project.git'):
            self.assertEqual(a.remote_path(origin),'team/project')
        self.assertEqual(a.remote_path(str(self.up)),'up')
        for origin in ('https://x/../../bad','https://x/a?token=secret','relative/path'):
            with self.subTest(origin=origin),self.assertRaises(ValueError):a.remote_path(origin)

    def test_push_moves_tip_publishes_once_and_never_rolls_back(self):
        new=self.advance(); result=self.attempt(rev=self.first,trigger='event',requested_by='#ci@fixture.test',event_id='p1')
        self.assertEqual((result['result'],result['from'],result['to'],result['requested']),('ok',self.first,new,self.first))
        self.assertEqual(len(self.bus.messages),1)
        channel,record,meta=self.bus.messages[0]
        self.assertEqual((channel,meta),('autopull-log',{'topic':'ab@'+result['host'],'tag':'ok'}))
        self.assertEqual(set(record),{'v','host','repo','branch','result','action','trigger','mode','at','agent','remote','requested_by','id','from','to','requested','discarded','script','error','ms'})
        self.assertEqual(record['id'],'p1');self.assertEqual(record['agent'],self.bus.name)
        self.assertEqual(self.attempt(rev=self.first)['result'],'skip')
        self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),new)

    def test_dirty_ff_preserves_all_files_and_diverged_preserves_head(self):
        self.advance();(self.repo/'a').write_text('SENTINEL_EDIT');(self.repo/'untracked').write_text('SENTINEL_UNTRACKED')
        result=self.attempt();self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'dirty')
        self.assertEqual((self.repo/'a').read_text(),'SENTINEL_EDIT');self.assertEqual((self.repo/'untracked').read_text(),'SENTINEL_UNTRACKED')
        (self.repo/'untracked').unlink();self.run_git(self.repo,'commit','-am','local-only-secret-subject')
        before=self.run_git(self.repo,'rev-parse','HEAD').strip()
        result=self.attempt();self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'diverged')
        self.assertEqual(result['to'],before);self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),before)

    def test_untracked_only_and_operation_state_are_dirty(self):
        self.advance();(self.repo/'untracked').write_text('keep')
        self.assertEqual(self.attempt()['error']['code'],'dirty');(self.repo/'untracked').unlink()
        (self.repo/'.git/MERGE_HEAD').write_text(self.first+'\n')
        self.assertEqual(self.attempt()['error']['code'],'dirty')
        self.assertEqual(self.attempt(self.config(mode='mirror'))['error']['code'],'dirty')

    def test_mirror_discards_two_files_counts_commits_and_keeps_ignored(self):
        new=self.advance();(self.repo/'a').write_text('edit');(self.repo/'untracked').write_text('remove')
        (self.repo/'.git/info/exclude').write_text('ignored\n');(self.repo/'ignored').write_text('keep ignored')
        result=self.attempt(self.config(mode='mirror'))
        self.assertEqual(result['result'],'ok');self.assertEqual(result['to'],new)
        self.assertEqual(result['discarded'],{'files':2,'commits':0})
        self.assertFalse((self.repo/'untracked').exists());self.assertEqual((self.repo/'a').read_text(),'two\n')
        self.assertTrue((self.repo/'ignored').is_file(),'mirror deleted an ignored file')
        self.assertEqual((self.repo/'ignored').read_text(),'keep ignored')
        self.run_git(self.repo,'commit','--allow-empty','-m','local-only')
        result=self.attempt(self.config(mode='mirror'));self.assertEqual(result['discarded']['commits'],1)

    def test_deploy_branch_pinning_reachability_and_exact_revision(self):
        self.run_git(self.seed,'checkout','-b','other');(self.seed/'a').write_text('other');self.run_git(self.seed,'commit','-am','other');self.run_git(self.seed,'push','-u','origin','other')
        other=self.run_git(self.seed,'rev-parse','HEAD').strip()
        result=self.attempt(action='deploy',branch='other');self.assertEqual((result['result'],result['branch'],result['to']),('ok','other',other))
        self.assertEqual(self.run_git(self.repo,'branch','--show-current').strip(),'other')
        # Effective branch persists even if a later read call detaches HEAD.
        self.run_git(self.repo,'checkout','--detach',self.first)
        result=self.attempt(self.config(mode='mirror'));self.assertEqual((result['branch'],result['to']),('other',other))
        result=self.attempt(self.config(branch='main'),action='deploy',branch='other')
        self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'invalid_event');self.assertEqual(result['from'],result['to'])
        result=self.attempt(action='deploy',branch='main',rev=other[:7]);self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'not_in_branch')
        self.assertEqual(self.run_git(self.repo,'branch','--show-current').strip(),'other')
        self.run_git(self.seed,'checkout','main');self.advance()
        result=self.attempt(action='deploy',branch='main',rev=self.first[:7]);self.assertEqual(result['to'],self.first)
        self.assertEqual(result['requested'],self.first)

    def test_force_pushed_announced_commit_not_reachable_is_refused(self):
        self.advance();self.run_git(self.seed,'checkout','--orphan','replacement');self.run_git(self.seed,'commit','-m','new root');self.run_git(self.seed,'push','--force','origin','replacement:main')
        result=self.attempt(self.config(mode='mirror'),rev=self.first)
        self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'not_in_branch');self.assertEqual(result['from'],result['to'])

    def test_records_and_answers_do_not_leak_checkout_url_or_subject(self):
        self.advance()
        with patch.dict(os.environ,{'AB_ROLE_DEPLOY':'1','AGENT_BUS_CONFIG':json.dumps(self.config())}): answer=g.answer('deploy ab')
        record=json.loads(answer)
        for secret in (str(self.repo),str(self.up),'second'):
            self.assertNotIn(secret,json.dumps(record))
        self.assertEqual(record['result'],'ok')
        with patch.object(a.Commands,'run',side_effect=OSError('UNIQUE_SECRET_URL_PATH_MESSAGE')):
            result=self.attempt()
        self.assertNotIn(str(self.repo),json.dumps(result));self.assertNotIn(str(self.up),json.dumps(result));self.assertNotIn('UNIQUE_SECRET',json.dumps(result));self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'fetch')

    def test_optional_log_failure_does_not_suppress_hook_or_corrupt_json(self):
        self.advance();seen=[]
        config=self.config();bus=Bus()
        with patch.object(bus,'publish',side_effect=TimeoutError('SECRET')),patch.object(a,'after_attempt',side_effect=lambda repo,root,record,publish:seen.append(record.copy())):
            result=a.attempt('ab',config,bus=bus,agent=bus.name)
            self.assertEqual(bus.publish.call_count,1)
        self.assertEqual(result['result'],'ok');self.assertEqual(len(seen),1)
        self.assertIn('log publish failed',a.last_status('ab'))
        config.pop('autopull-log');bus=Bus()
        with patch.object(a,'after_attempt') as hook:
            result=a.attempt('ab',config,bus=bus,agent=bus.name)
            self.assertEqual(result['result'],'skip');hook.assert_called_once()
        self.assertEqual(bus.messages,[])
        with patch.dict(os.environ,{'AB_ROLE_DEPLOY':'1','AGENT_BUS_CONFIG':json.dumps(self.config())}):
            value=g.answer('deploy ab');self.assertIsInstance(json.loads(value),dict)

    def test_hook_runs_after_publish_under_lock_and_no_hook_without_lock(self):
        self.advance();bus=Bus();seen=[]
        def hook(repo,checkout,record,publish):
            self.assertEqual(len(bus.messages),1)
            self.assertEqual(checkout,self.repo.resolve());self.assertEqual(repo['path'],str(self.repo))
            with self.assertRaises(TimeoutError),g.checkout_lock(checkout,time.monotonic()+.025):pass
            seen.append(record['to'])
        with patch.object(a,'after_attempt',side_effect=hook):a.attempt('ab',self.config(),bus=bus)
        self.assertEqual(len(seen),1)
        with g.checkout_lock(self.repo.resolve(),time.monotonic()+1),patch.object(a,'BUDGET',.03),patch.object(a,'after_attempt') as hook:
            result=self.attempt();self.assertIsInstance(result.get('error'),dict,result);self.assertEqual(result['error']['code'],'lock');hook.assert_not_called()
        with g.checkout_lock(self.repo.resolve(),time.monotonic()+.1):pass

    def test_two_concurrent_events_two_records_one_move_and_lock_released(self):
        self.advance();bus=Bus()
        with ThreadPoolExecutor(max_workers=2) as pool:
            results=list(pool.map(lambda _:a.attempt('ab',self.config(),bus=bus,trigger='event',branch='main'),range(2)))
        self.assertEqual(sorted(r['result'] for r in results),['ok','skip']);self.assertEqual(len(bus.messages),2)
        with g.checkout_lock(self.repo.resolve(),time.monotonic()+.1):pass

    def test_served_invalid_events_and_unknown_remote_or_branch(self):
        self.advance();config=self.config(follow=True);bus=Bus(config);runtime=a.Runtime(bus)
        try:
            for kw in ({'v':2},{'extra':1},{'commit':'a'*39},{'id':'bad/id'}):runtime.events(config,self.message(self.event(**kw)))
            self.assertEqual(len(bus.messages),4)
            self.assertTrue(all(m[1]['error']['code']=='invalid_event' for m in bus.messages))
            for kw in ({'remote':'other/project'},{'branch':'other'}):runtime.events(config,self.message(self.event(**kw)))
            self.assertEqual(len(bus.messages),4);self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),self.first)
        finally:runtime.close()

    def test_runtime_channel_authority_both_directions_and_roles(self):
        new=self.advance();config=self.config(follow=True);bus=Bus(config);runtime=a.Runtime(bus)
        try:
            direct=self.message(self.event())
            runtime.dispatch(direct);self.wait(lambda:bool(direct.answers))
            self.assertIn('unknown method',direct.answers[0]);self.assertEqual(bus.messages,[])
            for original in ('autopull-cmd2@fixture.test','autopull-cmd@other.test'):
                runtime.dispatch(self.message(self.event(),channel=original))
                runtime.close();runtime=a.Runtime(bus)
            self.assertEqual(bus.messages,[])
            runtime.dispatch(self.message(self.event(),channel='autopull-cmd@fixture.test'))
            self.wait(lambda:len(bus.messages)==1);self.assertEqual(bus.messages[0][1]['to'],new)
            for body in ('deploy ab','autopull ab now'):
                no_role=self.message(body);runtime.dispatch(no_role);self.wait(lambda:bool(no_role.answers));self.assertIn('requires deploy',no_role.answers[0])
        finally:runtime.close()

    def test_startup_one_record_only_followed_repos(self):
        new=self.advance();bus=Bus(self.config(follow=True));runtime=a.Runtime(bus)
        try:
            runtime.startup();self.wait(lambda:len(bus.messages)==1)
            self.assertEqual((bus.messages[0][1]['trigger'],bus.messages[0][1]['to']),('start',new))
        finally:runtime.close()
        bus=Bus(self.config());runtime=a.Runtime(bus)
        try:runtime.startup()
        finally:runtime.close()
        self.assertEqual(bus.messages,[])

    def test_child_env_token_free_and_roles_only_from_message(self):
        runtime=a.Runtime(Bus());real=subprocess.Popen;captured=[]
        def capture(*args,**kw):captured.append(kw['env']);return real(*args,**kw)
        try:
            with patch.dict(os.environ,{'AGENT_BUS_TOKEN':'SECRET','AB_ROLE_OWNER':'1','AB_ROLE_DEPLOY':'1'}),patch.object(subprocess,'Popen',side_effect=capture):
                answer=runtime.child(self.config(),self.message('clean ab'))
            self.assertIn('requires deploy',answer)
            self.assertNotIn('AGENT_BUS_TOKEN',captured[0]);self.assertFalse(any(k.startswith('AB_ROLE_') for k in captured[0]))
        finally:runtime.close()

    def test_every_destructive_deploy_reports_discarded_files_and_commits(self):
        self.advance();(self.repo/'a').write_text('local-committed')
        self.run_git(self.repo,'commit','-am','local-only')
        (self.repo/'b').write_text('local-uncommitted')
        result=self.attempt(self.config(mode='ff'),action='deploy')
        self.assertEqual(result['result'],'ok');self.assertEqual(result['discarded'],{'files':1,'commits':1})
        self.assertEqual((self.repo/'b').read_text(),'two\n')

    def test_discard_snapshot_taken_after_fetch(self):
        self.advance();real=a.Commands.run;modified=False
        def run(cmd,*args,**kw):
            nonlocal modified
            value=real(cmd,*args,**kw)
            if args[0]=='fetch' and not modified:
                (self.repo/'late-file').write_text('created while fetching');modified=True
            return value
        with patch.object(a.Commands,'run',run):result=self.attempt(self.config(mode='mirror'))
        self.assertEqual(result['discarded']['files'],1);self.assertFalse((self.repo/'late-file').exists())

    def test_equal_channels_and_self_delivered_results_are_refused(self):
        for command,log in [('same','same'),('same','same@fixture.test'),('same@fixture.test','same')]:
            config=self.config(follow=True);config.update({'autopull-cmd':command,'autopull-log':log})
            with self.assertRaises(ValueError):g.configuration(json.dumps(config),identity=Bus.name)
        self.advance();bus=Bus(self.config(follow=True));runtime=a.Runtime(bus)
        try:
            message=self.message(self.event(),channel='autopull-cmd@fixture.test');setattr(message,'from',bus.name)
            runtime.dispatch(message);runtime.close()
            self.assertEqual(bus.messages,[]);self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),self.first)
        finally:runtime.close()

    def test_startup_failure_does_not_abort_remaining_repositories(self):
        bus=Bus(self.config(follow=True));bus.config['repos']['second']={**self.repos['ab'],'follow':True}
        runtime=a.Runtime(bus);called=[]
        def attempt(name,*args,**kw):
            called.append(name)
            if name=='ab':raise RuntimeError('PRIVATE_PATH')
            return {}
        try:
            with patch.object(a,'attempt',side_effect=attempt):runtime.startup();runtime.close()
            self.assertEqual(called,['ab','second'])
        finally:runtime.close()

    def test_hex_named_ref_does_not_override_revision_object(self):
        wanted=self.advance();self.run_git(self.repo,'fetch','origin')
        self.run_git(self.repo,'branch',self.first[:7],wanted)
        result=self.attempt(action='deploy',branch='main',rev=self.first[:7])
        self.assertEqual(result['to'],self.first);self.assertEqual(result['requested'],self.first)



class Retirement(unittest.TestCase):
    def test_reinstall_retires_only_owned_timer_units_idempotently(self):
        import tempfile
        import timers
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);unit=root/'agent-bus-git.service';marker='# agent-bus standalone: #git@fixture.test\n'
            unit.write_text(marker+'[Service]\n');old=[]
            for suffix in ('service','timer'):
                path=root/('agent-bus-git-autopull-ab.'+suffix);path.write_text(marker+'[Unit]\n');old.append(path)
            unrelated=root/'agent-bus-other.timer';unrelated.write_text('keep')
            with patch.object(timers.starter,'command') as command:
                timers.retire(unit,'#git@fixture.test','fixture-systemctl')
                self.assertEqual(command.call_args_list[0].args[0],['fixture-systemctl','disable','--now','agent-bus-git-autopull-ab.timer'])
                self.assertEqual(command.call_args_list[2].args[0],['fixture-systemctl','disable','--now','agent-bus-git-autopull-ab.service'])
                count=command.call_count;timers.retire(unit,'#git@fixture.test','fixture-systemctl')
                self.assertEqual(command.call_count,count)
            self.assertTrue(unit.exists());self.assertEqual(unrelated.read_text(),'keep')
            self.assertTrue(all(not path.exists() for path in old))
            foreign=root/'agent-bus-git-autopull-outside.timer';foreign.write_text('foreign')
            with patch.object(timers.starter,'command') as command,self.assertRaises(ValueError):
                timers.retire(unit,'#git@fixture.test','fixture-systemctl')
            command.assert_not_called();self.assertEqual(foreign.read_text(),'foreign')
            foreign.unlink();foreign.symlink_to(unrelated)
            with self.assertRaises(ValueError):timers.retire(unit,'#git@fixture.test','fixture-systemctl')

    def test_removed_flag_refused_before_any_installer_action(self):
        program=Path(a.__file__).with_name('install.py')
        for flag in ('--autopull=ab=5m','--autopull'):
            value=subprocess.run([sys.executable,str(program),'install','--name','#git@fixture.test',flag],capture_output=True,text=True,timeout=2)
            self.assertEqual(value.returncode,2)
            self.assertIn('Configuration',value.stderr);self.assertIn('timers were removed',value.stderr)
