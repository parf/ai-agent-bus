#!/usr/bin/env python3
"""Fail when a test file runs in no gate check (J110).

smoke.sh runs some subsystems' tests by listing files, so a file nobody added to a list is never run
and can stay red for days (src/mcp/pin.test.ts did, since 2026-11-03). Every test file must be
accounted for in one of four ways:
  1. its name appears in smoke.sh, smoke-map.json or one of NAMED_IN;
  2. a `unittest discover -s DIR -p PATTERN` in smoke.sh or the map matches it;
  3. a smoke-agent-pool.py directory run of *.test.ts covers it;
  4. a DIRECTORY_RULES entry (a runner that takes a whole directory), which must still hold, or
     an EXEMPT entry that says why the gate does not run it and where it does run.
Run from src/: python3 -B check-test-lists.py
"""
import fnmatch
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.abspath(__file__))
SKIP = {"node_modules", "tmp", "target", ".git", "__pycache__", ".mypy_cache", "dist"}
PATTERNS = ("*.test.ts", "*.test.sh", "*_test.py", "test_*.py")
# Files besides smoke.sh and smoke-map.json whose lists name test files the gate runs.
NAMED_IN = ("clients/typescript/tests/check.sh",)
# A runner that takes a whole directory: (glob of the test files, file that holds the command, text of it).
DIRECTORY_RULES = (
    ("web/test/*.test.ts", "smoke.sh", '/web" && AGENT_BUS_BIN_DIR="$D" AGENT_BUS_TMP="$D" timeout 180 bun test'),
    ("agents/discord/*.test.ts", "smoke.sh", './agents/discord'),
    ("clients/python/tests/test_*.py", "clients/check.sh", 'discover -s "$here/python/tests"'),
)
# Not run by the gate: (reason, the file where it does run). The named file must still mention the test.
EXEMPT = {
    "acceptance/test_fresh_install.py": ("stub-only; runs as a Go test of agent-bus-setup", "cmd/agent-bus-setup/fresh_install_test.go"),
    "acceptance/test_mac2_cleanup.py": ("stub-only; runs as a Go test of agent-bus-setup", "cmd/agent-bus-setup/darwin_test.go"),
    "acceptance/guard_liveness_test.py": ("a podman harness check; runs under acceptance/guard.test.sh", "acceptance/guard.test.sh"),
    "acceptance/install_md_test.py": ("runs inside the install-md container (podman); slow acceptance", "acceptance/install-md-container.sh"),
    "container/checksum_test.py": ("runs first in the container acceptance (podman), which normal skips without the base image", "container/acceptance.sh"),
    "npm/container_test.py": ("runs only inside the disposable npm systemd fixture (podman)", "npm/container-check.sh"),
}


def read(path):
    with open(os.path.join(ROOT, path), encoding="utf-8", errors="replace") as f:
        return f.read()


def test_files():
    found = []
    for base, dirs, files in os.walk(ROOT):
        dirs[:] = [d for d in dirs if d not in SKIP]
        for name in files:
            if any(fnmatch.fnmatch(name, p) for p in PATTERNS):
                found.append(os.path.relpath(os.path.join(base, name), ROOT))
    return sorted(found)


def named_in(name, text):
    return re.search(r"(?<![\w./-])(?:[\w./-]*/)?" + re.escape(name) + r"(?![\w-])", text) is not None


def map_argvs(smoke_map):
    for group in smoke_map.get("fast", {}).values():
        for entry in group:
            yield " ".join(entry["argv"])


def named_for(path, text, argvs, ambiguous):
    """Whether a list names this file. A name that two test files share counts only on a line that also
    names the file's directory, so one file's listing never stands for the other's."""
    name, directory = os.path.basename(path), os.path.dirname(path)
    lines = text.split("\n") + argvs
    for line in lines:
        if name in line and named_in(name, line) and (name not in ambiguous or directory in line or os.path.basename(directory) in line):
            return True
    stem = os.path.splitext(name)[0]
    return stem in " ".join(argvs) and named_in(stem, " ".join(argvs)) and name not in ambiguous


def covered(path, text, smoke, argvs, ambiguous=frozenset()):
    name, directory = os.path.basename(path), os.path.dirname(path)
    if named_for(path, text, argvs, ambiguous):
        return "named"
    for source in (smoke, *argvs):
        for d, p in re.findall(r"discover -s (\S+) -p '?([^'\s]+)'?", source):
            if os.path.normpath(d) == directory and fnmatch.fnmatch(name, p):
                return "discover"
    for d in re.findall(r"smoke-agent-pool\.py \./(agents/\w+)", smoke):
        if directory == d and fnmatch.fnmatch(name, "*.test.ts"):
            return "pool"
    for glob, file, snippet in DIRECTORY_RULES:
        if fnmatch.fnmatch(path, glob) and snippet in read(file):
            return "directory"
    if path in EXEMPT and os.path.basename(path) in read(EXEMPT[path][1]):
        return "exempt"
    return None


def check(tests=None, smoke=None, smoke_map=None, named_text=None):
    """The problems found, as lines; none means every test file is accounted for."""
    tests = test_files() if tests is None else tests
    smoke = read("smoke.sh") if smoke is None else smoke
    smoke_map = json.loads(read("smoke-map.json")) if smoke_map is None else smoke_map
    argvs = list(map_argvs(smoke_map))
    text = smoke + "\n" + json.dumps(smoke_map) + "\n" + (named_text if named_text is not None else "\n".join(read(f) for f in NAMED_IN))
    names = [os.path.basename(t) for t in tests]
    ambiguous = frozenset(n for n in names if names.count(n) > 1)
    problems = [f"{t}: no check runs it; list it in smoke.sh and smoke-map.json, or add an EXEMPT entry saying where it runs"
                for t in tests if covered(t, text, smoke, argvs, ambiguous) is None]
    for path, (_reason, where) in EXEMPT.items():
        if not os.path.exists(os.path.join(ROOT, path)):
            problems.append(f"EXEMPT {path}: the file is gone; remove the entry")
        elif os.path.basename(path) not in read(where):
            problems.append(f"EXEMPT {path}: {where} no longer mentions it")
    for glob, file, snippet in DIRECTORY_RULES:
        if snippet not in read(file):
            problems.append(f"DIRECTORY_RULES {glob}: {file} no longer holds its command")
    return problems


def main():
    problems = check()
    for line in problems:
        print("FAIL", line)
    if problems:
        return 1
    print(f"ok: {len(test_files())} test files, each run by a check or exempt for a stated reason")
    return 0


if __name__ == "__main__":
    sys.exit(main())
