# Historical wave evidence

Recorded results before the documentation rewrite; not proof of the remaining installed stage gates.

# DONE — MVP

What this stage has finished, wave by wave, and what each one proved. The
active plan is [TODO.md](../TODO.md#todo-mvp); the stage's stable knowledge is
[README.md](../README.md#r08).

Everything below was accepted the same way
([PoC README § mutation first, then belief](../../../CLAUDE.md#mutation-first-then-belief)):
`src/smoke.sh --slow` green, and every rule broken again and watched turning a
**named** check red.

| Wave | Result |
|---|---|
| **A — calls grow up** | `done` beside `ack`, a caller's wait ending on it; `reply-to` a third party refused at accept when the address is unregistered; TTL and bound on the record instead of daemon-wide constants; the caller's deadline travelling to the service; several workers sharing one inbox; per-service `in`/`out` counters on a listing. |
| **B — it is many people's** | A token backs one principal and is the whole of a call. One socket per mapped account, supplying the caller. Tokens issued, rotated and persisted, with the previous one still good. Credentials behind the `store` port. The principal is an argument of the SSH forced command. Enrolment proves possession with the host's own `ssh-keygen`. The suite gave every name its own credential, so there is no shared token left to bypass with. |
| **C — who may reach what** | `allow` is a field on the record, so the daemon enforces what it holds and never reads a private configuration. Master is the node's layer and a service may refuse it. Every write verb goes through both layers, each falsified alone. Pub/sub: one copy per subscriber, into the subscriber's own inbox. |
| **E — a restart is not a loss** | A JSON snapshot behind the `dump` port, written at start, on a graceful stop and optionally once a minute; records, backlog and counters all come back, a drained queue stays drained, and a start after an unclean stop says so and from when. |
| **F — the faces grow up** | The daemon filters, so two principals get different catalogs and each matches what they may actually call. `agent-bus-web` renders records and a bounded feed of envelopes over HTTPS on its own hostname, bodies struck out in the bus rather than in the page. An anonymous visitor gets a form and nothing else, and the whole public signal is an empty 200. Signing in spends the caller's own token on a session the **bus** holds, so the browser carries only an id and a restarted dashboard logs nobody out; the child is given the shared socket and no credential, because one with the owner's authority is a credential mint. |
| **F.6 — the views** | Seven of the eight views on one signed-in page, each a reshape of what the bus already answered that caller. Backlogs oldest first, because depth alone cannot tell a burst from an outage, and a queue at its bound marked — the daemon answering that, since a record declaring no bound takes the daemon's. Loss against the name that suffered it. A request and its receipts one row. Only the refusal reasons that have happened. A credential named by a fingerprint and never by itself. |
| **G — least privilege** | One binary, two roles: a supervisor that opens every listener, chowns the per-user ones and hands the fds down, and a bus that serves them and holds the store. A dead child is started again onto the same sockets; a supervisor killed outright takes its children with it. |
| **H — somebody else installs it** | Five programs split by privilege. `agent-bus-setup` is root-only and prints the `sudo` line rather than half-installing; the unit carries the account, its home, one declarative capability and restart. `agent-bus-admin` writes the account's `authorized_keys`, one forced command per key. `agent-bus-token` prints a credential and nothing else, and `--key` gets one with no credential to start from. |

## Version and build checks

Shared [working rules § versioning](../../../CLAUDE.md#versioning),
[setup § build information](../../../docs/09-setup.md#build-information), and
[processes § process titles](../../../docs/11-processes.md#process-titles) are built.
The full slow smoke passed 491 checks; TypeScript typechecking passed too.
Each mutation below ran the full slow smoke in an isolated copy.

| Mutation | Named check that turned red |
|---|---|
| Replace the process title with a constant | `supervisor identifies its version and role` |
| Stop incrementing the bus counter | `bus counts accepted and refused requests on both socket kinds` |
| Stop incrementing the runner counter | `runner counts messages taken` |
| Hardcode a different Go release version | `agent-bus shares the release version and build_info` |
| Hardcode different MCP and Codex handshake versions | `MCP advertises the shared version`; `Codex advertises the shared version` |
| Omit the linker's build stamp | `build_info records the builder and build time` |
| Make admin select a different existing account | `and selects that account for sudo` |

The first run also exposed a harness assumption: an installed account cannot
execute a temporary binary in the invoking user's private directory. The
admin check now observes its account selection through a temporary sudo stub;
the following checks still exercise its file operations in an isolated home.
The first mutation runs preceded this correction and also reported that
failure; some parallel runs hit the existing HTTPS log timing check. The named
failures above were observed separately from those unrelated failures.

## What the reviews and the mutants caught

The value is in the ones that were **green for the wrong reason**, because
those are the checks a suite cannot find by passing:

| | |
|---|---|
| Two checks asked a **dead process** what it left behind | `pgrep -P <dead pid>` is always empty — orphans reparent to init — so "no bus is left behind" passed with a bus left behind. Pids are captured before the kill now, and required non-empty. |
| "A different bus pid" passed **with no bus at all** | an absent process is trivially a different one. |
| A stop that was not passed on **still ended the children** | the 6-second kill fallback did it, so the graceful path was never exercised. The check now reads what the bus wrote on the way out. |
| An installer run as the wrong user **still exited non-zero** | because `useradd` failed afterwards. The check now requires it to refuse *before* the first step. |
| A tampered signature check was **too weak** | trailing bytes after an armored `ssh-keygen` signature are ignored, so appending one changed nothing; truncation is the mutation that bites. |
| A socket's **inode across a restart** could not be expressed as a mutation | falsified by hand instead, and recorded as such rather than left implied. |
| A check that only matched `"deadline"` passed with the stamping **deleted** | a zero time is still a field. The year is the check. |
| A subscribe refused for the **wrong reason** | `subscribe jobs@srv1` was refused because `jobs@srv1` did not exist yet, not because it is a queue topic; the topic is now created first, and the refusal names its mode. |
| "A caller cannot state its own subscribers" was true **only on a restate** | the stored subscribers were copied back over the stated ones, hiding the fact that a brand-new record kept them. |
| A lost pub/sub copy was charged **wherever** | the only check was the node's total, which rises whichever inbox is debited. Per-inbox counters made the question askable, and the subscriber must now show the loss while the topic shows none. |
| The **oldest-message age** could not be got wrong | with one message in the queue the head is also the tail, so taking the age from the wrong end read identically. Two messages of different ages. |
| An `is_empty` check passed on a **crash** | breaking the empty-queue guard indexes an empty slice and panics; a check for an *absent* field passes just as well when the command falls over. It now asserts the record still comes back. |
| A snapshot condition that could **never be reached** | it guarded an inbox whose only state is its loss, but a drop is always followed by `in++` in the same call. Mutation proved it by changing nothing; the code is gone. |
| The guard that stops a caller's stated liveness coming back as observed had **no test** | the mutation that removed it changed nothing, because `register` cleans the record on the way in and `withLiveness` never sees a dirty one in the suite. Kept, and given a test at the one place every answer is built. |
| A stale note — a service **killed outright** — was never checked at all | nothing said that a note with no process behind it is cleared rather than offered as something to stop, and the mutation that reported it as running broke nothing. |
| A confined script writing in its **work directory** passed with the directory **not bound in** | the script and the work directory shared a parent, so the script's own read-only bind carried the work directory in with it and the property held whether it was bound or merely listed as writable. The script now lives outside that parent, and the check also looks for the file on the host afterwards. |
| A criterion that **could not be built** | "a backlog with a reader beside it as the control" describes a state this daemon cannot reach: an unfiltered reader is handed the message as it arrives, so a queue only grows where nobody is reading. The ranking rule written for it would have been unfalsifiable code. Rewritten as age against depth, with a reader-and-empty service as the control, and the correction kept in the criterion. |
| A queue "at its bound" measured against the **bound it declared** | which is zero for almost every record, so the mark was either always on or always off depending on which way it was written. The daemon answers it now, because only the daemon knows what a record that declares nothing gets. |
| An **empty credential** would have made every visitor the owner | the real hole this wave found, and not by a mutation: `POST /signin` with nothing typed forwarded nothing, and a socket that supplies the identity answers that. Guarded before it can be sent, and covered by a check that deliberately points the child at the owner's socket. |
| A session kept **inside the web child** passed everything but one check | which is why "a web child restarted mid-session logs nobody out" was written as the criterion before the code was. The mutant failed that check alone, exactly as the plan predicted. |
| Two of the plan's own **first-draft criteria** passed on PoC code | before a line of MVP work existed; both were rewritten. |

## Nine harness traps

Recorded in PoC README (PoC plan, removed 2026-09-18) as they were found, because each
one made a batch lie: port spacing between concurrent runs, editing `src/`
while a batch is copying it, a mutation the daemon cannot start with, a mutant
that does not compile, a `--slow` section skipped in the fast run, a
heuristic matching `"bad token"` instead of `"address already in use"`, and a
suite leaving its temporary directory behind because a shutdown dump raced
`rm -rf`.

The eighth is the harness reading only lines that **start with** `FAIL`: a
`t.Fatalf` line does not, so a claim held by a Go test alone cannot be named
as a mutant's expected check and reads as uncaught. Such a mutant names
`go race` and is watched failing by hand on its own assertion.

The ninth cost five mutants: a mutation that turns a **refusal into a
foreground service** hangs the check waiting on it, the suite hits the
harness's 900-second limit, and the `TimeoutExpired` used to escape and end
the whole batch. A refusal is now asked through a bounded `ab`, and the
harness reports a hung mutant and carries on. With the bound, `timeout`'s own
non-zero exit satisfies an exit-code check, so such a check has to assert the
**message** as well.
