#!/usr/bin/env python3
"""H.5.3: kill disposable bus children after acknowledged restrictions.
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

binary, out = (Path(p).resolve() for p in sys.argv[1:])
out.mkdir(mode=0o700)  # an existing evidence directory must not be overwritten
name = f"agent-bus-h53-{os.getpid()}"
state, runtime = Path("/var/lib") / name, Path("/run") / name
unit_path = Path("/run/systemd/system") / (name + ".service")
failures = []

def run(*args, **kw):
    return sp.check_output(list(map(str, args)), text=True, **kw)

def root(*args, **kw):
    return run("sudo", "-n", *args, **kw)

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

unit = run(binary / "agent-bus-setup", "--print-unit", "--owner", "owner@durability",
           "--addr", "127.0.0.1:0", "--exec", binary / "agent-busd")
unit = unit.replace("/var/lib/agent-bus/daemon", str(state))
unit = unit.replace("StateDirectory=agent-bus/daemon", f"StateDirectory={name}")
unit = unit.replace("/run/agent-bus", str(runtime))
unit = unit.replace("RuntimeDirectory=agent-bus\n", f"RuntimeDirectory={name}\n")
unit = "\n".join(line.replace(" -web", "") + " -dump-every 0" if line.startswith("ExecStart=") else line
                 for line in unit.splitlines()) + "\n"
(out / "unit.service").write_text(unit)
try:
    root("cp", out / "unit.service", unit_path)
    root("systemctl", "daemon-reload")
    root("systemctl", "start", name)
    wait_for(lambda: api("/identity")[0] == 200)
    supervisor = root("systemctl", "show", name, "-p", "MainPID", "--value").strip()
    owner = root("cat", state / "token").split()[1]
    creds = {}
    for who in ("runner@durability", "friend@durability"):
        status, _ = api("/user", owner, {"name": who, "create": True})
        check(status == 200, "create " + who)
        status, data = api("/token", owner, {"name": who})
        check(status == 200, "issue " + who)
        creds[who] = data["token"]
    victim, friend = creds["runner@durability"], creds["friend@durability"]
    for kind in ("ban", "group-removal", "acl-tightening"):
        check(api("/user/state", owner, {"name": "runner@durability", "state": "active"})[0] == 200,
              kind + " active-user setup")
        check(api("/group", owner, {"name": "@readers", "members": list(creds)})[0] == 200,
              kind + " group setup")
        target = kind + "@durability"
        check(api("/register", owner, {"name": target, "allow": ["@readers"], "no_master": True})[0] == 200,
              kind + " service setup")
        body = {"to": target, "body": "disposable access probe"}
        status, data = api("/session", victim, {})
        check(status == 200, kind + " existing session setup")
        session = data["session"]
        check(api("/send", victim, body)[0] == 200, kind + " old token positive control")
        check(api("/send", session, body)[0] == 200, kind + " existing session positive control")
        check(mapped("/send", body) == 200, kind + " mapped socket positive control")
        if kind == "ban":
            request = ("/user/state", {"name": "runner@durability", "state": "banned"})
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

    # A real rename error, not a mock writer: replace the snapshot target with
    # a directory. Retain its earlier file until the deliberate retry.
    check(api("/user/state", owner, {"name": "runner@durability", "state": "active"})[0] == 200,
          "write-failure active-user setup")
    root("mv", state / "dump.json", state / "saved.json")
    root("mkdir", state / "dump.json")
    change = {"name": "runner@durability", "state": "banned"}
    check(api("/user/state", owner, change)[0] == 500, "disk failure cannot acknowledge a ban")
    check(api("/status", victim)[0] == 403, "failed write does not promise memory rollback")
    root("rmdir", state / "dump.json")
    root("mv", state / "saved.json", state / "dump.json")
    check(api("/user/state", owner, change)[0] == 200, "explicit retry succeeds after disk recovery")
    crash("write-failure retry")
    check(api("/status", victim)[0] == 403, "retried ban survives crash")
    check(mapped("/status") == 403, "retried ban holds on mapped socket")
    check(api("/status", friend)[0] == 200, "disk recovery preserves unrelated user")
finally:
    (out / "journal.log").write_text(root("journalctl", "--no-pager", "-u", name))
    root("systemctl", "stop", name)
    root("rm", "-f", unit_path)
    root("systemctl", "daemon-reload")
    root("rm", "-rf", state, runtime)
if failures:
    raise SystemExit(1)
print("PASS acknowledged restrictions survive real bus-child crashes", flush=True)
