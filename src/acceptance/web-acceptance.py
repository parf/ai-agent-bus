#!/usr/bin/env python3
"""F.13.6: performance, accessibility and migration acceptance of the web face.

Usage: web-acceptance.py BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR [BASE_PORT]

Runs a real Chrome against disposable daemons and web faces this gate starts
itself: a populated directory, an empty one, and the populated face with its
bus taken away. Every bus call the web makes passes through a counting proxy,
so bus calls are measured separately from what the browser fetches. Nothing
here names or opens an installed daemon. Every wait is bounded; any FAIL line
exits non-zero. Evidence: Plans/MVP/done/web-acceptance.md.
"""
import http.client
import json
import os
import platform
import re
import shutil
import signal
import socket
import statistics
import subprocess as sp
import sys
import threading
import time
from pathlib import Path

from playwright.sync_api import sync_playwright

CHROME = "/usr/bin/google-chrome"
OWNER = "owner@localhost"
ROWS = 500          # populated directory: this many of each kind
LARGE = 2500        # the larger benchmark: agents after the second seeding

# Budgets, set from the measurements recorded in the evidence
# (Plans/MVP/done/web-acceptance.md#budgets). A page is its HTML plus what
# the browser fetches for it; bus calls are what the web child asks the
# daemon while rendering it, counted by the proxy below.
BUDGET_REQUESTS = 4           # browser requests per page, every fixture (measured max 4)
BUDGET_TOTAL_BYTES = 400_000  # browser bytes per page at 500 rows (measured max 306 KB, /account)
BUDGET_LISTING_BYTES = 60_000  # HTML of one paginated listing page (measured max 44 KB)
BUDGET_GROUPS_BYTES = 300_000  # the unpaginated Groups page at 500 groups (measured 222 KB)
BUDGET_BUS_CALLS = 8          # bus calls per page render, every fixture (measured max 8)
BUDGET_EXTRA_CALLS = 0        # populated minus empty bus calls, any page (measured 0)
BUDGET_LIST_MS = 250          # median server time, 500-row listing (measured max 31 ms)
BUDGET_LARGE_MS = 500         # median server time at 2500 agents (measured max 67 ms)
PAGINATED = ["/agents", "/services", "/queues", "/pubsub", "/personal", "/users"]

binary = Path(sys.argv[1]).resolve()
out = Path(sys.argv[2]).resolve()
base = int(sys.argv[3]) if len(sys.argv) > 3 else 35000
out.mkdir(parents=True, exist_ok=False)
checks = failures = 0
measure = {}


def check(ok, label, detail=""):
    global checks, failures
    checks += 1
    if ok:
        print("ok   " + label, flush=True)
    else:
        failures += 1
        print("FAIL " + label + (": " + str(detail) if detail else ""), flush=True)
    return ok


def wait(fn, label, seconds=20):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        try:
            v = fn()
            if v:
                return v
        except (OSError, ValueError, http.client.HTTPException):
            pass
        time.sleep(.05)
    raise RuntimeError("timed out: " + label)


# ---------------------------------------------------------------- the bus

class Bus:
    """One disposable daemon: its own directory, database, sockets, port."""

    def __init__(self, name, port):
        self.dir = out / name
        self.dir.mkdir()
        # A socket path is limited to 107 bytes, so the sockets live in a
        # short private directory of this user's runtime directory.
        runtime = Path(os.environ.get("XDG_RUNTIME_DIR") or out)
        self.run = runtime / f"ab-f136-{os.getpid()}-{name}"
        self.run.mkdir(mode=0o700)
        self.sock = self.run / "bus.sock"
        self.realm = ""  # records are named without one unless the daemon adds it
        self.log = open(self.dir / "daemon.log", "w")
        self.proc = sp.Popen([binary / "agent-busd", "-addr", f"127.0.0.1:{port}",
                              "-socket", self.sock, "-owner", OWNER,
                              "-db", self.dir / "bus.db", "-create", "-flush-every", "0"],
                             stdout=self.log, stderr=sp.STDOUT, start_new_session=True)
        wait(lambda: self.call("GET", "/status")[0] in (200, 401), name + " daemon answers")
        env = dict(os.environ, AGENT_BUS_ADDR=str(self.run / f"user-{account()}.sock"))
        env.pop("AGENT_BUS_TOKEN", None)
        self.token = sp.check_output([binary / "agent-bus-token", OWNER], env=env, text=True,
                                     timeout=10).strip()

    @property
    def at(self):
        return "%40" + self.realm if self.realm else ""

    @property
    def suffix(self):
        return "@" + self.realm if self.realm else ""

    def call(self, method, path, body=None, token=None, timeout=10):
        conn = http.client.HTTPConnection("bus", timeout=timeout)
        conn.sock = socket.socket(socket.AF_UNIX)
        conn.sock.settimeout(timeout)
        conn.sock.connect(str(self.sock))
        headers = {"X-Agent-Bus-Token": token if token is not None else getattr(self, "token", "")}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        conn.request(method, path, body=data, headers=headers)
        r = conn.getresponse()
        answer = r.read()
        conn.close()
        return r.status, answer

    def post(self, path, body):
        status, answer = self.call("POST", path, body)
        if status != 200:
            raise RuntimeError(f"seed {path} {body}: HTTP {status} {answer[:200]!r}")
        return answer

    def stop(self):
        if self.proc.poll() is None:
            self.proc.send_signal(signal.SIGTERM)
            try:
                self.proc.wait(10)
            except sp.TimeoutExpired:
                self.proc.kill()
                self.proc.wait(5)
        shutil.rmtree(self.run, ignore_errors=True)


