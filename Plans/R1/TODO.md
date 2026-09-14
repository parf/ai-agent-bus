# TODO R1

## Objective

Prepare the [proposed scope](README.md#scope) after MVP acceptance. Not started; there are no implementation waves committed yet.

## Next step

Owner confirms the scope and resolves [questions](QUESTIONS.md#open-questions). Existing decisions describe targets, not completed code.

## Dependencies

| Candidate work | Must precede it |
|---|---|
| Scoped credentials and encryption | [Settled lifetime policy](../../docs/02-access.md#token-lifetime); remaining key recovery and token grammar decisions |
| D.1–D.4 encryption carried from MVP | [Encryption acceptance](encryption-wave.md#d--the-bus-stops-reading-payloads); key lifecycle decisions |
| Federation | Namespace, record authenticity and clock decisions |
| Managed runner | Edge identity, config change behavior and dormant activation decisions; current method metadata work |
| Client libraries | Owner-approved protocol description |
| [Optional dashboard extensions](discovery.md#dashboard-extensions) | Required MVP dashboard acceptance; the AUTH, health, stats, federation and runner data each additional view reports |
| AUTH authorization freshness | [Q35](QUESTIONS.md#authorization-refresh), independently of credential lifetime |
| Release distributions | MVP installed acceptance, release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging |

Name implementation waves and falsifiable acceptance after those choices. Crypto acceptance must test what the daemon cannot decrypt; federation acceptance must exercise distinct nodes.

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
