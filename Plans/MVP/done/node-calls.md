# A node publishes the calls it has served — 0.5.39

Shipped at 0.5.39, one version after
[the node identity it amends](node-identity.md#a-node-says-what-it-is-to-anybody--0538).
The contract is
[what a node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself).

codex wrote the code. This file records the direction and the review, both mine.

> **The header changed again at 0.5.40.** The labels became `uptime:` and
> `calls: minute: / hour: / total:`, and the `(observed …)` spans were dropped
> from the header by owner decision, taken with the approximation stated. What
> is recorded below is 0.5.39 as it shipped; the measurement and the wire format
> did not change. Current contract:
> [what a node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself).

## What changed and why

The owner read the shipped footer and cut it down:

> we do not need OS reading - i want ONLY number of calls served by daemon

So the host load average left the public payload, and the bus's accepted and
dequeued **message** counts left with it. In their place is the one figure the
owner asked for: **calls the daemon has served**.

The header then settled over four further instructions — one line only; two
numbers; a five-minute window removed; a day window added and then withdrawn in
favour of a total:

```
AgentBus V0.5.39 · parf.us · 1h23m up · owner: parf@parf · min: 12 ; hr: 430 ; total: 1884
```

Each reversal cost minutes rather than a release, because the source was frozen
only at the end. The day window is the one worth naming: it was cancelled after
codex had committed to 1441 minute samples for it, and was stopped before that
landed.

## The mistake that shaped the previous version

At 0.5.38 I told codex, flatly, that **there is no request counter in the
daemon**. codex built accepted/dequeued message counts on that basis and named
them carefully so they would not claim to be calls — which is the only reason
the error cost one version instead of a wrong number on a public page.

It was false. `cmd/agent-busd/bus.go` wraps every handler on every listener:

```go
serve := func(l net.Listener, h http.Handler) {
    srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls.Add(1)
        h.ServeHTTP(w, r)
    }),
```

It had been there all along, and `ps` had been printing it all along. The
line I had already looked at that session, on the daemon then running, was
`agent-busd 0.5.28 ; Calls: 6605 ; bus` — the pre-0.5.38 process, up 21h45m.

I missed it because I grepped `internal/core` and `internal/api`. The counter
lives in `cmd/agent-busd` — in the process, not the library. **A negative
conclusion from a scoped search, published as a fact about the system.** That is
the whole lesson, and it is worth more than the feature.

## Why a total rather than a day

The counter is process-local and resets with the daemon. On a node restarted
within the last 24 hours — as this one had been — a `day:` figure would report
everything since start while calling itself a day. `total:` reports the
same number and is honest about it, and needs no observed span of its own
because `up` is on the same line and scopes it.

## What the figure means

Settled against source and stated on the page, not only here:

| | |
|---|---|
| requests **admitted**, not work completed | counted before the handler runs, so a refusal, a router 404 and a served call weigh the same. A node under a flood of rejected requests reads as busy, which is true |
| a long poll counts when it **starts** | a waiting `consume` was counted on arrival and may still be open. On a bus whose faces sit in long polls, a quiet minute in which readers attach still shows calls |
| the dashboard counts itself | loading a page adds to the figures it then shows. No page refreshes itself, so an open page left alone generates nothing — it is a person reloading who moves the number they are watching |
| `total:` is exact, the windows are not | the total is the counter read directly. `min:` and `hr:` are differences between a sampled reading and the live total, so neither is an exact minute or hour and each carries the span it actually observed |
| all three reset on restart | the counter lives with the process, and `up` beside it is what tells a restart from a quiet node. The baseline reading is taken after startup loading and immediately before the listeners serve, so no request precedes it |

## Five corrections the review got wrong

codex reproduced each in source and pushed back. All five were mine, four of
them the same shape — a confident claim from partial reading, stated as fact.

| What I claimed | What the source said |
|---|---|
| there is no request counter in the daemon | `bus.go:131-136`. Above |
| the per-caller series admits an anonymous caller to every record with no `Allow` entry | `acting("")` fails first at `acl.go:33`; an unnamed caller sees nothing |
| the page labels a window without saying what it observed | `frame.go` already rendered the observed span |
| an open page on a refresh generates its own traffic | no page refreshes itself — `views.go:25`, a comment I had read that day |
| a daemon up ten minutes shows one number three times | `min:` covers the last minute while `hr:` and `total:` cover all ten |

The standing rule that made this cheap: **a document contradicting source is the
document's problem.** It was stated to codex before they built and invoked every
time.

## What proves it

[`calls_test.go`](../../../src/cmd/agent-busd/calls_test.go) over the counter as
the daemon wires it, `internal/callstats` over the sampled windows, and
[`node_test.go`](../../../src/internal/api/node_test.go) over the public answer.

`src/smoke.sh --slow`: **583 passed, 0 failed**, exit 0, vet and race green
(`tmp/calls-39/slow.log:924`). Run from a frozen byte copy whose SHA-256 matches
the tracked script — `798f9b0a…63df27` for both — so nothing about the harness
differed from what the repository holds.

**16 isolated mutations, 16 caught, no survivors** (`tmp/calls-39/mutations.log`,
replayable with `mutate.py`). Each overlay runs the named package tests rather
than the full suite. What each one actually changes, and what its failure
therefore pins:

| Mutation | What it changes | What its catch pins |
|---|---|---|
| `admission-not-counted`, `admission-counted-twice` | `calls.Add(1)` → `Add(0)` / `Add(2)` | a request counts, exactly once |
| `count-after-handler` | moves the increment below `ServeHTTP` | it counts on admission, so a refusal counts |
| `omit-api-calls` | drops `node.Calls = &stats` | the bound counter reaches the public answer at all |
| `wrong-baseline` | inverts the baseline comparison | the window starts at the right sample |
| `duplicate-overwrites-baseline` | rewrites the newest sample in place | a repeated tick does not move the baseline under it |
| `stale-total` | `Total: 0` | the total is read, not assumed |
| `window-is-total` | `out.Total - baseline.total` → `out.Total` | a window is not the total wearing a label |
| `nominal-not-observed`, `lose-observed-span` | reports the nominal span / drops it from the page | the span shown is the one observed |
| `measured-zero-unavailable` | `Available` only when the count moved | a measured zero stays available and visible |
| `invent-unobserved-zero`, `hide-zero-window` | renders `0` for unobserved / hides a zero count | unobserved and zero are different, and both are shown |
| `invent-pre-sample-history` | marks a window available with no baseline | history absent is not history invented |
| `short-history` | retention `61` → `2` | truncated retention is caught rather than silently shortening windows |
| `wrong-header-total` | header total → `0` | the header prints the counter |

That is not every claim the page makes. The footer's explanatory text is not
mutated, and multi-listener coverage rests on a named test exercising both
wrappers rather than on a mutant.

Stamped `.39` binaries built at `parf@parf.us 2026-09-16 20:09:34`. Live
preflight: 7 records, 4 profiles, unchanged.

## What this is not

Not F.13.2, for the reasons its
[predecessor gives](node-identity.md#what-this-is-not) — and F.13.0, the owner's
design review, still gates that task. No new task ID: this is owner direction
inside an existing plan.

The counter is **not** a metric system, an SLO, or a record of work done. It is
one number the process already kept, published with its limits attached.
