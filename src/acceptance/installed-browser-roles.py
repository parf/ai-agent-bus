#!/usr/bin/env python3
"""Installed Chromium five-role authority/action matrix for F.12 against the 0.8 web."""

import argparse
import html
import json
import re
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import quote

from playwright.sync_api import sync_playwright


# A field label carries its hint text, so the accessible name only starts
# with the word.
NAME_FIELD = re.compile(r"^Name\b")


def check(ok: bool, message: str) -> None:
    if not ok:
        raise RuntimeError(message)


def read_token(path: Path) -> str:
    value = path.read_text().strip()
    check(bool(value), f"empty fixture token at {path}")
    return value


def sign_in(browser, base: str, token: str):
    context = browser.new_context(viewport={"width": 375, "height": 820})
    page = context.new_page()
    page.goto(base, wait_until="domcontentloaded")
    page.get_by_role("textbox", name="token").fill(token)
    page.get_by_role("button", name="sign in").click()
    page.wait_for_load_state("domcontentloaded")
    check(page.locator("form.who code").count() == 1, "browser role did not sign in")
    return context, page


def submit(page, button: str) -> None:
    page.get_by_role("button", name=button, exact=True).click()
    page.wait_for_load_state("domcontentloaded")


def direct_service_save(page, name: str, description: str):
    with page.expect_navigation() as navigation:
        page.evaluate(
            """([name, description]) => {
              const values = {
                action: 'save', name, descr: description, addr: 'svc.fresh.example:443',
                protocol: 'https', edit_allow: '1', allow: 'dave@fresh alice@fresh'
              };
              const form = document.createElement('form');
              form.method = 'post';
              form.action = '/service';
              for (const [key, value] of Object.entries(values)) {
                const input = document.createElement('input');
                input.name = key;
                input.value = value;
                form.appendChild(input);
              }
              document.body.appendChild(form);
              form.submit();
            }""",
            [name, description],
        )
    response = navigation.value
    return {"status": response.status, "text": page.content()}


def direct_group_save(page, name: str, members: str):
    with page.expect_navigation() as navigation:
        page.evaluate(
            """([name, members]) => {
              const form = document.createElement('form');
              form.method = 'post';
              form.action = '/groups';
              for (const [key, value] of Object.entries({action: 'save', name, members})) {
                // A textarea, as the real form has: an input drops the
                // newlines and would send one unparseable member.
                const input = document.createElement('textarea');
                input.name = key;
                input.value = value;
                form.appendChild(input);
              }
              document.body.appendChild(form);
              form.submit();
            }""",
            [name, members],
        )
    response = navigation.value
    return {"status": response.status, "text": page.content()}


def direct_post(page, action: str, values: dict):
    """A real same-origin form submission the page itself does not offer."""
    with page.expect_navigation() as navigation:
        page.evaluate(
            """([action, values]) => {
              const form = document.createElement('form');
              form.method = 'post';
              form.action = action;
              for (const [key, value] of Object.entries(values)) {
                const input = document.createElement('textarea');
                input.name = key;
                input.value = value;
                form.appendChild(input);
              }
              document.body.appendChild(form);
              form.submit();
            }""",
            [action, values],
        )
    response = navigation.value
    return {"status": response.status, "text": page.content()}


def refused(result) -> bool:
    """A refusal page, not a success page that happens to share a word."""
    return result["status"] in (403, 404) and ("Not yours to see" in result["text"] or "No such name" in result["text"])


def row(page, href: str):
    return page.locator(f'table tbody tr:has(a[href*="{href}"])')


