# Changelog

## 0.5.66 — 2026-09-17

Emphasize caller-owned and Personal service rows without conflating the two.
Remove the duplicate list Edit column and compact registration-update times.

## 0.5.65 — 2026-09-17

Add a decorative category image or glyph to every web page title. Move the
long Services and Users definitions into accessible native help popovers.

## 0.5.64 — 2026-09-17

Add counted section navigation and dedicated registration pages for Services,
Channels, Users and Groups. Show small stable choices directly and mark owned
records separately from daemon-authorized Edit actions.

## 0.5.63 — 2026-09-17

Move service and channel configuration replacement, ownership transfer and
registration removal behind a red Danger Zone link. Re-read transfer and
removal facts on server-rendered confirmation pages and return ordinary edits
to the resource they changed.

## 0.5.62 — 2026-09-17

Replace the single resource Maintainers group with an owner-assigned list of
Users, Groups, Agents and Services. Preserve nested authority and legacy
snapshots while emitting the new array form on every current face.

## 0.5.61 — 2026-09-17

Use `👾` for Agent in WEB and human CLI output. Keep the shared display
mapping as the sole source and retain plain machine values.

## 0.5.60 — 2026-09-17

Put entity glyphs directly before identity and group names in the web
directory. Remove the redundant second-line type label while keeping machine
values and editable syntax plain.

## 0.5.59 — 2026-09-17

Persist the local OS-account map and let the daemon Owner or Administrators
edit it through existing credentials. Setup flags seed once; a full daemon
restart applies changed private-socket identities and retires removed sockets.

## 0.5.58 — 2026-09-17

Import blank person names from the trusted Linux account adapter and GitHub
directory enrolment. Provider facts are retained with the verified challenge;
existing Administrator-entered names are never overwritten.

## 0.5.57 — 2026-09-17

Resolve nested groups consistently for ACL access and service Maintainer
authority. Cycles terminate, unknown references remain inert until populated,
and the protected Administrator group stays direct-only.

## 0.5.56 — 2026-09-17

Let users edit or clear only their own email, with protected profile fields
kept outside the self-service request. Administrators may now unban ordinary
users without gaining authority over peers or the daemon Owner.

## 0.5.55 — 2026-09-17

Persist daemon ownership and let the current Owner manage every service and
channel without widening message access. Ownership transfers to an active
registered User survive restart; startup now requires an explicit owner seed.

## 0.5.54 — 2026-09-17

Label User, Agent and Service identities consistently in WEB and human CLI
from daemon-stated kinds. JSON, URLs, CLI arguments and editable ACL values
keep their plain machine vocabulary; rows without a visible kind stay unlabeled.

## 0.5.53 — 2026-09-17

Show one Readers count across filtered and unfiltered consume requests in WEB,
human CLI and MCP listings. Reader counts are live observations, never health
or durable state; the older unfiltered boolean remains wire compatibility.

## 0.5.52 — 2026-09-17

Select a consume inbox explicitly across the API, CLI and MCP face. Topic and
tag now only filter messages within that inbox; omission reads the caller's
own, and the former address-shaped topic overload is removed.

## 0.5.51 — 2026-09-17

Group Personal services in a dedicated owner view without changing access or
delivery. Main service lists omit them; owners change classification, ACL and
Maintainers atomically, and the daemon owner can filter the Personal records
visible through ordinary access.

## 0.5.50 — 2026-09-17

Persist an owner-controlled Personal classification for services. Personal
services keep ordinary authorization and delivery while refusing user, group,
wildcard and Maintainer assignments; metadata refresh preserves the owner's
choice and combined policy changes validate atomically.

## 0.5.49 — 2026-09-17

Confine the supervised dashboard and its wrapper to one CPU, 256 MiB memory,
no swap and 64 tasks in a delegated web-only cgroup. Resource pressure may
restart web under the same limits while the bus stays available; missing or
unusable delegation keeps web down instead of starting it without limits.

## 0.5.48 — 2026-09-17

Protect the launcher-owned Codex App Server with a private per-run capability. The TUI and pusher authenticate; other local accounts cannot attach merely by finding its loopback port.

## 0.5.47 — 2026-09-17

Persist administrative changes before acknowledging success, so bans, removed memberships and tightened ACLs survive bus crashes. Snapshot capture and replacement are serialized; failed persistence returns an error and can be retried.

## 0.5.46 — 2026-09-17

Confine the supervised web child to its executable, shared API socket and explicit TLS inputs, excluding daemon state and privileged sockets. Bubblewrap and unprivileged user namespaces are required; sandbox failures never start an unconfined dashboard.

## 0.5.45 — 2026-09-17

