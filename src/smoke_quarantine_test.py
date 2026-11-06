#!/usr/bin/env python3
"""The flaky-check quarantine (J120): what the gate files and skips, and what smoke honours."""
import datetime
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SRC = Path(__file__).resolve().parent
ROOT = SRC.parent
spec = importlib.util.spec_from_file_location('smoke_quarantine', SRC/'smoke-quarantine.py')
q = importlib.util.module_from_spec(spec)
spec.loader.exec_module(q)

SMOKE = '''ok_exit "a web title" "$rc"
has "an install title" "$out" 'x'
'''
GO_TEST = 'package core\nimport "testing"\nfunc TestCoreRace(t *testing.T) {}\nfunc TestCLIRestart(t *testing.T) {}\n'
ISSUES = '''# Issues — Nov 6, 2026 · gate flakes

## Gate flakes

- ⚪ J130 an open item.
- ✅ J131 a closed item.
'''
# A first run with a Go failure in a ⚠️ package and one in the CLI, a named title in a
# non-⚠️ shard, a title in a ⚠️ shard, a fast failure that names nothing, and a
# unittest line that is a tool's output, not a check.
LOG = '''== 2026-11-06 from #w: b T-1
[go_unit]   FAIL go race: exit 1
[go_unit]     --- FAIL: TestCoreRace (0.10s)
[go_unit]     FAIL\tgithub.com/parf/ai-agent-bus/internal/core\t1.0s
[go_unit]     --- FAIL: TestCLIRestart (5.00s)
[go_unit]         --- FAIL: TestCLIRestart/sub (5.00s)
[go_unit]     FAIL\tgithub.com/parf/ai-agent-bus/cmd/agent-bus\t6.0s
[web_ts]   FAIL: test_x (mod.Class.test_x)
[web_ts]   FAIL a web title: exit 1
[agent_server_info]   FAIL an install title: [out] lacks [x]
[agent_slack] FAIL fast command agent_slack (nonzero exit or empty test selection)
passed 10, failed 4
== rerun after rc=1
[web_ts]   FAIL a rerun-only title: exit 1
passed 14, failed 0
'''


class Tree:
    """A small repository: the real selector and subsystem map, a stub smoke script."""

    def __init__(self):
        self.dir = tempfile.TemporaryDirectory()
        self.root = Path(self.dir.name)
        (self.root/'src/agents').mkdir(parents=True)
        (self.root/'src/internal/core').mkdir(parents=True)
        (self.root/'Plans/issues/2026-11').mkdir(parents=True)
        (self.root/'Plans/issues/done/2026-11').mkdir(parents=True)
        for f in ('src/smoke-select.py', 'src/go.mod', 'Plans/subsystems.md'):
            shutil.copy(ROOT/f, self.root/f)
        (self.root/'src/smoke.sh').write_text(SMOKE)
        (self.root/'src/agents/smoke.sh').write_text('')
        (self.root/'src/internal/core/core_test.go').write_text(GO_TEST)
        (self.root/'Plans/issues/2026-11/2026-11-06.md').write_text(ISSUES)
        (self.root/'Plans/issues/done/2026-11/2026-11-05.md').write_text('- ⚪ J132 moved to done.\n')
        self.map({})

    def map(self, quarantine):
        config = {'reverse': {}, 'fast': {}, 'checks': {
            'core_1': {'tags': 'ABDEH'}, 'go_unit': {'tags': 'ABCDEFGHM'}, 'web_ts': {'tags': 'I'},
            'agent_server_info': {'tags': 'G'}, 'agent_slack': {'tags': 'L'}}}
        if quarantine:
            config['quarantine'] = quarantine
        (self.root/'src/smoke-map.json').write_text(json.dumps(config, indent=2)+'\n')

    def config(self):
        return json.loads((self.root/'src/smoke-map.json').read_text())


