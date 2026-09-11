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
  claude session ──[ agent-bus mcp + channel push, one process ]──┐
                                                                  ├─→ agent-busd (Go)
  codex session  ──[ agent-bus mcp (push off) ]───────────────────┘    socket + HTTP
       └─ codex app-server on ws://127.0.0.1:PORT ←─[ pusher, same name ]
```

**One TypeScript package, two push modes** — and they land differently,
which the spike settled rather than guessed:

| | |
|---|---|
| Claude | **one process**: the MCP server pushes into its own session through a channel, exactly V1's `notifier-claude` shape |
| Codex | **two processes, one `AGENT_BUS_NAME`**: the MCP server inside the session with push off, and a pusher beside it holding the App Server connection. A session cannot push into itself through its own stdio |

Process placement is a runtime choice, not a layer boundary
([modules § languages](../../docs/10-modules.md#languages)); the recipe for
both is [`../../src/mcp/README.md`](../../src/mcp/README.md).

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | **new**; V1's protocol carries keys, trust and `event_hash` we do not have — take the identifier rules only |
| `mcp` face + both push modes | **TypeScript on bun** | **new**, all of it. V1 runs Claude's on bun and Codex's on Node because bun's WebSocket could not do a unix socket; bun reaches the App Server over `ws://` instead, so one runtime does both |

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

**One master token reaches every service**, and any holder may claim any
name — PoC checks the token, not who a caller says it is. It is issued over
SSH because sshd does the authentication for free
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

We did not only read V1 — we **used** it all through this PoC to talk to the
reviewer, and lost a message to it. That is the strongest evidence available
for what V2 is for. The first draft of this section overstated it, and the
reviewer corrected it against V1's own source; what follows is what survived.

| What happened on V1 | Why | What V2 does instead |
|---|---|---|
| A review answer was **accepted and never delivered**: it went to `agent-pub`, the label the sender's tool had defaulted `from_channel` to, and no session consumes that name. `delivery: null`, no error, found an hour later by asking | the reply route is a field the sender fills in, and a wrong one is indistinguishable from a right one until nothing happens | **the name is the address**, and a send to a name with no record is refused at `send` ([messaging § verbs](../../docs/04-messaging.md#verbs)). Not a cure by itself — see the honesty note below |
| Sending needed the whole envelope spelled out — `--to-user`, `--channel`, `--from-channel`, `--source`, `--type`, payload — on the low-level command | channel, user, session, source and event type are five fields that must agree, and the low-level verb exposes all of them | `agent-bus send <name> "text"`. Two identity parameters, supplied by the socket locally ([access § two parameters](../../docs/02-access.md#two-parameters)) |
| Finding a peer took three overlapping commands — `channel list`, `channel connected`, `agent-sessions` — answering in durable registration, live lease and named session | one registry, three views, and no single "who can I talk to?" | `agent-bus ls`: one registry, one record per name |
| `agent-bus help` answers `config_invalid: unknown command "help"` | no top-level help dispatch, so the argument parser rejects it like a bad config | one binary, and `help` prints the verbs |

**Where the first draft was wrong, corrected against V1's source:**

| I wrote | Actually |
|---|---|
| "channels are ephemeral — the channel dies and the results die with it" | **V1's channel inboxes are durable.** `CHANNEL_EVENTS` is a file-backed work-queue stream, `max_age: 0`, read by *named durable* consumers with explicit ack; closing a session releases the ownership lease, not the inbox. V1 *also* has a separate ephemeral request/reply path, and that is the one whose lifetime makes a late result unrecoverable |
| "acceptance and delivery live in separate journals, so the honest answer needs two queries" | one `event status` returns the event **and** the latest delivery observation. The cost is that you must ask *after* sending, not that it takes two calls |
| "a transport journal can only say it accepted bytes" | V1's observations come from the **runtime adapter** — turn started, turn completed — not from the transport. They still do not prove the work was done correctly, which is the real argument for a correlated business reply |
| "`delivery: null` means in flight or failed or evicted" | it means **no retained observation**; a recorded failure normally *has* a `delivery.failed`. The ambiguity of an absent one is a fair complaint, a silent failure is not |
| "three registries" | three **commands** over one registry |
| "a JSON payload file every time" | the wrapper takes inline JSON or stdin; it was the low-level verb that wanted a file |

**Where V1 was better than our PoC, and we took its answer:** its bound
**refuses** the new message (`discard: new`, per subject) where ours dropped
the oldest without a word. Both modes were already the design
([messaging § overflow](../../docs/04-messaging.md#overflow)) and PoC had
implemented only the lossy one, as the default. Now the receiver's record
says which it wants, **refusing is the default**, and a ring counts what it
threw away in `status` — a queue that forgets silently looks exactly like one
nobody sent to. That is V1 paying for itself: the case was real, so the
complexity is earned.

**Honesty about V2's own guarantees**, since a design document is worth
nothing if it flatters itself:

- A **sender's** name is checked for syntax, not for registration. Sending
  from an unregistered name succeeds and a reply to it fails with *no such
  name* — fire-and-forget is deliberately allowed, so a caller that wants an
  answer registers first, which is what `call` does.
- Registration proves an address, never a reader. A registered inbox nobody
  drains keeps accepting only while it has room — after that the default is to
  refuse the send ([messaging § overflow](../../docs/04-messaging.md#overflow)),
  which is the point: the sender is told.
- `ack` means **received**, not started or finished — our own script services
  ack before running the script. **No ack means unknown**, not proven loss.
  Receipts are better than a journal because they come from the only party
  that knows, not because they remove uncertainty.
- "Kept forever" means **while the daemon lives, and until dequeue or
  overflow.** Memory only, a restart clears it, and the Parquet dump that
  fixes that is MVP's ([messaging § durability](../../docs/04-messaging.md#durability)).

The through-line that does hold: V2's case is **simpler naming, routing and
operator workflow, with deliberately smaller semantics** — not that V1 lacked
durable name-owned queues. It did not.

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
edit each, running `src/smoke.sh --slow` in every copy — a check that did not
run caught nothing — and it is worth rewriting per
review round rather than keeping, because the mutations are the interesting
part and they are never the same twice.

Three traps in the harness itself, all met:

| Trap | Why it lies |
|---|---|
| a copy that cannot bind its port, because an earlier run left a daemon behind | it fails *for the wrong reason* and looks like a caught mutation |
| a mutation that makes the whole run slow — breaking the stop signal leaves every service waiting out its poll | it hits the timeout, which is not the same as the check failing |
| **a mutation that does not compile** | the suite exits before a single check runs, so there are no failures to see — and "no failure" reads as *green*, the exact opposite of the truth |

The last one inverts the answer rather than muddying it, so the harness has to
assert that the run *produced checks at all* before reading which of them
failed. When any of the three happens, run that one check on its own against
both builds.

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