def account():
    import pwd
    return pwd.getpwuid(os.getuid()).pw_name


def seed(bus, start, count, kinds):
    """count records of each kind, named so a search can find exactly one.
    Users come before the groups whose members they are."""
    def one(kind, i):
        n = f"{i:04d}"
        if kind == "agent":
            return "/register", {"name": f"#agent-{n}", "kind": "agent", "allow": ["*"],
                                 "descr": f"Seeded agent {n} for the directory benchmark"}
        if kind == "queue":
            return "/register", {"name": f"queue-{n}", "kind": "queue", "allow": ["*"], "descr": f"Seeded queue {n}"}
        if kind == "pubsub":
            return "/register", {"name": f"topic-{n}", "kind": "pubsub", "allow": ["*"], "descr": f"Seeded topic {n}"}
        if kind == "service":
            return "/register", {"name": f"service-{n}", "kind": "service", "allow": ["*"],
                                 "addr": f"http://127.0.0.1:{9000 + i % 500}/", "protocol": "http",
                                 "descr": f"Seeded service {n}"}
        if kind == "user":
            return "/user", {"name": f"person-{n}@localhost", "create": True,
                             "person_name": f"Seeded Person {n}", "email": f"p{n}@example.org"}
        members = [f"person-{start + (i - start + k) % count:04d}@localhost" for k in range(3)]
        return "/group", {"Name": f"@team-{n}", "Members": members}
    for kind in ("agent", "queue", "pubsub", "service", "user", "group"):
        if kind in kinds:
            for i in range(start, start + count):
                answer = json.loads(bus.post(*one(kind, i)) or b"{}")
                if kind == "agent" and isinstance(answer, dict) and "@" in answer.get("name", ""):
                    bus.realm = answer["name"].split("@", 1)[1]


# ------------------------------------------------------ the counting proxy

class CountingProxy:
    """Unix-socket HTTP relay between the web child and the daemon. It counts
    requests, not connections: a kept-alive connection carries many."""

    def __init__(self, path, upstream):
        self.path, self.upstream = Path(path), upstream
        self.calls = []
        self.bytes = 0  # answer bytes the daemon sent back
        self.lock = threading.Lock()
        self.listener = None
        self.conns = set()
        self.start()

    def start(self):
        self.path.unlink(missing_ok=True)
        s = socket.socket(socket.AF_UNIX)
        s.bind(str(self.path))
        s.listen(64)
        self.listener = s
        threading.Thread(target=self.accept, args=(s,), daemon=True).start()

    def down(self):
        """The bus goes away: nothing listens, live connections close."""
        s, self.listener = self.listener, None
        s.close()
        self.path.unlink(missing_ok=True)
        for c in list(self.conns):
            try:
                c.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
            c.close()

    def accept(self, s):
        while True:
            try:
                c, _ = s.accept()
            except OSError:
                return
            threading.Thread(target=self.serve, args=(c,), daemon=True).start()

    def serve(self, client):
        up = socket.socket(socket.AF_UNIX)
        try:
            up.connect(str(self.upstream))
        except OSError:
            client.close()
            return
        self.conns.update((client, up))

        def back():
            try:
                while True:
                    b = up.recv(65536)
                    if not b:
                        break
                    with self.lock:
                        self.bytes += len(b)
                    client.sendall(b)
            except OSError:
                pass
            for x in (client, up):
                try:
                    x.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
        threading.Thread(target=back, daemon=True).start()
        buf = b""
        try:
            while True:
                while b"\r\n\r\n" not in buf:
                    b = client.recv(65536)
                    if not b:
                        return
                    buf += b
                head, buf = buf.split(b"\r\n\r\n", 1)
                line = head.split(b"\r\n", 1)[0].decode("latin-1")
                headers = {k.strip().lower(): v.strip() for k, _, v in
                           (h.decode("latin-1").partition(":") for h in head.split(b"\r\n")[1:])}
                with self.lock:
                    self.calls.append(line.rsplit(" ", 1)[0])
                up.sendall(head + b"\r\n\r\n")
                if headers.get("transfer-encoding", "").lower() == "chunked":
                    while b"0\r\n\r\n" not in buf:
                        b = client.recv(65536)
                        if not b:
                            return
                        buf += b
                    body, buf = buf.split(b"0\r\n\r\n", 1)
                    up.sendall(body + b"0\r\n\r\n")
                else:
                    n = int(headers.get("content-length", "0"))
                    while len(buf) < n:
                        b = client.recv(65536)
                        if not b:
                            return
                        buf += b
                    up.sendall(buf[:n])
                    buf = buf[n:]
        except OSError:
            pass
        finally:
            self.conns.discard(client)
            self.conns.discard(up)
            client.close()
            up.close()

    def mark(self):
        with self.lock:
            return len(self.calls), self.bytes

    def since(self, mark):
        with self.lock:
            return list(self.calls[mark[0]:]), self.bytes - mark[1]


