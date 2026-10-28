"""RT32: every bundled agent has a page in one structure, an index entry and a docs link."""
import json
import re
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import agent_docs  # noqa: E402

HEADINGS = ["Why you'd want it", 'What it does', "You'll need", 'Try this first', 'Use examples',
            'Installing it', 'Configuration', 'Secret keys', 'Who may use it', 'Troubleshooting']
PAGES = agent_docs.ROOT / 'docs' / 'agents'


def headings(text):
    """Level-two headings outside fenced code blocks."""
    found, fence = [], None
    for line in text.splitlines():
        marker = re.match(r'^\s*(```|~~~)', line)
        if marker:
            fence = None if fence == marker.group(1) else fence or marker.group(1)
        elif fence is None and line.startswith('## '):
            found.append(line[3:].strip())
    return found


def in_order(want, have):
    it = iter(have)
    return all(any(h == w for h in it) for w in want)


def table_targets(text):
    """Pages linked from the first cell of the index table rows, fragments excluded."""
    return {m.group(1) for m in re.finditer(r'(?m)^\| \[[^\]]+\]\(([a-z0-9-]+\.md)\) \|', text)}


class AgentDocs(unittest.TestCase):
    def setUp(self):
        self.agents = agent_docs.agents()
        self.index = agent_docs.INDEX.read_text()
        self.source = agent_docs.go_const('SourceURL') + '/docs/agents/'

    def problems(self, name, page, docs):
        out = []
        path = PAGES / page
        if not path.is_file():
            return [f'{name}: no page {page}']
        have = headings(path.read_text())
        missing = [h for h in HEADINGS if h not in have]
        if missing or not in_order(HEADINGS, have):
            out.append(f'{name}: {page} headings missing or out of order: {missing}')
        if docs != self.source + page:
            out.append(f'{name}: docs is {docs!r}, want {self.source + page}')
        if name != 'bus' and page not in table_targets(self.index):
            out.append(f'{name}: no index table row linking {page}')
        return out

    def test_every_agent_has_page_structure_index_row_and_docs(self):
        found = [p for n, page, _, docs in self.agents if n not in agent_docs.PENDING for p in self.problems(n, page, docs)]
        self.assertEqual(found, [])

    def test_pending_entries_are_still_needed(self):
        names = {n for n, *_ in self.agents}
        for name in agent_docs.PENDING:
            self.assertIn(name, names, f'pending {name} names no bundled agent')
            row = next(r for r in self.agents if r[0] == name)
            self.assertNotEqual(self.problems(*row[:2], row[3]), [], f'{name} complies: remove it from PENDING')

    def test_the_list_is_generated_and_complete(self):
        start, end = agent_docs.block(self.index)
        self.assertEqual(self.index[start:end], agent_docs.render())
        lines = [l for l in self.index[start:end].splitlines() if l.startswith('- ')]
        self.assertEqual(len(lines), len(agent_docs.manifests()) + 1)  # + #bus
        self.assertGreaterEqual(len(lines), 14)
        # Independent of the generator: the git line from its own manifest, #bus pinned.
        git = json.loads((agent_docs.AGENTS / 'standalone/git/agent.json').read_text())
        self.assertIn('- [`#git`](git.md) — ' + git['description'], lines)
        self.assertIn('- [`#bus`](bus.md) — The daemon on the bus; read-only methods use your authority', lines)

    def test_table_rows_name_only_bundled_agent_pages(self):
        pages = {page for _, page, *_ in self.agents}
        self.assertEqual(table_targets(self.index) - pages, set())

    def test_headings_inside_fences_do_not_count(self):
        self.assertEqual(headings('# T\n\n```md\n## What it does\n```\n\n## Why you\'d want it\n'), ["Why you'd want it"])


if __name__ == '__main__':
    unittest.main()