Preserve SSH operator key entitlement and rotation requests through token delegation. Setup gives the daemon account a shell that can run restricted SSH commands and repairs the previous nologin default without replacing a custom shell.

## 0.5.44 — 2026-09-17

Empty ACLs now retain only resource-owner, assigned-Maintainer and own-inbox access, including after restore; implicit master access no longer opens them. Re-registration preserves omitted ACL settings, script runners accept explicit sharing options, and forms explain the restrictive default.

## 0.5.43 — 2026-09-16

Name daemon user/group managers Administrators, with the protected `@administrators` group and matching API, CLI and web labels. Migrate prior administrative membership and explicit service grants without promoting members of an existing ordinary group at the new name; service/channel Maintainer roles remain separate.

## 0.5.42 — 2026-09-16

Turn the header bus logo to face right.

## 0.5.41 — 2026-09-16

Add an inline red-bus logo spanning the identity and navigation rows; show AgentBus version, @ host, owner, uptime and calls beside it, including on sign-in.

## 0.5.40 — 2026-09-16

Simplify the header to uptime and minute/hour/total calls, keeping sampled spans in the API; the one-line footer shows matching builds once and identifies differing builds.

## 0.5.39 — 2026-09-16

The one-line dashboard header shows daemon calls for the sampled minute and hour,
with their observed spans, and the exact total since start. These admitted HTTP
requests replace host load and message counts; build details stay in the footer.

## 0.5.38 — 2026-09-16

Every dashboard page, including sign-in, shows the daemon release, host, owner,
uptime and host load, with separate daemon/web build details in its footer.
Public message windows count accepted inbox deliveries and dequeues from this
run, retain traffic after record removal, and state their observed span;
private status remains authenticated.

## 0.5.37 — 2026-09-16

Every refusal an endpoint decides is counted once, including the
request-validation and hidden-or-missing-name paths that answered callers
silently. Internal failures and router rejections stay outside the totals,
because neither is a caller being turned away.

## 0.5.36 — 2026-09-16

The dashboard stops merging what a record declares, what the daemon observed and
whether anything is alive: delivery, reader and reached are separate columns,
node totals say they are node-wide while every list says it is yours, an unset
queue setting says what it inherits instead of showing a guessed number, and a
refusal reason that never happened is drawn as the measured zero it is. The
directory names its three kinds of identity rather than guessing one, says which
of its counts the page worked out for itself, and reads the same way with no
unclassified identity on it as with one. An inbox with a reader waiting on one
topic is no longer described as having no reader at all — on the dashboard, in
the CLI listing and in the MCP catalogue, which had all said it and which the CLI
also let a protocol hint hide entirely. A queue holding messages is no longer
called stuck, an enabled record no longer implies a send would be accepted, and
the refusal counters say they are recorded counts rather than all refusals,
naming the paths that are missing — the contract says so too, and H.5.10 closes
it.

## 0.5.35 — 2026-09-16

A start deletes every record whose owner the daemon knows nothing about — to a
fixed point, because deleting is what makes the next orphan — with its queues,
its subscriptions and the readers blocked on it, and then drops the credentials
that answered for them. The interim guard that kept the two sweeps from
disagreeing is gone with it. Separately: a deleted name's group membership now
goes with the name, which it did not — a freed name is reclaimable, so whoever
registered it next inherited every group the previous holder was in.

## 0.5.34 — 2026-09-16

A paused or banned user's services refuse calls, in the daemon rather than as a
label on a page. The check is on the called name, so a stranger, a maintainer
and the daemon owner are all refused the same `403 suspended`, and so is the
service's own credential — kept is not accepted, and a credential that survives
a ban grants nothing while it lasts. A reader already blocked on the inbox is
released with the same reason at the moment of the pause. Nothing is destroyed:
the record, its queues, both credentials and the user record all survive, and
the same credential works again the moment the state is lifted. Owner
suspension is its own predicate rather than another input to a name's own
state, because a service whose owner turned delivery off and one whose owner is
suspended are two different facts and a page may not merge them.

## 0.5.33 — 2026-09-16

The verb that deleted a group is gone, from the API and from the dashboard, not
only from the page. A group is retired by emptying its membership: the name
stays, every record pointing at it still means what it meant, and putting a
member back brings it back. The removal request is refused rather than dropped,
because a request whose only removal-shaped field is ignored decodes as a
membership save with no members — a different operation, answered with success.
The dashboard's `POST /groups` rejects the old `delete` action for the same
reason: it rendered no button for it and accepted it anyway.

## 0.5.32 — 2026-09-16

