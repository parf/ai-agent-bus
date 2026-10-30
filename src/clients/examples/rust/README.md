# Rust digest worker

📌 **TL;DR:**
- **Problem:** hashing the same file again and again wastes work, and a lone digest has no second opinion.
- **Outcome:** the Rust worker computes a file's SHA-256, caches the verified digest in its JSON key-value store under a record lock, and has Java confirm it on a cache miss.
- **Start:** [run it](#try-this-first) with the other examples on a throwaway bus.

## Why you'd want it

- ♻️ **Same bytes, one digest.** Changed bytes get a new digest key; unchanged files are answered from the cache.
- 🔐 **Cross-checked.** On a miss it calls `#stats-java@localhost` on `stats` and requires Java's SHA-256 to match.

## What you get

- ✅ **A `digest` topic:** `{"path":"file"}` returns `worker`, `path`, `bytes`, `sha256`, `cached`, `java_verified` and Java's stats answer; errors return `worker` and `error`.
- ✅ **`stats` and `relay`** for the shared example harness.

## Try this first

Run every example, Rust among them:

```sh
src/clients/examples/run.sh
```

```text
ack from #stats-rust@localhost
{"worker": "rust", "path": "…/samples/sample.json", "bytes": 122, "sha256": "dc6785ad516a5e49b546a71251baaf8347417eb921d9a6c9346067547a6011a3", "cached": true, "java_verified": true}
```

- `demo.sh [path]` makes a digest call as the harness owner, defaulting to `DEMO_FILE`.

## Technical details

- **Locks:** same-content Rust tasks share a local mutex; that avoids the daemon's immediate same-principal lock reacquisition refusal.
- **Harness:** the shared [harness](../README.md) registers and starts this worker and Java together. `run.sh` reads its credential through `../tokens.sh`, using the harness transport and `STATS_ROOT`; build output uses the [shared Cargo target](../../README.md#checks).
- **Paths:** relative paths resolve under `STATS_ROOT`, and canonical paths must stay inside that root.
- **Topics:** stats counts bytes, LF lines plus a final unterminated line, literal TODO markers and SHA-256. Relay pops the next route name, appends `rust` to hops and delegates.
- **Serving:** two handlers run concurrently. SIGTERM stops intake and drains accepted work.
