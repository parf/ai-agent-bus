#!/usr/bin/env python3
"""G.1.3: a disposable real systemd unit; never mutate the live installation.

Usage: web-isolation.py BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR
Requires the installed agent-busd/runner accounts, systemd, sudo, Go and bwrap.
The generated policy is retained; only installation paths, ports and the test
web executable change. All token/state/SSH write probes target disposable files.
"""
import hashlib
import http.client
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import subprocess as sp
import sys
import time
import urllib.error
import urllib.request

source = Path(__file__).resolve().parent.parent
binary, out = (Path(p).resolve() for p in sys.argv[1:])
out.mkdir(mode=0o755)  # must not exist; preserve every earlier run
name = f"agent-bus-g13-{os.getpid()}"
state = Path("/var/lib") / name
runtime = Path("/run") / name
unit_path = Path("/run/systemd/system") / (name + ".service")

def run(*args, **kw):
    return sp.check_output(args, text=True, **kw)

def root(*args, **kw):
    return run("sudo", "-n", *map(str, args), **kw)

def check(condition, label):
    print(("ok " if condition else "FAIL ") + label, flush=True)
    if not condition:
        failures.append(label)

class UnixHTTP(http.client.HTTPConnection):
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.settimeout(3)
        self.sock.connect(str(runtime / "bus.sock"))

def api(path, token="", body=None):
    conn = UnixHTTP("bus", timeout=3)
    conn.request("POST" if body is not None else "GET", path,
                 json.dumps(body) if body is not None else None,
                 {"X-Agent-Bus-Token": token, "Content-Type": "application/json"})
    response = conn.getresponse()
    data = response.read()
    status = response.status
    conn.close()
    return status, json.loads(data) if status == 200 else data.decode()

def wait_for(fn):
    last = None
    for _ in range(100):
        try:
            value = fn()
            if value:
                return value
        except (OSError, ValueError, http.client.HTTPException) as e:
            last = e
        time.sleep(.1)
    raise RuntimeError(f"fixture did not become ready: {last}")

def page(token="", pid="1"):
    req = urllib.request.Request(f"http://127.0.0.1:{port}/", headers={
        "X-Agent-Bus-Token": token, "X-Probe-Host-Pid": pid})
    with urllib.request.urlopen(req, timeout=5) as res:
        return json.load(res)

failures = []
(out / "bin").mkdir()
for exe in ("agent-busd", "agent-bus-setup", "agent-bus-web"):
    shutil.copy2(binary / exe, out / "bin" / exe)
shutil.copy2(binary / "agent-bus-web", out / "web-real")
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
env = dict(os.environ, CGO_ENABLED="0")
run("go", "build", "-ldflags", f"-X main.state={state} -X main.runtimeDir={runtime}",
    "-o", str(out / "bin/agent-bus-web"), "./acceptance/web-probe", cwd=source, env=env)
unit = run(str(out / "bin/agent-bus-setup"), "--print-unit", "--owner", "owner@fixture",
           "--addr", "127.0.0.1:0", "--exec", str(out / "bin/agent-busd"))
unit = unit.replace("/var/lib/agent-bus/daemon", str(state))
unit = unit.replace("StateDirectory=agent-bus/daemon", f"StateDirectory={name}")
unit = unit.replace("/run/agent-bus", str(runtime))
unit = unit.replace("RuntimeDirectory=agent-bus\n", f"RuntimeDirectory={name}\n")
lines = []
for line in unit.splitlines():
    if line.startswith("ExecStart="):
        line += " -dump-every 0"
    lines.append(line)
