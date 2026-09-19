# External services

📌 **TL;DR:** A 📡 `service` is a card saying where something outside is and how to reach it; nothing on this bus answers for it.

## Status

| MVP | Scope |
|---|---|
| Built | The [record](#what-a-service-is), its required address and [protocol](#how-to-call-it), the [refusals](#it-has-no-queue-here) that follow from having no queue, and restore asking the same question. |
| Pending | [Service secrets](#secrets), the last of [0.6.0](../Plans/MVP/0.6.0-TODO.md#objective). |

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

**Pending, planned in [0.6.0](../Plans/MVP/0.6.0-TODO.md#remaining-work).**
Reaching an external thing usually needs a credential. A secret is that
credential, held on the record and handed to whoever its
[ACL](02-access.md#acl) already admits.

**A secret is not [registry configuration](03-records.md#configuring-a-template).**
They are the same shape — an opaque blob the daemon never reads inside, absent
from every listing, represented by a digest in any ordinary answer — and their
mechanics differ at every other point:

| | Configuration | Secret |
|---|---|---|
| Content | JSON, checked for being JSON | shell `KEY=value` lines |
| Belongs to | any record configured from an [agent template](03-records.md#agent-templates) | a `service` record and no other kind |
| Who reads it | the named record alone, its owner included refused | whoever the record's [ACL](02-access.md#acl) admits; no second list |
| What it is for | setup data that goes in and is used, not read back | a credential whose whole purpose is to be read back |

The rule that configuration never leaves the daemon for anyone but its own
record is unchanged by this.
