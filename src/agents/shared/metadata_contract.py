"""Executable grammar checks shared by each bundled Python agent's tests."""
import ast
from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
from unittest.mock import patch
from metadata import data, answer


def check(test, directory, agent, name):
    directory = Path(directory)
    manifest = json.loads((directory / 'agent.json').read_text())
    methods = set(manifest['methods'])
    if name in ('systemd', 'git'):
        actual = set(agent.TIERS) | {'help'}
    elif name == 'logwatch': actual = set(agent.FIELDS)
    elif name == 'server-info': actual = set(agent.COMMANDS) | {'all','help'}
    elif name == 'debug': actual = set(agent.METHODS)
    else:
        tree = ast.parse((directory / 'debug.py').read_text())
        actual = {node.comparators[0].value for node in ast.walk(tree) if isinstance(node, ast.Compare)
                  and isinstance(node.left, ast.Name) and node.left.id == 'command'
                  and isinstance(node.ops[0], ast.Eq) and isinstance(node.comparators[0], ast.Constant)}
    test.assertEqual(methods, actual, name + ': manifest methods differ from executable grammar')
    if name in ('git','logwatch'):
        fields = agent.FIELDS
        for method in actual - {'help'}:
            accepted = set(fields.get(method, set()))
            if name == 'git' and method != 'list': accepted.add('repo')
            test.assertEqual({a['name'] for a in manifest['methods'][method]['args']}, accepted, method)
    if name == 'systemd':
        test.assertEqual(manifest['methods']['list']['args'][0]['type'], list(agent.TYPES))
    with patch.dict(os.environ, {'AB_ROLE_OWNER':'1'}, clear=True):
        rendered = data(directory)
        test.assertEqual(set(rendered['methods']), actual)
        whole = json.loads(answer(directory, 'help-json'))
        test.assertEqual(set(whole['methods']), actual)
        test.assertEqual(set(whole['config']), set(manifest.get('config', {})))
        test.assertIsInstance(whole['secrets'], list)
        test.assertIn('help-json', whole['asks'])
        for declared in whole['secrets']:
            test.assertEqual(set(declared), {'name','help','required'})
            test.assertTrue(declared['help'])
        text = answer(directory)
        for method in actual - {'help'}:
            usage = manifest['methods'][method].get('text', method)
            test.assertTrue(any(line.removeprefix('[destructive] ').startswith(usage + ' ') or line.removeprefix('[destructive] ').startswith(usage + ':') for line in text.splitlines()), method)
        test.assertNotIn('help:', text)
        test.assertNotIn('Version:', text)
        test.assertIn('PolyForm-NC · github.com/parf/ai-agent-bus', text)
        for shared_name, definition in manifest.get('arguments', {}).items():
            if any(any(a['name'] == shared_name and not a.get('json_only') for a in m.get('args', [])) for m in rendered['methods'].values()):
                test.assertEqual(text.count(shared_name + ' ('), 1, shared_name)
        test.assertIn('Examples\n', text)
        test.assertIn('More: agent-bus help <name> config | secret | json', text)
        with patch.dict(os.environ, {'AGENT_BUS_NAME': '#fixture/instance@t'}):
            test.assertIn("More: agent-bus help '#fixture/instance@t' config | secret | json", answer(directory))
        test.assertNotIn('version', manifest)
        test.assertEqual(manifest['license'], 'PolyForm-Noncommercial-1.0.0')
        for key in ('description','author','homepage'): test.assertTrue(manifest[key])
        for method in actual - {'help'}:
            args = {'unit':'nginx.service','repo':'fixture','query':'needle','age':'1m',
                    'label':'before','target':'#target@t','id':'abc','paths':['README.md'],'message':'fixture','seconds':0,'KB':1}
            call = {'method':method}
            call.update({a['name']:args[a['name']] for a in manifest['methods'][method]['args'] if not a.get('optional')})
            if name == 'systemd': test.assertEqual(agent.call(json.dumps(call))[0], method)
            elif name == 'git': test.assertEqual(agent.parse(json.dumps(call))['method'], method)
            elif name == 'logwatch': test.assertEqual(agent.parse(json.dumps(call))['method'], method)
            elif name == 'server-info':
                with patch.object(agent, 'collect', return_value=[]), patch.object(agent.os, 'geteuid', return_value=0):
                    result = json.loads(agent.answer(json.dumps(call)))
                    test.assertEqual(set(result), set(agent.COMMANDS) if method == 'all' else {method})
            elif name == 'debug' and method in ('hello','env'):
                with patch.dict(os.environ, {'AB_ROLE_FIXTURE':'1'}, clear=True):
                    test.assertIn('hello, ' if method == 'hello' else 'AB_ROLE_FIXTURE=1', agent.answer('hello sample' if method == 'hello' else 'env'))

    # Cumulative roles are explicit unions, never inferred by the renderer.
    if name in ('systemd', 'git'):
        role, lower, method = ('admin','operate','restart') if name == 'systemd' else ('commit','deploy','pull')
        with patch.dict(os.environ, {'AB_ROLE_' + role.upper():'1'}, clear=True):
            test.assertIn(method, data(directory)['methods'])
        test.assertEqual(manifest['methods'][method]['role'], [lower, role])
    return sorted(actual)


if __name__ == '__main__':
    import importlib.util
    import sys
    import unittest
    directory = Path(sys.argv[1]).resolve()
    name = sys.argv[2]
    sys.path.insert(0, str(directory))
    script = {'systemd':'systemd.py', 'git':'git_agent.py', 'server-info':'server-info.py', 'logwatch':'logwatch.py', 'debug':'debug.py'}[name]
    spec = importlib.util.spec_from_file_location('agent_under_test', directory / script)
    agent = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(agent)
    print(json.dumps(check(unittest.TestCase(), directory, agent, name)))
