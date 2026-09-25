# Installed browser acceptance

📌 **TL;DR:** F.12 is done on 0.8.26. Real Chromium on a package-only
real-systemd host drives the redesigned 0.8 web as five roles plus a stranger,
through services, queues, pub/sub topics, users and groups. It checks denials,
activity, the cookie, and separate web-child and bus-child restarts. Breaking
form-origin validation or authorization each fails a named check.

## Scope

This reruns and extends the [session foundation](installed-browser-foundation.md#checks)
and the [authority matrix](installed-browser-role-matrix.md#checks) against the
0.8 addresses in the internal site map (`docs/web-face/site-map.md`). The
old drivers expected a Channels page; 0.8.4 split it into Queues (`/queues`) and
PubSub (`/pubsub`). The drivers are `src/acceptance/installed-browser.py` and
`installed-browser-roles.py`, run by `fresh-install-container.sh`. Every
credited action or refusal goes through Chromium. Fixture setup uses installed
administration and CLI programs only. Page sizes, contrast and keyboard access
are [F.13.6](web-acceptance.md#checks)'s, not repeated here.

## Checks

```
src/package.sh "$PWD/tmp/f12/dist"
src/acceptance/fresh-install.sh tmp/f12/dist/agent-bus-0.8.26-linux-x86_64.tar.gz tmp/f12/final
```

**Result:** exit 0, all 19 PASS lines, the whole fresh-install gate included.
The two browser steps were the stale part of that gate
([setup hardening](setup-hardening.md#checks)).

**Host:** Arch Linux image `localhost/agent-bus-fresh-install:arch-systemd`,
systemd 261.3, Chromium 153.0.8010.36, a 375 × 820 viewport, `--network=none`.
Only the archive, the fixture and the evidence directory are mounted. Evidence
is `browser.json`, `browser-roles.json` and four screenshots under
`tmp/f12/final/evidence/`. Tokens stay under `/root` in the container.

| Browser identity | Measured |
|---|---|
| Session (`installed-browser.py`) | Sign-in through the visible form. One `HttpOnly`, `SameSite=Strict` cookie that is not the token; the token is in neither URL nor page. Eleven 0.8 pages answer 200 with their own title; the header links each tab; `/channels` lands on `/queues`. Killing the confined web renderer keeps the session. Killing the bus child ends it. Sign-in afterwards gives a new cookie. A signed-out cookie replayed in a fresh context is anonymous |
| Resource Owner `bob` | Registers a 📡 service, edits it and assigns Maintainer `carol`. Registers a 📮 queue from the Queues tab, deactivates it (Status filter moves the row), and reactivates it. Registers a 📣 topic that delivers to the queue, and edits the topic's settings |
| Maintainer `carol` | Edits the service. The Maintainers and Personal fields are disabled for her, and she is not offered transfer |
| Ordinary user `dave` | Reads the service, queue and topic that admit him, with no Settings. Direct saves to the service and the queue are refused. Registers and owns `@browser-team`. User registration and the protected group are refused. As a positive control, he sees the rows the stranger does not |
| Stranger `eve` | The service, queue, topic and activity pages each answer exactly as the same page does for a name nobody holds (404, "No such name"). No listing row. Saving the service, deactivating the queue and saving the group are refused; so are the group editor and user registration. Signed out, four pages show sign-in, and an anonymous post changes nothing |
| Administrator `alice` | Creates a user and finds its Users row (Authority "User"), deactivates it, and finds it under Inactive. Edits another User's ordinary group. Cannot edit or save `@administrators`, cannot edit or deactivate the daemon Owner, and cannot manage the unrelated service. Registers a queue from the dashboard's own origin. Three real forms from a second loopback origin, for group, record and user, are each refused 403 and none of them created anything |
| Daemon Owner | Creates a user, saves `@administrators` and the ordinary group, and manages bob's service. Every refused change above left its record unchanged: the service and queue descriptions, the queue still active, `eve` not a member. The installed `#fresh-echo` agent's real call draws its activity chart, with a nonzero maximum and slot values |

## Mutations

`tmp/scripts/mutant.py` copies the tree and applies one overlay. It runs
`go vet`, builds the package and runs the whole fresh-install gate on a new
host. Each mutant compiled, reached the browser step and failed on an
assertion. **3/3 caught.**

| Change | Named failure |
|---|---|
| Form-origin validation broken: `sameOrigin` accepts every origin | `foreign-origin /group form was not refused` |
| Management authorization broken: `core` `manages` is true for every caller | `ordinary caller did not get the read-only service` |
| Use/visibility authorization broken: `core` `may` is true for every caller | `stranger was not answered unknown at /service?name=browser-svc: 200` |

The logs are `tmp/f12/mut/<name>/gate.log`.

A read-only OpenCode review raised five points. Four are fixed: each
stranger page is now compared with its own unregistered-name page; the
stranger's 403s must render "Not yours to see"; the anonymous post must be
answered with sign-in; the recorded browser version comes from the browser
that ran. The fifth was refuted. It said the refused foreign queue would be
invisible to the Administrator anyway, but a record's creator owns it and
sees it: a queue alice registers with only `dave@fresh` allowed answers her
200. The gate and all three mutants were rerun after the fixes.

## Repository verification

`PORT=37000 src/smoke.sh --slow` passed **784/0**. The final gate ran on
these driver SHA-256 values:

| File | SHA-256 |
|---|---|
| `installed-browser.py` | `fa5254759c22e61f705d63a988ec82a770940905d9d2c42226610510ba51eb4b` |
| `installed-browser-roles.py` | `dbdfc8953ac8c6b573e267210d70e821dc29ae440d84efc2e0c543003efc5dd5` |
| `fresh-install-container.sh` | `a9f22fccf95d525b87e6e880c98a1af2ae59a59f43ee079691d0d37af1702cd8` |

## Not claimed

- HTTPS: setup generates plain loopback HTTP, so `Secure` is asserted false.
- Browser engines other than Chromium.
