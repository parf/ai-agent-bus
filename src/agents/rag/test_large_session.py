"""A 200 KiB Markdown file through RAG with fake providers, end to end, and its recorded session.

The session is recorded from a fresh run and fails when a run differs. Regenerate with
`RAG_RECORD=1 python3 -m unittest test_large_session` after a deliberate change to what RAG does with a large file.
"""
import hashlib
import json
import os
from pathlib import Path
import sys
import unittest

HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE)]
from test_large import LargeStore,section
from test_rag import CFG,PROMPT,KV
from rules import RagError,parse,dumps
from chunks import chunks as heading_chunks
from core import Core

FIXTURE=HERE/'fixtures'/'large-session.jsonl'
SUBJECT={'namespace':'team','tableName':'docs','id':'Plans/big.md'}


def big(edit=None):
    """At least 200 KiB: 60 sections, each one chunk; edit replaces one section's text."""
    parts=['# Handbook\n\nWhat this handbook covers.\n\n']
    for n in range(60):
        title=f'Chapter {n}'
        parts.append(edit[1] if edit and edit[0]==n else section(title,11).replace(f'{title} paragraph 0 ',f'{title} paragraph 0 marker{n} '))
    return ''.join(parts)


EDITED=big((17,section('Chapter 17',11).replace('Chapter 17 paragraph 0 ','Chapter 17 paragraph 0 rewritten oolong ')))


class Session:
    """One RAG over fake providers, recording each step without what varies from run to run."""
    def __init__(self):
        self.store=LargeStore();self.app=Core(CFG,self.store,KV());self.app.cache.set('team','docs',PROMPT);self.steps=[]
    def call(self,**v):
        return self.app.run(parse(dumps({'namespace':'team','tableName':'docs',**v})))
    def step(self,name,**v):
        paid=len(self.store.paid)
        try:answer=self.call(**v)
        except RagError as error:answer={'refused':str(error)}
        if isinstance(answer,dict) and 'indexed_at' in answer:answer={**answer,'indexed_at':'<time>'}
        for hit in answer.get('records',[]) if isinstance(answer,dict) else []:hit['indexed_at']='<time>'
        embedded=[c for target,request in self.store.paid[paid:] if target=='voyage' for c in request['input']]
        keys={c['text']:c['key'] for text in (big(),EDITED) for c in heading_chunks(text)}
        chunks=sorted(p['payload']['_rag_key'] for p in self.store.points.values() if p['payload'].get('_rag_kind')=='chunk')
        call={k:val for k,val in v.items() if k!='bigfile'}
        if 'bigfile' in v:call['bigfile']={'size':v['bigfile']['size'],'sha256':v['bigfile']['sha256']}
        self.steps.append({'step':name,'call':call,'answer':answer,'embedded':[keys[t] for t in embedded],
                           'jev_calls':sum(1 for target,_ in self.store.paid[paid:] if target=='ai'),'stored_chunks':chunks})
    def put(self,name,text):
        ref=self.store.put_file(text)
        self.step(name,method='upsert',id=SUBJECT['id'],source=SUBJECT['id'],content_hash=ref['sha256'],bigfile=ref)


def record():
    s=Session()
    s.put('first upsert',big())
    s.step('check after the first upsert',method='get',id=SUBJECT['id'],check=True)
    s.step('search finds a chapter',method='search',query='marker17',limit=4)
    s.put('the same file again',big())
    s.put('one chapter edited',EDITED)
    s.step('check after the edit',method='get',id=SUBJECT['id'],check=True)
    s.step('search finds the edit',method='search',query='oolong',limit=4)
    s.step('search for the replaced text',method='search',query='marker17',limit=4)
    s.step('delete',method='delete',id=SUBJECT['id'])
    s.step('check after the delete',method='get',id=SUBJECT['id'],check=True)
    s.step('search after the delete',method='search',query='marker3',limit=4)
    return s.steps


class LargeSession(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.steps=record();cls.by={s['step']:s for s in cls.steps}

    def test_the_recorded_session_is_what_rag_does_today(self):
        text=''.join(json.dumps(s,ensure_ascii=False,sort_keys=True)+'\n' for s in self.steps)
        if os.environ.get('RAG_RECORD')=='1':FIXTURE.parent.mkdir(exist_ok=True);FIXTURE.write_text(text,encoding='utf-8')
        self.assertEqual(FIXTURE.read_text(encoding='utf-8'),text,'the fixture is stale: regenerate it with RAG_RECORD=1 and review the diff')

    def test_a_200_kib_file_is_stored_whole_and_confirmed_by_hash(self):
        self.assertGreaterEqual(len(big().encode()),200<<10)
        first=self.by['first upsert']
        self.assertEqual(first['answer']['content_hash'],hashlib.sha256(big().encode()).hexdigest())
        self.assertEqual(len(first['embedded']),len(heading_chunks(big())));self.assertEqual(first['jev_calls'],1)
        self.assertEqual(first['stored_chunks'],sorted(c['key'] for c in heading_chunks(big())))
        check=self.by['check after the first upsert']['answer']
        self.assertEqual((check['complete'],check['content_hash']),(True,first['answer']['content_hash']))

    def test_the_same_file_again_embeds_nothing(self):
        self.assertEqual(self.by['the same file again']['embedded'],[])

    def test_editing_one_section_re_embeds_only_that_section(self):
        edited=self.by['one chapter edited']
        only=[c['key'] for c in heading_chunks(EDITED) if c['heading']==['Handbook','Chapter 17']]
        self.assertEqual(edited['embedded'],only)
        self.assertEqual(edited['stored_chunks'],sorted(c['key'] for c in heading_chunks(EDITED)),'no stale chunk is left')
        check=self.by['check after the edit']['answer']
        self.assertEqual((check['complete'],check['content_hash']),(True,hashlib.sha256(EDITED.encode()).hexdigest()))
        self.assertEqual([h['heading_path'] for h in self.by['search finds the edit']['answer']['records']],[['Handbook','Chapter 17']])
        self.assertEqual(self.by['search for the replaced text']['answer']['records'],[],'the replaced text no longer answers')

    def test_after_the_delete_no_chunk_answers(self):
        self.assertEqual(self.by['delete']['answer'],{'status':'ok'})
        self.assertEqual(self.by['delete']['stored_chunks'],[])
        self.assertRegex(self.by['check after the delete']['answer']['refused'],'not_found')
        self.assertEqual(self.by['search after the delete']['answer']['records'],[])


if __name__=='__main__':unittest.main()
