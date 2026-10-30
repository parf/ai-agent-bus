# External services

📌 **TL;DR:**
- **Problem:** something outside the bus (a web service, a database host) must be findable and nameable like everything else, without pretending the bus runs it.
- **Outcome:** a 📡 `service` is a card saying where something outside is and how to reach it; nothing on this bus answers for it. Its address and protocol are required. Like a 📚 resource card it has no queue here, and it refuses everything that would need one.
- **Start:** look at the card: `agent-bus ls <service>` ([Try this first](#try-this-first)).

## Why you'd want it

- 📡 **An outside service has no name on the bus.** Now a card gives it one, with where it is and how to reach it.
- 🙅 **A message to it would go nowhere.** Now the card refuses what needs a queue it does not have.
- 🔐 **Its secrets must not sit in the card.** Now they are kept apart ([secrets](#secrets)).

## What you get

- ✅ **A card:** address, protocol and owner for something external.
- ✅ **Honest refusals:** it has no queue here, and says so.
- 🚫 **Not for:** running a program on the bus; that is an [agent](03-records.md#record-kinds).

## Try this first

- Look at a service card (read-only):

```sh
agent-bus ls <service-name>
```

## Technical details

The reference below keeps every section as it was; other pages link to these sections.

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
| is | a description seen by whoever the allow list admits; its secret is its Owner's and Maintainers' |
| is not | a destination: nothing is sent to it, no Deliver-To list names it, nothing consumes from it |
| carries | `addr`, `protocol`, description, configuration and its [secret](#secrets) |
| carries no | queue, and therefore no TTL, capacity, overflow policy, delivery switch, reader count or backlog |

A registration that states no `kind` stores `service`, because describing
something outside is the case a bare `register` is usually for. That is also why
a bare `register` must carry an address and a protocol: the daemon refuses a
service without both.

```sh
agent-bus register mysql-prod@srv1 --service-addr host:3306 --protocol mysql
```

## How to call it

**Call it directly**, at `addr`, speaking `protocol`. The bus is not in the path
at all; the four kinds that have a queue are reached by sending to the name
instead ([records § record kinds](03-records.md#record-kinds)).

- A service's `addr` and `protocol` are the registration's own words, stored raw and **never interpreted**.
- The daemon does not implement a second protocol and does not proxy.
- They are facts in the registry for whoever is choosing what to call ([discovery § what a listing answers](05-discovery.md#what-a-listing-answers)).

**`/etc/services` is the suggested vocabulary for `protocol`, and only a
- suggestion.** Use the name from it where there is one, so two people registering the same kind of thing write the same word.
- It is not a checked set and cannot become one: the file is outdated and incomplete — half of what anyone registers here (`mcp`, `grpc`, an in-house protocol) is not in it, and refusing those would make the field useless to the people who need it most.

A thing outside that somebody wants reachable **over** the bus is not this case:
register an 👾 for whatever reads its queue, and let that agent call the outside
thing ([runner § adapters](06-launchers.md#adapters)).

## It has no queue here

Each of the three doors into a queue — send, Deliver-To, consume — refuses a
service by its kind, **after** the allow list has been asked, so a caller who may
not see the name learns only that.

| Asked of a service | Answer |
|---|---|
| send to it | refused, naming the address to call instead |
| put it on a 📣's Deliver-To list | refused: there is no queue for a copy to land in |
| consume from it | refused: there is no queue here to read |
| its reader count and backlog | **not stated.** A zero would be an observation of something that does not exist |
| TTL, capacity, overflow policy, delivery switch | refused at registration and at a settings edit |
| a stored queue under its name | ignored and reported at [load](constitution.md#persistence-and-loading) |

## Secrets

Keep secrets in Secret and config in Configuration; see [configuration](03-records-agent.md#configuring-a-template).
A secret used only by one runner can live in its host files; writes through the bus still carry values in a durable message ([two places for secrets](runner/security.md#secrets)).

Reaching an external thing usually needs a credential. A secret is that
credential, held on the record and read by the record's Owner and
Maintainers ([private values](constitution.md#-private-values)); its
[ACL](02-access.md#acl) grants use of the service, not its credential.

| | |
|---|---|
| `agent-bus secret <name>` | print it |
| `cat .env \| agent-bus secret <name> -` | set it, bytes on stdin |
| `agent-bus secret <name> 'TOKEN=abc'` | the same, inline |
| the web face's [service registration](../src/web/web-face/records.md#register) | set it once, as the record is created |

- One verb, and the direction is whether a secret was handed to it — the shape [config](03-records-agent.md#configuring-a-template) uses.
- The read writes the bytes to stdout exactly as stored, with no trailing newline: a credential goes into a shell or an environment, and every byte added on the way is one whatever uses it has to strip off again.
- Setting one answers with the record, so what comes back is the digest and never what was just sent.

| Rule | |
|---|---|
| who may write it | the record's Owner and Maintainers, as for a configuration. A registration carries **neither half** — not the bytes, and not the digest — and a re-registration keeps the one already stored |
| who may read it | the record's Owner and Maintainers, and an 👾 agent its own. A caller who sees the record and may not read it is told it is private; one who may not see it learns only that there is no such name |
| what it must be | an **env file**, in the [env-file syntax](#env-file-syntax). Nothing application-specific is checked. A refusal names the line and never repeats it; the record holding a stored secret that is not an env file is ignored and reported at load |
| which kinds hold one | 📡 `service`, 👾 `agent` and 👥 `group`, under the [private-value rule](constitution.md#-private-values). Every other kind is reached by sending to its name — refused at the verb, and a stored record holding one is ignored and reported at [load](constitution.md#persistence-and-loading). |
| what a query gets | **`secret_sha`**, a SHA-256 of the stored bytes, on every answer that carries a record to a caller who may read the secret, and on the service's own page; anyone else gets no digest, which for a short secret would be the secret. The bytes are on no listing, record answer, page or log; the web editor only replaces it ([Configuration and Secret](../src/web/web-face/records.md#private-configuration)) |
| nothing to store | an empty secret is refused, because it reads back exactly like never having set one |
| when it is acknowledged | once it is durable. A credential the caller was told was stored, and which a restart then loses, is worse than a refusal |
| what a form sends | the bytes that were typed. A browser submits a textarea with CRLF line endings whatever the page was served with, so the dashboard normalises them before the call; the daemon stores what it is sent and would otherwise keep a carriage return nobody typed |

The digest answers the same questions a configuration's does
([why a digest at all](03-records-agent.md#why-a-digest-at-all)): whether a service
has a credential, whether a write landed, whether it has been rotated since,
and whether two hosts hold the same one.

**A secret is not [registry configuration](03-records-agent.md#configuring-a-template).**
Listings omit both values and include digests only for authorized readers.
Configuration is displayed to Owner and Maintainers; Secret is never displayed.

A secret is stored as written but must be an env file
([private values](constitution.md#-private-values)); there is no legacy
exception, so a record holding a non-conforming stored secret is ignored at load.

| | Configuration | Secret |
|---|---|---|
| Content | JSON, validated and compacted | an env file, checked for basic syntax and stored as written |
| Belongs to | an 👾 agent, a 📡 service or a 👥 group | the same three kinds |
| Who reads it | the Owner and Maintainers, and an agent its own ([private values](constitution.md#-private-values)) | the same |
| What it is for | JSON settings shown to managers and passed to scripts | a credential whose whole purpose is to be read back |

See [configuration](03-records-agent.md#configuring-a-template) for access and the script environment.

### Env file syntax

- One syntax for every `KEY=value` list the bus and the runner read — a record's secret, the runner's `env.dist`, an agent's and an instance's `env` — and one parser for all of them, so a secret the daemon accepts is one the runner can deliver.
- The syntax is the official [dotenv](https://github.com/motdotla/dotenv#what-rules-does-the-parsing-engine-follow) format, read as its own parser reads it, with nothing evaluated (owner decisions Q171–Q173, 2026-10-17), and fast and simple: a reference fails at once rather than being carried as text (owner rule, 2026-10-18).

| A line | Means |
|---|---|
| blank, or `#` first | nothing |
| `KEY=value`, `export KEY=value`, `KEY: value` | `KEY` letters, digits, `_`, `.` and `-`; whitespace around `=` is allowed |
| an unquoted value | runs to a `#` or the line's end, trimmed at both ends, spaces inside kept; `KEY=` is the empty string. A `#` ends it with or without a space before it — `PASS=a#b` is `a` — so quote a value that holds one |
| a value in `'…'`, `"…"` or `` `…` `` | the quotes removed, whitespace inside kept, `#` inside no comment; it may span lines |
| in `"…"` only | `\n` and `\r` become a newline and a carriage return; any other backslash is kept as written |
| a key given twice | the later wins |
| `$NAME`, `${…}`, `$(…)` in an unquoted or `"…"` value | **refused**, by its line: dotenvx would expand it, and nothing here does, so every reference is an unknown one. A `$` followed by no letter, `_`, `{` or `(` is ordinary text (`5$`, `cost $5`) |
| the same in `'…'` or `` `…` `` | literal text: single quotes are dotenvx's way to keep a `$` |

Where the reference parser skips a line it cannot read, the bus refuses it, by
its line number, never repeating it. NUL bytes are also refused because a
process environment cannot carry them. There is no legacy form: a stored secret
or an env file on disk that does not fit is fixed, not carried.

Since 0.8.154, the daemon and runner use the same parser; the Slack agent's
TypeScript reader passes the same upstream conformance fixture. Stored text
stays as written; parsing removes quotes and decodes the escapes above for
process delivery.

Before deploying this check over an older database, audit every stored secret
and runner env file using the new parser in the stopped-home backup window.
Fix any non-conforming entry before the strict daemon starts: an invalid stored
record is ignored and reported at load, with its name and reason.
