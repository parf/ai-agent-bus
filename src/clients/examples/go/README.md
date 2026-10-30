# Go: a verified file fingerprint

📌 **TL;DR:**
- **Problem:** a digest you computed alone is only as good as your code; a second language should confirm it.
- **Outcome:** the Go agent fingerprints a file (bytes, lines, blank lines, longest line, `TODO` count), then has Rust and Java verify the SHA-256.
- **Start:** [run it](#try-this-first) with the other examples on a throwaway bus.

## Why you'd want it

- 🔐 **Independent checking.** Rust computes the digest, and Java checks Rust's.
- 🔒 **No duplicate work.** A per-file lock refuses concurrent runs on the same file.

## What you get

- ✅ **A `fingerprint` topic:** `{"path":"/absolute/file"}` returns the inventory and `verified_by`.
- ✅ **A counter:** each successful report increments this agent's `files` key-value entry.

## Try this first

Run every example, Go among them:

```sh
src/clients/examples/run.sh
```

```text
== go
{"worker":"go","path":"…/samples/sample.json","bytes":122,"lines":1,"blank_lines":0,"longest_line_bytes":121,"todos":0,"sha256":"dc6785ad516a5e49b546a71251baaf8347417eb921d9a6c9346067547a6011a3","verified_by":"rust","java_verified":true}
```

- 👤 Under another caller's credentials, `./demo.sh /absolute/file` makes the native Go call and prints its answer.

## Technical details

- **Harness:** `run.sh` always selects `#stats-go@localhost` and reads its token from `EXAMPLES_TOKENS`; it never adopts an inherited caller identity. The shared [harness](../README.md) registers the names, and the worker never changes registry state. Start the Rust digest worker with the same root.
- **Paths:** they must resolve inside `STATS_ROOT`, and Go's rooted file handle also prevents symlink changes from escaping the root.
- **Topics:** `stats` runs the local inventory alone; `relay` appends `go` to `hops` and calls the first remaining `route` name. Errors answer `{"worker":"go","error":"…"}`.
- **Serving:** SIGTERM stops intake and drains handlers for up to 25 seconds. Four workers let nested calls keep flowing.
