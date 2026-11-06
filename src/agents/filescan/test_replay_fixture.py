"""The replayable FileScan session: recorded from a real FileScan run, and failing when a fresh run differs.

Regenerate with `FILESCAN_RECORD=1 python3 -m unittest test_replay_fixture` after a deliberate change to what FileScan sends.
"""
import itertools
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'shared'));sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
import filescan as fs
import replay
import test_filescan as base

FIXTURE=HERE/'fixtures'/'filescan-session.jsonl'
T0=1_700_000_000.0


def record():
    """Run FileScan over a temp folder and return the steps it produced, with nothing that varies from run to run."""
    with tempfile.TemporaryDirectory() as tmp:
        root=os.path.join(os.path.realpath(tmp),'vault');work=os.path.join(os.path.realpath(tmp),'work');os.makedirs(root);os.makedirs(work)
        bus=base.RagBus();bus.kinds['#rag@n.test']='agent';bus.kinds['#queue@n.test']='queue'
        agent=fs.FileScan(bus,{'delete_pct':60,'max_file_size':'64B','namespace':'vault','table':'notes'},root,work,clock=lambda:T0+500,warning=lambda *a:None)
        counter=itertools.count(1);agent.subs.id_factory=lambda:'%012x'%next(counter)
        def put(rel,data,at):
            p=Path(root)/rel;p.parent.mkdir(parents=True,exist_ok=True)
            p.write_bytes(data if isinstance(data,bytes) else data.encode());os.utime(p,(T0+at,T0+at))
        steps=[];seen={'sent':0,'calls':0}
        def step(name,body,roles=('owner',)):
            answer=json.loads(agent.handle(base.msg(body=body,roles=roles)))
            sent=[{'to':to,'topic':o['topic'],'tag':o['tag'],'body':json.loads(b)} for to,b,o in bus.sent[seen['sent']:]]
            calls=[{'to':to,'body':json.loads(b)} for to,b,o in bus.calls[seen['calls']:]]
            seen['sent']=len(bus.sent);seen['calls']=len(bus.calls)
            steps.append({'step':name,'call':body,'answer':answer,'sent':sent,'rag_calls':calls})
        put('a.md','first draft',1);put('notes/b.md','a note',2);put('notes/c.md','another note',3)
        put('big.md','x'*100,4);put('blob.dat',b'\xff\xfe\x00\x01',5)
        step('subscribe a queue','sub add #queue@n.test');step('subscribe rag','sub add #rag@n.test as rag vault notes')
        step('first scan','scan')
        put('a.md','second draft, changed',60);step('a file changes','scan')
        os.unlink(Path(root)/'notes/b.md');step('a file is deleted','scan')
        step('status','status');step('subscriptions','sub list')
        for rel in ('a.md','notes/c.md','big.md'):os.unlink(Path(root)/rel)
        step('everything but one file is deleted: the guard holds the deletes','scan')
        step('status while held','status')
        return steps


class Replay(unittest.TestCase):
    def test_the_recorded_session_is_what_filescan_sends_today(self):
        fresh=record();text=''.join(json.dumps(s,ensure_ascii=False,sort_keys=True)+'\n' for s in fresh)
        if os.environ.get('FILESCAN_RECORD')=='1':FIXTURE.parent.mkdir(exist_ok=True);FIXTURE.write_text(text,encoding='utf-8')
        self.assertEqual(FIXTURE.read_text(encoding='utf-8'),text,'the fixture is stale: regenerate it with FILESCAN_RECORD=1 and review the diff')

    def test_the_replay_harness_reads_every_step_and_lists_the_changes(self):
        steps=replay.load(FIXTURE);self.assertGreaterEqual(len(steps),8)
        got=list(replay.changes(steps))
        self.assertEqual([k for k,_,_ in got].count('created'),5)
        self.assertEqual([(k,p) for k,p,_ in got if k=='changed'],[('changed','a.md')])
        self.assertIn(('deleted','notes/b.md'),[(k,p) for k,p,_ in got])
        self.assertTrue(all(set(body)>={'change','path','warning'} for _,_,body in got))

    def test_the_session_shows_what_it_documents(self):
        steps={s['step']:s for s in replay.load(FIXTURE)}
        self.assertTrue(any('text' in m['body'] for m in steps['first scan']['sent']),'an inline text message')
        self.assertTrue(any('contents' in m['body'] for m in steps['first scan']['sent']),'a size-and-hash message without text')
        self.assertEqual({c['body']['method'] for c in steps['first scan']['rag_calls']},{'upsert','delete'},'skipped files get the delete form')
        self.assertIsNotNone(steps['status while held']['answer']['paused'])


if __name__=='__main__':unittest.main()
