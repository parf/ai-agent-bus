"""The selection contract, without building or running any runtime fixture."""
import importlib.util
from pathlib import Path
import re
import unittest

spec = importlib.util.spec_from_file_location('select_checks', Path(__file__).with_name('smoke-select.py'))
selector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selector)
ROOT = Path(__file__).resolve().parent.parent

class Selection(unittest.TestCase):
    def plan(self, paths, mode='normal', targets=''):
        return selector.plan(ROOT, mode, targets, paths)

    def test_telegram_leaf_keeps_other_faces_fast(self):
        p = self.plan(['src/agents/telegram/receive.ts'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_slack'], 'fast')
        self.assertEqual(p['modes']['A'], 'fast')
        self.assertEqual(self.plan(['src/agents/telegram/receive.ts'], 'fast')['checks']['agent_slack'], 'skip')

    def test_shared_router_selects_both_platforms(self):
        p = self.plan(['src/agents/shared/router.ts'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_slack'], 'normal')

    def test_helper_outside_leaf_prefixes_broadens_the_subsystem(self):
        p = self.plan(['src/agents/standalone/metadata.py'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_systemd'], 'normal')

    def test_lower_layer_includes_reverse_dependencies(self):
        p = self.plan(['src/internal/protocol/envelope.go'])
        for letter in 'ABCEFGHIJKL': self.assertEqual(p['modes'][letter], 'normal', letter)
        p = self.plan(['src/internal/core/core.go'])
        self.assertEqual(p['modes']['I'], 'normal')
        self.assertEqual(p['checks']['agent_telegram'], 'normal')

    def test_version_and_every_root_changelog_are_always_ignored(self):
        for paths in (['src/internal/version/VERSION'], ['CHANGELOG.0.9.md'], ['CHANGELOG.manual.md']):
            p = self.plan(paths)
            self.assertEqual(p['fallback'], 'nothing changed: all fast')
            self.assertEqual(set(p['modes'].values()), {'fast'})
            self.assertEqual(set(p['checks'].values()), {'fast','skip'})
        p = self.plan(['CHANGELOG.0.9.md', 'src/agents/telegram/config.ts'])
        self.assertEqual(p['checks']['agent_slack'], 'fast')

    def test_container_image_is_its_own_subsystem(self):
        p = self.plan(['src/container/Containerfile'])
        self.assertEqual({c for c, m in p['modes'].items() if m != 'fast'}, {'Q'})
        self.assertEqual({c for c, l in p['checks'].items() if l == 'normal'}, {'container_image'})
        self.assertEqual(self.plan([], 'normal', 'Q')['checks']['container_image'], 'normal')

    def test_shared_or_unknown_fails_closed(self):
        for path in ['src/smoke.sh', 'src/go.mod', 'src/web/bun.lock', 'new-unknown-file']:
            p = self.plan([path])
            self.assertEqual(set(p['checks'].values()), {'normal'}, path)

    def test_override_and_release(self):
        p = self.plan([], 'slow', 'L')
        self.assertEqual(p['checks']['agent_slack'], 'slow')
        self.assertEqual(p['checks']['web_ts'], 'fast')
        self.assertEqual(set(self.plan([], 'slow', 'all')['checks'].values()), {'slow'})
        self.assertEqual(self.plan([], 'normal', 'A,F')['modes']['F'], 'normal')
        for target in ['Z', 'L,I,', '--normal', 'L L']:
            with self.assertRaises(ValueError): self.plan([], targets=target)

    def test_docs_and_clean_tree_do_not_get_empty_green(self):
        for paths in [[], ['docs/00-overview.md'], ['Plans/R1.0-Release/TODO.md']]:
            p = self.plan(paths, 'fast')
            self.assertTrue(any(level != 'skip' for level in p['checks'].values()))

    def test_fast_uses_only_explicit_allowlists_and_normal_keeps_regressions(self):
        import json
        config = json.loads((ROOT/'src/smoke-map.json').read_text())
        p = self.plan(['src/smoke.sh'], 'fast')
        self.assertEqual({check for check,level in p['checks'].items() if level != 'skip'},set(config['fast']))
        p = self.plan(['src/smoke.sh'])
        self.assertEqual(set(p['checks'].values()),{'normal'})

    def test_map_covers_every_shard_and_core_section(self):
        script = (ROOT/'src/smoke.sh').read_text()
        names = re.search(r'SHARDS=\((.*?)\)', script, re.S)[1].split()
        expected = (set(names)-{'core'}) | {f'core_{n}' for n in range(1,11)}
        actual = set(self.plan([])['checks'])
        self.assertEqual(actual, expected)
        for prefix, letter in selector.prefixes(ROOT):
            self.assertIn(letter, selector.ALL)
        self.assertEqual(self.plan(['src/cmd/agent-bus-runner/serve.go'])['modes']['F'], 'normal')

if __name__ == '__main__': unittest.main()
