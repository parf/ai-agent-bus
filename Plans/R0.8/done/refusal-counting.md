# H.5.10 — every refusal an endpoint decides is counted

Shipped in 0.5.37. Evidence for the acceptance in [TODO](../TODO.md#remaining-work)
as that row stood before removal. The [refusals contract](../../../docs/05-discovery.md#refusals)
owns what the figures mean.

## Scope

`status` carried how many calls were turned away and for what, and a refusal
decided before the error map reached none of it. Four paths answered a caller
and counted nothing: an unparseable body, an invalid name in a token request, a
lookup of a name the daemon does not hold, and a consume whose topic names a
name rather than a filter. A page drawing those figures could not distinguish a
quiet node from a refusing one, which is the single thing the counter exists for.

Found by codex reviewing [F.13.1](web-meanings.md#what-it-turned-up), which had
to bound what the dashboard could claim while this was open.

## What changed

| | |
|---|---|
| [`Server.refuse`](../../../src/internal/api/server.go) | counts once and answers. The reason is passed, not derived from the status: `403` covers acl, suspended and enrolment, and `409` covers disabled, busy and second-reader, so counting by code would collapse distinctions the caller was already told apart |
| `oops` | the other half — always `500`, never counted, because failing a caller and refusing one are different things to be told about |
| `Server.read` | a method now, so a body it cannot decode counts like any other refusal. The largest of the four, since every JSON request-body endpoint goes through it — not every writing one: starting and ending a session decode nothing |
| `reply` | counts through the same call rather than counting and then answering separately |
| the authentication gate | counted and then answered; now does both in one call, which is where a second counting layer would otherwise show up |
| [the dashboard](../../../src/cmd/agent-bus-web/main.go), [the contract](../../../docs/05-discovery.md#refusals) | both carried a caveat written while this was open; both now state the coverage and both exclusions |

## The rule the exclusions follow

**An addressed endpoint's refusals count whatever the caller's standing.** A bad
token counts although nobody authenticated, and so does an unparseable
enrolment. Two things do not: a route the router rejects before any handler runs,
because nothing here refused it, and a failure of ours, which is a `500`.

That rule replaced two lists arrived at by different arguments — opencode's
sentence, and the reason the enrolment case is counted rather than excluded for
having no authenticated caller: a bad token has none either, and is counted.

## What proves it

[`refusals_test.go`](../../../src/internal/api/refusals_test.go). Every
assertion diffs the whole reason map across one call, so each reads *this reason
moved by exactly one and nothing else moved* — a fix that counted every refusal
under two reasons would pass a test that watched only its own.

| Claim | Test |
|---|---|
| the four silent paths, and an unparseable enrolment, count | `TestARefusalDecidedBeforeTheErrorMapStillCounts` |
| a bad token and a mapped error still count exactly once | `TestAlreadyCountedRefusalsAreNotCountedTwice` |
| a hidden name answers and counts as a missing one does | `TestAHiddenNameCountsAsAMissingOneDoes` |
| success, an empty wait, an unmatched route, a refused method and an internal failure count nothing | `TestWhatIsNotARefusalIsNotCounted` |
| a counted refusal uses a reason the table names | `TestACountedRefusalUsesAReasonTheTableNames` |

The internal-failure control is a real failing credential store on a token
rotation, with a recovery call at `200` after the failure is cleared — so the
`500` is the injected failure rather than a fixture that never worked. The
hidden-versus-missing pair is checked on `/consume` as well as `/lookup`, since
each branch bypasses independently and one certifies nothing about the other.

**Nine mutations, each run against the whole API package and each caught by a
named assertion** (`tmp/h510/mutations.log`, replayable with
`tmp/h510/mutations.py`): the decoder, lookup, consume and token paths answering
without counting; the token path counting `acl` instead of `malformed`; `reply`
and the authentication gate each counting a second time; an internal failure
counted as a refusal; and an empty consume counted. Each names the call and what
moved — *an unparseable body moved map[], want exactly map[malformed:1]* — so a
failure says which path stopped counting rather than that a total is wrong.

## What it turned up

| | |
|---|---|
| counting cannot be derived from the status | `403` and `409` each carry three reasons. The first design counted by code and would have merged them |
| the consume bypass is one branch, not two | a filter matching nothing is an ordinary empty wait, not a refusal. The only bypass was the topic naming a name |
| `/consume` defaults its inbox to the caller | the sole HTTP override is a caller-visible topic named with `topic=` and no tag; there is no `name=` parameter. Found by a fixture that passed one and watched it read the caller's own inbox, and it corrected a specification claim as well ([delegated drain](../web-handoff/pages.md#overview-)) |
| the coverage is a fact about the paths there are | every refusing path calls the shared counter, and nothing structural prevents a future handler answering around it. A guard on the writer's callers was considered and rejected: it would pin this implementation and still miss a new handler reaching for `http.Error` or the raw writer. The contract and the pages say what is counted, not that a bypass is impossible |

## Verification

Codex ran `PORT=23911 bash src/smoke.sh --slow` against the final code: **573 passed, 0 failed**, exit 0, including vet, race, MCP and launcher checks. Log: `tmp/h510/slow-final.log`. Claude and opencode reviewed the implementation; the release is committed, not deployed by this task.