class Check(unittest.TestCase):
    def setUp(self):
        self.t = Tree()
        self.addCleanup(self.t.dir.cleanup)

    def test_the_tree_quarantine_is_valid(self):
        self.assertEqual(q.check(ROOT), [])

    def test_open_entries_pass(self):
        self.t.map({'web_ts': {'a web title': 'J130'}, 'go_unit': {'TestCLIRestart': 'J130'}})
        self.assertEqual(q.check(self.t.root), [])

    def test_a_closed_moved_or_missing_j_fails(self):
        for j, says in (('J131', 'is ✅'), ('J132', 'closed or missing'), ('J999', 'closed or missing'), ('130', 'not a J id')):
            with self.subTest(j=j):
                self.t.map({'web_ts': {'a web title': j}})
                problems = q.check(self.t.root)
                self.assertEqual(len(problems), 1, problems)
                self.assertIn(says, problems[0])

    def test_a_name_the_tree_no_longer_holds_fails(self):
        self.t.map({'web_ts': {'a renamed title': 'J130'}, 'go_unit': {'TestGone': 'J130'}, 'no_shard': {'a web title': 'J130'}})
        problems = q.check(self.t.root)
        self.assertEqual(len(problems), 3, problems)
        self.assertTrue(any('no assertion titled' in p for p in problems))
        self.assertTrue(any('no Go test TestGone' in p for p in problems))
        self.assertTrue(any('no shard no_shard' in p for p in problems))


class Failures(unittest.TestCase):
    def setUp(self):
        self.t = Tree()
        self.addCleanup(self.t.dir.cleanup)

    def test_only_the_first_run_is_read_and_each_failure_gets_its_finest_name(self):
        items = {(i['shard'], i['name']): i for i in q.failures(self.t.root, q.first_run(LOG))}
        self.assertEqual(set(items), {
            ('go_unit', 'TestCoreRace'), ('go_unit', 'TestCLIRestart'),
            ('web_ts', 'a web title'), ('agent_server_info', 'an install title'), ('agent_slack', None)})
        self.assertEqual(items['go_unit', 'TestCoreRace']['risky'], True)  # internal/core is A
        self.assertEqual(items['go_unit', 'TestCLIRestart']['risky'], False)  # cmd/ is H
        self.assertEqual(items['web_ts', 'a web title']['risky'], False)
        self.assertEqual(items['agent_server_info', 'an install title']['risky'], True)  # tagged G
        self.assertEqual(items['agent_slack', None]['kind'], 'unnamed')

    def test_a_title_the_tree_does_not_hold_is_unnamed(self):
        items = q.failures(self.t.root, '[web_ts]   FAIL a made-up title: exit 1\n')
        self.assertEqual([(i['name'], i['kind']) for i in items], [(None, 'unnamed')])

    def test_a_go_test_whose_package_line_was_cut_is_treated_as_risky(self):
        items = q.failures(self.t.root, '[go_unit]     --- FAIL: TestCLIRestart (1s)\n')
        self.assertEqual([(i['name'], i['risky']) for i in items], [('TestCLIRestart', True)])