class Web:
    def __init__(self, name, bus, port):
        self.dir = bus.dir
        self.proxy = CountingProxy(bus.run / "counted.sock", bus.sock)
        self.url = f"http://127.0.0.1:{port}"
        self.log = open(self.dir / "web.log", "w")
        env = dict(os.environ, AGENT_BUS_ADDR=str(self.proxy.path))
        for k in ("AGENT_BUS_TOKEN", "AGENT_BUS_NAME"):
            env.pop(k, None)
        self.proc = sp.Popen([binary / "agent-bus-web", "-addr", f"127.0.0.1:{port}"], env=env,
                             stdout=self.log, stderr=sp.STDOUT, start_new_session=True)

        def up():
            c = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
            c.request("GET", "/healthz")
            return c.getresponse().status == 200
        wait(up, name + " web answers")

    def stop(self):
        if self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(10)
            except sp.TimeoutExpired:
                self.proc.kill()


# ------------------------------------------------------------ in the page

CONTRAST_JS = r"""() => {
  const rgba = s => { const m = s.match(/rgba?\(([^)]*)\)/); if (!m) return null;
    const p = m[1].split(/[\s,\/]+/).filter(Boolean).map(parseFloat);
    return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1]; };
  const over = (top, under) => { const a = top[3];
    return [0,1,2].map(i => top[i]*a + under[i]*(1-a)).concat([1]); };
  const lum = c => { const v = c.slice(0,3).map(x => { x /= 255;
    return x <= 0.03928 ? x/12.92 : Math.pow((x+0.055)/1.055, 2.4); });
    return 0.2126*v[0] + 0.7152*v[1] + 0.0722*v[2]; };
  const ratio = (a, b) => { const x = lum(a), y = lum(b);
    return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05); };
  const background = el => { const layers = [];
    for (let e = el; e && e.nodeType === 1; e = e.parentElement) {
      const cs = getComputedStyle(e);
      if (cs.backgroundImage && cs.backgroundImage !== 'none') return null;
      const c = rgba(cs.backgroundColor); if (c && c[3] > 0) { layers.push(c); if (c[3] >= 1) break; } }
    let bg = [255,255,255,1];
    for (let i = layers.length-1; i >= 0; i--) bg = over(layers[i], bg);
    return bg; };
  const out = []; const seen = new Set();
  const all = document.querySelectorAll('body *');
  for (const el of all) {
    const tag = el.tagName.toLowerCase();
    const svgText = el instanceof SVGElement && (tag === 'text' || tag === 'tspan');
    const control = ['input','select','textarea'].includes(tag) && el.type !== 'hidden' && el.type !== 'checkbox' && el.type !== 'radio';
    const own = [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim());
    if (!own && !control) continue;
    if (el instanceof SVGElement && !svgText) continue;
    const r = el.getBoundingClientRect();
    if (r.width < 2 || r.height < 2) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility !== 'visible' || parseFloat(cs.opacity) === 0) continue;
    if (el.closest('[aria-hidden=true]') && !svgText) continue;
    let fg = rgba(svgText ? cs.fill : cs.color); if (!fg) continue;
    const bg = background(el); if (!bg) continue;
    let alpha = 1; for (let e = el; e && e.nodeType === 1; e = e.parentElement) alpha *= parseFloat(getComputedStyle(e).opacity);
    fg = over([fg[0],fg[1],fg[2],fg[3]*alpha], bg);
    const size = parseFloat(cs.fontSize), weight = parseInt(cs.fontWeight) || 400;
    const large = size >= 24 || (size >= 18.66 && weight >= 700);
    const need = large ? 3 : 4.5;
    const got = ratio(fg, bg);
    const text = (control ? (el.value || el.getAttribute('placeholder') || '') : el.textContent).trim().slice(0, 40);
    out.push({tag, cls: el.getAttribute('class') || '', text, ratio: Math.round(got*100)/100, need});
  }
  return out;
}"""

