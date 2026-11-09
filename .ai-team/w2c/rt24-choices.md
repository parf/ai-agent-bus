# RT24: answer format and status counts

📌 **Recommendation:** keep the existing text answers and RT23 status ask; add an explicit `--json` result option to the existing read-only body methods. Keep status counts node-wide, with exactly the authority and definitions of the existing `/status` response. This is a decision note, not an implementation.

## What is already settled

- The first methods are `help`, `status`, `version` and `ls`, all read-only. Each uses the authenticated sender's authority. No management methods or new daemon powers are proposed.
- Description asks already have their own shapes: human text for `help`, JSON metadata for `help-json`. `help-json` describes methods; it is not the result of running `status` or `ls`.
- Current `#bus` body answers are text. `status` and ask `status` both return `ok: N records, M readers, up D`; `version` returns version/build text; `ls` returns lines of caller-visible records.
- Existing source is the baseline, not a proposal to rename the built-in record. The TODO's historical realm-qualified spelling and the current protected `#bus` identity are outside these two choices.

Sources: [RT24 in TODO](../../Plans/R1.0-Release/TODO.md#bundled-agents), [agent self-description](../../docs/constitution.md#agent-self-description), [current bus guide](../../docs/agents/bus.md#technical-details), [method implementation](../../src/internal/api/daemon_agent.go), [status counters](../../src/internal/core/bus.go).

## Choice 1: answers an AI can read reliably

| Option | Shape | Benefit | Cost |
|---|---|---|---|
| A — JSON by default | Body methods return JSON; description/status asks keep their RT23 shapes | No result parsing beyond JSON | Changes the text answers already used by people and scripts; the same method and ask can look different |
| **B — text default, explicit JSON (recommended)** | Existing calls stay text; `status --json`, `version --json`, `ls … --json` return only JSON in the reply body | Keeps a short human answer and gives an AI a declared machine form | Adds one output option to the existing methods |
| C — text only | Retain the current line and tab-separated forms, documented precisely | Least work | An AI must parse human wording, tabs, empty-list text and changing descriptions |

For B:

- `ask: status` stays one RT23 text line. A caller wanting machine data sends ordinary body `status --json`. The analogous version and listing calls use the same option. `help` stays text; ask `help-json` already supplies machine-readable metadata.
- A JSON result is the existing authorized daemon result, not a text preamble plus JSON, a Markdown fence or a second transport envelope. In particular an empty listing is `[]`, not “No matching records.”
- Reuse the existing API's field meanings and refusal classes. Do not invent a parallel status schema, new privilege checks, automatic retries or a health score. The exact declared return shapes must be pinned in the acceptance tests before implementation lands.
- Method help and `help-json` declare the output option. The bus reply envelope continues to carry routing/correlation; the body contains the chosen representation only.
- `ls` keeps precisely the current caller-visible selection, including its live-agent default and filters. JSON must never bypass that selection or expose private fields omitted by the existing API.

## Choice 2: what status counts

| Option | Meaning | Benefit | Cost |
|---|---|---|---|
| **A — existing node totals (recommended)** | The same snapshot as `/status`, obtained through `StatusFor(authenticated sender)` | One meaning for CLI, web, HTTP and `#bus`; no new authority or expensive listing pass | Must label these as node totals, since they differ from a caller's `ls` count |
| B — caller-visible totals | Count only records the caller can discover, plus a separately defined subset of their readers | Describes the caller's own view | Introduces new counting semantics and questions about visible records versus readable inbox statistics; not a one-to-one existing status call |
| C — both scopes | Return node totals and separately labelled visible totals | Answers both questions | More fields/work and both sets of definitions; unnecessary for the first read-only set |

For A, state the definitions beside the answer/help:

- **Records:** active/live records across all kinds on this node, using the existing core liveness rule; not just agents, owned records, visible records or readers. Inactive records and records hidden by an inactive owning user do not enter this count.
- **Readers:** outstanding consume waiters across inboxes, including filtered waits. This is a poll count, not unique agent names, processes or healthy workers. A transient cancelled poll can remain visible until cleanup. A handler executing work need not have an outstanding read at that moment.
- **Uptime:** this daemon process's existing `up` value. Counts are a point-in-time observation, not a transaction covering a later `ls`.
- Existing status facts such as queued, dropped and expired keep their API definitions if included in JSON: queued is current depth; dropped/expired are cumulative since start. They are not rates or proof of a current fault.
- Keep the existing authority split: general node totals are already exposed by `/status`; the owner-inactive detail is only returned to the daemon Owner/active Administrators by `StatusFor`, and remains absent for others. Do not turn an absent privileged field into zero.

## What `ok` means

Recommend `ok` to mean this authenticated read-only status request was served by the daemon. It does not certify every helper, an empty queue, delivery guarantees or external services. Zero readers, queue backlog and historical drop/refusal counters alone must not manufacture `degraded`.

The current implementation has no general live health aggregate behind `degraded`. Do not add thresholds, host probes or new fault state as part of closing these two choices. If a `degraded:` branch is required now, its trigger must first name an existing observable current fault; a previous unclean stop is historical context, not proof the present daemon is degraded.

## Proposed acceptance after the decision

- Text default and the status ask retain the chosen RT23 form. Each JSON body parses directly; empty listing is an array, and no preamble/fence appears.
- Two callers with different ACL visibility get different `ls` results and the same general node totals. Privileged-only status details remain absent for the ordinary caller.
- Fixtures pin live versus inactive records, all record kinds, filtered waiters versus unique agents, busy handlers without waiters, and current queue depth versus cumulative counters.
- Restore a text wrapper around JSON, broaden listing authority, count only agents as records, count only unfiltered waiters, or expose an owner-only detail: each owning check must fail on an assertion.

**Decision requested from #ab-dir:** choose answer option B and count option A, with the narrow `ok` meaning above; or select the alternatives and settle the additional semantics they introduce. No code starts before that decision.
