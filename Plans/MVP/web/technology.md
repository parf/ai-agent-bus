# Technology

Draft for peer review. Owner decisions are recorded in
[settled direction](README.md#settled-direction); this file holds the reasoning
and the consequences.

## Rendering

**`templ`**, adopted. Templates compile to typed Go components.

The gain that mattered to the decision is not ergonomics, it is enforcement. The
audited defects include a template asserting a consequence the daemon does not
have ([W05](../done/web-review.md#findings)), a page labelling three different
kinds of identity with one word ([W06](../done/web-review.md#findings)), and
tables without captions or scoped headers. A typed component layer turns a class
of those into compile errors: a data table that cannot be constructed without a
caption and scoped headers, a field that cannot be constructed without a label
and an error slot.

It is not protection against a semantic lie — codex's correction, and it is
right. A component can still be handed the wrong value with a confident label.
What it buys is that the *structural* obligations stop being discipline.

| | |
|---|---|
| Cost | One codegen step in `build.sh`, one dependency, one migration of nine templates |
| Not affected | The rendered output, the no-script rule, the process boundary, the wire |
| If refused | `html/template` with a strict partial contract. The design is identical; consistency becomes discipline rather than a guarantee |

**This was deliberately decided apart from the script question.** `templ` is
server-side rendering and buys no JavaScript. Both peers raised, independently,
that "modern Go" must not be allowed to smuggle in a relaxation of the script
rule, and the owner's two answers were taken separately for that reason.

## Script

None. The [no-script rule](../../../docs/05-discovery.md#rules-it-is-built-to)
stands unchanged, and no page, component or asset introduces one.

The reasoning that decided it, beyond the rule's own: not one of the seventeen
audit findings or sixteen inventory findings is fixed by partial DOM updates.
They are information architecture, form design, state language and data shape.
An enhancement layer would have bought motion on a dashboard whose problems are
all about what it says.

The declarative platform has also moved. Menus, disclosures and modals are
authorable without script since invoker commands reached cross-browser support
in December 2025 ([components](components.md#interaction-without-script)). The
2023 cost of this rule — losing those components — is largely gone.

## Routing and structure

| | |
|---|---|
| Router | `net/http` `ServeMux`. Method and path matching is sufficient; no router dependency |
| URLs | `?name=` for every resource. Names contain slashes — the majority of directory names do — and a path segment makes correctness depend on escaped-segment matching and unescaping agreeing across the mux, every hand-built href and every return-URL round trip. One rule for all resources |
| Assets | `embed`, served from this node. No CDN, no external fetch |
| Shape | Route → visitor-authenticated API calls → view preparation → component. Templates perform no I/O and no authorization |
| Data | Fetch only what the page needs. Obtain visible profile thumbnails with the already-authorized directory answer or one bounded batch read, never one daemon lookup per image ([W07](../done/web-review.md#findings)). GitHub metadata and photos must arrive through the daemon; templates do not call providers |
| Paging | Server-side over the authorized listing. It bounds HTML and browser work, not daemon response size; measure those separately before adding daemon-side querying |
| Cancellation | Bounded request timeouts and cancellation passed through the web/API boundary |

No database, no cross-user cache, no new wire design. Presentation problems are
solved in presentation.

The planned GitHub profile fields and photos are a data dependency, not a
presentation workaround: `protocol.User`, snapshot persistence, the trusted
directory result and an authorized thumbnail read must carry them before the
user page may show them. The trusted adapter downloads, validates and re-encodes
the GitHub photo or Gravatar fallback into a bounded local thumbnail. Neither
remote URL is emitted as an `<img src>`; the browser receives only local bytes.
Optional image failure writes no partial artifact, leaves enrolment available
and lets the UI fall back to a previous imported thumbnail or generated
initials. Byte and dimension bounds are implementation constants with tests at
both accepted and refused boundaries.

## Process boundary, unchanged

The web child remains the least trusted process and keeps no write path of its
own ([processes](../../../docs/11-processes.md#web-authority-boundary)). Forms
forward the visitor's session with an exact matching Origin. Responses are not
cached. Credentials and existing private configuration are never populated into
forms.

Two open confinement items are **not** closed by any of this and remain release
blockers: [G.1.2](../TODO.md#objective) resource confinement and
[G.1.3](../TODO.md#objective) credential and state isolation — the latter being
where codex's reproduced mapped-socket hazard belongs, in which a web child
pointed at an owner-mapped socket accepts any non-empty token. A redesigned page
does not close a confinement gate.

## What was rejected, and why

Recorded so it is not re-proposed.

| Rejected | Reason |
|---|---|
| htmx, Datastar, any enhancement layer | Owner decision. No audited finding is fixed by partial updates; inline handler attributes are script under another name; adoption is a one-way door for the templates |
| Themed admin templates (Tabler, CoreUI, AdminLTE and kin) | **Owner decision: a house layer.** The earlier rationale here — that their identity *is* JavaScript and CDN icon fonts — is withdrawn rather than caveated, because it is a universal claim we did not establish and Tabler is a counterexample: it documents CSS with optional JS and local hosting. codex's [S15](review/codex.md#specification-review-round-one). What remains, and is enough: the owner chose the house layer, and no matched-component comparison was run to price the alternative ([dissent](README.md#dissent)) |
| Carbon as a dependency | Components are JavaScript; only tokens and guidance transfer. The guidance is taken |
| Bulma as the base | Credible and documented as the fallback. Not chosen. Its own modal needs caller script, so it is not wholly script-free either |
| Salesforce Lightning | SLDS 2 coupled itself to the Salesforce platform and its component model; no longer the free-standing CSS system it was |
| Tailwind | Generated CSS is runtime-safe, but it supplies atoms rather than a table, a toolbar or a form pattern, and utility-soup templates are a real cost when templates are the review artifact |
| chi and other routers | Present route complexity is modest |
| A client-side data table | Ours is URL-driven, and that is the requirement. **Not** "every off-the-shelf table assumes client-side state" — a universal we did not survey and do not need: it is enough that a URL-driven table is what this dashboard requires and that we found none supplying it, so the component is hand-written either way. codex's [S15](review/codex.md#specification-review-round-one) |

**Typed components are a design choice, not a validator.** templ compiles a
component to a Go function whose parameters its *author* chooses, so a caption,
a label or an error slot is mandatory only if someone declares it mandatory.
Adopting templ does not make an omitted label a compile error; designing the
component API that way does. Recorded because the Q60 rationale read as though
the tool supplied the guarantee — codex's
[S15](review/codex.md#specification-review-round-one).

## Unverified, and owed before implementation

| | |
|---|---|
| A like-for-like comparison of the components we would actually use, across the house layer and at least one framework | codex's [dissent](README.md#dissent). The owner has decided; the comparison would still tell us what we gave up |
| Contrast measurement over the token pairs | the audited build shipped text at 3.54:1 because it was judged by eye |
| Browser bytes, requests and daemon calls, tracked separately | [F.13.6](../TODO.md#objective) |
