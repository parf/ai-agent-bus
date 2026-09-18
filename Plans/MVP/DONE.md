# DONE — MVP

Completed implementation and review evidence. Remaining requirements and stage
gates are in [TODO](TODO.md#objective).

| Work | Result | Evidence |
|---|---|---|
| Default GitHub profile adapter | Public profile metadata can be fetched without changing which realms require key-possession enrolment | [0.5.76 evidence](done/default-github-profile-adapter.md#checks) |
| Group detail and GitHub availability | Groups are a compact linked table with authority-scoped detail editors; a valid unique GitHub login survives unavailable optional profile metadata while explicit refresh remains atomic | [0.5.75 evidence](done/group-detail-and-github-availability.md#checks) |
| Compact web administration pages | Registration, User detail, Groups, Diagnostics and record detail share responsive cards, immediate help and grouped human-facing counts | [0.5.73 evidence](done/web-administration-redesign.md#checks) |
| Compact web registry and service detail | Selected Option 2 structure with Option 1 operational content; responsive User editor; compact help; about one day of Activity at ten-minute intervals | [0.5.72 evidence](done/web-registry-redesign.md#checks) |
| F.12 partial: installed browser authority matrix | Five real Chromium sessions exercise current service/channel, user and group controls, rendered denials, foreign-origin refusal and a real activity graph | [evidence](done/installed-browser-role-matrix.md#checks) |
| F.12 partial: installed browser foundation | Real Chromium signs in/out, checks the cookie boundary, visits current required pages, retains a session across web restart and loses it across bus restart | [evidence](done/installed-browser-foundation.md#checks) |
| Installed shared-host boundary | A package-only real-systemd host proves service-account state, two actual users' isolated sockets, and supervisor-only `CAP_CHOWN` with five installed mutations | [evidence](done/installed-shared-host.md#checks) |
| GitHub profile metadata and local photos | Login changes and explicit refresh atomically import provider fields; bounded local thumbnails render in User and caller-visible owner views without hotlinks or per-row bus reads | [0.5.71 evidence](done/github-profile.md#checks) |
| Web form recovery and keyboard entry | Refused forms retain only safe input in the shared shell; secrets stay empty and public/signed pages have a keyboard skip target | [0.5.70 evidence](done/web-form-recovery.md#checks) |
| H.1.1 upgrade and recovery | A populated packaged node stages, switches and verifies one release; failed or interrupted upgrades restore the previous release with its matching whole state tree | [0.5.69 evidence](done/upgrade-recovery.md#checks) |
| H.1 fresh installation | One verified archive installs every program and face through the root-only setup script; a real fresh systemd host starts the daemon/dashboard and completes a service call | [0.5.68 evidence](done/fresh-install.md#checks) |
| Web form and table conventions | Group members use the shared line-list textarea; numeric table columns align right with tabular figures | [evidence](done/group-editors-and-numeric-tables.md#checks) |
| Service row color key | My and Personal links match their row treatments; Personal overrides ownership and the repeated Yours word is removed | [evidence](done/service-row-color.md#checks) |
| Service row emphasis | Owned and Personal facts keep separate visible/color treatments; name is the single detail route and update time is compact | [0.5.66 evidence](done/service-row-emphasis.md#checks) |
| Runtime owner ACL and master removal | `@owner` admits one direct ownership cohort without becoming a group; launchers keep it on agent records; the former master layer is absent while daemon-Owner management remains | [0.5.74 evidence](done/owner-acl-runtime-group.md#checks) |
| Web title marks and compact help | Every current title has one decorative category mark; Services and Users definitions move from prose walls into native accessible help | [0.5.65 evidence](done/web-title-help.md#checks) |
| Web section navigation | Counted Services/Channels/Users/Groups links, dedicated registration pages, small-choice links/radios and separate Yours/Edit signals | [0.5.64 evidence](done/web-section-navigation.md#checks) |
| Service and Channel Danger Zone | Configuration replacement, ownership transfer and removal moved off ordinary detail; transfer/removal re-read facts on server-rendered confirmation pages | [0.5.63 evidence](done/service-danger-zone.md#checks) |
| Maintainers list | Owner-assigned User, Group, Agent and Service terms replace the single-group field; nested authority, migration and one-way array output retain existing boundaries | [0.5.62 evidence](done/maintainers-list.md#checks) |
| Agent glyph | WEB and human CLI use `👾` for daemon-stated Agent kinds while machine values remain plain | [0.5.61 evidence](done/alien-agent-glyph.md#checks) |
| Compact WEB glyph labels | Directory identity glyphs and Group glyphs directly prefix names without entering machine or editable values | [0.5.60 evidence](done/compact-web-glyphs.md#checks) |
| H.5.1 | Owner/Administrators durably edit the local-account map through existing credentials; a full restart applies listener identity changes without disturbing unrelated mappings | [0.5.59 evidence](done/account-map.md#checks) |
| PersonName imports | Linux account names enter through the setup/admin adapter; GitHub names are retained with verified enrolment keys; neither path accepts caller profile text | [0.5.58 evidence](done/person-name-imports.md#checks) |
| 0.5.57 live rollout | Upgrade preflight, public identity, direct-only Administrator refusal without mutation, mapped access and confined web verified | [live postflight](done/nested-groups.md#live-postflight) |
| Nested groups | Cycle-safe graph membership for ACL, Maintainer and directory views with direct-only Administrator authority | [0.5.57 evidence](done/nested-groups.md#checks) |
| 0.5.56 live rollout | Public identity, strict protected-field refusal without a profile change, separate self-email capability, confined web and peer reconnection verified | [live postflight](done/profile-authority.md#live-postflight) |
| Profile authority | Users edit only their own email; Administrators unban ordinary users without gaining authority over peers or the Owner | [0.5.56 evidence](done/profile-authority.md#checks) |
| 0.5.55 live rollout | Legacy owner establishment, public identity, mapped Owner access, confined web and peer reconnection verified without transferring live ownership | [live postflight](done/daemon-owner.md#live-postflight) |
| Daemon Owner authority | Required first-run seed, durable transfer, node-wide resource management without Administrator inheritance or ACL widening | [0.5.55 evidence](done/daemon-owner.md#checks) |
| 0.5.54 live rollout | Public identity, WEB and human CLI labels, raw JSON vocabulary, confinement and peer reconnection verified without registry changes | [live postflight](done/identity-display-labels.md#live-postflight) |
| Identity display labels | WEB and human CLI label User, Agent and Service from daemon-stated kinds while JSON, filters and editable ACL syntax stay plain | [0.5.54 evidence](done/identity-display-labels.md#checks) |
| 0.5.53 live rollout | Public identity, raw reader data, numeric CLI and WEB, fresh MCP rendering, web confinement and peer reconnection verified; existing MCP processes retain their loaded module until launcher restart | [live postflight](done/reader-visibility.md#live-postflight) |
| Reader visibility | WEB, human CLI and MCP show one numeric count across every outstanding consume request; the count is live, not health or durable state | [0.5.53 evidence](done/reader-visibility.md#checks) |
| 0.5.52 live rollout | Explicit inbox selection, address-shaped filters, malformed and unknown selection, process versions and peer reconnection verified without dequeuing a live message | [live postflight](done/inbox-selection.md#live-postflight) |
| Explicit inbox selection | API, CLI and MCP select the inbox separately from topic/tag filters; omission still reads the caller's own | [0.5.52 evidence](done/inbox-selection.md#checks) |
| 0.5.51 live rollout | Personal tab, corrected navigation, public identity, confined web process and peer reconnection verified without registry changes | [live postflight](done/personal-services-web.md#live-postflight) |
| Personal service web | Dedicated owner view, ACL-visible daemon-owner filter, main-list exclusion and atomic owner controls without changing access | [0.5.51 evidence](done/personal-services-web.md#checks) |
| Personal service core | Owner classification persists; atomic assignment limits reject users, groups, wildcard, Maintainers, Agents and Channels without changing access | [0.5.50 evidence](done/personal-services-core.md#checks) |
| 0.5.49 live rollout | Generated delegation active; web-only cgroup limits, real page, mapped bus call and peer reconnection verified read-only | [live postflight](done/web-resources.md#live-postflight) |
| G.1.2 | Web-only CPU, memory, swap and task limits; actual generated-unit pressure, recovery and fail-closed acceptance | [web resources](done/web-resources.md#checks) |
| 0.5.48 live rollout | Combined daemon update, confined web and peer reconnection verified; existing runtime sidecars retained | [rollout evidence](done/rollout-048.md#measured) |
| H.9.5 partial | Native authentication and concurrent Codex/OpenCode TUI, MCP and pusher isolation checks; Claude co-exercise remains open | [endpoint evidence](done/runtime-endpoint-auth.md#checks), [interactive evidence](done/runtime-interactive.md#checks) |
| H.5.3 | Administrative success waits for persistence; real bus-child crashes, failed disk writes and stale-checkpoint mutations exercised | [administrative durability](done/administrative-durability.md#checks) |
| G.1.3 | Supervised web filesystem, process and environment confinement; actual generated-unit acceptance with disposable canaries | [web isolation](done/web-isolation.md#checks) |
| H.5.2 | SSH forced-command entitlement and account-shell repair; isolated real-sshd acceptance and mutations | [SSH onboarding](done/ssh-onboarding.md#checks) |
| Default service access | Restricted empty ACLs apply after restore; explicit sharing, own-inbox access and metadata-refresh grants retained | [0.5.44 evidence](done/empty-acl.md#checks) |
| Administrator naming | Administrative role, protected group and client labels renamed; existing membership and explicit record grants migrated | [migration evidence](done/administrator-names.md#checks) |
| H.5.5, H.5.4 | A start deletes every record whose owner it knows nothing about, to a fixed point, and then drops the credentials that answered for them; the interim guard between the two sweeps is gone. Group membership now goes with a deleted name, which it did not | [orphan services](done/orphan-services.md#checks) |
| G.1.1 | The daemon never executes what a user supplied: registrations, configurations and message bodies stay data, and enrolment still runs the shipped verifier. An application boundary, not an OS-enforced one | [exec boundary](done/exec-boundary.md#what-proves-it) |
| H.5.10 | Every refusal an endpoint decides is counted once, on one shared path; the router's rejections and our own failures stay outside the totals, because neither is a caller being turned away | [refusal counting](done/refusal-counting.md#what-proves-it) |
| H.5.7 | A paused or banned user's services refuse calls in the daemon, on the called name rather than on who is asking; nothing is destroyed and the state lifts cleanly | [suspended owner](done/suspended-owner.md#checks) |
| H.5.6 | The group deletion verb is gone from the API and the dashboard, not only the page; a group is retired by emptying its membership | [group retirement](done/group-retirement.md#checks) |
| H.5.9 | `agent-bus-admin user add` creates the user it adds, so a fresh install can onboard somebody; an unreachable daemon refuses rather than leaving a key that works before the name exists | [user add evidence](done/user-add-provisions.md#checks) |
| F.13.5 partial | Referenced receipt correlation and qualified response links built; record detail and complete Activity share honest real-time graphs; overview, remaining diagnostics and installed acceptance remain open | [exchange evidence](done/exchange-evidence.md#checks), [activity evidence](done/service-activity.md#checks) |
| H.5.8 | The caller, its state and its authority over the target are settled under the hold the operation writes under; issuing and removal each became one held operation | [gate window](done/gate-window.md#scope) |
| Access review and smoke migration | Explicit fixture provisioning; initial unknown-principal gates checked; concurrent creation and issuance gaps reported | [access evidence](done/access-review.md#scope) |
| F.13.1/F.13.4 partial | The directory slice, before the meanings pass: user/other-identity classification, bounded directory, explicit credential cleanup and session revocation built; remaining journeys and installed acceptance stay open | [directory evidence](done/identity-cleanup.md#verification) |
| F.13.1 | Declared state, observed state and health are three different things on every page, and a value the daemon did not report is not guessed | [meanings evidence](done/web-meanings.md#what-proves-it) |
| F.6, F.6.1 and F.7–F.11 | Required dashboard tabs, user administration, protected maintainers, owner controls and activity graphs built | [owner-control evidence](done/owner-controls.md#verification) |
| A | Call, receipt, deadline and shared-reader behavior built | [wave evidence](done/wave-evidence.md#done--mvp) |
| B and C | Principal credentials, ownership, ACL and subscriptions built | [wave evidence](done/wave-evidence.md#done--mvp) |
| E | Restart persistence built | [wave evidence](done/wave-evidence.md#done--mvp) |
| F.1, F.3–F.5; F.6 partial | Filtered faces, dashboard and existing views built | [wave evidence](done/wave-evidence.md#done--mvp) |
| G | Process split and foreground service controls built | [wave evidence](done/wave-evidence.md#done--mvp) |
| H.2–H.7 | Setup, administration and token programs built | [wave evidence](done/wave-evidence.md#done--mvp) |
| H.8 and H.9–H.9.3 implementation | Runtime launchers and bundled MCP wiring built; live and installed gates remain | [launcher evidence](done/launcher-implementation.md#scope) |
| Shared version and build information | Program versions and process titles verified | [version checks](#version-and-build-checks) |
| Documentation migration | Current MVP separated from future plans; history and open questions retained | [migration record](done/document-migration.md#scope) |

## Version and build checks

The earlier full slow smoke passed 491 checks, with TypeScript typechecking and
named mutation failures recorded in [version evidence](done/wave-evidence.md#version-and-build-checks).
This documentation rewrite did not rerun runtime checks.

## What the reviews and the mutants caught

Detailed findings are archived in [mutation evidence](done/wave-evidence.md#what-the-reviews-and-the-mutants-caught).

## Nine harness traps

Historical harness lessons remain in [harness evidence](done/wave-evidence.md#nine-harness-traps).
