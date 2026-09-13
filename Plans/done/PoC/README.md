# PoC — agent-bus V2

Archived completed stage. This page describes the PoC at completion, not the current contracts. Current scope is [MVP](../../MVP/README.md#scope); current conventions are in [CLAUDE](../../../CLAUDE.md#working-rules).

What a developer must know to work on the PoC correctly. The active plan is
[TODO.md](TODO.md#todo-poc).

**[stages § PoC](scope.md#poc) fixes the scope**; the rest of
[docs/](../../../docs/00-overview.md#overview) is the design of record and governs *how*
anything in that scope is built. Read it that way round, or persistence,
crypto and the runner arrive through the back door.

## Purpose

Prove that **two agent sessions find each other and talk** over a V2 daemon.
Nothing else has to be true. The PoC is allowed to be thrown away; it is not
allowed to lie about what it demonstrated.

Scope is fixed in [stages § PoC](scope.md#poc) — read it before
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
| Claude | **one process**: the MCP server pushes into its own session through a channel, exactly Legacy-V1's `notifier-claude` shape |
| Codex | **two processes, one `AGENT_BUS_NAME`**: the MCP server inside the session with push off, and a pusher beside it holding the App Server connection. A session cannot push into itself through its own stdio |

Process placement is a runtime choice, not a layer boundary
([modules § languages](../../../docs/10-modules.md#languages)); the recipe for
both is [`../../../src/mcp/README.md`](../../../src/mcp/README.md#the-mcp-face).

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | **new**; Legacy-V1's protocol carries keys, trust and `event_hash` we do not have — take the identifier rules only |
| `mcp` face + both push modes | **TypeScript on bun** | **new**, all of it. Legacy-V1 runs Claude's on bun and Codex's on Node because bun's WebSocket could not do a unix socket; bun reaches the App Server over `ws://` instead, so one runtime does both |

Why the split: [modules § languages](../../../docs/10-modules.md#languages).

## Contracts that hold even in PoC

These are the ones a shortcut would quietly break.

| Invariant | Where it is stated |
|---|---|
| A call carries **a token and nothing else** — it backs one principal, and the socket is a credential of the same kind rather than an exemption from having one. The PoC sent a name beside it; that was dropped afterwards | [access § what a call carries](../../../docs/02-access.md#what-a-call-carries) |
| **The name is the identity.** No provider numeric id is ever a principal id | [identity § names](../../../docs/01-identity.md#names) |
| A **reply matches on topic + tag**; the bus adds no call machinery | [messaging § request and reply](../../../docs/04-messaging.md#request-and-reply) |
| **`ack` = got it, `done` = finished**, both emitted by the receiver | [messaging § receipts](../../../docs/04-messaging.md#receipts) |
| **Dependencies point inward**; only adapters touch the outside world, and no verb exists only in a face | [modules § the rule](../../../docs/10-modules.md#the-rule) |
| **An inbox has one reader**; a waiter filters by topic + tag and the daemon serves the match | [messaging § one reader per inbox](../../../docs/04-messaging.md#one-reader-per-inbox) |
| **A transport ack is not "the model acted"** — only a correlated reply is | [runner § adapters](../../../docs/08-runner-role.md#adapters) |
| **No external broker.** The daemon is the broker | [overview](../../../docs/00-overview.md#overview) |

## What PoC deliberately does not have

The list lives in [stages § PoC](scope.md#poc). The one
consequence worth repeating: bodies are plaintext, so the daemon *can* read
them, and "the bus never reads payloads" is R1's
([stages § R1](../../R1/README.md#scope)).

**One master token reaches every service**, and any holder may claim any
name — PoC checks the token, not who a caller says it is. It is issued over
SSH because sshd does the authentication for free
([access § getting a token](../../../docs/02-access.md#getting-a-token)).

## Legacy-V1 is the bar

**Write the small version first.** Legacy-V1's components are entangled with
JetStream's signed wires, `event_hash`, KV journals, `ADAPTER_PENDING`
recovery, delivery observations and a PHP handoff socket — none of which V2
has — so starting from them costs more than starting fresh.

**Then compare, and take Legacy-V1's solution where it is better.** It ran in
production; every awkward branch in it is a case somebody actually hit. If
Legacy-V1's approach is better, use it — the code included, when it comes free of the
JetStream machinery.

**Simplicity breaks the tie.** Complexity has to be paid for by a real case:
*"Legacy-V1 does X"* is not a reason, *"Legacy-V1 does X because Y happened"* is. Skipping a
Legacy-V1 behaviour is fine; skipping it without noticing is not.

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
`/rd/vhosts/realty/Plans/PRF-25/`. The PoC may be smaller than Legacy-V1 — it
may not be worse at what it does.

## What Legacy-V1 cost us, first hand

We did not only read Legacy-V1 — we **used** it all through this PoC to talk to
the reviewer, and lost a message to it. That is the strongest evidence
available for what V2 is for. The first draft of this section overstated it,
and the reviewer corrected it against Legacy-V1's own source; what follows is
what survived.

| What happened on Legacy-V1 | Why | What V2 does instead |
|---|---|---|
| A review answer was **accepted and never delivered**: it went to `agent-pub`, the label the sender's tool had defaulted `from_channel` to, and no session consumes that name. `delivery: null`, no error, found an hour later by asking | the reply route is a field the sender fills in, and a wrong one is indistinguishable from a right one until nothing happens | **the name is the address**, and a send to a name with no record is refused at `send` ([messaging § verbs](../../../docs/04-messaging.md#verbs)). Not a cure by itself — see the honesty note below |
| Sending needed the whole envelope spelled out — `--to-user`, `--channel`, `--from-channel`, `--source`, `--type`, payload — on the low-level command | channel, user, session, source and event type are five fields that must agree, and the low-level verb exposes all of them | `agent-bus send <name> "text"`. Identity is the token, and the socket supplies even that locally ([access § what a call carries](../../../docs/02-access.md#what-a-call-carries)) |
| Finding a peer took three overlapping commands — `channel list`, `channel connected`, `agent-sessions` — answering in durable registration, live lease and named session | one registry, three views, and no single "who can I talk to?" | `agent-bus ls`: one registry, one record per name |
| `agent-bus help` answers `config_invalid: unknown command "help"` | no top-level help dispatch, so the argument parser rejects it like a bad config | one binary, and `help` prints the verbs |

**Where the first draft was wrong, corrected against Legacy-V1's source:**

| I wrote | Actually |
|---|---|
| "channels are ephemeral — the channel dies and the results die with it" | **Legacy-V1's channel inboxes are durable.** `CHANNEL_EVENTS` is a file-backed work-queue stream, `max_age: 0`, read by *named durable* consumers with explicit ack; closing a session releases the ownership lease, not the inbox. Legacy-V1 *also* has a separate ephemeral request/reply path, and that is the one whose lifetime makes a late result unrecoverable |
| "acceptance and delivery live in separate journals, so the honest answer needs two queries" | one `event status` returns the event **and** the latest delivery observation. The cost is that you must ask *after* sending, not that it takes two calls |
| "a transport journal can only say it accepted bytes" | Legacy-V1's observations come from the **runtime adapter** — turn started, turn completed — not from the transport. They still do not prove the work was done correctly, which is the real argument for a correlated business reply |
| "`delivery: null` means in flight or failed or evicted" | it means **no retained observation**; a recorded failure normally *has* a `delivery.failed`. The ambiguity of an absent one is a fair complaint, a silent failure is not |
| "three registries" | three **commands** over one registry |
| "a JSON payload file every time" | the wrapper takes inline JSON or stdin; it was the low-level verb that wanted a file |

**Where Legacy-V1 was better than our PoC, and we took its answer:** its bound
**refuses** the new message (`discard: new`, per subject) where ours dropped
the oldest without a word. Both modes were already the design
([messaging § overflow](../../../docs/04-messaging.md#overflow)) and PoC had
implemented only the lossy one, as the default. Now the receiver's record
says which it wants, **refusing is the default**, and a ring counts what it
threw away in `status` — a queue that forgets silently looks exactly like one
nobody sent to. That is Legacy-V1 paying for itself: the case was real, so the
complexity is earned.

**Honesty about V2's own guarantees**, since a design document is worth
nothing if it flatters itself:

- A **sender's** name is checked for syntax, not for registration. Sending
  from an unregistered name succeeds and a reply to it fails with *no such
  name* — fire-and-forget is deliberately allowed, so a caller that wants an
  answer registers first, which is what `call` does.
- Registration proves an address, never a reader. A registered inbox nobody
  drains keeps accepting only while it has room — after that the default is to
  refuse the send ([messaging § overflow](../../../docs/04-messaging.md#overflow)),
  which is the point: the sender is told.
- `ack` means **received**, not started or finished — our own script services
  ack before running the script. **No ack means unknown**, not proven loss.
  Receipts are better than a journal because they come from the only party
  that knows, not because they remove uncertainty.
- "Kept forever" means **while the daemon lives, and until dequeue or
  overflow.** Memory only, a restart clears it, and the Parquet dump that
  fixes that is MVP's ([messaging § durability](../../../docs/04-messaging.md#durability)).

The through-line that does hold: V2's case is **simpler naming, routing and
operator workflow, with deliberately smaller semantics** — not that Legacy-V1
lacked durable name-owned queues. It did not.

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
| exercising a different path than the one it names | a *waiting reader* check whose inbox still held a message never blocked, so it measured the queued path twice |
| leaning on the refusal to end the command | `start` exits when it is refused and runs forever when it is not, so removing the refusal hung the check instead of failing it — bound anything whose success does not return |
| signalling the wrong process | `f() { ...; } &` backgrounds a subshell, so the service never got the signal |
| asking a dead process what it left behind | an orphan is reparented to init the moment its parent dies, so `pgrep -P <that parent>` is empty however badly it left — take the pids *before* the kill |

**Naming a shape does not remove it.** Three more checks were still asserting
their own `echo` long after that row was written — the sweep for repeats is
part of the fix, not a later tidy.

So: **for every fix, break it again and watch the check go red.** The harness
that does it for a batch of fixes is a loop over copies of `src/` with one
edit each, running `src/smoke.sh --slow` in every copy — a check that did not
run caught nothing — and it is worth rewriting per
review round rather than keeping, because the mutations are the interesting
part and they are never the same twice.

Seven traps in the harness itself, all met:

| Trap | Why it lies |
|---|---|
| a copy that cannot bind its port, because an earlier run left a daemon behind | it fails *for the wrong reason* and looks like a caught mutation |
| a mutation that makes the whole run slow — breaking the stop signal leaves every service waiting out its poll | it hits the timeout, which is not the same as the check failing — and an unguarded timeout takes the rest of the batch with it, so catch it per mutant |
| **a mutation that does not compile** | the suite exits before a single check runs, so there are no failures to see — and "no failure" reads as *green*, the exact opposite of the truth |
| a mutation that compiles but fails `go vet` — `x = x` is the easy one to write | the suite runs vet first, so it stops there; same inversion as the line above, from code that is legal Go |
| mutants spaced closer than the ports one run uses | a mutant that deliberately leaks a process lands on the next mutant's ports, and that one fails for a reason that has nothing to do with it |
| a mutation the daemon cannot start with | it compiles and it vets, and then the suite exits at its first gate with no check having run — the same inversion, from a change that looks harmless. A check only that mutation could falsify has to be falsified by hand instead |
| editing `src/` while the batch is running | each mutant copies the tree when its turn comes, so the later ones are mutating code the earlier ones never saw — and a pattern that moved is reported as *not found* |

A mutant that does not build, one that fails `go vet`, and one the daemon
cannot start with all invert the answer rather than muddying it, so the
harness has to assert that the run *produced checks at all* before reading
which of them failed. When any of the seven happens, run that one check on its
own against both builds.

## Working rules

- Task IDs are stable and never renumbered; use them in commits.
- One wave, one deliverable; commit at the end of each, push only when asked.
- A decision taken here lands in [decisions](../../../docs/decisions.md#mvp-decisions) and in
  the doc that owns it — never only in the plan.
- **Before a task with a Legacy-V1 counterpart closes**, compare the two: take Legacy-V1's
  solution where it is better, and record what we skip and why. Simplicity
  wins a tie.
- What the PoC teaches that contradicts the design is a **doc edit**, not a
  note in this file.
