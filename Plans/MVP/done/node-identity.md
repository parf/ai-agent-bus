# A node says what it is to anybody — 0.5.38

Shipped at 0.5.38. The owner asked that a person reaching this bus be able to
tell what it is and whose it is **without a credential**, including on the
sign-in page. The contract is
[what a node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself);
this file is how it was built and what was measured.

codex wrote the code. This file records the review, which was read-only.

## What the owner asked, and what it grew into

It began as four facts — release, build, owner, uptime — and grew twice while
being built, each time by the owner: **host name**, then **load**, and load
meant *both* the host's average and the bus's own traffic. The last of those
contradicted a sentence I had already written into the contract arguing that
publishing traffic to strangers was a poor trade. The sentence lost, and was
removed rather than left to argue with the decision above it.

## Three corrections the review got wrong

Worth more than the parts that went smoothly, because each is a way of reading
source that looks like verification and is not.

| What I claimed | What the source said |
|---|---|
| the per-caller series would admit an anonymous caller to every record with no `Allow` entry, so it must not serve this endpoint | **False.** `acting("")` → `knows("")` → `identityKind("")` finds `""` in neither `b.users` nor `b.records`, returns `DirectoryCredential`, and `may()` returns false at `acl.go:33` — before the `len(r.Allow)==0` line I had quoted. I read one line in isolation and never traced the guard above it |
| the page labels a window "last minute" without saying what it observed | **False.** `frame.go:83` already rendered `~{{.Window}}` with `(observed {{.Observed}})`. I reviewed the page without reading the page |
| an unknown host publishes as an empty string, unlike load's explicit nil | **True but not a defect.** `frame.go:76` renders `host unavailable`, so nothing fabricated reaches a reader. Pointer-versus-empty is a wire format, and those do not change on a reviewer's taste |

The conclusion of the first survived its own refutation, with a better reason.
Not that a caller-filtered series can never produce an aggregate — a privileged
caller gets one over the records it may see — but that it cannot produce **this**
figure **on an anonymous caller's behalf**: asked for a caller it cannot name it
would answer a confident **zero** on a busy node. So the dedicated aggregate is
right because the filtered path cannot answer this question for this caller,
not because it would leak.

## The one finding that held

`identity` called `Status()` purely to read `.Up`, and `Status()` locks `b.mu`
and sums every inbox before the rest is discarded; `NodeMessages` locks again.
Two acquisitions of the mutex every send and consume contends on, plus an
O(inboxes) walk it had no use for, on an **ungated route with no rate limiting
in front of it**. Not the only ungated route that touches bus state — `/enrol`
is one too, and it writes — but the only one paying for a full scan it then
discarded.

`b.started` is written once in `New()` and never again — the only other mention
in the package is the read it feeds — so it is immutable for the life of the
`Bus` and readable without the lock. codex added that accessor, `Status()` now
reuses it, and the public endpoint holds the registry once for bounded samples
and scans no inboxes.

## What the figures do and do not mean

Settled against source, and stated on the page rather than only here:

| | |
|---|---|
| they count **accepted** and **dequeued**, never *calls* | there is no request counter in the daemon. A figure labelled for something it does not count is worse than no figure |
| a publication counts its **copies**, not itself | `fanout` (`bus.go:657`) bumps the topic's own counter with no node total beside it; the per-subscriber copies count as they are accepted. So a publish to a topic nobody subscribes to moves the public number by zero, and the page says zero is not idle |
| the two are not a pair | straight-through delivery counts both at once, a queued message counts accepted now and dequeued whenever somebody reads. Either can exceed the other in a window |
| a removed name keeps its traffic | the total is process-local, so unregistering cannot erase it. The dashboard's per-caller series is the opposite by design, and both are stated |
| unobserved history is never zero | `Available` guards the window, and each one states the span it actually covered |

## What proves it

[`node_test.go`](../../../src/internal/api/node_test.go) over the real HTTP API,
[`frame_test.go`](../../../src/cmd/agent-bus-web/frame_test.go) over the rendered
shell, and core coverage of the aggregate itself.

`src/smoke.sh --slow`: **578 passed, 0 failed**, exit 0, vet and race green,
no warnings or skips (`tmp/node-identity/slow-frozen.log`). Run from a frozen
byte copy whose SHA-256 matches the tracked script — `50fea640…4728ca` for
both — so nothing about the harness differed from what the repository holds.

**22 isolated mutations, each caught by a named assertion**
(`tmp/node-identity/mutations-final.log`, replayable with `mutations.py`). Each
overlay runs the named API, core and web tests rather than the whole suite per
mutant.

Browser checks on populated fixtures at **390×844** (sign-in), **1280×800** and
**320×800** (services), document width within the viewport at each — measured,
in `tmp/node-identity/browser-metrics.log`, with the screenshots inspected.
Header and footer escaping against a hostile string, mixed daemon and web
versions, and a legacy daemon answering without the new fields are each pinned
by a test. **A long host name is not**: it is handled by
`overflow-wrap:anywhere` and nothing asserts it, so it rests on the CSS holding
rather than on a check that would fail if it stopped. The measured host was
`parf.us`. Recorded as a gap rather than counted as coverage.

### A void run is not a failed one, and not a green one

The first final `--slow` was **discarded, not interpreted**. A comment in
`smoke.sh` was edited while bash was still reading the script, so the shell
resumed at a shifted offset and raised a syntax error. That is a property of
how `bash` streams a file it is executing, not a result about this source.

The rerun executes an immutable byte copy (`*.local.sh`, untracked) with no
edits landing while it runs. The lesson is small and cheap: **a test harness
being edited underneath a run produces a log that looks like evidence and is
not.** Recorded because the failure mode is silent — nothing in the log says
the script changed.

## What this is not

This is **not** F.13.2. That task wants navigation, deep-link return, form-input
preservation, shared not-found presentation and distinct recovery states, and
none of those is claimed here. It is also gated on **F.13.0**, the owner's
review of the page designs, which has not happened. What this ships is the
shared shell's header and footer and the public call behind them — groundwork
F.13.2 will build on, and evidence for that task when it is taken up.

No new task ID was allocated: this is an owner request delivered inside an
existing plan, not a new unit of work.
