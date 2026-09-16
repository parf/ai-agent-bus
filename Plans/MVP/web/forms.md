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

## Consequential actions

Transfer, remove, ban, and remove-credential get a **server-rendered
confirmation page** naming the target and the consequence, and the daemon
rechecks authorization and current conditions at submission.

This is a rule and not a preference. Native `dialog` and the invoker-commands
API make a client-side confirmation easy to reach for, and it would quietly
remove the recheck — the state the confirmation described could have changed
between rendering it and acting on it. That is precisely the window class this
repository spent [0.5.31](../../../CHANGELOG.md) closing in the daemon; it must
not be reintroduced in the face.

The confirmation states what happens afterwards, and the wording is derived from
the current contract rather than copied between forms:

| Action | Consequence |
|---|---|
| Remove a service | The address goes and its credential goes with it; nothing answers to the name afterwards. A person's own credential stays — it is not a record's to drop ([unregistering](../../../docs/01-identity.md#unregistering)) |
| Transfer ownership | The new owner must be a registered principal who may act. Credentials already held are not revoked by a transfer |
| Ban a user | They keep their record and their services; they can do nothing until the state is lifted, and only the daemon owner can lift a ban |
| Remove an unused credential | The credential stops authenticating. It is removed because nothing answers for it |

Remove-a-service and transfer are the two whose help text was wrong in opposite
directions in the audited build; each now derives from its own contract section
and neither borrows the other's wording ([W05](../done/web-review.md#findings)).

## The set

| Form | Page | Fields | Result |
|---|---|---|---|
| Sign in | Sign in | token; validated local return | the page asked for |
| Sign out | shell | — | root |
| Filter a list | Services, Channels, Users, Activity | GET only; search, scope, state, kind or mode, sort, page | the same list, filters in the URL |
| Register a service | `/services/new` | name, description, kind, allow | the new service's page |
| Register a channel | `/channels/new` | name, description, delivery mode, allow | the new channel's page |
| Edit metadata | Service, Channel | description, address, protocol | the identity section |
| Edit queue policy | Service, Channel | TTL, capacity, overflow | the queue section |
| Edit access | Service, Channel | allow list, refuse master | the access section |
| Replace configuration | Service | configuration (always empty, never repopulated) | the configuration section, showing the new digest |
| Assign maintainers | Service, Channel | group, or none | the identity section |
| Enable / Disable | Service, Channel | — | the identity section |
| Transfer ownership | Service, Channel | new owner | **confirm**, then the identity section |
| Remove | Service, Channel | — | **confirm**, then the list it came from |
| Subscribe / Unsubscribe | Channel, pub/sub | — | the subscribers section |
| Remove a subscriber | Channel, pub/sub | subscriber | the subscribers section |
| Create a group | Groups | name, members | the new group's page |
| Edit members | Group | members | the group's members section |
| Create a user | Users | name, person name, email, GitHub login | the new user's page |
| Edit profile | User | person name, email, GitHub login | the profile section |
| Change state | User | the applicable transitions only | the identity section; **confirm** for ban |
| Remove a credential | User, non-user identity | — | **confirm**, then the directory |

No group-delete form. The handler accepts the action and no template renders it;
removing the verb from core and the API is [H.5.6](../TODO.md#objective).

No message composer. It needs a separately accepted body-handling workflow, and
this dashboard is for discovery, administration and envelope diagnostics.

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
| Maintainers assignment and transfer share a permission guard, so a maintainer who may manage cannot assign | service detail |
