#!/usr/bin/env python3
"""Checks that a worked guide page says exactly what the shard ran.

  pages.py quotes PAGE              every quoted block equals its source region, byte for byte
  pages.py run PAGE KIND            run the page's command blocks of that kind on the private test bus
                                    and compare each output block with what the commands printed

A page marks what it quotes with an HTML comment right before the fenced block:

  <!-- quote: <repo path> <region> -->     the block is the lines between "# region: NAME" and "# endregion"
  <!-- run: <kind> [ready=<name>] -->      the sh block is run as one bash script; with ready=, the next
                                           block waits until that agent has a reader on its inbox
  <!-- output -->                          the text block after a run block is its exact stdout

Prints "  ok   label" and "  FAIL label: why" lines for guide.sh to count. `run` needs the environment
guide.sh holds: GB_BIN, GB_ADDR, GB_TOKEN, GB_OWNER, PAGE_WORK (a copy of the packages, cwd of the
scripts) and PAGE_ENV (extra NAME=value words the page's commands use, e.g. RUNNER, AGENTS).
"""
import os
import re
import shlex
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
FENCE = re.compile(r"^```(\w*)\s*$")
MARK = re.compile(r"^<!--\s*(quote|run|output)\s*:?\s*(.*?)\s*-->\s*$")


def say(ok, label, why=""):
    print(("  ok   " if ok else "  FAIL ") + label + ("" if ok else f": {why}" if why else ""))
    return ok


def blocks(page):
    """(marker, argument, body) for every fenced block that follows a marker comment."""
    lines = (ROOT / page).read_text().split("\n")
    out, i = [], 0
    while i < len(lines):
        m = MARK.match(lines[i])
        if m and i + 1 < len(lines) and FENCE.match(lines[i + 1]):
            j = i + 2
            while j < len(lines) and not lines[j].startswith("```"):
                j += 1
            out.append((m.group(1), m.group(2), "\n".join(lines[i + 2:j])))
            i = j
        i += 1
    return out


def region(path, name):
    text = (ROOT / path).read_text().split("\n")
    try:
        start = text.index(f"# region: {name}")
        end = text.index("# endregion", start)
    except ValueError:
        return None
    return "\n".join(text[start + 1:end])


def quotes(page, least=1):
    seen = 0
    for kind, arg, body in blocks(page):
        if kind != "quote":
            continue
        path, name = arg.split()
        seen += 1
        want = region(path, name)
        label = f"{page}: the quoted {name} region matches {path}"
        if want is None:
            say(False, label, "no such region")
        else:
            say(body == want, label, "the page and the source differ")
    say(seen >= least, f"{page}: the page quotes at least {least} region(s)")


def run(page, kind):
    work = Path(os.environ["PAGE_WORK"])
    shim = Path(tempfile.mkdtemp(prefix="shim.", dir=work))
    (shim / "agent-bus").write_text(
        "#!/bin/sh\nexec env AGENT_BUS_ADDR=\"$GB_ADDR\" AGENT_BUS_TOKEN=\"$GB_TOKEN\" AGENT_BUS_NAME=\"$GB_OWNER\" "
        "\"$GB_BIN/agent-bus\" \"$@\"\n")
    (shim / "agent-bus").chmod(0o755)
    env = dict(os.environ, PATH=f"{shim}:{os.environ['PATH']}")
    for word in shlex.split(os.environ.get("PAGE_ENV", "")):
        k, _, v = word.partition("=")
        env[k] = v
    groups = []
    n = 0
    try:
        items = blocks(page)
        for idx, (marker, arg, body) in enumerate(items):
            if marker != "run" or arg.split()[0] != kind:
                continue
            n += 1
            ready = next((a[6:] for a in arg.split()[1:] if a.startswith("ready=")), None)
            out = work / f"out.{n}"
            sink, errs = open(out, "w"), open(f"{out}.err", "w")      # the page shows stdout, the answers
            p = subprocess.Popen(["bash", "-e", "-o", "pipefail", "-c", body], cwd=work, env=env, stdout=sink,
                                 stderr=errs, start_new_session=True)
            groups.append(p)
            if ready:   # a block that keeps running: the page says "leave it running"
                for _ in range(150):
                    r = subprocess.run(["agent-bus", "ls", ready], env=env, capture_output=True, text=True)
                    if '"readers":1' in r.stdout:
                        break
                    time.sleep(0.1)
                say(p.poll() is None and '"readers":1' in r.stdout, f"{page}: command block {n} ({kind}) keeps running and {ready} reads its inbox",
                    out.read_text()[-300:] + Path(f"{out}.err").read_text()[-300:])
                continue
            try:
                code = p.wait(timeout=60)
            except subprocess.TimeoutExpired:
                code = "timeout"
            say(code == 0, f"{page}: command block {n} ({kind}) ran", f"exit {code}: {out.read_text()[-200:]} {Path(f'{out}.err').read_text()[-300:]}")
            nxt = items[idx + 1] if idx + 1 < len(items) else None
            if nxt and nxt[0] == "output":
                say(out.read_text().rstrip("\n") == nxt[2], f"{page}: output block {n} is what block {n} printed",
                    f"printed [{out.read_text().rstrip()}]")
    finally:
        for p in groups:
            try:
                os.killpg(p.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        for p in groups:   # let each stop, so its name can be freed
            try:
                p.wait(timeout=15)
            except subprocess.TimeoutExpired:
                os.killpg(p.pid, signal.SIGKILL)


    say(n > 0, f"{page}: the page has {kind} command blocks")


if __name__ == "__main__":
    if sys.argv[1] == "quotes":
        quotes(sys.argv[2], int(sys.argv[3]) if len(sys.argv) > 3 else 1)
    elif sys.argv[1] == "run":
        run(sys.argv[2], sys.argv[3])
    else:
        sys.exit("usage: pages.py quotes PAGE | run PAGE KIND")
