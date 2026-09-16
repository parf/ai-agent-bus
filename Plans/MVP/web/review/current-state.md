# Current web inventory

## Evidence boundary

Snapshot of commit `471f550`, inspected 2026-09-16. Existing implementation,
not a target specification or a new wire design. The owner requested a
field-and-function inventory; names below transcribe existing code.
Recommendations belong in [Codex review](codex.md#junk-and-misleading-content).

The web child declares fifteen method/path patterns, uses nine HTML templates
and one SVG template. A route, a template and a conditional page state are
different counts. No separate overview, account, group-detail or service-edit
page exists in this snapshot.

The HTML template names are `dash`, `anon`, `services`, `service`, `groups`,
`activity`, `people`, `person` and `problem`; the SVG template is `avatar`.

Browser inspection used an isolated build of that commit, synthetic data,
owner and ordinary-user sessions, desktop and narrow viewports. No production
data was modified. These observations establish rendering, not normal workload,
actual user habits or exhaustive permission correctness.

## Routes and templates

All signed-in HTML templates use the [shared shell](../../../../src/cmd/agent-bus-web/main.go#L372):
document title, signed-in `.You`, sign-out form, navigation and main landmark.
Navigation destinations are declared once in [navItems](../../../../src/cmd/agent-bus-web/main.go#L359).
Sign out already appears on every signed-in page; the earlier audit's missing
sign-out finding is historical.

| Browser method and route | Rendered template or result | Data requested from bus | Source |
|---|---|---|---|
| GET `/` | `page` / template `dash`; `anon` without a session; error recovery on status failure | `/status`, `/ls`, `/recent`, `/names` | [main:85][M85] |
| GET `/services` | `serviceList` / `services`, excluding topics | `/status`, `/ls` | [admin:166][A166] |
| GET `/channels` | Same template, topics only; heading changes but document title and active nav still say services | Same as services | [admin:176][A166], [admin:324][A324] |
| GET `/service?name=…` | `serviceDetail` / `service`, including topics | `/status`, `/lookup?name=…`, `/groups` | [admin:194][A194] |
| GET `/groups` | `groupList` / `groups` | `/status`, `/groups` | [admin:209][A209] |
| GET `/activity?name=…` | `activityPage` / `activity`; absent name means all visible | `/status`, `/ls`, `/activity?name=…` | [activity:45][G45] |
| GET `/users` | `peoplePage` / `people` | `/status`, `/users` | [users:91][U91] |
| GET `/user?name=…&return=…` | `personPage` / `person`; missing name is Add user only for administrator | `/status`, `/users`, then select visible identity | [users:112][U112] |
| GET `/avatar?name=…` | `avatarPage` / `avatar`, SVG initial; no current HTML template references it | `/status`, entire `/users` response, then select visible identity | [users:133][U133] |
| GET `/healthz` | Empty success; no page | None | [main:81][M81] |
| POST `/signin` | New browser session and local redirect; otherwise `anon` | POST `/session` using submitted token | [main:133][M133] |
| POST `/signout` | Clear browser cookie, redirect `/` | DELETE `/session` when cookie exists | [main:166][M166] |
| POST `/service` | Action dispatch, refusal or redirect `/services?scope=my` | See service actions below | [admin:241][A241] |
| POST `/groups` | Action dispatch, refusal or redirect `/groups` | POST `/group` | [admin:305][A305] |
| POST `/user` | Action dispatch, refusal or validated directory return | See identity actions below | [users:152][U152] |

`GET /` is a catch-all ServeMux pattern, not an exact-root route. An unknown
GET path can therefore render diagnostics/sign-in. There is no dedicated
GET `/signin` handler; auth-required pages render the form at the requested URL.

## Visible fields by page

Lists enumerate displayed values, including repeated, blank and conditional
ones. An input's current value is also visible data. The forms inventory below
is part of this field inventory, not optional detail.

| Page or section | Every displayed dynamic field and conditional state | Source |
|---|---|---|
| Sign-in | Refusal text when present; token acquisition commands; hidden local return; token password input | [main:410][M410] |
| Services/channels list | Heading by channel flag; scope and availability selections; each record's linked Name, Owner, Disabled → Active/Inactive, Proto/Reading → External/Serving/Offline, Queued, At → Updated or unknown, CanManage → plain Manage/View text; no-matching-records row. Name-sorted by the listing handler | [admin:189][A166], [admin:324][A324] |
| Service/channel header | Name; Owner; Maintainers even when empty; Disabled state; Proto/Reading label; Queued; Oldest even when empty; Dropped; Expired; AtBound → full; At update timestamp/unknown; ConfigSHA even when empty | [admin:335][A335] |
| Pub/sub detail only | Subscriber names; No subscribers fallback; conditional subscriber-removal forms; both self-subscribe and self-unsubscribe buttons | [admin:340][A335] |
| Editable service details | Descr, Addr, Proto, Allow joined by spaces, NoMaster, TTL, Bound, Full; chosen maintainers group and available group names; see form guards below | [admin:343][A335] |
| Read-only service details | Descr and management-permission explanation; address, protocol, ACL and queue policy are not rendered outside the editor | [admin:358][A335] |
| Groups | Group name as heading; members joined into an editable input only when authorized; owner-protection text for the maintainers group; create form only for administrators; otherwise role explanation; no-groups fallback | [admin:359][A359] |
| Activity | Selected record Name; choice of every visible record; sampling/window/restart/dequeued explanation; five graph labels, independently scaled polylines and Max; accessible graph label includes metric and maximum; sample table has At, In, Out, Dropped, Expired, Refused; no points → collecting-first-sample | [activity:19][G19], [activity:71][G71] |
| Users summary/filter | PeopleCount and OtherCount over caller-visible directory before local search; Query and Kind; Start, End and Matched after filtering/paging; Add user if Administrator; Clear filters; Previous/Next URLs | [users:33][U33], [users:190][U190] |
| Registered-user rows | Optional PersonName; linked full Name; DaemonOwner/Maintainer → authority; State; no-users-on-this-page fallback | [users:201][U190] |
| Other-identity rows | Linked Name; Kind → registered name or credential without registered name; reason based on Kind and owned Services; record-inspection link, removal-review link if CanRemove, or administrator help; no-other-identities-on-this-page fallback | [users:204][U190] |
| Identity detail, all existing identities | Full Name; validated Back to directory; Services as links or No owned services | [users:214][U214] |
| Identity detail, registered user | State; daemon authority; Groups or no-memberships; profile inputs when CanEdit; otherwise PersonName, Email, GithubUser as unlabelled paragraphs plus edit-authority help; lifecycle controls as below | [users:225][U214] |
| Identity detail, non-user | Kind heading; explicit no-user/no-lifecycle statement; self-owned-record inspection link, retained-owned-services explanation, or unused-credential reason; removal consequences and form only if CanRemove | [users:217][U214] |
| Add user | Heading; profile/create fields only; no existing-user state, groups or owned-services sections | [users:228][U214] |
| Avatar SVG | Uppercase first rune of PersonName, otherwise Name, on fixed background | [users:140][U133], [users:238][U238] |
| Problem page | Title, optional Detail, Advice, optional Back/BackLabel; shell has caller when already established | [admin:100][A100] |

### Diagnostics fields

| Section | Every displayed field | Source |
|---|---|---|
| Page header | `agent-bus`, Refresh link, render time At | [main:432][M432] |
| Node | Status.Up, Services (labelled records), Queued, Waiting, Dropped, Expired; Unclean warning; nonempty Refusals sorted by Count then Reason, otherwise nothing-refused | [main:435][M432], [views:110][V110] |
| Stuck inboxes | Backlogs: Name, Reading → reading/nobody, Queued, Oldest, AtBound → full; empty → every queue is empty. Source selects every record with Queued > 0, ordered oldest/depth/name; it does not establish a stall | [main:444][M432], [views:51][V51] |
| Exchanges, common | Visibility/retention disclaimer; history-unavailable or no-envelopes state; optional reading-help disclosure; each row At with offset, ID and anchor, From, To, Topic/Tag or not supplied, optional ReplyTo.Service/Topic/Tag; N (original plus folded receipts) | [views:228][V228] |
| Ordinary exchange evidence | Ack observed; Done observed or no-completion-observed sentence; Matches as linked candidate IDs; qualified Late text; receipt-evidence disclosure listing each receipt's Receipt, ID, Re, From, To, At and Late | [views:253][V228] |
| Standalone receipt | Receipt type, Re if supplied, Notice explaining absent/mismatched/ambiguous/no-reference/receipt-target evidence | [views:181][V181], [views:253][V228] |
| Registry | Every visible record's Name, Kind, Descr, Reading → yes/blank, Queued, In, Out, ConfigSHA; empty → nothing registered. Renders incoming order; core List iterates a map without sorting | [main:450][M450], [core/bus:443](../../../../src/internal/core/bus.go#L443) |
| Loss by name | Name, Dropped, Expired for records with either loss nonzero, sorted combined loss descending/name; empty → nothing lost | [main:457][M450], [views:85][V85] |
| My names | NoNames error or Name, Kind (unregistered emphasized), Owner, Fingerprint, Issued timestamp/blank, Used time/not-this-run; no-credential fallback; unregister/legacy prose; rotation command/consequence help; envelope-only notice | [main:462][M450] |

## Forms and actual effects

All mutations forward the visitor's session. Listing controls use GET and do
not write. Hidden fields are listed explicitly. The form's visibility is not
the daemon's final authorization decision.

### Global and filters

| Form | Every input | Browser destination → effect | Source |
|---|---|---|---|
| Sign in | `token` password; optional hidden `return`; submit | POST `/signin` → POST `/session`, cookie, local redirect; token is not repopulated | [main:133][M133], [main:410][M410] |
| Sign out | No named inputs; submit | POST `/signout` → DELETE `/session`, expire cookie, redirect root | [main:166][M166], [main:379][M372] |
| Services/channels filter | `scope`: all/my; `state`: all/active/inactive; Filter | GET current list URL → filter visible records by owner and Disabled | [admin:176][A166], [admin:326][A324] |
| Activity filter | `name`: empty All visible or a visible record; Filter | GET `/activity?name=…` → fetch activity for that selection | [activity:45][G45], [activity:73][G71] |
| Directory filter | `q` search; `kind`: empty/users/other; Apply | GET `/users` → case-insensitive substring in Name/PersonName/Email/GithubUser, classification filter, bounded page | [users:33][U33], [users:195][U190] |

### Service and channel actions

Every row below submits POST `/service`. Every success currently redirects to
the same services/My URL, including channel actions. API errors render the
problem page; invalid capacity/configuration/kind can instead return plain text.
There is no server-rendered preservation of nonsensitive submitted fields.

| Action and visibility | Every input | Daemon operation and supplied fields | Source |
|---|---|---|---|
| Register, both lists, no additional template role guard | Hidden `action=create`; required `name`; `descr`; `allow`; service `kind` select generic/agent OR hidden `kind=topic` plus `mode` pubsub/queue | POST `/register`: Name, Kind, Mode, Descr, whitespace-split Allow; client supplies create-only header | [admin:279][A241], [admin:330][A324], [main:289][M278] |
| Save settings, CanManage | Hidden `name`, `action=save`; `descr`, `addr`, `protocol`, `allow`; checkbox `no_master`; `ttl`; numeric `bound` min zero; `overflow` strict/ring | POST `/manage`: all eight editable settings, including false/empty values; Bound parsed as integer, Allow whitespace-split | [admin:246][A241], [admin:344][A335] |
| Enable/Disable, CanManage | Hidden `name`; button `action=enable` or disable | POST `/manage`: Disabled false/true | [admin:257][A241], [admin:353][A335] |
| Replace configuration, CanManage | Hidden `name`, `action=configure`; required `config` textarea, empty, autocomplete off | JSON validation then POST `/configure`: Name, Config; existing private config is never populated | [admin:269][A241], [admin:354][A335] |
| Assign maintainers, CanManage and CanTransfer | Hidden `name`, `action=maintainers`; `maintainers` select None or returned group name | POST `/manage`: Maintainers | [admin:261][A241], [admin:355][A335] |
| Transfer, preceding guards and Name differs from Owner | Hidden `name`, `action=transfer`; required `owner` | POST `/manage`: Owner | [admin:265][A241], [admin:356][A335] |
| Subscribe/Unsubscribe my inbox, pubsub mode, outside CanManage | Hidden `name`; button `action=subscribe` or unsubscribe | POST `/subscribe`: Topic=name, Off flag; no choice of another subscriber | [admin:286][A241], [admin:342][A335] |
| Remove subscription, each subscriber if CanManage | Hidden `name`, `subscriber`; button `action=remove-subscriber` | POST `/subscriber/remove`: topic, subscriber | [admin:291][A241], [admin:341][A335] |
| Remove idle service, CanManage | Hidden `name`, `action=delete`; consequence text; submit | POST `/unregister`: name; no separate confirmation page | [admin:293][A241], [admin:357][A335] |

### Group and identity actions

| Action and visibility | Every input | Browser destination → daemon operation | Source |
|---|---|---|---|
| Save members, Administrator and either DaemonOwner or group is not maintainers | Hidden `name`; `members` whitespace-delimited input; button `action=save` | POST `/groups` → POST `/group`: Name, Members, Remove=false | [admin:305][A305], [admin:360][A359] |
| Create group, Administrator | Required `name`; `members`; button `action=save` | Same save endpoint and fields; redirect groups | [admin:305][A305], [admin:361][A359] |
| Legacy group delete, **no rendered control** | Handler accepts `action=delete`, `name`, `members` | POST `/groups` → POST `/group`, Remove=true; separate pending API-removal work, not a UI feature to restore | [admin:307][A305], [group policy](../../../../docs/01-identity.md#groups-and-maintainers) |
| Create user, New administrator view | Hidden `return`, `action=create`; required `name`; `person_name`; `email` type email; `github_user` | POST `/user` → POST daemon `/user`: profile fields and Create=true | [users:170][U152], [users:228][U214] |
| Save profile, CanEdit | Hidden `return`, `name`, `action=save`; `person_name`, `email`, `github_user` | POST `/user` → POST daemon `/user`: profile fields, Create=false | Same sources |
| Activate/Pause/Ban, existing non-owner user within editable branch | Hidden `name`, `return`; `action=active` only if CanActivate; `action=paused`, `action=banned` | POST `/user` → POST `/user/state`: name, state; no user-delete control | [users:176][U152], [users:233][U214] |
| Remove unused credential, non-user with CanRemove | Hidden `name`, `return`; button `action=remove-credential` | POST `/user` → POST `/identity/remove`: name; explicit removal consequences above form | [users:168][U152], [users:223][U214] |

User actions return to a validated `/users` URL with retained query state.
Failures use a separate problem page and do not re-render the submitted form.

## Errors and recovery

| Trigger | Current response | Source |
|---|---|---|
| No session on protected page | Sign-in form at requested URL with local return | [main:211][M211], [admin:128][A128] |
| Bus 401 | Sign-in, session-ended explanation | [admin:54][A38] |
| Bus 403 | Problem: Not yours to see; daemon payload plus permission/owner advice | [admin:59][A38] |
| Bus 404 | Problem: same hidden/missing explanation | [admin:67][A38] |
| Bus 503 / 500 | Distinct busy/retry or daemon-fault/log advice | [admin:75][A38] |
| Other bus refusal | Problem: That was refused, raw response body, nothing-changed statement | [admin:87][A38], [main:302][M278] |
| Transport error through fail | 502 problem page, error detail, unreachable-daemon advice | [admin:38][A38] |
| Root listing failure | Plain 502 text, bypassing fail | [main:109][M85] |
| Feed/names failure | Scoped NoFeed/NoNames text; remainder of diagnostics remains | [main:119][M85] |
| Local parse/origin/action failures | Plain error responses at handler branches; no consistent form recovery | [admin:223][A223], [users:152][U152], [main:175][M175] |

GET problems offer Try again at the same URL. POST problems use a same-host
referrer as Back to the page when available. That recovers navigation, not
server-preserved input. Advice saying nothing changed is not a general proof
of mutation outcome after a transport failure.

## Existing data omitted or misplaced

These are presentation gaps against the existing public record/user/envelope
types, not requests for new telemetry.

| Data | Current omission or placement | Source |
|---|---|---|
| Record Kind and Descr | Missing from service/channel list; present on diagnostics registry, Descr in editor/read-only detail | [admin:329][A324], [main:451][M450] |
| Topic Mode | Used for subscription branch and creation, not named in existing detail/list readout | [admin:333][A324], [admin:340][A335] |
| Addr, Proto, Allow, NoMaster, TTL, Bound, Full | Visible as editable inputs only; authorized read-only visitor loses their values | [admin:343][A335] |
| In/Out and per-record losses | In/Out confined to registry/activity; loss section separated from primary list; detail omits In/Out | [main:451][M450], [admin:337][A335] |
| Envelope TTL, Expires, Wait, Deadline | Existing [envelope fields](../../../../src/internal/protocol/envelope.go#L13); Deadline affects Late calculation but actual deadline is not displayed | [views:188][V181], [views:252][V228] |
| Hidden group membership | Backend deliberately returns empty member arrays for non-maintainers; page cannot reinterpret them as measured empty membership | [core/manage:141](../../../../src/internal/core/manage.go#L141), [admin:360][A359] |
| Avatars | SVG initial route exists, but no current page requests it | [users:133][U133], [users:190][U190] |
| Process health, complete per-message dequeue trace | Not established by reader presence or aggregate counters; no invented green/complete state | [listing observations](../../../../docs/05-discovery.md#what-a-listing-answers), [retained evidence](../../../../docs/05-discovery.md#retained-exchanges) |

## Browser evidence

Local review artifacts are under `tmp/web-redesign-review/` (ignored scratch,
not permanent assets or deployment). Every listed image was captured in this
review and opened for inspection. The sign-in screen was also inspected directly;
its initial capture was not saved after a tool path/size failure.

| Captures | State checked |
|---|---|
| `services.jpg`, `channels.jpg` | Owner; populated lists and inline creation; wrong Channels nav highlight |
| `service.jpg`, `channel.jpg` | Owner; bounded queue with loss, configuration digest, all permitted forms; pub/sub subscriber |
| `users.jpg`, `person.jpg`, `new-user.jpg` | Owner; active/paused users, empty other-identity section, profile and creation |
| `groups.jpg`, `activity.jpg`, `diagnostics.jpg` | Owner; members, populated/zero series, queued work/loss/envelopes/fingerprint |
| `readonly-service.jpg`, `readonly-groups.jpg`, `readonly-person.jpg` | Ordinary user; external address omitted, group members unavailable, read-only own profile |
| `missing.jpg` | Missing/hidden-style service recovery |
| `mobile-services.jpg`, `form-error.jpg` | Narrow list with horizontal table scrolling; invalid-name submission replaced by JSON-bearing problem page with no input fields |

Not exercised in this browser fixture: credential-only cleanup cohort, every
refusal code, every receipt-correlation shape, a large directory, keyboard-only
completion, assistive technology, dark mode or a complete accessibility audit.
Source-backed branches above remain in the inventory with that limitation.

[M81]: ../../../../src/cmd/agent-bus-web/main.go#L81
[M85]: ../../../../src/cmd/agent-bus-web/main.go#L85
[M133]: ../../../../src/cmd/agent-bus-web/main.go#L133
[M166]: ../../../../src/cmd/agent-bus-web/main.go#L166
[M175]: ../../../../src/cmd/agent-bus-web/main.go#L175
[M211]: ../../../../src/cmd/agent-bus-web/main.go#L211
[M278]: ../../../../src/cmd/agent-bus-web/main.go#L278
[M372]: ../../../../src/cmd/agent-bus-web/main.go#L372
[M410]: ../../../../src/cmd/agent-bus-web/main.go#L410
[M432]: ../../../../src/cmd/agent-bus-web/main.go#L432
[M450]: ../../../../src/cmd/agent-bus-web/main.go#L450
[A38]: ../../../../src/cmd/agent-bus-web/admin.go#L38
[A100]: ../../../../src/cmd/agent-bus-web/admin.go#L100
[A128]: ../../../../src/cmd/agent-bus-web/admin.go#L128
[A166]: ../../../../src/cmd/agent-bus-web/admin.go#L166
[A194]: ../../../../src/cmd/agent-bus-web/admin.go#L194
[A209]: ../../../../src/cmd/agent-bus-web/admin.go#L209
[A223]: ../../../../src/cmd/agent-bus-web/admin.go#L223
[A241]: ../../../../src/cmd/agent-bus-web/admin.go#L241
[A305]: ../../../../src/cmd/agent-bus-web/admin.go#L305
[A324]: ../../../../src/cmd/agent-bus-web/admin.go#L324
[A335]: ../../../../src/cmd/agent-bus-web/admin.go#L335
[A359]: ../../../../src/cmd/agent-bus-web/admin.go#L359
[U33]: ../../../../src/cmd/agent-bus-web/users.go#L33
[U91]: ../../../../src/cmd/agent-bus-web/users.go#L91
[U112]: ../../../../src/cmd/agent-bus-web/users.go#L112
[U133]: ../../../../src/cmd/agent-bus-web/users.go#L133
[U152]: ../../../../src/cmd/agent-bus-web/users.go#L152
[U190]: ../../../../src/cmd/agent-bus-web/users.go#L190
[U214]: ../../../../src/cmd/agent-bus-web/users.go#L214
[U238]: ../../../../src/cmd/agent-bus-web/users.go#L238
[G19]: ../../../../src/cmd/agent-bus-web/activity.go#L19
[G45]: ../../../../src/cmd/agent-bus-web/activity.go#L45
[G71]: ../../../../src/cmd/agent-bus-web/activity.go#L71
[V51]: ../../../../src/cmd/agent-bus-web/views.go#L51
[V85]: ../../../../src/cmd/agent-bus-web/views.go#L85
[V110]: ../../../../src/cmd/agent-bus-web/views.go#L110
[V181]: ../../../../src/cmd/agent-bus-web/views.go#L181
[V228]: ../../../../src/cmd/agent-bus-web/views.go#L228
