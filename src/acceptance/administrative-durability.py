#!/usr/bin/env python3
"""H.5.3: kill disposable bus children after acknowledged restrictions (0.7 SQLite).
Usage: administrative-durability.py BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR
Uses a real generated systemd unit, existing service accounts and isolated
state/sockets. Requires sudo; never opens or changes the live daemon's state.
"""
import http.client
import json
import os
from pathlib import Path
import socket
import subprocess as sp
import sys
import time

sys.dont_write_bytecode = True
from disposable import Fixture, root, run  # noqa: E402

binary, out = (Path(p).resolve() for p in sys.argv[1:])
out.mkdir(mode=0o700)  # an existing evidence directory must not be overwritten
name = f"agent-bus-h53-{os.getpid()}"
fx = Fixture(name)
state, runtime = fx.state, fx.runtime
owner_name = "owner@durability"
# The runner account's principal is its own account name (setup maps it so),
# which makes it the mapped-socket victim.
victim_name = "agent-bus-runner"
failures = []

def check(value, label):
    print(("ok " if value else "FAIL ") + label, flush=True)
    if not value:
        failures.append(label)

class UnixHTTP(http.client.HTTPConnection):
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.settimeout(4)
        self.sock.connect(str(runtime / "bus.sock"))

def api(path, token="", body=None):
    c = UnixHTTP("bus")
    c.request("POST" if body is not None else "GET", path,
              json.dumps(body) if body is not None else None,
              {"X-Agent-Bus-Token": token, "Content-Type": "application/json"})
    r = c.getresponse()
    status, data = r.status, r.read()
    c.close()
    return status, json.loads(data)

def mapped(path, body=None):
    args = ["runuser", "-u", "agent-bus-runner", "--", "curl", "--silent", "--show-error",
            "--max-time", "4", "--unix-socket", runtime / "user-agent-bus-runner.sock",
            "--output", "/dev/null", "--write-out", "%{http_code}"]
    if body is not None:
        args += ["--header", "Content-Type: application/json", "--data-binary", "@-"]
    return int(root(*args, "http://bus" + path, input=json.dumps(body) if body is not None else None))

def wait_for(fn):
    for _ in range(100):
        try:
            value = fn()
            if value:
                return value
        except (OSError, ValueError, http.client.HTTPException, sp.CalledProcessError):
            pass
        time.sleep(.1)
    raise RuntimeError("disposable daemon did not become ready")

def child():
    return run("pgrep", "-P", supervisor, "-x", "agent-busd").strip()

def crash(label):
    before = child()
    root("kill", "-KILL", before)
    after = wait_for(lambda: child() if child() != before else None)
    wait_for(lambda: api("/identity")[0] == 200)
    check(after != before, label + " uses a replacement bus process")

# No periodic flush: every write the gate sees on disk is an administrative one.
unit = fx.unit(binary / "agent-bus-setup", binary / "agent-busd", owner_name, flags="-flush-every 0")
unit = "\n".join(l.replace(" -web", "") if l.startswith("ExecStart=") else l
                 for l in unit.splitlines()) + "\n"
try:
    # A bounded filesystem of its own, so the storage failure below is a real
    # ENOSPC from the kernel and the live disk is never the one filled.
    fx.prepare(binary / "agent-busd", owner_name, mount="size=16m,mode=0700")
    fx.install(unit, out)
    root("systemctl", "start", name)
    wait_for(lambda: api("/identity")[0] == 200)
    supervisor = root("systemctl", "show", name, "-p", "MainPID", "--value").strip()
    owner = fx.owner_token(owner_name)
    creds = {}
    for who in (victim_name, "friend@durability"):
        status, _ = api("/user", owner, {"name": who, "create": True})
        check(status == 200, "create " + who)
        status, data = api("/token", owner, {"name": who})
        check(status == 200, "issue " + who)
        creds[who] = data["token"]
    victim, friend = creds[victim_name], creds["friend@durability"]
    for kind in ("deactivation", "group-removal", "acl-tightening"):
        check(api("/user/state", owner, {"name": victim_name, "status": "active"})[0] == 200,
              kind + " active-user setup")
        check(api("/group", owner, {"name": "@readers", "members": list(creds)})[0] == 200,
              kind + " group setup")
        target = kind + "@durability"
        check(api("/register", owner, {"name": target, "kind": "queue", "allow": ["@readers"]})[0] == 200,
              kind + " service setup")
        body = {"to": target, "body": "disposable access probe"}
        status, data = api("/session", victim, {})
        check(status == 200, kind + " existing session setup")
        session = data["session"]
        check(api("/send", victim, body)[0] == 200, kind + " old token positive control")
        check(api("/send", session, body)[0] == 200, kind + " existing session positive control")
        check(mapped("/send", body) == 200, kind + " mapped socket positive control")
        if kind == "deactivation":
            request = ("/user/state", {"name": victim_name, "status": "inactive"})
        elif kind == "group-removal":
            request = ("/group", {"name": "@readers", "members": ["friend@durability"]})
        else:
            request = ("/manage", {"name": target, "allow": ["friend@durability"]})
        check(api(request[0], owner, request[1])[0] == 200, kind + " acknowledged")
        check(api("/send", session, body)[0] == 403, kind + " existing session loses access immediately")
        crash(kind)
        check(api("/send", victim, body)[0] == 403, kind + " old token stays restricted after crash")
        check(mapped("/send", body) == 403, kind + " mapped socket stays restricted after crash")
        check(api("/send", session, body)[0] == 401, kind + " old session is invalid after restart")
        check(api("/send", friend, body)[0] == 200, kind + " unrelated user still works after crash")

    # A real storage error, not a mock writer: the database lives on its own
    # small tmpfs, and filling it makes SQLite's next commit fail with ENOSPC.
    # The daemon holds the database exclusively, so the failure is injected
    # beneath it rather than by editing the file. The filler is removed for
    # the deliberate retry, with the same daemon still running.
    check(api("/user/state", owner, {"name": victim_name, "status": "active"})[0] == 200,
          "write-failure active-user setup")
    check(api("/status", victim)[0] == 200, "write-failure victim positive control")
    sp.run(["sudo", "-n", "dd", "if=/dev/zero", f"of={state}/filler", "bs=64k"],
           stdout=sp.DEVNULL, stderr=sp.DEVNULL)
    free = root("df", "--output=avail", state)
    (out / "filled.txt").write_text(free)
    change = {"name": victim_name, "status": "inactive"}
    status, refusal = api("/user/state", owner, change)
    (out / "refusal.json").write_text(json.dumps(refusal) + "\n")
    # Attributed to the storage, not merely any 500.
    check(status == 500 and "full" in json.dumps(refusal), "disk failure cannot acknowledge a deactivation")
    check(api("/status", victim)[0] == 200, "failed write publishes nothing: memory is rolled back")
    root("rm", state / "filler")
    check(api("/user/state", owner, change)[0] == 200, "explicit retry succeeds after disk recovery")
    crash("write-failure retry")
    check(api("/status", victim)[0] == 403, "retried deactivation survives crash")
    check(mapped("/status") == 403, "retried deactivation holds on mapped socket")
    check(api("/status", friend)[0] == 200, "disk recovery preserves unrelated user")
finally:
    for label, fetch in (("journal.log", fx.journal), ("error.log", lambda: root("cat", fx.logs / "error.log"))):
        try:
            (out / label).write_text(fetch())
        except sp.CalledProcessError:
            pass
    fx.remove()
if failures:
    raise SystemExit(1)
print("PASS acknowledged restrictions survive real bus-child crashes", flush=True)
