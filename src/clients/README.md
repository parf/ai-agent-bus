# Client libraries

📌 **TL;DR:**
- **Problem:** an agent or service in another language should reach the bus without hand-written HTTP, retries or reply matching.
- **Outcome:** six native client libraries (Python, Rust, PHP, Java, Go, TypeScript) behave the same way: send, call, serve, key-value, locks and read-only settings.
- **Start:** pick your language in [the table](#what-you-get), then run its example ([Try this first](#try-this-first)).

## Why you'd want it

- 🔌 **Your language, your idioms.** Each library uses its own types and execution model, so it feels native.
- 🧾 **No protocol code.** Calls, replies, acks and deadlines are handled for you.
- 🧪 **One behaviour everywhere.** The same request and answer cases run in all six, so a call means the same in each.

## What you get

- ✅ **Send and call:** an ack confirms pickup; an answer, done and timeout are distinct outcomes.
- ✅ **Serve:** a bounded pool of handlers answers the messages sent to your name.
- ✅ **Per-record key-value, locks and read-only settings:** each record owns its KV, locks, Configuration and Secret.
- ✅ **Safe by default:** an uncertain write is never repeated automatically; consumption is at most once.

| Language | Code | Execution |
|---|---|---|
| Python | [Python](python/README.md) | Blocking calls; bounded handler threads |
| Rust | [Rust](rust/README.md) | Async Tokio; bounded handler tasks |
| PHP | [PHP](php/README.md) | Blocking calls; worker processes |
| Java | [Java](java/README.md) | Blocking calls; bounded virtual threads |
| Go | [Go](go/README.md) | Contexts and bounded goroutines |
| TypeScript | [TypeScript](typescript/README.md) | Async Node/Bun; bounded handlers |

## Try this first

Run working agents in all six languages together on a disposable bus:

```sh
src/clients/examples/run.sh codestats
```

- The [examples](examples/README.md) cover audits, digests, JSON validation, file fingerprints and a relay ring.
- The run leaves nothing on your node: its daemon and records are gone when it ends.
- Packages stay local to this repository.

## Technical details

- `Bus` is the credentialed caller and a `Record` is a target; construction does not register a name.
- Keep secrets in Secret and config in Configuration ([rule](../../docs/03-records-agent.md#configuring-a-template)).
- The [shared vision](../../Plans/R1.0-Release/done/client-libraries.md#shared-vision) owns the contract.
- Lock helpers have no fencing token.
- Cached private copies outlive server revocation; refresh invalidates the client's cache.

### A send's wait

- A send's `wait` states how long the sender will wait for an answer; the daemon turns it into the message's deadline and refuses a zero one.
- Every client therefore treats a wait of nothing (`0`, `"0s"` or `"0ms"` where it takes a string, a zero duration where it takes one) as no wait.
- It leaves the field out of the request, so `send(wait=0)` delivers like a send without one.

### Checks

- `bash src/smoke.sh` runs the client checks with the daemon and faces.
- Individual checks use `bash src/clients/check.sh python|rust|php|typescript|java|go|examples`.
- Rust checks, examples and packaging share the main checkout's `tmp/cargo-target` across worktrees; an explicit `CARGO_TARGET_DIR` overrides it.
- Fixtures use private daemons and scratch under the repository's `tmp/`, not the live bus.