FOCUS_JS = r"""() => { const e = document.activeElement; if (!e || e === document.body) return null;
  const cs = getComputedStyle(e); const r = e.getBoundingClientRect();
  const outline = cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) >= 1;
  const shadow = cs.boxShadow && cs.boxShadow !== 'none';
  return {tag: e.tagName.toLowerCase(), type: e.type || '', name: e.name || '',
    href: e.getAttribute('href') || '', text: (e.textContent || '').trim().slice(0, 30),
    inNav: !!e.closest('nav'), inMain: !!e.closest('main'), search: e.type === 'search',
    visible: (outline || shadow) && r.width > 0 && r.height > 0,
    inView: r.bottom > 0 && r.top < innerHeight && r.right > 0 && r.left < innerWidth}; }"""

OVERFLOW_JS = r"""() => { const d = document.documentElement;
  return {scroll: d.scrollWidth, client: d.clientWidth, body: document.body.scrollWidth}; }"""

AX_JS = r"""() => {
  const name = el => { const l = el.getAttribute('aria-label'); if (l && l.trim()) return l.trim();
    const by = el.getAttribute('aria-labelledby');
    if (by) { const t = by.split(/\s+/).map(i => document.getElementById(i)).filter(Boolean).map(x => x.textContent).join(' ').trim(); if (t) return t; }
    if (el.tagName.toLowerCase() === 'table') { const c = el.querySelector(':scope > caption'); if (c && c.textContent.trim()) return c.textContent.trim(); }
    const t = el.querySelector(':scope > title'); if (t && t.textContent.trim()) return t.textContent.trim();
    return ''; };
  const tables = [...document.querySelectorAll('table')].map(t => ({kind: 'table', name: name(t),
    headers: t.querySelectorAll('th').length, rows: t.querySelectorAll('tbody tr').length}));
  const graphs = [...document.querySelectorAll('svg')].filter(s => s.getAttribute('aria-hidden') !== 'true')
    .map(s => ({kind: 'graph', cls: s.getAttribute('class') || '', role: s.getAttribute('role') || '', name: name(s)}));
  return tables.concat(graphs); }"""


# ----------------------------------------------------------------- the run

PAGES = ["/", "/agents", "/services", "/queues", "/pubsub", "/personal", "/users", "/groups",
         "/activity", "/diagnostics", "/account", "/agents/new", "/services/new", "/queues/new",
         "/pubsub/new", "/groups/new", "/users/new"]
DETAIL = {"populated": ["/agent?name=%23agent-0007{at}", "/queue?name=queue-0007{at}",
                        "/pubsub/topic?name=topic-0007{at}", "/service?name=service-0007{at}",
                        "/group?name=%40team-0007", "/user?name=person-0007%40localhost",
                        "/user?name=owner%40localhost"],
          "empty": ["/user?name=owner%40localhost"]}
LISTINGS = ["/agents", "/services", "/queues", "/pubsub", "/personal", "/users", "/groups"]
SIZES = {"desktop": dict(viewport={"width": 1440, "height": 900}),
         "zoom200": dict(viewport={"width": 640, "height": 450}, device_scale_factor=2),
         "narrow": dict(viewport={"width": 420, "height": 860}, device_scale_factor=2, is_mobile=True,
                        has_touch=True)}


def sign_in(ctx, web, token):
    page = ctx.new_page()
    page.goto(web.url + "/", wait_until="load", timeout=15000)
    page.fill("input[name=token]", token)
    with page.expect_navigation(wait_until="load", timeout=15000):
        page.press("input[name=token]", "Enter")
    check(page.locator("form.who code").count() == 1, f"{web.url}: signed in through the real form")
    return page