`agent-bus-admin user add` creates the user it adds, so a fresh install can onboard somebody. Writing an `authorized_keys` line was never the whole of adding a person: strict issuing refuses a name the daemon holds nothing for, so the forced command the key reaches answered every newcomer with a refusal. The key line is written first because it is the half that can be taken back and a user is never deleted, and it is removed again if the daemon refuses, so the two halves land together or not at all. `--admin` grants maintainer standing by group membership, which is the only thing that grants it. An unreachable daemon refuses the whole operation rather than leaving a key that works before the name exists.

## 0.5.31 — 2026-09-15

A caller that stopped being one is no longer still acting. Who the caller is, what state they are in and what authority they have over what they are touching are established under the hold the operation writes under, not at the door and not beforehand: the gate released the registry before core took it, and in that gap a caller whose record was removed could register itself back into existence, a caller paused after the gate was served anyway, and asking for a token settled who owned the target before the credential was written — so a record changing hands in between handed the former owner the current owner’s credential. Removing an address and dropping its credential are now one operation, abandoned whole if the credential store refuses the write; a transfer cannot hand a record to somebody who cannot act; a removed principal’s blocked reads end with it; and the clause that let an unknown name create itself is gone, with enrolment saying so for itself instead.

## 0.5.30 — 2026-09-15

Diagnostics keeps message IDs and receipt references, correlates receipts against the full return route, and labels ordinary response matches as possibilities. Retained-history limits, missing evidence and narrow-screen layouts stay explicit; unused credentials are shown without guessing their origin.

## 0.5.29 — 2026-09-15

An unregistered name can do nothing. A name the daemon holds no profile and no record for was reading as an active user; it is now refused `401` on every call, counted as a credential refusal rather than as a suspension, and it is not issued a credential in the first place — the operation that left this node holding 231 credentials answering for nothing. Registering a record for an owner the daemon does not know is refused, and so is removing a record while its name still owns others, so an ordinary call can no longer leave a service owned by nobody. An unowned registration now means self-owned. Configuring a name that does not exist creates it, so it now obeys what creating obeys: a name in a realm somebody vouches for could be claimed by configuring it rather than registering it, and then issued a credential with no key proved. Issuing decides and mints under one hold, so a name cannot stop existing in between.

## 0.5.28 — 2026-09-15

The user directory distinguishes registered users from other identities, keeps unused credentials visible for review, and lets owners and maintainers remove eligible credentials explicitly. Cleanup rechecks current state, persists before revoking tokens and sessions, and preserves failed writes; directory search and pagination avoid per-avatar bus requests.

## 0.5.27 — 2026-09-15

The daemon drops credentials that answer for nothing at start: no record of their own and no registered user. This node had 232 of them against four records, every one listed as a user. One interim while orphan-service deletion is still pending — a name that owns services keeps its credential, so the sweep cannot leave a service nobody answers for.

## 0.5.26 — 2026-09-15

The supervisor no longer hands the dashboard child an `AGENT_BUS_TOKEN` it inherited: the dashboard is meant to hold no credential of its own, and that held only while nobody started the daemon from a shell that had one exported.

## 0.5.25 — 2026-09-15

A dashboard refusal is now a page rather than a dead end. An anonymous deep link gets the sign-in form at the address it asked for and returns there afterwards; a return address that is not this dashboard's own is refused. A bus that is not answering says so instead of claiming the visitor is not signed in, and an expired session, a permission refusal, an unknown name and a daemon fault each get their own recovery, on the shell, with the navigation still under them.

## 0.5.24 — 2026-09-15

Every dashboard page now shares one shell: a single navigation with the current entry marked, sign out beside the name it signs out, a document language, a viewport, a main landmark and a title of its own. Pages no longer reload themselves — the diagnostics page says when it was built and offers a Refresh link instead, and the activity page's own thirty-second reload is gone. Muted text meets the contrast minimum, and narrow screens get a smaller gutter with only wide tables scrolling.

## 0.5.23 — 2026-09-15

The authority levels are nested rather than side by side: adding somebody to `@maintainers` now makes them a registered user, as starting the daemon already did for the owner. A snapshot written before this rule is repaired on restore. Taking somebody out of the group leaves the person behind, since a user is never deleted.

## 0.5.22 — 2026-09-15

Removing a registration says what it does: no registration, no access. The dashboard offers no group deletion at all — a group is made inactive or banned as a person is, rather than removed out from under the records naming it. The group states themselves are pending.

## 0.5.21 — 2026-09-15

Two dashboard pages stop promising what the daemon refuses. Removing a registration no longer says credentials remain valid: the service credential goes with the address, and only a person's own stays. The maintainers group no longer offers a delete button, because that deletion is refused for everybody including the daemon owner.

