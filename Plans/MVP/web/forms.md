# Forms

Draft for peer review. Every form the dashboard offers, what it is for, and the
rules all of them obey.

## Rules

| | |
|---|---|
| One concern per form | A form asks one question, and a Danger Zone journey is always its own ([C02](review/codex.md#junk-and-misleading-content)). **Record detail is the exception, on owner instruction 2026-09-19:** its settings, access, classification and Maintainers are one editor, because they are one decision about one record and splitting them put the same allow list behind two doors |
| A control the caller may not use is disabled, not hidden | Owner instruction 2026-09-19. A hidden field makes a page's shape depend on who is reading it; a disabled one shows what the record has and says who may change it. A disabled control submits nothing, so a form that may change such a field states so in a hidden flag of its own, and the daemon still decides ([web authority](../../../docs/11-processes.md#web-authority-boundary)) |
| The form posts as the person | The web child has no write path of its own. It forwards the visitor's session and requires an exact matching Origin ([rules](../../../docs/05-discovery.md#rules-it-is-built-to)) |
| The form's visibility is not the decision | The daemon authorizes at submission. A rendered control is a convenience, never a grant |
| Invalid input returns the form | With the values preserved, an error summary at the top, and each error tied to its field. Never raw JSON; never a bare problem page that loses what was typed ([C13](review/codex.md#junk-and-misleading-content)) |
| Never echo a secret | Not a token, not private configuration, not a service secret. The configuration and secret fields are always empty and `autocomplete=off`, including after a refusal |
| A multi-line value is the bytes that were typed | A browser submits a textarea with CRLF whatever the page was served with. Where the daemon stores bytes as sent — a [service secret](../../../docs/06-services.md#secrets) — the face normalises them back, or a two-line credential is stored with a carriage return nobody typed |
| A form asks only what its kind has | `/channels/new` has no fields of its own: a queue declares TTL, capacity and overflow, a pub/sub topic holds nothing and declares none of it, so the section offers two forms and the page without a kind links to both |
| Success returns to what changed | The section that changed, with a specific result. Service and Channel registration and ordinary edits return to the affected resource; removal returns to the matching collection ([C06](review/codex.md#junk-and-misleading-content)) |
| Only offer transitions that apply | Built in 0.5.79: active offers Pause/Ban, paused offers Activate/Ban and banned offers Activate only when daemon-returned authority permits it |
| A failed transport promises nothing | "Nothing was changed" is not knowable when the request did not complete |
| Show small choices | Two or three values use labelled radio buttons, not a select. URL-backed filters use the [link/button rule](components.md#small-choice-controls) instead |
| One name per line | ACL and Maintainers use the same line-list textarea component with plain terms, never glyphs or comma-separated chips. Errors identify the refused line and preserve all submitted lines |

## Consequential actions

Transfer, remove, ban, and remove-credential get a **server-rendered
confirmation page** naming the target and the consequence, and the daemon
rechecks authorization and current conditions at submission.

This is a rule and not a preference, but **not for the reason I first gave.**
codex's correction: the daemon rechecks at the final submission whichever way
the confirmation was drawn, so a client-side dialog does not remove that
recheck. My sentence claiming it did was wrong.

What a server-rendered confirmation actually buys is that **the consequence it
describes is computed when it is shown.** A dialog carried in the page states
conditions as they were when that page was rendered, which may be minutes or
hours stale; a fetched confirmation re-reads them. The person is then agreeing
to something true rather than to something remembered. The daemon's authority
check is unaffected either way and is what actually protects the operation.

The confirmation states what happens afterwards, and the wording is derived from
the current contract rather than copied between forms:

| Action | Consequence |
|---|---|
| Remove a service | The address goes and its credential goes with it; nothing answers to the name afterwards. A person's own credential stays — it is not a record's to drop ([unregistering](../../../docs/01-identity-and-roles.md#unregistering)) |
| Transfer ownership | The new owner must be a registered principal who may act. Credentials already held are not revoked by a transfer |
| Ban a user | They keep their record and their services; they can do nothing until the state is lifted. Administrators may lift an ordinary user's ban; the daemon Owner controls protected authority levels |
| Remove an unused credential | The credential stops authenticating. It is removed because nothing answers for it |

Remove-a-service and transfer are the two whose help text was wrong in opposite
directions in the audited build; each now derives from its own contract section
and neither borrows the other's wording ([W05](../done/web-review.md#findings)).

Ban and unused-credential removal use the same server-rendered confirmation
rule as of 0.5.79. The final request still relies on daemon authorization.

## When the recheck refuses after you confirmed

Missing from my first draft; opencode's find. The daemon can refuse *after* the
person confirmed, because what they confirmed stopped being true — the canonical
case being a credential removal answered with *now backed by a user, record or
owned service; refresh the directory*.

**This is for a fact that changed, and only that.** A refusal can also arrive
because the face never had the fact in the first place — the worked case being a
filtered waiter, which blocks removal and is invisible to the field that reports
readers ([layouts](layouts.md#a-consequential-confirmation)). Nothing changed
there: the condition held before the confirmation was drawn and holds still. It
is the ordinary current-state refusal, carrying the daemon's own message. The
final recheck covers stale facts and incomplete facts alike, but they are
different things to be told, and calling an observability gap a race is a false
statement dressed as a helpful one — codex.

That is not an ordinary refusal and must not land in the generic one, where it
reads as a bug. It gets its own presentation: **the conditions changed, here is
what is true now**, with the current state shown and the action offered again if
it still applies. The person did nothing wrong and the system did nothing wrong;
the world moved between the question and the answer.

## The set

| Form | Page | Fields | Result |
|---|---|---|---|
| Sign in | Sign in | token; validated local return | the page asked for |
| Sign out | shell | — | root |
| Filter a list | Agents, Services, Channels, Users, Activity | GET only; search, view, state, kind, sort, page | the same list, filters in the URL |
| Register an agent | `/agents/new` | name, description, allow | the new agent's page |
| Register a service | `/services/new` | name, description, address, protocol, allow, secret (optional). The secret is a second call to its own verb, and is never repopulated | the new service's page |
| Register a queue | `/channels/new?kind=queue` | name, description, allow, TTL, capacity, overflow | the new queue's page |
| Register a pub/sub topic | `/channels/new?kind=pubsub` | name, description, allow. It keeps nothing, so it declares no queue policy | the new topic's page |
| Edit settings | Agent, Service, Channel | description; address and protocol on a 📡 only; TTL, capacity and overflow on everything with a queue; one plain ACL term per textarea line, `@owner` plain syntax; Personal on a 👾; Maintainers. The last two are disabled unless the caller is the Owner or a daemon administrator | the identity section |
| Replace configuration | Service, Channel | configuration (always empty, never repopulated) | **Danger Zone** only; the configuration section then shows the new digest |
| Enable / Disable | Service, Channel | — | the identity section |
| Transfer ownership | Service, Channel | new owner | **Danger Zone** only; **confirm**, then the identity section |
| Remove registration | Service, Channel | — | **Danger Zone** only; **confirm**, then the list it came from |
| Subscribe / Unsubscribe | Channel, pub/sub | — | the subscribers section |
| Remove a subscriber | Channel, pub/sub | subscriber | the subscribers section |
| Register a group | `/groups/new` | name; one identity or nested group per textarea line | the new group's page |
| Edit members | Group | one identity or nested group per textarea line | the group's members section |
| Register a user | `/users/new` | name, person name, email, GitHub login, company, location, Twitter/X | the new user's page |
| Edit profile | User | person name, email, GitHub login, company, location, Twitter/X | the profile section |
| Change state | User | the applicable transitions only | the identity section; **confirm** for ban |
| Remove a credential | User, non-user identity | — | **confirm**, then the directory |

The current work and Readers filters and the User kind filter use links, and
the Channel kind choice uses radios, as of 0.5.64. Other
small-choice conversions remain scoped to the page that owns them. A marked
selector submits on change through the repository-owned script and exposes an
Apply button through `noscript`; query values remain plain URL state.

**Configuration stays on channels.** The draft gave Replace configuration to
Service only; the shared detail template already offers it on any managed record
including channels (admin.go:354), so restricting it here would have removed a
working capability as a side effect of a documentation split. Removing it would
need its own decision — codex's
[S07](review/codex.md#specification-review-round-one).

**Subscribe and unsubscribe are not management.** They sit outside `CanManage`
in the build (admin.go:341) and stay outside it here: adding a subscription is
the subscriber's own opt-in, while *removing someone else's* is a manager's
action. Two different authorities in one section, named separately
([S06](review/codex.md#specification-review-round-one)).

ACL and Maintainers share the same textarea and line handling. ACL additionally
accepts `*` and runtime `@owner`; Maintainers accepts named users, groups,
agents and services but never `@owner`. The
built Maintainers list is defined by the
[authority contract](../../../docs/01-identity-and-roles.md#record-authority); the web
must not flatten it back into a single-group field.

**Danger Zone is a red text link, not a permanently open red panel.** It opens
a server-rendered resource subpage containing only Replace configuration,
Transfer ownership and Remove registration. Returning to the ordinary detail
page hides those forms again. Transfer and removal still use their separate,
fresh confirmation pages; the extra step changes presentation, not daemon
authorization.

This bounded form and confirmation flow is built in 0.5.63. The populated
Service and Channel journeys, including resource-specific returns, are complete
in 0.5.78.

The shared invalid-input return is built in 0.5.70 for current Service,
Channel, User and Group forms. It preserves only allowlisted nonsensitive
values, keeps private configuration empty, shows an alert summary and marks a
field invalid only when the face can attribute it without interpreting daemon
prose. The planned split of the remaining combined Service/Channel editors is
still pending.

No group-delete form. The handler accepts the action and no template renders it;
removing the verb from core and the API is [H.5.6](../TODO.md#objective).

No message composer. It needs a separately accepted body-handling workflow, and
this dashboard is for discovery, administration and envelope diagnostics.

Company, Location and Twitter/X are editable AgentBus User fields. GitHub may
fill them when its login is set or changed. **The web form no longer offers a
refresh control**: the owner removed it at 0.5.83, so setting or changing the
login is the one thing on this page that contacts the provider. `POST
/user/github-refresh` is unchanged for an authorized caller. Provider provenance, remote image facts and local
photo bytes are never form fields. The visible photo is the locally normalized
thumbnail, not the remote URL. GitHub's public email fills the existing Email
only when blank. There is no separate GitHub-email control.

Setting or changing GitHub login attempts to fetch the provider profile. A
provider-profile failure still saves the valid unique login; explicit Refresh
reports a failure without mutation. Optional
photo failure falls back without refusing the login. Saving unrelated profile
fields performs no provider call. Clearing the login clears provider provenance
and the local photo but retains ordinary User fields.

## Remaining form work

| | |
|---|---|
| Both Subscribe and Unsubscribe offered regardless of state | channel detail |
| Activate offered to an already-active user | user detail |
| Members editable only where they are visible, so an ordinary caller sees neither | groups |
