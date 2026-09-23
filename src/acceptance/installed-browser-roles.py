#!/usr/bin/env python3
"""Installed Chromium authority/action matrix for F.12."""

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


def start_foreign_origin(target: str):
    class ForeignForm(BaseHTTPRequestHandler):
        def do_GET(self):
            body = f"""<!doctype html><form method=post action="{html.escape(target)}">
              <input type=hidden name=action value=save>
              <input type=hidden name=name value=@foreign-origin>
              <input type=hidden name=members value=dave@fresh>
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
    return server, f"http://127.0.0.1:{server.server_port}/"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", required=True)
    parser.add_argument("--owner-token", type=Path, required=True)
    parser.add_argument("--administrator-token", type=Path, required=True)
    parser.add_argument("--resource-owner-token", type=Path, required=True)
    parser.add_argument("--maintainer-token", type=Path, required=True)
    parser.add_argument("--ordinary-token", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    tokens = {role: read_token(path) for role, path in {
        "owner": args.owner_token,
        "administrator": args.administrator_token,
        "resource_owner": args.resource_owner_token,
        "maintainer": args.maintainer_token,
        "ordinary": args.ordinary_token,
    }.items()}
    results = {}

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True, executable_path="/usr/bin/chromium", args=["--no-sandbox"])

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
        bob.goto(args.base + "/channels/new", wait_until="domcontentloaded")
        bob.get_by_role("link", name="Register a queue").click()
        bob.wait_for_load_state("domcontentloaded")
        bob.get_by_role("textbox", name=NAME_FIELD).fill("browser-topic@fresh")
        bob.get_by_label("Description").fill("Browser matrix channel")
        bob.locator('#form-create textarea[name="allow"]').fill("dave@fresh")
        submit(bob, "Register queue")
        check("one at a time" in bob.locator(".detail-meta").inner_text(), "resource Owner did not register the queue channel")
        # Status is active|inactive: Deactivate… confirms; the record's one
        # view is then inactive and unlisted, and Reactivate restores both.
        submit(bob, "Deactivate…")
        submit(bob, "Deactivate")
        check(bob.locator("h1 .state-inactive").count() == 1, "deactivated queue does not read as inactive")
        bob.goto(args.base + "/channels?q=browser-topic&state=active", wait_until="domcontentloaded")
        check(bob.locator('table a[href*="browser-topic"]').count() == 0, "inactive queue is listed as active")
        bob.goto(args.base + "/channels?q=browser-topic&state=inactive", wait_until="domcontentloaded")
        check(bob.locator('tr:has(a[href*="browser-topic"]) .status-glyph[aria-label="Inactive"]').count() == 1, "inactive queue is absent from the Inactive view")
        bob.goto(args.base + "/channel?name=browser-topic%40fresh", wait_until="domcontentloaded")
        submit(bob, "Reactivate")
        check(bob.locator('.status-glyph[aria-label="Active"]').count() == 1, "reactivated queue is not active")
        bob.goto(args.base + "/channels?q=browser-topic&state=active", wait_until="domcontentloaded")
        check(bob.locator('tr:has(a[href*="browser-topic"]) .status-glyph[aria-label="Active"]').count() == 1, "reactivated queue is not listed as active")
        bob.screenshot(path=args.evidence / "browser-resource-owner.png", full_page=True)
        bob_context.close()
        results["resource_owner"] = ["register external service", "edit settings", "assign maintainer after registration", "register queue channel", "deactivate and reactivate"]

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
        results["ordinary"] = ["read-only visible service", "service mutation denied", "user administration denied", "register and own a group", "protected group denied"]

        admin_context, admin = sign_in(browser, args.base, tokens["administrator"])
        check(admin.locator("form.who code").inner_text() == "alice@fresh", "Administrator identity is wrong")
        admin.goto(args.base + "/users/new", wait_until="domcontentloaded")
        admin.get_by_label("Identity").fill("admin-made@fresh")
        admin.get_by_label("Person name").fill("Admin Made")
        submit(admin, "Save profile")
        check("admin-made@fresh" in admin.locator("h1").inner_text(), "Administrator did not create an ordinary user")
        check(admin.locator("#profile-edit").count() == 1, "Administrator cannot edit the user it created")
        admin.goto(args.base + "/group?name=%40browser-team", wait_until="domcontentloaded")
        admin.locator("#members-edit").click()
        admin.wait_for_load_state("domcontentloaded")
        admin.locator('#form-save textarea[name="members"]').fill("dave@fresh\ncarol@fresh")
        submit(admin, "Save members")
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
        admin.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(admin.locator("h1", has_text="browser-svc@fresh").count() == 1, "unrelated service page is absent")
        check(admin.locator("#settings").count() == 0, "Administrator became an unrelated service Maintainer")
        denied = direct_service_save(admin, "browser-svc@fresh", "Administrator bypass")
        check(denied["status"] == 403 and "Not yours to see" in denied["text"], "Administrator service denial was not rendered")
        foreign_server, foreign_url = start_foreign_origin(args.base + "/groups")
        try:
            admin.goto(foreign_url, wait_until="domcontentloaded")
            with admin.expect_navigation() as navigation:
                admin.get_by_role("button", name="submit foreign form").click()
            response = navigation.value
            check(response.status == 403, "foreign-origin form was not refused")
            check("same-origin form required" in admin.locator("body").inner_text(), "foreign-origin refusal was not attributable")
        finally:
            foreign_server.shutdown()
            foreign_server.server_close()
        admin_context.close()
        results["administrator"] = ["create user", "edit another User's ordinary group", "protected authority denied", "unrelated service denied", "foreign origin denied"]

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
        submit(owner, "Save members")
        check(owner.locator(".member-list code", has_text="owner-made@fresh").count() == 1, "daemon Owner did not change the protected group")
        # The direct save the Administrator was refused succeeds for the
        # daemon Owner, so that refusal was about authority, not the request.
        own = direct_group_save(owner, "@administrators", "owner@fresh\nalice@fresh\nowner-made@fresh")
        check(own["status"] == 200 and "Not yours to see" not in own["text"], "the direct group save is refused even for the daemon Owner")
        owner.goto(args.base + "/group/edit?name=%40browser-team", wait_until="domcontentloaded")
        owner.locator('#form-save textarea[name="members"]').fill("dave@fresh\ncarol@fresh\nbob@fresh")
        submit(owner, "Save members")
        check(owner.locator(".member-list code", has_text="bob@fresh").count() == 1, "daemon Owner did not edit an ordinary group")
        owner.goto(args.base + "/service?name=browser-svc%40fresh", wait_until="domcontentloaded")
        check(owner.locator("#settings").count() == 1, "daemon Owner lacks root service management")
        owner.goto(args.base + "/agent?name=%23fresh-echo%40fresh", wait_until="domcontentloaded")
        check(owner.locator("h1", has_text="#fresh-echo@fresh").count() == 1, "the installed script agent's page is absent")
        until = time.monotonic() + 70
        while owner.locator("svg.activity-chart").count() == 0 and time.monotonic() < until:
            owner.wait_for_timeout(1000)
            owner.reload(wait_until="domcontentloaded")
        check(owner.locator("svg.activity-chart").count() == 1, "real installed agent activity never produced a graph")
        owner.get_by_role("link", name="View all activity and sample values").click()
        check(owner.locator("summary", has_text="Sample values").count() == 1, "full activity values are absent")
        owner.screenshot(path=args.evidence / "browser-owner-matrix.png", full_page=True)
        owner_context.close()
        results["daemon_owner"] = ["create user", "edit protected and ordinary groups", "manage another owner's service", "real agent activity graph"]

        (args.evidence / "browser-roles.json").write_text(json.dumps(results, indent=2) + "\n")
        browser.close()

    print("PASS installed Chromium authority/action matrix, origin refusal and activity graph")


if __name__ == "__main__":
    main()
