# An agent's record is an Inbox — 0.5.84

📌 **TL;DR:** The owner read `/services` on a node whose every record is an
agent and said *they are no services*. The documents already agreed; the page
did not.

Owner-steered correction to the [service and channel
journeys](web-review.md#findings) built in 0.5.78. The contracts are [service
kinds](../../../docs/03-records.md#service-kinds), [inbox
queues](../../../docs/04-messaging.md#inbox-queues) and [display
labels](../../../docs/05-discovery.md#identity-labels-in-web-and-cli). No
requirement row closed here.

## What the owner asked

| Asked | Read as | Built |
|---|---|---|
| *"http://localhost:6780/services show user's queues - first they are no services !!"* | a record that serves nothing is not a service | agent records leave `/services` |
| *"at least - channels"* | file them with the queues they are | they are listed on `/channels`, which gains a Type column and a Kind filter |
| *"and Type should be "User" not an "Agent""*, answered and settled as **📥 Inbox** | the Type column must name the record, not repeat the name | `👾 Agent` becomes `📥 Inbox` in the web and the CLI |

**Why not the word User.** [Identities](../../../docs/01-identity-and-roles.md#identities)
already gives *User* one meaning — a registered person — and `claude/ab-dvp@parf.us`
is not one. Inbox names the record while Agent stays the principal that reads
it, so the two words stop competing for one row. The owner chose it.

## What the page was actually showing

Every registered **user** gets an implicit record of kind `agent` — their own
inbox — at `core/users.go` in `SetUser`. Agents register their own. So on the
live node all seven records are `kind: agent`, all owned by `parf@parf`, and
`/services` consisted entirely of queues while `/channels` was empty.

The contradiction was already written down: [inbox
queues](../../../docs/04-messaging.md#inbox-queues) opens *every registered
agent has its own queue — an implicit queue topic named after it*. A queue topic
listed under Services is the page disagreeing with the document, not a
preference about layout.

## Result

| Before | After |
|---|---|
| `/services` held `generic` and `agent`, with a Kind filter naming both | `/services` holds `generic`, and offers no filter for a page with one kind |
| `/channels` held `topic` alone | `/channels` holds `topic` and `agent`, with a **Type** column and a **Kind** filter |
| an agent record had no delivery mode and answered no mode filter | an inbox reads and filters as the queue it is, through one `deliveryMode` helper |
| `/services/new` offered Service or Agent | `/services/new` registers a service; `/channels/new` chooses channel or inbox |
| `👾 Agent` | `📥 Inbox`, in the shared `display` package, so web and CLI moved together |

`channelRecord(kind)` is the single predicate. Every branch that asked
`kind == protocol.KindTopic` to mean "belongs with the channels" now asks it:
the section a loaded record marks, the category counts, the page partition, the
detail return path, and the four action redirects.

**The stored kind did not change.** `agent` is still what the daemon holds and
still what the JSON answers; this is a display and placement decision, and no
registration, ACL or wire value moved. The smoke check that the JSON carries no
display glyph still passes.

**What a mode means for an inbox.** It stores none, and an absent mode is not a
third kind of delivery — it is the queue the agent reads. `deliveryMode` reads
it as a queue for the filter, and the create action drops the channel form's
delivery choice when the kind is not a topic, so a submitted radio cannot write
a mode onto an inbox.

## Checks

`src/smoke.sh --slow` green on the committed bytes. Five new live checks sit in
the web section: an agent's record is linked on `/channels`, its row reads
`📥 Inbox`, it carries no `/service` row link, a registered channel still reads
`Channel`, and the Inbox filter excludes the channels.

**17 mutations, 0 unaccounted.** Each is a compiling change to product code
paired with the named check it must break; the runner refuses a result credited
by `[build failed]` or by a test pattern that matched nothing. The list is
`tmp/scripts/inbox-mutations.py`.

| Mutation | Check that caught it |
|---|---|
| `channelRecord` forgets `agent`, three ways | the channel journey, the section counts, and the ordinary caller's own record |
| `channelRecord` returns true for everything | the service list |
| `deliveryMode` stops defaulting to queue | the channel journey's inbox row |
| `📥 Inbox` back to `👾 Agent`, label and glyph | the channel journey, the CLI listing, the directory |
| the title mark back to `👾` | the fixed title-mark categories |
| the Type column deleted from the channels table | the channel journey |
| the Inbox filter link deleted | the channel journey |
| the strip label and the node help back to Agents | the node strip |
| `recordKindPath` forgets `agent` | the account and user journeys |
| the services form registers an agent again | dedicated registration pages |
| the create action keeps the delivery choice | the inbox registration test |
| the channels help back to the Channel-only wording | list help popovers |

Two of those checks had to be written before they could catch anything: the
inbox registration test, and an owned inbox beside an owned service on one
user's page, since a route claim cannot be falsified where every record is the
same kind.

### Rendered, on a disposable node

Six records — two services, two inboxes (one with a slash in its name), a
pub/sub channel and a queue channel — read through headless Chromium at 1280 and
375, signed in as the node owner. `tmp/scripts/inbox-live.sh` builds the node,
`tmp/scripts/inbox-browser.ts` reads it.

| | `/services` | `/channels` |
|---|---|---|
| category count | All (2) | All (4) |
| Type column | `⚙️ Service` on both rows | `📥 Inbox` twice, `Channel` twice |
| delivery mode | — | `Queue · one at a time` for both inboxes, `Pub/sub · copy to each` for the pub/sub topic |
| row link | `/service?name=` | `/channel?name=`, including `claude%2ftwo%40srv1` |
| filter rows | Delivery, Reader, Queue | those three, plus **Kind** (All · Channel · 📥 Inbox) and **Delivery mode** |
| `?kind=agent` | — | the two inboxes, no channels |
| `?kind=topic` | — | the two channels, no inboxes |

`/service?name=bot@srv1` is titled `Inbox bot@srv1 · agent-bus` and carries the
`📥` title mark. The node strip reads `Services + Inboxes + Channels 6`, and
every figure computes to `text-align: right`.

**Nothing overflows loose at 375.** Every clipped element on Overview, Services
and Channels is inside a scrolling container — the navigation, a title mark, or
the record table — and the document itself does not scroll horizontally on any
of the three, the widest of which now has twelve columns.

| Width | page | clipped | inside a scroller | document scrolls |
|---|---|---|---|---|
| 375 | `/` | 12 | 12 | no |
| 375 | `/services` | 17 | 17 | no |
| 375 | `/channels` | 20 | 20 | no |

## Not implemented

- Whether `/channels` should be renamed now that it holds two kinds. It is
  still titled Channels and the menu entry is unchanged; the Type column says
  which row is which.
- The ACL help still reads *the Owner's Services and Agents*, and deliberately:
  an allow list admits **principals**, and an agent is a principal. Only the
  record it reads is an inbox.
- `agent-bus ls -h` still prints the raw word `topic` for a channel, because
  `display.Entity` has no case for it and the web supplies its own. Pre-existing
  and untouched here.
- The Kind filter reads `Channel` beside `📥 Inbox`: `entityLabel` gives a
  channel no glyph, because the Channels section's mark is a drawn SVG rather
  than an emoji. The asymmetry is visible and deliberate for now.
