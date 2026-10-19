# Client libraries

Clients share one model: `Bus` is the credentialed caller, a `Record` is a target,
and each record owns its KV, locks and read-only Configuration and Secret.
Keep secrets in Secret and config in Configuration ([rule](../../docs/03-records-agent.md#configuring-a-template)). Construction
does not register a name. The [shared vision](../../Plans/R1.0-Release/done/client-libraries.md#shared-vision)
owns the contract; each language uses its own types and execution model.

| Language | Code | Execution |
|---|---|---|
| Python | [Python](python/README.md) | Blocking calls; bounded handler threads |
| Rust | [Rust](rust/README.md) | Async Tokio; bounded handler tasks |
| PHP | [PHP](php/README.md) | Blocking calls; worker processes |
| Java | [Java](java/README.md) | Blocking calls; bounded virtual threads |
| Go | [Go](go/README.md) | Contexts and bounded goroutines |
| TypeScript | [TypeScript](typescript/README.md) | Async Node/Bun; bounded handlers |

The [examples](examples/README.md) run native agents together on a disposable bus:
audits, digests, JSON validation, file fingerprints and a relay ring.

Packages remain local;
no package registry publication is part of this work.

An ack confirms pickup; an answer, done and timeout are distinct outcomes.
Consumption is at most once. Clients never automatically repeat an uncertain
write. Lock helpers have no fencing token. Cached private copies outlive server
revocation; refresh invalidates the client's cache.

## Checks

`bash src/smoke.sh --slow` runs the client checks with the daemon and faces.
Individual checks use `bash src/clients/check.sh python|rust|php|typescript|java|go|examples`. Fixtures use
private daemons and scratch under the repository's `tmp/`, not the live bus.
