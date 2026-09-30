# Changelog 0.8

📌 **TL;DR:** Changes on the 0.8 feature line, newest first, one or two lines
per version. 0.8 is an even line: it starts from the stable 0.7.20. The
previous line is [changelog 0.7](Plans/CHANGELOG.0.7.md#changelog-07).

## 0.8.81 — 2026-09-30

Unregister deletes every lock on the removed record; unregister and deactivation
refuse pending takes, with authority checked again before a waiting lock is granted.

## 0.8.80 — 2026-09-30

A record stores its owner's `user_id` beside the name (schema 8), and one whose
owner vanished and was recreated under the same name is ignored at start
rather than handed to the new User (K.24).

## 0.8.79 — 2026-09-30

A record registered under a name an ignored record held inherits nothing: the
allow lists, Maintainers and Group memberships that named the old holder drop
it in the same commit (K.23).

## 0.8.78 — 2026-09-30

Add user and a User's page take a pasted `ssh-ed25519` key from whoever may
edit that User; the daemon writes the same forced `agent-bus-token` line
`agent-bus-admin user add` does, under a lock both share (`-ssh-keys`).

## 0.8.77 — 2026-09-30

Every record keeps a key-value store of string, int and JSON values, used by
its Owner, Maintainers and own Agent: `agent-bus kv`, `/kv*` and `ab_kv_*`,
with atomic set modes, `inc` and nine JSON operations, each write committed
before it is answered (schema 7).

## 0.8.76 — 2026-09-30

An Agent's `script` field says what `agent-bus start` serves it with: the
runner writes it there, not in `addr`, and `ls`, the MCP catalogue and the
Agent's web page show it (Q124).

## 0.8.75 — 2026-09-30

A Locks page in the web face lists every lock on the records you may use,
with a kind filter and search; `GET /holders` with no record answers that listing.

## 0.8.74 — 2026-09-30

Locks belong to a record: its Owner, Maintainers and own Agent use them, and
the API names it `record`; a Group's members no longer use its locks by
membership. Private values follow the same rule: the Owner and Maintainers read
and write a record's configuration and secret, and an Agent reads its own; the
allow list, a Group's membership and the daemon Owner's office no longer grant
them. A deactivation ends only the locks of what it made inactive. A Resource
read through an MCP Service needs its credential, so it is refused to those who
may not read it. Setup writes sample secrets as each record's Owner.

## 0.8.73 — 2026-09-30

Personal sits beside Name and Template beside URI, with file, schema and log
examples and an RFC 6570 link. Resources use 📚 across web, CLI and docs.

## 0.8.72 — 2026-09-30

Registration and editing share Maintainers and Personal controls; initial
Maintainers are validated and stored at creation. Resource template help
explains URI formats and links the MCP specification. MCP descriptions and
receipt instructions live in YAML; all lock tools include `lock` in their names.

## 0.8.71 — 2026-09-30

The web face shows shared locks on a group's page (release, force release,
time left), Resources as a full kind (list, card detail, registration), and
Users and Groups move right after Overview in the left nav. The /holders
answer now carries each hold's expiry — a changed shape, `{holder, expires}`
per lock, for any script reading it — and `agent-bus holders` prints the time
left.

## 0.8.70 — 2026-09-30

📄 Resource records: a card for data an MCP client may read, one `resource` kind
with a template flag, listed and read through the MCP face by its source (an
Agent, an MCP Service, or an `https://` fetch); `register --uri`.

## 0.8.69 — 2026-09-30

A deactivation ends every lock at once: the API's user-state and manage
handlers reset the lock table on a successful inactive, replacing the
absence-stamp machinery.

## 0.8.68 — 2026-09-30

Locks re-audit fixes: waiters are woken by Holders' and Release's expiry
paths (tested by named mutants), the smoke timing check uses the has idiom,
the extend dispatch reaches main, and the test helper stops the lock sweep.

## 0.8.67 — 2026-09-30

