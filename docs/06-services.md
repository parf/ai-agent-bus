# External services

📌 **TL;DR:** A 📡 `service` is a card saying where something outside is and how to reach it; nothing on this bus answers for it.

## Status

| MVP | Scope |
|---|---|
| Built | The [record](#what-a-service-is), its required address and [protocol](#how-to-call-it), the [refusals](#it-has-no-queue-here) that follow from having no queue, restore asking the same question, and [secrets](#secrets). |
| Pending | 0.7 [env-file validation](#secrets); the cutover inventory names existing nonconforming secrets, which must be fixed before activation. |

## What a service is

`service` is one of the [five record kinds](03-records.md#five-record-kinds),
and it is the **external** one. What runs behind a name on this bus is an
[agent](03-records.md#five-record-kinds); `service` keeps the word for the thing
a registration only describes.

The record is **information for the people and agents its
[allow list](02-access.md#acl) admits**: where the thing is, how to speak to it,
what it is for, and the [credential](#secrets) to use.

| A service | |
|---|---|
| is | a description read by whoever the allow list admits |
| is not | a destination: nothing is sent to it, nothing subscribes it, nothing consumes from it |
| carries | `addr`, `protocol`, description, configuration and its [secret](#secrets) |
| carries no | queue, and therefore no TTL, capacity, overflow policy, delivery switch, reader count or backlog |

A registration that states no `kind` stores `service`, because describing
something outside is the case a bare `register` is usually for. That is also why
a bare `register` must carry an address and a protocol: the daemon refuses a
service without both.

```sh
agent-bus register mysql-prod@srv1 --addr host:3306 --protocol mysql
```

## How to call it

**Call it directly**, at `addr`, speaking `protocol`. The bus is not in the path
at all; the four kinds that have a queue are reached by sending to the name
instead ([records § five record kinds](03-records.md#five-record-kinds)).

A service's `addr` and `protocol` are the registration's own words, stored raw
and **never interpreted**. The daemon does not implement a second protocol and
does not proxy. They are facts in the registry for whoever is choosing what to
call ([discovery § what a listing answers](05-discovery.md#what-a-listing-answers)).

**`/etc/services` is the suggested vocabulary for `protocol`, and only a
suggestion.** Use the name from it where there is one, so two people registering
the same kind of thing write the same word. It is not a checked set and cannot
become one: the file is outdated and incomplete — half of what anyone registers
here (`mcp`, `grpc`, an in-house protocol) is not in it, and refusing those would
make the field useless to the people who need it most.

A thing outside that somebody wants reachable **over** the bus is not this case:
register an 👾 for whatever reads its queue, and let that agent call the outside
thing ([runner § adapters](08-runner-role.md#adapters)).

## It has no queue here

Each of the three doors into a queue — send, subscribe, consume — refuses a
service by its kind, **after** the allow list has been asked, so a caller who may
not see the name learns only that.

| Asked of a service | Answer |
|---|---|
| send to it | refused, naming the address to call instead |
| subscribe it to a topic | refused: there is no queue for a copy to land in |
| consume from it | refused: there is no queue here to read |
| its reader count and backlog | **not stated.** A zero would be an observation of something that does not exist |
| TTL, capacity, overflow policy, delivery switch | refused at registration and at a settings edit |
| a snapshot holding a queue under its name | refused, as [an unknown kind is](03-records.md#restoring-a-record) |

## Secrets

Reaching an external thing usually needs a credential. A secret is that
credential, held on the record and handed to whoever its
[ACL](02-access.md#acl) already admits.

| | |
|---|---|
| `agent-bus secret <name>` | print it |
| `cat .env \| agent-bus secret <name> -` | set it, bytes on stdin |
| `agent-bus secret <name> 'TOKEN=abc'` | the same, inline |
| the dashboard's [service registration](05-discovery.md#compact-administration-pages) | set it once, as the record is created |

One verb, and the direction is whether a secret was handed to it — the shape
[agent-template](03-records.md#configuring-a-template) uses. The read writes
the bytes to stdout exactly as stored, with no trailing newline: a credential
goes into a shell or an environment, and every byte added on the way is one
whatever uses it has to strip off again. Setting one answers with the record,
so what comes back is the digest and never what was just sent.

| Rule | |
|---|---|
| who may write it | the principals with [record management authority](01-identity-and-roles.md#groups), as for a configuration. A registration carries **neither half** — not the bytes, and not the digest — and a re-registration keeps the one already stored |
| who may read it | whoever the record's [ACL](02-access.md#acl) admits, with no second list. Asked before the kind and before the secret exists, so a caller the list does not admit learns only that there is no such name |
| which kinds hold one | 📡 `service` alone. Every other kind is reached by sending to its name, so there is nothing outside for a credential to unlock — refused at the verb, and a snapshot holding one is refused at [restore](03-records.md#restoring-a-record) |
| what a query gets | **`secret_sha`**, a SHA-256 of the stored bytes, on every answer that carries a record, and on the service's own page. The bytes are on no listing, no record answer, no page and no log |
| nothing to store | an empty secret is refused, because it reads back exactly like never having set one |
| when it is acknowledged | once it is durable. A credential the caller was told was stored, and which a restart then loses, is worse than a refusal |
| what a form sends | the bytes that were typed. A browser submits a textarea with CRLF line endings whatever the page was served with, so the dashboard normalises them before the call; the daemon stores what it is sent and would otherwise keep a carriage return nobody typed |

The digest answers the same questions a configuration's does
([why a digest at all](03-records.md#why-a-digest-at-all)): whether a service
has a credential, whether a write landed, whether it has been rotated since,
and whether two hosts hold the same one.

**A secret is not [registry configuration](03-records.md#configuring-a-template).**
Through 0.6 they are the same shape — an opaque blob the daemon never reads
inside, absent from every listing, represented by a digest in any ordinary
answer — and their mechanics differ at every other point:

The table below describes behavior built through 0.6. **Pending for 0.7,** the
owner has replaced the opaque-content rule with
[basic env-file validation](constitution.md#-service). There is no legacy
exception: the cutover inventory names a nonconforming stored secret by Service
and digest, and activation waits for the operator to replace or remove it.

| | Configuration | Secret |
|---|---|---|
| Content | JSON, checked for being JSON | **opaque bytes**; `KEY=value` is the caller's convention and the daemon never parses it |
| Belongs to | any record configured from an [agent template](03-records.md#agent-templates) | a `service` record and no other kind |
| Who reads it | the named record alone, and its owner is refused too | whoever the record's [ACL](02-access.md#acl) admits; no second list |
| What it is for | setup data that goes in and is used, not read back | a credential whose whole purpose is to be read back |

**Built through 0.6, the daemon does not read inside a secret.** `KEY=value`
lines are what callers agree to write, not a grammar anything checks: blank
lines, comments, `export`, duplicate keys and an invalid identifier are all the
caller's business, and a malformed secret is discovered by whatever uses it.
Only an empty secret is refused, because it reads back exactly like never having
set one. The pending 0.7 rule above replaces this content contract.

The rule that configuration never leaves the daemon for anyone but its own
record is unchanged by this.
