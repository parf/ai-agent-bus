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
    'why': ('why', "why you'd want it", 'why you want it', 'why it exists', 'problem'),
    'get': ('what you get', 'what it does', 'outcome'),
    'how': ('how to use it', 'try this first', 'use examples', 'how to start', 'one way to start', 'start', 'how'),
    'technical': ('technical details', 'technical', 'how it works', 'internals', 'details'),
}
# Internal terms a lead must not open with (CLAUDE.md: broker, registry, daemon, queue, wire, storage
# detail; no language or library). The lead is the first sentence of the why part.
JARGON = ('broker', 'registry', 'daemon', 'queue', 'wire', 'storage', 'sqlite', 'goroutine', 'protocol',
          'bun', 'typescript', 'python', 'golang', 'mutex', 'kv store', 'ndjson', 'json schema')
FIRST_FIFTH = 0.20
WALL = 300                # characters in one prose paragraph; about three wrapped lines is 240

# Not rewritten: the contract, the index and the vocabulary (docs-rewrite.md "Not rewritten").
EXEMPT = ('docs/constitution.md', 'docs/decisions.md', 'docs/glossary.md')

# Pages that must pass. Each batch of the docs rewrite adds its pages here as it lands.
ENFORCED = ['docs/runner/%s.md' % name for name in (
    'README', 'agents', 'configuration', 'getting-started', 'operations', 'security')] + [
    'docs/user/%s.md' % name for name in ('README', 'ai-cli', 'backup', 'cli', 'daemon', 'dedicated-runner')] + [
    'docs/%s.md' % name for name in (
    '00-overview', '01-identity-and-authority', '02-access', '02-access-remote', '03-records', '03-records-agent',
    '03-records-channel', '03-records-kv', '03-records-resource', '03-records-service', '04-messaging', '05-discovery')] + [
    'docs/agents/%s.md' % name for name in (
    'ai', 'db', 'fetch', 'git', 'git-autopull', 'qdrant', 'rag', 'server-info', 'systemd', 'voyage',
    'README', 'at', 'bus', 'sms', 'smtp', 'tmux-sessions', 'webhook',
    'cron', 'debug', 'eventlog', 'imap', 'logwatch', 'slack', 'telegram')]

ENFORCED += ['docs/%s.md' % name for name in ('06-dedicated-runner', '06-launchers', '08-processes', '09-daemon-api')]

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
    # `starts` is the earliest marker of a part, a heading or a bold label such as the TL;DR's
    # "**Problem:**": it says whether the part exists and how early it is. `placed` is where the part
    # really sits: its heading when it has one, so a TL;DR that names every part cannot hide a page
    # whose headings are out of order.
    starts, heads = {}, {}
    for kind, pos, line in blocks:
        if kind == 'line':
            part = label_of(line)
            if part and part not in starts:
                starts[part] = pos
            if part and part not in heads and re.match(r'^\s{0,3}#{1,6}\s', line):
                heads[part] = pos
    placed = {p: heads.get(p, starts[p]) for p in starts}
    missing = [p for p in PARTS if p not in starts]
    if missing:
        failures.append(('missing-part', 'no ' + ', '.join(missing) + ' part'))
    present = [p for p in PARTS if p in starts]
    ordered = sorted(present, key=lambda p: placed[p])
    if ordered != present:
        failures.append(('order', 'parts out of order: ' + ' < '.join(ordered) + ' (want ' + ' < '.join(present) + ')'))
    limit = FIRST_FIFTH * len(body)
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
    # The lead of the why part must not open with an internal term: the first why marker and, when
    # it is another, the why heading.
    jargon = None
    for why_at in sorted({starts['why'], placed['why']}) if 'why' in starts else []:
        rest = [pos for pos in placed.values() if pos > why_at] + [pos for pos in starts.values() if pos > why_at]
        why = body[why_at:min(rest) if rest else len(body)]
        lines = why.splitlines()
        # A bold label carries its text on the same line; a heading's text is what follows it.
        labelled = re.match(r'^\s*(?:[-*]\s+)?(?:\d+\.\s+)?\*\*[^*]+\*\*:?\s*(.*)$', lines[0])
        first = re.sub(r'\s+', ' ', ' '.join(([labelled.group(1)] if labelled else []) + lines[1:]))
        lead = re.split(r'(?<=[.!?])\s', first.strip(), maxsplit=1)[0].lower()
        for term in JARGON:
            if re.search(r'\b' + re.escape(term) + r'\b', lead):
                jargon = term
                break
        if jargon:
            break
    if jargon:
        failures.append(('jargon-lead', 'the why opens with "' + jargon + '"'))
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

