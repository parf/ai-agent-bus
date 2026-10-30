"""The docs-order checker: every page answers why, what you get, how to use it and only then the details.

The rule is CLAUDE.md "Writing conventions" and Plans/R1.0-Release/docs-rewrite.md. This file holds the
checker, the tests that prove it can fail, and the list of pages that must pass today.

  python3 src/agents/docs_order_test.py --report [file ...]   every page's failures, never fails
  python3 -m unittest src/agents/docs_order_test.py           the checker's own tests, and ENFORCED pages

Pages join ENFORCED as their batch lands; until then the report says how far the docs are.
"""
import re
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

# A page is rewritten into four parts, in this order. A part starts at a heading or a bold label
# (`## Why`, `**Why:**`) whose first words are one of these.
PARTS = ('why', 'get', 'how', 'technical')
LABELS = {
    'why': ('why', "why you'd want it", 'why it exists', 'problem'),
    'get': ('what you get', 'what it does', 'outcome'),
    'how': ('how to use it', 'try this first', 'use examples', 'how to start', 'how'),
    'technical': ('technical details', 'technical', 'internals', 'details'),
}
# Internal terms a lead must not open with (CLAUDE.md: broker, registry, daemon, queue, wire, storage
# detail; no language or library). The lead is the first sentence of the why part.
JARGON = ('broker', 'registry', 'daemon', 'queue', 'wire', 'storage', 'sqlite', 'goroutine', 'protocol',
          'bun', 'typescript', 'python', 'golang', 'mutex', 'kv store', 'ndjson', 'json schema')
FIRST_FIFTH = 0.20
FIRST_FIFTH_MIN = 600     # characters: a short page's fifth is not a few words
WALL = 300                # characters in one prose paragraph; about three wrapped lines is 240

# Not rewritten: the contract, the index and the vocabulary (docs-rewrite.md "Not rewritten").
EXEMPT = ('docs/constitution.md', 'docs/decisions.md', 'docs/glossary.md')

# Pages that must pass. Each batch of the docs rewrite adds its pages here as it lands.
ENFORCED = []

KINDS = ('order', 'missing-part', 'first-fifth', 'empty-example', 'no-example', 'jargon-lead', 'wall')


def label_of(line):
    """The part a line starts, or None: a heading or a bold label whose words begin with a known label."""
    m = re.match(r'^\s{0,3}#{1,6}\s+(.*)$', line) or re.match(r'^\s*(?:[-*]\s+)?(?:\d+\.\s+)?\*\*([^*]+?)\*\*(.*)$', line)
    if not m:
        return None
    words = re.sub(r"[*_`:]|[\U0001F300-\U0001FAFF☀-➿]", '', m.group(1)).strip().lower()
    for part in PARTS:
        # longest label first so "how to use it" is not read as "how"
        for label in sorted(LABELS[part], key=len, reverse=True):
            # a one-word label must be the whole heading ("How it is stored" is not "How")
            longer = len(label.split()) > 1 and re.match(re.escape(label) + r'(?:[ (,]|$)', words)
            if words == label or longer:
                return part
    return None


def split_blocks(text):
    """(kind, start, text) for fenced code, <details> bodies and prose lines, in order; offsets are characters."""
    out, pos, fence, buf, fstart = [], 0, None, [], 0
    for line in text.splitlines(keepends=True):
        stripped = line.strip()
        marker = re.match(r'^\s*(```+|~~~+)', line)
        if fence is None and marker:
            fence, buf, fstart = marker.group(1)[:3], [], pos
        elif fence is not None and stripped.startswith(fence):
            out.append(('fence', fstart, ''.join(buf)))
            fence = None
        elif fence is not None:
            buf.append(line)
        else:
            out.append(('line', pos, line))
        pos += len(line)
    return out


