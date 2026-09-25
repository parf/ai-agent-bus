# Cutover to the closed kind set

📌 **TL;DR:** Stop the daemon, rewrite `kind` in a copy of its snapshot, and
start the new binaries on that copy. Nothing is re-registered, so nothing a
record already carries can be overwritten. This is J.12; running it on the
live node is J.13 and has not been done.

This is [J.12](0.6.0-TODO.md#remaining-work). Running it on the live node is
J.13 and has not been done.

## Why the snapshot and not a re-registration

| | |
|---|---|
| Re-registering | overwrites the record's [allow list](../../docs/02-access.md#acl) with whatever the caller sends, which is the one property a migration most easily loses |
| Editing the snapshot | moves `kind` and touches nothing else. Owner, ACL, Maintainers, Personal, configuration and the queues are carried through as bytes |

A queue lives in the snapshot beside its record, not inside it, so a
re-registration would not restore one either: the inbox would have to be
rebuilt from a file anyway.

## What this version refuses to restore

The daemon names the record and stops
([records § restoring a record](../../docs/03-records.md#restoring-a-record)).
The supervisor restarts the bus, which fails the same way, so **the node stays
down until the snapshot is right** — which is why step 3 keeps an untouched
copy.

| Refused | Comes from |
|---|---|
| an unknown kind — `generic`, `topic` | the closed set |
| a service with no address or no protocol | a card with nothing on it |
| a service carrying TTL, capacity, overflow, the delivery switch or subscribers | a service has no queue here |
| a queue under a service name | the same |
| Personal on anything but an agent | [Personal](../../docs/03-records.md#personal-and-shared) |
| a secret on anything but a service | [secrets](../../docs/06-services.md#secrets) |

## The mapping

| Stored | Becomes | Decided by |
|---|---|---|
| `generic` with both `addr` and `protocol` | 📡 `service` | it is called somewhere else |
| `generic` otherwise | 👾 `agent` | something on this bus reads it |
| `agent` whose name is a person's own inbox | 👤 `user` | `owner == name` and a user profile exists |
| `agent` otherwise | 👾 `agent` | unchanged |
| `topic` with `mode: pubsub` | 📣 `pubsub` | the retired `mode` |
| `topic` with `mode: queue` or none | 📮 `queue` | the same |

`mode` is removed from every record. A record that becomes a 📡 also loses
`ttl`, `bound`, `overflow`, `disabled` and `subs`, **and its inbox must be
empty** — drain it first, or leave the record an agent.

The address/protocol test is a default, not an answer: a `generic` record with
an address that something on this bus does read is an agent. Decide each one
before running the transform and write the exceptions into it.

## The procedure

| | |
|---|---|
| 1 | `agent-bus ls > before.json` as the daemon owner. This is what step 7 compares against |
| 2 | `systemctl stop agent-busd`, so the snapshot is written by a graceful stop |
| 3 | `cp dump.json dump.before.json`. Untouched, and the rollback |
| 4 | Run the transform below: `dump.before.json` in, `dump.next.json` out |
| 5 | Install the new binaries ([setup § install](../../docs/09-setup.md#install)) |
| 6 | `cp dump.next.json dump.json && systemctl start agent-busd` |
| 7 | Verify — below |
| 8 | Rollback if it does not: stop, restore `dump.before.json` **and** the old binaries. The old daemon does not read a snapshot this one wrote |

<details>
<summary>The transform</summary>

```python
import json, sys

src, dst = sys.argv[1], sys.argv[2]
snap = json.load(open(src))
people = {u["name"] for u in snap.get("Users", [])}

for r in snap["Records"]:
    kind, mode = r.get("kind"), r.pop("mode", "")
    if kind == "topic":
        r["kind"] = "pubsub" if mode == "pubsub" else "queue"
    elif kind == "generic":
        r["kind"] = "service" if r.get("addr") and r.get("protocol") else "agent"
    if r["kind"] == "agent" and r["name"] in people and r["owner"] == r["name"]:
        r["kind"] = "user"
    if r["kind"] == "service":
        for queue_field in ("ttl", "bound", "overflow", "disabled", "subs"):
            r.pop(queue_field, None)

json.dump(snap, open(dst, "w"), indent=1)
```

It edits `kind`, removes `mode`, and copies every other field through. That is
the whole of why the six properties survive.

</details>

## Verifying

Six properties, each asked separately, against `before.json` and the running
daemon:

| Property | How |
|---|---|
| Owner | `owner` equal on every record |
| ACL | `allow` equal on every record |
| Maintainers | `maintainers` equal on every record |
| Personal | `personal` equal on every record |
| Configuration | read back **as the record it belongs to** — `agent-bus agent-template <name>` with that name's credential. A digest would call two spellings of the same bytes different |
| Queued work | `queued` on each record equals the messages its queue held |

And that the kinds actually moved: one of each mapping row, and no `mode`
left anywhere.

## What it was checked against

Checked 2026-09-19 on a snapshot written in the pre-enum shape — seven records
covering both topic modes, a `generic` with an address and one without, a
person's own inbox, a Personal agent, a record with Maintainers and a
configuration, and two queues holding messages.

| | |
|---|---|
| Before the transform | the new daemon refuses the snapshot, naming the record and the kind |
| A service left holding a queue | refused, naming the queue |
| After the transform | 14 checks pass: the six properties, the five mappings, and `mode` gone |
| Each property dropped from the transform in turn | its own check fails. Dropping `owner` also takes the records themselves, because a record whose owner the daemon does not know is [swept at start](../../docs/01-identity-and-roles.md#orphaned-records) |

Two of those came out of running it rather than writing it: a converted 📡
keeps `overflow` from the old record and is refused for it, and a 📡 whose
inbox still holds messages is refused as well.

## The live node, 2026-09-19

Run on the owner's instruction the same day, following the procedure above.

| | |
|---|---|
| Before | `agent-busd 0.5.84`, 11 records, **every one `agent`**, four queues drained, nothing this version refuses |
| The transform changed | four names and nothing else: `parf@parf`, `chief@srv1`, `plain@srv1`, `piped@srv1`, each a person's own inbox, `agent` → 👤 `user` |
| Checked before installing | the transformed file differs from the original in those four `kind` values alone — users, groups, accounts, daemon owner and all four queues byte-identical |
| After | `0.6.9` serving, 7 👾 and 4 👤, **zero differences** in Owner, ACL, Maintainers, Personal, Config and queued work; the accepted and dequeued counters came across (305, 393, 564, 10) |
| Faces | six of the seven agent faces reconnected by themselves; `opencode/oab@parf.us` has no reader and waits on its launcher |
| Rollback kept | `dump.before.json` beside the live snapshot |

The `@owner` cohort is unaffected by the four conversions: a caller that **is**
the owner matches before the kind is looked at, and a 👤 was never part of that
cohort anyway ([ACL](../../docs/02-access.md#acl)).
