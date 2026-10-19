#!/usr/bin/env python3
"""Test-only systemd stand-in: executes the written unit against a test bus."""
import json
import os
from pathlib import Path
import shlex
import signal
import subprocess
import sys
import time

root = Path(os.environ["STANDALONE_TEST_ROOT"])
verb = sys.argv[1]
unit = sys.argv[-1]
pidfile = root / (unit + ".pid")


def alive():
    if not pidfile.exists():
        return False
    pid = int(pidfile.read_text())
    try:
        return Path(f"/proc/{pid}/stat").read_text().split(")", 1)[1].split()[0] != "Z"
    except FileNotFoundError:
        return False


def stop():
    if alive():
        os.kill(int(pidfile.read_text()), signal.SIGTERM)
        end = time.monotonic() + 40
        while alive() and time.monotonic() < end:
            time.sleep(.02)
        if alive():
            raise RuntimeError("fixture child did not stop")
    pidfile.unlink(missing_ok=True)


if verb == "is-active":
    sys.exit(0 if alive() else 3)
if verb in ("disable", "restart"):
    stop()
if verb in ("start", "restart"):
    fields = dict(line.split("=", 1) for line in (root / "units" / unit).read_text().splitlines() if "=" in line)
    envfile = Path(shlex.split(fields["EnvironmentFile"])[0])
    env = {k: v for k, v in os.environ.items() if not k.startswith("AGENT_BUS_")}
    env.update(dict(line.split("=", 1) for line in envfile.read_text().splitlines()))
    env["XDG_STATE_HOME"] = str(root / "state")
    argv = [a.replace("%%", "%").replace("$$", "$") for a in shlex.split(fields["ExecStart"])]
    with (root / "run.log").open("ab") as log:
        p = subprocess.Popen(argv, env=env, stdout=log, stderr=log, start_new_session=True)
    pidfile.write_text(str(p.pid))
