#!/usr/bin/env python3
"""F.12 session, cookie and restart journey on the packaged disposable real-systemd host."""

import argparse
import json
import os
import signal
import subprocess
import time
from pathlib import Path

from playwright.sync_api import Error as PlaywrightError
from playwright.sync_api import sync_playwright


def check(ok: bool, message: str) -> None:
    if not ok:
        raise RuntimeError(message)


def process_env(pid: int) -> bytes:
    try:
        return Path(f"/proc/{pid}/environ").read_bytes()
    except (FileNotFoundError, PermissionError, ProcessLookupError):
        return b""


def parent(pid: int) -> int:
    try:
        for line in Path(f"/proc/{pid}/status").read_text().splitlines():
            if line.startswith("PPid:"):
                return int(line.split()[1])
    except (FileNotFoundError, PermissionError, ProcessLookupError, ValueError):
        pass
    return -1


def bus_child(supervisor: int) -> int:
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        if parent(pid) == supervisor and b"AGENT_BUS_ROLE=bus\0" in process_env(pid):
            return pid
    raise RuntimeError("bus child was not found")


def web_renderer() -> int:
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            command = (entry / "cmdline").read_bytes().replace(b"\0", b" ").strip()
        except (FileNotFoundError, PermissionError, ProcessLookupError):
            continue
        if command == b"/agent-bus-web":
            return int(entry.name)
    raise RuntimeError("confined web renderer was not found")


def wait_replacement(find, old: int, label: str, timeout: float = 12) -> int:
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        try:
            current = find()
            if current != old:
                return current
        except RuntimeError:
            pass
        time.sleep(0.1)
    raise RuntimeError(f"{label} was not replaced")


