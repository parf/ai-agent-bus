"""Pin authored help text and JSON; each agent owns its own reviewed fixture.

Add help.golden.json beside a manifest to include it in this check. Fixtures
contain the owner-visible help, with only the agent's own version and caller name
made independent of the machine running the check and of each deploy's bump.
"""
from pathlib import Path
import json
import os
import unittest
from unittest.mock import patch

import metadata

AGENTS = Path(__file__).resolve().parents[1]


def rendered(directory):
    """The help as reviewed: the agent's own version, which each deploy may bump, is the placeholder `<version>`."""
    own = json.loads((directory / 'agent.json').read_text())['version']
    with patch.dict(os.environ, {}, clear=True):
        text = metadata.answer(directory, roles=['owner'])
        value = json.loads(metadata.answer(directory, 'help-json', roles=['owner']))
    assert value['version'] == own, 'help reports the version of its own agent.json'
    value['version'] = '<version>'
    footer = [i for i, line in enumerate(text.split('\n')) if line == 'v' + own or line.startswith('v' + own + ' · ')]
    assert len(footer) == 1, 'the footer names the version once'
    lines = text.split('\n')
    lines[footer[0]] = lines[footer[0]].replace('v' + own, 'v<version>', 1)
    return {'text': '\n'.join(lines), 'json': value}


class GoldenHelp(unittest.TestCase):
    def test_authored_help_matches_reviewed_fixtures(self):
        fixtures = sorted(AGENTS.rglob('help.golden.json'))
        self.assertTrue(fixtures, 'no reviewed help fixtures found')
        for fixture in fixtures:
            with self.subTest(agent=str(fixture.parent.relative_to(AGENTS))):
                self.assertEqual(json.loads(fixture.read_text()), rendered(fixture.parent),
                                 'help changed without its reviewed golden fixture: ' + str(fixture))


if __name__ == '__main__':
    unittest.main()
