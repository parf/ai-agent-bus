"""RAG 0.3 contract checks; static discovery never reaches a backend or provider."""
import contextlib
import io
import json
from pathlib import Path
import shlex
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parents[1]/'clients/python'),str(HERE.parent/'shared')]
from rules import parse,dumps,RagError,NamedRefusal,METHODS,SEARCH_MODES,CATALOGUE_MAX
from rag import Agent,VERSION
from test_rag import CFG


class Contract(unittest.TestCase):
    def request(self,agent,value):
        with contextlib.redirect_stderr(io.StringIO()):
            return agent.handle(SimpleNamespace(ask=None,roles=[],body=dumps(value),envelope={}))

    def test_capabilities_need_neither_namespace_configuration_nor_backend(self):
        for cfg,model in ((None,None),(CFG,CFG['embedding_model'])):
            agent=Agent(None,cfg)
            with patch('core.Core.run',side_effect=AssertionError('backend must not run')),patch('core.Core.verify',side_effect=AssertionError('verify must not run')):
                answer=json.loads(self.request(agent,{'method':'capabilities'}))
            self.assertEqual(answer['version'],VERSION)
            self.assertEqual(answer['methods'],list(METHODS))
            self.assertEqual(answer['modes'],list(SEARCH_MODES))
            self.assertEqual(answer['embedding_model'],model)
            self.assertLess(answer['limits']['catalogue_bytes'],512<<10)
            self.assertEqual(answer['limits']['catalogue_bytes'],CATALOGUE_MAX)
            self.assertNotIn('ready',answer)

    def test_every_data_method_requires_a_real_namespace(self):
        for method in set(METHODS)-{'capabilities','namespaces'}:
            for ns in (None,'',' ','*'):
                with self.subTest(method=method,namespace=ns),self.assertRaises(RagError):
                    parse(dumps({'method':method,'namespace':ns}))
        self.assertEqual(parse('{"method":"keyword_index","namespace":"a"}'),{'method':'keyword_index','namespace':'a'})
        for method in ('capabilities','namespaces'):
            with self.assertRaises(RagError):parse(dumps({'method':method,'namespace':'a'}))

    def test_search_modes_are_explicit_and_never_coerced(self):
        base={'method':'search','namespace':'a','query':'tea'}
        self.assertEqual(parse(dumps(base))['mode'],'keyword')
        for mode in SEARCH_MODES:self.assertEqual(parse(dumps(base|{'mode':mode}))['mode'],mode)
        for mode in ('dense','auto',None,True):
            with self.assertRaises(NamedRefusal) as c:parse(dumps(base|{'mode':mode}))
            self.assertEqual(c.exception.code,'unsupported_mode')
        self.assertEqual(parse(dumps(base|{'query':'???','mode':'semantic'}))['mode'],'semantic')
        with self.assertRaises(RagError):parse(dumps(base|{'query':'???'}))

    def test_manifest_examples_and_vocabulary_are_the_parser_contract(self):
        manifest=json.loads((HERE/'agent.json').read_text())
        self.assertEqual(set(manifest['methods']),set(METHODS))
        for method,info in manifest['methods'].items():
            request=parse(shlex.split(info['example'])[-1])
            self.assertEqual(request['method'],method)
            with self.assertRaises(RagError):parse(dumps(request|{'unknown':True}))
        with self.assertRaises(NamedRefusal) as c:parse('{"method":"not-a-method"}')
        self.assertEqual(c.exception.code,'unsupported_method')

    def test_provenance_is_required_and_supplied_hash_is_verified(self):
        base={'method':'upsert','namespace':'a','tableName':'t','id':'x','data':'tea'}
        with self.assertRaises(RagError):parse(dumps(base))
        good=base|{'source':'notes/tea.md'}
        self.assertEqual(parse(dumps(good))['source'],'notes/tea.md')
        for value in ('0'*64,None,True):
            with self.assertRaises(RagError):parse(dumps(good|{'content_hash':value}))


if __name__=='__main__':unittest.main()