unit = "\n".join(lines).replace("[Install]", f"""Environment=AGENT_BUS_WEB_ADDR=127.0.0.1:{port}
Environment=AGENT_BUS_TOKEN=unrelated-inherited-canary
Environment=UNRELATED_SECRET=another-inherited-canary

[Install]""") + "\n"
(out / "unit.service").write_text(unit)
try:
    root("cp", out / "unit.service", unit_path)
    root("systemctl", "daemon-reload")
    root("systemctl", "start", name)
    wait_for(lambda: api("/identity")[0] == 200)
    root("mkdir", "-p", state / ".ssh")
    root("sh", "-c", 'printf "%s\\n" "DISPOSABLE SSH CANARY" > "$1"', "sh", state / ".ssh/authorized_keys")
    root("chown", "-R", "agent-busd:agent-busd", state / ".ssh")
    owner_token = root("cat", state / "token").split()[1]
    check(api("/user", owner_token, {"name": "visitor@fixture", "create": True})[0] == 200,
          "ordinary visitor is created in disposable daemon")
    code, data = api("/token", owner_token, {"name": "visitor@fixture"})
    check(code == 200, "visitor gets their own credential")
    visitor_token = data["token"]
    supervisor = root("systemctl", "show", name, "-p", "MainPID", "--value").strip()
    bus_pid = run("pgrep", "-P", supervisor, "-x", "agent-busd").strip()
    # Prove the target exists and is readable/writable by this account before
    # attributing refusal to confinement. No live path is opened here.
    for rel in ("token", "dump.json", ".ssh/authorized_keys"):
        root("runuser", "-u", "agent-busd", "--", "test", "-r", state / rel)
        root("runuser", "-u", "agent-busd", "--", "test", "-w", state / rel)
    root("runuser", "-u", "agent-busd", "--", "test", "-r", f"/proc/{bus_pid}/root{state}/token")
    # Mapped socket really grants owner standing without a token outside.
    mapped = root("runuser", "-u", "agent-busd", "--", "curl", "--silent", "--fail",
                  "--unix-socket", runtime / "user-agent-busd.sock", "http://bus/status")
    check(json.loads(mapped)["you"] == "owner@fixture", "mapped socket positive control")
    before = {p: root("sha256sum", state / p).split()[0]
              for p in ("token", "dump.json", ".ssh/authorized_keys")}
    result = wait_for(lambda: page(visitor_token, bus_pid))
    (out / "probe.json").write_text(json.dumps(result, indent=2) + "\n")
    for key in ("read token", "write token", "read dump.json", "write dump.json",
                "read .ssh/authorized_keys", "write .ssh/authorized_keys",
                "mapped socket", "host proc root", "host process visibility", "environment"):
        check(result.get(key) is True, "web refuses " + key)
    check(result.get("api status") == 200 and result.get("api you") == "visitor@fixture",
          "shared socket forwards ordinary visitor authority")
    for token, label in (("", "anonymous"), ("bad-token", "invalid credential")):
        check(page(token, bus_pid).get("api status") == 401, "shared socket refuses " + label)
    after = {p: root("sha256sum", state / p).split()[0] for p in before}
    check(before == after, "write canaries stayed unchanged")
    (out / "canary-hashes.json").write_text(json.dumps({"before": before, "after": after}, indent=2))
    if not failures:
        # Real renderer, identical launch path and unit; it must still serve
        # an anonymous login and a visitor session after the adversarial probe.
        root("systemctl", "stop", name)
        shutil.copy2(out / "web-real", out / "bin/agent-bus-web")
        root("systemctl", "start", name)
        def login_page():
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/signin", timeout=3) as r:
                return r.read().decode()
        check("Sign in" in wait_for(login_page), "real confined renderer serves sign-in")
        import http.cookiejar
        import urllib.parse
        jar = http.cookiejar.CookieJar()
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
        with client.open(f"http://127.0.0.1:{port}/signin",
                         urllib.parse.urlencode({"token": visitor_token}).encode(), timeout=5) as r:
            html = r.read().decode()
        check("visitor@fixture" in html, "real confined renderer signs in as visitor")
        root("systemctl", "stop", name)
        root("mkdir", "-p", state / "tls")
        root("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
             "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost",
             "-keyout", state / "tls/key", "-out", state / "tls/cert", stderr=sp.DEVNULL)
        root("chown", "-R", "agent-busd:agent-busd", state / "tls")
        context = ssl.create_default_context(cadata=root("cat", state / "tls/cert"))
        tls_unit = unit.replace("[Install]", f"""Environment=AGENT_BUS_WEB_CERT={state}/tls/cert
Environment=AGENT_BUS_WEB_KEY={state}/tls/key

[Install]""")
        (out / "tls-unit.service").write_text(tls_unit)
        root("cp", out / "tls-unit.service", unit_path)
        root("systemctl", "daemon-reload")
        root("systemctl", "start", name)
        def tls_page():
            with urllib.request.urlopen(f"https://localhost:{port}/signin", context=context, timeout=3) as r:
                return r.read().decode()
        check("Sign in" in wait_for(tls_page), "confined renderer reads explicitly bound TLS files")
        root("systemctl", "stop", name)
        root("rm", state / "tls/key")
        root("systemctl", "start", name)
        wait_for(lambda: api("/identity")[0] == 200)
        time.sleep(1)
        with socket.socket() as s:
            check(s.connect_ex(("127.0.0.1", port)) != 0, "missing TLS input cannot downgrade to HTTP")
        root("systemctl", "stop", name)
        absent_wrap = unit.replace("[Install]", "Environment=PATH=/no-bubblewrap-here\n\n[Install]")
        (out / "absent-wrapper.service").write_text(absent_wrap)
        root("cp", out / "absent-wrapper.service", unit_path)
        root("systemctl", "daemon-reload")
        root("systemctl", "start", name)
        time.sleep(1)
        with socket.socket() as s:
            check(s.connect_ex(("127.0.0.1", port)) != 0, "missing sandbox cannot start an unconfined web child")
        check("bubblewrap is required" in root("journalctl", "--no-pager", "-u", name),
              "missing sandbox has an explicit diagnostic")
finally:
    (out / "journal.log").write_text(root("journalctl", "--no-pager", "-u", name))
    root("systemctl", "stop", name)
    root("rm", "-f", unit_path)
    root("systemctl", "daemon-reload")
    # The names are generated above, never taken from user input.
    root("rm", "-rf", state, runtime)
if failures:
    raise SystemExit(1)
print("PASS web authority isolation under generated systemd policy", flush=True)
