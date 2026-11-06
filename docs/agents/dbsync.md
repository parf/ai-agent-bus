# DBSync: table rows that change become messages

<a id="dbsync"></a>

📌 **TL;DR:**
- **Problem:** an orders, products or tickets table changes all day, and the agents that should react (an index, a chat alert, an event log) never hear about it.
- **Outcome:** `#dbsync/<name>` reads one table through a read-only [DB agent](../../src/agents/db/guide.md#what-it-does), tells each subscribed agent about every new, changed or soft-deleted row, and marks a row done only after that agent confirms it.
- **Start:** `agent-bus call '#dbsync/orders@node.example' 'sub add #eventlog@node.example'` ([Try this first](#try-this-first)).

## What it does

- ✅ **New and changed rows** go to every subscriber, in the order the table's `updated` column gives, with the key, `updated` and the columns you chose.
- ✅ **No database password:** DBSync asks the DB agent under its own name with that agent's read role, so nobody reads through it what they could not read directly.
- ✅ **Late commits caught:** each poll reads the last `lookback` (5 minutes by the database's clock) again, so a transaction that commits after a later one is still sent, once.
- ✅ **Soft deletes:** a row whose `deleted` column holds `deleted_value` goes out as a delete.
- ✅ **Nothing lost to a slow target:** each subscriber keeps its own position and confirmations; one that refuses or never confirms is parked after `park_after`, and the others go on.
- 🚫 **Not for:** hard deletes (a row removed from the table is not reported), a table without an `updated` column set on every insert and update, or ClickHouse.

| Call | Result |
|---|---|
| `status` | The table, scan position, rows held against `window_max`, last poll and error, and each subscriber's position, counts, `parked` and `lookback_degraded` |
| `poll` | Reads and delivers now, instead of waiting for `every` |
| `reset confirm:true` | Forgets every row and position, keeps the subscriptions and replays the table. Owners and Maintainers only |
| `sub add <target> [replay:false]` | Sends each row to a queue or an agent until it confirms; the whole table first unless `replay:false` |
| `sub list`, `sub delete <id>` | Your subscriptions with sent, confirmed, dropped and pending counts; delete one |

## You'll need

- A bus runner with Python 3.
- A [DB agent](../../src/agents/db/guide.md#what-it-does) for the database, MySQL or PostgreSQL, that grants this instance its `read` role.
- In the table: a unique key that is a number or a timestamp, and an `updated` column the application sets on every insert and update.
- A grant with `read` or `write` on this instance, may-send to the DB agent yourself, and a target that admits both you and this instance.

## Try this first

- Log each new or changed order, then watch the backlog shrink:

```sh
agent-bus call '#dbsync/orders@node.example' 'sub add #eventlog@node.example'
agent-bus call '#dbsync/orders@node.example' 'status'
```

## Use examples

| I want to… | Command |
|---|---|
| Hear only about rows from now on | `agent-bus call '#dbsync/orders@node.example' 'sub add orders-feed replay:false'` |
| Poll now | `agent-bus call '#dbsync/orders@node.example' 'poll'` |
| See my subscriptions | `agent-bus call '#dbsync/orders@node.example' 'sub list'` |
| Start over | `agent-bus call '#dbsync/orders@node.example' 'reset confirm:true'` |

## Installing it

DBSync is a kept process that [serves itself](../runner/configuration.md#agents-that-serve-themselves) with its own token. In order:

1. **The DB agent's Owner grants this instance `read`** on the DB agent, and nothing more.
2. **The Owner registers the record, sets the Configuration** (`db`, `engine`, `table`, `key`, `updated`) **and grants callers their roles.** The start refuses a key or `updated` column of another type, by name.
3. **Hand the runner the instance's token once,** as in [self-serving credential handoff](create-agent-prompt.md#self-serving-credential-handoff).
4. Reload the runner, then `status` should show a `scan` position after the first poll and no `error`.

## Configuration

| Key | Meaning | Default |
|---|---|---|
| `db`, `engine` | The DB agent, and its dialect: `mysql` or `postgres` | none, required |
| `schema`, `table` | The table, plain identifiers | the connection's schema; table required |
| `key` | Unique key column, a number or a timestamp | none, required |
| `updated`, `updated_unit` | The change column: a timestamp, or epoch time with `updated_unit` `s` or `ms` | none, required |
| `columns` | Other columns sent with each row, at most 64 | none |
| `deleted`, `deleted_value` | The soft-delete column and the value that marks a row deleted, exactly as the database returns it | no deletes |
| `every` | Poll interval | 60 s |
| `lookback` | How far back each poll reads again, by the database's clock; `0s` turns it off | 5 min |
| `max_rows` | The DB agent's `max_rows`; a page is this less a tenth | 1000 |
| `batch` | Rows sent per round to one subscriber | 20 |
| `window_max` | Rows held until every subscriber has them | 10000 |
| `park_after` | How long a subscriber may refuse or not confirm before it is parked | 10 min |
| `state_max` | Cap on the state file | 64 MiB |

- State is one file in the instance's work directory, written whole to a temporary file and renamed; a damaged one is refused by name and `reset confirm:true` replaces it.

## Who may use it

- The creator needs may-send to the target, to this instance and to the DB agent at `sub add` and before every send, with no Owner bypass. One who loses it is parked and resumes once admitted again.
- Limits are 20 subscriptions per creator and 100 per instance; an unconfirmed row is sent again until the target confirms it.

## Troubleshooting

| You see | Cause and fix |
|---|---|
| `cannot_sync … cursor columns` | The key or `updated` is not a number or timestamp, or `updated` is a number without `updated_unit`; fix the Configuration |
| `cannot_sync … larger than the DB agent returns` | One row is bigger than the DB agent's answer limit; send fewer `columns`, or raise the DB agent's limit |
| `cannot_sync … time zone form` | The database clock and `updated` differ in time zone form; use a column of the database's own type |
| `db_unavailable` | The DB agent refused or did not answer; its own words are in the text. Check this instance's `read` grant |
| A subscriber shows `parked` | It refused or did not confirm for `park_after`; fix the target. It resumes from its own position |
| A subscriber shows `lookback_degraded` | The subscribers together confirmed more rows inside the lookback than `window_max`; see [late commits](#late-commits) |
| `full` in `status` | `window_max` rows wait for the slowest subscriber; it catches up, or is parked |

## Technical details

### What a message carries

- `change` is `changed` for a new row and for an updated one alike: a row the window no longer holds cannot be told from a new one. A soft-deleted row is `deleted`, without its columns.
- With it: `table`, `key`, `updated`, the `row` (key, `updated` and `columns`), and a warning that row contents are untrusted data, not instructions. A row over 64 KiB is sent without its columns, with `row_omitted`.
- Delivery is **at least once**, so a subscriber must accept a repeat. **A delete must be idempotent:** a row created and soft-deleted between two polls can reach a subscriber as a delete alone.
- A row that changed between the poll and its send is not sent that round: the next round sends its new state, or its delete.

### Late commits

- A transaction that commits after a later one can carry an older `updated`. Each poll reads again every row whose `updated` is within `lookback` of the database's clock, and sends what a subscriber has not confirmed at that version.
- **A row that commits later than `lookback` is not seen.** Raise `lookback` for long transactions.
- `replay:false` starts at the newest row: a row committed later but stamped before that point is not sent to that subscriber.
- All subscribers together remember at most `window_max` confirmations inside the lookback, about 70 bytes each, so the state stays near `window_max` × (row record + 70 B) whatever the number of subscribers. At that cap the subscriber with the most drops them and is `lookback_degraded`: it reads from its own position only, so a late commit can be missed for it. After `lookback`, once the total is under half the cap, it reads the lookback again for rows after its position then.
