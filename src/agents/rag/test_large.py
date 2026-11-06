"""A Markdown file over 32 KiB by bigfile reference: heading chunks, vector reuse, the completeness check and citations."""
import hashlib
import json
from pathlib import Path
import sys
import time
import unittest
from unittest.mock import Mock

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from rules import *
from chunks import chunks as heading_chunks,LARGE_FORMAT,LARGE_CHUNKS
from blocks import Blocks,Budget
from core import Core
from agent_bus.errors import Refusal
from test_rag import CFG,PROMPT,KV,Store


def section(title,words,level=2):
    return '#'*level+' '+title+'\n\n'+'\n\n'.join(f'{title} paragraph {i} '+'word '*60 for i in range(words))+'\n\n'


def document(edit=None):
    """About 40 KiB: a preamble, then sections; edit replaces one section's text."""
    parts=['Preamble text before any heading.\n\n','# Plan\n\nIntro of the plan.\n\n']
    for n in range(10):
        title=f'Part {n}'
        parts.append(edit[1] if edit and edit[0]==n else section(title,11))
    return ''.join(parts)


class LargeStore(Store):
    def __init__(self,dimensions=2):
        super().__init__(dimensions);self.sparse=True;self.files={};self.downloads=[]
    def put_file(self,text,record='#filescan/vault@node',name=None):
        raw=text.encode();digest=hashlib.sha256(raw).hexdigest();name=name or digest
        self.files[(record,name)]=raw
        return {'record':record,'name':name,'size':len(raw),'sha256':digest}
    def bigfile(self,record,name,max_size):
        self.downloads.append((record,name))
        if (record,name) not in self.files:raise NamedRefusal('bigfile_unavailable','the referenced bigfile is absent or expired; send it again')
        return self.files[(record,name)]