class Apply(unittest.TestCase):
    def setUp(self):
        self.t = Tree()
        self.addCleanup(self.t.dir.cleanup)
        self.log = self.t.root/'gate.log'
        self.log.write_text(LOG)

    def test_safe_tests_are_quarantined_and_the_rest_filed_open(self):
        lines, paths = q.apply(self.t.root, self.log, 'ab-w1a/T-1', datetime.date(2026, 11, 6))
        self.assertEqual(lines[0], 'RERUN-GREEN go_unit, web_ts, agent_server_info, agent_slack: quarantined 2 with a J-item')
        # J ids follow the highest in Plans (J132), in item order.
        self.assertEqual(lines[1:], [
            'OPEN J133: go_unit / TestCoreRace not quarantined (⚠️ NEEDS LEAD)',
            'OPEN J136: agent_server_info / an install title not quarantined (⚠️ NEEDS LEAD)',
            'OPEN J137: agent_slack not quarantined (UNNAMED)'])
        self.assertEqual(self.t.config()['quarantine'], {'go_unit': {'TestCLIRestart': 'J134'}, 'web_ts': {'a web title': 'J135'}})
        issues = (self.t.root/'Plans/issues/2026-11/2026-11-06.md').read_text()
        self.assertTrue(issues.startswith(ISSUES.rstrip('\n')+'\n\n## Gate quarantine\n\n- ⚪ J133 '))
        self.assertEqual([int(n) for n in __import__('re').findall(r'^- ⚪ J(\d+) ', issues, 8)], [130, 133, 134, 135, 136, 137])
        self.assertIn('NEEDS LEAD', issues.split('J133', 1)[1].split('- ⚪', 1)[0])
        self.assertEqual(paths, ['src/smoke-map.json', 'Plans/issues/2026-11/2026-11-06.md'])
        # What the gate wrote is itself a valid quarantine.
        self.assertEqual(q.check(self.t.root), [])

    def test_nothing_safe_leaves_the_map_alone_and_says_so(self):
        self.log.write_text('[agent_slack] FAIL fast command agent_slack (x)\n== rerun after rc=1\n')
        before = (self.t.root/'src/smoke-map.json').read_text()
        lines, _ = q.apply(self.t.root, self.log, 'b', datetime.date(2026, 11, 7))
        self.assertEqual(lines, ['RERUN-GREEN agent_slack: nothing quarantined', 'OPEN J133: agent_slack not quarantined (UNNAMED)'])
        self.assertEqual((self.t.root/'src/smoke-map.json').read_text(), before)
        self.assertTrue((self.t.root/'Plans/issues/2026-11/2026-11-07.md').read_text().startswith('# Issues — Nov 7, 2026\n\n## Gate quarantine\n\n- ⚪ J133 '))


class Smoke(unittest.TestCase):
    """smoke.sh's own lines: the shard reads its entries and the helpers honour them."""

    def run_smoke_lines(self, entries, shard, script):
        text = (SRC/'smoke.sh').read_text()
        block = text[text.index('declare -A QUAR\n'):]
        block = block[:block.index('\nfi\n')+4]
        helpers = '\n'.join(l for l in text.splitlines() if l.startswith(('quarantined() ', 'ok_exit() ', 'has() ')))
        with tempfile.NamedTemporaryFile('w', suffix='.tsv') as f:
            f.write(''.join('\t'.join(e)+'\n' for e in entries))
            f.flush()
            return subprocess.run(['bash', '-c', f'GOFLAGS=-p=4; SHARD={shard}; SMOKE_QUARANTINE={f.name}; pass=0; fail=0; skipped=0\n{block}\n{helpers}\n{script}'],
                                  capture_output=True, text=True, check=True).stdout

    def test_a_quarantined_title_is_counted_as_skipped_and_another_still_fails(self):
        out = self.run_smoke_lines([('web_ts', 'a web title', 'J5'), ('other', 'b', 'J6')], 'web_ts',
                                   'ok_exit "a web title" 1; ok_exit "b" 1; has "c" x y; echo "$pass $fail $skipped"')
        self.assertEqual(out.splitlines(), ['  QUARANTINED a web title (J5)', '  FAIL b: exit 1', '  FAIL c: [x] lacks [y]', '0 2 1'])

    def test_go_names_reach_goflags_and_only_this_shards(self):
        out = self.run_smoke_lines([('go_unit', 'TestA', 'J5'), ('go_unit', 'TestB', 'J6'), ('core', 'TestC', 'J7')], 'go_unit', 'echo "$GOFLAGS"')
        self.assertEqual(out.strip(), '-p=4 -skip=^(TestA|TestB)$')
        out = self.run_smoke_lines([('core', 'TestC', 'J7')], 'go_unit', 'echo "$GOFLAGS"')
        self.assertEqual(out.strip(), '-p=4')


if __name__ == '__main__':
    unittest.main()