def visit(page, web, path):
    """Load one page and say what it cost: browser requests and bytes, and the
    bus calls the web child made while rendering it."""
    seen = []
    handler = lambda req: seen.append(req)
    page.on("requestfinished", handler)
    page.on("requestfailed", handler)
    time.sleep(.05)
    mark = web.proxy.mark()
    resp = page.goto(web.url + path, wait_until="load", timeout=20000)
    page.wait_for_timeout(150)
    page.remove_listener("requestfinished", handler)
    page.remove_listener("requestfailed", handler)
    calls, bus_bytes = web.proxy.since(mark)
    total = html = 0
    for req in seen:
        try:
            s = req.sizes()
            n = s["responseBodySize"] + s["responseHeadersSize"]
        except Exception:
            n = 0
        total += n
        if req.resource_type == "document" and req.redirected_to is None:
            html = n
    timing = page.evaluate("() => { const n = performance.getEntriesByType('navigation')[0];"
                           " return n ? n.responseEnd - n.requestStart : -1 }")
    return {"status": resp.status if resp else 0, "requests": len(seen), "bytes": total,
            "html": html, "bus": len(calls), "bus_bytes": bus_bytes, "calls": calls, "ms": round(timing, 1),
            "url": page.url}


def run():
    ports = iter(range(base, base + 50))
    populated = Bus("populated", next(ports))
    empty = Bus("empty", next(ports))
    buses = [populated, empty]
    webs = []
    try:
        t0 = time.monotonic()
        seed(populated, 0, ROWS, {"agent", "queue", "pubsub", "service", "user", "group"})
        seeded_s = time.monotonic() - t0
        # Traffic, so the Activity page has graphs to label and Diagnostics
        # has exchanges to tabulate.
        for i in range(40):
            populated.post("/send", {"to": f"#agent-{i % 5:04d}{populated.suffix}", "body": f"hello {i}"})
            populated.post("/send", {"to": f"queue-{i % 3:04d}{populated.suffix}", "body": f"work {i}"})
        wp = Web("populated", populated, next(ports))
        we = Web("empty", empty, next(ports))
        webs += [wp, we]
        measure["conditions"] = conditions(populated, seeded_s)
        with sync_playwright() as p:
            browser = p.chromium.launch(executable_path=CHROME, args=["--no-sandbox"])
            measure["conditions"]["chrome"] = browser.version
            try:
                browse(browser, wp, we, populated, empty)
            finally:
                browser.close()
    finally:
        for w in webs:
            w.stop()
        for b in buses:
            b.stop()
        (out / "measurements.json").write_text(json.dumps(measure, indent=1))


def conditions(bus, seeded_s):
    cpu = next((l.split(":", 1)[1].strip() for l in Path("/proc/cpuinfo").read_text().splitlines()
                if l.startswith("model name")), "?")
    mem = next((l.split()[1] for l in Path("/proc/meminfo").read_text().splitlines()
                if l.startswith("MemTotal")), "0")
    status = json.loads(bus.call("GET", "/status")[1])
    return {"cpu": cpu, "cores": os.cpu_count(), "mem_gib": round(int(mem) / 1048576, 1),
            "kernel": platform.release(), "load": os.getloadavg(),
            "version": (binary / "internal").exists() and
            Path(__file__).resolve().parents[1].joinpath("internal/version/VERSION").read_text().strip(),
            "rows_each": ROWS, "seed_seconds": round(seeded_s, 1),
            "db_bytes": (bus.dir / "bus.db").stat().st_size if (bus.dir / "bus.db").exists() else 0,
            "status_keys": sorted(status)[:40] if isinstance(status, dict) else []}


