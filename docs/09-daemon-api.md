# Daemon API

📌 **TL;DR:**
- **Problem:** to build a client you need to know which routes exist and how every call is made.
- **Outcome:** one HTTP surface and the wire contract of its client part: the routes, how a call is made, and for the routes every client uses (send and consume, key-value, locks, bigfiles, status and identity, configuration) the fields, defaults, status codes and shapes. What a route means stays in the topic that owns it.
- **Start:** make one call through the command line and see the answer: `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔌 **A client needs the routes.** Now they are grouped by what they do.
- 🪪 **A call must say who it is.** Now each call carries a credential, checked before anything else.
- 📖 **The meaning of a route is elsewhere.** Now each group points to the topic that owns it.

## What you get

- ✅ **How a call is made:** where it goes, who is calling and what returns.
- ✅ **Routes in groups:** node, credentials, registry, messaging, channels, private data, locks, people.
- ✅ **A contract for the client routes:** fields, defaults, status codes and answer shapes, with [what is stable](#stability-and-compatibility) and what is not.
- ✅ **A closed list:** the page says what else does not exist.
- 🚫 **Not for:** the meaning of a route; each topic owns that.

## Try this first

Ask the bus for its status:

```sh
agent-bus status
```

The CLI makes the API call and prints the complete JSON response.

## How a call is made

| | |
|---|---|
| Where | the shared socket, a mapped account's own socket, or the [TCP listener](08-processes.md#the-tcp-listener), TLS once on ([remote access](02-access-remote.md#choosing-a-way)) |
| Who is calling | the `X-Agent-Bus-Token` header, or the account socket in its place; on the account socket, an owned Agent's token makes the call that Agent's ([what a call carries](02-access.md#what-a-call-carries), [local socket](02-access.md#local-socket)) |
| Body | JSON on a `POST`, which has 30 s to arrive or is answered `408`; raw bytes on a bigfile `PUT`, which has 30 s of silence and a [total budget](03-records-kv.md#bigfiles) instead; query parameters on a `GET` |
| Answer | JSON, and a refusal is `{"error": "...", "kind": "..."}` with the status code and the `kind` that [refusals](05-discovery.md#refusals) owns; a router rejection is a refusal too: `404` or `405` with kind `no_route` (`405` carries an `Allow` header) |
| Unknown fields | an unknown JSON member or query key is ignored on every route; a known one is validated, and a duplicate or invalid value is `malformed`; a body is exactly one JSON document, anything after it is `malformed` |

One exception: `GET /secret` answers the stored bytes as `text/plain`, because
a credential goes into a shell or an environment and re-quoting it is a chance
to mangle it ([secrets](03-records-service.md#secrets)).

## Node

| Route | |
|---|---|
| `GET /identity` | who this node is, its release and its sampled call counts — **needs no credential** ([what a node says about itself](05-discovery.md#what-a-node-says-about-itself)) |
| `GET /status` | totals and counters, and who the caller is and with what authority. The count of live records is `records`; a user record's owned records are also `records`; `realm` is the [bus setup](agents/bus.md#configuration)'s default realm |
| `GET /recent` | the last exchanges, bodies struck out ([what it shows](05-discovery.md#what-it-shows)) |
| `GET /activity` | one name's last day, or the caller's visible sum ([activity history](05-discovery.md#activity-history)) |
| `GET /activity/days` | a range of calendar days, `from` and `to` as yymmdd, at most 62; `totals=1` for each record's sums ([activity history](05-discovery.md#activity-history)) |

## Credentials

**Unstable, admin-only** except where [the client contract](#the-client-contract) lists the route: these may change in any release ([stability](#stability-and-compatibility)).

| Route | |
|---|---|
| `POST /token` | mint a credential for a name the caller is entitled to ([getting a token](02-access.md#getting-a-token)) |
| `POST /enrol` | claim a name in a realm a directory backs, by signature — **needs no credential, because it is where one comes from** ([proving possession](02-access.md#proving-possession)) |
| `POST /session` · `DELETE /session` | open and end a web-face session ([signing in](05-discovery.md#signing-in)) |
| `GET /names` | the names the caller holds a credential for, with whose each one is and what it is for — a fingerprint, never the bytes ([what it shows](05-discovery.md#what-it-shows)) |

## Registry

**Stable:** `GET /lookup` and `GET /ls` (read-only); **unstable, admin-only:** registering, managing and removing records, which the CLI, the web face and `agent-bus-admin` use.

| Route | |
|---|---|
| `POST /register` · `POST /unregister` | state a record, or remove an idle one ([registration](01-identity-and-authority.md#registration), [unregistering](01-identity-and-authority.md#unregistering)) |
| `POST /manage` | one record's description, address, protocol, queue settings, allow list, `deliver_to`, Maintainers, Personal, status and owner, each list whole or by `add` / `add_to_set` / `remove` deltas ([record authority](01-identity-and-authority.md#record-authority)) |
| `GET /ls` · `GET /lookup` | the records the caller may see, or one of them ([what a listing answers](05-discovery.md#what-a-listing-answers)) |
| `GET /manages?who=&name=` | `{"manages": true}` when `who` may manage the record by the rule `/manage` applies; asked only about a record the caller may see, so an agent decides for the sender it was handed without a copy of the rule ([record authority](01-identity-and-authority.md#record-authority)) |

## Messaging

| Route | |
|---|---|
| `GET /may-send?name=` | check the caller's [send permission](#checking-permission-to-send), without a message |
| `POST /send` | put a message in a name's queue ([inbox queues](04-messaging.md#inbox-queues)) |
| `GET /consume` | take the next one, waiting up to the caller's deadline ([push and pull](04-messaging.md#push-and-pull)); `topic`, `tag` and `original_to` [select](04-messaging.md#inbox-selection-and-filters) which; `host` states [where the reader is](05-discovery.md#where-a-pool-member-says-it-is); `asks=only` takes [only messages that carry an ask](04-messaging.md#an-asks-only-read) |

A reply and a receipt are ordinary sends ([request and reply](04-messaging.md#request-and-reply)).

### Checking permission to send

- `GET /may-send?name=%23systemd%40node.example` checks the authenticated caller's permission and answers `{"may_send":true}` when allowed.
- A refusal has the same HTTP status and error reason as `/send`'s target check.
- An ACL refusal stays `403`, including a name hidden from that caller; an inactive or absent target answers `404`.

- Both operations use one target-admission function: active name, destination ACL (including Personal), User-inbox rules and whether the record receives bus messages.
- There is no `from` option and no request body.
- The dry run queues no message and changes no message, refusal or activity counter.
- It checks permission now; it reserves nothing and does not validate a future message's fields, queue capacity or onward delivery.

**For another name.** `GET /may-send?who=<name>&name=<target>` answers by Send's
own rule for `who` instead of the caller, for an agent that sends for someone
else (cron, `#at`, a subscription). `who` is optional, given once and nonempty;
without it the caller's own form above is unchanged; an unknown key is ignored, as on every route, and a repeated `name` or `who` is `400`.

