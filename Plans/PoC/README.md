# PoC — agent-bus V2

What a developer must know to work on the PoC correctly. The active plan is
[TODO.md](TODO.md).

**[stages § PoC](../../docs/12-stages.md#poc) fixes the scope**; the rest of
[docs/](../../docs/00-overview.md) is the design of record and governs *how*
anything in that scope is built. Read it that way round, or persistence,
crypto and the runner arrive through the back door.

## Purpose

Prove that **two agent sessions find each other and talk** over a V2 daemon.
Nothing else has to be true. The PoC is allowed to be thrown away; it is not
allowed to lie about what it demonstrated.

Scope is fixed in [stages § PoC](../../docs/12-stages.md#poc) — read it before
adding anything here.

## Target shape

```
  claude session ──[ agent-bus mcp, channel push ]──┐
                                                    ├─→ agent-busd (Go)
  codex session  ──[ agent-bus mcp, app-server push ]┘    socket + HTTP
                     └─ codex app-server proxy (stdio)
```

**One TypeScript package, two push modes, one process per session.** V1's
`notifier-claude` is already an MCP server that also pushes, so this is its
shape, not a new idea; process placement is a runtime choice, not a layer
boundary ([modules § languages](../../docs/10-modules.md#languages)). Whether
Codex's MCP process can reach the App Server socket is the first spike — if it
cannot, the push mode becomes its own process and nothing else changes.

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | **new**; V1's protocol carries keys, trust and `event_hash` we do not have — take the identifier rules only |
| `mcp` face + both push modes | **TypeScript on bun** | **new**, all of it. V1 runs Claude's on bun and Codex's on Node — the WebSocket forced that, and the stdio proxy removes it |

Why the split: [modules § languages](../../docs/10-modules.md#languages).

## Contracts that hold even in PoC

These are the ones a shortcut would quietly break.

| Invariant | Where it is stated |
|---|---|
| A call carries **exactly two parameters** — `user@realm` and a token; the socket supplies them, it does not remove them, and a client off the socket supplies both itself | [access § two parameters](../../docs/02-access.md#two-parameters) |
| **The name is the identity.** No provider numeric id is ever a principal id | [identity § names](../../docs/01-identity.md#names) |
| A **reply matches on topic + tag**; the bus adds no call machinery | [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| **`ack` = got it, `done` = finished**, both emitted by the receiver | [messaging § receipts](../../docs/04-messaging.md#receipts) |
| **Dependencies point inward**; only adapters touch the outside world, and no verb exists only in a face | [modules § the rule](../../docs/10-modules.md#the-rule) |
| **An inbox has one reader**; a waiter filters by topic + tag and the daemon serves the match | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| **A transport ack is not "the model acted"** — only a correlated reply is | [runner § adapters](../../docs/08-runner-role.md#adapters) |
| **No external broker.** The daemon is the broker | [overview](../../docs/00-overview.md) |

## What PoC deliberately does not have

The list lives in [stages § PoC](../../docs/12-stages.md#poc). The one
consequence worth repeating: bodies are plaintext, so the daemon *can* read
them, and "the bus never reads payloads" only becomes true at MVP.

One master token per user reaches every service, and it is issued over SSH
because sshd does the authentication for free
([access § getting a token](../../docs/02-access.md#getting-a-token)).

## V1 is the bar

**Write the small version first.** V1's components are entangled with
JetStream's signed wires, `event_hash`, KV journals, `ADAPTER_PENDING`
recovery, delivery observations and a PHP handoff socket — none of which V2
has — so starting from them costs more than starting fresh.

**Then compare, and take V1's solution where it is better.** It ran in
production; every awkward branch in it is a case somebody actually hit. If
V1's approach is better, use it — the code included, when it comes free of the
JetStream machinery.

**Simplicity breaks the tie.** Complexity has to be paid for by a real case:
*"V1 does X"* is not a reason, *"V1 does X because Y happened"* is. Skipping a
V1 behaviour is fine; skipping it without noticing is not.

| Ours | Compare against | Look for |
|---|---|---|
| envelope, names | `libs/go/protocol` | which identifier rules turned out to matter |
| core verbs | `libs/go/app` | what send, consume and discovery had to handle in production |
| CLI | `cli/commands.go` | the flags a real operator needed |
| MCP face | `mcp/src` | the tool surface agents actually used |
| Claude push | `notifier-claude/server.ts` | MCP-server-plus-push in one process; reply correlation |
| Codex push | `notifier-codex/` | the App Server call sequence; idle vs busy, thread changes, disconnects, runtime rejection |
| the layer rules V2 inherited | `PRF-25/README.md` § Mandatory code layers | |

Code at `/rd/service/agent-bus/`, normative design at
`/rd/vhosts/realty/Plans/PRF-25/`. The PoC may be smaller than V1 — it may not
be worse at what it does.

## What V1 cost us, first hand

We have not only read V1 — we **used** it all through this PoC to talk to the
reviewer, and lost a message to it. That is the strongest evidence available
for what V2 is for, so it is written down as it happened, not as an opinion.

| What happened on V1 | Why | What V2 does instead |
|---|---|---|
| A review answer was accepted and **never delivered**: it was addressed to `agent-pub`, the *label* the sender had put in `from_channel`, and nothing consumes that name. `delivery: null`, no error, found an hour later by asking | a sender-chosen label is not an address, and the bus cannot tell the difference | **the name is the address.** A send to a name with no record is refused at `send` ([messaging § verbs](../../docs/04-messaging.md#verbs)); a reply goes to the `from` of an envelope, which is an inbox by construction |
| `accepted: true` came back anyway, and whether anything arrived lived in a second place — `agent-bus event status --event-hash …`, where `delivery` may be `null` for a message in flight, one that failed, or one evicted from a retention window | acceptance and delivery are separate journals, so the honest answer needs two queries and still cannot distinguish three states | **one state.** `send` queues into the receiver's inbox or fails; the envelope comes back. Whether a *model acted* is the receiver's own `ack` or reply, never the transport's ([receipts](../../docs/04-messaging.md#receipts)) |
| Sending one sentence took six flags and a JSON payload file: `--to-user`, `--channel`, `--from-channel`, `--source`, `--type`, `--payload-file` | channel, user, session, source and event type are five namespaces that all have to agree | `agent-bus send <name> "text"`. Two identity parameters, supplied by the socket locally ([access § two parameters](../../docs/02-access.md#two-parameters)) |
| Finding a peer meant three overlapping commands — `channel list`, `channel connected`, `agent-sessions` — whose answers differ in durable registration, live lease and named session | registration, liveness and naming are three registries | `agent-bus ls`: one registry, one record per name |
| `agent-bus help` answers `config_invalid: unknown command "help"`, and there are two binaries at two paths, one of which refuses to run without a credentials file | the CLI is a thin shell over a config loader that runs first | one binary, and `help` prints the verbs |
| **Channels are ephemeral: the channel dies and the results die with it** — work sent to a session that ends is never seen by anyone, ever | the address is the *connection*, so it cannot outlive the process holding it | **every registered name owns a queue, and the address outlives the process** ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)). Send to a service that has never run and the work waits; the process that turns up later under that name gets it. Checked: a message sent to `absent@srv1` before anything started is acked and answered when a service finally appears |

**Liveness is not the answer to "did anyone get it?" — the receipt is.**
V1 knows who is *connected* (`channel connected`: owner, host, pid, lease),
and V2 designs the same thing for later
([discovery § health checker](../../docs/05-discovery.md#health-checker)) —
but knowing a session was alive a second ago does not say your message was
picked up. **`ack` does**, and **`done`** says the work finished
([messaging § receipts](../../docs/04-messaging.md#receipts)): both come from
the receiver, which is the only party that knows. A journal in the transport
can say a byte stream was accepted and nothing more. So the sender's question
— *picked up, or lost?* — is answered end to end by a message, and it is
answered even when the receiver starts long after the send.

What liveness would still buy is knowing **before** sending, and a dashboard
that can show a queue nobody is draining. That is MVP's, measured against
V1's lease.

**How far "forever" goes in PoC:** as long as the daemon lives. The registry
and the queues are memory, and a restart clears both — the design already
dumps them to Parquet and reloads
([messaging § durability](../../docs/04-messaging.md#durability)), and PoC
leaves that out on purpose ([stages § PoC](../../docs/12-stages.md#poc)).

The lesson under all of it: **every one of those failures was silent.** The
rule V2 keeps is not "fewer features" but *refuse early and out loud* — an
unknown name, a mistyped topic, a third receipt value, an empty token file
are all errors at the point of the call, not discoveries later.

## Mutation first, then belief

A green check is evidence of nothing until it has been seen to fail. The
hollow ones keep the same few shapes, and every one of these has actually
been found here:

| Shape | The check that had it |
|---|---|
| taking the sender's word for a delivery | `ab_reply` said it replied; the routing was nonsense |
| counting nothing | a doubled delivery passed a check that only looked at the last one |
| grepping output that is non-empty either way | `$(cmd; echo -n nothing)` contains *nothing* whatever `cmd` did |
| grepping a word the success answer also contains | a refused receipt and an accepted one both say `receipt` |
| signalling the wrong process | `f() { ...; } &` backgrounds a subshell, so the service never got the signal |

So: **for every fix, break it again and watch the check go red.** The harness
that does it for a batch of fixes is a loop over copies of `src/` with one
edit each, running `src/smoke.sh` in every copy; it is worth rewriting per
review round rather than keeping, because the mutations are the interesting
part and they are never the same twice.

Two traps in the harness itself, both met: a copy that cannot bind its port
because an earlier run left a daemon behind fails *for the wrong reason* and
looks like a caught mutation; and a mutation that makes the whole run slow —
breaking the stop signal leaves every service waiting out its poll — hits the
timeout, which is not the same as the check failing. When either happens, run
that one check on its own against both builds.

## Working rules

- Task IDs are stable and never renumbered; use them in commits.
- One wave, one deliverable; commit at the end of each, push only when asked.
- A decision taken here lands in [decisions](../../docs/decisions.md) and in
  the doc that owns it — never only in the plan.
- **Before a task with a V1 counterpart closes**, compare the two: take V1's
  solution where it is better, and record what we skip and why. Simplicity
  wins a tie.
- What the PoC teaches that contradicts the design is a **doc edit**, not a
  note in this file.