Locks audit fixes: every expiry wakes its waiters, a cancelled wait is never
granted, `extend` (holder only), self-take refused at once, a group's latest
absence ends holds taken before it, `--force` is its own audited operation
naming the displaced holder, CLI defaults --wait to 30s, and non-member gates
and ttl clamps are checked.

## 0.8.66 — 2026-09-30

Shared locks: `lock`, `try-lock`, `release [--force]` and `holders`, each lock
in a Group that is its namespace and ACL, a ttl on every one, memory only.
CLI verbs, API routes and the `ab_lock`, `ab_release`, `ab_holders` MCP tools.

## 0.8.65 — 2026-09-29

A foreground `agent-bus start` survives daemon restarts and dropped links: it
waits with backoff, shows `; away`, and retries answers; only a refusal ends it.

## 0.8.64 — 2026-09-29

The `ab-*` launchers stay on the one personal socket too, and `start` refuses
any relative script path rather than failing it at run time with exit 127.

## 0.8.63 — 2026-09-29

A personal socket takes the token of an agent its account owns, so a runner
serves on that one socket — the only one forwarded to another host.

## 0.8.62 — 2026-09-29

The web face takes the node's TLS too: setup gives it its own copy of the
certificate, its port answers TLS and redirects plain HTTP to `https://`.

## 0.8.61 — 2026-09-29

Deactivation for active agents, services, queues, topics and users now starts
in the Danger Zone, then uses its existing confirmation.

## 0.8.60 — 2026-09-29

Optional TLS on the daemon's port, beside plain HTTP on the same port (a dual
listener): setup asks, generates a self-signed certificate or installs yours
with its chain, and clients pin its fingerprint (`token --fingerprint`).

## 0.8.56 — 2026-09-29

Every program's help opens with its name and the shared version: `agent-bus`,
`agent-bus --help`, and `--help` on `agent-busd`, `-admin`, `-setup`, `-token`.

## 0.8.55 — 2026-09-29

`agent-bus ls` and `ab_ls` list, by default, only the agents being read now;
`--kind`/`kind` lists one kind, `--all`/`all` everything.

## 0.8.54 — 2026-09-25

`agent-busd` binds any TCP address it is given, so agents on other hosts
connect directly; off loopback it is plain HTTP and says so at start.

## 0.8.53 — 2026-09-25

`agent-busd -addr` no longer falls back to the CLI's `AGENT_BUS_ADDR`; two web
texts say "database" and "kept across restarts"; a new installed real-browser
gate walks the TypeScript face with its CDN assets.

## 0.8.52 — 2026-09-24

A pub/sub topic takes no TTL, capacity or overflow policy, and each published
copy lives by its recipient's TTL; a stored topic's old settings are dropped at
load. Its web page no longer shows a queue Policy card.

## 0.8.51 — 2026-09-24

The release package carries the whole web face: every `web/` file is in the
manifest, checked and installed, so a packaged install starts it; `--upgrade`
from a pre-0.8.50 release works again, and a second `--samples` run succeeds.
The web face's ps line reads `agent-bus-web <version> ; Calls: <count>`, like
the Go programs.

## 0.8.50 — 2026-09-24

The web face is the TypeScript face only: `agent-bus-setup` installs it as its
own `agent-bus-web` account (home `/var/lib/agent-bus/web`) and locked-down
systemd unit on `127.0.0.1:6780`, linked to the current release's `web/`. The
Go dashboard, its supervised child, bubblewrap confinement and web cgroup are
removed; `agent-busd -web` is accepted and ignored so old units still start.
`agent-bus-setup --samples` / `--remove-samples` add and take away a Star
Wars and Spaceballs sample node: every kind, in realms. 0.8.47–0.8.49 are skipped.

## 0.8.46 — 2026-09-24

Activity names its dates in the title, in bold, and no longer talks about
ten-minute slots.

## 0.8.45 — 2026-09-24

The web face draws every kind with the same Unicode glyph the CLI prints
(👤 👾 📮 📣 📡 👥, 🔱 👮), in place of its own Lucide drawings.

## 0.8.44 — 2026-09-24

Version bump: the daemon, CLI and web face released together at one version,
with 0.8.42's durable activity days and 0.8.43's Personal filter.

