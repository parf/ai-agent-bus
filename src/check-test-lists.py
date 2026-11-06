#!/usr/bin/env python3
"""Fail when a test file runs in no gate check (J110).

smoke.sh runs some subsystems' tests by listing files, so a file nobody added to a list is never run
and can stay red for days (src/mcp/pin.test.ts did, since 2026-11-03). Every test file must be
accounted for by a command that RUNS, never by a mention. What runs is smoke.sh's code (comments
stripped) at top level or inside a function some check calls, the fast argvs of smoke-map.json, and
NAMED_IN. A name in a comment, in a function nothing calls, or in the map's "checks" or "reverse" paths
counts for nothing. A test file is accounted for when:
  1. its name appears in that code;
  2. a `unittest discover -s DIR -p PATTERN` there matches it;
  3. a smoke-agent-pool.py directory run of *.test.ts covers it;
  4. a DIRECTORY_RULES entry (a runner that takes a whole directory) still holds;
  5. VIA names a script the gate runs, which itself runs the test; or
  6. an EXEMPT entry says why the gate does not run it and where it does run (a command there).
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
# Files besides smoke.sh and the fast argvs whose commands name test files the gate runs.
NAMED_IN = ("clients/typescript/tests/check.sh",)
# A runner that takes a whole directory: (glob of the test files, file that holds the command, text of it).
DIRECTORY_RULES = (
    ("web/test/*.test.ts", "smoke.sh", '/web" && AGENT_BUS_BIN_DIR="$D" AGENT_BUS_TMP="$D" timeout 180 bun test'),
    ("agents/discord/*.test.ts", "smoke.sh", './agents/discord'),
    ("clients/python/tests/test_*.py", "clients/check.sh", 'discover -s "$here/python/tests"'),
)
# A script the gate runs that itself runs the test: test -> script. Both must be commands, not mentions.
VIA = {
    "acceptance/guard_liveness_test.py": "acceptance/guard.test.sh",
}
# Not run by the gate: (reason, the file where it does run). That file must run it in a command.
EXEMPT = {
    "acceptance/test_fresh_install.py": ("stub-only; runs as a Go test of agent-bus-setup", "cmd/agent-bus-setup/fresh_install_test.go"),
    "acceptance/test_mac2_cleanup.py": ("stub-only; runs as a Go test of agent-bus-setup", "cmd/agent-bus-setup/darwin_test.go"),
    "acceptance/install_md_test.py": ("runs inside the install-md container (podman); slow acceptance", "acceptance/install-md-container.sh"),
    "npm/container_test.py": ("runs only inside the disposable npm systemd fixture (podman)", "npm/container-check.sh"),
}


def read(path):
    with open(os.path.join(ROOT, path), encoding="utf-8", errors="replace") as f:
        return f.read()


def strip_shell_comment(line):
    """The line without its # comment: a # outside quotes at the start of a word."""
    quote, i = None, 0
    while i < len(line):
        c = line[i]
        if quote == "'":
            if c == "'":
                quote = None
        elif c == "\\":
            i += 1
        elif quote == '"':
            if c == '"':
                quote = None
        elif c in "'\"":
            quote = c
        elif c == "#" and (i == 0 or line[i - 1] in " \t;&|("):
            return line[:i]
        i += 1
    return line


def code_of(path):
    """A runner file's commands: comments removed, by the file's own comment syntax."""
    text = read(path)
    if path.endswith(".go"):
        text = re.sub(r"/\*.*?\*/", "", text, flags=re.S)
        return "\n".join(re.sub(r"//.*$", "", line) for line in text.split("\n"))
    return "\n".join(strip_shell_comment(line) for line in text.split("\n"))


def functions_of(lines):
    """smoke.sh split into top-level lines and named functions (name() { ... } closed at column 0)."""
    top, funcs, current, body = [], {}, None, []
    for line in lines:
        if current is None:
            m = re.match(r"^(\w+)\(\)\s*\{(.*)$", line)
            if not m:
                top.append(line)
            elif m.group(2).rstrip().endswith("}"):
                funcs[m.group(1)] = line
            else:
                current, body = m.group(1), [line]
        else:
            body.append(line)
            if line.rstrip() == "}":
                funcs[current], current = "\n".join(body), None
    return top, funcs


