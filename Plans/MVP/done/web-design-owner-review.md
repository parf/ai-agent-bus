# Web design owner review

📌 **TL;DR:** F.13.0 is complete through the owner's rendered-page selection
and iterative correction of every principal web journey.

## Evidence

The review was continuous rather than a one-time document sign-off. The owner
selected the visual structure of Option 2 and the operational content of Option
1, then inspected and corrected live or current-source pages throughout the
implementation. That work covered the shared frame, Services, Personal,
Channels, Activity, Service detail, registration forms, Users, User detail,
Groups, Group detail, Account, Diagnostics and narrow layouts.

Corrections were applied to the specification or built page before each slice
closed. They include compact help, title marks, navigation and registration
entry points, URL-backed filters, line-list editors, grouped numeric values,
My/Personal visual precedence, responsive tables, local profile photos,
ordinary editable provider-populated fields, linked Group and record names,
one-day Activity history and bounded detail graphs. The owner also explicitly
delegated remaining implementation choices for the overnight work.

The capability map stayed intact: later slices moved or compacted every
capability rather than deleting one. Each row below names where it is served
today, verified against the routes and templates in
`src/cmd/agent-bus-web/`.

| Capability | Current home | Note |
|---|---|---|
| Registry catalogue | `/services`, `/channels`, `/personal` | left Diagnostics in F.13.5; the record tables carry every fact that table uniquely exposed |
| Held work | `/services?work=held`, `/channels?work=held`, and Diagnostics § inboxes holding messages | the Overview Find links are these filters |
| Delivery counters | the Services and Channels tables (Queued or Held, Accepted, Dequeued) | cumulative across restarts, stated on the page |
| Activity history | `/activity` | bounded, process-local |
| Refusal reasons | `/diagnostics#refusals` | whole-node, by reason, closed reason set |
| Retained exchanges | Diagnostics § exchanges in retained history | envelope metadata only, never bodies |
| Loss | `/diagnostics#loss` | dropped and expired, caller-visible records |
| Credentials | `/account`, and `/credential-remove` for removal | caller-scoped |
| User lifecycle | `/user` § Access, and `/user-ban` for the ban confirmation | activate, pause, ban |
| Group impact | `/group` § used by visible records | names the record, its kind and how it uses the group |
| Settings, Allow and Maintainers | `/service` and `/channel` detail | separate forms; `@owner` is documented as ACL syntax |
| Private configuration | `/service-danger` § replace configuration | never displayed, only replaced |
| Ownership transfer | `/service-danger` § transfer, confirmed at `/service-confirm` | |
| Record removal | `/service-danger`, confirmed at `/service-confirm` | confirmation states queued count and outstanding reads |

Populated, empty, error and 375 px views were inspected during their owning
slices; each slice records its screenshots, behavioral checks and mutations
separately.

This closes the design-review gate. It does not substitute for F.13.6 whole-site
accessibility/performance/migration acceptance or F.12 installed multi-role
browser acceptance.