def browse(browser, wp, we, populated, empty):
    results = measure.setdefault("pages", {})
    fixtures = [("populated", wp, populated), ("empty", we, empty)]
    # -- every page, every size, both fixtures: status, reflow, contrast, names
    for size, opts in SIZES.items():
        for fixture, web, bus in fixtures:
            ctx = browser.new_context(**opts)
            page = sign_in(ctx, web, bus.token)
            for path in PAGES + [d.format(at=populated.at) for d in DETAIL[fixture]]:
                v = visit(page, web, path)
                key = f"{fixture} {size} {path}"
                results[key] = {k: v[k] for k in ("status", "requests", "bytes", "html", "bus", "bus_bytes", "ms")}
                check(v["status"] == 200 and "/signin" not in v["url"], f"{key}: renders", v["status"])
                o = page.evaluate(OVERFLOW_JS)
                check(o["scroll"] <= o["client"] + 1, f"{key}: reflow, no horizontal page scroll",
                      f"scrollWidth {o['scroll']} > {o['client']}")
                if size == "narrow":
                    vp = page.evaluate("() => document.querySelector('meta[name=viewport]')?.content || ''")
                    check("width=device-width" in vp, f"{key}: viewport follows the device", vp)
                if size != "zoom200":
                    bad = [c for c in page.evaluate(CONTRAST_JS) if c["ratio"] < c["need"]]
                    check(not bad, f"{key}: measured text contrast meets WCAG AA", bad[:4])
                if size == "desktop":
                    ax = page.evaluate(AX_JS)
                    unnamed = [a for a in ax if not a["name"]]
                    check(not unnamed, f"{key}: every table and graph has an accessible name", unnamed[:3])
                    check(all(a["headers"] > 0 for a in ax if a["kind"] == "table"),
                          f"{key}: every table has header cells")
                    check(v["requests"] <= BUDGET_REQUESTS, f"{key}: browser requests within budget",
                          v["requests"])
                    check(v["bytes"] <= BUDGET_TOTAL_BYTES, f"{key}: browser bytes within budget", v["bytes"])
                    check(v["bus"] <= BUDGET_BUS_CALLS, f"{key}: bus calls within budget",
                          f'{v["bus"]} calls: {v["calls"][:6]}')
            ctx.close()
    # The activity graph is the one chart; it must exist to be named.
    ctx = browser.new_context(**SIZES["desktop"])
    page = sign_in(ctx, wp, populated.token)
    page.goto(wp.url + "/activity", wait_until="load")
    graphs = [a for a in page.evaluate(AX_JS) if a["kind"] == "graph"]
    check(any(g["role"] == "img" and g["name"] for g in graphs), "populated activity: a graph is drawn and labelled",
          graphs[:3])
    # -- directory cost does not grow with rows
    for path in LISTINGS + ["/", "/diagnostics", "/activity"]:
        a = results[f"populated desktop {path}"]
        b = results[f"empty desktop {path}"]
        check(a["bus"] - b["bus"] <= BUDGET_EXTRA_CALLS,
              f"{path}: bus calls do not grow with rows ({b['bus']} empty, {a['bus']} at {ROWS})")
        check(a["requests"] <= b["requests"],
              f"{path}: browser requests do not grow with rows ({b['requests']} empty, {a['requests']} at {ROWS})")
    for path in PAGINATED:
        a = results[f"populated desktop {path}"]
        check(a["html"] <= BUDGET_LISTING_BYTES, f"{path}: one listing page of {ROWS} rows within budget", a["html"])
    a = results["populated desktop /groups"]
    check(a["html"] <= BUDGET_GROUPS_BYTES, f"/groups: {ROWS} groups within budget", a["html"])
    # -- benchmark: median of five server times, at 500 rows and at 2500 agents
    bench = measure.setdefault("bench", {})
    for path in LISTINGS:
        ms = sorted(visit(page, wp, path)["ms"] for _ in range(5))
        bench[f"{path} {ROWS}"] = statistics.median(ms)
        check(bench[f"{path} {ROWS}"] <= BUDGET_LIST_MS, f"{path}: median server time at {ROWS} rows within budget",
              bench[f"{path} {ROWS}"])
    t0 = time.monotonic()
    seed(populated, ROWS, LARGE - ROWS, {"agent"})
    bench["seed_large_s"] = round(time.monotonic() - t0, 1)
    for path in ["/users", "/account", "/diagnostics", "/groups", f"/agent?name=%23agent-0007{populated.at}"]:
        r = [visit(page, wp, path) for _ in range(5)]
        bench[f"{path} {LARGE}"] = statistics.median(x["ms"] for x in r)
        bench[f"{path} {LARGE} bus_bytes"] = r[0]["bus_bytes"]
    runs = [visit(page, wp, "/agents") for _ in range(5)]
    bench[f"/agents {LARGE}"] = statistics.median(r["ms"] for r in runs)
    bench[f"/agents {LARGE} html"] = runs[0]["html"]
    bench[f"/agents {LARGE} bus"] = runs[0]["bus"]
    bench[f"/agents {LARGE} bus_bytes"] = runs[0]["bus_bytes"]
    check(runs[0]["bus"] == results["populated desktop /agents"]["bus"],
          f"/agents: bus calls equal at {ROWS} and {LARGE} rows", (results["populated desktop /agents"]["bus"], runs[0]["bus"]))
    for key in [k for k in bench if k.endswith(f" {LARGE}")]:
        check(bench[key] <= BUDGET_LARGE_MS, f"{key.rsplit(' ', 1)[0]}: median server time at {LARGE} agents within budget",
              bench[key])
    q = visit(page, wp, "/agents?q=agent-2400")
    check(page.locator("tbody tr").count() == 1 and q["bus"] == runs[0]["bus"],
          f"/agents?q=: a search at {LARGE} rows finds one row at the same bus cost", q["bus"])
    ctx.close()
    # -- keyboard operation
    keyboard(browser, wp, populated)
    # -- legacy entry points
    legacy(browser, wp, populated)
    # -- error fixture: the bus is gone
    unavailable(browser, wp, populated)