def running_code(smoke, check_names):
    """What smoke.sh runs: its top-level lines and the functions a check calls, directly or through
    another function. A function nothing calls is dead text."""
    lines = [strip_shell_comment(line) for line in smoke.split("\n")]
    top, funcs = functions_of(lines)
    run = {name for name in check_names if name in funcs}
    text = "\n".join(top)
    changed = True
    while changed:
        changed = False
        reached = text + "\n" + "\n".join(funcs[f] for f in run)
        for name in funcs:
            if name not in run and re.search(r"(?<![\w-])" + re.escape(name) + r"(?![\w-])", reached):
                run.add(name)
                changed = True
    return text + "\n" + "\n".join(funcs[f] for f in sorted(run))


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


def fast_argvs(smoke_map):
    """Only the fast lists run; "checks" and "reverse" hold paths that select, not commands."""
    for group in smoke_map.get("fast", {}).values():
        for entry in group:
            yield " ".join(entry["argv"])


def named_for(path, code, argvs, ambiguous):
    """Whether a command names this file. A name that two test files share counts only on a line that
    also names the file's directory, so one file's listing never stands for the other's."""
    name, directory = os.path.basename(path), os.path.dirname(path)
    for line in code.split("\n") + argvs:
        if name in line and named_in(name, line) and (name not in ambiguous or directory in line or os.path.basename(directory) in line):
            return True
    stem = os.path.splitext(name)[0]
    joined = " ".join(argvs)
    return stem in joined and named_in(stem, joined) and name not in ambiguous


def runs_in(file, name):
    """Whether a runner file's commands (not its comments) run the test."""
    code = code_of(file)
    if file.endswith(".go"):
        return any("exec.Command" in line and name in line for line in code.split("\n"))
    return named_in(name, code)


def covered(path, code, argvs, ambiguous, smoke_code):
    name, directory = os.path.basename(path), os.path.dirname(path)
    for source in (code, *argvs):
        for d, p in re.findall(r"discover -s (\S+) -p '?([^'\s]+)'?", source):
            if os.path.normpath(d) == directory and fnmatch.fnmatch(name, p):
                return "discover"
    for d in re.findall(r"smoke-agent-pool\.py \./(agents/\w+)", code):
        if directory == d and fnmatch.fnmatch(name, "*.test.ts"):
            return "pool"
    for glob, file, snippet in DIRECTORY_RULES:
        if fnmatch.fnmatch(path, glob) and snippet in (smoke_code if file == "smoke.sh" else code_of(file)):
            return "directory"
    if named_for(path, code, argvs, ambiguous):
        return "named"
    if path in VIA and named_in(os.path.basename(VIA[path]), code) and runs_in(VIA[path], name):
        return "via"
    if path in EXEMPT and runs_in(EXEMPT[path][1], name):
        return "exempt"
    return None


def check(tests=None, smoke=None, smoke_map=None, named_code=None):
    """The problems found, as lines; none means every test file is accounted for."""
    tests = test_files() if tests is None else tests
    smoke = read("smoke.sh") if smoke is None else smoke
    smoke_map = json.loads(read("smoke-map.json")) if smoke_map is None else smoke_map
    argvs = list(fast_argvs(smoke_map))
    smoke_code = running_code(smoke, smoke_map.get("checks", {}).keys())
    code = smoke_code + "\n" + (named_code if named_code is not None else "\n".join(code_of(f) for f in NAMED_IN))
    names = [os.path.basename(t) for t in tests]
    ambiguous = frozenset(n for n in names if names.count(n) > 1)
    problems = [f"{t}: no check runs it; list it in a command smoke.sh runs and in smoke-map.json, or add an EXEMPT entry saying where it runs"
                for t in tests if covered(t, code, argvs, ambiguous, smoke_code) is None]
    for path, (_reason, where) in EXEMPT.items():
        if not os.path.exists(os.path.join(ROOT, path)):
            problems.append(f"EXEMPT {path}: the file is gone; remove the entry")
        elif not runs_in(where, os.path.basename(path)):
            problems.append(f"EXEMPT {path}: {where} no longer runs it in a command")
    for path, script in VIA.items():
        if not named_in(os.path.basename(script), code) or not runs_in(script, os.path.basename(path)):
            problems.append(f"VIA {path}: the gate no longer runs {script}, or it no longer runs the test")
    for glob, file, snippet in DIRECTORY_RULES:
        if snippet not in (smoke_code if file == "smoke.sh" else code_of(file)):
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
