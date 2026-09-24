# TODO Web face rewrite

📌 **TL;DR:** Eleven steps from an empty `src/web/` to the Go face removed.
Each step ends with a commit, a `0.8.x` PATCH bump and a falsifiable check that was
first seen to fail ([mutation first](../../CLAUDE.md#mutation-first-then-belief)).
Design and rules are in the [README](README.md#web-face-rewrite).

## Objective

Every address in the [site map](../../docs/web-face/site-map.md#every-address)
served by the TypeScript face under its own account, to the spec, in the new
design; the Go face gone.

## Next step

W.1. Every choice before it is [settled](DECISIONS.md#decisions).

## Steps

| ID | Step | Acceptance |
|---|---|---|
| W.1 | **Skeleton.** `src/web/` package, `Bun.serve`, the JSX runtime, the daemon client, security headers, origin checks, body cap, `/healthz`, `--version` | unescaped text in a view fails a test; a foreign or missing `Origin` on a POST gets `403 same-origin form required`; a transport error's text never reaches a page; `--version` prints the build stamp |
| W.2 | **Design system.** Tokens, both themes, the stylesheet, fonts, every component in `src/web/ui/`, and a `/_styleguide` page served only with `AGENT_BUS_WEB_DEV=1` | the style guide renders every component in both themes at 1440, 768 and 390 px with no sideways scroll; axe finds no violation; an external tag without `integrity` fails a test; with the CDN blocked every page still works; the owner approves the look before W.4 |
| W.3 | **Shell and session.** Frame, sidebar, header, footer, landing (signed-out homepage), `POST /signin`, `POST /signout`, the problem pages, the signed-out answer, `return` rules | a signed-out request to any page shows sign-in with the right status and `return`; `return=//evil` lands on `/`; the cookie never holds the token and has no `Max-Age`; every row of [problem page](../../docs/web-face/shell.md#problem-page) is produced by a daemon answer in a contract test |
| W.4 | **Overview** (signed-in homepage) | every attention item condition from [the spec](../../docs/web-face/node.md#attention-items) appears and sorts as stated; an ordinary user never sees the owner-inactive item; zero reads as `—` in the strip |
| W.5 | **Activity and Diagnostics** | 144 slots with hour ticks; a hidden name is `404`; exchange folding passes a table of cases from [Diagnostics](../../docs/web-face/node.md#diagnostics), `reply_to` included; refusals list every reason at a measured `0` |
| W.6 | **Record lists and register** | filters, sort, tabs and paging per [lists](../../docs/web-face/records.md#lists), `kind` kept on page 2; a register refusal returns the form with every allowlisted value and never the secret; `412` marks `name` |
| W.7 | **Record detail, settings, deactivate, Danger Zone, `POST /service`, `POST /service-confirm`** | controls render from `can_manage` / `can_transfer` only (a flipped flag in a fixture removes them); a stale confirmation answers `409 The conditions changed`; a transfer or delete without the confirmation step is refused; a `Line N:` refusal marks its field |
| W.8 | **People.** Users, user, register and edit user, `POST /user`, deactivation and credential removal, Groups, group, register and edit group, `POST /groups`, group Danger Zone, Account | a non-Administrator sees only their own row; `@administrators` is editable only by the daemon Owner; a Personal group not named `@<you>/…` is refused before any call; no Transfer on a prefixed group |
| W.9 | **Enhancements.** Palette, shortcuts, theme toggle, copy buttons, flash toasts, view transitions | with JavaScript off every page and form still works (browser check); the palette lists only names the visitor's `/ls` answered; reduced motion disables transitions |
| W.10 | **Package and confine.** `bun build --compile` in `src/build.sh` and `src/release.sh`; setup creates the account and unit | the unit runs as `agent-bus-web` with no capabilities; it cannot read `/var/lib/agent-bus` or a mapped socket; memory and task limits hold under the pressure checks the [installed resource evidence](../MVP/done/web-resources.md#checks) used; installed in a container, never on the host |
| W.11 | **Cutover.** Side-by-side run, owner acceptance page by page, then the Go face removed and docs promoted ([cutover](README.md#cutover)) | every site-map row has a passing check against the TS face; `smoke.sh --slow` green with the Go web checks removed; no reference to `agent-bus-web` Go code remains |

## Verification plan

| Layer | What | Where |
|---|---|---|
| Unit | views, escaping, pure face rules, form retention | `bun test` in `src/web/` |
| Contract | each page against a disposable `agent-busd -create` with fixture users, agents, queues, topics, groups and every role | a new `web-ts` shard in `src/smoke.sh --slow`, its own `PORT` base |
| Browser | keyboard paths, JavaScript off, three widths, both themes, axe, screenshots for the owner | Playwright in the container, as the installed-browser gates run |
| Review | OpenCode review of each step; a Fable review before cutover | over the bus |