## 0.8.43 — 2026-09-24

Personal is a filter on each kind's list (`/agents?personal=1`), kept by the
sidebar from kind to kind; lists drop columns that never differ; the current
sidebar section is a bright accent pill.

## 0.8.42 — 2026-09-24

Stored activity days are whole: a per-name ledger replaces the ring's window, so
midnight, a stop or a long gap never shortens a day. Totals come in one answer,
and `/status` states today and the retention.

## 0.8.41 — 2026-09-24

Activity is kept per calendar day for 400 days — one zstd row per name and
yymmdd date, non-empty slots only, written at each ten-minute boundary (schema
6) — and Activity and every record page show Day, Week and Month with Prev/Next.

## 0.8.40 — 2026-09-24

Every record list shows its Register action in the page head, empty or not.

## 0.8.39 — 2026-09-24

The favicon is a red bus filling a white square, big enough to read in a tab.

## 0.8.38 — 2026-09-24

The web face's favicon is a white bus on the brand red, readable at 16 px on
dark and light tabs.

## 0.8.37 — 2026-09-24

On Personal, the chosen kind drives the tabs, the sidebar and a single Register
action, which opens that kind's form already Personal.

## 0.8.36 — 2026-09-24

The Activity chooser lists only records with hits, as `name (hits)`.

## 0.8.35 — 2026-09-24

The web face's Activity chooser shows each record's hits in the last day.

## 0.8.34 — 2026-09-24

The web face takes Fable's review: only a hex session id is a session, `return`
values are checked everywhere, `412` marks the name, a user inbox has no Danger
Zone, and the unit may execute bun and its five libraries only.

## 0.8.33 — 2026-09-24

The TypeScript web face survives a malformed cookie, escapes the day ribbon's
labels, answers 405 on asset paths, and its unit sees nothing else under `/var/lib`.

## 0.8.32 — 2026-09-24

The TypeScript web face (`src/web`, Plans/R0.8-MVP/web) serves every page from the site
map on bun, with the new design; the Go face still runs beside it until cutover.

## 0.8.31 — 2026-09-23

`ab-claude` counts a channel Claude registered under the enclosing Git repository,
so a session in a subdirectory no longer warns that it has no channel.

## 0.8.30 — 2026-09-23

A launcher finds its account's installed socket by uid, as the CLI does, not
by `$USER`: without it Bun reported `unknown`, and a changed one named another account.
`ab-claude` reports a channel registration that `claude mcp add` claimed but never wrote.

## 0.8.29 — 2026-09-23

A Codex session's TUI gets the enforced mode too — no approval prompts, full
access — as its App Server always did.

## 0.8.28 — 2026-09-23

A launcher whose App Server or OpenCode server dies at startup now prints the
server's own last words, which were in a log its cleanup removed.

## 0.8.27 — 2026-09-23

`agent-bus-admin` writes the invoked `/usr/local/bin` path into a forced command,
which survives deploys, and refuses to swap another daemon's address for the
installed account; the installed browser gates refuse to run outside their container.

## 0.8.26 — 2026-09-23

Every web table has an accessible name, and the daemon's Users listing builds
one index instead of rescanning every record per User. F.13.6 browser gate.

## 0.8.25 — 2026-09-23

A face replaced within one watch is recovery, not a loss to reload; a zombie face
is dead; a reinstall refuses an unreadable current release; `release.sh` writes
the relative `current` link setup expects.

## 0.8.24 — 2026-09-23

A launcher notices its MCP face dying, keeps messages queued, restores it
through Codex or OpenCode (Claude: `/mcp` → Reconnect) and prints the exact
resume command when its runtime server dies; H.9.6 recovery accepted live.

## 0.8.23 — 2026-09-23

Setup over a running node restarts it for a changed unit or release, waits for
that release and build, and reports the durable Owner; `--reinstall` refuses a
daemon holding its home outside systemd and rolls a midway failure back.

## 0.8.22 — 2026-09-23

`/status` counts records inactive through their owner and the messages they
hold, for Administrators; the Overview shows one attention item from it.

