"""Pin authored help text and JSON; each agent owns its own reviewed fixture.

Add help.golden.json beside a manifest to include it in this check. Fixtures
contain the owner-visible help, with only the release version and caller name
made independent of the machine running the check.
"""
from pathlib import Path
import json
import os
import unittest
from unittest.mock import patch

import metadata

AGENTS = Path(__file__).resolve().parents[1]


def rendered(directory):
    with patch.dict(os.environ, {}, clear=True), patch.object(metadata, 'VERSION', '<version>'):
        return {
            'text': metadata.answer(directory, roles=['owner']),
            'json': json.loads(metadata.answer(directory, 'help-json', roles=['owner'])),
        }


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