📌 **TL;DR:**
- **Why:** you lose messages from many places; this keeps them in one place for you.
- **What you get:** one inbox for every source, and replies that go back where they came from.
- **How to use it:** send one message and read it back.

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
- A message is kept for at most a day, and at most a thousand wait at once.
- Replies carry the id of the message they answer, so a reply finds its way back.
- Nothing is sent twice: a message that was delivered is not delivered again.
- The inbox is read in the order messages arrived, oldest first.
- A sender that is not allowed to reach a name is refused at once, naming the rule.
"""


# The shape of the owner-approved runner page (docs/runner/README.md): a TL;DR carrying problem,
# outcome and start, then the four parts under their own headings, then a way on.
RUNNER_SHAPE = """# AgentBus runner

**Turn any script into a safe, shared team tool.**

📌 **TL;DR:**
- **Problem:** a handy script stays a private hack, because sharing it means sharing its passwords.
- **Outcome:** the runner gives the script a name, keeps its passwords apart and lets your team call it.
- **Start:** drop the script in a folder, reload, call it by name.

## Why you want it

- 🔑 **A script needs a password.** Now the password stays on the server.
- 🧱 **A script can touch everything.** Now it runs confined.

## What you get

- ✅ **No message code:** the script reads its input and prints its answer.
- ✅ **Secrets kept apart:** passwords live outside the code.

## One way to start

- Follow the getting started page: a short script, a reload, one call.

<details>
<summary>See it on a real node (commands that ran today)</summary>

```sh
agent-bus ls --tag runner --table
```

</details>

## How it works

- The runner reads each package and serves it under its own name.
- Each package is a folder with a short description file and the script.
- A reload reads the folders again and starts, stops or leaves each agent as asked.
- Secrets sit in a private folder of the runner's account, never in the package.
- Each run gets its own work folder, memory limit and process limit.
- Every run is logged with the name of the caller.
- A script that stops is started again, with a pause that grows each time.
- The status call lists each agent and its state.
- Disabled agents stay installed and are not started.
- Instances of one package share its code and keep their own settings.
- Calls to an agent wait for a free worker, up to the limit set for it.

## Choose your starting point