## 0.8.21 — 2026-09-23

Restore ignores and reports a group nesting a missing group and an account
mapping for nobody, whose socket goes unserved; a duplicate-ID record keeps its
credential; the daemon account's socket follows an Owner transfer live.

## 0.8.20 — 2026-09-23

Web problem pages show the daemon's sentence rather than its JSON envelope and
name a suspension; the session cookie has no Max-Age of its own.

## 0.8.19 — 2026-09-23

`src/release.sh` deploys a commit, never the working tree, into its own release
directory with rollback; `build_info` names the commit it was built from.

## 0.8.18 — 2026-09-23

Push retries only a 403 that says the principal is suspended; any other 403
stops it as before, and a stop ends every wait at once.

## 0.8.17 — 2026-09-23

Push waits out a suspension, asking again every 30 minutes, and resumes after
reactivation instead of going off for the session's life.

## 0.8.16 — 2026-09-23

The project's links are on the landing page once: the footer copy of them is
gone, and with it the flag that put it there.

## 0.8.15 — 2026-09-23

The page a stranger reaches is a landing page: the picture, what agent-bus is,
the project's links, and the sign-in card at the bottom with an ⓘ that says how
to get a token.

## 0.8.14 — 2026-09-23

Activity review fixes: a read ticks the rings first, a damaged stored day is
refused whole and reported once, the node ring follows the bus clock, and `/activity` has its own wire type.

## 0.8.13 — 2026-09-23

The sign-in page's project picture is served by this node, as a downscaled
copy in the binary: the page's own `img-src 'self'` refused the repository-hosted
original, so it showed nothing at all.

## 0.8.12 — 2026-09-23

Activity is a day ring per record: 144 ten-minute slots of the node's local
clock, saved with the queues (store schema 5) and restored at start; down time reads zero; the web draws a fixed day.

## 0.8.11 — 2026-09-23

The call counter is read at every minute of the clock, so "Calls, minute" is
one minute and the 61 readings one hour, not ten minutes and ten hours.

## 0.8.10 — 2026-09-23

A section's Personal tab counts its own kind and opens `/personal?kind=…`;
old `/channel` addresses redirect to the record's section.

## 0.8.9 — 2026-09-23

Web page review: the section navigation wraps on a phone, an empty shared list
points to its Personal records, and Diagnostics, Account, Group and Overview layouts fit.

## 0.8.8 — 2026-09-23

The web Users page lists Users only, like the other list pages: one toolbar,
Contact, Agents and Last used columns, an empty-state card; leftover names move
to Diagnostics, shown only while one exists, and `/users?kind=other` redirects there.

## 0.8.7 — 2026-09-23

The signed-out page says what agent-bus is, shows the project's picture and
links the repository and its author; pages behind the gate are unchanged.

## 0.8.6 — 2026-09-23

A group may be named `@<owner>/<name>[@realm]`, reserved to that User; a
Personal group must be, and a prefixed group is never transferred.

## 0.8.5 — 2026-09-23

Every web settings form offers every field its kind may change: a group's
description, Personal, Maintainers, secret and Danger Zone; an agent's secret.

## 0.8.4 — 2026-09-23

The web Channels section is two: 📮 Queues and 📣 PubSub, each with its own list,
registration and settings; old `/channels` addresses redirect to the right one.

## 0.8.3 — 2026-09-23

`agent-bus ls -h` has a LAST USED column and a lookup carries `last_used` too;
groups are marked 👥 like every kind; usage lines show the realm as optional.
It also carries what shipped under a repeated 0.8.2: a listing carries each
name's `last_used` and puts the most recently used first; `ab_ls` says how long ago.

## 0.8.2 — 2026-09-23

Push stops when the daemon refuses the credential or the read, instead of
asking again every two seconds for as long as the session lives.

## 0.8.1 — 2026-09-23

`consume --topic` or `--tag` alone selects on that field only; a topic filter
no longer misses every tagged message on its topic.

## 0.8.0 — 2026-09-23

Opens the 0.8 line. A launcher prints a runtime adapter's "attached" status in
green, not in the red it keeps for failures.
