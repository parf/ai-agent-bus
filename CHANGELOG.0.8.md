# Changelog 0.8

📌 **TL;DR:** Changes on the 0.8 feature line, newest first, one or two lines
per version. 0.8 is an even line: it starts from the stable 0.7.20. The
previous line is [changelog 0.7](Plans/CHANGELOG.0.7.md#changelog-07).

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