## 0.5.20 — 2026-09-15

`agent-bus-web` refuses to start when a certificate was asked for and is not there, instead of logging the fact and serving plain HTTP. Asking means supplying `-cert` or `-key`; either without the other is the same ask and the same refusal. Not asking at all is unchanged and still plain HTTP on loopback.

## 0.5.19 — 2026-09-15

A send refused because the receiver's queue is full answers `429` rather than `503`: a full queue is the sender outrunning the reader, and `503` is left to mean a service that is itself unavailable. The counted refusal reason is unchanged.

## 0.5.18 — 2026-09-15

The dashboard is `http://127.0.0.1:6780` and nothing else: the borrowed `localhost.direct` hostname, the certificate kept under that name and the 443/8443 binding fallback are gone, since the certificate that scheme existed for had expired. HTTPS remains for anyone who supplies `-cert` and `-key`. The API root's redirect to the dashboard is now `301`.

## 0.5.17 — 2026-09-15

A removed name keeps nothing: no reservation for its last owner, and no credential. Whoever asks for a removed name next gets it, its previous owner included. Reserving it cost a permanent entry and a permanent credential per address — one launcher-smoke run left roughly 250 in one person's name list — for protection this stage does not need; restoring it is R1.2's.

## 0.5.16 — 2026-09-15

The name list says whose each credential is and what it is for, so a person can tell their own identity from the services they registered.

## 0.5.15 — 2026-09-15

The dashboard's service and channel listings, and a record's own page, show when the record was last written.

## 0.5.14 — 2026-09-15

The API root sends a browser to the dashboard instead of answering nothing: an exact-root 303 to `-dashboard` / `AGENT_BUS_DASHBOARD`, leaving a mistyped route its 404. One package now owns where the dashboard is reached.

## 0.5.13 — 2026-09-13

Coordinate `ab_rename` through the launcher so runtime titles, inbox readers, MCP credentials and saved session bindings move together; serialize concurrent renames and preserve the launching account’s ownership.

## 0.5.12 — 2026-09-13

Give all smart launchers session-aware terminal titles with exit restoration and rotating Kitty/Konsole tab colors; keep terminal helpers internal and add OpenCode’s violet palette.

## 0.5.11 — 2026-09-13

Add `ab_rename`: a session changes the address it is registered under, taking its own title when given no name. The new address is registered before the old one is released, and a busy old address is kept and reported rather than dropped with its queue.

## 0.5.10 — 2026-09-13

Prefer OpenCode's user-installed wrapper over a system binary earlier in PATH, preserving wrapper settings when launched from fish; explicit `OPENCODE_BIN` still wins.

## 0.5.9 — 2026-09-13

Bind OpenCode's TUI and inbox reader to the same explicit session, including fresh starts; do not wait for a session-selection event that initial startup does not emit.

## 0.5.8 — 2026-09-13

Add `ab-opencode`: an opencode push adapter over the runtime's HTTP server and event stream, and a launcher that owns a password-protected loopback server the session attaches to.

## 0.5.7 — 2026-09-13

Add the required dashboard tabs with service/channel owner controls, protected maintainer groups, user profiles and pause/ban controls, and bounded activity graphs. Persist administration state and cancel blocked reads when access is removed.

## 0.5.6 — 2026-09-13

Use template/instance addresses in `ab-claude` and `ab-codex`; migrate saved dot-form addresses on restart while preserving explicit names and busy old inboxes.
Exclude a session's own previous registration when assigning its label, avoiding a false `#2` during migration or rename.

## 0.5.5 — 2026-09-13

Add `agent-bus unregister` for idle registry entries, preserving credentials and ownership; show the owner in `ls -h`.
The `ab-*` launchers derive a new address after a session rename on restart and remove the old entry only when idle; explicit bus names stay fixed.

## 0.5.4 — 2026-09-13

Add `agent-bus ls -h` for a readable registry table, including filtered and single-record listings.

## 0.5.3 — 2026-09-13

Make the token helper use local socket discovery when no address is configured, preserving explicit addresses and token identity.

## 0.5.2 — 2026-09-13

Discover local bus sockets automatically in the CLI and both launchers; allow local session tokens through the shared socket across OS accounts.
Report the failing bus endpoint before starting runtime sidecars.

## 0.5.1 — 2026-09-13

Add Claude and Codex launchers with bus MCP tools, automatic execution, session continuation and distinct session identities.
Build portable launcher and MCP artifacts alongside the stamped Go programs.

## 0.5.0 — 2026-09-13

Establish one shared MVP version across all programs and MCP handshakes; `--version` also reports build information in Go programs.
Daemon and runner process titles show the version and live call counts in `ps`.
