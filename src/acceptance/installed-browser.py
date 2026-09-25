#!/usr/bin/env python3
"""Real-browser journey through the installed TypeScript web face.

Runs only inside installed-browser.sh's disposable container: it restarts the
node's own units. Each check names what a broken face would do differently;
a page that loads without its CDN assets, a CSP violation, a lost session or
a sideways scroll fails its own line.
"""

import argparse
import json
import subprocess
import time
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright

COOKIE = "agent_bus_session"
OWNER = "owner@fresh"


def check(ok: bool, message: str) -> None:
    if not ok:
        raise RuntimeError(message)


def healthy(base: str, timeout: float = 20) -> None:
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        try:
            with urllib.request.urlopen(base + "/healthz", timeout=2) as r:
                if r.status == 200:
                    return
        except OSError:
            pass
        time.sleep(0.2)
    raise RuntimeError("the web face did not come back")


def restart(unit: str, base: str) -> None:
    subprocess.run(["systemctl", "restart", unit], check=True)
    healthy(base)
    # The web face answers /healthz before the daemon it asks: wait for it too.
    until = time.monotonic() + 20
    while time.monotonic() < until:
        try:
            urllib.request.urlopen("http://127.0.0.1:6767/identity", timeout=2)
            return
        except OSError:
            time.sleep(0.2)
    raise RuntimeError(f"{unit} restart: the daemon did not come back")


