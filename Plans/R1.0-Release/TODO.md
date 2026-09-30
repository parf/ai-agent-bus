# TODO R1

## Objective

Prepare the [proposed scope](README.md#scope). Not started; there are no
implementation waves committed yet.

## Next step

Record-defined roles come first. The other kept topics — locks, the key-value
store, Resource records, method metadata, federation, installable distribution,
the managed runner and the client libraries — need resolution of their
[questions](QUESTIONS.md#open-questions), the split's Q138 and Q139 first. Federation is on hold. Existing
decisions describe targets, not completed code. Identity, AUTH, encryption and
observability are [R1.1](../R1.1/TODO.md#objective).

## Dependencies

| Candidate work | Must precede it |
|---|---|
| [Record-defined roles](roles.md#record-defined-roles) | Typed actor terms and User ownership, built in 0.7; an owner-approved storage and transport representation; [Q138](QUESTIONS.md#open-questions) |
| Federation — on hold | The owner taking it off hold; then the namespace decision ([Q20](QUESTIONS.md#open-questions)) and what chaining needs from R1.1 ([Q137](QUESTIONS.md#open-questions)) |
| Managed runner | Edge identity, config change behavior and dormant activation decisions (Q15, Q19, Q12); [method metadata](method-metadata.md#method-metadata); the pool hostname field ([Q139](QUESTIONS.md#open-questions)) |
| [Service method metadata](method-metadata.md#method-metadata) | The MVP description-only behavior it replaces ([decision](../../docs/decisions.md#settled)); an owner-approved representation |
| Client libraries | Owner-approved protocol description ([Q17](QUESTIONS.md#open-questions)) |
| Release distributions | Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging |
| [Key-value store](kv.md#per-record-storage) — `kv_get`, `kv_set(record, name, value, how)` with `how` set, add or replace, `kv_delete`, `kv_inc`, JSON operations; the record's Owner, Maintainers and own Agent | the persistence ports it shares with the rest of the daemon's state |

Name implementation waves and falsifiable acceptance after those choices.
Federation acceptance must exercise distinct nodes.

## Key-value acceptance

| Task | Done when; mutation that must fail |
|---|---|
| KV.1 Store and authority | Values of each type round-trip on a record, survive a restart and are no such entity while the record is inactive. The Owner, a Maintainer (through a group) and the record's own Agent read and write; a caller on its allow list is refused. Letting the allow list in, or dropping the store with an inactive record, fails a named check |
| KV.2 `kv_set` modes | `set` writes; `add` refuses a present name and `replace` an absent one, each saying so. Two concurrent `add`s of one name: exactly one succeeds. Treating `add` as `set`, or a non-atomic check-then-write, fails a named check |
| KV.3 Increment and JSON operations | `kv_inc` from many concurrent callers ends at the exact sum; `push`, `pull` and add-to-set are atomic inside a `json` value. A read-modify-write outside the store's lock fails a named check |

## Backup acceptance

The [backup contract](runner.md#backing-it-up) is settled; its invocation remains
[Q33](QUESTIONS.md#open-questions). Implementation must restore an archive with
the intended user's key and refuse an unrelated key. Replace encryption with
plaintext output and the format/decryption check must fail; encrypt to the
wrong recipient and the intended-user restore must fail. Confirm backup creation
works with only the public key available.

## Distribution acceptance

Implement the [release artifacts](distribution.md#release-artifacts) and [container contract](distribution.md#container-runtime).

| Task | Done when; mutation that must fail |
|---|---|
| P.1 npm package | On a clean supported host without the checkout or Go toolchain, install the published package, start the bus, complete a service request/reply and an MCP tool call. Omit a required executable or MCP runtime asset from the package and the corresponding exercise fails |
| P.2 Container image | Pull the published image on a clean host, start each role from the supplied instructions and complete a service request/reply. Break either role's entry point and delivery fails. Recreate the daemon container with its volume and verify the registered service and issued credential still work; move storage outside the volume and recovery fails. Request unavailable sandboxing and require refusal; silently downgrading must fail the check |
| P.3 Release evidence | Query every shipped program's version in both distributions and compare with the release source; require stamped Go build information and packaged license. Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check |