def start_foreign_origin(base: str):
    """A second loopback origin serving real forms aimed at the dashboard.
    SameSite=Strict does not stop them: a different port is the same site, so
    the session cookie is sent and only the dashboard's origin check refuses."""
    forms = {
        "/group": ("/groups", {"action": "save", "name": "@foreign-origin", "members": "dave@fresh"}),
        "/service": ("/service", {"action": "create", "kind": "queue", "name": "foreign-queue@fresh", "allow": "dave@fresh"}),
        "/user": ("/user", {"action": "create", "name": "foreign-user@fresh", "person_name": "Foreign"}),
    }

    class ForeignForm(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path not in forms:
                self.send_error(404)
                return
            target, values = forms[self.path]
            fields = "".join(f'<input type=hidden name="{html.escape(k)}" value="{html.escape(v)}">' for k, v in values.items())
            body = f"""<!doctype html><form method=post action="{html.escape(base + target)}">{fields}
              <button>submit foreign form</button></form>""".encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, _format, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), ForeignForm)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, f"http://127.0.0.1:{server.server_port}"


def main() -> None:
    # It signs in, creates Users and kills the node's web and bus children, so
    # it runs only inside the disposable container fresh-install.sh starts,
    # which podman marks with /run/.containerenv. On a real host it would act
    # on the live daemon.
    if not Path("/run/.containerenv").exists():
        raise SystemExit("refusing: this gate acts on the node's own daemon and runs only inside the fresh-install container (src/acceptance/fresh-install.sh)")
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", required=True)
    parser.add_argument("--owner-token", type=Path, required=True)
    parser.add_argument("--administrator-token", type=Path, required=True)
    parser.add_argument("--resource-owner-token", type=Path, required=True)
    parser.add_argument("--maintainer-token", type=Path, required=True)
    parser.add_argument("--ordinary-token", type=Path, required=True)
    parser.add_argument("--stranger-token", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--browser", default="/usr/bin/chromium")
    args = parser.parse_args()
    tokens = {role: read_token(path) for role, path in {
        "owner": args.owner_token,
        "administrator": args.administrator_token,
        "resource_owner": args.resource_owner_token,
        "maintainer": args.maintainer_token,
        "ordinary": args.ordinary_token,
        "stranger": args.stranger_token,
    }.items()}
    results = {}

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True, executable_path=args.browser, args=["--no-sandbox"])

        # A resource Owner registers an external service (0.7: an address and
        # a protocol, no #) and a queue, edits the service's settings, and
        # assigns a Maintainer on the settings form: registration carries none.
        bob_context, bob = sign_in(browser, args.base, tokens["resource_owner"])
        check(bob.locator("form.who code").inner_text() == "bob@fresh", "resource Owner identity is wrong")
        bob.goto(args.base + "/services/new", wait_until="domcontentloaded")
        check(bob.locator("#form-create").count() == 1, "service registration form is absent")
        check(bob.locator('#form-create [name="maintainers"]').count() == 0, "registration form asks for Maintainers it cannot carry")
        bob.get_by_role("textbox", name=NAME_FIELD).fill("browser-svc@fresh")
        bob.get_by_label("Description").fill("Browser matrix service")
        bob.get_by_label("Address").fill("svc.fresh.example:443")
        bob.get_by_label("Protocol").fill("https")
        # Visible to the Administrator, so its refusal to manage is about
        # authority and not an unknown name.
        bob.locator('#form-create textarea[name="allow"]').fill("dave@fresh\nalice@fresh")
        submit(bob, "Register service")
        check("browser-svc@fresh" in bob.locator("h1").inner_text(), "resource Owner did not register its service")
        check(bob.locator("dd code", has_text="svc.fresh.example:443").count() == 1, "registered service lost its address")
        bob.locator("#settings").click()
        bob.wait_for_load_state("domcontentloaded")
        bob.locator('#form-save input[name="descr"]').fill("Resource Owner updated")
        submit(bob, "Save settings")
        bob.goto(args.base + "/service/edit?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(bob.locator('#form-save input[name="descr"]').input_value() == "Resource Owner updated", "resource Owner settings did not persist")
        check(bob.locator('#form-save textarea[name="maintainers"]').is_enabled(), "resource Owner cannot assign Maintainers")
        bob.locator('#form-save textarea[name="maintainers"]').fill("carol@fresh")
        submit(bob, "Save settings")
        check("Maintainers: carol@fresh" in bob.locator(".detail-meta").inner_text(), "resource Owner did not assign the Maintainer")
        # The same direct save the other roles are refused succeeds for the
        # Owner, so their refusals are about authority and not the request.
        own = direct_service_save(bob, "browser-svc@fresh", "Resource Owner direct")
        check(own["status"] == 200 and "Not yours to see" not in own["text"], "the direct save is refused even for the Owner, so its denials prove nothing")
        bob.goto(args.base + "/service-danger?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(bob.locator("#form-transfer").count() == 1, "resource Owner lacks its own transfer")
        # 📮 Queues (0.8.4 split Channels): a queue's own register page, list
        # and record page, /queues and /queue?name=.
        bob.goto(args.base + "/queues", wait_until="domcontentloaded")
        bob.locator("nav.section-nav").get_by_role("link", name="Register queue", exact=True).click()
        bob.wait_for_load_state("domcontentloaded")
        check(bob.url.endswith("/queues/new"), f"Queues' Register tab led to {bob.url}")
        bob.get_by_role("textbox", name=NAME_FIELD).fill("browser-queue@fresh")
        bob.get_by_label("Description").fill("Browser matrix queue")
        bob.locator('#form-create textarea[name="allow"]').fill("dave@fresh")
        submit(bob, "Register queue")
        check("browser-queue@fresh" in bob.locator("h1").inner_text(), "resource Owner did not register the queue")
        check("/queue?name=" in bob.url, f"a registered queue did not open its /queue page: {bob.url}")
        # Status is active|inactive: Deactivate… confirms; the record's one
        # view is then inactive and unlisted, and Reactivate restores both.
        submit(bob, "Deactivate…")
        submit(bob, "Deactivate")
        check(bob.locator("h1 .state-inactive").count() == 1, "deactivated queue does not read as inactive")
        bob.goto(args.base + "/queues?q=browser-queue&state=active", wait_until="domcontentloaded")
        check(bob.locator('table a[href*="browser-queue"]').count() == 0, "inactive queue is listed as active")
        bob.goto(args.base + "/queues?q=browser-queue&state=inactive", wait_until="domcontentloaded")
        check(bob.locator('tr:has(a[href*="browser-queue"]) .status-glyph[aria-label="Inactive"]').count() == 1, "inactive queue is absent from the Inactive view")
        bob.goto(args.base + "/queue?name=browser-queue%40fresh", wait_until="domcontentloaded")
        submit(bob, "Reactivate")
        check(bob.locator('.status-glyph[aria-label="Active"]').count() == 1, "reactivated queue is not active")
        bob.goto(args.base + "/queues?q=browser-queue&state=active", wait_until="domcontentloaded")
        check(bob.locator('tr:has(a[href*="browser-queue"]) .status-glyph[aria-label="Active"]').count() == 1, "reactivated queue is not listed as active")

        # 📣 PubSub: a topic delivering to that queue, on /pubsub and
        # /pubsub/topic?name=, then its settings.
        bob.goto(args.base + "/pubsub", wait_until="domcontentloaded")
        bob.locator("nav.section-nav").get_by_role("link", name="Register pub/sub topic", exact=True).click()
        bob.wait_for_load_state("domcontentloaded")
        check(bob.url.endswith("/pubsub/new"), f"PubSub's Register tab led to {bob.url}")
        bob.get_by_role("textbox", name=NAME_FIELD).fill("browser-topic@fresh")
        bob.get_by_label("Description").fill("Browser matrix topic")
        bob.locator('#form-create textarea[name="subs"]').fill("browser-queue@fresh")
        bob.locator('#form-create textarea[name="allow"]').fill("dave@fresh")
        submit(bob, "Register pub/sub topic")
        check("browser-topic@fresh" in bob.locator("h1").inner_text(), "resource Owner did not register the topic")
        check("/pubsub/topic?name=" in bob.url, f"a registered topic did not open its /pubsub/topic page: {bob.url}")
        check(bob.locator('details:has(summary:text-is("Deliver-To")) code', has_text="browser-queue@fresh").count() == 1, "the topic page does not show its Deliver-To queue")
        bob.locator("#settings").click()
        bob.wait_for_load_state("domcontentloaded")
        check("/pubsub/topic/edit" in bob.url, f"topic Settings led to {bob.url}")
        bob.locator('#form-save input[name="descr"]').fill("Topic owner updated")
        submit(bob, "Save settings")
        bob.goto(args.base + "/pubsub?q=browser-topic", wait_until="domcontentloaded")
        check(bob.locator('tr:has(a[href*="browser-topic"])', has_text="Topic owner updated").count() == 1, "the topic's saved description is not in its PubSub row")
        bob.screenshot(path=args.evidence / "browser-resource-owner.png", full_page=True)
        bob_context.close()
        results["resource_owner"] = ["register external service", "edit settings", "assign maintainer after registration", "register queue", "deactivate and reactivate queue", "register pub/sub topic delivering to the queue", "edit topic settings"]

        maint_context, maint = sign_in(browser, args.base, tokens["maintainer"])
        check(maint.locator("form.who code").inner_text() == "carol@fresh", "Maintainer identity is wrong")
        maint.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(maint.locator("#settings").count() == 1, "assigned Maintainer lacks settings")
        maint.locator("#settings").click()
        maint.wait_for_load_state("domcontentloaded")
        check(maint.locator('#form-save textarea[name="maintainers"]').is_disabled(), "Maintainer may change owner-only Maintainers")
        check(maint.locator('#form-save input[name="personal"]').is_disabled(), "Maintainer may change owner-only classification")
        maint.locator('#form-save input[name="descr"]').fill("Maintainer updated")
        submit(maint, "Save settings")
        check("Maintainers: carol@fresh" in maint.locator(".detail-meta").inner_text(), "Maintainer's save dropped the Maintainers")
        maint.goto(args.base + "/service/edit?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(maint.locator('#form-save input[name="descr"]').input_value() == "Maintainer updated", "Maintainer settings did not persist")
        maint.get_by_role("link", name="Danger Zone").click()
        maint.wait_for_load_state("domcontentloaded")
        check(maint.locator("h1", has_text="Danger Zone").count() == 1, "Maintainer did not reach the Danger Zone")
        check(maint.locator("#form-transfer").count() == 0, "Maintainer received owner-only transfer")
        maint_context.close()
        results["maintainer"] = ["edit settings", "no classification or Maintainers", "no transfer"]

        # Any User registers a group and owns it (0.7). An ordinary User still
        # administers no user and manages no record it is not named on.
        ordinary_context, ordinary = sign_in(browser, args.base, tokens["ordinary"])
        check(ordinary.locator("form.who code").inner_text() == "dave@fresh", "ordinary identity is wrong")
        ordinary.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(ordinary.locator("#settings").count() == 0 and "You can view this record" in ordinary.locator("body").inner_text(), "ordinary caller did not get the read-only service")
        denied = direct_service_save(ordinary, "browser-svc@fresh", "Ordinary bypass")
        check(denied["status"] == 403 and "Not yours to see" in denied["text"], "ordinary service denial was not rendered")
        for path, name in (("/queue?name=browser-queue%40fresh", "browser-queue@fresh"),
                           ("/pubsub/topic?name=browser-topic%40fresh", "browser-topic@fresh")):
            response = ordinary.goto(args.base + path, wait_until="domcontentloaded")
            check(response.status == 200 and ordinary.locator("h1", has_text=name).count() == 1, f"ordinary caller cannot read {name}, which admits it")
            check(ordinary.locator("#settings").count() == 0, f"ordinary caller is offered {name}'s settings")
        denied = direct_post(ordinary, "/service", {"action": "save", "name": "browser-queue@fresh", "descr": "Ordinary bypass", "edit_allow": "1", "allow": "dave@fresh\neve@fresh"})
        check(refused(denied), f"ordinary queue save was not refused: {denied['status']}")
        # Positive controls for the stranger below: the same listing queries
        # and record pages show dave, who is on each allow list, a row.
        for listing, href in (("/services?q=browser-svc", "browser-svc%40fresh"),
                              ("/queues?q=browser-queue", "browser-queue%40fresh"),
                              ("/pubsub?q=browser-topic", "browser-topic%40fresh")):
            ordinary.goto(args.base + listing, wait_until="domcontentloaded")
            check(row(ordinary, href).count() == 1, f"{listing} does not list the record to dave, who is admitted")
        response = ordinary.goto(args.base + "/users/new", wait_until="domcontentloaded")
        check(response.status == 403 and "Not yours to see" in ordinary.locator("h1").inner_text(), "ordinary caller was not refused at /users/new")
        ordinary.goto(args.base + "/groups", wait_until="domcontentloaded")
        ordinary.get_by_role("link", name="Register group").click()
        ordinary.wait_for_load_state("domcontentloaded")
        ordinary.get_by_role("textbox", name=NAME_FIELD).fill("@browser-team")
        ordinary.locator('#form-save textarea[name="members"]').fill("dave@fresh")
        submit(ordinary, "Register group")
        check("@browser-team" in ordinary.locator("h1").inner_text(), "ordinary User did not register a group")
        check("Owner: dave@fresh" in ordinary.locator(".detail-meta").inner_text(), "the group's registrant is not its Owner")
        response = ordinary.goto(args.base + "/group/edit?name=%40administrators", wait_until="domcontentloaded")
        check(response.status == 403 and "Not yours to see" in ordinary.locator("h1").inner_text(), "ordinary caller was not refused the protected group")
        ordinary_context.close()
        results["ordinary"] = ["read-only visible service, queue and topic", "service and queue mutation denied", "user administration denied", "register and own a group", "protected group denied"]

        # A stranger: a signed-in User on none of these lists and in no group.
        # Every record answers exactly as an unregistered name does, the
        # listings hold no row for it, and each change is refused.
        stranger_context, stranger = sign_in(browser, args.base, tokens["stranger"])
        check(stranger.locator("form.who code").inner_text() == "eve@fresh", "stranger identity is wrong")
        # Each page is compared with the same page for a name nobody holds.
        for route, name in (("/service", "browser-svc"), ("/queue", "browser-queue"),
                            ("/pubsub/topic", "browser-topic"), ("/activity", "browser-svc")):
            response = stranger.goto(f"{args.base}{route}?name=never-registered%40fresh", wait_until="domcontentloaded")
            unknown = (response.status, stranger.locator("main").inner_text().replace("never-registered", "NAME"))
            response = stranger.goto(f"{args.base}{route}?name={name}%40fresh", wait_until="domcontentloaded")
            seen = (response.status, stranger.locator("main").inner_text().replace(name, "NAME"))
            check(response.status == 404 and "No such name" in seen[1], f"stranger was not answered unknown at {route}?name={name}: {response.status}")
            check(seen == unknown, f"{route}?name={name} answers the stranger differently from an unregistered name")
        for listing, href in (("/services?q=browser-svc", "browser-svc%40fresh"),
                              ("/queues?q=browser-queue", "browser-queue%40fresh"),
                              ("/pubsub?q=browser-topic", "browser-topic%40fresh")):
            stranger.goto(args.base + listing, wait_until="domcontentloaded")
            check(stranger.locator(f'main a[href*="{href}"]').count() == 0, f"{listing} shows the stranger a record it may not see")
        for action, values in (("/service", {"action": "save", "name": "browser-svc@fresh", "descr": "Stranger bypass", "addr": "svc.fresh.example:443", "protocol": "https", "edit_allow": "1", "allow": "eve@fresh"}),
                               ("/service", {"action": "deactivate", "name": "browser-queue@fresh"}),
                               ("/groups", {"action": "save", "name": "@browser-team", "members": "eve@fresh"})):
            denied = direct_post(stranger, action, values)
            check(refused(denied), f"stranger {values['action']} of {values['name']} was not refused: {denied['status']}")
        response = stranger.goto(args.base + "/group/edit?name=%40browser-team", wait_until="domcontentloaded")
        check(response.status == 403 and "Not yours to see" in stranger.locator("h1").inner_text(), "stranger reached another User's group editor")
        response = stranger.goto(args.base + "/users/new", wait_until="domcontentloaded")
        check(response.status == 403 and "Not yours to see" in stranger.locator("h1").inner_text(), "stranger reached user registration")
        stranger_context.close()

        # Nobody signed in: every page is the sign-in page, and a posted
        # change is not applied.
        anonymous = browser.new_context(viewport={"width": 375, "height": 820})
        anon = anonymous.new_page()
        for path in ("/services", "/queue?name=browser-queue%40fresh", "/users", "/groups/new"):
            anon.goto(args.base + path, wait_until="domcontentloaded")
            check(anon.locator('input[name="token"]').count() == 1 and anon.locator("form.who").count() == 0, f"anonymous {path} is not the sign-in page")
        anon.goto(args.base + "/", wait_until="domcontentloaded")
        posted = direct_post(anon, "/service", {"action": "deactivate", "name": "browser-queue@fresh"})
        check(anon.locator("form.who").count() == 0, "an anonymous post signed somebody in")
        check(anon.locator('input[name="token"]').count() == 1 or posted["status"] in (401, 403), f"an anonymous post was answered {posted['status']}, not with sign-in")
        # The daemon Owner below finds the queue still active.
        anonymous.close()
        results["stranger"] = ["service, queue, topic and activity answer as unregistered", "no listing row", "service save, queue deactivate and group save refused", "group editor and user registration refused", "anonymous pages are sign-in and an anonymous post changes nothing"]

        admin_context, admin = sign_in(browser, args.base, tokens["administrator"])
        check(admin.locator("form.who code").inner_text() == "alice@fresh", "Administrator identity is wrong")
        admin.goto(args.base + "/users/new", wait_until="domcontentloaded")
        admin.get_by_label("Identity").fill("admin-made@fresh")
        admin.get_by_label("Person name").fill("Admin Made")
        submit(admin, "Save profile")
        check("admin-made@fresh" in admin.locator("h1").inner_text(), "Administrator did not create an ordinary user")
        check(admin.locator("#profile-edit").count() == 1, "Administrator cannot edit the user it created")
        # Users (0.8.8): the listing row, then Deactivate from the user's
        # Access card, confirmed; the Status filter moves the row.
        admin.goto(args.base + "/users?q=admin-made", wait_until="domcontentloaded")
        check(row(admin, "admin-made%40fresh").count() == 1, "the created user has no Users row")
        check(row(admin, "admin-made%40fresh").locator('td[data-label="Authority"]').inner_text().strip() == "User", "the created user's Authority is not User")
        admin.goto(args.base + "/user?name=admin-made%40fresh", wait_until="domcontentloaded")
        admin.locator("details.access-change summary").click()
        submit(admin, "Deactivate…")
        submit(admin, "Deactivate user")
        admin.goto(args.base + "/users?q=admin-made", wait_until="domcontentloaded")
        check(row(admin, "admin-made%40fresh").count() == 0, "a deactivated user is still listed as Active")
        admin.goto(args.base + "/users?q=admin-made&state=inactive", wait_until="domcontentloaded")
        check(row(admin, "admin-made%40fresh").count() == 1, "a deactivated user is absent from the Inactive view")
        admin.goto(args.base + "/users?q=alice", wait_until="domcontentloaded")
        check(row(admin, "alice%40fresh").locator('td[data-label="Authority"]').inner_text().strip() == "Daemon administrator", "the Administrator's Users row does not say so")
        admin.goto(args.base + "/group?name=%40browser-team", wait_until="domcontentloaded")
        admin.locator("#members-edit").click()
        admin.wait_for_load_state("domcontentloaded")
        admin.locator('#form-save textarea[name="members"]').fill("dave@fresh\ncarol@fresh")
        submit(admin, "Save group")
        check(admin.locator(".member-list code", has_text="carol@fresh").count() == 1, "Administrator did not edit another User's ordinary group")
        admin.goto(args.base + "/group?name=%40administrators", wait_until="domcontentloaded")
        check(admin.locator("h1", has_text="@administrators").count() == 1, "protected group page is absent")
        check(admin.locator("#members-edit").count() == 0, "Administrator can edit the protected group")
        denied = direct_group_save(admin, "@administrators", "owner@fresh\nalice@fresh\ndave@fresh")
        check(denied["status"] == 403 and "Not yours to see" in denied["text"], "Administrator protected-group denial was not rendered")
        admin.goto(args.base + "/user?name=owner%40fresh", wait_until="domcontentloaded")
        check(admin.locator("h1", has_text="owner@fresh").count() == 1, "daemon Owner's page is absent")
        check(admin.locator("#profile-edit").count() == 0, "Administrator can edit the daemon Owner")
        response = admin.goto(args.base + "/user/edit?name=owner%40fresh", wait_until="domcontentloaded")
        check(response.status == 403, "Administrator reached the daemon Owner's profile editor")
        admin.goto(args.base + "/user?name=owner%40fresh", wait_until="domcontentloaded")
        check(admin.locator('form[action="/user-deactivate"]').count() == 0, "Administrator is offered the daemon Owner's deactivation")
        denied = direct_post(admin, "/user", {"name": "owner@fresh", "action": "inactive", "return": "/users"})
        check(refused(denied), f"Administrator deactivating the daemon Owner was not refused: {denied['status']}")
        admin.goto(args.base + "/users?q=owner%40fresh", wait_until="domcontentloaded")
        check(row(admin, "owner%40fresh").count() == 1, "the daemon Owner left the Active view after a refused deactivation")
        admin.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(admin.locator("h1", has_text="browser-svc@fresh").count() == 1, "unrelated service page is absent")
        check(admin.locator("#settings").count() == 0, "Administrator became an unrelated service Maintainer")
        denied = direct_service_save(admin, "browser-svc@fresh", "Administrator bypass")
        check(denied["status"] == 403 and "Not yours to see" in denied["text"], "Administrator service denial was not rendered")
        # The same three creations succeed for this Administrator from the
        # dashboard's own origin (user and group above, a queue here), so
        # each foreign refusal is about origin alone.
        admin.goto(args.base + "/queues/new", wait_until="domcontentloaded")
        admin.get_by_role("textbox", name=NAME_FIELD).fill("admin-queue@fresh")
        submit(admin, "Register queue")
        check("admin-queue@fresh" in admin.locator("h1").inner_text(), "the Administrator's same-origin queue registration failed")
        foreign_server, foreign_url = start_foreign_origin(args.base)
        try:
            for route, created in (("/group", "/group?name=%40foreign-origin"),
                                   ("/service", "/queue?name=foreign-queue%40fresh"),
                                   ("/user", "/user?name=foreign-user%40fresh")):
                admin.goto(foreign_url + route, wait_until="domcontentloaded")
                with admin.expect_navigation() as navigation:
                    admin.get_by_role("button", name="submit foreign form").click()
                response = navigation.value
                check(response.status == 403, f"foreign-origin {route} form was not refused")
                check("same-origin form required" in admin.locator("body").inner_text(), f"foreign-origin {route} refusal was not attributable")
                response = admin.goto(args.base + created, wait_until="domcontentloaded")
                check(response.status == 404, f"the refused foreign-origin {route} form still created {created}")
        finally:
            foreign_server.shutdown()
            foreign_server.server_close()
        admin_context.close()
        results["administrator"] = ["create user", "edit another User's ordinary group", "protected authority denied", "unrelated service denied", "users listing, deactivate a user", "daemon Owner deactivation denied", "foreign-origin group, record and user forms denied and not created"]

        owner_context, owner = sign_in(browser, args.base, tokens["owner"])
        check(owner.locator("form.who code").inner_text() == "owner@fresh", "daemon Owner identity is wrong")
        owner.goto(args.base + "/users/new", wait_until="domcontentloaded")
        owner.get_by_label("Identity").fill("owner-made@fresh")
        owner.get_by_label("Person name").fill("Owner Made")
        submit(owner, "Save profile")
        check("owner-made@fresh" in owner.locator("h1").inner_text(), "daemon Owner did not create a user")
        owner.goto(args.base + "/group?name=%40administrators", wait_until="domcontentloaded")
        check(owner.locator("#members-edit").count() == 1, "daemon Owner cannot edit the protected group")
        owner.locator("#members-edit").click()
        owner.wait_for_load_state("domcontentloaded")
        owner.locator('#form-save textarea[name="members"]').fill("owner@fresh\nalice@fresh\nowner-made@fresh")
        submit(owner, "Save group")
        check(owner.locator(".member-list code", has_text="owner-made@fresh").count() == 1, "daemon Owner did not change the protected group")
        # The direct save the Administrator was refused succeeds for the
        # daemon Owner, so that refusal was about authority, not the request.
        own = direct_group_save(owner, "@administrators", "owner@fresh\nalice@fresh\nowner-made@fresh")
        check(own["status"] == 200 and "Not yours to see" not in own["text"], "the direct group save is refused even for the daemon Owner")
        owner.goto(args.base + "/group/edit?name=%40browser-team", wait_until="domcontentloaded")
        owner.locator('#form-save textarea[name="members"]').fill("dave@fresh\ncarol@fresh\nbob@fresh")
        submit(owner, "Save group")
        check(owner.locator(".member-list code", has_text="bob@fresh").count() == 1, "daemon Owner did not edit an ordinary group")
        owner.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(owner.locator("#settings").count() == 1, "daemon Owner lacks root service management")
        # Every refusal above left the records as their managers last saved them.
        owner.goto(args.base + "/service/edit?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(owner.locator('#form-save input[name="descr"]').input_value() == "Maintainer updated", "a refused save changed the service description")
        owner.goto(args.base + "/queues?q=browser-queue&state=active", wait_until="domcontentloaded")
        check(row(owner, "browser-queue%40fresh").count() == 1, "a refused deactivation made the queue inactive")
        owner.goto(args.base + "/queue/edit?name=browser-queue%40fresh", wait_until="domcontentloaded")
        check(owner.locator('#form-save input[name="descr"]').input_value() == "Browser matrix queue", "a refused save changed the queue description")
        owner.goto(args.base + "/group?name=%40browser-team", wait_until="domcontentloaded")
        check(owner.locator(".member-list code", has_text="eve@fresh").count() == 0, "a refused group save added the stranger")
        owner.goto(args.base + "/agent?name=%23fresh-echo%40fresh", wait_until="domcontentloaded")
        check(owner.locator("h1", has_text="#fresh-echo@fresh").count() == 1, "the installed script agent's page is absent")
        until = time.monotonic() + 70
        while owner.locator("svg.activity-chart").count() == 0 and time.monotonic() < until:
            owner.wait_for_timeout(1000)
            owner.reload(wait_until="domcontentloaded")
        check(owner.locator("svg.activity-chart").count() == 1, "real installed agent activity never produced a graph")
        owner.get_by_role("link", name="View all activity and slot values").click()
        owner.wait_for_load_state("domcontentloaded")
        check(owner.locator("summary", has_text="Slot values").count() == 1, "full activity values are absent")
        label = owner.locator("svg.activity-chart").first.get_attribute("aria-label") or ""
        peak = re.search(r"shared maximum (\d+)", label)
        check(peak is not None and int(peak.group(1)) >= 1, f"the activity chart does not report the measured call: {label!r}")
        owner.screenshot(path=args.evidence / "browser-owner-matrix.png", full_page=True)
        owner_context.close()
        results["daemon_owner"] = ["create user", "edit protected and ordinary groups", "manage another owner's service", "every refused change left its record unchanged", "real agent activity graph"]

        (args.evidence / "browser-roles.json").write_text(json.dumps(results, indent=2) + "\n")
        browser.close()

    print("PASS installed Chromium five-role matrix: services, queues, pub/sub, users and groups, stranger and foreign-origin refusals, activity graph")


if __name__ == "__main__":
    main()