def main() -> None:
    if not Path("/run/.containerenv").exists():
        raise SystemExit("refusing: this gate restarts the node's own units and runs only inside the installed-browser container (src/acceptance/installed-browser.sh)")
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", required=True)
    ap.add_argument("--token", type=Path, required=True)
    ap.add_argument("--version", required=True)
    ap.add_argument("--evidence", type=Path, required=True)
    ap.add_argument("--browser", default="/usr/bin/chromium")
    args = ap.parse_args()
    base, token = args.base, args.token.read_text().strip()
    check(bool(token), "the owner token fixture is empty")

    problems: list[str] = []
    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True, executable_path=args.browser, args=["--no-sandbox"])
        context = browser.new_context(viewport={"width": 1440, "height": 900})
        # A CSP refusal is an event, not always a console line: record both.
        context.add_init_script("window.__csp = []; document.addEventListener('securitypolicyviolation', e => window.__csp.push(e.blockedURI + ' ' + e.violatedDirective));")
        page = context.new_page()
        page.on("console", lambda m: m.type == "error" and problems.append(f"console: {m.text}"))
        page.on("pageerror", lambda e: problems.append(f"script: {e}"))
        page.on("requestfailed", lambda r: problems.append(f"request failed: {r.url} {r.failure}"))
        page.on("response", lambda r: r.status >= 400 and problems.append(f"http {r.status}: {r.url}"))

        def clean(where: str) -> None:
            csp = page.evaluate("window.__csp || []")
            check(not csp, f"{where}: CSP refused {csp}")
            check(not problems, f"{where}: {problems}")

        def signed_in(where: str) -> None:
            who = page.locator("form.who code")
            check(who.count() == 1 and who.inner_text() == OWNER, f"{where}: not signed in as {OWNER}")

        # Anonymous landing, then the visible sign-in form.
        page.goto(base + "/", wait_until="networkidle")
        check(page.locator("input#token").count() == 1, "the landing page has no sign-in field")
        check(page.locator("form.who").count() == 0, "an anonymous visitor is shown as signed in")
        page.fill("input#token", token)
        page.get_by_role("button", name="Sign in").click()
        page.wait_for_load_state("networkidle")
        signed_in("after sign-in")
        cookies = [c for c in context.cookies() if c["name"] == COOKIE]
        check(len(cookies) == 1, "the session cookie was not set exactly once")
        cookie = cookies[0]
        check(cookie["value"] != token, "the session cookie repeats the token")
        check(cookie["httpOnly"] and cookie["sameSite"] == "Strict", "the session cookie is not HttpOnly and SameSite=Strict")
        check(not cookie["secure"], "plain installed HTTP marks the cookie Secure")
        check(token not in page.url and token not in page.content(), "the token reached the URL or the page")

        # The CDN assets arrived and ran: fonts, icons, charts.
        page.evaluate("document.fonts.ready")
        check(page.evaluate("document.fonts.check('16px \"Geist Variable\"')"), "the Geist font did not load from the CDN")
        check(page.locator("svg.lucide").count() > 0, "no Lucide icon was drawn")
        check(page.locator("i[data-lucide]").count() == 0, "Lucide placeholders were left undrawn")
        clean("Overview")

        pages = {
            "/": "Overview", "/agents": "Agents", "/services": "Services", "/queues": "Queues", "/pubsub": "PubSub",
            "/queues?personal=1": "Queues", "/users": "Users", "/groups": "Groups", "/activity": "Activity",
            "/activity?range=week": "Activity", "/activity?range=month": "Activity", "/diagnostics": "Diagnostics",
            "/account": "Account", "/agent?name=%23c3po%40tatooine": "c3po@tatooine",
            "/queue?name=air-supply%40druidia": "air-supply@druidia",
            "/pubsub/topic?name=spaceballs-merch%40spaceball": "spaceballs-merch@spaceball",
            "/service?name=schwartz%40vega": "schwartz@vega", "/user?name=luke%40tatooine": "Luke Skywalker",
        }
        titles = {}
        for path, title in pages.items():
            r = page.goto(base + path, wait_until="networkidle")
            check(r.status == 200, f"{path} answered {r.status}")
            check(page.locator("h1").count() == 1, f"{path} has no single title")
            titles[path] = page.locator("h1").inner_text()
            check(title in titles[path], f"{path} is titled {titles[path]!r}, not {title!r}")
            signed_in(path)
            clean(path)
        page.goto(base + "/activity", wait_until="networkidle")
        check(page.locator(".uplot-host canvas").count() > 0, "the activity chart was not drawn by uPlot")
        page.screenshot(path=args.evidence / "activity-1440.png", full_page=True)

        # The theme toggle persists as a cookie and survives a reload.
        before = page.evaluate("document.documentElement.dataset.theme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')")
        page.locator("button[data-action=theme]").first.click()
        # The switch runs inside a view transition, so it lands a frame later.
        try:
            page.wait_for_function("b => ['dark', 'light'].includes(document.documentElement.dataset.theme) && document.documentElement.dataset.theme !== b", arg=before, timeout=3000)
        except Exception:
            raise RuntimeError(f"the theme toggle did not switch from {before!r}")
        after = page.evaluate("document.documentElement.dataset.theme")
        check(any(c["name"] == "ab_theme" and c["value"] == after for c in context.cookies()), "the chosen theme was not kept as a cookie")
        page.reload(wait_until="networkidle")
        check(page.evaluate("document.documentElement.dataset.theme") == after, "the chosen theme did not survive a reload")

        # The palette opens on its shortcut, finds a sample record and goes there.
        page.keyboard.press("Control+k")
        check(page.locator("dialog.palette[open]").count() == 1, "Ctrl+K did not open the palette")
        page.keyboard.type("c3po")
        # Records come from /palette.json, so a match for a sample record
        # proves the palette's own fetch as well as its drawing.
        hit = page.locator("dialog.palette li", has_text="c3po")
        try:
            hit.first.wait_for(timeout=5000)
        except Exception:
            raise RuntimeError(f"the palette found no c3po record: {page.locator('dialog.palette li').all_inner_texts()[:8]}")
        hit.first.click()
        page.wait_for_load_state("networkidle")
        check("c3po" in page.url, f"the palette did not open the record: {page.url}")
        clean("palette")

        # A phone-width viewport has no sideways scroll.
        phone = browser.new_context(viewport={"width": 390, "height": 844})
        phone.add_cookies([{"name": COOKIE, "value": cookie["value"], "url": base}])
        pp = phone.new_page()
        for path in ("/", "/agents", "/activity", "/queue?name=air-supply%40druidia"):
            pp.goto(base + path, wait_until="networkidle")
            wide = pp.evaluate("document.documentElement.scrollWidth - document.documentElement.clientWidth")
            check(wide <= 1, f"{path} scrolls sideways by {wide}px at 390px")
        pp.screenshot(path=args.evidence / "overview-390.png", full_page=True)
        phone.close()

        # The face holds no session: restarting it keeps the visitor signed in.
        restart("agent-bus-web", base)
        page.goto(base + "/services", wait_until="networkidle")
        signed_in("after the web face restarted")

        # The daemon never stores a session: its restart signs everyone out.
        restart("agent-busd", base)
        page.goto(base + "/services", wait_until="networkidle")
        check(page.locator("form.who").count() == 0, "a session outlived the daemon restart")
        page.goto(base + "/", wait_until="networkidle")
        # Both pages met the ended session's cookie and answered 401 on
        # purpose; those lines, and only those, are expected.
        check(problems and all("401" in p for p in problems), f"after the daemon restart: {problems}")
        problems.clear()
        page.fill("input#token", token)
        page.get_by_role("button", name="Sign in").click()
        page.wait_for_load_state("networkidle")
        signed_in("sign-in after the daemon restart")
        renewed = [c for c in context.cookies() if c["name"] == COOKIE]
        check(len(renewed) == 1 and renewed[0]["value"] != cookie["value"], "the daemon restart did not yield a new session")

        # Sign-out ends the session: the same cookie replayed is nobody.
        page.locator("form.who button[type=submit]").click()
        page.wait_for_load_state("networkidle")
        check(page.locator("input#token").count() == 1, "sign-out did not return to the sign-in form")
        replay = browser.new_context()
        replay.add_cookies([{"name": COOKIE, "value": renewed[0]["value"], "url": base}])
        rp = replay.new_page()
        rp.goto(base + "/account", wait_until="networkidle")
        check(rp.locator("form.who").count() == 0, "a signed-out session still authenticates when replayed")
        replay.close()
        clean("sign-out")

        (args.evidence / "browser.json").write_text(json.dumps({
            "version": args.version,
            "browser": subprocess.check_output([args.browser, "--version"], text=True).strip(),
            "cookie": {"httpOnly": cookie["httpOnly"], "sameSite": cookie["sameSite"], "secure": cookie["secure"]},
            "titles": titles,
        }, indent=2) + "\n")
        browser.close()
    print("PASS a real Chromium signs in, loads the CDN fonts, icons and charts under the CSP, walks every page and the sample records, "
          "keeps the theme, drives the palette, fits 390px, survives a web restart, loses its session to a daemon restart, and signs out")


if __name__ == "__main__":
    main()
