"""A fast green must include an allowlisted test, not an empty match."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, Mock

spec = importlib.util.spec_from_file_location('fast_checks', Path(__file__).with_name('smoke-fast.py'))
fast = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fast)

class Fast(unittest.TestCase):
    def test_empty_bun_match_and_command_failure_are_not_green(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bun = root/'bun'
            (root/'smoke-map.json').write_text(json.dumps({'fast':{'probe':[{'argv':['bun','test']} ]}}))
            for body, expected in [('echo " 0 pass"',1), ('echo " 1 pass"; exit 1',1), ('echo " 1 pass"',0)]:
                bun.write_text('#!/bin/sh\n'+body+'\n')
                bun.chmod(0o700)
                with patch.object(fast,'ROOT',root), patch.dict(os.environ,{'PATH':directory+':'+os.environ['PATH']}), contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(fast.run('probe'),expected,body)
            with patch.object(fast,'ROOT',root),contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(fast.run('missing'),1)

    def test_budget_is_shared_across_commands(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root/'smoke-map.json').write_text(json.dumps({'fast':{'probe':[{'argv':['probe']},{'argv':['probe']}]}}))
            process = Mock(returncode=0)
            process.communicate.return_value = ('ok\n',None)
            with patch.object(fast,'ROOT',root), patch.object(fast.subprocess,'Popen',return_value=process), patch.object(fast.time,'monotonic',side_effect=[0,0,14,14.1]), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(fast.run('probe'),0)
            self.assertEqual([call.kwargs['timeout'] for call in process.communicate.call_args_list],[15,1])

if __name__ == '__main__': unittest.main()
