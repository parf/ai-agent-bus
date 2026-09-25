# External services

📌 **TL;DR:** A 📡 `service` is a card saying where something outside is and how
to reach it; nothing on this bus answers for it. It is the external one of the
six record kinds, so it has no queue here and refuses everything that would
need one. Its address and protocol are required.

## Status

| MVP | Scope |
|---|---|
| Built | The [record](#what-a-service-is), its required address and [protocol](#how-to-call-it), the [refusals](#it-has-no-queue-here) that follow from having no queue, restore asking the same question, and [secrets](#secrets) as validated env files (0.7.8). |

## What a service is

`service` is one of the [record kinds](03-records.md#record-kinds),
and it is the **external** one. What runs behind a name on this bus is an
[agent](03-records.md#record-kinds); `service` keeps the word for the thing
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
instead ([records § record kinds](03-records.md#record-kinds)).

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
| a stored queue under its name | ignored and reported at [load](constitution.md#persistence-and-loading) |

## Secrets

Reaching an external thing usually needs a credential. A secret is that
credential, held on the record and handed to whoever its
[ACL](02-access.md#acl) already admits.

| | |
|---|---|
| `agent-bus secret <name>` | print it |
| `cat .env \| agent-bus secret <name> -` | set it, bytes on stdin |
| `agent-bus secret <name> 'TOKEN=abc'` | the same, inline |
| the web face's [service registration](web-face/records.md#register) | set it once, as the record is created |

One verb, and the direction is whether a secret was handed to it — the shape
[agent-template](03-records.md#configuring-a-template) uses. The read writes
the bytes to stdout exactly as stored, with no trailing newline: a credential
goes into a shell or an environment, and every byte added on the way is one
whatever uses it has to strip off again. Setting one answers with the record,
so what comes back is the digest and never what was just sent.

| Rule | |
|---|---|
| who may write it | the principals with [record management authority](01-identity-and-roles.md#groups), as for a configuration. A registration carries **neither half** — not the bytes, and not the digest — and a re-registration keeps the one already stored |
| who may read it | an 👾 agent's, that agent alone; a 📡 service's, whoever its [ACL](02-access.md#acl) admits, with no second list. Asked before the kind and before the secret exists, so a caller the list does not admit learns only that there is no such name |
| what it must be | an **env file**: each line blank, a `#` comment, or `KEY=value` with an optional `export `, `KEY` a shell identifier and a quoted value closed on its line. Nothing application-specific is checked. A refusal names the line and never repeats it; a stored secret that is not an env file is ignored and reported at load |
| which kinds hold one | 📡 `service`, 👾 `agent` and 👥 `group` — a group's read by its members — under the [private-value rule](constitution.md#-private-values). Every other kind is reached by sending to its name — refused at the verb, and a stored record holding one is ignored and reported at [load](constitution.md#persistence-and-loading). |
| what a query gets | **`secret_sha`**, a SHA-256 of the stored bytes, on every answer that carries a record, and on the service's own page. The bytes are on no listing, no record answer, no page and no log |
| nothing to store | an empty secret is refused, because it reads back exactly like never having set one |
| when it is acknowledged | once it is durable. A credential the caller was told was stored, and which a restart then loses, is worse than a refusal |
| what a form sends | the bytes that were typed. A browser submits a textarea with CRLF line endings whatever the page was served with, so the dashboard normalises them before the call; the daemon stores what it is sent and would otherwise keep a carriage return nobody typed |

The digest answers the same questions a configuration's does
([why a digest at all](03-records.md#why-a-digest-at-all)): whether a service
has a credential, whether a write landed, whether it has been rotated since,
and whether two hosts hold the same one.

**A secret is not [registry configuration](03-records.md#configuring-a-template).**
Both are private values absent from every listing and represented by a digest
in any ordinary answer, and their mechanics differ at every other point:

A secret is stored as written but must be an env file
([private values](constitution.md#-private-values)); there is no legacy
exception, so a non-conforming stored secret is ignored at load.

| | Configuration | Secret |
|---|---|---|
| Content | JSON, validated and compacted | an env file, checked for basic syntax and stored as written |
| Belongs to | an 👾 agent, a 📡 service or a 👥 group | the same three kinds |
| Who reads it | an agent itself (its owner is refused), or whoever a service's or group's [ACL](02-access.md#acl) admits | the same |
| What it is for | setup data that goes in and is used, not read back | a credential whose whole purpose is to be read back |

<details>
<summary>History: secrets before 0.7.8</summary>

Through 0.6 the daemon did not read inside a secret. `KEY=value`
lines are what callers agree to write, not a grammar anything checks: blank
lines, comments, `export`, duplicate keys and an invalid identifier are all the
caller's business, and a malformed secret is discovered by whatever uses it.
Only an empty secret is refused, because it reads back exactly like never having
set one. The 0.7 rule above, built in 0.7.8, replaced this content contract.

</details>

The rule that configuration never leaves the daemon for anyone but its own
record is unchanged by this.
