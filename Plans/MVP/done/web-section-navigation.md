# Web section navigation and registration

📌 **TL;DR:** 0.5.64 makes small list choices visible, moves registration to
dedicated section pages and distinguishes ownership from edit authority.

> **Superseded in 0.5.66:** the visible ownership distinction remains, but the
> duplicate list Edit link was removed. The linked name now provides the one
> route to detail and its daemon-authorized controls.

## Result

Services now exposes counted **All**, **My** and **Personal** links; Channels,
Users and Groups expose their own counted section links. Counts use the page's
existing caller-visible answers before the smaller filters and make no extra
registry read. Hidden records therefore neither appear nor contribute.

Registration moved from the bottom of list pages to `/services/new`,
`/channels/new`, `/users/new` and `/groups/new`. User and Group links and routes
repeat the daemon-returned Administrator boundary. The legacy empty `/user`
route remains. A created User returns to its detail; Group creation returns to
the list until the separately planned Group detail exists.

Delivery and identity-kind filters with three stable values are links that
retain other URL state. Service Kind and Channel delivery mode are labelled
radios with the same plain wire values. Owned resource rows visibly say
**Yours**; **Edit** follows `CanManage`, so ownership and assigned management
do not become the same claim.

## Checks

Targeted tests cover caller-visible category counts, hidden-record exclusion,
state retention and plain query values, a single `/ls` read, Service/Channel
parity, registration-form removal from lists, dedicated and legacy routes,
conditional User/Group entries and direct `403` refusals, radio values, the
created-User detail redirect, and the three ownership/management shapes:
owned-and-editable, maintained-and-editable, and view-only.

The package and full Go test suites passed. The fast repository smoke passed
**488 checks with zero failures**. The final targeted mutation set caught
**15/15** changes covering counts, retained URL state, registration entry and
route authority, radio wire values, ownership and edit authority, the single
registry read, redirects, legacy entry and removal of inline forms.

The first mutation exposed a hollow fixture: the other owner's Personal record
was hidden, so the test did not exercise the caller-specific count rule. That
survivor is retained without credit. A first rerun then started from a broken
service-principal fixture; its apparent catches are also retained without
credit. After the baseline itself passed, the corrected 15-change run caught
every mutation.

Documentation validation checked **153 files** and **2,849 local links** with
zero errors. The frozen slow smoke passed **609/0**, including vet and race.
All **203** frozen source/version hashes matched afterwards. The tracked runner
and its frozen copy both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

## Limits

This slice does not claim Group detail, invalid-input form preservation,
search, sorting, paging, page-title glyphs, compact help, activity placement,
owner photos or the remaining visual redesign.

## Live postflight

Commit `bcccde3` was built once for both binaries with build stamp
`parf@parf.us 2026-09-17 19:57:54` and deployed by restarting the approved
live unit. The daemon reported 0.5.64 and anonymous daemon status remained
`401`. Authenticated reads of Services, Personal, Channels, Users and Groups,
plus all four dedicated registration pages, returned `200`. The rendered pages
contained their counted section links and active state, the separate
**Yours**/**Edit** signals, and the plain-valued radio choices. No live record
was created or changed.

The web child remained inside its delegated cgroup with a 256 MiB memory
limit, no swap, 64 PIDs and one CPU. It had zero effective capabilities and
an environment containing only `AGENT_BUS_ADDR=/bus.sock` and `PWD=/`.