def tab_through(page, limit=120):
    seen = []
    for _ in range(limit):
        page.keyboard.press("Tab")
        f = page.evaluate(FOCUS_JS)
        if f is None:
            break
        seen.append(f)
        if seen[0] is not f and f["href"] == seen[0]["href"] and f["text"] == seen[0]["text"] and len(seen) > 2:
            break
    return seen


def keyboard(browser, web, bus):
    ctx = browser.new_context(**SIZES["desktop"])
    page = ctx.new_page()
    # Sign in with the keyboard: the token field has focus on arrival.
    page.goto(web.url + "/", wait_until="load")
    check(page.evaluate("() => document.activeElement && document.activeElement.name") == "token",
          "keyboard: the sign-in token field has focus on arrival")
    page.keyboard.type(bus.token)
    with page.expect_navigation(wait_until="load", timeout=15000):
        page.keyboard.press("Enter")
    check(page.locator("form.who code").count() == 1, "keyboard: sign in with typing and Enter only")
    for path, wants in [("/agents", ["nav", "search", "select", "row"]),
                        ("/users", ["nav", "search", "row"]),
                        ("/groups", ["nav", "row"]),
                        ("/queues/new", ["nav", "field", "submit"]),
                        ("/activity", ["nav"]),
                        ("/diagnostics", ["nav"])]:
        page.goto(web.url + path, wait_until="load")
        seq = tab_through(page)
        first = seq[0] if seq else {}
        check(first.get("href") == "#main", f"keyboard {path}: the first Tab reaches the skip link", first)
        reach = {"nav": sum(1 for f in seq if f["inNav"]) >= 9,
                 "search": any(f["search"] for f in seq),
                 "select": any(f["tag"] == "select" for f in seq),
                 "row": any(f["inMain"] and f["tag"] == "a" and "name=" in f["href"] for f in seq),
                 "field": any(f["tag"] == "input" and f["inMain"] and f["type"] in ("text", "") for f in seq),
                 "submit": any(f["tag"] == "button" and f["inMain"] for f in seq)}
        for w in wants:
            check(reach[w], f"keyboard {path}: Tab order reaches {w}")
        hidden = [f for f in seq if not f["visible"]]
        check(not hidden, f"keyboard {path}: every focused control shows its focus", hidden[:3])
        lost = [f for f in seq if not f["inView"]]
        check(not lost, f"keyboard {path}: focus is scrolled into view", lost[:3])
    # The skip link moves focus past the navigation.
    page.goto(web.url + "/agents", wait_until="load")
    page.keyboard.press("Tab")
    page.keyboard.press("Enter")
    page.keyboard.press("Tab")
    f = page.evaluate(FOCUS_JS) or {}
    check(f.get("inMain") and not f.get("inNav"), "keyboard: the skip link's next Tab is inside main", f)
    # Search by keyboard: reach the field, type, Enter.
    page.goto(web.url + "/agents", wait_until="load")
    for _ in range(60):
        page.keyboard.press("Tab")
        if (page.evaluate(FOCUS_JS) or {}).get("search"):
            break
    page.keyboard.type("agent-0042")
    with page.expect_navigation(wait_until="load", timeout=15000):
        page.keyboard.press("Enter")
    check("q=agent-0042" in page.url and page.locator("tbody tr").count() == 1,
          "keyboard: a search typed and submitted with Enter narrows to one row", page.url)
    # Register a queue by keyboard only.
    page.goto(web.url + "/queues/new", wait_until="load")
    for _ in range(60):
        page.keyboard.press("Tab")
        f = page.evaluate(FOCUS_JS) or {}
        if f.get("name") == "name":
            break
    check(f.get("name") == "name", "keyboard: Tab reaches the queue name field", f)
    page.keyboard.type("keyboard-queue")
    with page.expect_navigation(wait_until="load", timeout=15000):
        page.keyboard.press("Enter")
    check("/queue?name=keyboard-queue" in page.url and page.locator("h1").count() == 1,
          "keyboard: a queue registered with typing and Enter opens its page", page.url)
    ctx.close()


