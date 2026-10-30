"""Synthetic post-attempt script fixtures; runtime tests follow the core landing.

No live checkout, account, service or external command destination is used.
Fixtures deliberately retain failed-script effects so ordering and process
cleanup checks can assert what actually happened, rather than mock a send.
"""
import copy
import json
import os
from pathlib import Path
import sys


def attempt_record(**changes):
    """The owner-approved result shape with only synthetic, public values."""
    record = {
        'v': 1, 'host': 'host.fixture.test', 'repo': 'fixture', 'branch': 'main',
        'result': 'ok', 'action': 'autopull', 'trigger': 'event', 'mode': 'ff',
        'at': '2026-10-29T12:00:00-04:00', 'agent': '#git@fixture.test',
        'remote': 'fixture/project', 'requested_by': '#pipeline@fixture.test',
        'id': 'pipeline-fixture', 'from': 'a' * 40, 'to': 'b' * 40,
        'requested': 'b' * 40, 'discarded': None, 'script': None,
        'error': None, 'ms': 12,
    }
    record.update(copy.deepcopy(changes))
    return record


class ScriptFixtures:
    """Installed allowlist plus checkout, rooted in a caller-owned temp dir."""
    def __init__(self, base):
        self.base = Path(base)
        self.state = self.base / 'state' / 'agent-bus-git'
        self.allowlist = self.state / 'autopull.d'
        self.checkout = self.base / 'checkout'
        self.journal = self.base / 'script-order.jsonl'
        self.child_effect = self.base / 'late-child-effect'
        self.allowlist.mkdir(parents=True, mode=0o700)
        self.checkout.mkdir(mode=0o700)
        self.journal.touch(mode=0o600)

    def publish(self, record):
        self.append({'kind': 'published', 'record': copy.deepcopy(record)})

    def append(self, value):
        with self.journal.open('a', encoding='utf-8') as stream:
            stream.write(json.dumps(value, ensure_ascii=False) + '\n')

    def events(self):
        return [json.loads(line) for line in self.journal.read_text().splitlines()]

    def install(self, name, *, exit_code=0, stderr=b'', stdout_size=0,
                delay=0, ignore_term=False, child_delay=None, read_stdin=True):
        """Create a real executable, never a shell-expanded command string."""
        assert name and Path(name).name == name and name not in ('.', '..')
        path = self.allowlist / name
        source = '#!' + sys.executable + '\n' + '''import json, os, pathlib, signal, subprocess, sys, time
'''
        if ignore_term:
            source += 'signal.signal(signal.SIGTERM, signal.SIG_IGN)\n'
        source += 'record = json.load(sys.stdin)\n' if read_stdin else 'record = None\n'
        source += 'child = None\n'
        if child_delay is not None:
            child = ('import pathlib,time; time.sleep(' + repr(child_delay) + '); '
                     'pathlib.Path(' + repr(str(self.child_effect)) + ').write_text("late effect")')
            source += 'child = subprocess.Popen([' + repr(sys.executable) + ', "-c", ' + repr(child) + ']).pid\n'
        source += (
            'entry = {"kind":"script", "name":' + repr(name)
            + ', "cwd":os.getcwd(), "uid":os.getuid(), "record":record, "child":child}\n'
            + 'with open(' + repr(str(self.journal)) + ', "a", encoding="utf-8") as stream:\n'
            + '    stream.write(json.dumps(entry, ensure_ascii=False) + "\\n")\n'
        )
        if stdout_size:
            source += ('for offset in range(0, ' + repr(stdout_size) + ', 65536):\n'
                       '    os.write(1, b"x" * min(65536, ' + repr(stdout_size) + ' - offset))\n')
        if stderr:
            source += 'os.write(2, ' + repr(stderr) + ')\n'
        if delay:
            source += 'time.sleep(' + repr(delay) + ')\n'
        source += 'sys.exit(' + repr(exit_code) + ')\n'
        path.write_text(source, encoding='utf-8')
        path.chmod(0o700)
        return path

    def alias(self, name, target):
        path = self.allowlist / name
        path.symlink_to(target)
        return path