def body_of(text):
    """The page after its first heading line (the title), with the offset it starts at."""
    m = re.search(r'(?m)^# .*\n', text)
    return (text[m.end():], m.end()) if m else (text, 0)


def check(text):
    """Failures of one page: a list of (kind, message). Empty means the page follows the rule."""
    failures = []
    body, _ = body_of(text)
    blocks = split_blocks(body)
    # Where each part starts: its first marker outside code.
    starts = {}
    for kind, pos, line in blocks:
        if kind == 'line':
            part = label_of(line)
            if part and part not in starts:
                starts[part] = pos
    missing = [p for p in PARTS if p not in starts]
    if missing:
        failures.append(('missing-part', 'no ' + ', '.join(missing) + ' part'))
    present = [p for p in PARTS if p in starts]
    ordered = sorted(present, key=lambda p: starts[p])
    if ordered != present:
        failures.append(('order', 'parts out of order: ' + ' < '.join(ordered) + ' (want ' + ' < '.join(present) + ')'))
    limit = max(FIRST_FIFTH * len(body), FIRST_FIFTH_MIN)
    for part in ('why', 'get'):
        if part in starts and starts[part] > limit:
            failures.append(('first-fifth', part + ' starts after the first fifth of the page'))
    if 'why' not in starts:
        failures.append(('first-fifth', 'no why in the first fifth'))
    if 'how' in starts and starts['how'] > limit:
        failures.append(('first-fifth', 'no way to start in the first fifth'))
    # Examples: a fenced block must say something, and the how part needs one.
    for kind, pos, content in blocks:
        if kind == 'fence' and not content.strip():
            failures.append(('empty-example', 'an empty fenced block at character %d' % pos))
    for m in re.finditer(r'<details[^>]*>(.*?)</details>', body, re.S):
        inner = re.sub(r'<summary>.*?</summary>', '', m.group(1), flags=re.S).strip()
        if not inner:
            failures.append(('empty-example', 'an empty <details> block at character %d' % m.start()))
    if 'how' in starts:
        after = [s for s in (starts[p] for p in present if starts[p] > starts['how'])]
        end = min(after) if after else len(body)
        section = body[starts['how']:end]
        if '```' not in section and '~~~' not in section and not re.search(r'`[^`\n]+`', section):
            failures.append(('no-example', 'the how part has no example'))
    # The lead of the why part must not open with an internal term.
    if 'why' in starts:
        rest = [starts[p] for p in present if starts[p] > starts['why']]
        why = body[starts['why']:min(rest) if rest else len(body)]
        lines = why.splitlines()
        # A bold label carries its text on the same line; a heading's text is what follows it.
        labelled = re.match(r'^\s*(?:[-*]\s+)?(?:\d+\.\s+)?\*\*[^*]+\*\*:?\s*(.*)$', lines[0])
        first = re.sub(r'\s+', ' ', ' '.join(([labelled.group(1)] if labelled else []) + lines[1:]))
        lead = re.split(r'(?<=[.!?])\s', first.strip(), maxsplit=1)[0].lower()
        for term in JARGON:
            if re.search(r'\b' + re.escape(term) + r'\b', lead):
                failures.append(('jargon-lead', 'the why opens with "' + term + '"'))
                break
    # Walls: a prose paragraph (not a list, table, heading or code) of more than WALL characters.
    para = []
    for kind, pos, line in blocks:
        plain = kind == 'line' and line.strip() and not re.match(r'^\s*([-*+|#>]|\d+\.|<)', line)
        if plain:
            para.append(line.strip())
            continue
        if sum(len(p) + 1 for p in para) > WALL:
            failures.append(('wall', 'a paragraph of %d characters; use bullets or a table' % sum(len(p) + 1 for p in para)))
        para = []
    if sum(len(p) + 1 for p in para) > WALL:
        failures.append(('wall', 'a paragraph of %d characters; use bullets or a table' % sum(len(p) + 1 for p in para)))
    return failures