| Case | Answer |
|---|---|
| `who` may send to the target | `200` `{"may_send":true,"who":"<canonical who>"}` |
| `who` may not: refused by the ACL, Personal or User-inbox rule, an external kind, or `who` is unknown or inactive | `200` `{"may_send":false,"who":"<canonical who>"}`, one answer for each, with no reason |
| the caller cannot see the target (hidden, absent or inactive) | `404` `no such name`, the same for each, as [`/manages?who=`](#registry) |
| a malformed name, an empty or repeated `who` | `400` |

- The `who` echo is how a client knows the daemon read `who`: a daemon older than this form ignores it and answers for the caller, so a client refuses an answer whose `who` is missing or is not the one it asked about.
- A refusal is not an error and records no refusal counter, queue entry or activity.
- A `true` means a plain message to that name would be admitted now, nothing about queue capacity, TTL or what the receiver does, and it goes stale at once: ask again before each send, and still handle a refusal from the send.
- It is an answer about ACL membership that the caller could already read from the record's allow list, the same disclosure as `/manages?who=`; pass only the authenticated sender (the envelope's `from`), never a name taken from a message body.
- The clients are `Bus.may_send(who, name)` (Python) and `bus.maySend(who, name)` (TypeScript): a boolean, a `404` raised as the daemon's refusal, and everything else (transport, timeout, a foreign answer) raised, never `false`.

## Channels

