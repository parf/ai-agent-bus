# Installed browser authority matrix

📌 **TL;DR:** Real Chromium exercises the current installed controls and their
authority boundaries; the matrix must run again after the F.13 redesign.

## Scope

This extends the [installed session foundation](installed-browser-foundation.md#checks)
on the same package-only real-systemd host. Five separate browser sessions use
the daemon Owner, an Administrator, a resource Owner, an assigned resource
Maintainer and an ordinary user. Fixture setup uses installed administration
and CLI programs; every credited web action or refusal runs through Chromium.

This is current-UI F.12 evidence. It does not approve or exercise the pending
F.13 redesign, and it does not replace the required post-redesign rerun.

## Checks

The accepted run is under
`tmp/installed-browser-roles-20260918a/baseline-6/evidence/`.

| Browser identity | Measured action or boundary |
|---|---|
| Daemon Owner | Creates a user, saves the protected Administrator group, edits an ordinary group, manages another user's service and opens the real service activity graph and sample values |
| Administrator | Creates a user, creates and edits an ordinary group, cannot render or submit protected-group changes, cannot edit the daemon Owner and cannot manage an unrelated service |
| Resource Owner | Registers a service and queue channel, edits settings and atomically assigns the service Maintainer |
| Resource Maintainer | Edits ordinary service settings, but receives neither owner-only classification/Maintainer assignment nor ownership transfer |
| Ordinary user | Reads an explicitly shared service, but a direct same-origin mutation and the user/group administration pages are refused |

A real installed echo call supplies the nonzero Activity sample. A form served
from a second loopback origin is refused, while the same-origin user, group,
service and channel forms succeed. Denials are asserted from their browser
responses; a missing control alone does not receive denial credit.

The driver records the role inventory in `browser-roles.json` and captures the
resource-owner and activity pages. Tokens remain under `/root` in the
disposable host and are neither logged nor copied to evidence.

Three exploratory runs receive no credit. The first used Playwright's partial
label match and confused Name with the ACL's “one name” label. The second used
`fetch`, which the page's own CSP refused before the request reached
authorization. The third polled activity on a service that had not received
the measured call. Exact labels, real form submission and the called service
fix the three attribution errors.

## Mutations

Six isolated source overlays each built a package and ran a fresh real-systemd
host. Final credited result: **6/6 caught**.

| Change | Named failure |
|---|---|
| Remove assigned-Maintainer management | Maintainer lacks Settings |
| Give every Administrator resource management | Unrelated service exposes Settings |
| Remove the daemon Owner's management override | Owner lacks root service management |
| Give every Maintainer ownership transfer | Maintainer receives owner-only classification |
| Render the protected-group editor for Administrators | Protected editor appears |
| Accept every form origin | Foreign-origin form is not refused |

The first origin overlay did not compile because it left parse variables
unused and receives no credit; the corrected compiling bypass is the one in
the table. Logs are under `tmp/installed-browser-role-mutations/`.

One additional core overlay removed the owner-only protected-group guard and
survived this installed browser run, so it receives no credit here. The focused
core test fails under that overlay and continues to pin the branch. The browser
matrix therefore claims the protected rendered surface and normal denial, not
independent mutation credit for that exact core branch.

## Repository verification

The fast suite passed **488/0**. The byte-frozen slow suite passed **609/0**;
all **222** tracked and candidate `src/` hashes remained unchanged. The
documentation checker covered **167** Markdown files and **2,879** local links
with zero errors.

The frozen runner SHA-256 is
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.
The installed-host fixture and role-driver SHA-256 values are
`50710753be6f4045b0cb09c76422de887cad68de947599c3a99e6b0af9f7ce4e`
and
`4a2e0cfa56d4f689a9cf4aedca2b09e83e2f56f4bf8118571d64765254fc49cf`.
The frozen manifest and slow log are under
`tmp/installed-browser-roles-final/`.

## Remaining browser work

Retain this matrix while implementing F.13, then rerun it against the approved
service/channel and user/group journeys at the required desktop, narrow and
zoomed sizes. The redesign gate remains open.