def pages():
    """The pages the rule applies to under docs/ today; the rest of the scope joins as its batch is planned."""
    return sorted(p for p in (ROOT / 'docs').rglob('*.md') if str(p.relative_to(ROOT)) not in EXEMPT)


def report(paths=None):
    rows = []
    for path in paths or pages():
        rows.append((path, check(path.read_text())))
    return rows


def print_report(paths=None):
    rows = report(paths)
    failing = [(p, f) for p, f in rows if f]
    counts = {k: 0 for k in KINDS}
    for _, fails in failing:
        for k in {k for k, _ in fails}:
            counts[k] += 1
    for path, fails in failing:
        print(str(path.relative_to(ROOT)) + ': ' + '; '.join(sorted({k for k, _ in fails})))
    print('%d pages, %d fail, %d pass' % (len(rows), len(failing), len(rows) - len(failing)))
    print('pages with each failure: ' + ', '.join('%s %d' % (k, counts[k]) for k in KINDS))
    return len(rows), len(failing)


GOOD = """# Thing

📌 **TL;DR:** what it is.

## Why

You get messages from many places and lose them; this keeps them in one place for you.

## What you get

- One inbox for every source.
- A reply goes back where it came from.

## How to use it

```
agent-bus send '#me' 'hello'
```

<details>
<summary>More examples</summary>

```
agent-bus consume --wait 5s
```

</details>

## Technical details

- Messages are held until read.
"""


def mutate(text, old, new):
    assert text.count(old) == 1, old
    return text.replace(old, new)


