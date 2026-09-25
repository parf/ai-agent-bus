# TODO Web face rewrite

📌 **TL;DR:** Eleven steps from an empty `src/web/` to the Go face removed.
Each step ends with a commit, a `0.8.x` PATCH bump with its `CHANGELOG.0.8.md` line, spec edits for what it changed, screenshots for the owner at three widths in both themes (W.3 on) and a falsifiable check that was
first seen to fail ([mutation first](../../../CLAUDE.md#mutation-first-then-belief)).
Design and rules are in the [README](README.md#web-face-rewrite).

## Objective

Every address in the [site map](../../../docs/web-face/site-map.md#every-address)
served by the TypeScript face under its own account, to the spec, in the new
design; the Go face gone.

## Next step

None: W.1–W.11 are built ([DONE](DONE.md#done--web)). The installed fresh-install and upgrade container gates passed against the TypeScript face on 0.8.51, sample data included, and the installed real-browser gate (W.12) on 0.8.53.

## Steps

| ID | Step | Acceptance |
|---|---|---|
| W.1 | **Skeleton.** `src/web/` package, `Bun.serve`, the JSX runtime, the daemon client, security headers, origin checks, body cap, `/healthz`, `--version` | unescaped text in a view fails a test; a foreign or missing `Origin` on a POST gets `403 same-origin form required`; a transport error's text never reaches a page; `--version` prints the version; no file is written anywhere (transpiler cache off); a body over 1 MiB gets `413`; `HEAD` answers as `GET` with no body; `/favicon.ico` is `404`; hashed assets carry `immutable` and pages `no-store`; the face serves `/healthz` from the checkout under [the unit](README.md#the-unit) in the container (directives that break it are recorded there) |
| W.2 | **Design system.** Tokens, both themes, the stylesheet, fonts, every component in `src/web/ui/`, the landing and shell drawn in it, and a `/_styleguide` page served only with `AGENT_BUS_WEB_DEV=1` | the style guide renders every component in both themes at 1440, 768 and 390 px with no sideways scroll; axe finds no violation; an external tag without `integrity`, a CSP source that is not an exact URL, a rendered `style=` attribute and a TS glyph that differs from the Go `display` dump each fail a test; `/_styleguide` is `404` without the flag; every pinned URL answers `200`; the owner approves the look, landing and shell included, before W.3 |
| W.3 | **Shell and session.** Frame, sidebar and its collapse, header, footer, theme toggle, view transitions, landing (signed-out homepage), `POST /signin`, `POST /signout`, the problem pages, the signed-out answer, `return` rules | a signed-out request to any page shows sign-in with the right status and `return`; `return=//evil` lands on `/`; `recordReturn` and `directoryReturn` keep only their own listing paths; the cookie never holds the token and has no `Max-Age`; every row of [problem page](../../../docs/web-face/shell.md#problem-page) is produced by a daemon answer in a contract test |
| W.4 | **Overview** (signed-in homepage) | every attention item condition from [the spec](../../../docs/web-face/node.md#attention-items) appears and sorts as stated; an ordinary user never sees the owner-inactive item; zero reads as `—` in the strip |
| W.5 | **Activity and Diagnostics** | 144 slots with hour ticks; a hidden name is `404`; exchange folding passes a table of cases from [Diagnostics](../../../docs/web-face/node.md#diagnostics), `reply_to` included; refusals list every reason at a measured `0` |
| W.6 | **Record lists and register** | filters, sort, tabs and paging per [lists](../../../docs/web-face/records.md#lists), `kind` kept on page 2; `/channels`, `/channels/new`, `/channel` and `/channel/edit` answer their `301`s; a register refusal returns the form with every allowlisted value and never the secret; `412` marks `name` |
| W.7 | **Record detail, settings, deactivate, Danger Zone, `POST /service`, `POST /service-confirm`** | controls render from `can_manage` / `can_transfer` only (a flipped flag in a fixture removes them); a stale confirmation answers `409 The conditions changed`; a transfer or delete without the confirmation step is refused; a `Line N:` refusal marks its field |
| W.8 | **People.** Users, user, register and edit user, `POST /user`, deactivation and credential removal, Groups, group, register and edit group, `POST /groups`, group Danger Zone, Account | a non-Administrator sees only their own row; directory paging clamps and keeps `q` and `state`; `@administrators` is editable only by the daemon Owner; a Personal group not named `@<you>/…` is refused before any call; no Transfer on a prefixed group |
| W.9 | **Enhancements.** Palette, shortcuts, copy buttons, flash toasts | the palette lists only names the visitor's `/ls` answered; `/palette.json` signed out answers `401` JSON and a cross-site fetch is refused; a flash toast does not return on back navigation; reduced motion disables transitions; no `securitypolicyviolation` on any page; new addresses (`/palette.json`, hashed assets, `/_styleguide`) have site-map rows |
| W.10 | **Confine.** `src/web/install-dev.sh` creates the account, the `/var/lib/agent-bus/web` link to the checkout and [the unit](README.md#the-unit) on `6781` | `systemd-analyze security` scores 1.5 or lower; a `bun test` reads `agent-bus-web.service` and fails when `User=`, the empty `CapabilityBoundingSet`, the Environment allowlist or `IPAddressAllow=localhost` changes; the process runs as `agent-bus-web` with no capabilities; it cannot write its own code, see `/var/lib/agent-bus/daemon`, open a mapped socket, exec anything but bun (`agent-busd` tried by path is refused), or connect out; memory and task limits hold under the pressure checks the [installed resource evidence](../done/web-resources.md#checks) used; each check broken once by removing its directive; installed in a container, never on the host |
| W.11 | **Cutover.** Side-by-side run, owner acceptance page by page, then the Go face removed and docs promoted ([cutover](README.md#cutover)) | every site-map row has a passing check against the TS face; `smoke.sh --slow` green with the Go web checks removed; removed with the Go face: `src/cmd/agent-bus-web/`, the supervisor's `-web`, bubblewrap and cgroup code, setup's `Delegate=` / `DelegateSubgroup=` and `TestUnitDelegatesOnlyTheWebControllers`, the Go web acceptance scripts (`web-acceptance.py`, `web-isolation.py`, `web-resources.py`, `installed-browser.py`, `installed-browser-roles.py`, `web-probe`, `resource-pressure`) or their port to the TS face, and the bubblewrap line in `fresh-install.Containerfile`; docs updated: [setup § install](../../../docs/09-setup.md#install) (bwrap requirement), [external tools](../../../docs/10-modules.md#external-tools) (bwrap row), [browser acceptance](../../../docs/05-discovery.md#browser-acceptance); `grep` finds no reference to the Go face |

## Verification plan

| Layer | What | Where |
|---|---|---|
| Unit | views, escaping, pure face rules, form retention | `bun test` in `src/web/` |
| Contract | each page against a disposable `agent-busd -create` with fixture users, agents, queues, topics, groups and every role | a `web_ts` shard added to `SHARDS` in `src/smoke.sh --slow`; its fifteen ports follow from its index |
| Browser | keyboard paths, three widths, both themes, axe, screenshots for the owner | the container's Python Playwright and Chromium, as the installed-browser gates run; axe-core loaded into the page from the pinned CDN; test tooling, not product code |
| Review | OpenCode review of each step; a Fable review before cutover | over the bus |