| Route | |
|---|---|
| `POST /subscribe` | the caller taking **itself** off a 📣 Deliver-To list; putting a name on is the manager's ([Deliver-To](04-messaging.md#deliver-to)) |
| `POST /deliver-to/remove` | whoever manages the channel taking somebody else, `recipient`, off its Deliver-To list ([record authority](01-identity-and-authority.md#record-authority)) |

Publishing is `POST /send` to the channel; what it stamps on the message is
[the channel's own name](03-records-channel.md#what-publish-puts-on-the-message).

## Private data

Keep secrets in Secret and config in Configuration; see [configuration](03-records-agent.md#configuring-a-template).

| Route | |
|---|---|
| `POST /configure` · `GET /config` | write/read JSON settings as its **Owner or Maintainer**, or read an Agent's own configuration as that Agent; not a Secret, compact on save ([configuring a template](03-records-agent.md#configuring-a-template)) |
| `POST /secret` · `GET /secret` | write the secret of a 👾, 📡 or 👥, or read it under the [private-value rule](constitution.md#-private-values) ([secrets](03-records-service.md#secrets)) |

Neither leaves the daemon by any other route. An answer that carries a record
carries `config_sha` and `secret_sha` in their place only to a caller who may
read them ([private values](constitution.md#-private-values)); to anyone else
both are absent.

## Locks and key-value

Both live on a record and answer to its management authority.

| Route | |
|---|---|
| `POST /lock` · `POST /try-lock` · `POST /extend` · `POST /release` · `POST /release-force` · `GET /holders` | named locks on a record: wait for one, try once, lengthen its ttl, give it back, release another holder's, list holders ([shared locks](01-identity-and-authority.md#shared-locks)) |
| `GET /kv` · `GET /kv/list` · `POST /kv/set` · `POST /kv/delete` · `POST /kv/inc` · `POST /kv/json` | a record's key-value store: string, int and JSON values ([key-value store](03-records-kv.md#key-value-store)) |

## People, groups and accounts

**Unstable, admin-only:** every route below, `POST /records/export` and `POST /records/import` included, may change in any release and is not part of the client contract. Who may call these is [positions and scopes](01-identity-and-authority.md#positions-and-scopes); most need daemon administration.

| Route | |
|---|---|
| `GET /users` · `POST /user` | the people this node knows, and adding or editing one ([users and profiles](01-identity-and-authority.md#users-and-profiles)) |
| `POST /user/state` | `{"name", "status"}`, active or inactive ([user states](01-identity-and-authority.md#user-states)) |
| `POST /user/key` | add or replace a User's `ssh-ed25519` key in the daemon account's `authorized_keys` ([SSH admin](administration.md#ssh-admin)) |
| `GET /inactive` | the one read-only view of inactive records, for the web face ([record status](constitution.md#common-record-fields)) |
| `POST /user/github-refresh` · `POST /profile` | refresh a GitHub profile import; edit the profile fields |
| `POST /identity/remove` | remove a person and what answered for them ([orphaned records](01-identity-and-authority.md#orphaned-records)) |
| `GET /groups` · `POST /group` | group membership ([groups](01-identity-and-authority.md#groups)) |
| `GET /accounts` · `POST /account` | the local account map, which needs a restart to take effect ([local socket](02-access.md#local-socket)) |
| `POST /owner` | transfer daemon ownership ([daemon owner](01-identity-and-authority.md#daemon-owner)) |
| `GET /debug` · `POST /debug` | read or switch the debug log, `{"on": true}`; daemon Owner only ([logs](administration.md#logs)) |

## Stability and compatibility

| Class | Routes | Promise |
|---|---|---|
| **Stable** | the [client contract](#the-client-contract) below: `/send`, `/consume`, `/may-send`, `/kv*`, the lock routes, `/bigfile*`, `/status`, `/identity`, `/config`, `/lookup`, `/ls` | a change that breaks a conforming client is a MAJOR version |
| **Unstable, admin-only** | users, groups, accounts, profiles, keys, `/owner`, `/enrol`, `/token`, `/session`, `/names`, `/debug`, `/activity*`, `/recent`, `/records/export`, `/records/import`, `/register`, `/manage` and the other registry and credential routes | may change in any release; for the CLI, the web face and `agent-bus-admin`, not for third-party clients |

- **Version rule.** `GET /identity` carries `protocol`, an integer starting at 1, and `protocol_min`, the oldest client protocol the daemon serves (1 now). Before 1.0.0, a wire change amends protocol 1 and the shared vectors; both integers stay 1 ([pre-1.0 protocol decision](../Plans/R1.0-Release/DECISIONS.md#recorded-decisions)). From 1.0.0, any wire change raises the protocol integer; a breaking change raises `protocol_min`, and so the MAJOR version.
- **What a client does.** A client built for protocol P:
  - P below `protocol_min`: "daemon too new for this client", fatal at connect;
  - daemon `protocol` below P: a "daemon too old" warning at connect, not fatal (the MCP face also adds one note to its first tool result, so the model sees it), and a later refusal of kind `no_route` reports "daemon too old for <operation>";
  - otherwise it stays silent.
- **Missing or malformed.** An `/identity` with neither field is an old daemon: protocol 0, minimum 0, the warning above. Fields that are present but not integers, negative, or with `protocol_min` above `protocol` are a fatal protocol error naming the field.
- **Compatible changes** keep the protocol number's meaning: a new route, a new optional request field, a new answer field a client may ignore, a new value in an open list. A client must ignore answer fields it does not know.
- **Refusal kinds are a closed set:** `acl`, `authority`, `busy`, `credential`, `enrolment`, `full`, `malformed`, `name-taken`, `second-reader`, `suspended`, `unavailable`, `not_found`, `no_route`. A new kind is a breaking change. A `500` has no kind and a client says "internal daemon error".
- **Deprecation.** A route, field or kind is first documented as deprecated for at least one released MINOR version, and removed only in a MAJOR version. The removals made at the freeze, with no notice because nothing stable depended on them: `POST /subscriber/remove` and the `subscriber` fallback on `/deliver-to/remove` (use `recipient`), the `services` count in `/status` and the `services` list on a user (use `records`), and the web face's `/pubsub/topic*` and `/service-*` aliases. No CLI alias is removed.


### Every route and its class

One row for each route the daemon registers, read from every Go source. Two tests hold it:

- `TestEveryRegisteredRouteHasAContractRow` fails for a route with no row, a row for a route that is not registered, or a registration it cannot read as `HandleFunc("METHOD /path", …)`.
- `TestEveryStableRoutePathIsInTheClientContract` fails for a stable route whose whole path is missing from [the client contract](#the-client-contract).

| Route | Class | Where |
|---|---|---|
| `DELETE /session` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /accounts` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /activity` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /activity/days` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /bigfile` | stable | [Bigfiles](#bigfiles) |
| `GET /bigfile/list` | stable | [Bigfiles](#bigfiles) |
| `GET /config` | stable | [Configuration](#configuration) |
| `GET /consume` | stable | [Send and consume](#send-and-consume) |
| `GET /debug` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /groups` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /holders` | stable | [Locks](#locks) |
| `GET /identity` | stable | [Status and identity](#status-and-identity) |
| `GET /inactive` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /kv` | stable | [Key-value](#key-value) |
| `GET /kv/list` | stable | [Key-value](#key-value) |
| `GET /lookup` | stable | [what a listing answers](05-discovery.md#what-a-listing-answers) |
| `GET /ls` | stable | [what a listing answers](05-discovery.md#what-a-listing-answers) |
| `GET /manages` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /may-send` | stable | [Checking permission to send](#checking-permission-to-send) |
| `GET /names` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /recent` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /secret` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /status` | stable | [Status and identity](#status-and-identity) |
| `GET /users` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `GET /{$}` | unstable | browser redirect to the web face, not an API route |
| `POST /account` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /bigfile/delete` | stable | [Bigfiles](#bigfiles) |
| `POST /configure` | stable | [Configuration](#configuration) |
| `POST /debug` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /deliver-to/remove` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /enrol` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /extend` | stable | [Locks](#locks) |
| `POST /group` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /identity/remove` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /kv/delete` | stable | [Key-value](#key-value) |
| `POST /kv/inc` | stable | [Key-value](#key-value) |
| `POST /kv/json` | stable | [Key-value](#key-value) |
| `POST /kv/set` | stable | [Key-value](#key-value) |
| `POST /lock` | stable | [Locks](#locks) |
| `POST /manage` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /owner` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /profile` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /records/export` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /records/import` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /register` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /release` | stable | [Locks](#locks) |
| `POST /release-force` | stable | [Locks](#locks) |
| `POST /secret` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /send` | stable | [Send and consume](#send-and-consume) |
| `POST /session` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /subscribe` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /token` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /try-lock` | stable | [Locks](#locks) |
| `POST /unregister` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /user` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /user/github-refresh` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /user/key` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `POST /user/state` | unstable | admin-only, [People, groups and accounts](#people-groups-and-accounts) and the groups above |
| `PUT /bigfile` | stable | [Bigfiles](#bigfiles) |

## The client contract

Common to every route below:

- **Credential** in `X-Agent-Bus-Token`, or the account socket in its place; none is `401`.
- **Names** are canonical: a record name in a request is the name as [registered](01-identity-and-authority.md#names); an answer names the caller as `you` or `from`.
- **Bodies** are JSON on a `POST` of at most 1 MiB, with the one exception of a key-value write, which allows a 512 KiB value escaped; a body that does not arrive in 30 s is `408` (a bigfile `PUT` has 30 s of silence and a [total budget](03-records-kv.md#bigfiles) instead); `GET` carries query parameters.
- **Times** are RFC 3339 in UTC; **durations** are Go durations (`30s`, `5m`) except where a field says whole seconds.

### Send and consume

`POST /send` puts one message in a name's queue and answers the accepted envelope, `200`.

| Field | Who sets it | Meaning |
|---|---|---|
| `to`, `body` | the caller | destination name and text; `body` is empty for an `ask` |
| `topic`, `tag` | the caller | labels a receiver or a waiting reader filters on; both optional |
| `receipt` | the caller | `ack` or `done`, only from the receiver of the message `re` names; any other value is `400` |
| `re` | the caller | the `message_id` a receipt is about |
| `ask` | the caller | `help`, `help-config`, `help-secret`, `help-json`, `mcp` or `status`; the body must be empty and no receipt may accompany it |
| `reply_to` | the caller | `{name, topic?, tag?}`: where an answer goes when it is not the sender; `name` must be registered |
| `ttl` | the caller | how long the message is worth delivering, a duration; the receiver's bound caps it; invalid is `400` |
| `wait` | the caller | how long the caller will wait for an answer, a duration; travels as `deadline`; invalid is `400` |
| `message_id`, `from`, `at`, `roles`, `original_to`, `forwards`, `expires`, `deadline` | the daemon | `from` is the authenticated caller and replaces any value sent; `roles` are what the caller holds toward `to` and a caller stating any is refused; the rest are set at accept |

- **Refusals:** `400 malformed` (bad name, `ttl`, `wait`, `receipt` or `ask`), `401 credential`, `403 acl` or `suspended`, `404 not_found`, `429 full` when the receiver's queue is at its bound.
- **No other message routes:** an ack, a done and a reply are sends; there is no `/ack`, `/receipt` or `/call`.
- **Dry run:** `GET /may-send?name=<target>` (and `who=<name>` for another sender) answers `{"may_send": true|false}` by send's own target rule and queues nothing; its cases and refusals are [checking permission to send](#checking-permission-to-send).

`GET /consume` takes the next message and waits for one.

| Query | Default | Meaning |
|---|---|---|
| `inbox` | the caller, only when the key is absent | the queue to read; a key that is present is taken as the name, even when empty |
| `topic`, `tag`, `original_to` | none | select which message in the queue |
| `wait` | `30s`, at most `60s` | how long to wait; not a duration is `400` |
| `share` | absent | present, even `share=false`, says this reader is one of a pool; two readers of one inbox need both to say so |
| `host` | none | where the reader is, a hostname for the listing; not a hostname is `400` |
| `asks` | none | `only` takes only messages that carry an `ask`; any other value is `400` |

- **Answers:** `200` with the envelope; `204` with no body when nothing arrived in time, or the caller went away (the message goes back); `409 second-reader` for an incompatible reader; the refusals of send for the rest.
- **Delivery** is at most once: a message handed to a caller that then vanishes is requeued only if the connection closed first.

### Key-value

A record's store has three kinds, `string`, `int` and `json`; `kind` defaults to `string`; each kind is its own namespace.

| Route | Request | Answer |
|---|---|---|
| `GET /kv` | query `record`, `name`, `kind` | `{record, kind, name, value}`; a string that is not UTF-8 comes as `value_base64`; an `int` is a JSON number; `404 not_found` if absent |
| `POST /kv/set` | `{record, kind?, name, value \| value_base64, how?}`; `how` is `set` (default), `add` (only if absent) or `replace` (only if present) | `{record, kind, name}`; `add` on a held name is `412 name-taken`, `replace` on an absent one `404 not_found` |
| `POST /kv/delete` | `{record, kind?, name}` | `{record, kind, name, deleted}` |
| `POST /kv/inc` | `{record, name, n?}`, `n` a signed 64-bit integer, default 1 | `{record, kind:"int", name, value}` |
| `POST /kv/json` | `{record, name, ops}`, at most 100 operations applied all or none | `{record, name, results:[{changed, value?}]}` |
| `GET /kv/list` | query `record`, or none | with `record`: `{record, kind, values:{string,int,json}}`, each a list of `{name, size, preview \| preview_base64}`; without it, an array of `{record, kind, string, int, json}` counts |

- **Limits:** a name is 1 to 256 bytes of UTF-8, a value at most 512 KiB, a record at most 10,000 names of one kind; a limit is `413 malformed`. An `int` is a signed 64-bit integer, sent as its digits (never rounded); one that does not fit, or a value of the wrong type, is `400 malformed`.
- **Absent versus null:** an absent field is not a stored `null`; a JSON value may be `null` and is stored as such.
- **Authority:** the record's Owner, Maintainers and own Agent; anyone else is `403 acl`; an unknown record is `404 not_found`. Meaning and the JSON operations: [key-value store](03-records-kv.md#key-value-store).

### Locks

| Route | Request | Answer |
|---|---|---|
| `POST /lock` | `{record, name, ttl}` and query `wait` (0 to `60s`, default 0) | `{record, name, holder}` once granted; a lock held by another is `409 busy` naming the holder in `error`; one the caller already holds is `409 busy` ("use extend") |
| `POST /try-lock` | `{record, name, ttl}` | as `/lock` with no wait |
| `POST /extend` | `{record, name, ttl}` | `{record, name, holder}`; not the holder is `409 busy`, no lock is `404 not_found` |
| `POST /release` | `{record, name}` | `{record, name}`; held by another is `409 busy`; nobody holds it `404 not_found` |
| `POST /release-force` | `{record, name}` | `{record, name, holder}` naming the displaced holder; audited |
| `GET /holders` | query `record` | `{record, locks:{<name>:{holder, expires}}}`; with no `record`, an array of `{record, kind, name, holder, expires}` for every lock the caller may use |

- **Fields:** `ttl` is a Go duration above 0 and at most 24 h; a lock name is 1 to 256 bytes of UTF-8; a `wait` past `60s` or not a duration is `400`. The daemon's `wait` is immediate when omitted; the CLI defaults to `30s`.
- **Order:** waiters are granted in arrival order. A lock lives in memory and ends at its `expires`, with its record, or at a restart.
- **Authority:** a record's management authority, as for key-value. [Shared locks](01-identity-and-authority.md#shared-locks).

### Bigfiles

A bigfile is a named value up to 32 MiB held for a limited time. The bytes are a raw body, not JSON.

| Route | Request | Answer |
|---|---|---|
| `PUT /bigfile` | query `record`, `name`, `ttl` (whole seconds, 1 to 2,592,000, default 86,400), `memory` (`true` or `false`, default `false`); body the raw bytes; optional header `X-Bigfile-Sha256`, 64 lowercase hex digits | `200` `{record, name, size, sha256, held, expires, replaced}`; a hash that does not match the bytes is refused before anything is stored |
| `GET /bigfile` | query `record`, `name` | `200` the raw bytes as `application/octet-stream` with `Content-Length` and headers `X-Bigfile-Sha256`, `X-Bigfile-Held` (`file` or `memory`) and `X-Bigfile-Expires` |
| `GET /bigfile/list` | query `record` | `{record, keys:[{name, size, sha256, held, expires}]}` sorted by name, with no expired key and no uploader |
| `POST /bigfile/delete` | `{record, name}` | `{record, name, deleted}` |

- **Limits:** a name is 1 to 256 bytes of UTF-8; 32 MiB per key, 256 MiB and 100 keys per record, 1 GiB in the daemon; a put of no bytes is `400`; over a limit is `413 malformed`.
- **Refusals:** `403 acl` (put and delete need the Owner, a Maintainer or the record's own Agent; get and list also admit the allow list), `404 not_found` (absent or expired key, unknown record), `400 malformed` (bad `ttl`, `memory`, name or hash, an empty body), `503 unavailable` with the cause in `error` when the bigfile store cannot be opened.
- **Early refusal:** a refusal that comes before the body is read is sent after the daemon has read and discarded the declared bytes for at most two seconds, so a client still writing sees the status and its cause; a client may send `Expect: 100-continue` to see it before sending the body, and the connection is closed after such an answer.
- **A replace** is a `200` with `replaced: true`, never a `412`. A put never retries; a lost reply may leave the key stored, and the client reads the list to see.
- **Integrity:** the answer's `sha256` is that of the bytes stored, and a get's header is that of the bytes sent; a client checks both.

### Status and identity

| Route | Credential | Answer |
|---|---|---|
| `GET /identity` | none | `{version, build_info, owner, up, host, calls, protocol, protocol_min}`; `calls` is `{total, windows:[{window, observed, available, count}]}`; nothing else, and `Cache-Control: no-store` |
| `GET /status` | required | `{up, realm?, records, queued, waiting, dropped, expired, unclean?, refused?, kinds, owner_inactive?, you, administrator?, daemon_owner?, today, activity_days_kept}`; `refused` is a count per kind, only for kinds that happened |

- **`records`** counts live records; the older `services` count is removed.
- **A client** reads `you` for who the daemon says it is, and compares it with the name it expected.

### Configuration

| Route | Request | Answer |
|---|---|---|
| `GET /config` | query `name` | the record's stored Configuration as compact JSON, `null` when none; its Owner and Maintainers, or an Agent for itself; anyone else `403 acl` |
| `POST /configure` | `{name, config}`, `config` any JSON value | the redacted record; stored compact, with the same `config_sha` whatever its spacing |

- **Not a secret:** Configuration is public to those who may read it; a secret is a separate route and is not part of this contract. [Configuring a template](03-records-agent.md#configuring-a-template).

## Client capability matrix

| Capability | Python | TypeScript | Go | Rust | Java | PHP |
|---|---|---|---|---|---|---|
| send, consume, call | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| key-value | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| locks and holders | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bigfiles | ✅ | ✅ | ✅ | — | — | — |
| may-send | ✅ | ✅ | — | — | — | — |
| `asks` on consume | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| status, identity, configuration | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

- A dash is a capability the library does not have; the route still works for any HTTP caller.
- The six libraries share one conformance suite; its coverage is [protocol conformance](../src/clients/README.md#protocol-conformance).
- Event logs and subscriptions use plain send/call to `#bus`, not a separate library API. All six libraries run the shared event-log and event-subscription vectors with executed examples: [Go](../src/clients/go/README.md#event-logs), [Python](../src/clients/python/README.md#event-subscriptions), [TypeScript](../src/clients/typescript/README.md#event-subscriptions), [Rust](../src/clients/rust/README.md#event-subscriptions), [Java](../src/clients/java/README.md#event-logs) and [PHP](../src/clients/php/README.md#event-logs). The owning behavior is [agent logs](agents/bus.md#agent-logs).

## Nothing else

- The CLI, the MCP face and the web face are clients of exactly these routes and have no private channel into the daemon ([faces](05-discovery.md#faces)).
- The web face's pages are served by a separate process ([processes](08-processes.md#the-web-face)).
- The API's own root has no page: where a web-face address is configured, `GET /` permanently redirects to it, and nothing below the root redirects at all, so a mistyped route stays a 404 ([where it listens](05-discovery.md#where-it-listens)).

## Status

| MVP | Scope |
|---|---|
| Built | Every route above, and the frozen client contract: refusal `kind`, `protocol` 1 and `protocol_min` 1 in `/identity`, `no_route` for unknown routes, unknown fields ignored and duplicates refused, the retired aliases removed (0.9.264). |
| Built, with V1 | A check that every route has a row here and every stable route is in the client contract (the two tests named above), and shared conformance vectors for key-value, locks, bigfiles, identity and refusals, run by the Go, Python and TypeScript clients ([vectors](../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24)). |
| Built, with S2v | The Rust, PHP and Java adapters run the shared key-value, lock, identity and refusal vectors ([acceptance evidence](../Plans/R1.0-Release/DONE.md#t-24-s2v-the-rust-php-and-java-clients-run-the-shared-vectors)); bigfiles remain outside their client surface. |
