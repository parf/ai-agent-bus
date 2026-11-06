"""The agent pack's protocol declaration (protocol.json) and where it must agree.

The declaration is the pack's one statement of the daemon protocols its agents speak. The client libraries the pack bundles
check the daemon at connect against their own constant, so the declaration must equal those constants, read here from their
source files and never imported; and it must cover the protocols the daemon in this repository speaks. Every package ships it.
"""
import json
from pathlib import Path
import re
import unittest

HERE = Path(__file__).resolve().parent
SRC = HERE.parent


def declared():
    return json.loads((HERE / 'protocol.json').read_text())


def constant(path, pattern):
    text = (SRC / path).read_text()
    found = re.findall(pattern, text, re.M)
    if len(found) != 1:
        raise AssertionError(f'{path}: expected one {pattern!r}, found {found}')
    return int(found[0])


class PackProtocol(unittest.TestCase):
    def test_the_declaration_is_exactly_the_two_identity_fields(self):
        value = declared()
        self.assertEqual(set(value), {'protocol', 'protocol_min'})
        self.assertTrue(all(type(v) is int and v >= 0 for v in value.values()), value)
        self.assertLessEqual(value['protocol_min'], value['protocol'])

    def test_it_equals_what_each_bundled_client_library_speaks(self):
        value = declared()
        clients = {
            'python': constant('clients/python/agent_bus/protocol.py', r'^PROTOCOL = (\d+)\s*$'),
            'typescript': constant('clients/typescript/src/protocol.ts', r'^export const CLIENT_PROTOCOL = (\d+);\s*$'),
            'go': constant('clients/go/protocol.go', r'^const ClientProtocol = (\d+)\s*$'),
        }
        for name, protocol in clients.items():
            with self.subTest(client=name):
                self.assertEqual((value['protocol_min'], value['protocol']), (protocol, protocol),
                                 f'the pack declares {value}; the {name} client speaks only {protocol}: change both or neither')

    def test_it_covers_the_daemon_in_this_repository(self):
        value = declared()
        node = 'internal/protocol/node.go'
        daemon = constant(node, r'^const Protocol = (\d+)\s*$')
        daemon_min = constant(node, r'^const ProtocolMin = (\d+)\s*$')
        self.assertGreaterEqual(daemon_min, value['protocol_min'])
        self.assertLessEqual(daemon, value['protocol'])

    def test_no_bundled_agent_carries_a_client_of_its_own(self):
        # A Go client inside an agent would be a protocol constant this test does not read.
        self.assertEqual([p.name for p in HERE.rglob('go.mod')], [])
        self.assertEqual([str(p.relative_to(HERE)) for p in HERE.rglob('*.go')], [])

    def test_every_package_that_ships_the_pack_version_ships_the_declaration(self):
        scripts = sorted(p for p in HERE.rglob('package.sh') if 'agents/VERSION' in p.read_text())
        self.assertGreaterEqual(len(scripts), 18)
        for script in scripts:
            text = script.read_text()
            with self.subTest(package=str(script.relative_to(HERE))):
                want = 'agents/protocol.json' if script == HERE / 'package.sh' else 'support/agents/protocol.json'
                self.assertIn(want, text)
                self.assertRegex(text, r'protocol\.json" "[^"]*/' + re.escape(want) + '"')


if __name__ == '__main__':
    unittest.main()