def legacy(browser, web, bus):
    ctx = browser.new_context(**SIZES["desktop"])
    page = sign_in(ctx, web, bus.token)
    bus.post("/register", {"name": "legacy-queue", "kind": "queue", "allow": ["*"]})
    bus.post("/register", {"name": "legacy-topic", "kind": "pubsub", "allow": ["*"]})
    r = bus.at
    cases = [
        ("/channels", "/queues", "Queues"),
        ("/channels?q=queue-0001", "/queues?q=queue-0001", "Queues"),
        ("/channels?kind=pubsub", "/pubsub", "PubSub"),
        ("/channels?kind=pubsub&q=topic-0001", "/pubsub?q=topic-0001", "PubSub"),
        ("/channels/new", "/queues/new", "Register"),
        ("/channels/new?kind=pubsub", "/pubsub/new", "Register"),
        (f"/channel?name=legacy-queue{r}", f"/queue?name=legacy-queue{r}", "legacy-queue"),
        (f"/channel?name=legacy-topic{r}", f"/pubsub/topic?name=legacy-topic{r}", "legacy-topic"),
        (f"/channel/edit?name=legacy-queue{r}", f"/queue/edit?name=legacy-queue{r}", "legacy-queue"),
        (f"/channel/edit?name=legacy-topic{r}", f"/pubsub/topic/edit?name=legacy-topic{r}", "legacy-topic"),
        ("/users?kind=other", "/diagnostics#leftovers", "Diagnostics"),
        # The retired kind=users filter is ignored: the Users page, not a refusal.
        ("/users?kind=users", "/users?kind=users", "Users"),
    ]
    for old, new, heading in cases:
        resp = page.goto(web.url + old, wait_until="load")
        h1 = page.locator("h1").first.inner_text() if page.locator("h1").count() else ""
        check(resp.status == 200 and page.url == web.url + new and heading.lower() in h1.lower(),
              f"legacy {old} lands on {new}", (resp.status, page.url, h1))
    # /favicon.ico is a deliberate 404: the icon is the SVG.
    for path, want in [("/healthz", 200), ("/ui.js", 200), ("/favicon.svg", 200), ("/favicon.ico", 404),
                       ("/agent-bus.jpg", 200)]:
        resp = page.goto(web.url + path)
        check(resp.status == want, f"asset {path} answers {want}", resp.status)
    ctx.close()


def unavailable(browser, web, bus):
    ctxs = {s: browser.new_context(**o) for s, o in SIZES.items()}
    pages = {s: sign_in(c, web, bus.token) for s, c in ctxs.items()}
    web.proxy.down()
    try:
        for size, page in pages.items():
            for path in ["/", "/agents", "/users", "/groups", "/activity", "/diagnostics", "/account",
                         f"/agent?name=%23agent-0007{bus.at}"]:
                v = visit(page, web, path)
                key = f"error {size} {path}"
                measure.setdefault("pages", {})[key] = {k: v[k] for k in ("status", "requests", "bytes", "html", "bus", "bus_bytes", "ms")}
                text = page.locator("main").inner_text() if page.locator("main").count() else ""
                # 502: the dashboard answers, the daemon behind it does not.
                check(v["status"] == 502 and "bus is not answering" in text.lower() and "127.0.0.1" not in text
                      and "counted.sock" not in text,
                      f"{key}: says the bus is unavailable without its address", (v["status"], text[:120]))
                o = page.evaluate(OVERFLOW_JS)
                check(o["scroll"] <= o["client"] + 1, f"{key}: reflow, no horizontal page scroll",
                      f"scrollWidth {o['scroll']} > {o['client']}")
                if size != "zoom200":
                    bad = [c for c in page.evaluate(CONTRAST_JS) if c["ratio"] < c["need"]]
                    check(not bad, f"{key}: measured text contrast meets WCAG AA", bad[:4])
                if size == "desktop":
                    check(v["requests"] <= BUDGET_REQUESTS, f"{key}: browser requests within budget", v["requests"])
    finally:
        web.proxy.start()
        for c in ctxs.values():
            c.close()


if __name__ == "__main__":
    t = time.monotonic()
    try:
        run()
    except Exception as e:  # a crash is a failure, never a pass
        failures += 1
        print(f"FAIL gate aborted: {type(e).__name__}: {e}", flush=True)
    for key, value in measure.get("bench", {}).items():
        print(f"measured {key}: {value}")
    print(f"checks {checks}, failed {failures}, {time.monotonic() - t:.0f}s", flush=True)
    sys.exit(1 if failures else 0)
