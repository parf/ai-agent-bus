# Forms

Draft for peer review. Every form the dashboard offers, what it is for, and the
rules all of them obey.

## Rules

| | |
|---|---|
| One concern per form | A form asks one question. `/service` today is eight concerns in one page of always-open editors ([C02](review/codex.md#junk-and-misleading-content)) |
| The form posts as the person | The web child has no write path of its own. It forwards the visitor's session and requires an exact matching Origin ([rules](../../../docs/05-discovery.md#rules-it-is-built-to)) |
| The form's visibility is not the decision | The daemon authorizes at submission. A rendered control is a convenience, never a grant |
| Invalid input returns the form | With the values preserved, an error summary at the top, and each error tied to its field. Never raw JSON; never a bare problem page that loses what was typed ([C13](review/codex.md#junk-and-misleading-content)) |
| Never echo a secret | Not a token, not private configuration. The configuration field is always empty and `autocomplete=off` |
| Success returns to what changed | The section that changed, with a specific result. Not a list, and not `/services?scope=my`, which is where every service and channel action lands today ([C06](review/codex.md#junk-and-misleading-content)) |
| Only offer transitions that apply | An active user is offered Activate today, beside Pause and Ban, all styled alike ([C12](review/codex.md#junk-and-misleading-content)) |
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
| Filter a list | Services, Channels, Users, Activity | GET only; search, view, state, kind or mode, sort, page | the same list, filters in the URL |
| Register a service | `/services/new` | name, description, kind, allow | the new service's page |
| Register a channel | `/channels/new` | name, description, delivery mode, allow | the new channel's page |
| Edit metadata | Service, Channel | description, address, protocol | the identity section |
| Edit queue policy | Service, Channel | TTL, capacity, overflow | the queue section |
| Edit access | Service, Channel | one plain ACL term per textarea line; refuse master | the access section |
| Replace configuration | Service, Channel | configuration (always empty, never repopulated) | **Danger Zone** only; the configuration section then shows the new digest |
| Assign maintainers | Service, Channel | one named user, group, agent or service per textarea line | the identity section |
| Enable / Disable | Service, Channel | — | the identity section |
| Transfer ownership | Service, Channel | new owner | **Danger Zone** only; **confirm**, then the identity section |
| Remove registration | Service, Channel | — | **Danger Zone** only; **confirm**, then the list it came from |
| Subscribe / Unsubscribe | Channel, pub/sub | — | the subscribers section |
| Remove a subscriber | Channel, pub/sub | subscriber | the subscribers section |
| Register a group | `/groups/new` | name; one identity or nested group per textarea line | the new group's page |
| Edit members | Group | one identity or nested group per textarea line | the group's members section |
| Register a user | `/users/new` | name, person name, email, GitHub login | the new user's page |
| Edit profile | User | person name, email, GitHub login | the profile section |
| Refresh GitHub profile | User with a GitHub login | — | the same profile with provider fields and local photo refreshed |
| Change state | User | the applicable transitions only | the identity section; **confirm** for ban |
| Remove a credential | User, non-user identity | — | **confirm**, then the directory |

The current Service/Channel Delivery filters and User kind filter use links,
and Service Kind and Channel delivery mode use radios, as of 0.5.64. Other
small-choice conversions remain scoped to the page that owns them.

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
accepts `*`; Maintainers accepts named users, groups, agents and services. The
built Maintainers list is defined by the
[authority contract](../../../docs/01-identity-and-roles.md#services); the web
must not flatten it back into a single-group field.

**Danger Zone is a red text link, not a permanently open red panel.** It opens
a server-rendered resource subpage containing only Replace configuration,
Transfer ownership and Remove registration. Returning to the ordinary detail
page hides those forms again. Transfer and removal still use their separate,
fresh confirmation pages; the extra step changes presentation, not daemon
authorization.

This bounded form and confirmation flow is built in 0.5.63. Other Service and
Channel journey work in this proposal remains pending.

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

GitHub company, location, Twitter/X handle and the photo-source metadata are
provider facts, not form fields. They are imported by the trusted directory path
and rendered read-only; neither a user nor an Administrator can restate them
through the web form. The visible photo is the locally normalized thumbnail,
not the remote URL. GitHub's public email is different: it fills the existing
Email only when blank, after which the ordinary AgentBus email-editing rules
apply. There is no separate GitHub-email control.

Setting or changing GitHub login fetches the provider profile before the update
commits. A provider-profile failure returns the form with the old login and
metadata intact; optional photo failure falls back without refusing the login.
Saving unrelated profile fields performs no provider call. Clearing the login
clears provider metadata and the local photo but retains Person name and Email.

## Where the current forms stop making sense

| | |
|---|---|
| Eight concerns, one page, all editors open at once | Service detail |
| Every success leaves the page it acted on | all service and channel actions |
| A channel subscription answers by returning to Services | subscribe, unsubscribe, remove-subscriber |
| Both Subscribe and Unsubscribe offered regardless of state | channel detail |
| Activate offered to an already-active user | user detail |
| Removal offered with no confirmation step | service removal |
| An invalid name returns JSON and discards the description | register |
| Members editable only where they are visible, so an ordinary caller sees neither | groups |
