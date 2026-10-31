"""Per-folder subscriptions with a local TLS mailbox and an in-memory bus."""
import copy
import json
import socket
import ipaddress
import time
from urllib.parse import urlparse
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import test_imap as fixture
import test_forwarding as old
import test_destructive
from folder_watch import FolderForwarder, REGISTRY
from mailbox_session import Session
from budget import alarm_budget
from cache import Cache
from forwarding import PROGRESS, selector

CONNECT = socket.create_connection


class Calls(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    call = fixture.IMAPTests.call
    folder = test_destructive.Calls.folder

    def setUp(self):
        fixture.IMAPTests.setUp(self)
        self.bus = old.Bus()
        self.clock = old.Clock()
        self.agent = fixture.proxy.Agent(self.bus, self.cfg, self.env, self.work.name)
        self.addCleanup(self.agent.close)
        source = Session(lambda: fixture.proxy.config(self.cfg), self.env, self.work.name, self.clock)
        self.watch = FolderForwarder(self.bus, self.bus.name, self.work.name, source, self.clock, self.clock, send_one=old.deliver(self.bus))
        self.agent.forwarder = self.watch
        self.envelope = {'from':'#author@t', 'original_to':self.bus.name}
        self.other = self.folder('Other Box', '')
        self.server.next_uid = 100

    def add(self, folder=None, target='#target@t'):
        value={'method':'sub.add','target':target}
        if folder is not None:value['folder']=folder
        answer=self.call(json.dumps(value))
        self.assertTrue(answer.startswith('added '),answer)
        return answer.split()[1]

    def arrive(self, uid=100, folder=None, subject='fresh'):
        data=self.other if folder else {'messages':self.server.messages}
        data['messages'][uid]=old.raw(subject)
        if folder:self.other['next_uid']=uid+1
        else:self.server.next_uid=uid+1

    def test_separate_folders_same_uid_no_cross_delivery_and_restart_no_repeat(self):
        default=self.add(target='#default@t')
        other=self.add('Other Box', '#other@t')
        oldrow=copy.deepcopy(self.bus.kv.data['sub.'+default])
        self.assertNotIn('folder',oldrow['selector'])
        self.assertIn('folder',self.bus.kv.data['sub.'+other]['selector'])
        self.assertEqual(self.bus.kv.data['sub.'+other]['selector']['folder'],'Other Box')
        self.arrive(subject='inbox only')
        self.arrive(folder='Other Box',subject='other only')
        self.watch.tick()
        self.assertEqual([(to,'inbox only' in body,'other only' in body) for to,body,_ in self.bus.sent],
                         [('#default@t',True,False),('#other@t',False,True)])
        self.assertIn(PROGRESS,self.bus.kv.data)
        keys=[key for key in self.bus.kv.data if key.startswith(PROGRESS+'.')]
        self.assertEqual(len(keys),1)
        self.assertNotEqual(self.bus.kv.data[PROGRESS]['identity'],self.bus.kv.data[keys[0]]['identity'])
        self.assertFalse(any(c.split()[1:2] == [b'SELECT'] or b' UID STORE ' in c for c in self.server.commands))
        self.watch.close()
        source=Session(lambda:fixture.proxy.config(self.cfg),self.env,self.work.name,self.clock)
        self.watch=FolderForwarder(self.bus,self.bus.name,self.work.name,source,self.clock,self.clock,send_one=old.deliver(self.bus))
        self.agent.forwarder=self.watch
        self.watch.tick()
        self.assertEqual(len(self.bus.sent),2)
        self.assertEqual(self.bus.kv.data['sub.'+default]['selector'],oldrow['selector'])

    def test_first_subscription_on_other_folder_does_not_reset_default_progress(self):
        self.add()
        self.arrive(subject='default pending')
        default=copy.deepcopy(self.bus.kv.data[PROGRESS])
        self.add('Other Box')
        self.assertEqual(self.bus.kv.data[PROGRESS],default)
        self.watch.tick()
        self.assertEqual(len(self.bus.sent),1)
        self.assertIn('default pending',self.bus.sent[0][1])

    def test_manual_cache_survives_nondefault_subscription_and_status_is_provider_free(self):
        self.assertIn('fetched',self.call('sync'))
        before=Path(self.work.name,'mail-cache/7.eml').stat().st_ino
        self.assertIn('fetched',self.call('sync --folder "Other Box"'))
        self.add('Other Box')
        self.arrive(folder='Other Box')
        self.watch.tick()
        self.assertTrue(Path(self.work.name,'mail-cache/7.eml').is_file())
        self.assertEqual(Path(self.work.name,'mail-cache/7.eml').stat().st_ino,before)
        commands=len(self.server.commands)
        status=self.agent.status()
        self.assertIn('folder="INBOX"',status)
        self.assertIn('folder="Other Box"',status)
        self.assertEqual(len(self.server.commands),commands)
        listing=self.call('sub list')
        self.assertIn('folder="Other Box"',listing)

    def test_pre_step5_synced_folder_names_recover_without_fetch_or_progress_change(self):
        self.assertIn('fetched',self.call('sync --folder "Other Box"'))
        for key in list(self.bus.kv.data):
            if key.startswith(REGISTRY):self.bus.kv.data.pop(key)
        before={k:copy.deepcopy(v) for k,v in self.bus.kv.data.items() if k.startswith(PROGRESS)}
        fetched=len(self.server.fetches)
        self.assertNotIn('folder="Other Box"',self.agent.status())
        self.assertIn('Other Box',self.call('folders'))
        self.assertIn('folder="Other Box"',self.agent.status())
        self.assertEqual(len(self.server.fetches),fetched)
        self.assertEqual({k:v for k,v in self.bus.kv.data.items() if k.startswith(PROGRESS)},before)

    def test_uidvalidity_reset_on_other_folder_does_not_reset_default(self):
        self.add(target='#default@t');self.add('Other Box','#other@t');self.watch.tick()
        before=copy.deepcopy(self.bus.kv.data[PROGRESS])
        self.other.update(validity=4, messages={1:old.raw('new epoch')}, next_uid=2)
        self.clock.advance(60);self.watch.tick()
        self.assertEqual(self.bus.sent,[])  # Reset rebaselines; old mail is not a new arrival.
        self.other['messages'][2]=old.raw('new epoch arrival');self.other['next_uid']=3
        self.clock.advance(60);self.watch.tick()
        self.assertEqual([(to,'new epoch' in body) for to,body,_ in self.bus.sent],[('#other@t',True)])
        self.assertEqual(self.bus.kv.data[PROGRESS],before)

    def test_renaming_and_deleting_nondefault_subscribed_folder_and_ancestor_refused(self):
        self.cfg['allow_destructive']=True
        self.folder('Other Box/Child','')
        self.add('Other Box/Child')
        for name in ['Other Box','Other Box/Child']:
            for body in ['folder rename '+json.dumps(name)+' New','folder delete '+json.dumps(name)+' --force']:
                self.assertIn('subscribed folder',self.call(body))
        self.assertFalse(any(c.split()[1] in (b'RENAME',b'DELETE') for c in self.server.commands))

    def test_refused_author_cannot_add_and_revocation_drops_next_folder_send(self):
        self.bus.author_allowed=False
        answer=self.call('sub add #target@t --folder "Other Box"')
        self.assertIn('author may not send',answer)
        self.assertFalse(any(k.startswith('sub.') for k in self.bus.kv.data))
        self.bus.author_allowed=True
        sub=self.add('Other Box')
        self.bus.author_allowed=False
        self.arrive(folder='Other Box')
        self.watch.tick()
        self.assertEqual(self.bus.sent,[])
        self.assertEqual(self.bus.kv.data['sub.'+sub]['dropped'],1)
        self.assertEqual(self.bus.kv.data['sub.'+sub]['sent'],0)

    def test_missing_noselect_folder_does_not_store_a_subscription(self):
        self.folder('Parent','\\Noselect')
        for name in ['Missing','Parent']:
            self.assertTrue(self.call('sub add #target@t --folder '+name).startswith('error:'))
        self.assertFalse(any(k.startswith('sub.') for k in self.bus.kv.data))

    def test_deleting_one_folder_subscription_keeps_other_progress(self):
        self.add()
        other=self.add('Other Box')
        before=copy.deepcopy(self.bus.kv.data[PROGRESS])
        self.assertEqual(self.call('sub delete '+other),'deleted')
        self.assertIn(PROGRESS,self.bus.kv.data)
        self.assertEqual(self.bus.kv.data[PROGRESS],before)
        self.assertFalse(any(k.startswith(PROGRESS+'.') for k in self.bus.kv.data))
        self.assertIn('folder="Other Box"',self.agent.status())

    def test_subscribed_status_includes_folder_even_without_sync_descriptor(self):
        self.add('Other Box')
        for key in list(self.bus.kv.data):
            if key.startswith(REGISTRY):self.bus.kv.data.pop(key)
        self.assertIn('folder="Other Box"',self.agent.status())

    def test_polling_fallback_waits_sixty_seconds_without_idle_command(self):
        self.add('Other Box')
        self.watch.tick()
        self.arrive(folder='Other Box')
        self.clock.advance(10);self.watch.tick()
        self.assertEqual(self.bus.sent,[])
        self.clock.advance(50);self.watch.tick()
        self.assertEqual(len(self.bus.sent),1)
        self.assertFalse(any(c.split()[1:2]==[b'IDLE'] for c in self.server.commands))
        self.assertGreater(self.watch.wait(),0.05)

    def test_idle_folders_deliver_before_poll_interval_with_done_before_fetch(self):
        self.server.extra_capabilities=['IDLE']
        self.add('Other Box')
        self.watch.tick()
        self.assertTrue(any(b' IDLE' in c for c in self.server.commands))
        self.assertIn('; IDLE',self.agent.status())
        self.arrive(folder='Other Box')
        self.clock.advance(10);self.watch.tick()
        self.assertEqual(len(self.bus.sent),1)
        self.assertIn('fresh',self.bus.sent[0][1])
        commands=self.server.commands
        idle=next(i for i,c in enumerate(commands) if b' IDLE' in c)
        done=next(i for i,c in enumerate(commands[idle+1:],idle+1) if c==b'DONE\r\n')
        fetch=next(i for i,c in enumerate(commands[done+1:],done+1) if b' UID FETCH ' in c)
        self.assertLess(idle,done);self.assertLess(done,fetch)

    def test_advertised_but_refused_idle_falls_back_without_repeated_attempts(self):
        self.server.extra_capabilities=['IDLE'];self.server.idle_refused=True
        self.add('Other Box');self.watch.tick()
        self.arrive(folder='Other Box');self.clock.advance(60);self.watch.tick()
        self.assertEqual(len(self.bus.sent),1)
        self.assertEqual(sum(b' IDLE' in c for c in self.server.commands),1)
        self.assertIn('; polling',self.agent.status())

    def test_manual_only_wait_does_not_spin_on_unsubscribed_folder(self):
        self.watch.tick()
        self.assertEqual(self.watch.wait(),10)

    def test_idle_continuation_timeout_is_bounded_and_closes_connection(self):
        self.server.extra_capabilities=['IDLE'];self.server.idle_silent=True
        self.add('Other Box')
        source=self.watch.slots['Other Box']['source']
        source.connection.sock.settimeout(.75)
        before=time.monotonic()
        from mailbox_session import SessionError
        with patch('mailbox_session.alarm_budget',lambda _seconds:alarm_budget(.05)),self.assertRaises(SessionError):
            source.start_idle()
        self.assertLess(time.monotonic()-before,.35)
        self.assertIsNone(source.connection)
        self.assertIsNone(source.idle_tag)

    def test_status_store_failure_is_a_cached_degraded_answer(self):
        from agent_bus.errors import TransportError
        with patch.object(self.bus.kv,'list',side_effect=TransportError('private')):
            self.assertEqual(self.agent.status(),'degraded: folder state unavailable')


class RealBus(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    folder = test_destructive.Calls.folder

    def test_nondefault_folder_through_handler_and_real_delivery_helper(self):
        from test_subscriptions_bus import Fixture
        live=Fixture(self);self.addCleanup(live.close)
        address=urlparse(live.info['addr'])
        self.assertTrue(ipaddress.ip_address(address.hostname).is_loopback)
        allowed={('127.0.0.1',self.server.server_address[1]),(address.hostname,address.port)}
        def only_fixtures(address,*args,**kwargs):
            if address not in allowed:raise AssertionError('external connection forbidden')
            return CONNECT(address,*args,**kwargs)
        patch('socket.create_connection',side_effect=only_fixtures).start()
        owner,author,imap,receiver=[live.client(name) for name in
            ('owner@fixture','#caller@fixture','#worker@fixture','#helper@fixture')]
        owner.record(receiver.name).manage(allow=[author.name,imap.name])
        other=self.folder('Other Box','');other['next_uid']=100
        clock=old.Clock()
        agent=fixture.proxy.Agent(imap,self.cfg,self.env,self.work.name);self.addCleanup(agent.close)
        source=Session(lambda:fixture.proxy.config(self.cfg),self.env,self.work.name,clock)
        agent.forwarder=FolderForwarder(imap,imap.name,self.work.name,source,clock,clock)
        author.send(imap.name,'sub add '+receiver.name+' --folder "Other Box"')
        request=imap.consume(wait=.2);self.assertIsNotNone(request)
        answer=agent.handle(request);self.assertTrue(answer.startswith('added '),answer)
        sub=answer.split()[1]
        other['messages'][100]=old.raw('only this folder');other['next_uid']=101
        agent.forwarder.tick()
        reply=receiver.consume(wait=.5);self.assertIsNotNone(reply)
        self.assertIn('only this folder',reply.body)
        self.assertIn('created by '+author.name,reply.body)
        self.assertIsNone(receiver.consume(wait=.05))
        owner.record(receiver.name).manage(allow=[imap.name])
        other['messages'][101]=old.raw('revoked');other['next_uid']=102
        clock.advance(60);agent.forwarder.tick()
        self.assertIsNone(receiver.consume(wait=.1))
        row=imap.record(imap.name).kv.get('sub.'+sub,kind='json')
        self.assertEqual((row['sent'],row['dropped']),(1,1))


class Arguments(unittest.TestCase):
    def test_folder_option_preserves_regex_and_json_matches(self):
        for body,pattern,folder in [
            ('sub add #target@t /^a  b$/i --folder "Other Box"','/^a  b$/i','Other Box'),
            ('sub add #target@t --folder="邮件 &"','','邮件 &'),
            ('sub add #target@t /a --folder literal/ --folder INBOX','/a --folder literal/','INBOX')]:
            self.assertEqual(fixture.proxy.parse(body),fixture.proxy.parse(json.dumps({'method':'sub.add','target':'#target@t','filter':pattern,'folder':folder})))
        self.assertEqual(fixture.proxy.parse('sub add #target@t /a --folder literal/')['filter'],'/a --folder literal/')
        for body in ['sub add #target@t --folder','sub add #target@t --folder A --folder B',
                     'sub add #target@t --folder "bad\r\nfolder"']:
            with self.assertRaises(ValueError):fixture.proxy.parse(body)
        self.assertEqual(selector({'validity':1,'from_uid':2}),{'validity':1,'from_uid':2})
