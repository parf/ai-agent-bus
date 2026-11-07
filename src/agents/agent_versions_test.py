"""Every bundled agent has its own version and release day: the pack contract refuses an agent.json without either.

A bundled agent is every agent.json under src/agents. Its `version` is a SemVer and its `released` a real YYYY-MM-DD day,
moved together by each deploy's bump. An agent that is not bundled may carry neither and then shows neither.
"""
import datetime
import json
from pathlib import Path
import re
import unittest

HERE = Path(__file__).resolve().parent


def problems(manifest):
    """What a bundled agent's manifest lacks or gets wrong, each naming its field."""
    found = []
    version = manifest.get('version')
    if not isinstance(version, str) or not re.fullmatch(r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)', version):
        found.append(f'version must be a MAJOR.MINOR.PATCH SemVer, not {version!r}')
    released = manifest.get('released')
    try:
        valid = isinstance(released, str) and re.fullmatch(r'\d{4}-\d{2}-\d{2}', released) and datetime.date.fromisoformat(released)
    except ValueError:
        valid = False
    if not valid:
        found.append(f'released must be a YYYY-MM-DD day, not {released!r}')
    return found


def manifests():
    return sorted(p for p in HERE.rglob('agent.json') if 'node_modules' not in p.parts)


class AgentVersions(unittest.TestCase):
    def test_every_bundled_agent_declares_version_and_released(self):
        paths = manifests()
        self.assertGreaterEqual(len(paths), 20, 'the bundled agents were not found')
        for path in paths:
            with self.subTest(agent=str(path.parent.relative_to(HERE))):
                self.assertEqual(problems(json.loads(path.read_text())), [])

    def test_a_missing_or_malformed_field_is_named(self):
        good = {'version': '1.0.0', 'released': '2026-11-07'}
        self.assertEqual(problems(good), [])
        self.assertEqual(problems({'released': '2026-11-07'}), ["version must be a MAJOR.MINOR.PATCH SemVer, not None"])
        self.assertEqual(problems({'version': '1.0.0'}), ["released must be a YYYY-MM-DD day, not None"])
        for bad in ('1.0', '1.0.0-rc1', '01.0.0', 1, ''):
            self.assertEqual(len(problems({**good, 'version': bad})), 1, bad)
            self.assertIn('version', problems({**good, 'version': bad})[0])
        for bad in ('2026-13-01', '2026-02-30', '2026-1-7', 'yesterday', '2026-11-07T00:00:00Z', 20261107, ''):
            self.assertEqual(len(problems({**good, 'released': bad})), 1, bad)
            self.assertIn('released', problems({**good, 'released': bad})[0])


if __name__ == '__main__':
    unittest.main()