class Large(unittest.TestCase):
    def setUp(self):
        self.store=LargeStore();self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.cache.set('team','docs',PROMPT)
    def call(self,**v):
        return self.app.run(parse(dumps({'namespace':'team','tableName':'docs',**v})))
    def put(self,text,id='Plans/big.md',**more):
        ref=self.store.put_file(text)
        return self.call(method='upsert',id=id,source=id,content_hash=ref['sha256'],bigfile=ref,**more)
    def check(self,id='Plans/big.md'):
        return self.call(method='get',id=id,check=True)
    def embedded(self):
        return [text for target,request in self.store.paid if target=='voyage' for text in request['input']]
    def chunk_points(self):
        return [p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk']

    # The heading rules, before any claim that unchanged sections are reused.
    def test_repeated_sibling_headings_get_their_own_ordinals(self):
        text='# Plan\n\n## Notes\n\nfirst notes\n\n## Notes\n\nsecond notes\n\n# Other\n\n## Notes\n\nother notes\n'
        found=heading_chunks(text)
        paths=[c['heading'] for c in found]
        self.assertEqual(paths,[['Plan'],['Plan','Notes #1'],['Plan','Notes #2'],['Other'],['Other','Notes']])
        self.assertEqual(len({c['key'] for c in found}),len(found),'every chunk has its own identity')
    def test_fence_content_and_other_non_headings_never_change_the_structure(self):
        text=('---\ntitle: x\n# not a heading in front matter\n---\n'
              '# Real\n\n```\n# inside a fence\n## also inside\n```\n\n'
              '   ~~~~\n# inside a tilde fence indented by three spaces\n~~~\n# still inside: a shorter close does not close\n~~~~\n\n'
              '#hashtag is text\n    # four spaces is code\nSetext title\n===\n\n'
              '## Second\n\n````\n# an unclosed fence runs to the end\n')
        found=heading_chunks(text)
        self.assertEqual([c['heading'] for c in found],[[],['Real'],['Real','Second']])
        self.assertIn('# not a heading in front matter',found[0]['text'])
        self.assertIn('# inside a tilde fence',found[1]['text']);self.assertIn('Setext title',found[1]['text'])
        self.assertIn('# an unclosed fence runs to the end',found[2]['text'])
    def test_lines_count_from_one_and_cover_the_section(self):
        text='intro\n\n# A\n\none\ntwo\n\n# B\n\nthree\n'
        found=heading_chunks(text)
        self.assertEqual([c['lines'] for c in found],[[1,1],[3,6],[8,10]],'a range ends at the last line with text')
    def test_more_than_256_chunks_is_refused_before_any_paid_call(self):
        text=''.join(f'## S{i}\n\nx\n\n' for i in range(LARGE_CHUNKS+1))
        with self.assertRaisesRegex(RagError,'more than 256 chunks'):self.put(text)
        self.assertEqual(self.store.paid,[])

    # Upsert by reference and the completeness check.
    def test_a_large_file_is_stored_whole_and_the_check_confirms_its_hash(self):
        text=document();answer=self.put(text)
        digest=hashlib.sha256(text.encode()).hexdigest()
        self.assertGreater(len(text.encode()),32<<10)
        self.assertEqual((answer['content_hash'],answer['source'],answer['id']),(digest,'Plans/big.md','Plans/big.md'))
        self.assertNotIn('data',answer);self.assertNotIn('embeddings',answer)
        head=self.store.points[point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})]['payload']
        self.assertNotIn('data',head,'the head never holds the text');self.assertEqual(head['_rag_format'],LARGE_FORMAT)
        self.assertGreater(len(head['_rag_chunks']),8,'a large head names more than eight chunks')
        self.assertEqual(len(self.chunk_points()),len(head['_rag_chunks']))
        self.assertEqual(''.join(self.embedded()),''.join(c['text'] for c in heading_chunks(text)))
        got=self.check()
        self.assertEqual(got,{'namespace':'team','tableName':'docs','id':'Plans/big.md','source':'Plans/big.md',
                              'content_hash':digest,'indexed_at':head['indexed_at'],'complete':True})
    def test_the_check_answers_complete_false_while_a_named_chunk_is_missing(self):
        self.put(document())
        victim=sorted(self.chunk_points(),key=lambda p:p['payload']['_rag_index'])[5]
        del self.store.points[victim['id']]
        self.assertIs(self.check()['complete'],False)
    def test_the_check_works_for_a_small_record_and_never_returns_text_or_vectors(self):
        self.call(method='upsert',id='small',source='small.md',data='Tea is a drink.')
        got=self.check('small')
        self.assertEqual(set(got),{'namespace','tableName','id','source','content_hash','indexed_at','complete'})
        self.assertIs(got['complete'],True)
        reads=[c for c in self.store.calls if c[1]=='/points/scroll' and c[2].get('with_vector')]
        self.assertEqual(reads,[],'the check reads no vectors')
    def test_the_check_refuses_a_false_check_and_an_absent_record(self):
        with self.assertRaisesRegex(RagError,'check must be true'):self.call(method='get',id='x',check=False)
        with self.assertRaisesRegex(RagError,'not_found'):self.check('absent')
    def test_the_check_never_confirms_a_record_whose_head_is_a_delete_tombstone(self):
        self.put(document())
        self.store.points[point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})]['payload']['_rag_kind']='deleted'
        with self.assertRaisesRegex(RagError,'not_found'):self.check()
    def test_a_small_head_names_at_most_eight_chunks_and_a_large_head_256(self):
        self.put(document());self.call(method='upsert',id='small',source='small.md',data='Tea is a drink.')
        for id,count in (('small',9),('Plans/big.md',LARGE_CHUNKS+1)):
            head=self.store.points[point_id({'namespace':'team','tableName':'docs','id':id})]['payload']
            head['_rag_chunks']=[point_id({'namespace':'team','tableName':'docs','id':id},head['_rag_rev'],i) for i in range(count)]
            with self.assertRaisesRegex(RagError,'invalid head chunk references'):self.check(id)
    def test_a_large_head_is_not_held_to_the_small_records_scalar_cap(self):
        self.store=LargeStore(1024);self.app=Core({**CFG,'dimensions':1024},self.store,self.kv)
        self.app.cache.set('team','docs',PROMPT)
        self.put(document())
        head=self.store.points[point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})]['payload']
        self.assertGreater(len(head['_rag_chunks'])*1024,SCALARS_MAX,'the large head is over the small path\'s scalar cap')
        try:got=self.check()
        except RagError as error:self.fail(f'a large head was refused: {error}')
        self.assertIs(got['complete'],True)
    def test_plain_get_of_a_large_record_keeps_its_caps(self):
        self.put(document())
        with self.assertRaisesRegex(RagError,'stored record exceeds read cap'):self.call(method='get',id='Plans/big.md')

    # Re-sync: reuse, identity and atomic replacement.
    def test_editing_one_section_embeds_only_that_section(self):
        self.put(document());self.store.paid.clear()
        changed=section('Part 4',11).replace('paragraph 3','paragraph three')
        text=document((4,changed));self.put(text)
        expected=[c['text'] for c in heading_chunks(text) if c['heading']==['Plan','Part 4']]
        self.assertEqual(self.embedded(),[t for t in expected if 'paragraph three' in t])
        self.assertIs(self.check()['complete'],True)
        self.assertEqual(self.check()['content_hash'],hashlib.sha256(text.encode()).hexdigest())
    def test_a_changed_heading_path_is_embedded_again_though_its_text_is_the_same(self):
        text=document();self.put(text);self.store.paid.clear()
        renamed=text.replace('# Plan\n','# Plan B\n',1)
        self.put(renamed)
        moved=[c['text'] for c in heading_chunks(renamed) if c['heading'][:1]==['Plan B']]
        self.assertEqual(sorted(self.embedded()),sorted(moved),'every chunk under the renamed heading is new, though Part sections kept their text')
    def test_a_resync_leaves_no_stale_chunk_answering(self):
        self.put(document())
        text=document((2,section('Part 2',1).replace('Part 2 paragraph 0','fresh oolong')));self.put(text)
        rev=self.store.points[point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})]['payload']['_rag_rev']
        self.assertTrue(all(p['payload']['_rag_rev']==rev for p in self.chunk_points()))
        hits=self.call(method='search',query='paragraph 7',limit=16)['records']
        self.assertTrue(all(h['content_hash']==hashlib.sha256(text.encode()).hexdigest() for h in hits))
    def test_a_head_that_never_lands_keeps_the_previous_version_answering(self):
        old=document();self.put(old)
        head_id=point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})
        self.store.fail=lambda verb,path,body:verb=='PUT' and any(p['id']==head_id for p in body.get('points',[]))
        new=document((1,section('Part 1',2).replace('Part 1 paragraph 0','darjeeling')))
        with self.assertRaises(Unknown):self.put(new)
        self.store.fail=None
        got=self.check()
        self.assertEqual(got['content_hash'],hashlib.sha256(old.encode()).hexdigest(),'no confirmation before the head is published')
        self.assertIs(got['complete'],True)
        hits=self.call(method='search',query='darjeeling')['records']
        self.assertEqual(hits,[],'chunks of a version whose head never landed never answer')

    def test_an_interrupted_attempt_keeps_its_paid_chunks_for_the_resend(self):
        text=''.join(section(f'Topic {i}',1) for i in range(80))
        self.assertGreater(len(heading_chunks(text)),32,'more than one embedding batch')
        calls=[]
        real=self.store.embed
        def embed(request):
            calls.append(len(request['input']))
            if len(calls)==2:raise Deadline('message deadline reached')
            return real(request)
        self.store.embed=embed
        with self.assertRaisesRegex(RagError,'deadline: not stored; prior record unchanged'):self.put(text)
        with self.assertRaisesRegex(RagError,'not_found'):self.check()
        self.store.embed=real;self.store.paid.clear()
        self.put(text)
        first=[c['text'] for c in heading_chunks(text)][:calls[0]]
        self.assertFalse(set(first)&set(self.embedded()),'the first batch, paid and stored, is not paid again')
        self.assertEqual(len(self.embedded()),len(heading_chunks(text))-calls[0])
        self.assertIs(self.check()['complete'],True)
    def test_a_reused_chunk_takes_the_new_versions_labels_and_keeps_only_its_vector(self):
        self.put(document())
        self.store.probabilities={'drink':.1,'other':.9}
        text=document((3,section('Part 3',2)));self.put(text)
        rev=self.store.points[point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})]['payload']['_rag_rev']
        for p in self.chunk_points():
            self.assertEqual((p['payload']['_rag_labels'],p['payload']['_rag_rev']),(['other'],rev),'every chunk carries the new version\'s labels')
    def test_a_vector_is_never_reused_for_other_text_under_an_equal_hash_field(self):
        text=document();self.put(text)
        victim=next(p for p in self.chunk_points() if p['payload']['_rag_heading']==['Plan','Part 2'])
        victim['payload']['_rag_text']=victim['payload']['_rag_text'].replace('paragraph','PARAGRAPH')
        self.store.paid.clear();self.put(text+'\n# Tail\n\nnew tail\n')
        self.assertIn(next(c['text'] for c in heading_chunks(text) if c['key']==victim['payload']['_rag_key']),self.embedded())
    def test_a_delete_of_a_file_that_never_published_sweeps_its_chunks(self):
        head_id=point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})
        self.store.fail=lambda verb,path,body:verb=='PUT' and any(p['id']==head_id for p in body.get('points',[]))
        with self.assertRaises(Unknown):self.put(document())
        self.store.fail=None
        self.assertTrue(self.chunk_points(),'paid chunks were stored before the head')
        with self.assertRaisesRegex(RagError,'not_found'):self.call(method='delete',id='Plans/big.md')
        self.assertEqual(self.chunk_points(),[])
    def test_the_sweep_takes_only_the_deleted_ids_chunks(self):
        self.app.cache.set('team','notes',PROMPT)
        self.put(document(),id='Plans/other.md')
        ref=self.store.put_file(document())
        self.call(method='upsert',tableName='notes',id='Plans/big.md',source='Plans/big.md',content_hash=ref['sha256'],bigfile=ref)
        def owned(table,id):
            return sorted(p['id'] for p in self.chunk_points() if (p['payload']['tableName'],p['payload']['id'])==(table,id))
        before={k:owned(*k) for k in (('docs','Plans/other.md'),('notes','Plans/big.md'))}
        head_id=point_id({'namespace':'team','tableName':'docs','id':'Plans/big.md'})
        self.store.fail=lambda verb,path,body:verb=='PUT' and any(p['id']==head_id for p in body.get('points',[]))
        with self.assertRaises(Unknown):self.put(document())
        self.store.fail=None
        with self.assertRaisesRegex(RagError,'not_found'):self.call(method='delete',id='Plans/big.md')
        self.assertEqual(owned('docs','Plans/big.md'),[],'the headless chunks are swept')
        self.assertTrue(all(before.values()))
        self.assertEqual({k:owned(*k) for k in before},before,'another id, and the same id in another table, keep every chunk')
    def test_a_provider_failure_before_the_head_says_the_prior_record_is_unchanged(self):
        old=document();self.put(old)
        for provider,text in (('voyage','fake Voyage unavailable'),('ai','fake AI unavailable')):
            self.store.fail_paid=provider
            with self.assertRaisesRegex(RagError,'^not stored; prior record unchanged: '+text) as raised:
                self.put(document((3,section('Part 3',2))))
            self.assertNotIsInstance(raised.exception,Unknown)
            self.assertEqual(self.check()['content_hash'],hashlib.sha256(old.encode()).hexdigest())
        self.store.fail_paid=None
    def test_a_late_start_is_refused_before_the_download_and_any_paid_call(self):
        ref=self.store.put_file(document())
        envelope={'deadline':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime(time.time()+60))}
        request=parse(dumps({'method':'upsert','namespace':'team','tableName':'docs','id':'a','source':'a','content_hash':ref['sha256'],'bigfile':ref}))
        with self.assertRaisesRegex(RagError,'deadline_too_short'):self.app.run(request,envelope)
        self.assertEqual((self.store.paid,self.store.downloads),([],[]))

    # Citations.
    def test_a_hit_cites_its_heading_path_and_lines(self):
        text=document((6,section('Part 6',1).replace('Part 6 paragraph 0','assam tea')));self.put(text)
        hit=self.call(method='search',query='assam')['records'][0]
        mine=next(c for c in heading_chunks(text) if 'assam' in c['text'])
        self.assertEqual(hit['heading_path'],['Plan','Part 6']);self.assertEqual(hit['lines'],mine['lines'])
        lines=text.split('\n')[mine['lines'][0]-1:mine['lines'][1]]
        self.assertTrue(any('assam' in l for l in lines),'the cited lines hold the hit')
        self.assertEqual(lines[0],'## Part 6')
    def test_a_small_record_hit_keeps_todays_fields(self):
        self.call(method='upsert',id='small',source='small.md',data='Tea is a drink.')
        hit=self.call(method='search',query='tea')['records'][0]
        self.assertNotIn('heading_path',hit);self.assertNotIn('lines',hit);self.assertEqual(hit['chunk_index'],0)

    # Refusals, all before any paid call.
    def test_a_download_that_is_not_the_referenced_version_is_refused_unpaid(self):
        ref=self.store.put_file(document())
        self.store.files[(ref['record'],ref['name'])]=document((0,section('Part 0',1))).encode()
        with self.assertRaisesRegex(RagError,'bigfile_mismatch'):
            self.call(method='upsert',id='a',source='a',content_hash=ref['sha256'],bigfile=ref)
        self.assertEqual(self.store.paid,[])
    def test_an_absent_reference_is_refused_unpaid(self):
        ref=self.store.put_file(document());self.store.files.clear()
        with self.assertRaisesRegex(RagError,'bigfile_unavailable'):
            self.call(method='upsert',id='a',source='a',content_hash=ref['sha256'],bigfile=ref)
        self.assertEqual(self.store.paid,[])
    def test_the_reference_is_checked_when_parsed(self):
        ref=self.store.put_file(document())
        base={'method':'upsert','namespace':'t','tableName':'d','id':'a','source':'a','content_hash':ref['sha256']}
        for bad,word in (({**ref,'size':(1<<20)+1},'size'),({**ref,'extra':1},'needs record'),({k:ref[k] for k in ('record','name','size')},'needs record')):
            with self.assertRaisesRegex(RagError,word):parse(dumps({**base,'bigfile':bad}))
        with self.assertRaisesRegex(RagError,'content_hash must equal'):parse(dumps({**base,'content_hash':'0'*64,'bigfile':ref}))
        with self.assertRaisesRegex(RagError,'replaces data'):parse(dumps({**base,'bigfile':ref,'data':'x'}))
        with self.assertRaisesRegex(RagError,'text/markdown only'):parse(dumps({**base,'bigfile':ref,'mime-type':'text/plain'}))
    def test_classification_reads_the_first_32_kib(self):
        text=document();self.put(text)
        state=next(r['state'] for t,r in self.store.paid if t=='ai')
        self.assertEqual(state,text.encode()[:32<<10].decode('utf-8','ignore'))
    def test_a_large_upsert_gets_thirty_minutes_unless_its_sender_waits_less(self):
        seen=[]
        self.store.begin=lambda budget:seen.append(budget.end-time.monotonic())
        self.put(document())
        self.assertGreater(seen[-1],1700)
        self.call(method='get',id='Plans/big.md',check=True)
        self.assertLess(seen[-1],121)


class Download(unittest.TestCase):
    """The bus side of the reference: the daemon's refusals become named ones."""
    def blocks(self,error):
        bus=Mock();bus.record.return_value.bigfile.get.side_effect=error
        b=Blocks(bus,CFG);b.begin(Budget());return b
    def test_refusals_are_named(self):
        for status,word in ((404,'bigfile_unavailable'),(403,'bigfile_forbidden')):
            with self.assertRaisesRegex(RagError,word):self.blocks(Refusal(status,'no')).bigfile('#filescan/v@n','k',1<<20)
    def test_the_download_reads_the_named_record(self):
        bus=Mock();bus.record.return_value.bigfile.get.return_value=(b'abc',{})
        b=Blocks(bus,CFG);b.begin(Budget())
        self.assertEqual(b.bigfile('#filescan/v@n','k',1<<20),b'abc')
        bus.record.assert_called_with('#filescan/v@n');bus.record.return_value.bigfile.get.assert_called_with('k',max_size=1<<20)


if __name__=='__main__':
    unittest.main()
