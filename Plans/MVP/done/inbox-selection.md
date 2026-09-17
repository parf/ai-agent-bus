# Explicit inbox selection

📌 **TL;DR:** 0.5.52 separates the inbox address from message topic and tag filters in every face.

## Scope

`GET /consume` accepts an optional `inbox`. The CLI exposes it as `--inbox`;
the MCP `ab_consume` tool exposes the same field. Omission reads the caller's
own inbox. Topic and tag only filter inside the selected inbox, regardless of
whether a topic looks like an address or names a registered channel.

Core authorization, delivery and refusal reasons are unchanged. An explicit
inbox is still read as the authenticated caller: existing access, standing,
Disabled and single-reader rules decide the result.

## Checks

- Default own-inbox, explicit unfiltered inbox and explicit filtered inbox
  reads are separate positive controls.
- A registered channel name used as a topic still filters the caller's inbox;
  an unmatched address-shaped topic ends as an empty wait rather than the
  retired overload's `404`.
- Unauthorized selection counts a refusal on the selected record rather than
  the caller's record. A missing selected inbox remains `404 unknown`.
- An explicitly empty inbox is malformed; omission alone selects the caller.
- CLI query construction and MCP schema/forwarding carry inbox independently
  of topic and tag.
- While MCP push owns the caller's inbox, an explicit other inbox can be read;
  explicitly selecting the caller's inbox reaches the daemon's `409` reader
  gate rather than the face's convenience guard.

OpenCode reviewed the source and approved it after the empty-inbox edge was
added to both the HTTP and MCP checks.

Final frozen `src/inbox-selection-smoke.local.sh --slow`: **589 passed, 0
failed**, exit 0; vet and race passed (`tmp/inbox-selection/slow-final.log`,
lines 10–11 and 940). The frozen script and tracked `src/smoke.sh` both have
SHA-256 `628a2c98fb76456cf629b44fb26dfb99903c59e23a43f27cc3577bb90005c0a8`.
The 178-entry tracked/new source manifest was unchanged after the run
(`source-frozen.sha256` and `source-verify.log`).

## Mutations

The targeted run caught all **8 of 8** mutations
(`tmp/inbox-selection/mutations.log`).

| Changed behavior | Named catch |
|---|---|
| Ignore the explicit inbox | Explicit authorized reads return the wrong queue |
| Treat an empty inbox as omission | `inbox=` reads the caller's queue instead of answering `400` |
| Restore topic-address inbox selection | A registered address-shaped filter reads the channel queue |
| Discard topic and tag filters | Distractors win ahead of the requested message |
| Charge a refusal to the caller's inbox | Selected-record and caller-record refusal totals disagree |
| Drop `--inbox` from the CLI query | CLI query construction loses the selected inbox |
| Accept a valueless CLI inbox | The required-value test reaches HTTP instead of refusing locally |
| Drop inbox from the MCP HTTP query | The direct MCP bus test observes no `inbox` parameter |

The first frozen slow run ended **588 passed, 1 failed** and receives no green
credit (`slow-initial-red.log`). Its explicit-own/push fixture passed `wait=0s`,
so the MCP face exhausted its local deadline and returned `nothing waiting`
without calling the daemon. The corrected fixture uses a positive wait; push
already holds the inbox, so core's `409` is immediate and the assertion now
requires that exact status and reader error.

An earlier freeze command used root-relative paths while already inside
`src/`; `cp` failed immediately and no test ran. It receives no evidence
credit.

## Limits

This changes selection syntax, not who may read an inbox. It adds no fallback
for the retired topic-address overload and no new refusal reason.