def wait_page(page, url: str, signed_in: bool, timeout: float = 12) -> None:
    until = time.monotonic() + timeout
    last = ""
    while time.monotonic() < until:
        try:
            page.goto(url, wait_until="domcontentloaded", timeout=1500)
            owner = page.locator("form.who code").count() == 1
            signin = page.locator('input[name="token"]').count() == 1
            if (signed_in and owner) or (not signed_in and signin):
                return
            last = f"owner={owner} signin={signin} title={page.title()}"
        except PlaywrightError as err:
            last = str(err)
        page.wait_for_timeout(100)
    raise RuntimeError(f"page did not become {'signed in' if signed_in else 'anonymous'}: {last}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", required=True)
    parser.add_argument("--token", type=Path, required=True)
    parser.add_argument("--supervisor", type=int, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--browser", default="/usr/bin/chromium")
    args = parser.parse_args()
    token = args.token.read_text().strip()
    check(bool(token), "fixture owner token is empty")

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(
            headless=True,
            executable_path=args.browser,
            args=["--no-sandbox"],
        )
        context = browser.new_context(viewport={"width": 375, "height": 820})
        page = context.new_page()
        page.goto(args.base, wait_until="domcontentloaded")
        check(page.locator('input[name="token"]').count() == 1, "anonymous sign-in form is absent")
        check(page.locator("form.who").count() == 0, "an anonymous browser is shown as signed in")
        page.get_by_role("textbox", name="token").fill(token)
        page.get_by_role("button", name="sign in").click()
        page.wait_for_load_state("domcontentloaded")
        check(page.locator("form.who code").inner_text() == "owner@fresh", "visible sign-in did not become the owner")

        cookies = [cookie for cookie in context.cookies() if cookie["name"] == "agent_bus_session"]
        check(len(cookies) == 1, "session cookie was not set exactly once")
        cookie = cookies[0]
        check(cookie["value"] != token, "session cookie repeats the token")
        check(cookie["httpOnly"] is True, "session cookie is not HttpOnly")
        check(cookie["sameSite"] == "Strict", "session cookie is not SameSite=Strict")
        check(cookie["secure"] is False, "plain installed HTTP unexpectedly marks the cookie Secure")
        check(token not in page.url and token not in page.content(), "token reached the URL or rendered page")

        # Every tab is its own page: "/" also answers an unknown path, so each
        # is held to its own title rather than to having one.
        # The 0.8 addresses (docs/web-face/site-map.md): Channels split into
        # Queues and PubSub in 0.8.4; the header links every one of them.
        routes = {"/": "Overview", "/agents": "Agents", "/services": "Services", "/queues": "Queues",
                  "/pubsub": "PubSub", "/personal": "Personal", "/users": "Users", "/groups": "Groups",
                  "/activity": "Activity", "/diagnostics": "Diagnostics", "/account": "Account"}
        titles = {}
        for route, title in routes.items():
            response = page.goto(args.base + route, wait_until="domcontentloaded")
            check(response.status == 200, f"{route} answered {response.status}")
            check(page.locator("h1").count() == 1, f"{route} has no unique title")
            check(page.locator("form.who code").inner_text() == "owner@fresh", f"{route} lost the signed-in identity")
            titles[route] = page.locator("h1").inner_text()
            check(title in titles[route], f"{route} is titled {titles[route]!r}, not {title}")
        for route in ("/agents", "/services", "/queues", "/pubsub", "/users", "/groups", "/activity", "/diagnostics"):
            check(page.locator(f'header nav a[href="{route}"]').count() == 1, f"the header does not link {route}")
        page.goto(args.base + "/channels", wait_until="domcontentloaded")
        check(page.url == args.base + "/queues", f"the retired /channels address did not redirect to /queues: {page.url}")
        titles["/channels"] = "-> " + page.url[len(args.base):]
        page.screenshot(path=args.evidence / "browser-owner.png", full_page=True)

        old_web = web_renderer()
        os.kill(old_web, signal.SIGKILL)
        new_web = wait_replacement(web_renderer, old_web, "web renderer")
        wait_page(page, args.base + "/services", True)
        check(page.locator("form.who code").inner_text() == "owner@fresh", "web restart lost the bus-held session")

        old_bus = bus_child(args.supervisor)
        os.kill(old_bus, signal.SIGKILL)
        new_bus = wait_replacement(lambda: bus_child(args.supervisor), old_bus, "bus child")
        wait_page(page, args.base + "/", False)
        check(page.locator("form.who").count() == 0, "old session retained authority after bus restart")

        page.get_by_role("textbox", name="token").fill(token)
        page.get_by_role("button", name="sign in").click()
        page.wait_for_load_state("domcontentloaded")
        check(page.locator("form.who code").inner_text() == "owner@fresh", "sign-in after bus restart failed")
        renewed = [item for item in context.cookies() if item["name"] == "agent_bus_session"]
        check(len(renewed) == 1 and renewed[0]["value"] != cookie["value"], "bus restart did not yield a new browser session")
        page.get_by_role("button", name="sign out").click()
        page.wait_for_load_state("domcontentloaded")
        check(page.locator('input[name="token"]').count() == 1, "visible sign-out did not return to sign-in")
        replay = browser.new_context(viewport={"width": 375, "height": 820})
        replay.add_cookies([{
            "name": "agent_bus_session",
            "value": renewed[0]["value"],
            "url": args.base,
            "httpOnly": True,
            "sameSite": "Strict",
        }])
        replay_page = replay.new_page()
        replay_page.goto(args.base, wait_until="domcontentloaded")
        check(replay_page.locator('input[name="token"]').count() == 1, "signed-out session still authenticates when replayed")
        replay.close()
        page.screenshot(path=args.evidence / "browser-signed-out.png", full_page=True)

        (args.evidence / "browser.json").write_text(json.dumps({
            "browser": subprocess.check_output(["/usr/bin/chromium", "--version"], text=True).strip(),
            "cookie": {"httpOnly": cookie["httpOnly"], "sameSite": cookie["sameSite"], "secure": cookie["secure"]},
            "routes": titles,
            "web": {"before": old_web, "after": new_web},
            "bus": {"before": old_bus, "after": new_bus},
            "viewport": {"width": 375, "height": 820},
        }, indent=2) + "\n")
        browser.close()

    print("PASS installed Chromium sign-in, cookie boundary, 0.8 pages, web and bus restart semantics and sign-out")


if __name__ == "__main__":
    main()
