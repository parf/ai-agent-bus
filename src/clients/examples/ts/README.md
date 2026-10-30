# TypeScript JSON validator

📌 **TL;DR:**
- **Problem:** a JSON file can be valid and still hide TODOs or be misread; one worker should not be the only judge.
- **Outcome:** `#stats-ts@localhost` validates a JSON file, reports its top-level type and keys, then asks Python for a TODO audit and PHP's statistics.
- **Start:** [run it](#try-this-first) with the other examples on a throwaway bus.

## Why you'd want it

- 🧩 **Local check plus peer review.** An unavailable auditor is reported separately from the JSON's validity.
- 🧮 **A counter for free.** Each inspected file increments the worker's own `files` key-value integer.

## What you get

- ✅ **A `validate-json` topic:** `{"path":"/absolute/path/file.json"}` returns `worker`, `path`, `valid`, `type` and `keys` (or `error`), and `audit` (or `audit_error`).
- ✅ **`stats` and `relay`** for interop checks: bytes, newline-based lines, literal TODO count and SHA-256; relay appends `ts` and calls the next peer until the route is empty.

## Try this first

Run every example, TypeScript among them:

```sh
src/clients/examples/run.sh
```

```text
== ts
{
  "worker": "ts",
  "path": "…/samples/sample.json",
  "valid": true,
  "type": "object",
  "keys": ["service", "agents", "retries", "enabled"],
  "audit": { "worker": "python", "…": "…" }
}
TypeScript validation assertions passed; invalid UTF8 rejected
```

- The demo needs the Python and PHP peers, and asserts the JSON type and keys, both peer identities and the rejection of an invalid UTF-8 file.
- A second caller can run `demo.sh /absolute/path/file.json` with its own bus credentials.

## Technical details

- **Running alone:** run `run.sh` with `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN`, `AGENT_BUS_NAME` and `STATS_ROOT` after registering the name in `agents.txt`; Bun runs the native TypeScript client. The shared [harness](../README.md) supplies the principals and sample files.
- **Paths:** the real file must be inside `STATS_ROOT`; opened-descriptor checks prevent symlink replacement from reading outside it. Invalid requests return JSON errors.
- **Serving:** two handlers allow peer calls while serving. SIGTERM stops intake and drains active work for up to 25 seconds.
