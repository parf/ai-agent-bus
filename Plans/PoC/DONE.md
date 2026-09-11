# DONE — PoC

| Wave | Result |
|---|---|
| **A — two shells talk** | `agent-busd` on a unix socket and loopback HTTP, in-memory registry and inboxes; `agent-bus` with `status`, `register`, `ls`, `send`, `consume`, `reply`. `src/smoke.sh` is the acceptance (22 checks) and `go test -race ./...` the regressions. |
| **A review** | Codex reviewed `c8cde3a` over the V1 bus and found two real defects; both fixed, both now covered by tests that fail against the old code. |
| **B — the live slice** | `src/mcp/` on bun: the MCP face (`ab_ls`, `ab_send`, `ab_consume`, `ab_reply`), self-registration, and both push modes. `src/smoke.sh` is 26 checks and carries two harnesses of its own — 12 for the face, 7 for Claude push. |
| **B review** | Codex reviewed `170de8f` and reproduced four defects, two of which the smoke passed *for the wrong reason*. All fixed; every one now has a check that fails against the old code. |
| **B.3 review** | Codex reviewed `1779e62` against a mock App Server and found the Claude blocker, the Codex **topology** mistake, and four state and lifecycle defects. All fixed. The Codex adapter now has a fake-App-Server harness of its own. |

**A, what it proved**

- Both listeners speak the same HTTP+JSON; the CLI reaches either.
- The two parameters are enforced: a wrong token and a name without a realm are both refused.
- A message sent while nobody is reading waits, and arrives afterwards.
- `reply <message-id>` works with **no reply state in the daemon** — the CLI resolves the id from the envelope it consumed ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).
- One reader per inbox: a second unfiltered `consume` is refused with 409, while a filtered waiter is served beside it ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)).

**What the review caught**

| | |
|---|---|
| **Filtered priority was never implemented** | `Send` walked waiters in registration order, so an unfiltered reader that blocked first took a filtered waiter's reply — the exact thing the rule exists to prevent. Now two passes: matching filtered waiters, then the reader. |
| **A cancelled wait could swallow a message** | a `Send` that reported success could hand the envelope to a waiter whose deadline had just fired, and it vanished. Now the cancel path settles under the same lock: take what was delivered, or remove the waiter so nothing can be. |
| **Names were not canonical** | `" x@y "` registered one inbox and `send` reached another. Canonicalisation moved into core, so every face gets the same answer. |
| **The docs promised more than the code** | the bus enforces *one outstanding unfiltered read*, not process ownership of an inbox. The doc now says that. |
| **The smoke script proved less than it claimed** | one assertion passed unconditionally, and nothing tested two competing waiters — which is why 13/13 missed the priority bug. |
| **`trap 'kill ${DPID:-0}'`** | with a build failure before `DPID` was set, that is `kill 0` — a SIGTERM to the whole process group. |

Smaller fixes: no reply body kept on disk (routing fields only, one file per
message, so parallel consumes cannot overwrite each other), one HTTP transport
per CLI process instead of one per poll, a stale socket is checked before
removal and a live one refused, `chmod 0600` and `Serve` errors are fatal
instead of ignored, header and idle timeouts, and a bounded graceful shutdown.

---

## B.0 — the runtime spike

**Question**: does the TypeScript side have to run on Node, as V1 does, because
bun cannot speak WebSocket over a unix socket?

**Answer: no. Nothing needs a WebSocket.** `codex app-server` speaks
**newline-delimited JSON-RPC on stdio**, and bun drives it with `Bun.spawn`
and a `flush()` — `initialize` answered, and `thread/list` returned the live
sessions with their `cwd`, including the interactive Codex session this repo
had exchanged messages with minutes earlier.

| Tried | Result |
|---|---|
| `codex app-server` on stdio, NDJSON | **works**, from bun |
| the same with `Content-Length` framing | refused: *"Failed to deserialize JSONRPCMessage"* |
| `codex app-server --listen unix://…` spoken to directly | silent — it is a **WebSocket** listener, which is exactly what bit V1 |
| `codex app-server proxy [--sock …]` | exits 0 with no output; its control socket is not plain JSON-RPC. Not needed, so not pursued |

**Consequence**: one runtime. The push adapter spawns `codex app-server` and
talks NDJSON, instead of attaching to a WebSocket listener a launcher had to
start. It also removes V1's `ws` dependency and its `permessage-deflate`
workaround.

