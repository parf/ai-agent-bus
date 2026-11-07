"""Pin authored help text and JSON; each agent owns its own reviewed fixture.

Add help.golden.json beside a manifest to include it in this check. Fixtures
contain the owner-visible help, with only the agent's own version and release day and
the caller name made independent of the machine running the check and of each deploy's bump.
"""
from pathlib import Path
import json
import os
import re
import unittest
from unittest.mock import patch

import metadata

AGENTS = Path(__file__).resolve().parents[1]


def rendered(directory):
    """The help as reviewed: the agent's own version and release day, which each deploy may bump, are the placeholders `<version>` and `<released>`."""
    manifest = json.loads((directory / 'agent.json').read_text())
    own, released = manifest['version'], manifest.get('released')
    with patch.dict(os.environ, {}, clear=True):
        text = metadata.answer(directory, roles=['owner'])
        value = json.loads(metadata.answer(directory, 'help-json', roles=['owner']))
    assert value['version'] == own, 'help reports the version of its own agent.json'
    value['version'] = '<version>'
    assert value.get('released') == released, 'help reports the release day of its own agent.json'
    if released: value['released'] = '<released>'
    footer = [i for i, line in enumerate(text.split('\n')) if line == 'v' + own or line.startswith('v' + own + ' · ')]
    assert len(footer) == 1, 'the footer names the version once'
    lines = text.split('\n')
    lines[footer[0]] = lines[footer[0]].replace('v' + own, 'v<version>', 1)
    if released:
        assert lines[footer[0]].count(' · released ' + released) == 1, 'the footer names the release day once, after the version'
        lines[footer[0]] = lines[footer[0]].replace(' · released ' + released, ' · released <released>', 1)
    return {'text': '\n'.join(lines), 'json': value}


class GoldenHelp(unittest.TestCase):
    def test_authored_help_matches_reviewed_fixtures(self):
        fixtures = sorted(AGENTS.rglob('help.golden.json'))
        self.assertTrue(fixtures, 'no reviewed help fixtures found')
        for fixture in fixtures:
            with self.subTest(agent=str(fixture.parent.relative_to(AGENTS))):
                self.assertEqual(json.loads(fixture.read_text()), rendered(fixture.parent),
                                 'help changed without its reviewed golden fixture: ' + str(fixture))

    def test_a_manifest_without_version_shows_none(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp, patch.dict(os.environ, {}, clear=True):
            Path(tmp, 'agent.json').write_text(json.dumps({'description': 'No version', 'license': 'LicenseRef-PolyForm-Strict-1.0.0', 'methods': {}}))
            self.assertNotIn('version', json.loads(metadata.answer(tmp, 'help-json', roles=['owner'])))
            text = metadata.answer(tmp, roles=['owner'])
            self.assertIsNone(re.search(r'(^|\n)v\d', text), text)
            self.assertIn('\nPolyForm Strict\n', text)
            Path(tmp, 'agent.json').write_text(json.dumps({'description': 'x', 'version': '2.3.4', 'license': 'LicenseRef-PolyForm-Strict-1.0.0', 'methods': {}}))
            self.assertIn('\nv2.3.4 · PolyForm Strict\n', metadata.answer(tmp, roles=['owner']))


if __name__ == '__main__':
    unittest.main()
