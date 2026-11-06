#!/usr/bin/env python3
"""Flaky-check quarantine (J120, TD7): the gate's rerun of a red shard is never silent.

The quarantine lives in src/smoke-map.json as "quarantine": {shard: {name: "J<n>"}}.
A name is a Go test (TestX), skipped through GOFLAGS -skip, or the literal title of a
shell assertion (has, lacks, ok_exit, bad_exit, is_empty, delta), which then prints
QUARANTINED and counts as skipped. Fast and normal runs honour it; slow ignores it.

  list               shard<TAB>name<TAB>J, one per entry, for the smoke driver
  check              every entry names a real shard, a name the tree still holds and an
                     open J-item in Plans/issues/<month>/; prints each problem, exit 1
  apply LOG BRANCH   after a red-then-green gate: read the first run's FAIL lines in LOG,
                     file one J-item per failing test, quarantine those that may be, and
                     print the receipt lines
"""
import json
from pathlib import Path
import re
import sys
import datetime
import importlib.util

ROOT = Path(__file__).resolve().parent.parent
HELPERS = ('has', 'lacks', 'ok_exit', 'bad_exit', 'is_empty', 'delta')
SMOKES = ('src/smoke.sh', 'src/agents/smoke.sh')
GO_TEST = re.compile(r'^Test[A-Za-z0-9_]*$')
# The FAIL line each assertion helper prints, title first.
TITLE = re.compile(r'^(?P<t>.+?): (?:exit -?\d+|\[.*\] (?:lacks|still has) \[.*|expected failure, got exit 0|got \[.*|expected \+.*)$')


