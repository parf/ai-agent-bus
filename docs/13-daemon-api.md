# Daemon API

📌 **TL;DR:** One HTTP surface, grouped below. This page names the routes and
says how a call is made: where it goes, what identifies the caller, and what
comes back. What each route means stays in the topic that owns it.

## Status

| MVP | Scope |
|---|---|
| Built | Every route below. |
| Pending | Nothing here. The routes follow their owning topics; this page has no contract of its own. |

## How a call is made

| | |
|---|---|
| Where | the shared socket, a mapped account's own socket, or the TCP listener ([where it listens](05-discovery.md#where-it-listens)) |
| Who is calling | the `X-Agent-Bus-Token` header, or the account socket in its place ([what a call carries](02-access.md#what-a-call-carries), [local socket](02-access.md#local-socket)) |
| Body | JSON on a `POST`; query parameters on a `GET` |
| Answer | JSON, and a refusal is `{"error": "..."}` with the status code and the counted reason that [refusals](05-discovery.md#refusals) owns |

One exception: `GET /secret` answers the stored bytes as `text/plain`, because
a credential goes into a shell or an environment and re-quoting it is a chance
to mangle it ([secrets](06-services.md#secrets)).

## Node

| Route | |
|---|---|
| `GET /identity` | who this node is, its release and its sampled call counts — **the one route needing no credential** ([what a node says about itself](05-discovery.md#what-a-node-says-about-itself)) |
| `GET /status` | totals and counters, and who the caller is and with what authority |
| `GET /recent` | the last exchanges, bodies struck out ([what it shows](05-discovery.md#what-it-shows)) |
| `GET /activity` | one name's history ([activity history](05-discovery.md#activity-history)) |

## Credentials

| Route | |
|---|---|
| `POST /token` | mint a credential for a name the caller is entitled to ([getting a token](02-access.md#getting-a-token)) |
| `POST /enrol` | claim a name in a realm a directory backs, by signature — **needs no credential, because it is where one comes from** ([proving possession](02-access.md#proving-possession)) |
| `POST /session` · `DELETE /session` | open and end a dashboard session ([signing in](05-discovery.md#signing-in)) |
| `GET /names` | the names the caller holds a credential for, with whose each one is and what it is for — a fingerprint, never the bytes ([what it shows](05-discovery.md#what-it-shows)) |

## Registry

| Route | |
|---|---|
| `POST /register` · `POST /unregister` | state a record, or remove an idle one ([registration](01-identity-and-roles.md#registration), [unregistering](01-identity-and-roles.md#unregistering)) |
| `POST /manage` | one record's description, address, protocol, queue settings, allow list, 📣 Deliver-To list, Maintainers, Personal, delivery switch and owner ([record authority](01-identity-and-roles.md#record-authority)) |
| `GET /ls` · `GET /lookup` | the records the caller may see, or one of them ([what a listing answers](05-discovery.md#what-a-listing-answers)) |

## Messaging

| Route | |
|---|---|
| `POST /send` | put a message in a name's queue ([inbox queues](04-messaging.md#inbox-queues)) |
| `GET /consume` | take the next one, waiting up to the caller's deadline ([push and pull](04-messaging.md#push-and-pull)) |

A reply and a receipt are ordinary sends ([request and reply](04-messaging.md#request-and-reply)).

## Channels

| Route | |
|---|---|
| `POST /subscribe` | the caller taking **itself** off a 📣 Deliver-To list; putting a name on is the manager's ([subscribers](04-messaging.md#subscribers)) |
| `POST /subscriber/remove` | whoever manages the channel taking somebody else off its Deliver-To list ([record authority](01-identity-and-roles.md#record-authority)) |

Publishing is `POST /send` to the channel; what it stamps on the message is
[the channel's own name](07-channels.md#what-publish-puts-on-the-message).

## Private data

| Route | |
|---|---|
| `POST /configure` · `GET /config` | write a record's configuration, or read it **as that record and no one else** ([configuring a template](03-records.md#configuring-a-template)) |
| `POST /secret` · `GET /secret` | write a 📡's credential, or read it as whoever its allow list admits ([secrets](06-services.md#secrets)) |

Neither leaves the daemon by any other route: every answer that carries a
record carries `config_sha` and `secret_sha` in their place.

## People, groups and accounts

Who may call these is [role names and scopes](01-identity-and-roles.md#role-names-and-scopes);
most need daemon administration.

| Route | |
|---|---|
| `GET /users` · `POST /user` | the people this node knows, and adding or editing one ([users and profiles](01-identity-and-roles.md#users-and-profiles)) |
| `POST /user/state` | `{"name", "status"}`, active or inactive ([user states](01-identity-and-roles.md#user-states)) |
| `GET /inactive` | the one read-only view of inactive records, for the web face ([record status](constitution.md#common-record-fields)) |
| `POST /user/github-refresh` · `POST /profile` | refresh a provider snapshot; edit the profile fields |
| `POST /identity/remove` | remove a person and what answered for them ([orphaned records](01-identity-and-roles.md#orphaned-records)) |
| `GET /groups` · `POST /group` | group membership ([groups](01-identity-and-roles.md#groups)) |
| `GET /accounts` · `POST /account` | the local account map, which needs a restart to take effect ([local socket](02-access.md#local-socket)) |
| `POST /owner` | transfer daemon ownership ([daemon owner](01-identity-and-roles.md#daemon-owner)) |

## Nothing else

The CLI, the MCP face and the dashboard are clients of exactly these routes and
have no private channel into the daemon ([faces](05-discovery.md#faces)). The
dashboard's own pages are served by a separate process
([processes](11-processes.md#the-processes)). The API's own root has no page: where a
dashboard address is configured, `GET /` permanently redirects to it, and
nothing below the root redirects at all, so a mistyped route stays a 404
([where it listens](05-discovery.md#where-it-listens)).
