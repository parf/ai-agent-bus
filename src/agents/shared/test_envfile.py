import json
from pathlib import Path
import unittest
from envfile import parse

ROOT = Path(__file__).resolve().parents[2]


class Envfile(unittest.TestCase):
    def test_shared_go_vectors(self):
        vectors = json.loads((ROOT/'internal/envfile/testdata/common.json').read_text())
        for vector in vectors:
            with self.subTest(vector=vector['name']):
                if vector.get('line', 0):
                    with self.assertRaisesRegex(ValueError, 'line '+str(vector['line'])) as error:
                        parse(vector['source'])
                    self.assertNotIn('PRIVATE_SENTINEL',str(error.exception))
                else:
                    want = [{'Key':v['Key'],'Value':v['Value'],'Required':v.get('Required',False)} for v in vector.get('want') or []]
                    self.assertEqual(parse(vector['source']),want)

    def test_quotes_required_and_nul(self):
        values=parse("A='a # b' # required because needed\nB=\"a\\nb\"\nC=x\n")
        self.assertEqual(values,[{'Key':'A','Value':'a # b','Required':True},
                                 {'Key':'B','Value':'a\nb','Required':False},
                                 {'Key':'C','Value':'x','Required':False}])
        with self.assertRaisesRegex(ValueError,'line 2: NUL'):
            parse('A=x\nB=PRIVATE_SENTINEL\0')