**Not proven here**: that `turn/steer` reaches a thread a live TUI owns. That
is B.3's job; B.0 only had to decide the runtime.

The managed daemon started during the spike was stopped again; nothing was
left running.

**Not done in A, on purpose**: `call`, `ack`, topics, script services, the MCP
face, any push adapter, SSH token issuance.

## B — the live slice

**B.1/B.2 — the MCP face**

One TypeScript package on bun: `bus.ts` (the daemon as seen from TypeScript),
`server.ts` (four tools over stdio). Each tool is one `fetch` and nothing
else, over `fetch(url, { unix })` — the layer rule, in the smallest possible
form. It registers `AGENT_BUS_NAME` at start, which is what makes "find each
other by name" have a name.

Reply context lives in the face, because the daemon keeps none
([messaging § reply routing](../../docs/04-messaging.md#reply-routing)): the
last 200 messages, **routing fields only** — sender, topic, tag. Bodies are
unbounded and a reply needs none of one.

**B.3 — the two push modes**

One loop (`push.ts`), two deliveries. The loop long-polls `consume` and hands
the envelope to a mode; while it runs it holds the inbox's one unfiltered
read, so `ab_consume` says so instead of colliding with it.

| Mode | How |
|---|---|
| `claude` | `notifications/claude/channel` — content plus string metadata. The session needs `--dangerously-load-development-channels` |
| `codex` | spawn `codex app-server`, NDJSON on stdio: `initialize` → `thread/list` newest for this cwd → `thread/resume`, else `thread/start`; per message `turn/steer` if a turn is running, else `turn/start` |

**Proven**: Claude push end to end in the smoke — a peer sends, the
notification arrives with the body and routing, `ab_reply` answers it, and the
peer receives the answer with the original topic and tag. Codex push proven as
far as a fresh cwd goes: `thread/start` then `turn/start` both succeeded
against a real `codex app-server`.

⚠️ **Still not proven**: that `turn/steer` reaches a thread a live TUI owns —
B.0 could not answer it and neither could this, because testing it means
steering a session somebody is using. It is the by-hand criterion.

**B — V1 compared**

| V1 | Taken | Skipped, and why |
|---|---|---|
| `notifier-claude/server.ts` | the notification shape (`content` + string `meta`), and saying in the instructions that a transport ack is not an answer — this bit us live, when a reply stayed in the UI | the PHP handoff socket, the `agent-bus consume channel` subprocess, the raw signed-wire store with its TTL and byte budget. All of them serve the journal; we keep routing fields and nothing else |
| `notifier-codex/dispatcher.ts` | the call sequence, the steer-vs-start branch, tracking the active turn from `turn/started` / `turn/completed`, and falling back to `turn/start` when steer is rejected — V1 hit that and left the reason in a comment | the adapter journal, delivery events, `ADAPTER_PENDING` recovery, sandbox-policy plumbing, thread re-selection on `thread/goal/cleared`. Each is a durability contract PoC does not have |
| `notifier-codex/app-server.ts` | nothing structural — a WebSocket client we do not need | the `ws` dependency and the `permessage-deflate` workaround, both removed by B.0 |
| dedup by `event_hash` | nothing | V1 is at-least-once because the journal can redeliver; our `consume` is at-most-once, so there is nothing to deduplicate |

**B review — what Codex found**

| | Finding | Fix |
|---|---|---|
| HIGH | a cancelled `ab_consume` kept polling, took the next message and threw the answer away | the handler's abort signal reaches `fetch`; a smoke check that fails without it |
| MED | the low-level `Server` does not enforce the schema it advertises, so `ab_send` with no `text` sent an empty body | explicit argument checks, and a smoke check |
| MED | the reply-context map held whole envelopes — 200 bodies, unbounded | routing fields only |
| — | `ab_reply` on an evicted id claimed the session never consumed it | says which, and what to do instead |
| smoke | the harness read `stderr` before killing the server, so a hang became a permanent hang | drain from the start, kill with a bound |
| smoke | nothing checked what the peer received: with `ab_reply`'s topic and tag mutated to `WRONG`, every check still passed | the peer is in-process now and asserts body, sender, topic and tag. Verified by re-running the mutation: it fails |
| smoke | a JSON-RPC error collapsed to `text: ""`, `isError: false` | checked before the result shape |
| smoke | a host without bun passed green with the face unchecked | missing bun is a failure |

**Accepted as a PoC limit, not fixed**: the face registers once at start, so a
daemon restart leaves it connected but unreachable. Memory-only storage means
a restart loses everything anyway
([stages § PoC](../../docs/12-stages.md#poc)); restarting the face with the
daemon is the contract, and the MCP instructions say so.

**B.3 review — what Codex found**

It ran the adapter against a mock App Server rather than a real one, which is
how it reached states a live session will not produce on demand.

| | Finding | Fix |
|---|---|---|
| BLOCKER | the server declared only `capabilities.tools`. Claude Code registers a channel listener **only** for a server that declares `experimental: {"claude/channel": {}}`, so the connection was refused outright. The raw smoke accepted any JSON notification, so it could not see this | declared in `claude` mode |
| **topology** | the adapter spawned its **own** `codex app-server`. That is a second process: it can resume a thread's saved history, but steering it never reaches the session a person is typing in. V1 does not do this — one App Server, TUI and notifier both attached | the face attaches to a shared server given by `AGENT_BUS_CODEX_WS`, and says so in the log when it is driving its own instead |
| | a rejected `turn/steer` fell through to `turn/start` for *any* error, so a timeout — where the steer may well have landed — started a second turn | only a protocol rejection, and only after the server confirms no turn is running. Overload and timeouts propagate |
| | `thread/resume` reporting a turn already in progress was ignored, so the first message started a turn instead of steering | the active turn comes from the resume |
| | a `turn/completed` arriving before a slow response let that response resurrect a finished turn | an epoch counter; a response that is out of date does not win |
| | server-initiated requests were parsed as notifications and left unanswered, hanging the turn | refused with `-32601`. This client approves nothing |
| | `stop()` did not abort the poll, so a delivery could run after it | the loop aborts the request and re-checks after every await |
| | push was not tied to the transport closing | `server.onclose` and the signals stop it |
| | a 409 retried every two seconds, which would steal the read from whoever legitimately owns it | one message, then stop |
| smoke | **a doubled `deliver()` left everything green** — nothing counted notifications | two distinct messages, asserted by id, order and count. Verified: the mutation now fails |
| smoke | the stderr-before-kill hang, fixed in `smoke.ts`, had been copied into `smoke-push.ts` | fixed there too |

Codex also confirmed what did **not** need changing: no lock is needed while
`start()` completes before a serial loop; the pending entry is registered
before the write, so a fast answer cannot race it; and the line framing is
correct. Refusing an unfiltered `ab_consume` while allowing a filtered one is
the right cut — with the caveat that the filter only wins once the waiter is
registered, so a *pushed* reply cannot be re-read.

## B.4 — loading it

| | |
|---|---|
| Claude, as a plugin | `.claude-plugin/plugin.json` declares the server. `claude --plugin-dir src/mcp` loads it and `mcp list` reports **connected** |
| Claude, without it | `--mcp-config .mcp.json` — proven by a real session calling `ab_ls` and getting the registry back |
| Codex | `[mcp_servers.agent-bus]` in `~/.codex/config.toml` |

A plugin manifest cannot know the session's name, so the face **derives** one:
`<runtime>.<cwd>@<host>`, trimmed to the name rule
([identity § names](../../docs/01-identity.md#names)) — the way V1 names its
channels. Zero configuration beyond the token. Verified: a session started with
no `AGENT_BUS_NAME` registered as
`claude-code.home-parf-src-ai-agent-bus-src-mcp@parf.us`.

## The live run

**Wave B is met.** A live interactive **Claude Code** session asked a live
interactive **Codex** session a question over the V2 bus and got the answer
back, matched on topic and tag. Claude's side:

```
● codex.session@parf.us replied: 23
```

Codex's side, the same exchange:

```
Name one prime number between 20 and 30. Answer with just the number.
Answer by calling ab_send with to: "claude.session@parf.us", topic: "c2c", tag: "x1", …
• Called agent-bus.ab_send({"to":"claude.session@parf.us","topic":"c2c","tag":"x1","text":"23"})
  └ the bus accepted 01dc484f817cbd33 for claude.session@parf.us (topic c2c, tag x1)
```

Driven under tmux — two real TUIs, no harness pretending to be one. Each half
was proven on its own first: a peer asked each session a question and got a
correlated reply.

**The Claude half** needed only what the design already said: `--mcp-config`
plus `--dangerously-load-development-channels server:agent-bus`, the same
invocation V1 uses, and the `claude/channel` capability declared. Two things
that do **not** work: `--strict-mcp-config` alongside the channels flag, and
loading the face **as a plugin** — a plugin-provided MCP server is not
resolvable as a channel source under either `server:agent-bus` or
`server:plugin:ab:agent-bus`. The plugin is still how the *tools* load; the
channel needs the `--mcp-config` form.

## The live run — Codex

**Done.** A bus message reached a **live interactive Codex TUI** and came back
as a correlated reply, matched on topic and tag:

```
{"from":"codex.session@parf.us","to":"asker@parf.us","topic":"final","tag":"f1","body":"42"}
```

Driven under tmux: one App Server on `ws://127.0.0.1:8421`, the TUI attached
with `--remote`, the pusher attached beside it. Five things had to be found
first, and each is now either fixed or written down:

| What happened | What it means |
|---|---|
| `thread/list` returned **nothing** for a directory whose TUI was running | a TUI has no thread until someone types into it. The pusher must pick the thread at the first *message*, and if the session has never been used it starts its own instead |
| the face-as-MCP-server never started its push loop | the App Server starts its own MCP servers, so such a face cannot dial back into the App Server that is still starting it. **Codex needs two processes under one name** — tools inside the session, pusher beside it. This is V1's split, and now we know why |
| `ab_reply` had nothing to reply to | the pusher consumed the message and the tools live in the other process. A Codex session answers with **`ab_send`**, and the pushed text carries the routing |
| `ab_reply({"message":"42"})` — invented argument names | the pushed text now spells the call out. The model got it right immediately afterwards |
| every tool call came back *"user rejected"* | a turn started by a bus message has no human, so the App Server asks the **client that started the turn** to approve. Blanket refusal killed the session's own reply; blanket approval would hand a remote peer the user's permissions. The pusher approves **only agent-bus's own tool calls** and declines the rest — two checks in the smoke |

`approvalPolicy: "never"` means *deny without asking*, not *allow*: it blocks
the reply. `on-request` is the default, and the reviewer is always the person
in the TUI.

**The earlier headless failure is explained.** A `claude -p` run took the
message off the daemon and never surfaced it. The cause was not headless mode:
it was `--strict-mcp-config`, which silently leaves the channels flag with no
server to bind to. The same invocation without it works interactively.

## B.5 — the plugin's own commands

`/ab:ls` and `/ab:send` are in `commands/`. Colon namespace, because these are
plugin commands and not MCP tools ([glossary](../../docs/glossary.md)).

**The SessionStart hook was dropped**: the MCP server already registers itself
at start, so a hook doing it again is a second implementation of B.2 with
nothing to add.

**Not done in B, on purpose**: `call`, `ack`, topics, script services, SSH
token issuance.

## C — the rest of the verbs

Eleven verbs now exist ([stages § PoC](../../docs/12-stages.md#poc)), and
`src/smoke.sh` is 38 checks.

**C.1 — `call` and `ack`.** `call` is `send` plus the filtered wait, with no
third verb on the wire and no dispatcher in the client: the daemon already
serves a filtered consume ahead of the unfiltered reader. It makes up a unique
tag when none is given, so nothing else can answer that wait.

`ack` is an ordinary message on the same topic and tag that names the message
it is about, exactly as the design says. That means a caller's wait *sees* it,
so `call` reports a receipt and keeps waiting for the answer.

Two rules had to be settled, and both are in
[decisions](../../docs/decisions.md):

| | |
|---|---|
| **a registered topic named alone is an inbox to read; with a tag beside it, a filter** | `consume --topic` had two meanings — read the topic, or filter my own inbox. The daemon decides once, so every face agrees. A reply always carries a tag, which is what keeps them apart |
| **a caller states its own record before it calls** | found by the smoke: the service's reply came back *no such receiver*. An answer needs an address to arrive at |

**C.2 — topics.** `topic create` is sugar for registering a record of kind
topic; `publish` is a send to it. A queue topic is an inbox with a name, so a
consumer that was down reads the backlog afterwards. Publishing to a pub/sub
topic answers *MVP* rather than inventing a second meaning of subscription.

**C.3 — scripts as services.** `agent-bus start <name> --algo=std|args
<script> [-N]`, or the same as JSON on stdin. The `start` process is the
inbox's **one reader**; it acks, spawns the script per message up to `-N` at
once, and sends what the script printed. A non-zero exit means no reply — the
caller waits and times out. That says the *answer* did not arrive, not that
nothing happened: a script may fail after its side effects.
The script never sees the bus: `args` hands it the body as `$1`, `std` hands
it the envelope on stdin, and both get the envelope in the environment.

**Not done in C, on purpose**: pub/sub fan-out, TTL, `done`, `reply-to`,
deadlines, supervision, sandboxing, restart.

### C — what the review changed

Codex reviewed C against a pinned copy and ran probes rather than reading;
five of its findings were real, and one of them my own three-terminal
walk-through hit an hour later. What was wrong, and what it is now:

| Was | Now |
|---|---|
| `start` registered `<name>` but read the inbox of whatever `AGENT_BUS_NAME` launched it — invisible while the two are the same, which is all the smoke ever did | the process **becomes the service**: it reads and answers as the name it registered, and the launcher stays its owner |
| "the bound is taken before the goroutine, so with all N busy nothing is consumed" — untrue: the slot was taken *after* the consume, so one extra message was always held in the process | the slot is taken **before** the consume. A service that dies loses only what its scripts were running |
| `SIGTERM` was checked between messages, so a service sat in a 55s consume and then ran the next one anyway | the signal cancels the consume; nothing new is taken, and the running scripts are waited for |
| `call` re-registered its caller on every call, overwriting kind, description and owner | it states the record **only if it has none** |
| `call` validated `--wait` after sending — a typo answered "bad --wait" for work already accepted | everything that can be refused is refused before anything is sent |
| a mistyped topic name quietly became a filter on your own inbox and timed out | a name-shaped topic registered nowhere is **refused** |
| any string was a receipt | `ack` or `done`, checked in core |

And three in the smoke itself, all found by breaking the fixes again rather
than by reading: `ab … &` backgrounds a *shell function*, so the signal in the
new stop check went to a subshell while the service ran on (`abx` execs, so
`$!` is the service); "nothing came back" was written as
`$(cmd; echo -n nothing)`, which says *nothing* whatever `cmd` did; and the
refused receipt was checked by grepping a word the accepted answer also
contains. Each fix has a check, and every check was watched failing —
[plan § mutation first, then belief](README.md#mutation-first-then-belief).

## D — close the stage

**D.1 — the token over SSH.** [`src/static-token`](../../src/static-token) is
the forced command, and it is mostly comment because there is nothing to
invent: sshd has already authenticated the caller against a key they own, so
the script prints the file `agent-busd` made on first run and refuses
anything else. Setup is one `authorized_keys` line, given in the script's
own header
([access § getting a token](../../docs/02-access.md#getting-a-token)).

The daemon needed no change for it — `loadToken` already created and read
that file — which is the point: the token path is a file, not a protocol.

While writing it the docs and the code turned out to disagree on the variable
name (`AGENT_BUS_USER_TOKEN` in two documents, `AGENT_BUS_TOKEN` everywhere in
the code). The code's name won: it is the third of the same triad as
`AGENT_BUS_NAME` and `AGENT_BUS_ADDR`.

**D.2 — the recipe.** [`src/README.md`](../../src/README.md) — build, check,
and the three terminals: the daemon, a service (by hand or as a shell
script), and a caller. Running it as written is what caught the `start`
inbox bug the same hour Codex reported it.

**D.3 — every criterion in [stages § PoC](../../docs/12-stages.md#poc).**
Walked one by one; the evidence is a named check in `src/smoke.sh` — 60 of
them now — unless the row says otherwise.

| Criterion | |
|---|---|
| daemon on a unix socket **and** HTTP | both listeners answer `status` |
| Go daemon and CLI, TypeScript MCP face on bun | `go test -race ./...`, three bun harnesses |
| one master token, no per-service anything | a wrong token is 401 on either listener |
| tokens issued over SSH | D.1 — and once for real, over a throwaway `sshd` on 127.0.0.1 with the `authorized_keys` line from the script's header: `ssh … static-token` returned the token, `ssh … 'cat /etc/passwd'` returned exit 64 and *this account offers one command*. The smoke checks the script itself; nothing in it needs sshd |
| no encryption, no sessions | plaintext by construction; the daemon refuses a non-loopback `-addr` |
| basic request/reply with `ack` | a service acks, then answers; the caller gets the answer, not the receipt |
| a script is a service, `-N` at a time, both algos | `echo "Hello $1"` answers a call; `std` gets the envelope on stdin and in the environment |
| basic MCP face: list, send, consume | its own harness over stdio, driving the server exactly as a client does |
| install: the built binary, no npm | ⚠️ true for the daemon and CLI. The MCP face is TypeScript by decision, so it needs bun and one dependency — `bun install`, never npm ([modules § languages](../../docs/10-modules.md#languages)) |
| setup: a pubkey behind the forced command | D.1 |
| storage in memory | there is no other kind here |
| loopback or an SSH tunnel only | the daemon exits rather than bind a public interface |
| the eleven verbs | all eleven, `--follow` included, each with a check |
| `consume --topic` reads a queue topic | and with a tag beside it filters your own inbox — one message in each, so the check can tell |

**Works at the end of PoC**, the seven statements:

| | |
|---|---|
| a Claude Code session talks to a Codex session and back, by name | ✅ live, by hand, both directions — [the live run](#the-live-run) |
| a publisher emits to a topic without being a registered service | ✅ `drive-by@srv1` has no record |
| a consumer reads that topic, including what was sent while it was down | ✅ published first, consumed after |
| register something; another party finds it by name and calls it | ✅ `ls` then `call` |
| an agent asks the MCP face what it can use, and can send and consume | ✅ `ab_ls`, `ab_send`, `ab_consume`, `ab_reply` |
| a service acks and answers; the caller blocks and gets the right one back, matched by topic + tag | ✅ and the reply is checked field by field at the peer, not by the sender's word |
| `echo "Hello $1"` in a file is a service, found by another party | ✅ started with one command, found through `ls`, answers a `call` |

**Reviewed by Codex against the pinned commit**, which confirmed the wave C
fixes with its own probes and found four more:

| | |
|---|---|
| the **manual** service recipe could not work: the shell that answers kept its own name, so `consume` read the wrong inbox | the answering shell takes the service's name; the recipe also builds the binaries it then calls, and reads the token after the daemon's first run. Run as written, by hand |
| `static-token`'s check asked whether the output **contained** the token — a debug line printed before it kept the smoke green, while `$(ssh … static-token)` would hold an unusable credential | equality, a separate exit check, and the issued token is used to authenticate against the daemon |
| an **empty** token file was success: exit 0 and nothing on stdout | empty or whitespace is exit 69, like a missing one. The helper still never invents a token |
| `--wait` bounded the daemon's wait but not the exchange, so a slow transfer ran to the client's generic timeout | `context.WithDeadline` on the consume, through the `callCtx` the stop fix added |

Its two suggested strengthenings are in as well: the caller's record is
compared whole rather than grepped for a word, and the stop check waits until
the daemon reports an outstanding consume and then asserts exit 0.

## After the stage closed — a full queue is never silent

Reopened narrowly, on the owner's word, for one bug the review turned up:
**1001 sends into a 1000-bound queue dropped message "0" and said nothing**.
Silence is the failure V1 taught us to hate, so the fix is about noise, not
capacity.

| | |
|---|---|
| who decides | the **receiver**, on its own record — `--overflow` on `register` and `topic create`, stored as `overflow` |
| default | `strict`: the send is **refused**, 503, and the error names the queue that is full |
| the other mode | `ring`: the oldest is dropped **and counted** — `status` carries `dropped` |
| what is gone | a loss with no trace. Either the sender is told now, or the count says it happened |

The owner's call, asked and answered: overflow now, reject-new as the default,
and the reply-address contract stays MVP wording. V1's `discard: new` was the
better answer and we took it
([messaging § overflow](../../docs/04-messaging.md#overflow)).

Mutation-verified, as the rule requires: the default flipped back to `ring`,
the strict refusal disabled, and the counter dropped — each turns a named
check red on its own.

**What is not true, and is meant not to be**: bodies are plaintext, so *the
bus never reads payloads* is a claim MVP earns, not this stage
([stages § PoC](../../docs/12-stages.md#poc)).