def selector(root):
    spec = importlib.util.spec_from_file_location('smoke_select', root/'src/smoke-select.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load(root):
    return json.loads((root/'src/smoke-map.json').read_text())


def entries(root):
    return [(shard, name, j) for shard, names in load(root).get('quarantine', {}).items() for name, j in names.items()]


def risky_letters(root):
    """The ⚠️ subsystems of Plans/subsystems.md."""
    return {m[1] for m in re.finditer(r'^\| ([A-Q]) \|[^|]+\| ⚠️', (root/'Plans/subsystems.md').read_text(), re.M)}


def shard_tags(root, shard):
    checks = load(root)['checks']
    if shard == 'core':
        return ''.join(row['tags'] for name, row in checks.items() if name.startswith('core_'))
    return checks.get(shard, {}).get('tags')


def go_modules(root):
    """Import path prefix -> tree directory, longest first."""
    mods = []
    for mod in sorted(root.glob('src/**/go.mod')):
        if 'node_modules' in mod.parts:
            continue
        m = re.search(r'^module\s+(\S+)', mod.read_text(), re.M)
        if m:
            mods.append((m[1], str(mod.parent.relative_to(root))))
    return sorted(mods, key=lambda x: -len(x[0]))


def package_letter(root, package):
    for prefix, directory in go_modules(root):
        if package == prefix or package.startswith(prefix+'/'):
            path = directory+package[len(prefix):]+'/'
            return next((letter for p, letter in selector(root).prefixes(root) if path.startswith(p)), None)
    return None


def title_count(root, title):
    """How many assertions carry this exact title, across both smoke scripts."""
    pattern = re.compile(r'\b(?:%s)\s+"%s"' % ('|'.join(HELPERS), re.escape(title)))
    return sum(len(pattern.findall((root/f).read_text())) for f in SMOKES)


def title_in_tree(root, title):
    return title_count(root, title) > 0


def go_packages(root, name):
    """The directories whose tests define this Go test function: -skip matches it in each of them."""
    pattern = re.compile(r'^func %s\(' % re.escape(name), re.M)
    return sorted({str(p.parent.relative_to(root)) for p in root.glob('src/**/*_test.go')
                   if 'node_modules' not in p.parts and pattern.search(p.read_text(errors='replace'))})


def go_test_in_tree(root, name):
    return bool(go_packages(root, name))


def not_unique(root, name):
    """Why a skip by this name would also skip another test, or None."""
    if GO_TEST.match(name):
        found = go_packages(root, name)
        return f'name not unique: {len(found)} packages define {name}' if len(found) > 1 else None
    n = title_count(root, name)
    return f'name not unique: {n} assertions carry this title' if n > 1 else None


def first_run(log_text):
    """The smoke output of the first normal run: everything before the rerun marker."""
    return log_text.split('\n== rerun after', 1)[0]


def failures(root, text):
    """[{shard, name, kind, risky, why}] for every FAIL in one smoke run's output."""
    risky = risky_letters(root)
    found = {}
    pending = {}
    for line in text.splitlines():
        m = re.match(r'^\[([a-z0-9_]+)\]( +)(.*)$', line)
        if not m:
            continue
        shard, pad, rest = m[1], m[2], m[3]
        shard_found = found.setdefault(shard, {'go': [], 'titles': [], 'unnamed': [], 'orphan': []})
        g = re.match(r'^--- FAIL: (Test[A-Za-z0-9_]*)', rest)
        if g:
            if g[1] not in pending.setdefault(shard, []):
                pending[shard].append(g[1])
            continue
        p = re.match(r'^FAIL\t(\S+)', rest)
        if p:
            names = pending.pop(shard, [])
            for name in names:
                shard_found['go'].append((name, p[1]))
            # A package that failed with no test named (a build failure, a panic or
            # timeout) is red that no Go name covers.
            if not names:
                shard_found['orphan'].append(rest.replace('\t', ' '))
            continue
        if re.match(r'^panic: ', rest):
            shard_found['orphan'].append(rest)
            continue
        # A check's own FAIL line: never a unittest "FAIL:" or "FAILED (" line, never go's indented output.
        if len(pad) <= 3 and rest.startswith('FAIL '):
            t = TITLE.match(rest[5:])
            if t and title_in_tree(root, t['t']):
                shard_found['titles'].append(t['t'])
            else:
                shard_found['unnamed'].append(rest[5:])
    for shard, names in pending.items():  # a test whose package line was cut off
        found.setdefault(shard, {'go': [], 'titles': [], 'unnamed': [], 'orphan': []})['go'].extend((n, None) for n in names)
    items = []
    for shard, f in found.items():
        tags = shard_tags(root, shard) or ''
        if f['go']:
            # Go names are finer than the shell title that wraps the go run.
            seen = set()
            for name, package in f['go']:
                if name in seen:
                    continue
                seen.add(name)
                letter = package_letter(root, package) if package else None
                twin = not_unique(root, name)
                if twin:
                    items.append(dict(shard=shard, name=name, kind='go', risky=True, why=twin))
                elif letter is None:
                    items.append(dict(shard=shard, name=name, kind='go', risky=True, why='its package names no subsystem'))
                else:
                    items.append(dict(shard=shard, name=name, kind='go', risky=letter in risky, why='subsystem '+letter))
        else:
            hit = sorted(set(tags) & risky)
            for title in dict.fromkeys(f['titles']):
                twin = not_unique(root, title)
                items.append(dict(shard=shard, name=title, kind='title', risky=bool(hit) or not tags or bool(twin),
                                  why=twin or ('shard tags '+(''.join(hit) if hit else tags) if tags else 'the shard has no tags')))
        # FAIL lines that name nothing are often a tool's own output under a named
        # check, so they count only in a shard with no name at all. A Go package that
        # failed with no test named is always red of its own.
        orphan = f['orphan'] or ([] if f['go'] or f['titles'] else f['unnamed'])
        if orphan:
            items.append(dict(shard=shard, name=None, kind='unnamed', risky=False, why=orphan[0][:200]))
    return items


def next_j(root):
    numbers = [int(n) for p in (root/'Plans').rglob('*.md') for n in re.findall(r'\bJ(\d+)\b', p.read_text(errors='replace'))]
    return max(numbers, default=0)+1


def open_items(root):
    """J number -> glyph, for every item under Plans/issues/<month>/ (done/ is closed)."""
    status = {}
    for p in (root/'Plans/issues').glob('[0-9][0-9][0-9][0-9]-[0-9][0-9]/*.md'):
        for m in re.finditer(r'^- (\S+) J(\d+)\b', p.read_text(), re.M):
            status[int(m[2])] = m[1]
    return status


def check(root):
    problems = []
    config = load(root)
    shards = {'core'} | set(config['checks'])
    status = open_items(root)
    for shard, name, j in entries(root):
        where = f'quarantine {shard} / {name}'
        if shard not in shards:
            problems.append(f'{where}: no shard {shard} in smoke-map.json checks')
        if GO_TEST.match(name):
            if not go_test_in_tree(root, name):
                problems.append(f'{where}: no Go test {name} in the tree')
        elif not title_in_tree(root, name):
            problems.append(f'{where}: no assertion titled exactly so in {" or ".join(SMOKES)}')
        twin = not_unique(root, name)
        if twin:
            problems.append(f'{where}: {twin}, so the skip would reach another test')
        m = re.fullmatch(r'J(\d+)', j)
        if not m:
            problems.append(f'{where}: "{j}" is not a J id')
        elif int(m[1]) not in status:
            problems.append(f'{where}: {j} is not an item under Plans/issues/<month>/ (closed or missing): remove the entry with its fix')
        elif status[int(m[1])] == '✅':
            problems.append(f'{where}: {j} is ✅: remove the entry in the fix that closed it')
    return problems


def write_map(root, config):
    (root/'src/smoke-map.json').write_text(json.dumps(config, indent=2, ensure_ascii=False)+'\n')


def apply(root, log, branch, today=None):
    """Quarantine and file; returns (receipt lines, changed paths)."""
    text = first_run(Path(log).read_text(errors='replace'))
    items = failures(root, text)
    if not items:
        items = [dict(shard='unknown', name=None, kind='unnamed', risky=False, why='no shard FAIL line in the first run')]
    today = today or datetime.date.today()
    issues = root/'Plans/issues'/today.strftime('%Y-%m')/(today.isoformat()+'.md')
    config = load(root)
    j = next_j(root)
    entries_text, lines, shards = [], [], []
    for item in items:
        jid = f'J{j}'
        j += 1
        label = item['shard']+(' / '+item['name'] if item['name'] else '')
        shards.append(item['shard'])
        if item['kind'] == 'unnamed':
            lines.append(f'OPEN {jid}: {label} not quarantined (UNNAMED)')
            state = f'not quarantined: UNNAMED, the FAIL line names no skippable test ({item["why"]}); a person names it in src/smoke-map.json "quarantine"'
        elif item['risky']:
            lines.append(f'OPEN {jid}: {label} not quarantined (⚠️ NEEDS LEAD)')
            state = f'not quarantined: ⚠️ NEEDS LEAD ({item["why"]}), an intermittent failure there may be a real race; the owning lead fixes it or quarantines it by hand with one non-QA reader'
        else:
            config.setdefault('quarantine', {}).setdefault(item['shard'], {})[item['name']] = jid
            state = f'quarantined in src/smoke-map.json ({item["why"]}); its fix removes the entry in the same landing'
        entries_text.append(
            f'- ⚪ J{jid[1:]} `{label}` failed the gate\'s first normal run and passed its rerun.\n'
            f'  - Evidence: landing of `{branch}`; first-run FAIL lines in `{log}`.\n'
            f'  - State: {state}.\n')
    if 'quarantine' in config:
        write_map(root, config)
    issues.parent.mkdir(parents=True, exist_ok=True)
    body = issues.read_text() if issues.exists() else f'# Issues — {today.strftime("%b")} {today.day}, {today.year}\n'
    if '\n## Gate quarantine\n' not in body:
        body = body.rstrip('\n')+'\n\n## Gate quarantine\n\n'
    else:
        body = body.rstrip('\n')+'\n'
    body += ''.join(entries_text)
    issues.write_text(body)
    head = 'RERUN-GREEN '+', '.join(dict.fromkeys(shards))+': '
    quarantined = [i for i in items if i['kind'] != 'unnamed' and not i['risky']]
    head += (f'quarantined {len(quarantined)} with a J-item' if quarantined else 'nothing quarantined')
    return [head]+lines, ['src/smoke-map.json', str(issues.relative_to(root))]


if __name__ == '__main__':
    args = sys.argv[1:]
    if args == ['list']:
        for shard, name, j in entries(ROOT):
            print(f'{shard}\t{name}\t{j}')
    elif args == ['check']:
        problems = check(ROOT)
        for p in problems:
            print(p)
        sys.exit(1 if problems else 0)
    elif len(args) == 3 and args[0] == 'apply':
        lines, paths = apply(ROOT, args[1], args[2])
        print('\n'.join(lines))
        print('PATHS '+' '.join(paths), file=sys.stderr)
    else:
        print(__doc__, file=sys.stderr)
        sys.exit(2)
