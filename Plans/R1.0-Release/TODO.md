# TODO R1

## Objective

Prepare the [proposed scope](README.md#scope). Started ahead of the release on
the 0.8 line: shared locks, Resource records, the key-value store and the
first part of roles are built ([scope](README.md#scope)); no release wave is
committed yet.

## Next step

Record-defined roles come first: how a role reaches an Agent and the generated
`owner` and `maintainer` roles are built; assigned roles in allow lists
([role syntax](roles.md#role-syntax)) are next. The other kept topics — method metadata, federation, installable distribution,
the managed runner and the client libraries — need resolution of their
[questions](QUESTIONS.md#open-questions), the split's Q139 first. Federation is on hold. Existing
decisions describe targets, not completed code. Identity, AUTH, encryption and
observability are [R1.1](../R1.1/TODO.md#objective).

## Dependencies

| Candidate work | Must precede it |
|---|---|
| [Record-defined roles](roles.md#record-defined-roles) | Typed actor terms and User ownership, built in 0.7; the transport, built in 0.8.97; an owner-approved storage representation for assigned roles |
| Federation — on hold | The owner taking it off hold; then the namespace decision ([Q20](QUESTIONS.md#open-questions)) and what chaining needs from R1.1 ([Q137](QUESTIONS.md#open-questions)) |
| Managed runner | Edge identity, config change behavior and dormant activation decisions (Q15, Q19, Q12); [method metadata](method-metadata.md#method-metadata); the pool hostname field ([Q139](QUESTIONS.md#open-questions)) |
| [Service method metadata](method-metadata.md#method-metadata) | The MVP description-only behavior it replaces ([decision](../../docs/decisions.md#settled)); an owner-approved representation |
| [Client libraries](client-libraries.md#scope) | Review the proposed interface; owner-approved protocol description ([Q17](QUESTIONS.md#open-questions)) |
| Release distributions | Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging |

Name implementation waves and falsifiable acceptance after those choices.
Federation acceptance must exercise distinct nodes.

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