- A table of where to go next.
"""


def plain_tldr(page):
    """The page with a TL;DR that names no part, so only the headings place the parts."""
    return mutate(page, '''📌 **TL;DR:**
- **Why:** you lose messages from many places; this keeps them in one place for you.
- **What you get:** one inbox for every source, and replies that go back where they came from.
- **How to use it:** send one message and read it back.
''', '📌 **TL;DR:** a short summary of the page.\n')


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
        page = mutate(plain_tldr(GOOD), '## Why\n\nYou get messages from many places and lose them; this keeps them in one place for you.\n\n', '')
        self.assertIn('first-fifth', self.kinds(page))
        self.assertIn('missing-part', self.kinds(page))

    def test_why_after_the_first_fifth_fails(self):
        filler = '\n'.join('- A line of introduction number %d that says nothing yet.' % i for i in range(40)) + '\n\n'
        page = mutate(plain_tldr(GOOD), '## Why', filler + '## Why')
        self.assertIn('why starts after the first fifth of the page', [m for _, m in check(page)])

    def test_what_you_get_after_the_first_fifth_fails(self):
        # Why stays first; the outcome and the way to start are pushed past the fifth by the why part.
        filler = '\n'.join('- Another reason number %d that goes on for a while without saying what you get.' % i for i in range(30))
        page = mutate(plain_tldr(GOOD), '## What you get', filler + '\n\n## What you get')
        self.assertIn('get starts after the first fifth of the page', [m for _, m in check(page)])

    def test_no_way_to_start_in_the_first_fifth_fails(self):
        filler = '\n'.join('- One more thing you get, number %d, listed before any way to start.' % i for i in range(30)) + '\n\n'
        page = mutate(plain_tldr(GOOD), '## How to use it', filler + '## How to use it')
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
        page = mutate(plain_tldr(GOOD), '## Why\n\nYou get messages from many places and lose them; this keeps them in one place for you.',
                      '- **Why:** the daemon keeps messages in one place for you.\n')
        self.assertIn('jargon-lead', self.kinds(page))

    def test_a_wall_of_text_fails(self):
        wall = ' '.join(['This is a long run of plain words that goes on and on without a single bullet in sight.'] * 6)
        page = mutate(GOOD, '- One inbox for every source.', wall)
        self.assertIn('wall', self.kinds(page))

    def test_a_short_page_is_held_to_the_same_fifth(self):
        # A 344-character body whose parts start at about 60%, 70% and 81%: a short page has no floor.
        intro = 'An introduction that says nothing about why, what or how, and goes on a little. ' * 2
        page = '# Short\n\n' + intro.strip() + '\n\n## Why\n\nYou lose messages.\n\n## What you get\n\n- One inbox.\n\n## How to use it\n\n`agent-bus ls`\n\n## Technical details\n\n- Held until read.\n'
        body, _ = body_of(page)
        self.assertLess(len(body), 400)
        kinds = {m for _, m in check(page)}
        self.assertIn('why starts after the first fifth of the page', kinds)
        self.assertIn('no way to start in the first fifth', kinds)

    def test_the_fifth_is_twenty_percent_not_thirty(self):
        # Intro filler before the parts, sized so the why starts at a chosen share of the body.
        rest = plain_tldr(GOOD).split('## Why', 1)
        tail = '## Why' + rest[1]
        head = rest[0]

        def share(percent):
            # filler f with (len(head_body)+f) / (len(head_body)+f+len(tail)) == percent
            base = len(body_of(head)[0])
            f = int(percent * len(tail) / (1 - percent)) - base
            self.assertGreater(f, 0)
            page = head + ('x ' * (f // 2)) + '\n\n' + tail
            body, _ = body_of(page)
            at = body.index('## Why') / len(body)
            return page, at

        late, at = share(0.25)
        self.assertGreater(at, 0.22)
        self.assertIn('why starts after the first fifth of the page', [m for _, m in check(late)])
        early, at = share(0.15)
        self.assertLess(at, 0.19)
        self.assertNotIn('why starts after the first fifth of the page', [m for _, m in check(early)])

    def test_labels_read_like_the_agent_pages(self):
        for word, part in (('Why you would want it', None), ("Why you'd want it", 'why'), ('What it does', 'get'),
                           ('Try this first', 'how'), ('How to use it', 'how'), ('Technical details', 'technical'),
                           ('**Why:** x', 'why'), ('How it is stored', None), ('Details', 'technical')):
            line = '## ' + word if not word.startswith('**') else word
            self.assertEqual(label_of(line), part, word)

    def test_the_approved_runner_pages_shape_passes(self):
        self.assertEqual(check(RUNNER_SHAPE), [])

    def test_a_tldr_that_names_every_part_does_not_hide_out_of_order_headings(self):
        # The TL;DR is in order; the headings below put the technical details first.
        start = RUNNER_SHAPE.index('## How it works')
        end = RUNNER_SHAPE.index('## Choose your starting point')
        section = RUNNER_SHAPE[start:end]
        page = RUNNER_SHAPE[:start] + RUNNER_SHAPE[end:]
        page = mutate(page, '## Why you want it', section + '## Why you want it')
        self.assertIn('order', self.kinds(page))

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

    def test_the_runner_guide_is_enforced_and_passes(self):
        # The first page rewritten to the standard must pass; a checker that rejects it is the one that is wrong.
        self.assertIn('docs/runner/README.md', ENFORCED)
        self.assertEqual(check((ROOT / 'docs/runner/README.md').read_text()), [])

    def test_enforced_pages_pass(self):
        for name in ENFORCED:
            self.assertEqual(check((ROOT / name).read_text()), [], name)


if __name__ == '__main__':
    if '--report' in sys.argv:
        names = [a for a in sys.argv[1:] if a != '--report']
        print_report([Path(n).resolve() for n in names] or None)
    else:
        unittest.main()