class Checker(unittest.TestCase):
    def kinds(self, text):
        return {k for k, _ in check(text)}

    def test_a_page_in_order_passes(self):
        self.assertEqual(check(GOOD), [])

    def test_details_first_fails(self):
        technical = '## Technical details\n\n- Messages are held until read.\n\n'
        page = mutate(GOOD, '\n## Technical details\n\n- Messages are held until read.\n', '')
        page = mutate(page, '## Why', technical + '## Why')
        self.assertIn('order', self.kinds(page))

    def test_details_in_the_middle_fails(self):
        technical = '## Technical details\n\n- Messages are held until read.\n\n'
        page = mutate(GOOD, '\n## Technical details\n\n- Messages are held until read.\n', '')
        page = mutate(page, '## How to use it', technical + '## How to use it')
        self.assertIn('order', self.kinds(page))

    def test_no_why_fails(self):
        page = mutate(GOOD, '## Why\n\nYou get messages from many places and lose them; this keeps them in one place for you.\n\n', '')
        self.assertIn('first-fifth', self.kinds(page))
        self.assertIn('missing-part', self.kinds(page))

    def test_why_after_the_first_fifth_fails(self):
        filler = '\n'.join('- A line of introduction number %d that says nothing yet.' % i for i in range(40)) + '\n\n'
        page = mutate(GOOD, '## Why', filler + '## Why')
        self.assertIn('why starts after the first fifth of the page', [m for _, m in check(page)])

    def test_what_you_get_after_the_first_fifth_fails(self):
        # Why stays first; the outcome and the way to start are pushed past the fifth by the why part.
        filler = '\n'.join('- Another reason number %d that goes on for a while without saying what you get.' % i for i in range(30))
        page = mutate(GOOD, '## What you get', filler + '\n\n## What you get')
        self.assertIn('get starts after the first fifth of the page', [m for _, m in check(page)])

    def test_no_way_to_start_in_the_first_fifth_fails(self):
        filler = '\n'.join('- One more thing you get, number %d, listed before any way to start.' % i for i in range(30)) + '\n\n'
        page = mutate(GOOD, '## How to use it', filler + '## How to use it')
        self.assertIn('no way to start in the first fifth', [m for _, m in check(page)])

    def test_how_before_what_you_get_fails(self):
        page = mutate(GOOD, '## What you get\n\n- One inbox for every source.\n- A reply goes back where it came from.\n\n', '')
        page = mutate(page, '## Technical details', '## What you get\n\n- One inbox for every source.\n\n## Technical details')
        self.assertIn('order', self.kinds(page))

    def test_how_before_why_fails(self):
        page = mutate(GOOD, '## Why', '## How to use it\n\n`agent-bus ls`\n\n## Why', )
        self.assertIn('order', self.kinds(page))

    def test_an_empty_example_block_fails(self):
        page = mutate(GOOD, "```\nagent-bus send '#me' 'hello'\n```", '```\n\n```')
        self.assertIn('empty-example', self.kinds(page))

    def test_an_empty_details_block_fails(self):
        page = mutate(GOOD, '```\nagent-bus consume --wait 5s\n```\n\n', '')
        self.assertIn('empty-example', self.kinds(page))

    def test_a_how_part_without_an_example_fails(self):
        page = mutate(GOOD, "```\nagent-bus send '#me' 'hello'\n```", 'Send a message.')
        page = mutate(page, "<details>\n<summary>More examples</summary>\n\n```\nagent-bus consume --wait 5s\n```\n\n</details>\n\n", '')
        self.assertIn('no-example', self.kinds(page))

    def test_a_jargon_lead_fails(self):
        page = mutate(GOOD, 'You get messages from many places and lose them; this keeps them in one place for you.',
                      'The broker routes messages between agents; this keeps them in one place for you.')
        self.assertIn('jargon-lead', self.kinds(page))

    def test_jargon_after_a_plain_lead_passes(self):
        page = mutate(GOOD, 'You get messages from many places and lose them; this keeps them in one place for you.',
                      'You lose messages from many places; this keeps them in one place for you. The broker does the keeping.')
        self.assertNotIn('jargon-lead', self.kinds(page))

    def test_a_bold_label_why_with_a_jargon_lead_fails(self):
        page = mutate(GOOD, '## Why\n\nYou get messages from many places and lose them; this keeps them in one place for you.',
                      '- **Why:** the daemon keeps messages in one place for you.\n')
        self.assertIn('jargon-lead', self.kinds(page))

    def test_a_wall_of_text_fails(self):
        wall = ' '.join(['This is a long run of plain words that goes on and on without a single bullet in sight.'] * 6)
        page = mutate(GOOD, '- One inbox for every source.', wall)
        self.assertIn('wall', self.kinds(page))

    def test_labels_read_like_the_agent_pages(self):
        for word, part in (('Why you would want it', None), ("Why you'd want it", 'why'), ('What it does', 'get'),
                           ('Try this first', 'how'), ('How to use it', 'how'), ('Technical details', 'technical'),
                           ('**Why:** x', 'why'), ('How it is stored', None), ('Details', 'technical')):
            line = '## ' + word if not word.startswith('**') else word
            self.assertEqual(label_of(line), part, word)

    def test_a_fence_hides_a_heading(self):
        page = GOOD + '\n```\n## Why\n```\n'
        self.assertEqual(check(page), [])


class Pages(unittest.TestCase):
    def test_the_scope_is_found(self):
        found = pages()
        self.assertGreater(len(found), 40)
        names = {str(p.relative_to(ROOT)) for p in found}
        self.assertFalse(names & set(EXEMPT))
        self.assertIn('docs/07-setup.md', names)

    def test_the_report_counts_without_failing(self):
        rows = report()
        self.assertGreater(len(rows), 40)
        self.assertTrue(all(isinstance(fails, list) for _, fails in rows))

    def test_enforced_pages_pass(self):
        for name in ENFORCED:
            self.assertEqual(check((ROOT / name).read_text()), [], name)


if __name__ == '__main__':
    if '--report' in sys.argv:
        names = [a for a in sys.argv[1:] if a != '--report']
        print_report([Path(n).resolve() for n in names] or None)
    else:
        unittest.main()
